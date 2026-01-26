package event

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
	"github.com/DioGolang/GoFleet/internal/application/usecase/order"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	"github.com/DioGolang/GoFleet/pkg/logger"
	carrier "github.com/DioGolang/GoFleet/pkg/otel"
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
	Logger          logger.Logger
}

func NewConsumer(
	conn *amqp.Connection,
	grpcClient pb.FleetServiceClient,
	repo outbound.OrderRepository,
	l logger.Logger,
) *Consumer {
	return &Consumer{
		Conn:            conn,
		GrpcClient:      grpcClient,
		OrderRepository: repo,
		Logger:          l,
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
			amqpCarrier := carrier.AMQPHeadersCarrier(d.Headers)
			ctx := context.Background()
			ctx = otel.GetTextMapPropagator().Extract(ctx, amqpCarrier)

			tracer := otel.GetTracerProvider().Tracer("worker-tracer")
			ctx, span := tracer.Start(ctx, "ProcessOrder", trace.WithAttributes(
				attribute.String("queue.name", queueName),
				attribute.String("messaging.message_id", d.MessageId),
			))

			c.Logger.Info(ctx, "Received message from queue",
				logger.String("queue", queueName),
			)

			var orderDTO order.CreateOutput
			if err := json.Unmarshal(d.Body, &orderDTO); err != nil {
				c.Logger.Error(ctx, "Failed to unmarshal event body", logger.WithError(err))
				span.RecordError(err)
				span.End()
				d.Nack(false, false)
				continue
			}

			span.SetAttributes(attribute.String("order.id", orderDTO.ID))
			c.Logger.Debug(ctx, "Searching for driver...", logger.String("order_id", orderDTO.ID))
			ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
			res, err := c.GrpcClient.SearchDriver(ctxTimeout, &pb.SearchDriverRequest{OrderId: orderDTO.ID})
			cancel()

			if err != nil {
				c.Logger.Warn(ctx, "No drivers found, retrying later",
					logger.String("order_id", orderDTO.ID),
					logger.WithError(err),
				)
				span.End()
				d.Nack(false, true)
				continue
			}

			c.Logger.Info(ctx, "Driver found",
				logger.String("driver_name", res.Name),
				logger.String("driver_id", res.DriverId),
			)
			err = c.OrderRepository.UpdateStatus(ctx, orderDTO.ID, "DISPATCHED", res.DriverId)
			if err != nil {
				c.Logger.Error(ctx, "Failed to update order status in DB", logger.WithError(err))
				span.RecordError(err)
				span.End()
				d.Nack(false, true)
				continue
			}

			d.Ack(false)
			c.Logger.Info(ctx, "Order dispatched successfully",
				logger.String("order_id", orderDTO.ID),
				logger.Float64("final_price", orderDTO.FinalPrice),
			)
			span.End()
		}
	}()
	
	c.Logger.Info(context.Background(), "[*] Waiting for messages. To exit press CTRL+C", logger.String("queue", queueName))
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
