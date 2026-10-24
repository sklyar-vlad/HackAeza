// Package jobs owns job submission metadata and durable execution contracts.
package jobs

import (
	"context"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

// Submission identifies the authenticated actor and original request, before merging a PATCH.
// Idempotency is scoped by actor, dataset and operation; PayloadHash is the request's SHA-256 digest.
type Submission struct {
	ActorID        string
	RequestID      string
	IdempotencyKey string
	PayloadHash    [32]byte
}

// Accepted is persisted with the candidate and idempotency record in one transaction.
// Calculation is the previously published generation. The handler adds statusUrl for HTTP 202.
type Accepted struct {
	JobID       entity.JobID
	CreatedAt   time.Time
	Calculation entity.Calculation
}

// Receipts is checked after authentication/decoding and before merging settings or checking versions.
// AcceptSnapshot/AcceptSettings must repeat the lookup inside their write transaction to close races.
type Receipts interface {
	// Lookup returns nil when the key is absent, and ErrIdempotencyConflict for different content.
	// An empty IdempotencyKey disables lookup. Kind identifies the operation within this dataset.
	Lookup(ctx context.Context, kind entity.JobKind, submission Submission) (*Accepted, error)
}

// Lease is an opaque fencing token, unique for each claim attempt.
// Every worker mutation must atomically check ownership, running status and expiry.
type Lease struct {
	JobID entity.JobID
	Token string
}

type Claim struct {
	Lease     Lease
	Job       entity.CalculationJob
	ExpiresAt time.Time
	RequestID string // Original submission correlation ID for logs and JobError.
}

type Reader interface {
	// Get checks actor access to the dataset; it is not limited to the job's creator.
	// Unknown, inaccessible and expired jobs all return entity.ErrNotFound.
	Get(ctx context.Context, actorID string, id entity.JobID) (entity.CalculationJob, error)
}

// Queue does not expose Succeed: successful completion belongs to atomic result publication.
type Queue interface {
	// Claim returns nil when no runnable job exists; claims must be exclusive across workers.
	Claim(ctx context.Context, workerID string, leaseDuration time.Duration) (*Claim, error)
	Renew(ctx context.Context, lease Lease, leaseDuration time.Duration) error
	SetProgress(ctx context.Context, lease Lease, progress entity.JobProgress) error
	Retry(ctx context.Context, lease Lease, notBefore time.Time) error
	Fail(ctx context.Context, lease Lease, failure entity.JobError) error
}
