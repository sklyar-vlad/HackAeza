package jobs

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ScheduleTime merges timer ticks into the global queue. The published-state lock
// is shared with imports, settings changes, claims and publication on every worker.
// A skipped tick is recovered from the published calculation's timestamp next time.
func (r *Repository) ScheduleTime(ctx context.Context) (*Accepted, error) {
	var accepted *Accepted
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockState(ctx, tx); err != nil {
			return err
		}
		var due bool
		err := tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM published_state s
			JOIN calculation_versions c ON c.version = s.active_calculation_version
			JOIN model_versions m ON m.version = s.active_model_version
			JOIN services v ON v.snapshot_version = c.source_version
			WHERE s.id = 1 AND v.status IN ('active', 'suspended', 'prolong_wait')
			AND (
				c.calculated_at + m.refresh_interval_seconds * interval '1 second' <= clock_timestamp()
				OR (v.expires_at - m.horizon_days * interval '24 hours' > c.calculated_at
					AND v.expires_at - m.horizon_days * interval '24 hours' <= clock_timestamp())
				OR (v.expires_at + interval '1 microsecond' > c.calculated_at
					AND v.expires_at + interval '1 microsecond' <= clock_timestamp())
				OR (v.status = 'prolong_wait'
					AND v.expires_at + interval '120 hours 1 microsecond' > c.calculated_at
					AND v.expires_at + interval '120 hours 1 microsecond' <= clock_timestamp())
			)
			AND NOT EXISTS (SELECT 1 FROM calculation_jobs WHERE status IN ('queued', 'running'))
			AND NOT EXISTS (SELECT 1 FROM calculation_jobs
				WHERE kind = 'time_recalculation' AND status = 'failed'
				AND finished_at > clock_timestamp() - interval '1 minute'
				AND base_calculation_version = c.version)
		)`).Scan(&due)
		if err != nil || !due {
			return err
		}
		value := &Accepted{}
		err = tx.QueryRow(ctx, `INSERT INTO calculation_jobs (kind, request_id, base_calculation_version)
			SELECT 'time_recalculation', 'time:' || gen_random_uuid()::text, active_calculation_version
			FROM published_state WHERE id = 1 RETURNING id::text, created_at`).Scan(&value.JobID, &value.CreatedAt)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE published_state SET pending_job_id = $1::uuid WHERE id = 1`, value.JobID); err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `SELECT c.version, c.source_version, c.model_version, c.calculated_at
			FROM published_state s JOIN calculation_versions c ON c.version = s.active_calculation_version
			WHERE s.id = 1`).Scan(&value.Calculation.Version, &value.Calculation.SourceVersion,
			&value.Calculation.ModelVersion, &value.Calculation.CalculatedAt)
		if err == nil {
			accepted = value
		}
		return err
	})
	return accepted, err
}
