package event

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/DioGolang/GoFleet/internal/application/dto"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"time"
)

type Consumer struct {
	Conn       *amqp.Connection
	GrpcClient pb.FleetServiceClient
}

func NewConsumer(conn *amqp.Connection, grpcClient pb.FleetServiceClient) *Consumer {
	return &Consumer{
		Conn:       conn,
		GrpcClient: grpcClient,
	}
}

func (c *Consumer) Start(queueName string) error {
	ch, err := c.Conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	msgs, err := ch.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			fmt.Printf("I received a message: %s\n", d.Body)

			var orderDTO dto.CreateOrderOutput
			err := json.Unmarshal(d.Body, &orderDTO)
			if err != nil {
				log.Printf("Erro ao fazer parse do JSON: %v", err)
				continue
			}

			fmt.Printf("Seeking driver for order %s...\n", orderDTO.ID)
			time.Sleep(2 * time.Second) // Simula latência (cálculo de rota)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()

			res, err := c.GrpcClient.SearchDriver(ctx, &pb.SearchDriverRequest{
				OrderId: orderDTO.ID,
			})

			if err != nil {
				log.Printf("❌ Erro ao buscar motorista: %v", err)
				d.Ack(false) // Ou d.Nack() para tentar depois
				continue
			}

			fmt.Printf("✅ Motorista encontrado: %s (%s) \n", res.Name, res.DriverId)

			// 3. Acknowledge (Avisar ao RabbitMQ que pode apagar a mensagem)
			d.Ack(false)
		}
	}()

	fmt.Println(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever

	return nil
}
