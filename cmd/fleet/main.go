package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DioGolang/GoFleet/internal/infra/database"
	"github.com/DioGolang/GoFleet/internal/infra/web/handler"
	"github.com/DioGolang/GoFleet/pkg/logger"

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

	//LOG
	zapLogger := logger.NewZapLogger(config.OtelServiceName, false)
	zapLogger.Info(ctx, "Fleet initializing...")
	fail := func(msg string, err error) {
		zapLogger.Error(ctx, msg, logger.WithError(err))
		os.Exit(1)
	}

	// Setup OpenTelemetry
	shutdownOtel, err := otel.InitProvider(ctx, config.OtelServiceName, config.OtelExporterOTLPEndpoint)
	if err != nil {
		fail("failed to init OTel", err)
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
		fail("failed to connect to Redis: %v", err)
	}
	defer func(rdb *redis.Client) {
		fmt.Println("Closing redis...")
		err := rdb.Close()
		if err != nil {
			zapLogger.Error(ctx, "Error closing redis", logger.WithError(err))
		}
	}(rdb)

	locationRepo := database.NewRedisLocationRepository(rdb, zapLogger)

	// Service & Seeding
	fleetService := service.NewFleetService(locationRepo, zapLogger)
	setupSeedData(ctx, fleetService)

	// =========================================================================
	// MONITORING SERVER (Embedded Management Port)
	// =========================================================================
	go func() {
		mux := http.NewServeMux()

		healthHandler, err := handler.NewHealthHandler(
			handler.WithName(config.OtelServiceName, "1.0.0"),

			handler.WithRedis(func(ctx context.Context) error {
				return rdb.Ping(ctx).Err()
			}),
		)

		if err != nil {
			zapLogger.Error(ctx, "failed to init health handler", logger.WithError(err))
			return
		}

		mux.Handle("/health", healthHandler)

		// mux.Handle("/metrics", promhttp.Handler())

		zapLogger.Info(ctx, "Monitoring server running on :2112")
		if err := http.ListenAndServe(":2112", mux); err != nil {
			zapLogger.Error(ctx, "Monitoring server failed", logger.WithError(err))
		}
	}()

	// gRPC Server com Interceptor OTel
	lis, err := net.Listen("tcp", ":"+config.FleetPort)
	if err != nil {
		fail("failed to listen on port", err)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	pb.RegisterFleetServiceServer(grpcServer, fleetService)
	reflection.Register(grpcServer)

	// Start gRPC Server
	go func() {
		zapLogger.Info(ctx, "Fleet gRPC Server running on port", logger.String("FleetPort", config.FleetPort))
		if err := grpcServer.Serve(lis); err != nil {
			zapLogger.Error(ctx, "Fleet gRPC Server running on port", logger.WithError(err))
		}
	}()

	// 8. Graceful Shutdown
	<-ctx.Done()
	zapLogger.Info(ctx, "Shutting down fleet service...")

	grpcServer.GracefulStop()
	zapLogger.Info(ctx, "Service exited cleanly")
}

func setupSeedData(ctx context.Context, s *service.FleetService) {
	_ = s.UpdateDriverPosition(ctx, "Joao-da-Silva", -23.55, -46.63)
	_ = s.UpdateDriverPosition(ctx, "Maria-Longe", -23.60, -46.70)
	fmt.Println("Simulated GPS data loaded into Redis!")
}
