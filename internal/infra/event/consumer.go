package event

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/DioGolang/GoFleet/internal/application/dto"
	"github.com/DioGolang/GoFleet/internal/application/port"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"time"
)

type Consumer struct {
	Conn            *amqp.Connection
	GrpcClient      pb.FleetServiceClient
	OrderRepository port.OrderRepository
}

func NewConsumer(conn *amqp.Connection, grpcClient pb.FleetServiceClient, repo port.OrderRepository) *Consumer {
	return &Consumer{
		Conn:            conn,
		GrpcClient:      grpcClient,
		OrderRepository: repo,
	}
}

func (c *Consumer) Start(queueName string) error {
	ch, err := c.Conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := c.setupTopology(ch, queueName); err != nil {
		return fmt.Errorf("error when configuring topology: %w", err)
	}

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
			fmt.Printf("📦 Recebi mensagem. Processando...\n")

			var orderDTO dto.CreateOrderOutput
			if err := json.Unmarshal(d.Body, &orderDTO); err != nil {
				log.Printf("Erro parse JSON: %v", err)
				d.Nack(false, false)
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			res, err := c.GrpcClient.SearchDriver(ctx, &pb.SearchDriverRequest{OrderId: orderDTO.ID})
			cancel()

			if err != nil {
				log.Printf("❌ No drivers found: %v. Trying again later...", err)
				d.Nack(false, true)
				continue
			}

			fmt.Printf("Driver found: %s. Updating bank...\n", res.Name)

			err = c.OrderRepository.UpdateStatus(context.Background(), orderDTO.ID, "DISPATCHED", res.DriverId)
			if err != nil {
				log.Printf("Error saving to bank: %v", err)
				d.Nack(false, true)
				continue
			}

			// 3. Sucesso total
			d.Ack(false)
			fmt.Printf("Order %s dispatched successfully!\n", orderDTO.ID)
		}
	}()

	fmt.Println(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever

	return nil
}

func (c *Consumer) setupTopology(ch *amqp.Channel, queueName string) error {
	_, err := ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}
