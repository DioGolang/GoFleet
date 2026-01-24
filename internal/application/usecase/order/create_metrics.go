package order

import (
	"context"
	"github.com/DioGolang/GoFleet/pkg/metrics"
	"time"
)

type CreateOrderMetricsDecorator struct {
	Next    CreateUseCase
	Metrics metrics.Metrics
}

func (d *CreateOrderMetricsDecorator) Execute(ctx context.Context, input CreateInput) (CreateOutput, error) {
	start := time.Now()
	output, err := d.Next.Execute(ctx, input)
	d.Metrics.RecordUseCaseExecution("CreateOrder", err == nil, time.Since(start))
	return output, err
}
