package main

import (
	"database/sql"
	"fmt"
	"github.com/DioGolang/GoFleet/configs"
	"github.com/DioGolang/GoFleet/internal/infra/database"
	"github.com/DioGolang/GoFleet/internal/infra/event"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	config, err := configs.LoadConfig(".")
	if err != nil {
		panic(err)
	}
	// RabbitMQ
	rabbitURL := fmt.Sprintf("amqp://guest:guest@%s:%s/", config.RabbitMQHost, config.AMQPort)
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	//Postgres
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)
	db, err := sql.Open(config.DBDriver, dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	repository := database.NewOrderRepository(db)

	// gRPC
	grpcURL := fmt.Sprintf("%s:%s", config.FleetHost, config.FleetPort)
	grpcConn, err := grpc.NewClient(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer grpcConn.Close()

	grpcClient := pb.NewFleetServiceClient(grpcConn)

	consumer := event.NewConsumer(conn, grpcClient, repository)

	err = consumer.Start("orders.created")
	if err != nil {
		panic(err)
	}
}
