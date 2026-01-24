package order

import (
	"context"
	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
	"github.com/DioGolang/GoFleet/internal/domain/entity"
	"github.com/DioGolang/GoFleet/pkg/events"
)

type CreateUseCaseImpl struct {
	OrderRepository outbound.OrderRepository
	OrderCreated    events.Event
	EventDispatcher events.EventDispatcher
}

func NewCreateOrderUseCase(orderRepository outbound.OrderRepository, created events.Event, dispatcher events.EventDispatcher) *CreateUseCaseImpl {
	return &CreateUseCaseImpl{
		OrderRepository: orderRepository,
		OrderCreated:    created,
		EventDispatcher: dispatcher,
	}
}

func (uc *CreateUseCaseImpl) Execute(ctx context.Context, input CreateInput) (CreateOutput, error) {
	order, err := entity.NewOrder(input.ID, input.Price, input.Tax)
	if err != nil {
		return CreateOutput{}, err
	}
	err = uc.OrderRepository.Save(order)
	if err != nil {
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
	return output, nil
}
