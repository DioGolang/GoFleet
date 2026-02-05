package event

import (
	"context"
	"math"
	"math/rand/v2"
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
				const maxBackoff = 30 * time.Second

				floatWait := float64(baseWait) * math.Pow(2, float64(attempt))
				calculatedWait := time.Duration(floatWait)

				if calculatedWait > maxBackoff {
					calculatedWait = maxBackoff
				}

				jitterWait := rand.N(calculatedWait + 1)

				if jitterWait < 100*time.Millisecond {
					jitterWait = 100 * time.Millisecond
				}

				log.Warn(ctx, "Transient failure, retrying with jitter...",
					logger.String("handler", handlerName),
					logger.Int("attempt", attempt+1),
					logger.String("wait", jitterWait.String()),
					logger.String("cap_wait", calculatedWait.String()),
					logger.WithError(err),
				)

				timer := time.NewTimer(jitterWait)
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
