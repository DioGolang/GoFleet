package event

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/DioGolang/GoFleet/pkg/logger"
)

type RedisIdempotencyStore interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
	Del(ctx context.Context, key string) error
}

func WrapIdempotency(
	log logger.Logger,
	store RedisIdempotencyStore,
	handlerName string,
	ttl time.Duration,
	next MessageHandler,
) MessageHandler {

	return func(ctx context.Context, msg []byte, headers map[string]interface{}) error {

		var eventID string

		if v, ok := headers["x-event-id"]; ok {
			eventID = fmt.Sprintf("%v", v)
		}

		if eventID == "" {
			hash := sha256.Sum256(msg)
			eventID = fmt.Sprintf("hash:%x", hash)
		}

		key := fmt.Sprintf("dedup:%s:%s", handlerName, eventID)

		saved, err := store.SetNX(ctx, key, "processing", ttl)

		if err != nil {
			// CRÍTICO: Se o Redis cair, o que fazemos?
			// Opção A (Fail Open): Processa e corre risco de duplicação.
			// Opção B (Fail Closed): Retorna erro e para o consumo (Segurança).
			log.Error(ctx, "Redis unavailable for idempotency check",
				logger.WithError(err))

			return fmt.Errorf("idempotency store unavailable: %w", err)
		}

		if !saved {
			// DUPLICATA DETECTADA
			log.Info(ctx, "Duplicate event dropped by Idempotency Guard",
				logger.String("handler", handlerName),
				logger.String("event_id", eventID),
			)
			return nil // Ack silencioso (Sucesso falso)
		}

		// 3. Execução do Handler Real
		err = next(ctx, msg, headers)

		// 4. Tratamento de Falha do Negócio
		if err != nil {
			log.Warn(ctx, "Handler logic failed, releasing lock for retry",
				logger.String("key", key),
				logger.WithError(err),
			)

			// Remove a chave para que o próximo Retry (da Wait Queue)
			if delErr := store.Del(ctx, key); delErr != nil {
				log.Error(ctx, "Failed to release idempotency lock (Zombie Key Risk)",
					logger.String("key", key),
					logger.WithError(delErr),
				)
			}
		}

		return err
	}
}
