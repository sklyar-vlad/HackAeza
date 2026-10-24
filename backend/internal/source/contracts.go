// Package source owns snapshot import and source status.
package source

import (
	"context"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

// Repository is consumed by the source service; its implementation lives in repository.go.
// handler.go decodes HTTP, service.go validates and orchestrates, repository.go handles storage.
type Repository interface {
	jobs.Receipts

	// AcceptSnapshot atomically persists the candidate, job and idempotency receipt before returning.
	// Replays return the original receipt, checked before ErrJobInProgress. No published data changes.
	AcceptSnapshot(ctx context.Context, snapshot entity.SourceSnapshot, submission jobs.Submission) (jobs.Accepted, error)
	// Status reads published data and, if present, the pending global job ID.
	Status(ctx context.Context) (entity.SourceStatus, error)
}
