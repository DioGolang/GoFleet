package outbound

import (
	"context"

	"github.com/DioGolang/GoFleet/internal/domain/entity"
)

type OrderRepository interface {
	Save(ctx context.Context, order *entity.Order) error
	SaveOutboxEvent(ctx context.Context, eventID, aggID, eventType string, eventVersion int32, payload []byte, topic string) error
	FindByID(ctx context.Context, id string) (*entity.Order, error)
	UpdateStatus(ctx context.Context, id string, status string, driverID string) error
}
