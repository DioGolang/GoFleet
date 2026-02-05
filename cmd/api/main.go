package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DioGolang/GoFleet/configs"
	"github.com/DioGolang/GoFleet/internal/application/usecase/order"
	"github.com/DioGolang/GoFleet/internal/domain/event"
	"github.com/DioGolang/GoFleet/internal/infra/database"
	infraEvent "github.com/DioGolang/GoFleet/internal/infra/event"
	"github.com/DioGolang/GoFleet/internal/infra/web/handler"
	middlewareMetrics "github.com/DioGolang/GoFleet/internal/infra/web/middleware"
	"github.com/DioGolang/GoFleet/pkg/logger"
	"github.com/DioGolang/GoFleet/pkg/metrics"
	"github.com/DioGolang/GoFleet/pkg/otel"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/riandyrn/otelchi"
)

func main() {
	config, err := configs.LoadConfig(".", "fleet-api")
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// SETUP OpenTelemetry
	shutdownOtel, err := otel.InitProvider(ctx, config.OtelServiceName, config.OtelExporterOTLPEndpoint)
	if err != nil {
		log.Fatalf("failed to init OTel: %v", err)
	}
	defer shutdownOtel()

	// =========================================================================
	// METRICS
	// =========================================================================
	reg := prometheus.NewRegistry()
	prometheusMetrics := metrics.NewPrometheusMetrics(reg, config.OtelServiceName)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		err = http.ListenAndServe(":2112", mux)
		if err != nil {
			return
		}
	}()

	// =========================================================================
	//LOG
	// =========================================================================
	zapLogger := logger.NewZapLogger(config.OtelServiceName, false)
	fail := func(msg string, err error) {
		zapLogger.Error(ctx, msg, logger.WithError(err))
		os.Exit(1)
	}
	zapLogger.Info(ctx, "Application starting")

	// =========================================================================
	// DATABASE (PostgreSQL)
	// =========================================================================
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)
	db, err := sql.Open(config.DBDriver, dsn)
	if err != nil {
		fail("db driver error", err)
	}
	if err := db.PingContext(ctx); err != nil {
		fail("db connection unreachable", err)
	}
	defer func(db *sql.DB) {
		zapLogger.Info(ctx, "Closing Database...")
		err := db.Close()
		if err != nil {
			fail("Error closing database", err)
		}
	}(db)

	// =========================================================================
	// MESSAGING (RabbitMQ)
	// =========================================================================
	rabbitURL := fmt.Sprintf("amqp://guest:guest@%s:%s/", config.RabbitMQHost, config.AMQPort)
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		fail("rabbitmq connection failed", err)
	}
	defer func(conn *amqp.Connection) {
		zapLogger.Info(ctx, "Closing RabbitMQ...")
		err := conn.Close()
		if err != nil {
			fmt.Printf("Error closing RabbitMQ: %v\n", err)
		}
	}(conn)

	ch, err := conn.Channel()
	if err != nil {
		fail("rabbitmq channel failed", err)
	}
	defer func(ch *amqp.Channel) {
		zapLogger.Info(ctx, "Closing rabbitmq channel...")
		err := ch.Close()
		if err != nil {
			fmt.Printf("Error closing rabbitmq channel: %v\n", err)
		}
	}(ch)

	err = ch.ExchangeDeclare(
		"orders_exchange", "direct", true, false, false, false, nil,
	)
	if err != nil {
		fail("failed to declare exchange", err)
	}

	// =========================================================================
	// DEPENDENCIES: OUTBOX & UNIT OF WORK
	// =========================================================================

	uow := database.NewUnitOfWork(db)
	eventDispatcher := infraEvent.NewDispatcher(ch, zapLogger)
	queries := database.New(db)
	relay := infraEvent.NewOutboxRelay(queries, db, eventDispatcher, zapLogger)

	go func() {
		zapLogger.Info(ctx, "Starting Outbox Relay...")
		relay.Run(ctx)
	}()

	go func() {
		go relay.RunRescuer(ctx)
	}()

	// =========================================================================
	// RATE LIMITER (Defense in Depth)
	// =========================================================================
	// Config: 10 req/s, Burst de 20. Limpeza a cada 1 min.
	rateLimiter := middlewareMetrics.NewRateLimiter(middlewareMetrics.RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             20,
		CleanupInterval:   1 * time.Minute,
		ClientTimeout:     3 * time.Minute,
	})

	// =========================================================================
	// DEPENDENCIES & HANDLERS
	// =========================================================================
	orderCreated := event.NewOrderCreated()
	createOrderUseCase := order.NewCreateOrderUseCase(uow, orderCreated, zapLogger)

	createOrderUseCaseWithMetrics := &order.CreateOrderMetricsDecorator{
		Next:    createOrderUseCase,
		Metrics: prometheusMetrics,
	}
	orderHandler := handler.NewOrderHandler(createOrderUseCaseWithMetrics, zapLogger)

	// ROUTER COM OTEL MIDDLEWARE
	r := chi.NewRouter()
	r.Use(otelchi.Middleware(config.OtelServiceName, otelchi.WithChiRoutes(r)))
	//r.Use

	//Checks
	healthHandler := handler.NewHealthHandler(
		config.OtelServiceName,
		handler.WithPostgres(db),
		handler.WithRabbitMQ(rabbitURL),
	)

	//API
	r.Use(rateLimiter.Handler(zapLogger))
	r.Use(middlewareMetrics.MetricsWrapper(prometheusMetrics))
	r.Use(middlewareMetrics.RequestLogger(zapLogger))
	r.Use(middleware.Recoverer)

	r.Get("/health", healthHandler.ServeHTTP)
	r.Post("/api/v1/orders", orderHandler.Create)

	// HTTP SERVER SHUTDOWN
	srv := &http.Server{
		Addr:    ":" + config.WebServerPort,
		Handler: r,
	}

	go func() {
		zapLogger.Info(ctx, "Server running on port", logger.String("WebServerPort", config.WebServerPort))

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fail("listen: %s\n", err)
		}
	}()

	<-ctx.Done()

	zapLogger.Info(ctx, "\nShutting down gracefully...")

	// Timeout to finish pending requests
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		fail("Server forced to shutdown", err)
	}
	zapLogger.Info(ctx, "Server exited cleanly")
}
