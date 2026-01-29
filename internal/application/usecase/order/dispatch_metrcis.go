package order

import (
	"context"
	"time"

	"github.com/DioGolang/GoFleet/pkg/metrics"
)

type DispatchOrderMetricsDecorator struct {
	Next    DispatchUseCase
	Metrics metrics.Metrics
}

func (d DispatchOrderMetricsDecorator) Execute(ctx context.Context, input DispatchInput) error {
	start := time.Now()
	err := d.Next.Execute(ctx, input)
	d.Metrics.RecordUseCaseExecution("DispatchOrder", err == nil, time.Since(start))
	return err
}
