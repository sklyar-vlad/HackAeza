package calculation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

var ErrBatchRecordLimit = errors.New("customer batch exceeds its record limit; reduce batch size or increase WORKER_MAX_BATCH_RECORDS")

type Publisher interface {
	Publish(context.Context, jobs.Lease, func(pgx.Tx) (entity.JobResult, error)) (entity.CalculationJob, error)
}

type PostgresRepository struct {
	pool            *pgxpool.Pool
	publisher       Publisher
	maxBatchRecords int
}

func NewRepository(pool *pgxpool.Pool, publisher Publisher, maxBatchRecords int) *PostgresRepository {
	return &PostgresRepository{pool: pool, publisher: publisher, maxBatchRecords: maxBatchRecords}
}

func (r *PostgresRepository) Prepare(ctx context.Context, lease jobs.Lease, asOf time.Time) (Run, error) {
	run := Run{Lease: lease}
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockLease(ctx, tx, lease); err != nil {
			return err
		}
		var base, source, model int64
		err := tx.QueryRow(ctx, `SELECT j.kind, coalesce(j.base_calculation_version, 0),
			coalesce(c.version, 0), coalesce(c.source_version, 0), s.active_model_version, c.calculated_at,
			coalesce(j.candidate_source_version, c.source_version, 0), coalesce(j.candidate_model_version, s.active_model_version)
			FROM calculation_jobs j CROSS JOIN published_state s
			LEFT JOIN calculation_versions c ON c.version = s.active_calculation_version
			WHERE j.id = $1::uuid AND s.id = 1`, lease.JobID).Scan(&run.Kind, &base,
			&run.Base.Version, &run.Base.SourceVersion, &run.Base.ModelVersion, &run.Base.CalculatedAt, &source, &model)
		if err != nil {
			return err
		}
		if base != run.Base.Version {
			return entity.ErrVersionConflict
		}
		if asOf.IsZero() {
			if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&asOf); err != nil {
				return err
			}
		}
		asOf = asOf.UTC().Truncate(time.Microsecond)
		if run.Base.CalculatedAt != nil && asOf.Before(*run.Base.CalculatedAt) {
			return entity.ErrVersionConflict
		}
		err = tx.QueryRow(ctx, `SELECT version, updated_at, coefficients, medium_from, high_from,
			horizon_days, refresh_interval_seconds, score_formula, loss_formula FROM model_versions WHERE version = $1`, model).
			Scan(&run.Settings.Version, &run.Settings.UpdatedAt, &run.Settings.Coefficients,
				&run.Settings.Thresholds.MediumFrom, &run.Settings.Thresholds.HighFrom, &run.Settings.HorizonDays,
				&run.Settings.RefreshIntervalSeconds, &run.Settings.ScoreFormula, &run.Settings.LossFormula)
		if err != nil {
			return err
		}
		if source != 0 {
			if err := normalizeSnapshot(ctx, tx, source); err != nil {
				return err
			}
			err = tx.QueryRow(ctx, `SELECT accounts_count, services_count, transactions_count, warnings
				FROM source_snapshots WHERE version = $1`, source).Scan(&run.Counts.Accounts,
				&run.Counts.Services, &run.Counts.Transactions, &run.Warnings)
			if err != nil {
				return err
			}
		}
		// Only one job is active globally. Abandoned attempts are invisible to API
		// readers and cannot be written by an expired owner after this lock.
		for _, table := range []string{"analytics_results", "service_results", "customer_results"} {
			if _, err := tx.Exec(ctx, `DELETE FROM `+table+` WHERE calculation_version IN
				(SELECT version FROM calculation_versions WHERE published_at IS NULL)`); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM calculation_versions WHERE published_at IS NULL`); err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `INSERT INTO calculation_versions (source_version, model_version, base_version, calculated_at)
			VALUES (nullif($1::bigint, 0), $2, nullif($3::bigint, 0), $4)
			RETURNING version, coalesce(source_version, 0), model_version, calculated_at`, source, model, base, asOf).
			Scan(&run.Target.Version, &run.Target.SourceVersion, &run.Target.ModelVersion, &run.Target.CalculatedAt)
		if err != nil {
			return err
		}
		return checkLease(ctx, tx, lease)
	})
	return run, err
}

func lockLease(ctx context.Context, tx pgx.Tx, lease jobs.Lease) error {
	var id int16
	if err := tx.QueryRow(ctx, `SELECT id FROM published_state WHERE id = 1 FOR UPDATE`).Scan(&id); err != nil {
		return err
	}
	var jobID string
	err := tx.QueryRow(ctx, `SELECT id::text FROM calculation_jobs WHERE id = $1::uuid
		AND status = 'running' AND lease_token = $2::uuid AND lease_until > clock_timestamp() FOR UPDATE`,
		lease.JobID, lease.Token).Scan(&jobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ErrLeaseLost
	}
	return err
}

func checkLease(ctx context.Context, tx pgx.Tx, lease jobs.Lease) error {
	var live bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM calculation_jobs
		WHERE id = $1::uuid AND status = 'running' AND lease_token = $2::uuid
		AND lease_until > clock_timestamp())`, lease.JobID, lease.Token).Scan(&live); err != nil {
		return err
	}
	if !live {
		return entity.ErrLeaseLost
	}
	return nil
}

