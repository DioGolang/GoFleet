package order

import (
	"context"
	"fmt"

	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
)

type DispatchUseCaseImpl struct {
	Repo outbound.OrderRepository
}

func NewDispatchUseCase(repo outbound.OrderRepository) *DispatchUseCaseImpl {
	return &DispatchUseCaseImpl{Repo: repo}
}

func (uc *DispatchUseCaseImpl) Execute(ctx context.Context, input DispatchInput) error {

	order, err := uc.Repo.FindByID(ctx, input.OrderID)
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	if err := order.Dispatch(input.DriverID); err != nil {
		return fmt.Errorf("domain rule violation: %w", err)
	}

	if err := uc.Repo.UpdateStatus(ctx, order.ID(), order.StatusName(), order.DriverID()); err != nil {
		return fmt.Errorf("failed to save order: %w", err)
	}

	return nil
}
