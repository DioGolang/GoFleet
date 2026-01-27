package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
	"github.com/DioGolang/GoFleet/internal/application/usecase/order"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	"github.com/DioGolang/GoFleet/pkg/logger"
	carrier "github.com/DioGolang/GoFleet/pkg/otel"
	"github.com/sony/gobreaker"

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

func (c *Consumer) Start(queueName string, handler MessageHandler) error {
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

			err := handler(ctx, d.Body)

			if err != nil {
				if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, context.DeadlineExceeded) {
					c.Logger.Warn(ctx, "Resilience trigger: retrying message", logger.WithError(err))
					d.Nack(false, true)
				} else {
					c.Logger.Error(ctx, "Fatal handler error: discarding message", logger.WithError(err))
					d.Nack(false, false)
				}
			}
			span.End()
		}
	}()

	c.Logger.Info(context.Background(), "[*] Waiting for messages. To exit press CTRL+C", logger.String("queue", queueName))
	<-forever

	return nil
}

func (c *Consumer) setupTopology(ch *amqp.Channel, queueName string) error {
	dlxName := "dlx_exchange"
	err := ch.ExchangeDeclare(
		dlxName,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare dlx: %w", err)
	}

	waitQueueName := queueName + ".wait"
	waitQueueArgs := amqp.Table{
		"x-dead-letter-exchange":    "amq.direct",
		"x-dead-letter-routing-key": queueName,
		"x-message-ttl":             30000, // 30s(em ms)
	}
	_, err = ch.QueueDeclare(
		waitQueueName,
		true,
		false,
		false,
		false,
		waitQueueArgs,
	)
	if err != nil {
		return fmt.Errorf("failed to declare wait queue: %w", err)
	}

	err = ch.QueueBind(
		waitQueueName, queueName, dlxName, false, nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind wait queue: %w", err)
	}

	mainQueueArgs := amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": queueName,
	}
	_, err = ch.QueueDeclare(
		queueName, true, false, false, false, mainQueueArgs,
	)
	if err != nil {
		return fmt.Errorf("failed to declare main queue: %w", err)
	}

	err = ch.QueueBind(queueName, queueName, "amq.direct", false, nil)
	if err != nil {
		return err
	}
	return nil
}

func (c *Consumer) ProcessOrder(ctx context.Context, msg []byte) error {
	var orderDto order.CreateInput
	if err := json.Unmarshal(msg, &orderDto); err != nil {
		c.Logger.Error(ctx, "invalid json", logger.WithError(err))
		return fmt.Errorf("invalid json: %w", err)
	}

	req := &pb.SearchDriverRequest{OrderId: orderDto.ID}
	res, err := c.GrpcClient.SearchDriver(ctx, req)
	if err != nil {
		c.Logger.Error(ctx, "grpc search driver failed", logger.WithError(err))
		return fmt.Errorf("grpc search driver failed: %w", err)
	}

	err = c.OrderRepository.UpdateStatus(ctx, orderDto.ID, "DISPATCHED", res.DriverId)
	if err != nil {
		c.Logger.Error(ctx, "db update failed", logger.WithError(err))
		return fmt.Errorf("db update failed: %w", err)
	}
	return nil
}
