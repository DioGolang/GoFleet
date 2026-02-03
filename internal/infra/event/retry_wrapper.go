package event

import (
	"context"
	"math"
	"time"

	"github.com/DioGolang/GoFleet/pkg/logger"
	"github.com/DioGolang/GoFleet/pkg/metrics"
)

func WrapExponentialBackoff(
	log logger.Logger,
	metrics metrics.Metrics,
	handlerName string,
	maxRetries int,
	baseWait time.Duration,
	next MessageHandler,
) MessageHandler {
	return func(ctx context.Context, msg []byte, headers map[string]interface{}) error {
		var err error
		for attempt := 0; attempt <= maxRetries; attempt++ {
			err = next(ctx, msg, headers)
			if err == nil {
				return nil
			}
			if attempt < maxRetries {
				wait := baseWait * time.Duration(math.Pow(2, float64(attempt)))

				log.Warn(ctx, "Transient failure, retrying...",
					logger.String("handler", handlerName),
					logger.Int("attempt", attempt+1),
					logger.String("wait", wait.String()),
					logger.WithError(err),
				)

				timer := time.NewTimer(wait)
				select {
				case <-timer.C:
				case <-ctx.Done():
					if !timer.Stop() {
						<-timer.C
					}
					return ctx.Err()
				}
			}
		}

		log.Error(ctx, "Max retries reached, giving up.",
			logger.String("handler", handlerName),
			logger.WithError(err),
		)
		metrics.RecordUseCaseExecution(handlerName+"_final_failure", false, 0)
		return err
	}
}
