package jobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

const MaxAttempts = 3

var (
	ErrInvalidInput = errors.New("invalid job parameters")
	ErrUnauthorized = errors.New("invalid or expired session")
	idempotencyKey  = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
)

const jobColumns = `id::text, kind, status, created_at, started_at, finished_at,
	attempt, processed_customers, total_customers, result, error`

const liveLease = `id = $1::uuid AND status = 'running'
	AND lease_token = $2::uuid AND lease_until > clock_timestamp()`

type Repository struct {
	pool *pgxpool.Pool
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func validUUID(value string) bool {
	var id pgtype.UUID
	return len(value) == 36 && id.Scan(value) == nil && id.Valid
}

func validSubmission(kind entity.JobKind, submission Submission) bool {
	return (kind == entity.JobModelSettings || kind == entity.JobSourceSnapshot) &&
		submission.ActorID != "" && submission.RequestID != "" &&
		(submission.IdempotencyKey == "" || idempotencyKey.MatchString(submission.IdempotencyKey))
}

func (r *Repository) SessionActor(ctx context.Context, token string) (string, error) {
	hash := sha256.Sum256([]byte(token))
	var actor string
	err := r.pool.QueryRow(ctx, `SELECT s.user_id FROM auth_sessions s
		JOIN app_users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL
		AND s.expires_at > clock_timestamp() AND u.disabled_at IS NULL`, hash[:]).Scan(&actor)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnauthorized
	}
	return actor, err
}

func (r *Repository) Get(ctx context.Context, actorID string, id entity.JobID) (entity.CalculationJob, error) {
	if actorID == "" || !validUUID(string(id)) {
		return entity.CalculationJob{}, entity.ErrNotFound
	}
	job, err := scanJob(r.pool.QueryRow(ctx, `SELECT `+jobColumns+`
		FROM calculation_jobs WHERE id = $1::uuid AND EXISTS (
		SELECT 1 FROM app_users WHERE id = $2 AND disabled_at IS NULL)`, id, actorID))
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.CalculationJob{}, entity.ErrNotFound
	}
	return job, err
}

func scanJob(row pgx.Row, extra ...any) (entity.CalculationJob, error) {
	var job entity.CalculationJob
	var processed, total *int64
	var result, failure []byte
	fields := []any{&job.JobID, &job.Kind, &job.Status, &job.CreatedAt, &job.StartedAt,
		&job.FinishedAt, &job.Attempt, &processed, &total, &result, &failure}
	if err := row.Scan(append(fields, extra...)...); err != nil {
		return job, err
	}
	if processed != nil && total != nil {
		job.Progress = &entity.JobProgress{ProcessedCustomers: *processed, TotalCustomers: *total}
	}
	if result != nil {
		switch job.Kind {
		case entity.JobModelSettings:
			var value entity.ModelSettingsResponse
			if err := json.Unmarshal(result, &value); err != nil {
				return job, fmt.Errorf("decode model job result: %w", err)
			}
			job.Result = value
		case entity.JobSourceSnapshot:
			var value entity.SnapshotResult
			if err := json.Unmarshal(result, &value); err != nil {
				return job, fmt.Errorf("decode snapshot job result: %w", err)
			}
			job.Result = value
		}
	}
	if failure != nil {
		if err := json.Unmarshal(failure, &job.Error); err != nil {
			return job, fmt.Errorf("decode job error: %w", err)
		}
	}
	return job, nil
}

func (r *Repository) Lookup(ctx context.Context, kind entity.JobKind, submission Submission) (*Accepted, error) {
	if !validSubmission(kind, submission) {
		return nil, ErrInvalidInput
	}
	return lookup(ctx, r.pool, kind, submission)
}

