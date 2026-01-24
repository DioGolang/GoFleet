package event

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
	"github.com/DioGolang/GoFleet/internal/application/usecase/order"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	carrier "github.com/DioGolang/GoFleet/pkg/otel"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Consumer struct {
	Conn            *amqp.Connection
	GrpcClient      pb.FleetServiceClient
	OrderRepository outbound.OrderRepository
}

func NewConsumer(conn *amqp.Connection, grpcClient pb.FleetServiceClient, repo outbound.OrderRepository) *Consumer {
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
			fmt.Printf("📦 I received message. Processing...\n")
			amqpCarrier := carrier.AMQPHeadersCarrier(d.Headers)
			ctx := context.Background()
			ctx = otel.GetTextMapPropagator().Extract(ctx, amqpCarrier)

			tracer := otel.GetTracerProvider().Tracer("worker-tracer")
			ctx, span := tracer.Start(ctx, "ProcessOrder", trace.WithAttributes(
				attribute.String("queue.name", queueName),
			))

			var orderDTO order.CreateOutput
			if err := json.Unmarshal(d.Body, &orderDTO); err != nil {
				log.Printf("Erro parse JSON: %v", err)
				span.RecordError(err)
				span.End()
				d.Nack(false, false)
				continue
			}

			span.SetAttributes(attribute.String("order.id", orderDTO.ID))
			ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
			res, err := c.GrpcClient.SearchDriver(ctxTimeout, &pb.SearchDriverRequest{OrderId: orderDTO.ID})
			cancel()

			if err != nil {
				log.Printf("❌ No drivers found: %v. Trying again later...", err)
				span.RecordError(err)
				span.End()
				d.Nack(false, true)
				continue
			}

			fmt.Printf("Driver found: %s. Updating bank...\n", res.Name)
			err = c.OrderRepository.UpdateStatus(ctx, orderDTO.ID, "DISPATCHED", res.DriverId)
			if err != nil {
				log.Printf("Error saving to bank: %v", err)
				span.RecordError(err)
				span.End()
				d.Nack(false, true)
				continue
			}

			d.Ack(false)
			fmt.Printf("Order %s dispatched successfully!\n", orderDTO.ID)
			span.End()
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
