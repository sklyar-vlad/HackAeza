// Package model owns risk model settings and their updates.
package model

import (
	"context"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

// Candidate contains settings merged and validated by the model service.
// BaseVersion must still be current when accepted, even if the client omitted expectedVersion.
type Candidate struct {
	BaseVersion int64
	Settings    entity.ModelSettings
}

type Repository interface {
	jobs.Receipts

	Published(ctx context.Context) (entity.ModelSettingsResponse, error)
	// AcceptSettings atomically persists candidate, job and idempotency receipt.
	// A matching replay wins over current version/busy checks; a fresh stale candidate conflicts.
	// Published settings stay unchanged until calculation publication.
	AcceptSettings(ctx context.Context, candidate Candidate, submission jobs.Submission) (jobs.Accepted, error)
}
