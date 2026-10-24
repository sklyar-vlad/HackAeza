// Package risk defines the pure calculation boundary. It has no HTTP, SQL or cache access.
package risk

import (
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

// Input includes all services of one account, so its balance is not spent independently per service.
// Transactions only refer to these services. Slices and settings are read-only during a run.
type Input struct {
	Account      entity.Account
	Services     []entity.Service
	Transactions []entity.Transaction
}

// Engine implementations must be safe for concurrent calls and deterministic for the same inputs.
// asOf is the coordinator's single UTC reference time, never time.Now inside the engine.
// Missing data is an insufficient_data result, not an infrastructure error.
type Engine interface {
	Calculate(input Input, settings entity.ModelSettings, asOf time.Time) (entity.CustomerDetail, error)
}
