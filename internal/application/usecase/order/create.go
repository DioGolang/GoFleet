package usecase

import (
	"context"
	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
	"github.com/DioGolang/GoFleet/internal/application/usecase/order"
	"github.com/DioGolang/GoFleet/internal/domain/entity"
	"github.com/DioGolang/GoFleet/pkg/events"
)

type CreateOrderUseCase struct {
	OrderRepository outbound.OrderRepository
	OrderCreated    events.Event
	EventDispatcher events.EventDispatcher
}

func NewCreateOrderUseCase(orderRepository outbound.OrderRepository, created events.Event, dispatcher events.EventDispatcher) *CreateOrderUseCase {
	return &CreateOrderUseCase{
		OrderRepository: orderRepository,
		OrderCreated:    created,
		EventDispatcher: dispatcher,
	}
}

func (uc *CreateOrderUseCase) Execute(ctx context.Context, input order.CreateOrderInput) (order.CreateOrderOutput, error) {
	order, err := entity.NewOrder(input.ID, input.Price, input.Tax)
	if err != nil {
		return order.CreateOrderOutput{}, err
	}
	err = uc.OrderRepository.Save(order)
	if err != nil {
		return order.CreateOrderOutput{}, err
	}

	output := order.CreateOrderOutput{
		ID:         order.ID(),
		FinalPrice: order.FinalPrice(),
	}
	uc.OrderCreated.SetPayload(output)
	err = uc.EventDispatcher.Dispatch(ctx, uc.OrderCreated)
	if err != nil {
		return order.CreateOrderOutput{}, err
	}
	return output, nil
}
