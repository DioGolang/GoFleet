package entity

import "errors"

var ErrInvalidStateTransition = errors.New("invalid state transition")

type OrderState interface {
	Name() string
	Dispatch(o *Order, driverID string) error
	SendToManual(o *Order) error
	Deliver(o *Order) error
	Cancel(o *Order) error
}
