package event

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/DioGolang/GoFleet/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type RedisIdempotencyStore interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

func WrapIdempotency(
	log logger.Logger,
	store RedisIdempotencyStore,
	handlerName string,
	ttl time.Duration,
	next MessageHandler,
) MessageHandler {
	return func(ctx context.Context, msg []byte) error {
		hash := sha256.Sum256(msg)
		key := fmt.Sprintf("dedup:%s:%x", handlerName, hash)

		saved, err := store.SetNX(ctx, key, "processing", ttl).Result()

		if err != nil {
			log.Error(ctx, "Redis idempotency check failed (proceeding to fallback)",
				logger.String("key", key),
				logger.WithError(err),
			)
			return next(ctx, msg)
		}

		if !saved {
			log.Warn(ctx, "Duplicate event dropped by Idempotency Guard",
				logger.String("handler", handlerName),
				logger.String("key", key),
			)
			return nil
		}

		err = next(ctx, msg)

		if err != nil {
			log.Warn(ctx, "Handler failed, releasing idempotency lock to allow retry",
				logger.String("key", key),
				logger.WithError(err),
			)

			delErr := store.Del(context.Background(), key).Err()
			if delErr != nil {
				log.Error(ctx, "Failed to release idempotency lock",
					logger.String("key", key),
					logger.WithError(delErr),
				)
			}
		}
		return err
	}
}
