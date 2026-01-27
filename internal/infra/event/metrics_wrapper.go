package event

import (
	"context"
	"errors"
	"time"

	"github.com/DioGolang/GoFleet/pkg/metrics"
	"github.com/sony/gobreaker"
)

func WrapResilientConsumer(
	m metrics.Metrics,
	handlerName string,
	timeout time.Duration,
	cb *gobreaker.CircuitBreaker,
	next MessageHandler,
) MessageHandler {
	return func(ctx context.Context, msg []byte) error {
		start := time.Now()

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		_, err := cb.Execute(func() (interface{}, error) {
			return nil, next(ctx, msg)
		})

		if errors.Is(err, gobreaker.ErrOpenState) {
			m.RecordUseCaseExecution(handlerName, false, time.Since(start))
			return err
		}

		duration := time.Since(start)
		success := err == nil
		m.RecordUseCaseExecution(handlerName, success, duration)

		return err
	}
}
