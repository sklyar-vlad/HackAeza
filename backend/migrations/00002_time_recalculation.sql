-- +goose Up
ALTER TABLE calculation_jobs DROP CONSTRAINT calculation_jobs_kind_check;
ALTER TABLE calculation_jobs ADD CONSTRAINT calculation_jobs_kind_check
    CHECK (kind IN ('model_settings', 'source_snapshot', 'time_recalculation'));

ALTER TABLE calculation_jobs DROP CONSTRAINT calculation_jobs_check2;
ALTER TABLE calculation_jobs ADD CONSTRAINT calculation_jobs_check2 CHECK (
    (kind = 'source_snapshot' AND candidate_source_version IS NOT NULL AND candidate_model_version IS NULL)
    OR (kind = 'model_settings' AND candidate_model_version IS NOT NULL AND candidate_source_version IS NULL)
    OR (kind = 'time_recalculation' AND candidate_source_version IS NULL AND candidate_model_version IS NULL
        AND base_calculation_version IS NOT NULL)
);

-- +goose Down
-- Rollback is intentionally rejected while time jobs exist: their published results must be retained.
ALTER TABLE calculation_jobs DROP CONSTRAINT calculation_jobs_check2;
ALTER TABLE calculation_jobs ADD CONSTRAINT calculation_jobs_check2 CHECK (
    (kind = 'source_snapshot' AND candidate_source_version IS NOT NULL AND candidate_model_version IS NULL)
    OR (kind = 'model_settings' AND candidate_model_version IS NOT NULL AND candidate_source_version IS NULL)
);
ALTER TABLE calculation_jobs DROP CONSTRAINT calculation_jobs_kind_check;
ALTER TABLE calculation_jobs ADD CONSTRAINT calculation_jobs_kind_check
    CHECK (kind IN ('model_settings', 'source_snapshot'));
