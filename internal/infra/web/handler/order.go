package handler

import (
	"encoding/json"
	"github.com/DioGolang/GoFleet/internal/application/usecase/order"
	"net/http"
)

type Order struct {
	EventService       any
	CreateOrderUseCase order.CreateUseCase
}

func NewOrderHandler(uc order.CreateUseCase) *Order {
	return &Order{
		CreateOrderUseCase: uc,
	}
}

func (h *Order) Create(w http.ResponseWriter, r *http.Request) {
	var dto order.CreateInput

	err := json.NewDecoder(r.Body).Decode(&dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	output, err := h.CreateOrderUseCase.Execute(r.Context(), dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(output)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
