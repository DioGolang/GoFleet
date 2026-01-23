package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/DioGolang/GoFleet/configs"
	"github.com/DioGolang/GoFleet/internal/application/usecase"
	"github.com/DioGolang/GoFleet/internal/domain/event"
	"github.com/DioGolang/GoFleet/internal/infra/database"
	infraEvent "github.com/DioGolang/GoFleet/internal/infra/event"
	"github.com/DioGolang/GoFleet/internal/infra/web"
	"github.com/DioGolang/GoFleet/pkg/otel"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/riandyrn/otelchi"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	config, err := configs.LoadConfig(".", "fleet-api")
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 2. SETUP OpenTelemetry
	shutdownOtel, err := otel.InitProvider(ctx, config.OtelServiceName, config.OtelExporterOTLPEndpoint)
	if err != nil {
		log.Fatalf("failed to init OTel: %v", err)
	}
	defer shutdownOtel()

	// DATABASE (PostgreSQL)
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)
	db, err := sql.Open(config.DBDriver, dsn)
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	defer func(db *sql.DB) {
		fmt.Println("Closing Database...")
		err := db.Close()
		if err != nil {
			fmt.Printf("Error closing database: %v\n", err)
		}
	}(db)

	// MESSAGING (RabbitMQ)
	rabbitURL := fmt.Sprintf("amqp://guest:guest@%s:%s/", config.RabbitMQHost, config.AMQPort)
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		log.Fatalf("rabbitmq connection failed: %v", err)
	}
	defer func(conn *amqp.Connection) {
		fmt.Println("Closing RabbitMQ...")
		err := conn.Close()
		if err != nil {
			fmt.Printf("Error closing RabbitMQ: %v\n", err)
		}
	}(conn)

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("rabbitmq channel failed: %v", err)
	}
	defer func(ch *amqp.Channel) {
		fmt.Println("Closing rabbitmq channel...")
		err := ch.Close()
		if err != nil {
			fmt.Printf("Error closing rabbitmq channel: %v\n", err)
		}
	}(ch)

	_, err = ch.QueueDeclare(
		"orders.created", // name
		true,             // durable
		false,            // delete when unused
		false,            // exclusive
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		panic(err)
	}

	err = ch.QueueBind(
		"orders.created",
		"orders.created",
		"amq.direct",
		false,
		nil,
	)
	if err != nil {
		panic(err)
	}

	// DEPENDENCIES & HANDLERS
	orderRepository := database.NewOrderRepository(db)
	orderCreated := event.NewOrderCreated()
	eventDispatcher := infraEvent.NewDispatcher(ch)
	createOrderUseCase := usecase.NewCreateOrderUseCase(orderRepository, orderCreated, eventDispatcher)
	orderHandler := web.NewOrderHandler(createOrderUseCase)

	// ROUTER COM OTEL MIDDLEWARE
	r := chi.NewRouter()
	r.Use(otelchi.Middleware(config.OtelServiceName, otelchi.WithChiRoutes(r)))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/api/v1/orders", orderHandler.Create)

	// HTTP SERVER SHUTDOWN
	srv := &http.Server{
		Addr:    ":" + config.WebServerPort,
		Handler: r,
	}

	go func() {
		fmt.Println("Server running on port", config.WebServerPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-ctx.Done()
	fmt.Println("\nShutting down gracefully...")

	// Timeout to finish pending requests
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	fmt.Println("Server exited cleanly")
}
