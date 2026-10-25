package analytics

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/platform/redis"
)

type Repository struct {
	pool  *pgxpool.Pool
	cache *redis.Cache
}

func NewRepository(pool *pgxpool.Pool, cache *redis.Cache) *Repository {
	return &Repository{pool: pool, cache: cache}
}

func (r *Repository) Summary(ctx context.Context) (entity.AnalyticsSummary, error) {
	var s entity.AnalyticsSummary
	err := r.pool.QueryRow(ctx, `
		SELECT coalesce(c.version, 0), coalesce(c.source_version, 0), p.active_model_version, c.calculated_at, m.horizon_days
		FROM published_state p
		JOIN model_versions m ON m.version = p.active_model_version
		LEFT JOIN calculation_versions c ON c.version = p.active_calculation_version`,
	).Scan(&s.Calculation.Version, &s.Calculation.SourceVersion, &s.Calculation.ModelVersion, &s.Calculation.CalculatedAt, &s.Loss.HorizonDays)
	if err != nil {
		return entity.AnalyticsSummary{}, fmt.Errorf("read published calculation: %w", err)
	}
	if s.Calculation.CalculatedAt != nil {
		*s.Calculation.CalculatedAt = s.Calculation.CalculatedAt.UTC()
	}
	if s.Calculation.Version == 0 {
		return s, nil
	}
	key := redis.Key("summary", s.Calculation.Version, nil)
	var cached entity.AnalyticsSummary
	if r.cache.Get(ctx, key, &cached) {
		return cached, nil
	}
	err = r.pool.QueryRow(ctx, `
		SELECT coalesce(a.total_customers, 0), coalesce(a.assessed_customers, 0),
			coalesce(a.low_count, 0), coalesce(a.medium_count, 0), coalesce(a.high_count, 0), coalesce(a.unknown_count, 0),
			coalesce(a.total_services, 0), coalesce(a.at_risk_services, 0),
			a.loss_amount_cents, coalesce(a.renewal_revenue_cents, 0), coalesce(a.unassessed_revenue_cents, 0),
			coalesce(a.is_partial, false), coalesce(a.loss_explanation, '')
		FROM calculation_versions c
		LEFT JOIN analytics_results a ON a.calculation_version = c.version
		WHERE c.version = $1`, s.Calculation.Version,
	).Scan(&s.TotalCustomers, &s.AssessedCustomers,
		&s.CustomersByRisk.Low, &s.CustomersByRisk.Medium, &s.CustomersByRisk.High, &s.CustomersByRisk.Unknown,
		&s.TotalServices, &s.AtRiskServices,
		&s.Loss.AmountCents, &s.Loss.RenewalRevenueCents, &s.Loss.UnassessedRevenueCents,
		&s.Loss.IsPartial, &s.Loss.Explanation)
	if err != nil {
		return entity.AnalyticsSummary{}, fmt.Errorf("read analytics summary: %w", err)
	}
	r.cache.Set(ctx, key, s)
	return s, nil
}
