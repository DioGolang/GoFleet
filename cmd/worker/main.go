package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/DioGolang/GoFleet/pkg/logger"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DioGolang/GoFleet/configs"
	"github.com/DioGolang/GoFleet/internal/infra/database"
	"github.com/DioGolang/GoFleet/internal/infra/event"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	"github.com/DioGolang/GoFleet/pkg/otel"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, err := configs.LoadConfig(".", "gofleet-worker")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	//LOG
	zapLogger := logger.NewZapLogger(config.OtelServiceName, false)
	zapLogger.Info(ctx, "Worker initializing...")
	fail := func(msg string, err error) {
		zapLogger.Error(ctx, msg, logger.WithError(err))
		os.Exit(1)
	}

	// OpenTelemetry Provider
	shutdownOtel, err := otel.InitProvider(ctx, config.OtelServiceName, config.OtelExporterOTLPEndpoint)
	if err != nil {
		fail("failed to init OTel", err)
	}
	defer shutdownOtel()

	// Postgres Connection
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)
	db, err := sql.Open(config.DBDriver, dsn)
	if err != nil {
		fail("db connection failed: %v", err)
	}
	defer func(db *sql.DB) {
		zapLogger.Info(ctx, "Closing Database...")
		err := db.Close()
		if err != nil {
			zapLogger.Error(ctx, "Error closing database", logger.WithError(err))
		}
	}(db)

	repository := database.NewOrderRepository(db)

	// gRPC Client
	grpcURL := fmt.Sprintf("%s:%s", config.FleetHost, config.FleetPort)
	grpcConn, err := grpc.NewClient(grpcURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}], "methodConfig": [{"name": [{"service": ""}], "retryPolicy": {"maxAttempts": 5, "initialBackoff": "0.1s", "maxBackoff": "1s", "backoffMultiplier": 2.0, "retryableStatusCodes": ["UNAVAILABLE"]}}]}`),
	)
	if err != nil {
		log.Fatalf("grpc connection failed: %v", err)
	}
	defer func(grpcConn *grpc.ClientConn) {
		fmt.Println("Closing gRPC...")
		err := grpcConn.Close()
		if err != nil {
			fmt.Printf("Error closing gRPC: %v\n", err)
		}
	}(grpcConn)
	grpcClient := pb.NewFleetServiceClient(grpcConn)

	// 5. RabbitMQ Connection
	rabbitURL := fmt.Sprintf("amqp://guest:guest@%s:%s/", config.RabbitMQHost, config.AMQPort)
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		fail("rabbitmq connection failed", err)
	}
	defer func(conn *amqp.Connection) {
		zapLogger.Info(ctx, "Closing RabbitMQ...")
		err := conn.Close()
		if err != nil {
			zapLogger.Error(ctx, "Error closing RabbitMQ", logger.WithError(err))
		}
	}(conn)

	// Consumer Logic
	consumer := event.NewConsumer(conn, grpcClient, repository, zapLogger)

	// consumer em uma goroutine para não bloquear o shutdown
	errChan := make(chan error, 1)
	go func() {
		zapLogger.Info(ctx, "Starting consumer loop", logger.String("queue", "orders.created"))
		if err := consumer.Start("orders.created"); err != nil {
			zapLogger.Error(ctx, "Consumer failed", logger.WithError(err))
			errChan <- err
		}
	}()

	// Wait for exit signal or error
	select {
	case <-ctx.Done():
		zapLogger.Info(ctx, "Worker stopping gracefully....")
	case err := <-errChan:
		fail("Worker consumer error", err)
	}

	//
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	zapLogger.Info(ctx, "Worker exited")
}
