package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

type Submission struct {
	ActorID        string
	RequestID      string
	IdempotencyKey string
	PayloadHash    [32]byte
}

type Accepted struct {
	JobID       entity.JobID
	CreatedAt   time.Time
	Calculation entity.Calculation
}

type Receipts interface {
	Lookup(ctx context.Context, kind entity.JobKind, submission Submission) (*Accepted, error)
}

type Creator interface {
	Receipts
	LookupForUpdate(ctx context.Context, tx pgx.Tx, kind entity.JobKind, submission Submission) (*Accepted, error)
	Create(ctx context.Context, tx pgx.Tx, kind entity.JobKind, candidateVersion int64, submission Submission) (Accepted, error)
}

type Lease struct {
	JobID entity.JobID
	Token string
}

type Claim struct {
	Lease     Lease
	Job       entity.CalculationJob
	ExpiresAt time.Time
	RequestID string
}

type Reader interface {
	Get(ctx context.Context, actorID string, id entity.JobID) (entity.CalculationJob, error)
}

type Queue interface {
	Claim(ctx context.Context, workerID string, leaseDuration time.Duration) (*Claim, error)
	Renew(ctx context.Context, lease Lease, leaseDuration time.Duration) error
	SetProgress(ctx context.Context, lease Lease, progress entity.JobProgress) error
	Retry(ctx context.Context, lease Lease, notBefore time.Time) error
	Fail(ctx context.Context, lease Lease, failure entity.JobError) error
}
