package main

import (
	"context"
	"database/sql"
	"fmt"
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

	// OpenTelemetry Provider
	shutdownOtel, err := otel.InitProvider(ctx, config.OtelServiceName, config.OtelExporterOTLPEndpoint)
	if err != nil {
		log.Fatalf("failed to init OTel: %v", err)
	}
	defer shutdownOtel()

	// Postgres Connection
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)
	db, err := sql.Open(config.DBDriver, dsn)
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	defer db.Close()

	repository := database.NewOrderRepository(db)

	// gRPC Client
	grpcURL := fmt.Sprintf("%s:%s", config.FleetHost, config.FleetPort)
	grpcConn, err := grpc.NewClient(grpcURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		log.Fatalf("grpc connection failed: %v", err)
	}
	defer grpcConn.Close()
	grpcClient := pb.NewFleetServiceClient(grpcConn)

	// 5. RabbitMQ Connection
	rabbitURL := fmt.Sprintf("amqp://guest:guest@%s:%s/", config.RabbitMQHost, config.AMQPort)
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		log.Fatalf("rabbitmq connection failed: %v", err)
	}
	defer conn.Close()

	// Consumer Logic
	consumer := event.NewConsumer(conn, grpcClient, repository)

	// consumer em uma goroutine para não bloquear o shutdown
	errChan := make(chan error, 1)
	go func() {
		log.Printf("Worker [%s] consuming from orders.created...", config.OtelServiceName)
		if err := consumer.Start("orders.created"); err != nil {
			errChan <- err
		}
	}()

	// Wait for exit signal or error
	select {
	case <-ctx.Done():
		log.Println("Worker stopping gracefully...")
	case err := <-errChan:
		log.Fatalf("Worker consumer error: %v", err)
	}

	//
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Worker exited")
}
