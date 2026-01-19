package main

import (
	"github.com/DioGolang/GoFleet/configs"
	"github.com/DioGolang/GoFleet/internal/infra/event"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	config, err := configs.LoadConfig(".")
	if err != nil {
		panic(err)
	}
	uri := "amqp://guest:guest@localhost:" + config.AMQPort + "/"
	conn, err := amqp.Dial(uri)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	grpcConn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer grpcConn.Close()

	grpcClient := pb.NewFleetServiceClient(grpcConn)

	consumer := event.NewConsumer(conn, grpcClient)

	err = consumer.Start("orders.created")
	if err != nil {
		panic(err)
	}
}