func checkRun(ctx context.Context, tx pgx.Tx, run Run) error {
	if run.Target.CalculatedAt == nil || run.Settings.Version != run.Target.ModelVersion {
		return entity.ErrVersionConflict
	}
	var valid bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM calculation_versions c
		CROSS JOIN published_state p
		JOIN calculation_jobs j ON j.id = $1::uuid
		LEFT JOIN calculation_versions b ON b.version = p.active_calculation_version
		LEFT JOIN source_snapshots s ON s.version = c.source_version
		WHERE p.id = 1 AND c.version = $2 AND c.published_at IS NULL
		AND coalesce(c.source_version, 0) = $3 AND c.model_version = $4 AND c.calculated_at = $5
		AND coalesce(c.base_version, 0) = $6 AND coalesce(p.active_calculation_version, 0) = $6
		AND p.active_model_version = $7 AND p.pending_job_id = j.id AND j.kind = $8
		AND j.base_calculation_version IS NOT DISTINCT FROM c.base_version
		AND coalesce(j.candidate_source_version, b.source_version, 0) = $3
		AND coalesce(j.candidate_model_version, p.active_model_version) = $4
		AND coalesce(s.accounts_count, 0) = $9 AND coalesce(s.services_count, 0) = $10
		AND coalesce(s.transactions_count, 0) = $11
		AND coalesce(b.source_version, 0) = $12 AND b.calculated_at IS NOT DISTINCT FROM $13::timestamptz)`,
		run.Lease.JobID, run.Target.Version, run.Target.SourceVersion, run.Target.ModelVersion, run.Target.CalculatedAt,
		run.Base.Version, run.Base.ModelVersion, run.Kind, run.Counts.Accounts, run.Counts.Services,
		run.Counts.Transactions, run.Base.SourceVersion, run.Base.CalculatedAt).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return entity.ErrVersionConflict
	}
	return nil
}

// Import already validates the payload. Expand its arrays inside PostgreSQL so
// the worker never holds the complete JSON snapshot in its Go heap.
func normalizeSnapshot(ctx context.Context, tx pgx.Tx, version int64) error {
	var ready bool
	if err := tx.QueryRow(ctx, `SELECT accounts_count IS NOT NULL FROM source_snapshots WHERE version = $1`, version).Scan(&ready); err != nil {
		return err
	}
	if ready {
		return nil
	}
	for _, statement := range []string{
		`INSERT INTO accounts SELECT s.version, x.id, x.email, x.balance, x."withdrawBalance", x."bonusBalance", x."bonusState", x."totalReplenished"
		FROM (SELECT version, raw_payload->'accounts' AS payload FROM source_snapshots WHERE version = $1
 UNION ALL SELECT snapshot_version, payload FROM source_payload_parts WHERE snapshot_version = $1 AND kind = 'accounts') s
 CROSS JOIN LATERAL jsonb_to_recordset(s.payload) AS x
		(id bigint, email text, balance bigint, "withdrawBalance" bigint, "bonusBalance" bigint, "bonusState" text, "totalReplenished" bigint)
		WHERE s.version = $1`,
		`INSERT INTO services SELECT s.version, x.id, x."ownerId", x.name, x."paymentTerm", x."autoProlong",
		x."createdAt", x."expiresAt", x.status, x.price
		FROM (SELECT version, raw_payload->'services' AS payload FROM source_snapshots WHERE version = $1
 UNION ALL SELECT snapshot_version, payload FROM source_payload_parts WHERE snapshot_version = $1 AND kind = 'services') s
 CROSS JOIN LATERAL jsonb_to_recordset(s.payload) AS x
		(id bigint, "ownerId" bigint, name text, "paymentTerm" text, "autoProlong" boolean,
		"createdAt" timestamptz, "expiresAt" timestamptz, status text, price bigint) WHERE s.version = $1`,
		`INSERT INTO transactions SELECT s.version, x.id, x."serviceId", x.amount, x.status, x.type, x."createdAt", x."performedAt"
		FROM (SELECT version, raw_payload->'transactions' AS payload FROM source_snapshots WHERE version = $1
 UNION ALL SELECT snapshot_version, payload FROM source_payload_parts WHERE snapshot_version = $1 AND kind = 'transactions') s
 CROSS JOIN LATERAL jsonb_to_recordset(s.payload) AS x
		(id bigint, "serviceId" bigint, amount integer, status text, type text, "createdAt" timestamptz, "performedAt" timestamptz)
		WHERE s.version = $1`,
		`UPDATE source_snapshots SET accounts_count = (SELECT count(*) FROM accounts WHERE snapshot_version = $1),
		services_count = (SELECT count(*) FROM services WHERE snapshot_version = $1),
		transactions_count = (SELECT count(*) FROM transactions WHERE snapshot_version = $1) WHERE version = $1`,
	} {
		if _, err := tx.Exec(ctx, statement, version); err != nil {
			return fmt.Errorf("normalize snapshot: %w", err)
		}
	}
	return nil
}

var _ Repository = (*PostgresRepository)(nil)
