package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

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

func (r *OrderRepositoryImpl) FindByID(ctx context.Context, id string) (*entity.Order, error) {
	model, err := r.GetOrder(ctx, id)
	if err != nil {
		return nil, err
	}

	price, err := strconv.ParseFloat(model.Price, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse price for order %s: %w", id, err)
	}

	tax, err := strconv.ParseFloat(model.Tax, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tax for order %s: %w", id, err)
	}

	finalPrice, err := strconv.ParseFloat(model.FinalPrice, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse final_price for order %s: %w", id, err)
	}

	driverID := ""
	if model.DriverID.Valid {
		driverID = model.DriverID.String
	}

	return entity.Restore(
		model.ID,
		price,
		tax,
		finalPrice,
		model.Status,
		driverID,
	)
}
