package usecase

import (
	"github.com/DioGolang/GoFleet/internal/application/dto"
	"github.com/DioGolang/GoFleet/internal/application/port"
	"github.com/DioGolang/GoFleet/internal/domain/entity"
)

type CreateOrderUseCase struct {
	OrderRepository port.OrderRepository
}

func NewCreateOrderUseCase(orderRepository port.OrderRepository) *CreateOrderUseCase {
	return &CreateOrderUseCase{
		OrderRepository: orderRepository,
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

	return output, nil
}
