package model

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

type Postgres struct {
	pool  *pgxpool.Pool
	queue jobs.Creator
}

func NewRepository(pool *pgxpool.Pool, queue jobs.Creator) *Postgres {
	return &Postgres{pool: pool, queue: queue}
}

const publishedSQL = `
	SELECT m.version, m.updated_at, m.coefficients, m.medium_from, m.high_from,
		m.horizon_days, m.refresh_interval_seconds, m.score_formula, m.loss_formula,
		coalesce(c.version, 0), coalesce(c.source_version, 0), c.calculated_at, j.id::text
	FROM published_state p JOIN model_versions m ON m.version = p.active_model_version
	LEFT JOIN calculation_versions c ON c.version = p.active_calculation_version
	LEFT JOIN calculation_jobs j ON j.id = p.pending_job_id AND j.status IN ('queued', 'running')
	WHERE p.id = 1`

func published(row pgx.Row) (entity.ModelSettingsResponse, error) {
	var result entity.ModelSettingsResponse
	s := &result.Settings
	err := row.Scan(&s.Version, &s.UpdatedAt, &s.Coefficients, &s.Thresholds.MediumFrom, &s.Thresholds.HighFrom,
		&s.HorizonDays, &s.RefreshIntervalSeconds, &s.ScoreFormula, &s.LossFormula,
		&result.Calculation.Version, &result.Calculation.SourceVersion, &result.Calculation.CalculatedAt, &result.PendingJobID)
	if err != nil {
		return result, fmt.Errorf("read published model: %w", err)
	}
	s.UpdatedAt = s.UpdatedAt.UTC()
	result.Calculation.ModelVersion = s.Version
	if result.Calculation.CalculatedAt != nil {
		*result.Calculation.CalculatedAt = result.Calculation.CalculatedAt.UTC()
	}
	return result, nil
}

func (r *Postgres) Published(ctx context.Context) (entity.ModelSettingsResponse, error) {
	return published(r.pool.QueryRow(ctx, publishedSQL))
}

func (r *Postgres) Lookup(ctx context.Context, kind entity.JobKind, submission jobs.Submission) (*jobs.Accepted, error) {
	return r.queue.Lookup(ctx, kind, submission)
}

func (r *Postgres) AcceptSettings(ctx context.Context, candidate Candidate, submission jobs.Submission) (jobs.Accepted, error) {
	var accepted jobs.Accepted
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		previous, err := r.queue.LookupForUpdate(ctx, tx, entity.JobModelSettings, submission)
		if err != nil {
			return err
		}
		if previous != nil {
			accepted = *previous
			return nil
		}
		var version int64
		if err := tx.QueryRow(ctx, `SELECT active_model_version FROM published_state WHERE id = 1`).Scan(&version); err != nil {
			return err
		}
		if version != candidate.BaseVersion {
			return entity.ErrVersionConflict
		}
		s := candidate.Settings
		if err := tx.QueryRow(ctx, `
			INSERT INTO model_versions (coefficients, medium_from, high_from, horizon_days, refresh_interval_seconds, score_formula, loss_formula)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING version`,
			s.Coefficients, s.Thresholds.MediumFrom, s.Thresholds.HighFrom, s.HorizonDays,
			s.RefreshIntervalSeconds, s.ScoreFormula, s.LossFormula).Scan(&version); err != nil {
			return err
		}
		accepted, err = r.queue.Create(ctx, tx, entity.JobModelSettings, version, submission)
		return err
	})
	if err != nil {
		return jobs.Accepted{}, err
	}
	return accepted, nil
}

var _ Repository = (*Postgres)(nil)
