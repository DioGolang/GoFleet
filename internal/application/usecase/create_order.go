package usecase

import (
	"github.com/DioGolang/GoFleet/internal/application/dto"
	"github.com/DioGolang/GoFleet/internal/application/port"
	"github.com/DioGolang/GoFleet/internal/domain/entity"
	"github.com/DioGolang/GoFleet/pkg/events"
)

type CreateOrderUseCase struct {
	OrderRepository port.OrderRepository
	OrderCreated    events.Event
	EventDispatcher events.EventDispatcher
}

func NewCreateOrderUseCase(orderRepository port.OrderRepository, created events.Event, dispatcher events.EventDispatcher) *CreateOrderUseCase {
	return &CreateOrderUseCase{
		OrderRepository: orderRepository,
		OrderCreated:    created,
		EventDispatcher: dispatcher,
	}
}

func (uc *CreateOrderUseCase) Execute(input dto.CreateOrderInput) (dto.CreateOrderOutput, error) {
	order, err := entity.NewOrder(input.ID, input.Price, input.Tax)
	if err != nil {
		return dto.CreateOrderOutput{}, err
	}
	err = uc.OrderRepository.Save(order)
	if err != nil {
		return dto.CreateOrderOutput{}, err
	}

	output := dto.CreateOrderOutput{
		ID:         order.ID(),
		FinalPrice: order.FinalPrice(),
	}
	uc.OrderCreated.SetPayload(output)
	err = uc.EventDispatcher.Dispatch(uc.OrderCreated)
	if err != nil {
		return dto.CreateOrderOutput{}, err
	}
	return output, nil
}
