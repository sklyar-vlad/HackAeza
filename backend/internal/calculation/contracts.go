package calculation

import (
	"context"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
	"github.com/sklyar-vlad/HackAeza/backend/internal/risk"
)

type Run struct {
	Kind     entity.JobKind
	Lease    jobs.Lease
	Base     entity.Calculation
	Target   entity.Calculation
	Settings entity.ModelSettings
	Counts   entity.SourceCounts
	Warnings []entity.ImportWarning
}

type Repository interface {
	Prepare(ctx context.Context, lease jobs.Lease, asOf time.Time) (Run, error)
	Customers(ctx context.Context, run Run, afterID entity.ID, limit int) ([]risk.Input, error)
	SaveResults(ctx context.Context, run Run, results []entity.CustomerDetail) error
	Publish(ctx context.Context, run Run, summary entity.AnalyticsSummary, warnings []entity.ImportWarning) (entity.CalculationJob, error)
}
