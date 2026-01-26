package handler

import (
	"encoding/json"
	"github.com/DioGolang/GoFleet/internal/application/usecase/order"
	"github.com/DioGolang/GoFleet/pkg/logger"
	"net/http"
)

type Order struct {
	EventService       any
	CreateOrderUseCase order.CreateUseCase
	Logger             logger.Logger
}

func NewOrderHandler(uc order.CreateUseCase, l logger.Logger) *Order {
	return &Order{
		CreateOrderUseCase: uc,
		Logger:             l,
	}
}

func (h *Order) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var dto order.CreateInput

	err := json.NewDecoder(r.Body).Decode(&dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	h.Logger.Info(ctx, "creating new order",
		logger.String("order_id", dto.ID),
		logger.String("customer_id", dto.ID),
	)

	output, err := h.CreateOrderUseCase.Execute(r.Context(), dto)
	if err != nil {
		h.Logger.Error(ctx, "order creation failed",
			logger.WithError(err),
			logger.String("order_id", dto.ID),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(output)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
