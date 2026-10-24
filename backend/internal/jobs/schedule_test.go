package jobs

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

func testTimeRepository(t *testing.T) (*Repository, *pgxpool.Pool) {
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
	schema := "time_test_" + strings.ToLower(rand.Text())
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
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, name := range []string{"00001_initial.sql", "00002_time_recalculation.sql"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, strings.Split(string(data), "-- +goose Down")[0]); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	return NewRepository(pool), pool
}

func execTimeSQL(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql); err != nil {
		t.Fatal(err)
	}
}

func seedTimeCalculation(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	execTimeSQL(t, pool, `
		INSERT INTO source_snapshots (accounts_count, services_count, transactions_count) VALUES (1, 1, 0);
		INSERT INTO accounts VALUES (1, 1, 'test@example.org', 0, 0, 0, 'locked', 0);
		INSERT INTO services VALUES (1, 1, 1, 'test-service', 'month', false,
			clock_timestamp() - interval '1 day', clock_timestamp() + interval '1 hour', 'active', 100);
		INSERT INTO calculation_versions (source_version, model_version, calculated_at, published_at)
			VALUES (1, 1, clock_timestamp() - interval '2 minutes', clock_timestamp());
		UPDATE published_state SET active_calculation_version = 1 WHERE id = 1;
	`)
}

func TestScheduleTimeDueBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		sql  string
		due  bool
	}{
		{"interval elapsed", "", true},
		{"not due", `UPDATE calculation_versions SET calculated_at = clock_timestamp(), published_at = clock_timestamp();`, false},
		{"hourly cadence", `UPDATE model_versions SET refresh_interval_seconds = 86400;
			UPDATE services SET payment_term = 'hour';`, true},
		{"monthly cadence", `UPDATE model_versions SET refresh_interval_seconds = 86400;`, false},
		{"enters horizon", `UPDATE model_versions SET refresh_interval_seconds = 86400;
			UPDATE services SET expires_at = clock_timestamp() + interval '7 days' - interval '1 minute';`, true},
		{"payment deadline", `UPDATE model_versions SET refresh_interval_seconds = 86400;
			UPDATE services SET expires_at = clock_timestamp() - interval '1 minute';`, true},
		{"payment waiting ages", `UPDATE model_versions SET refresh_interval_seconds = 86400;
			UPDATE services SET created_at = clock_timestamp() - interval '10 days',
			expires_at = clock_timestamp() - interval '5 days 1 minute', status = 'prolong_wait';`, true},
		{"deleted", `UPDATE services SET status = 'deleted';`, false},
		{"blocked", `UPDATE services SET status = 'blocked';`, false},
		{"no snapshot", `UPDATE published_state SET active_calculation_version = NULL;`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, pool := testTimeRepository(t)
			seedTimeCalculation(t, pool)
			if tc.sql != "" {
				execTimeSQL(t, pool, tc.sql)
			}
			accepted, err := repo.ScheduleTime(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if (accepted != nil) != tc.due {
				t.Fatalf("queued=%v, want %v", accepted != nil, tc.due)
			}
		})
	}
}

