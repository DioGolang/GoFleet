package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hellofresh/health-go/v5"
)

type healthOptions struct {
	componentName string
	version       string
	checks        []*health.Config
}

type HealthOption func(*healthOptions)

func WithName(name, version string) HealthOption {
	return func(o *healthOptions) {
		o.componentName = name
		o.version = version
	}
}

func WithCheck(name string, timeout time.Duration, checkFunc func(context.Context) error) HealthOption {
	return func(o *healthOptions) {
		if checkFunc == nil {
			fmt.Printf("WARN: Check function for '%s' is nil. Skipping.\n", name)
			return
		}

		o.checks = append(o.checks, &health.Config{
			Name:      name,
			Timeout:   timeout,
			SkipOnErr: false,
			Check:     checkFunc,
		})
	}
}

func WithPostgres(checkFunc func(context.Context) error) HealthOption {
	return WithCheck("postgres", 2*time.Second, checkFunc)
}

func WithRedis(checkFunc func(context.Context) error) HealthOption {
	return WithCheck("redis", 1*time.Second, checkFunc)
}

func WithRabbitMQ(checkFunc func(context.Context) error) HealthOption {
	return WithCheck("rabbitmq", 1*time.Second, checkFunc)
}

func NewHealthHandler(opts ...HealthOption) (http.Handler, error) {
	options := &healthOptions{
		componentName: "default-service",
		version:       "0.0.0",
		checks:        make([]*health.Config, 0),
	}

	for _, opt := range opts {
		opt(options)
	}

	h, err := health.New(health.WithComponent(health.Component{
		Name:    options.componentName,
		Version: options.version,
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to init health container: %w", err)
	}

	for _, check := range options.checks {
		if err := h.Register(*check); err != nil {
			return nil, fmt.Errorf("failed to register check '%s': %w", check.Name, err)
		}
	}

	return h.Handler(), nil
}
