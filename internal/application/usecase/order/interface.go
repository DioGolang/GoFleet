package order

import (
	"context"
)

type CreateUseCase interface {
	Execute(ctx context.Context, input CreateInput) (CreateOutput, error)
}
