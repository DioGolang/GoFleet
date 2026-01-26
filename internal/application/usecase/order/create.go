package order

import (
	"context"
	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
	"github.com/DioGolang/GoFleet/internal/domain/entity"
	"github.com/DioGolang/GoFleet/pkg/events"
	"github.com/DioGolang/GoFleet/pkg/logger"
)

type CreateUseCaseImpl struct {
	OrderRepository outbound.OrderRepository
	OrderCreated    events.Event
	EventDispatcher events.EventDispatcher
	Logger          logger.Logger
}

func NewCreateOrderUseCase(
	orderRepository outbound.OrderRepository,
	created events.Event,
	dispatcher events.EventDispatcher,
	log logger.Logger,
) *CreateUseCaseImpl {
	return &CreateUseCaseImpl{
		OrderRepository: orderRepository,
		OrderCreated:    created,
		EventDispatcher: dispatcher,
		Logger:          log,
	}
}

func (uc *CreateUseCaseImpl) Execute(ctx context.Context, input CreateInput) (CreateOutput, error) {
	uc.Logger.Info(ctx, "Starting order creation", logger.String("order_id", input.ID))

	order, err := entity.NewOrder(input.ID, input.Price, input.Tax)
	if err != nil {
		return CreateOutput{}, err
	}

	err = uc.OrderRepository.Save(order)
	if err != nil {
		uc.Logger.Error(ctx, "failed to save", logger.WithError(err))
		return CreateOutput{}, err
	}

	output := CreateOutput{
		ID:         order.ID(),
		FinalPrice: order.FinalPrice(),
	}
	uc.OrderCreated.SetPayload(output)
	err = uc.EventDispatcher.Dispatch(ctx, uc.OrderCreated)
	if err != nil {
		return CreateOutput{}, err
	}
	uc.Logger.Info(ctx, "Order created successfully")
	return output, nil
}
