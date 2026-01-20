package fleet

import (
	"context"
	"fmt"
	"github.com/DioGolang/GoFleet/configs"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/service"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"net"
)

func main() {
	config, err := configs.LoadConfig(".")
	if err != nil {
		panic(err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort),
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		panic(fmt.Sprintf("Erro ao conectar no Redis: %v", err))
	}

	fleetService := service.NewFleetService(rdb)

	// --- SEEDING (Popular dados falsos para teste) ---
	ctx := context.Background()
	err = fleetService.UpdateDriverPosition(ctx, "Joao-da-Silva", -23.55, -46.63)
	if err != nil {
		return
	}
	err = fleetService.UpdateDriverPosition(ctx, "Maria-Longe", -23.60, -46.70)
	if err != nil {
		return
	} // Longe
	fmt.Println("🗺️  Dados de GPS simulados carregados no Redis!")

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		panic(err)
	}
	grpcServer := grpc.NewServer()

	pb.RegisterFleetServiceServer(grpcServer, fleetService)

	fmt.Println("Fleet gRPC Server running on port 50051")
	if err := grpcServer.Serve(lis); err != nil {
		panic(err)
	}

}
