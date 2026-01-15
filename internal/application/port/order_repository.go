package port

import "github.com/DioGolang/GoFleet/internal/domain/entity"

type OrderRepository interface {
	Save(order *entity.Order) error
}
