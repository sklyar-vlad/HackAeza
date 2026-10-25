package jobs

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

func (r *Repository) Publish(ctx context.Context, lease Lease, publish func(pgx.Tx) (entity.JobResult, error)) (entity.CalculationJob, error) {
	var job entity.CalculationJob
	if publish == nil {
		return job, ErrInvalidInput
	}
	err := r.withLease(ctx, lease, func(tx pgx.Tx, _ int, _ string) error {
		var kind entity.JobKind
		var current, base, sourceVersion, modelVersion int64
		err := tx.QueryRow(ctx, `SELECT j.kind, coalesce(j.base_calculation_version, 0),
			coalesce(s.active_calculation_version, 0), coalesce(c.source_version, 0), s.active_model_version
			FROM calculation_jobs j CROSS JOIN published_state s
			LEFT JOIN calculation_versions c ON c.version = s.active_calculation_version
			WHERE j.id = $1::uuid AND s.id = 1`, lease.JobID).Scan(&kind, &base, &current, &sourceVersion, &modelVersion)
		if err != nil {
			return err
		}
		if base != current {
			return entity.ErrVersionConflict
		}
		result, err := publish(tx)
		if err != nil {
			return err
		}
		switch result.(type) {
		case entity.ModelSettingsResponse, *entity.ModelSettingsResponse:
			if kind != entity.JobModelSettings {
				return ErrInvalidInput
			}
		case entity.SnapshotResult, *entity.SnapshotResult:
			if kind != entity.JobSourceSnapshot {
				return ErrInvalidInput
			}
		case entity.TimeRecalculationResult, *entity.TimeRecalculationResult:
			if kind != entity.JobTimeRecalculation {
				return ErrInvalidInput
			}
		default:
			return ErrInvalidInput
		}
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		var output struct {
			Calculation entity.Calculation   `json:"calculation"`
			Settings    entity.ModelSettings `json:"settings"`
		}
		if err := json.Unmarshal(data, &output); err != nil {
			return err
		}
		calculation := output.Calculation
		if calculation.Version <= base || calculation.CalculatedAt == nil {
			return ErrInvalidInput
		}
		if (kind == entity.JobSourceSnapshot && calculation.ModelVersion != modelVersion) ||
			(kind == entity.JobTimeRecalculation && (calculation.SourceVersion != sourceVersion || calculation.ModelVersion != modelVersion)) ||
			(kind == entity.JobModelSettings && (calculation.SourceVersion != sourceVersion || output.Settings.Version != calculation.ModelVersion)) {
			return entity.ErrVersionConflict
		}
		var published bool
		err = tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM published_state s
			JOIN calculation_versions c ON c.version = s.active_calculation_version
			JOIN calculation_jobs j ON j.id = $1::uuid
			WHERE s.id = 1 AND c.version = $2 AND coalesce(c.source_version, 0) = $3
			AND c.model_version = $4 AND c.calculated_at = $5 AND c.published_at IS NOT NULL
			AND s.active_model_version = c.model_version
			AND c.base_version IS NOT DISTINCT FROM j.base_calculation_version
			AND ((j.kind = 'source_snapshot' AND c.source_version = j.candidate_source_version)
			OR (j.kind = 'model_settings' AND c.model_version = j.candidate_model_version)
			OR (j.kind = 'time_recalculation' AND c.source_version = $3 AND c.model_version = $4)))`,
			lease.JobID, calculation.Version, calculation.SourceVersion, calculation.ModelVersion, calculation.CalculatedAt).Scan(&published)
		if err != nil {
			return err
		}
		if !published {
			return entity.ErrVersionConflict
		}
		tag, err := tx.Exec(ctx, `UPDATE calculation_jobs SET status = 'succeeded', finished_at = clock_timestamp(),
			worker_id = NULL, lease_token = NULL, lease_until = NULL,
			result_calculation_version = $3, result = $4 WHERE `+liveLease,
			lease.JobID, lease.Token, calculation.Version, data)
		if err := leaseUpdate(tag, err); err != nil {
			return err
		}
		if err := clearPending(ctx, tx, lease.JobID); err != nil {
			return err
		}
		job, err = scanJob(tx.QueryRow(ctx, `SELECT `+jobColumns+` FROM calculation_jobs WHERE id = $1::uuid`, lease.JobID))
		return err
	})
	if err != nil {
		return entity.CalculationJob{}, err
	}
	return job, nil
}

var (
	_ Queue    = (*Repository)(nil)
	_ Reader   = (*Repository)(nil)
	_ Receipts = (*Repository)(nil)
)
