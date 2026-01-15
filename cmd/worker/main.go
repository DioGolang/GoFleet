package main

import (
	"fmt"
	"github.com/DioGolang/GoFleet/configs"
	"github.com/DioGolang/GoFleet/internal/infra/event"
	amqp "github.com/rabbitmq/amqp091-go"
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

	fmt.Println("🚀 Worker iniciado")

	consumer := event.NewConsumer(conn)

	err = consumer.Start("orders.created")
	if err != nil {
		panic(err)
	}
}
