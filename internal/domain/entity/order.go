package entity

import "errors"

var (
	ErrPriceIsRequired = errors.New("price is required")
	ErrPriceMustBePos  = errors.New("price must be greater than zero")
	ErrTaxMustBePos    = errors.New("tax must be greater than or equal to zero")
)

type Order struct {
	id         string
	price      float64
	tax        float64
	finalPrice float64
	state      OrderState
	driverID   string
}

func NewOrder(id string, price float64, tax float64) (*Order, error) {
	order := &Order{
		id:    id,
		price: price,
		tax:   tax,
		state: &PendingState{},
	}

	err := order.Validate()
	if err != nil {
		return nil, err
	}

	err = order.CalculateFinalPrice()
	if err != nil {
		return nil, err
	}

	return order, nil
}

func (o *Order) Validate() error {
	if o.id == "" {
		return ErrIDIsRequired
	}
	if o.price <= 0 {
		return ErrPriceIsRequired
	}
	if o.price <= 0 {
		return ErrPriceMustBePos
	}
	if o.tax < 0 {
		return ErrTaxMustBePos
	}
	return nil
}

func (o *Order) CalculateFinalPrice() error {
	o.finalPrice = o.price + o.tax
	return nil
}

func (o *Order) TransitionTo(newState OrderState) {
	o.state = newState
}

func (o *Order) ID() string {
	return o.id
}

func (o *Order) Price() float64 {
	return o.price
}

func (o *Order) Tax() float64 {
	return o.tax
}

func (o *Order) FinalPrice() float64 {
	return o.finalPrice
}

func (o *Order) DriverID() string {
	return o.driverID
}

func (o *Order) Dispatch(driverID string) error {
	return o.state.Dispatch(o, driverID)
}

func (o *Order) Deliver() error {
	return o.state.Deliver(o)
}

func (o *Order) Cancel() error {
	return o.state.Cancel(o)
}

func (o *Order) StatusName() string {
	return o.state.Name()
}

func ParseState(statusName string) (OrderState, error) {
	switch statusName {
	case "PENDING":
		return &PendingState{}, nil
	case "DISPATCHED":
		return &DispatchedState{}, nil
	case "DELIVERED":
		return &DeliveredState{}, nil
	case "CANCELLED":
		return &CancelledState{}, nil
	default:
		return nil, errors.New("unknown state")
	}
}
