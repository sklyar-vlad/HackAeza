package source

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

type PostgresRepository struct {
	pool  *pgxpool.Pool
	queue jobs.Creator
}

func NewRepository(pool *pgxpool.Pool, queue jobs.Creator) *PostgresRepository {
	return &PostgresRepository{pool: pool, queue: queue}
}

func (r *PostgresRepository) Lookup(ctx context.Context, kind entity.JobKind, submission jobs.Submission) (*jobs.Accepted, error) {
	return r.queue.Lookup(ctx, kind, submission)
}

func (r *PostgresRepository) AcceptSnapshot(ctx context.Context, snapshot entity.SourceSnapshot, submission jobs.Submission) (jobs.Accepted, error) {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return jobs.Accepted{}, err
	}
	var accepted jobs.Accepted
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		previous, err := r.queue.LookupForUpdate(ctx, tx, entity.JobSourceSnapshot, submission)
		if err != nil {
			return err
		}
		if previous != nil {
			accepted = *previous
			return nil
		}
		var version int64
		if err := tx.QueryRow(ctx, `INSERT INTO source_snapshots (raw_payload, captured_at)
			VALUES ($1, $2) RETURNING version`, payload, snapshot.CapturedAt).Scan(&version); err != nil {
			return err
		}
		accepted, err = r.queue.Create(ctx, tx, entity.JobSourceSnapshot, version, submission)
		return err
	})
	if err != nil {
		return jobs.Accepted{}, err
	}
	return accepted, nil
}

func (r *PostgresRepository) Status(ctx context.Context) (entity.SourceStatus, error) {
	var status entity.SourceStatus
	err := r.pool.QueryRow(ctx, `SELECT s.version IS NOT NULL,
		coalesce(s.accounts_count, 0), coalesce(s.services_count, 0), coalesce(s.transactions_count, 0),
		s.received_at, s.captured_at, coalesce(c.version, 0), coalesce(c.source_version, 0),
		p.active_model_version, c.calculated_at, j.id::text
		FROM published_state p
		LEFT JOIN calculation_versions c ON c.version = p.active_calculation_version
		LEFT JOIN source_snapshots s ON s.version = c.source_version
		LEFT JOIN calculation_jobs j ON j.id = p.pending_job_id AND j.status IN ('queued', 'running')
		WHERE p.id = 1`).Scan(&status.SnapshotPresent,
		&status.Counts.Accounts, &status.Counts.Services, &status.Counts.Transactions,
		&status.ReceivedAt, &status.CapturedAt, &status.Calculation.Version, &status.Calculation.SourceVersion,
		&status.Calculation.ModelVersion, &status.Calculation.CalculatedAt, &status.PendingJobID)
	return status, err
}

var _ Repository = (*PostgresRepository)(nil)
