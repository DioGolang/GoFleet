package order

import (
	"context"
	"encoding/json"

	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
	"github.com/DioGolang/GoFleet/internal/domain/entity"
	"github.com/DioGolang/GoFleet/pkg/events"
	"github.com/DioGolang/GoFleet/pkg/logger"
	"github.com/google/uuid"
)

type CreateUseCaseImpl struct {
	UoW          outbound.UnitOfWork
	OrderCreated events.Event
	Logger       logger.Logger
}

func NewCreateOrderUseCase(
	uow outbound.UnitOfWork,
	created events.Event,
	log logger.Logger,
) *CreateUseCaseImpl {
	return &CreateUseCaseImpl{
		UoW:          uow,
		OrderCreated: created,
		Logger:       log,
	}
}

func (uc *CreateUseCaseImpl) Execute(ctx context.Context, input CreateInput) (CreateOutput, error) {
	uc.Logger.Info(ctx, "Starting order creation", logger.String("order_id", input.ID))

	order, err := entity.NewOrder(input.ID, input.Price, input.Tax)
	if err != nil {
		return CreateOutput{}, err
	}

	output := CreateOutput{
		ID:         order.ID(),
		FinalPrice: order.FinalPrice(),
	}
	uc.OrderCreated.SetPayload(output)
	payloadBytes, err := json.Marshal(output)
	if err != nil {
		uc.Logger.Error(ctx, "failed to marshal event payload", logger.WithError(err))
		return CreateOutput{}, err
	}

	err = uc.UoW.Do(ctx, func(provider outbound.RepositoryProvider) error {
		repo := provider.Order()

		if err := repo.Save(order); err != nil {
			return err
		}

		err = repo.SaveOutboxEvent(
			ctx,
			uuid.New().String(),
			order.ID(),
			uc.OrderCreated.GetName(),
			payloadBytes,
			"orders.created",
		)
		return err
	})
	if err != nil {
		uc.Logger.Error(ctx, "failed to execute transactional creation", logger.WithError(err))
		return CreateOutput{}, err
	}
	uc.Logger.Info(ctx, "Order created successfully (Atomic Transaction)")
	return output, nil

}
