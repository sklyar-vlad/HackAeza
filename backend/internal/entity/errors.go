package entity

import "errors"

var (
	ErrNotFound            = errors.New("resource not found or inaccessible")
	ErrVersionConflict     = errors.New("published version changed")
	ErrJobInProgress       = errors.New("global update already in progress")
	ErrIdempotencyConflict = errors.New("idempotency key reused with different content")
	ErrLeaseLost           = errors.New("job lease expired or replaced")
)
