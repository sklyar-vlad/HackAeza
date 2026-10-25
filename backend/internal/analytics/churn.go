package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/platform/redis"
)

func (r *Repository) Churn(ctx context.Context, from, to time.Time) (entity.ChurnSeries, error) {
	series := entity.ChurnSeries{Points: []entity.ChurnPoint{}}
	var horizonDays int64
	err := r.pool.QueryRow(ctx, `
		SELECT coalesce(c.version, 0), coalesce(c.source_version, 0), p.active_model_version, c.calculated_at, m.horizon_days
		FROM published_state p
		JOIN model_versions m ON m.version = p.active_model_version
		LEFT JOIN calculation_versions c ON c.version = p.active_calculation_version`,
	).Scan(&series.Calculation.Version, &series.Calculation.SourceVersion, &series.Calculation.ModelVersion,
		&series.Calculation.CalculatedAt, &horizonDays)
	if err != nil {
		return entity.ChurnSeries{}, fmt.Errorf("read churn calculation: %w", err)
	}
	key := redis.Key("churn", series.Calculation.Version, []string{from.Format(time.DateOnly), to.Format(time.DateOnly)})
	var cached entity.ChurnSeries
	if r.cache.Get(ctx, key, &cached) {
		return cached, nil
	}
	var firstDay, horizon time.Time
	if at := series.Calculation.CalculatedAt; at != nil {
		*at = at.UTC()
		firstDay = at.Truncate(24 * time.Hour)
		horizon = at.AddDate(0, 0, int(horizonDays))
	}
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		point := entity.ChurnPoint{Date: day.Format(time.DateOnly)}
		if series.Calculation.Version != 0 && !day.Before(firstDay) && !day.After(horizon) {
			forecast, count := 0.0, int64(0)
			point.ForecastNonRenewals, point.AtRiskServices = &forecast, &count
		}
		series.Points = append(series.Points, point)
	}
	if series.Calculation.Version == 0 {
		return series, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT to_char(greatest(s.expires_at, $3::timestamptz) AT TIME ZONE 'UTC', 'YYYY-MM-DD'),
			CASE WHEN bool_and(r.assessment_status = 'assessed') THEN sum(r.risk_score) / 100 END,
			CASE WHEN bool_and(r.assessment_status = 'assessed') THEN count(*) FILTER (WHERE r.risk_level IN ('medium', 'high')) END
		FROM services s
		JOIN service_results r ON r.source_version = s.snapshot_version AND r.service_id = s.id AND r.calculation_version = $1
		WHERE s.snapshot_version = $2 AND s.status IN ('active', 'suspended', 'prolong_wait')
			AND s.expires_at <= $4
			AND greatest(s.expires_at, $3::timestamptz) >= $5
			AND greatest(s.expires_at, $3::timestamptz) < $6
		GROUP BY 1`, series.Calculation.Version, series.Calculation.SourceVersion,
		series.Calculation.CalculatedAt, horizon, from, to.AddDate(0, 0, 1))
	if err != nil {
		return entity.ChurnSeries{}, fmt.Errorf("read churn forecast: %w", err)
	}
	defer rows.Close()
	byDate := make(map[string]int, len(series.Points))
	for i, point := range series.Points {
		byDate[point.Date] = i
	}
	for rows.Next() {
		var point entity.ChurnPoint
		if err := rows.Scan(&point.Date, &point.ForecastNonRenewals, &point.AtRiskServices); err != nil {
			return entity.ChurnSeries{}, fmt.Errorf("scan churn forecast: %w", err)
		}
		if i, ok := byDate[point.Date]; ok {
			series.Points[i] = point
		}
	}
	if err := rows.Err(); err != nil {
		return entity.ChurnSeries{}, fmt.Errorf("read churn rows: %w", err)
	}
	r.cache.Set(ctx, key, series)
	return series, nil
}
