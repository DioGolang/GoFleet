package entity

type PendingState struct{}

func (s *PendingState) Name() string { return "PENDING" }

func (s *PendingState) Dispatch(o *Order, driverID string) error {
	o.driverID = driverID
	o.TransitionTo(&DispatchedState{})
	return nil
}

func (s *PendingState) Deliver(o *Order) error {
	return ErrInvalidStateTransition
}

func (s *PendingState) Cancel(o *Order) error {
	o.TransitionTo(&CancelledState{})
	return nil
}

type DispatchedState struct{}

func (s *DispatchedState) Name() string { return "DISPATCHED" }

func (s *DispatchedState) Dispatch(o *Order, driverID string) error {
	return ErrInvalidStateTransition
}

func (s *DispatchedState) Deliver(o *Order) error {
	o.TransitionTo(&DeliveredState{})
	return nil
}

func (s *DispatchedState) Cancel(o *Order) error {
	o.TransitionTo(&CancelledState{})
	return nil
}

type DeliveredState struct{}

func (s *DeliveredState) Name() string { return "DELIVERED" }

func (s *DeliveredState) Dispatch(o *Order, driverID string) error { return ErrInvalidStateTransition }
func (s *DeliveredState) Deliver(o *Order) error                   { return ErrInvalidStateTransition }
func (s *DeliveredState) Cancel(o *Order) error                    { return ErrInvalidStateTransition }

type CancelledState struct{}

func (s *CancelledState) Name() string                             { return "CANCELLED" }
func (s *CancelledState) Dispatch(o *Order, driverID string) error { return ErrInvalidStateTransition }
func (s *CancelledState) Deliver(o *Order) error                   { return ErrInvalidStateTransition }
func (s *CancelledState) Cancel(o *Order) error                    { return ErrInvalidStateTransition }
