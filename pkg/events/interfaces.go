package events

import (
	"context"
	"time"
)

type Event interface {
	GetName() string
	GetDateTime() time.Time
	GetPayload() interface{}
	SetPayload(payload interface{})
}

type EventDispatcher interface {
	Register(eventName string, handler EventHandler) error
	Dispatch(ctx context.Context, event Event) error
	Remove(eventName string, handler EventHandler) error
	Has(eventName string, handler EventHandler) bool
	Clear()
}

type EventHandler interface {
	Handler(event Event)
}
