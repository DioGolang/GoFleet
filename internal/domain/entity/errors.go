package entity

import "errors"

var (
	ErrIDIsRequired = errors.New("id is required")
	ErrInvalidID    = errors.New("invalid id format")
)
