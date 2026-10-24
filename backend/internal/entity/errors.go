package entity

import "errors"

// Adapters wrap these errors; services and handlers inspect them with errors.Is.
// HTTP statuses and response formatting belong to handlers.
var (
	ErrNotFound            = errors.New("resource not found or inaccessible")
	ErrVersionConflict     = errors.New("published version changed")
	ErrJobInProgress       = errors.New("global update already in progress")
	ErrIdempotencyConflict = errors.New("idempotency key reused with different content")
	ErrLeaseLost           = errors.New("job lease expired or replaced")
)
