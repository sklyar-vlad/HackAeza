package calculation

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
	"github.com/sklyar-vlad/HackAeza/backend/internal/risk"
	"github.com/sklyar-vlad/HackAeza/backend/internal/source"
)

type queryCounter struct{ selects atomic.Int64 }

func (q *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(data.SQL)), "SELECT") {
		q.selects.Add(1)
	}
	return ctx
}
func (*queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func calculationDatabase(t *testing.T) (*pgxpool.Pool, *queryCounter) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := "worker_test_" + strings.ToLower(rand.Text())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	counter := &queryCounter{}
	cfg.ConnConfig.Tracer = counter
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, strings.Split(string(data), "-- +goose Down")[0]); err != nil {
			t.Fatalf("apply migration: %v", err)
		}
	}
	return pool, counter
}

func calculationSQL(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql); err != nil {
		t.Fatal(err)
	}
}

func importWorkerFixture(t *testing.T, pool *pgxpool.Pool, count int, now time.Time) *jobs.Claim {
	t.Helper()
	calculationSQL(t, pool, `INSERT INTO app_users (id, username, password_hash) VALUES ('operator', 'operator', 'test-only');`)
	snapshot := entity.SourceSnapshot{Accounts: []entity.Account{}, Services: []entity.Service{}, Transactions: []entity.Transaction{}}
	for id := 1; id <= count; id++ {
		snapshot.Accounts = append(snapshot.Accounts, entity.Account{ID: entity.ID(id), Email: fmt.Sprintf("user%d@example.org", id), BonusState: entity.BonusLocked})
		for n := range 2 {
			serviceID := entity.ID(2*id + n)
			snapshot.Services = append(snapshot.Services, entity.Service{
				ID: serviceID, OwnerID: entity.ID(id), Name: fmt.Sprintf("service-%d", serviceID),
				PaymentTerm: entity.PaymentHour, Status: entity.ServiceActive, Price: 100,
				CreatedAt: entity.SourceTimestamp{Time: now.Add(-24 * time.Hour)},
				ExpiresAt: entity.SourceTimestamp{Time: now.Add(7*24*time.Hour - 3*time.Second)},
			})
			snapshot.Transactions = append(snapshot.Transactions, entity.Transaction{
				ID: serviceID, ServiceID: serviceID, Amount: 100, Status: entity.TransactionNotPerformed,
				Type: entity.TransactionProlong, CreatedAt: entity.SourceTimestamp{Time: now.Add(-time.Hour)},
			})
		}
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	queue := jobs.NewRepository(pool)
	service := source.NewService(source.NewRepository(pool, queue))
	if _, err := service.Import(context.Background(), payload, jobs.Submission{ActorID: "operator", RequestID: "worker-test"}); err != nil {
		t.Fatal(err)
	}
	claim, err := queue.Claim(context.Background(), "test-worker", time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("claim fixture: %v, %v", claim, err)
	}
	return claim
}

type observedRepository struct {
	Repository
	pool      *pgxpool.Pool
	t         *testing.T
	active    int64
	fixedTime time.Time
}

func (r *observedRepository) Prepare(ctx context.Context, lease jobs.Lease, asOf time.Time) (Run, error) {
	if !r.fixedTime.IsZero() {
		asOf = r.fixedTime
	}
	return r.Repository.Prepare(ctx, lease, asOf)
}

func (r *observedRepository) SaveResults(ctx context.Context, run Run, results []entity.CustomerDetail) error {
	if err := r.Repository.SaveResults(ctx, run, results); err != nil {
		return err
	}
	var active int64
	err := r.pool.QueryRow(ctx, `SELECT coalesce(active_calculation_version, 0) FROM published_state`).Scan(&active)
	if err == nil && active != r.active {
		r.t.Errorf("partial results became visible: active=%d, want %d", active, r.active)
	}
	return err
}

func TestPostgresWorkerBatchesAndPublishesAllJobKinds(t *testing.T) {
	pool, counter := calculationDatabase(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	claim := importWorkerFixture(t, pool, 9, now)
	queue := jobs.NewRepository(pool)
	repo := NewRepository(pool, queue, 50000)
	run, err := repo.Prepare(ctx, claim.Lease, now.Add(-10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	counter.selects.Store(0)
	inputs, err := repo.Customers(ctx, run, 0, 3)
	if err != nil || len(inputs) != 3 {
		t.Fatalf("batch: %v, %v", inputs, err)
	}
	if reads := counter.selects.Load(); reads != 3 {
		t.Fatalf("batch needed %d SELECTs instead of 3", reads)
	}
	for _, input := range inputs {
		if len(input.Services) != 2 || len(input.Transactions) != 2 {
			t.Fatalf("incomplete customer loaded: %+v", input)
		}
	}
	options := DefaultOptions()
	options.BatchSize, options.Parallelism = 3, 2
	observed := &observedRepository{Repository: repo, pool: pool, t: t, fixedTime: now.Add(-10 * time.Second)}
	coordinator, err := NewCoordinator(observed, queue, risk.ScoreEngine{}, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Execute(ctx, claim.Lease); err != nil {
		t.Fatal(err)
	}
	initial := assertPublishedWorker(t, pool, queue, claim.Lease.JobID, 9, 0)
	if initial.Kind != entity.JobSourceSnapshot || initial.Result.(entity.SnapshotResult).Counts.Transactions != 18 {
		t.Fatalf("unexpected snapshot result: %+v", initial)
	}
	var previous int64
	if err := pool.QueryRow(ctx, `SELECT active_calculation_version FROM published_state`).Scan(&previous); err != nil {
		t.Fatal(err)
	}
	accepted, err := queue.ScheduleTime(ctx)
	if err != nil || accepted == nil {
		t.Fatalf("time boundary did not create a job: %v", err)
	}
	claim, err = queue.Claim(ctx, "test-worker", time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("claim timer: %v", err)
	}
	observed.active, observed.fixedTime = previous, time.Time{}
	if err := coordinator.Execute(ctx, claim.Lease); err != nil {
		t.Fatal(err)
	}
	timed := assertPublishedWorker(t, pool, queue, claim.Lease.JobID, 9, 75)
	if timed.Kind != entity.JobTimeRecalculation {
		t.Fatalf("unexpected time result: %+v", timed)
	}
	if err := pool.QueryRow(ctx, `SELECT active_calculation_version FROM published_state`).Scan(&previous); err != nil {
		t.Fatal(err)
	}
	calculationSQL(t, pool, `INSERT INTO model_versions
		(coefficients, medium_from, high_from, horizon_days, refresh_interval_seconds, score_formula, loss_formula)
		SELECT '{"renewalWithoutAutoProlong":0,"insufficientRenewalFunds":0,"paymentWaiting":1}',
		medium_from, high_from, horizon_days, refresh_interval_seconds, score_formula, loss_formula FROM model_versions WHERE version = 1;`)
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		_, err := queue.Create(ctx, tx, entity.JobModelSettings, 2, jobs.Submission{ActorID: "operator", RequestID: "model-test"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	claim, err = queue.Claim(ctx, "test-worker", time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("claim settings: %v", err)
	}
	observed.active = previous
	failing := engineFunc(func(input risk.Input, settings entity.ModelSettings, asOf time.Time) (entity.CustomerDetail, error) {
		if input.Account.ID == 4 {
			return entity.CustomerDetail{}, errors.New("temporary engine error")
		}
		return (risk.ScoreEngine{}).Calculate(input, settings, asOf)
	})
	broken, err := NewCoordinator(observed, queue, failing, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := broken.Execute(ctx, claim.Lease); err == nil {
		t.Fatal("failed calculation published")
	}
	var active, staged int64
	if err := pool.QueryRow(ctx, `SELECT active_calculation_version,
		(SELECT count(*) FROM customer_results r JOIN calculation_versions c ON c.version = r.calculation_version
		WHERE c.published_at IS NULL) FROM published_state`).Scan(&active, &staged); err != nil {
		t.Fatal(err)
	}
	if active != previous || staged != 3 {
		t.Fatalf("failure exposed incomplete data: active=%d, staged=%d", active, staged)
	}
	if err := queue.Retry(ctx, claim.Lease, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	calculationSQL(t, pool, `UPDATE calculation_jobs SET not_before = clock_timestamp() WHERE status = 'queued';`)
	claim, err = queue.Claim(ctx, "replacement-worker", time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("claim retry: %v", err)
	}
	if err := coordinator.Execute(ctx, claim.Lease); err != nil {
		t.Fatal(err)
	}
	model := assertPublishedWorker(t, pool, queue, claim.Lease.JobID, 9, 0)
	if model.Kind != entity.JobModelSettings || model.Result.(entity.ModelSettingsResponse).Settings.Version != 2 {
		t.Fatalf("unexpected model result: %+v", model)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM calculation_versions WHERE published_at IS NULL`).Scan(&staged); err != nil || staged != 0 {
		t.Fatalf("abandoned attempts were not cleaned up: %d, %v", staged, err)
	}
}

func assertPublishedWorker(t *testing.T, pool *pgxpool.Pool, queue *jobs.Repository, jobID entity.JobID, count int64, expectedScore float64) entity.CalculationJob {
	t.Helper()
	ctx := context.Background()
	job, err := queue.Get(ctx, "operator", jobID)
	if err != nil || job.Status != entity.JobSucceeded || job.Progress == nil || job.Progress.ProcessedCustomers != count || job.Progress.TotalCustomers != count {
		t.Fatalf("job was not completed: %+v, %v", job, err)
	}
	var customers, services, sourceVersion, unknownMoney int64
	var minScore, maxScore float64
	err = pool.QueryRow(ctx, `SELECT count(*), min(r.risk_score), max(r.risk_score), min(r.source_version),
		count(*) FILTER (WHERE r.loss_amount_cents IS NULL)
		FROM customer_results r JOIN published_state s ON s.active_calculation_version = r.calculation_version`).
		Scan(&customers, &minScore, &maxScore, &sourceVersion, &unknownMoney)
	if err != nil || customers != count || minScore != expectedScore || maxScore != expectedScore || sourceVersion != 1 || unknownMoney != count {
		t.Fatalf("published customers: count=%d, score=%v..%v, source=%d, unknownMoney=%d, err=%v", customers, minScore, maxScore, sourceVersion, unknownMoney, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM service_results r JOIN published_state s ON s.active_calculation_version = r.calculation_version`).Scan(&services); err != nil || services != 2*count {
		t.Fatalf("published services: %d, %v", services, err)
	}
	return job
}

func TestPostgresWorkerRejectsOversizedBatchAndExpiredWriter(t *testing.T) {
	pool, _ := calculationDatabase(t)
	ctx := context.Background()
	claim := importWorkerFixture(t, pool, 1, time.Now().UTC().Truncate(time.Millisecond))
	queue := jobs.NewRepository(pool)
	repo := NewRepository(pool, queue, 3)
	run, err := repo.Prepare(ctx, claim.Lease, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Customers(ctx, run, 0, 1); !errors.Is(err, ErrBatchRecordLimit) {
		t.Fatalf("batch limit was ignored: %v", err)
	}
	calculationSQL(t, pool, `UPDATE calculation_jobs SET created_at = clock_timestamp() - interval '3 minutes',
		started_at = clock_timestamp() - interval '2 minutes', lease_until = clock_timestamp() - interval '1 minute';`)
	if err := repo.SaveResults(ctx, run, nil); !errors.Is(err, entity.ErrLeaseLost) {
		t.Fatalf("expired worker saved results: %v", err)
	}
	if _, err := repo.Publish(ctx, run, newSummary(run), nil); !errors.Is(err, entity.ErrLeaseLost) {
		t.Fatalf("expired worker published: %v", err)
	}
}
