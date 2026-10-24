package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

func (r *Repository) Claim(ctx context.Context, workerID string, leaseDuration time.Duration) (*Claim, error) {
	if workerID == "" || leaseDuration < time.Millisecond {
		return nil, ErrInvalidInput
	}
	var claim *Claim
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockState(ctx, tx); err != nil {
			return err
		}
		var id entity.JobID
		var status entity.JobStatus
		var attempt int
		var requestID string
		err := tx.QueryRow(ctx, `SELECT id::text, status, attempt, request_id FROM calculation_jobs
			WHERE (status = 'queued' AND not_before <= clock_timestamp())
			OR (status = 'running' AND lease_until <= clock_timestamp())
			ORDER BY created_at, id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &status, &attempt, &requestID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if attempt >= MaxAttempts {
			failure, err := json.Marshal(exhausted(requestID))
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE calculation_jobs SET status = 'failed', finished_at = clock_timestamp(),
				worker_id = NULL, lease_token = NULL, lease_until = NULL, error = $2 WHERE id = $1::uuid`, id, failure); err != nil {
				return err
			}
			return clearPending(ctx, tx, id)
		}
		if status == entity.JobRunning {
			_, err := tx.Exec(ctx, `UPDATE calculation_jobs SET status = 'queued', started_at = NULL,
				worker_id = NULL, lease_token = NULL, lease_until = NULL,
				processed_customers = NULL, total_customers = NULL,
				not_before = clock_timestamp() + $2 * interval '5 seconds' WHERE id = $1::uuid`, id, attempt)
			return err
		}
		value := &Claim{Lease: Lease{JobID: id}}
		value.Job, err = scanJob(tx.QueryRow(ctx, `UPDATE calculation_jobs
			SET status = 'running', started_at = clock_timestamp(), attempt = attempt + 1,
			worker_id = $2, lease_token = gen_random_uuid(),
			lease_until = clock_timestamp() + $3 * interval '1 millisecond',
			processed_customers = NULL, total_customers = NULL
			WHERE id = $1::uuid RETURNING `+jobColumns+`, lease_token::text, lease_until, request_id`,
			id, workerID, leaseDuration.Milliseconds()), &value.Lease.Token, &value.ExpiresAt, &value.RequestID)
		if err == nil {
			claim = value
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return claim, nil
}

func (r *Repository) Renew(ctx context.Context, lease Lease, leaseDuration time.Duration) error {
	if leaseDuration < time.Millisecond {
		return ErrInvalidInput
	}
	return r.withLease(ctx, lease, func(tx pgx.Tx, _ int, _ string) error {
		tag, err := tx.Exec(ctx, `UPDATE calculation_jobs
			SET lease_until = greatest(lease_until, clock_timestamp() + $3 * interval '1 millisecond')
			WHERE `+liveLease, lease.JobID, lease.Token, leaseDuration.Milliseconds())
		return leaseUpdate(tag, err)
	})
}

func (r *Repository) SetProgress(ctx context.Context, lease Lease, progress entity.JobProgress) error {
	if progress.ProcessedCustomers < 0 || progress.TotalCustomers < progress.ProcessedCustomers {
		return ErrInvalidInput
	}
	return r.withLease(ctx, lease, func(tx pgx.Tx, _ int, _ string) error {
		tag, err := tx.Exec(ctx, `UPDATE calculation_jobs SET processed_customers = $3, total_customers = $4
			WHERE `+liveLease, lease.JobID, lease.Token, progress.ProcessedCustomers, progress.TotalCustomers)
		return leaseUpdate(tag, err)
	})
}

func (r *Repository) Retry(ctx context.Context, lease Lease, notBefore time.Time) error {
	return r.withLease(ctx, lease, func(tx pgx.Tx, attempt int, requestID string) error {
		if attempt >= MaxAttempts {
			return fail(ctx, tx, lease, exhausted(requestID))
		}
		tag, err := tx.Exec(ctx, `UPDATE calculation_jobs SET status = 'queued', started_at = NULL,
			worker_id = NULL, lease_token = NULL, lease_until = NULL,
			processed_customers = NULL, total_customers = NULL,
			not_before = greatest($3::timestamptz, clock_timestamp() + $4 * interval '5 seconds')
			WHERE `+liveLease, lease.JobID, lease.Token, notBefore, attempt)
		return leaseUpdate(tag, err)
	})
}

func (r *Repository) Fail(ctx context.Context, lease Lease, failure entity.JobError) error {
	if failure.Code == "" || failure.Message == "" {
		return ErrInvalidInput
	}
	return r.withLease(ctx, lease, func(tx pgx.Tx, _ int, requestID string) error {
		failure.RequestID = requestID
		return fail(ctx, tx, lease, failure)
	})
}

func fail(ctx context.Context, tx pgx.Tx, lease Lease, failure entity.JobError) error {
	data, err := json.Marshal(failure)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE calculation_jobs SET status = 'failed', finished_at = clock_timestamp(),
		worker_id = NULL, lease_token = NULL, lease_until = NULL, error = $3
		WHERE `+liveLease, lease.JobID, lease.Token, data)
	if err := leaseUpdate(tag, err); err != nil {
		return err
	}
	return clearPending(ctx, tx, lease.JobID)
}

func exhausted(requestID string) entity.JobError {
	return entity.JobError{Code: "RETRIES_EXHAUSTED", Message: "Исчерпано число попыток расчёта. Предыдущие результаты сохранены.", RequestID: requestID}
}

func (r *Repository) withLease(ctx context.Context, lease Lease, action func(pgx.Tx, int, string) error) error {
	if !validLease(lease) {
		return entity.ErrLeaseLost
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockState(ctx, tx); err != nil {
			return err
		}
		var attempt int
		var requestID string
		err := tx.QueryRow(ctx, `SELECT attempt, request_id FROM calculation_jobs
			WHERE `+liveLease+` FOR UPDATE`, lease.JobID, lease.Token).Scan(&attempt, &requestID)
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.ErrLeaseLost
		}
		if err != nil {
			return err
		}
		return action(tx, attempt, requestID)
	})
}

func lockState(ctx context.Context, tx pgx.Tx) error {
	var id int16
	return tx.QueryRow(ctx, `SELECT id FROM published_state WHERE id = 1 FOR UPDATE`).Scan(&id)
}

func clearPending(ctx context.Context, tx pgx.Tx, id entity.JobID) error {
	_, err := tx.Exec(ctx, `UPDATE published_state SET pending_job_id = NULL WHERE pending_job_id = $1::uuid`, id)
	return err
}

func validLease(lease Lease) bool {
	return validUUID(string(lease.JobID)) && validUUID(lease.Token)
}

func leaseUpdate(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return entity.ErrLeaseLost
	}
	return nil
}
