package order

import (
	"context"
)

type CreateUseCase interface {
	Execute(ctx context.Context, input CreateInput) (CreateOutput, error)
}

type DispatchUseCase interface {
	Execute(ctx context.Context, input DispatchInput) error
}
