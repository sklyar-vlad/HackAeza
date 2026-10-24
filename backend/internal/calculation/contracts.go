// Package calculation coordinates preparation, bounded parallel calculation and publication.
package calculation

import (
	"context"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
	"github.com/sklyar-vlad/HackAeza/backend/internal/risk"
)

// Run pins immutable source/settings versions for the whole attempt.
// Target.CalculatedAt is set to Prepare's asOf, shared by every Engine call.
// Base is checked again at publication; Target is not yet visible to readers.
type Run struct {
	Lease    jobs.Lease
	Base     entity.Calculation
	Target   entity.Calculation
	Settings entity.ModelSettings
	Counts   entity.SourceCounts
}

type Repository interface {
	// Prepare checks the lease and pins the candidate source/settings without publishing them.
	Prepare(ctx context.Context, lease jobs.Lease, asOf time.Time) (Run, error)
	// Customers returns a bounded batch ordered by account ID > afterID, initially zero.
	// Each input includes ALL of that customer's services and transactions from the pinned version.
	// No rows means EOF. Implementations must avoid per-customer SQL queries.
	Customers(ctx context.Context, run Run, afterID entity.ID, limit int) ([]risk.Input, error)
	// SaveResults writes only staging data; repeated batches replace the same generation/customer.
	SaveResults(ctx context.Context, run Run, results []entity.CustomerDetail) error
	// Publish checks lease, base versions and completeness, then commits source/settings switch,
	// all results, summary and succeeded job/result in ONE transaction. Failed attempts stay invisible.
	// A stale lease returns ErrLeaseLost; a stale base returns ErrVersionConflict.
	Publish(ctx context.Context, run Run, summary entity.AnalyticsSummary, warnings []entity.ImportWarning) (entity.CalculationJob, error)
}