func TestTimeJobsCoalesceAndUseExistingLeaseAndPublication(t *testing.T) {
	repo, pool := testTimeRepository(t)
	seedTimeCalculation(t, pool)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan *Accepted, 16)
	errorsCh := make(chan error, 16)
	for range 16 {
		wg.Go(func() {
			accepted, err := repo.ScheduleTime(ctx)
			results <- accepted
			errorsCh <- err
		})
	}
	wg.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	var accepted *Accepted
	for result := range results {
		if result != nil {
			if accepted != nil {
				t.Fatal("multiple workers queued duplicate timer jobs")
			}
			accepted = result
		}
	}
	if accepted == nil {
		t.Fatal("no time job was scheduled")
	}
	claim, err := repo.Claim(ctx, "worker-one", time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("claim: %v, %v", claim, err)
	}
	other, err := repo.Claim(ctx, "worker-two", time.Minute)
	if err != nil || other != nil {
		t.Fatalf("second worker received live job: %v, %v", other, err)
	}
	if result, err := repo.ScheduleTime(ctx); err != nil || result != nil {
		t.Fatalf("running job must absorb timer ticks: %v, %v", result, err)
	}
	execTimeSQL(t, pool, `UPDATE calculation_jobs SET created_at = clock_timestamp() - interval '3 minutes',
		started_at = clock_timestamp() - interval '2 minutes',
		lease_until = clock_timestamp() - interval '1 minute' WHERE status = 'running';`)
	_, err = repo.Publish(ctx, claim.Lease, func(pgx.Tx) (entity.JobResult, error) {
		t.Fatal("expired worker was allowed to publish")
		return nil, nil
	})
	if !errors.Is(err, entity.ErrLeaseLost) {
		t.Fatalf("expired lease: %v", err)
	}
	if _, err := repo.Claim(ctx, "worker-two", time.Minute); err != nil {
		t.Fatal(err)
	}
	execTimeSQL(t, pool, `UPDATE calculation_jobs SET not_before = clock_timestamp() WHERE status = 'queued';`)
	reclaimed, err := repo.Claim(ctx, "worker-two", time.Minute)
	if err != nil || reclaimed == nil || reclaimed.Lease.Token == claim.Lease.Token {
		t.Fatalf("reclaim: %v, %v", reclaimed, err)
	}
	for _, tc := range []struct {
		name          string
		source, model int64
	}{
		{"source changed", 2, 1},
		{"model changed", 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := repo.Publish(ctx, reclaimed.Lease, func(pgx.Tx) (entity.JobResult, error) {
				asOf := time.Now().UTC()
				return entity.TimeRecalculationResult{Calculation: entity.Calculation{
					Version: 2, SourceVersion: tc.source, ModelVersion: tc.model, CalculatedAt: &asOf,
				}}, nil
			})
			if !errors.Is(err, entity.ErrVersionConflict) {
				t.Fatalf("time job changed source/model: %v", err)
			}
		})
	}
	job, err := repo.Publish(ctx, reclaimed.Lease, func(tx pgx.Tx) (entity.JobResult, error) {
		var calculation entity.Calculation
		err := tx.QueryRow(ctx, `INSERT INTO calculation_versions
			(source_version, model_version, base_version, calculated_at, published_at)
			VALUES (1, 1, 1, clock_timestamp(), clock_timestamp())
			RETURNING version, source_version, model_version, calculated_at`).Scan(&calculation.Version,
			&calculation.SourceVersion, &calculation.ModelVersion, &calculation.CalculatedAt)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `UPDATE published_state SET active_calculation_version = $1 WHERE id = 1`, calculation.Version)
		return entity.TimeRecalculationResult{Calculation: calculation}, err
	})
	if err != nil || job.Status != entity.JobSucceeded {
		t.Fatalf("publish: %v, %v", job, err)
	}
	if _, ok := job.Result.(entity.TimeRecalculationResult); !ok {
		t.Fatalf("unexpected result: %T", job.Result)
	}
	if result, err := repo.ScheduleTime(ctx); err != nil || result != nil {
		t.Fatalf("fresh publication scheduled twice: %v, %v", result, err)
	}
	_, err = repo.Publish(ctx, reclaimed.Lease, func(pgx.Tx) (entity.JobResult, error) {
		t.Fatal("completed job was published twice")
		return nil, nil
	})
	if !errors.Is(err, entity.ErrLeaseLost) {
		t.Fatalf("completed lease: %v", err)
	}
}

func TestTimeJobWaitsForImportAndBacksOffAfterFailure(t *testing.T) {
	repo, pool := testTimeRepository(t)
	seedTimeCalculation(t, pool)
	ctx := context.Background()
	execTimeSQL(t, pool, `INSERT INTO calculation_jobs (kind, request_id, candidate_source_version, base_calculation_version)
		VALUES ('source_snapshot', 'import', 1, 1);`)
	if accepted, err := repo.ScheduleTime(ctx); err != nil || accepted != nil {
		t.Fatalf("timer interfered with import: %v, %v", accepted, err)
	}
	claim, err := repo.Claim(ctx, "worker", time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("claim import: %v", err)
	}
	if err := repo.Fail(ctx, claim.Lease, entity.JobError{Code: "TEST", Message: "test failure"}); err != nil {
		t.Fatal(err)
	}
	accepted, err := repo.ScheduleTime(ctx)
	if err != nil || accepted == nil {
		t.Fatalf("timer lost after busy queue: %v", err)
	}
	claim, err = repo.Claim(ctx, "worker", time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("claim timer: %v", err)
	}
	if err := repo.Fail(ctx, claim.Lease, entity.JobError{Code: "TEST", Message: "test failure"}); err != nil {
		t.Fatal(err)
	}
	if accepted, err := repo.ScheduleTime(ctx); err != nil || accepted != nil {
		t.Fatalf("failed job produced immediate duplicate: %v, %v", accepted, err)
	}
	execTimeSQL(t, pool, `UPDATE calculation_jobs SET created_at = clock_timestamp() - interval '3 minutes',
		started_at = clock_timestamp() - interval '2 minutes', finished_at = clock_timestamp() - interval '1 minute 1 second'
		WHERE kind = 'time_recalculation';`)
	if accepted, err := repo.ScheduleTime(ctx); err != nil || accepted == nil {
		t.Fatalf("retry did not recover: %v, %v", accepted, err)
	}
}
