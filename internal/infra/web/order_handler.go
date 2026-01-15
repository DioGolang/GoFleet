package web

import (
	"encoding/json"
	dto2 "github.com/DioGolang/GoFleet/internal/application/dto"
	"github.com/DioGolang/GoFleet/internal/application/usecase"
	"net/http"
)

type OrderHandler struct {
	EventService       any
	CreateOrderUseCase *usecase.CreateOrderUseCase
}

func NewOrderHandler(uc *usecase.CreateOrderUseCase) *OrderHandler {
	return &OrderHandler{
		CreateOrderUseCase: uc,
	}
}

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var dto dto2.CreateOrderInput

	err := json.NewDecoder(r.Body).Decode(&dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	output, err := h.CreateOrderUseCase.Execute(dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(output)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
