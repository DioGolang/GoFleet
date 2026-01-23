package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DioGolang/GoFleet/configs"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/service"
	"github.com/DioGolang/GoFleet/pkg/otel"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, err := configs.LoadConfig(".", "gofleet-fleet")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Setup OpenTelemetry
	shutdownOtel, err := otel.InitProvider(ctx, config.OtelServiceName, config.OtelExporterOTLPEndpoint)
	if err != nil {
		log.Fatalf("failed to init OTel: %v", err)
	}
	defer shutdownOtel()

	// Redis Client
	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort),
	})
	// Check connection com timeout
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		log.Fatalf("failed to connect to Redis: %v", err)
	}
	defer func(rdb *redis.Client) {
		fmt.Println("Closing redis...")
		err := rdb.Close()
		if err != nil {
			fmt.Printf("Error closing redis: %v\n", err)
		}
	}(rdb)

	// Service & Seeding
	fleetService := service.NewFleetService(rdb)
	setupSeedData(ctx, fleetService)

	// gRPC Server com Interceptor OTel
	lis, err := net.Listen("tcp", ":"+config.FleetPort)
	if err != nil {
		log.Fatalf("failed to listen on port %s: %v", config.FleetPort, err)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	pb.RegisterFleetServiceServer(grpcServer, fleetService)
	reflection.Register(grpcServer)

	// Start gRPC Server
	go func() {
		log.Printf("Fleet gRPC Server running on port %s", config.FleetPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// 8. Graceful Shutdown
	<-ctx.Done()
	log.Println("Shutting down fleet service...")

	grpcServer.GracefulStop()
	log.Println("Service exited cleanly")
}

func setupSeedData(ctx context.Context, s *service.FleetService) {
	_ = s.UpdateDriverPosition(ctx, "Joao-da-Silva", -23.55, -46.63)
	_ = s.UpdateDriverPosition(ctx, "Maria-Longe", -23.60, -46.70)
	fmt.Println("Simulated GPS data loaded into Redis!")
}
