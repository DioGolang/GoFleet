package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/DioGolang/GoFleet/internal/domain/entity"
)

type OrderRepositoryImpl struct {
	Db *sql.DB
	*Queries
}

func NewOrderRepository(db *sql.DB) *OrderRepositoryImpl {
	return &OrderRepositoryImpl{Db: db, Queries: New(db)}
}

func (r *OrderRepositoryImpl) Save(order *entity.Order) error {

	priceStr := fmt.Sprintf("%.2f", order.Price())
	taxStr := fmt.Sprintf("%.2f", order.Tax())
	finalPriceStr := fmt.Sprintf("%.2f", order.FinalPrice())

	err := r.CreateOrder(context.Background(), CreateOrderParams{
		ID:         order.ID(),
		Price:      priceStr,
		Tax:        taxStr,
		FinalPrice: finalPriceStr,
		Status:     order.StatusName(),
		DriverID:   sql.NullString{String: order.DriverID(), Valid: order.DriverID() != ""},
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *OrderRepositoryImpl) UpdateStatus(ctx context.Context, id string, status string, driverID string) error {
	return r.UpdateOrderStatus(ctx, UpdateOrderStatusParams{
		Status:   status,
		DriverID: sql.NullString{String: driverID, Valid: driverID != ""},
		ID:       id,
	})
}
