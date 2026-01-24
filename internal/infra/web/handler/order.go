package web

import (
	"encoding/json"
	dto2 "github.com/DioGolang/GoFleet/internal/application/usecase/order"
	"net/http"
)

type OrderHandler struct {
	EventService       any
	CreateOrderUseCase *dto2.CreateUseCaseImpl
}

func NewOrderHandler(uc *dto2.CreateUseCaseImpl) *OrderHandler {
	return &OrderHandler{
		CreateOrderUseCase: uc,
	}
}

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var dto dto2.CreateInput

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
