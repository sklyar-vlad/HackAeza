package metrics

import (
	"context"
	"crypto/subtle"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	customers   = prometheus.NewDesc("churn_customers", "Number of customers by risk level.", []string{"risk_level"}, nil)
	loss        = prometheus.NewDesc("churn_potential_loss_eur", "Scenario estimate of renewal revenue at risk in EUR.", nil, nil)
	available   = prometheus.NewDesc("churn_loss_estimate_available", "Whether a loss estimate is available.", nil, nil)
	version     = prometheus.NewDesc("churn_calculation_version", "Published calculation version.", nil, nil)
	age         = prometheus.NewDesc("churn_calculation_age_seconds", "Seconds since the published calculation was made.", nil, nil)
	services    = prometheus.NewDesc("churn_services", "Number of services in the published calculation.", []string{"scope"}, nil)
	jobs        = prometheus.NewDesc("churn_jobs", "Recalculation jobs that are not finished, by kind and status.", []string{"kind", "status"}, nil)
	scrapeError = prometheus.NewDesc("churn_metrics_scrape_error", "Whether reading published metrics from PostgreSQL failed.", nil, nil)
)

type collector struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func (c collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{customers, loss, available, version, age, services, jobs, scrapeError} {
		ch <- d
	}
}

func (c collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.collect(ctx, ch); err != nil {
		c.logger.Error("collect metrics", zap.Error(err))
		ch <- prometheus.MustNewConstMetric(scrapeError, prometheus.GaugeValue, 1)
		return
	}
	ch <- prometheus.MustNewConstMetric(scrapeError, prometheus.GaugeValue, 0)
}

func (c collector) collect(ctx context.Context, ch chan<- prometheus.Metric) error {
	var calc int64
	var calculatedAt *time.Time
	var low, medium, high, unknown, total, atRisk int64
	var cents *int64
	err := c.pool.QueryRow(ctx, `
		SELECT coalesce(c.version, 0), c.calculated_at,
			coalesce(a.low_count, 0), coalesce(a.medium_count, 0), coalesce(a.high_count, 0), coalesce(a.unknown_count, 0),
			coalesce(a.total_services, 0), coalesce(a.at_risk_services, 0), a.loss_amount_cents
		FROM published_state p
		LEFT JOIN calculation_versions c ON c.version = p.active_calculation_version
		LEFT JOIN analytics_results a ON a.calculation_version = c.version
		WHERE p.id = 1`).Scan(&calc, &calculatedAt, &low, &medium, &high, &unknown, &total, &atRisk, &cents)
	if err != nil {
		return err
	}
	for level, count := range map[string]int64{"low": low, "medium": medium, "high": high, "unknown": unknown} {
		ch <- prometheus.MustNewConstMetric(customers, prometheus.GaugeValue, float64(count), level)
	}
	amount, known := math.NaN(), 0.0
	if cents != nil {
		amount, known = float64(*cents)/100, 1
	}
	ch <- prometheus.MustNewConstMetric(loss, prometheus.GaugeValue, amount)
	ch <- prometheus.MustNewConstMetric(available, prometheus.GaugeValue, known)
	ch <- prometheus.MustNewConstMetric(version, prometheus.GaugeValue, float64(calc))
	if calculatedAt != nil {
		ch <- prometheus.MustNewConstMetric(age, prometheus.GaugeValue, time.Since(*calculatedAt).Seconds())
	}
	ch <- prometheus.MustNewConstMetric(services, prometheus.GaugeValue, float64(total), "total")
	ch <- prometheus.MustNewConstMetric(services, prometheus.GaugeValue, float64(atRisk), "at_risk")

	rows, err := c.pool.Query(ctx, `SELECT kind, status, count(*) FROM calculation_jobs
		WHERE status IN ('queued', 'running') GROUP BY kind, status`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, status string
		var count int64
		if err := rows.Scan(&kind, &status, &count); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(jobs, prometheus.GaugeValue, float64(count), kind, status)
	}
	return rows.Err()
}

var (
	requests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total", Help: "HTTP requests handled by the API.",
	}, []string{"method", "route", "status"})
	durations = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_request_duration_seconds", Help: "HTTP request duration in seconds.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "route"})
)

func Observe(c *gin.Context) {
	started := time.Now()
	c.Next()
	route := c.FullPath()
	if route == "" {
		route = "unmatched"
	}
	requests.WithLabelValues(c.Request.Method, route, strconv.Itoa(c.Writer.Status())).Inc()
	durations.WithLabelValues(c.Request.Method, route).Observe(time.Since(started).Seconds())
}

func Handler(pool *pgxpool.Pool, token string, logger *zap.Logger) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collector{pool: pool, logger: logger},
		requests, durations,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	metrics := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if token == "" {
			http.Error(w, "metrics are disabled: METRICS_TOKEN is not set", http.StatusForbidden)
			return
		}
		header := r.Header.Get("Authorization")
		given, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(given), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		metrics.ServeHTTP(w, r)
	})
}
