package calculation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

var assessmentColumns = []string{"assessment_status", "risk_level", "risk_score", "loss_amount_cents",
	"renewal_revenue_cents", "unassessed_revenue_cents", "is_partial", "loss_explanation", "features", "reasons"}

func (r *PostgresRepository) SaveResults(ctx context.Context, run Run, results []entity.CustomerDetail) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockLease(ctx, tx, run.Lease); err != nil {
			return err
		}
		if err := checkRun(ctx, tx, run); err != nil {
			return err
		}
		ids := make([]int64, len(results))
		for i, result := range results {
			ids[i] = int64(result.ID)
			for _, service := range result.Services {
				if service.OwnerID != result.ID {
					return fmt.Errorf("service %d does not belong to customer %d", service.ID, result.ID)
				}
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM service_results r USING services s
			WHERE r.calculation_version = $1 AND r.source_version = s.snapshot_version
			AND r.service_id = s.id AND s.owner_id = ANY($2::bigint[])`, run.Target.Version, ids); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM customer_results
			WHERE calculation_version = $1 AND customer_id = ANY($2::bigint[])`, run.Target.Version, ids); err != nil {
			return err
		}
		customerColumns := append([]string{"calculation_version", "customer_id", "source_version", "services_count"}, assessmentColumns...)
		_, err := tx.CopyFrom(ctx, pgx.Identifier{"customer_results"}, customerColumns,
			pgx.CopyFromSlice(len(results), func(i int) ([]any, error) {
				result := results[i]
				row := []any{run.Target.Version, result.ID, run.Target.SourceVersion, int64(len(result.Services))}
				return assessmentRow(row, result.Risk, result.Features, result.Loss)
			}))
		if err != nil {
			return err
		}
		serviceColumns := append([]string{"calculation_version", "service_id", "source_version"}, assessmentColumns...)
		customer, service := 0, 0
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"service_results"}, serviceColumns, pgx.CopyFromFunc(func() ([]any, error) {
			for customer < len(results) && service >= len(results[customer].Services) {
				customer++
				service = 0
			}
			if customer == len(results) {
				return nil, nil
			}
			result := results[customer].Services[service]
			service++
			row := []any{run.Target.Version, result.ID, run.Target.SourceVersion}
			return assessmentRow(row, result.Risk, result.Features, result.Loss)
		}))
		if err != nil {
			return err
		}
		return checkLease(ctx, tx, run.Lease)
	})
}

func assessmentRow(row []any, assessment entity.RiskAssessment, features []entity.FeatureResult, loss entity.LossEstimate) ([]any, error) {
	if features == nil {
		features = []entity.FeatureResult{}
	}
	reasons := assessment.Reasons
	if reasons == nil {
		reasons = []entity.RiskReason{}
	}
	featureJSON, err := json.Marshal(features)
	if err != nil {
		return nil, err
	}
	reasonJSON, err := json.Marshal(reasons)
	if err != nil {
		return nil, err
	}
	return append(row, assessment.Status, assessment.Level, assessment.Score, loss.AmountCents,
		loss.RenewalRevenueCents, loss.UnassessedRevenueCents, loss.IsPartial, loss.Explanation, featureJSON, reasonJSON), nil
}

func (r *PostgresRepository) Publish(ctx context.Context, run Run, summary entity.AnalyticsSummary, warnings []entity.ImportWarning) (entity.CalculationJob, error) {
	return r.publisher.Publish(ctx, run.Lease, func(tx pgx.Tx) (entity.JobResult, error) {
		if err := checkRun(ctx, tx, run); err != nil {
			return nil, err
		}
		if summary.Calculation.Version != run.Target.Version || summary.Calculation.SourceVersion != run.Target.SourceVersion ||
			summary.Calculation.ModelVersion != run.Target.ModelVersion || summary.Calculation.CalculatedAt == nil ||
			!summary.Calculation.CalculatedAt.Equal(*run.Target.CalculatedAt) || summary.Loss.HorizonDays != run.Settings.HorizonDays {
			return nil, entity.ErrVersionConflict
		}
		var customers, services int64
		err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM customer_results WHERE calculation_version = $1),
			(SELECT count(*) FROM service_results WHERE calculation_version = $1)`, run.Target.Version).Scan(&customers, &services)
		if err != nil {
			return nil, err
		}
		if customers != run.Counts.Accounts || services != run.Counts.Services || summary.TotalCustomers != customers || summary.TotalServices != services {
			return nil, fmt.Errorf("cannot publish incomplete calculation")
		}
		_, err = tx.Exec(ctx, `INSERT INTO analytics_results (calculation_version, total_customers, assessed_customers,
			low_count, medium_count, high_count, unknown_count, total_services, at_risk_services, loss_amount_cents,
			renewal_revenue_cents, unassessed_revenue_cents, is_partial, loss_explanation)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
			run.Target.Version, summary.TotalCustomers, summary.AssessedCustomers, summary.CustomersByRisk.Low,
			summary.CustomersByRisk.Medium, summary.CustomersByRisk.High, summary.CustomersByRisk.Unknown,
			summary.TotalServices, summary.AtRiskServices, summary.Loss.AmountCents, summary.Loss.RenewalRevenueCents,
			summary.Loss.UnassessedRevenueCents, summary.Loss.IsPartial, summary.Loss.Explanation)
		if err != nil {
			return nil, err
		}
		tag, err := tx.Exec(ctx, `UPDATE calculation_versions SET published_at = clock_timestamp()
			WHERE version = $1 AND published_at IS NULL`, run.Target.Version)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() != 1 {
			return nil, entity.ErrVersionConflict
		}
		tag, err = tx.Exec(ctx, `UPDATE published_state SET active_calculation_version = $1, active_model_version = $2
			WHERE id = 1 AND coalesce(active_calculation_version, 0) = $3
			AND active_model_version = $4 AND pending_job_id = $5::uuid`,
			run.Target.Version, run.Target.ModelVersion, run.Base.Version, run.Base.ModelVersion, run.Lease.JobID)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() != 1 {
			return nil, entity.ErrVersionConflict
		}
		switch run.Kind {
		case entity.JobTimeRecalculation:
			return entity.TimeRecalculationResult{Calculation: run.Target}, nil
		case entity.JobModelSettings:
			return entity.ModelSettingsResponse{Calculation: run.Target, Settings: run.Settings}, nil
		case entity.JobSourceSnapshot:
			if warnings == nil {
				warnings = []entity.ImportWarning{}
			}
			return entity.SnapshotResult{Calculation: run.Target, Counts: run.Counts, Warnings: warnings}, nil
		default:
			return nil, fmt.Errorf("unsupported calculation kind %q", run.Kind)
		}
	})
}