func lookup(ctx context.Context, query rowQuerier, kind entity.JobKind, submission Submission) (*Accepted, error) {
	if submission.IdempotencyKey == "" {
		return nil, nil
	}
	var operation entity.JobKind
	var hash, data []byte
	err := query.QueryRow(ctx, `SELECT r.operation, r.payload_hash, r.accepted
		FROM idempotency_receipts r JOIN calculation_jobs j ON j.id = r.job_id
		WHERE r.actor_id = $1 AND r.idempotency_key = $2
		AND (r.expires_at > clock_timestamp() OR j.status IN ('queued', 'running'))`,
		submission.ActorID, submission.IdempotencyKey).Scan(&operation, &hash, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if operation != kind || !bytes.Equal(hash, submission.PayloadHash[:]) {
		return nil, entity.ErrIdempotencyConflict
	}
	var accepted Accepted
	if err := json.Unmarshal(data, &accepted); err != nil {
		return nil, fmt.Errorf("decode job receipt: %w", err)
	}
	return &accepted, nil
}

func (r *Repository) Create(ctx context.Context, tx pgx.Tx, kind entity.JobKind, candidateVersion int64, submission Submission) (Accepted, error) {
	if tx == nil || candidateVersion < 1 || !validSubmission(kind, submission) {
		return Accepted{}, ErrInvalidInput
	}
	var accepted Accepted
	err := tx.QueryRow(ctx, `SELECT coalesce(c.version, 0), coalesce(c.source_version, 0),
		s.active_model_version, c.calculated_at FROM published_state s
		LEFT JOIN calculation_versions c ON c.version = s.active_calculation_version
		WHERE s.id = 1 FOR UPDATE OF s`).Scan(&accepted.Calculation.Version,
		&accepted.Calculation.SourceVersion, &accepted.Calculation.ModelVersion, &accepted.Calculation.CalculatedAt)
	if err != nil {
		return accepted, err
	}
	if previous, err := lookup(ctx, tx, kind, submission); err != nil {
		return accepted, err
	} else if previous != nil {
		return *previous, nil
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM calculation_jobs
		WHERE status IN ('queued', 'running'))`).Scan(&active); err != nil {
		return accepted, err
	}
	if active {
		return accepted, entity.ErrJobInProgress
	}
	err = tx.QueryRow(ctx, `INSERT INTO calculation_jobs
		(kind, created_by, request_id, candidate_source_version, candidate_model_version, base_calculation_version)
		VALUES ($1, $2, $3, CASE WHEN $1 = 'source_snapshot' THEN $4::bigint END,
		CASE WHEN $1 = 'model_settings' THEN $4::bigint END, nullif($5::bigint, 0))
		RETURNING id::text, created_at`, kind, submission.ActorID, submission.RequestID,
		candidateVersion, accepted.Calculation.Version).Scan(&accepted.JobID, &accepted.CreatedAt)
	if err != nil {
		var constraint *pgconn.PgError
		if errors.As(err, &constraint) && constraint.ConstraintName == "calculation_jobs_active_idx" {
			return accepted, entity.ErrJobInProgress
		}
		return accepted, err
	}
	if _, err := tx.Exec(ctx, `UPDATE published_state SET pending_job_id = $1::uuid WHERE id = 1`, accepted.JobID); err != nil {
		return accepted, err
	}
	if submission.IdempotencyKey != "" {
		data, err := json.Marshal(accepted)
		if err != nil {
			return accepted, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM idempotency_receipts
			WHERE actor_id = $1 AND idempotency_key = $2`, submission.ActorID, submission.IdempotencyKey); err != nil {
			return accepted, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO idempotency_receipts
			(actor_id, idempotency_key, operation, payload_hash, job_id, accepted, expires_at)
			VALUES ($1, $2, $3, $4, $5::uuid, $6, clock_timestamp() + interval '24 hours')`,
			submission.ActorID, submission.IdempotencyKey, kind, submission.PayloadHash[:], accepted.JobID, data)
		if err != nil {
			return accepted, err
		}
	}
	return accepted, nil
}
