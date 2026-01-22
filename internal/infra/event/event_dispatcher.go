package event

import (
	"context"
	"encoding/json"
	"github.com/DioGolang/GoFleet/pkg/events"
	carrier "github.com/DioGolang/GoFleet/pkg/otel"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"time"
)

type Dispatcher struct {
	RabbitMQChannel *amqp.Channel
}

func NewDispatcher(ch *amqp.Channel) *Dispatcher {
	return &Dispatcher{RabbitMQChannel: ch}
}

func (ed *Dispatcher) Dispatch(ctx context.Context, event events.Event) error {
	headers := make(amqp.Table)
	otel.GetTextMapPropagator().Inject(ctx, carrier.AMQPHeadersCarrier(headers))

	payload, err := json.Marshal(event.GetPayload())
	if err != nil {
		return err
	}

	err = ed.RabbitMQChannel.PublishWithContext(
		ctx,
		"amq.direct",
		"orders.created",
		false,
		false,
		amqp.Publishing{
			Headers:     headers,
			ContentType: "application/json",
			Timestamp:   time.Now(),
			Body:        payload,
		})
	return err
}

func (ed *Dispatcher) Register(eventName string, handler events.EventHandler) error { return nil }
func (ed *Dispatcher) Remove(eventName string, handler events.EventHandler) error   { return nil }
func (ed *Dispatcher) Has(eventName string, handler events.EventHandler) bool       { return false }
func (ed *Dispatcher) Clear()                                                       {}
