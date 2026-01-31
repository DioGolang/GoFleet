package event

import (
	"context"
	"encoding/json"
	"time"

	"github.com/DioGolang/GoFleet/pkg/events"
	"github.com/DioGolang/GoFleet/pkg/logger"
	carrier "github.com/DioGolang/GoFleet/pkg/otel"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
)

type Dispatcher struct {
	RabbitMQChannel *amqp.Channel
	Logger          logger.Logger
}

func NewDispatcher(ch *amqp.Channel, log logger.Logger) *Dispatcher {
	return &Dispatcher{RabbitMQChannel: ch, Logger: log}
}

func (ed *Dispatcher) Dispatch(ctx context.Context, event events.Event) error {
	headers := make(amqp.Table)
	otel.GetTextMapPropagator().Inject(ctx, carrier.AMQPHeadersCarrier(headers))

	ed.Logger.Debug(ctx, "Preparing to publish event",
		logger.String("event", event.GetName()),
	)

	payload, err := json.Marshal(event.GetPayload())
	if err != nil {
		ed.Logger.Error(ctx, "Failed to marshal event payload",
			logger.String("event", event.GetName()),
			logger.WithError(err),
		)
		return err
	}

	err = ed.RabbitMQChannel.PublishWithContext(
		ctx,
		"orders_exchange",
		"orders.created",
		false,
		false,
		amqp.Publishing{
			Headers:     headers,
			ContentType: "application/json",
			Timestamp:   time.Now(),
			Body:        payload,
		})
	if err != nil {
		ed.Logger.Error(ctx, "Failed to publish message to RabbitMQ",
			logger.String("event", event.GetName()),
			logger.WithError(err),
		)
		return err
	}

	ed.Logger.Info(ctx, "Event published to RabbitMQ",
		logger.String("event", event.GetName()),
		logger.String("exchange", "amq.direct"),
		logger.String("routing_key", "orders.created"),
	)
	return nil
}

func (ed *Dispatcher) DispatchRaw(ctx context.Context, routingKey string, payload []byte) error {
	headers := make(amqp.Table)
	otel.GetTextMapPropagator().Inject(ctx, carrier.AMQPHeadersCarrier(headers))

	ed.Logger.Debug(ctx, "Relaying event from outbox",
		logger.String("routing_key", routingKey),
	)

	err := ed.RabbitMQChannel.PublishWithContext(
		ctx,
		"orders_exchange",
		routingKey,
		false,
		false,
		amqp.Publishing{
			Headers:     headers,
			ContentType: "application/json",
			Timestamp:   time.Now(),
			Body:        payload,
		})

	if err != nil {
		ed.Logger.Error(ctx, "Failed to relay message to RabbitMQ",
			logger.String("routing_key", routingKey),
			logger.WithError(err),
		)
		return err
	}

	return nil
}

func (ed *Dispatcher) Register(eventName string, handler events.EventHandler) error { return nil }
func (ed *Dispatcher) Remove(eventName string, handler events.EventHandler) error   { return nil }
func (ed *Dispatcher) Has(eventName string, handler events.EventHandler) bool       { return false }
func (ed *Dispatcher) Clear()                                                       {}
