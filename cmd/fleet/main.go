package fleet

import (
	"fmt"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/service"
	"google.golang.org/grpc"
	"net"
)

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		panic(err)
	}
	grpcServer := grpc.NewServer()

	fleetService := service.NewFleetService()
	pb.RegisterFleetServiceServer(grpcServer, fleetService)

	fmt.Println("Fleet gRPC Server running on port 50051")
	if err := grpcServer.Serve(lis); err != nil {
		panic(err)
	}

}
