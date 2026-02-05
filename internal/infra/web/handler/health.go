package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/hellofresh/health-go/v5"
	healthRabbit "github.com/hellofresh/health-go/v5/checks/rabbitmq"
	"github.com/redis/go-redis/v9"
)

type healthOptions struct {
	checks []*health.Config
}

type HealthOption func(*healthOptions)

func WithPostgres(db *sql.DB) HealthOption {
	return func(o *healthOptions) {
		if db == nil {
			return
		}
		o.checks = append(o.checks, &health.Config{
			Name:      "postgres",
			Timeout:   5 * time.Second,
			SkipOnErr: false,
			Check: func(ctx context.Context) error {
				return db.PingContext(ctx)
			},
		})
	}
}

func WithRedis(rdb *redis.Client) HealthOption {
	return func(o *healthOptions) {
		if rdb == nil {
			return
		}
		o.checks = append(o.checks, &health.Config{
			Name:      "redis",
			Timeout:   3 * time.Second,
			SkipOnErr: false,
			Check: func(ctx context.Context) error {
				return rdb.Ping(ctx).Err()
			},
		})
	}
}

func WithRabbitMQ(dsn string) HealthOption {
	return func(o *healthOptions) {
		if dsn == "" {
			return
		}
		o.checks = append(o.checks, &health.Config{
			Name:      "rabbitmq",
			Timeout:   3 * time.Second,
			SkipOnErr: false,
			Check: healthRabbit.New(healthRabbit.Config{
				DSN: dsn,
			}),
		})
	}
}

func NewHealthHandler(serviceName string, opts ...HealthOption) http.Handler {
	options := &healthOptions{
		checks: make([]*health.Config, 0),
	}

	for _, opt := range opts {
		opt(options)
	}

	h, _ := health.New(health.WithComponent(health.Component{
		Name:    serviceName,
		Version: "1.0.0",
	}))

	for _, check := range options.checks {
		h.Register(*check)
	}

	return h.Handler()
}
