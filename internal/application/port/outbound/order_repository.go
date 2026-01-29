package outbound

import (
	"context"

	"github.com/DioGolang/GoFleet/internal/domain/entity"
)

type OrderRepository interface {
	Save(order *entity.Order) error
	UpdateStatus(ctx context.Context, id string, status string, driverID string) error
	FindByID(ctx context.Context, id string) (*entity.Order, error)
}
