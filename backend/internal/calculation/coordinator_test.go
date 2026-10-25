package calculation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
	"github.com/sklyar-vlad/HackAeza/backend/internal/risk"
)

type engineFunc func(risk.Input, entity.ModelSettings, time.Time) (entity.CustomerDetail, error)

func (f engineFunc) Calculate(input risk.Input, settings entity.ModelSettings, asOf time.Time) (entity.CustomerDetail, error) {
	return f(input, settings, asOf)
}

type testRepository struct {
	count, saved, maxBatch, loads int
	prepared                      Run
	published                     bool
	summary                       entity.AnalyticsSummary
	beforeSave                    func()
}

func (r *testRepository) Prepare(context.Context, jobs.Lease, time.Time) (Run, error) {
	return r.prepared, nil
}

func (r *testRepository) Customers(ctx context.Context, _ Run, after entity.ID, limit int) ([]risk.Input, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.saved != int(after) {
		return nil, errors.New("next batch loaded before previous batch was saved")
	}
	r.loads++
	inputs := []risk.Input{}
	for id := int(after) + 1; id <= r.count && len(inputs) < limit; id++ {
		inputs = append(inputs, risk.Input{Account: entity.Account{ID: entity.ID(id)},
			Services: []entity.Service{{ID: entity.ID(id), OwnerID: entity.ID(id)}}})
	}
	r.maxBatch = max(r.maxBatch, len(inputs))
	return inputs, nil
}

func (r *testRepository) SaveResults(ctx context.Context, _ Run, results []entity.CustomerDetail) error {
	if r.beforeSave != nil {
		r.beforeSave()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r.saved += len(results)
	return nil
}

func (r *testRepository) Publish(ctx context.Context, _ Run, summary entity.AnalyticsSummary, _ []entity.ImportWarning) (entity.CalculationJob, error) {
	if err := ctx.Err(); err != nil {
		return entity.CalculationJob{}, err
	}
	r.published, r.summary = true, summary
	return entity.CalculationJob{}, nil
}

func preparedTestRepository(count int) *testRepository {
	asOf := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	return &testRepository{count: count, prepared: Run{Target: entity.Calculation{CalculatedAt: &asOf},
		Settings: entity.ModelSettings{HorizonDays: 7}, Counts: entity.SourceCounts{Accounts: int64(count), Services: int64(count)}}}
}

func testResult(input risk.Input) entity.CustomerDetail {
	score := 50.0
	return entity.CustomerDetail{Customer: entity.Customer{Account: input.Account, ServicesCount: int64(len(input.Services)),
		Risk: entity.RiskAssessment{Status: entity.AssessmentAssessed, Level: entity.RiskMedium, Score: &score},
		Loss: entity.LossEstimate{IsPartial: true}},
		Services: []entity.ServiceAssessment{{Service: input.Services[0], Risk: entity.RiskAssessment{Level: entity.RiskMedium}}}}
}

func TestCoordinatorBoundsParallelismAndMemoryAndUsesOneTimestamp(t *testing.T) {
	for _, parallelism := range []int{1, 3} {
		t.Run(fmt.Sprint(parallelism), func(t *testing.T) {
			repo := preparedTestRepository(17)
			queue := &testQueue{}
			var running, peak atomic.Int64
			engine := engineFunc(func(input risk.Input, _ entity.ModelSettings, asOf time.Time) (entity.CustomerDetail, error) {
				n := running.Add(1)
				defer running.Add(-1)
				for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
				}
				if !asOf.Equal(*repo.prepared.Target.CalculatedAt) || len(input.Services) != 1 || input.Services[0].OwnerID != input.Account.ID {
					return entity.CustomerDetail{}, errors.New("timestamp or customer input changed")
				}
				time.Sleep(time.Millisecond)
				return testResult(input), nil
			})
			options := DefaultOptions()
			options.Parallelism, options.BatchSize = parallelism, 5
			coordinator, err := NewCoordinator(repo, queue, engine, options)
			if err != nil {
				t.Fatal(err)
			}
			if err := coordinator.Execute(context.Background(), jobs.Lease{}); err != nil {
				t.Fatal(err)
			}
			if peak.Load() > int64(parallelism) || (parallelism > 1 && peak.Load() < 2) || running.Load() != 0 {
				t.Fatalf("pool peak=%d, still running=%d", peak.Load(), running.Load())
			}
			if repo.maxBatch > 5 || repo.saved != 17 || !repo.published || repo.loads != 5 {
				t.Fatalf("unexpected batching: %+v", repo)
			}
			if repo.summary.TotalCustomers != 17 || repo.summary.AtRiskServices != 17 || repo.summary.Loss.AmountCents != nil {
				t.Fatalf("unexpected summary: %+v", repo.summary)
			}
			progress := queue.progress
			if len(progress) != 5 || progress[0].ProcessedCustomers != 0 || progress[4].ProcessedCustomers != 17 {
				t.Fatalf("unexpected progress: %v", progress)
			}
			for i := 1; i < len(progress); i++ {
				if progress[i].ProcessedCustomers <= progress[i-1].ProcessedCustomers || progress[i].TotalCustomers != 17 {
					t.Fatalf("non-monotonic progress: %v", progress)
				}
			}
		})
	}
}

func TestCoordinatorDoesNotPublishOnErrorPanicOrCancellation(t *testing.T) {
	for _, failure := range []string{"error", "panic", "cancel"} {
		t.Run(failure, func(t *testing.T) {
			repo := preparedTestRepository(20)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var calls atomic.Int64
			engine := engineFunc(func(input risk.Input, _ entity.ModelSettings, _ time.Time) (entity.CustomerDetail, error) {
				calls.Add(1)
				switch failure {
				case "error":
					return entity.CustomerDetail{}, errors.New("engine failed")
				case "panic":
					panic("engine failed")
				default:
					cancel()
					return testResult(input), nil
				}
			})
			options := DefaultOptions()
			options.BatchSize, options.Parallelism = 5, 2
			coordinator, err := NewCoordinator(repo, &testQueue{}, engine, options)
			if err != nil {
				t.Fatal(err)
			}
			if err := coordinator.Execute(ctx, jobs.Lease{}); err == nil {
				t.Fatal("failed calculation returned success")
			}
			if repo.published || repo.saved != 0 || calls.Load() > 2 {
				t.Fatalf("work continued after failure: saved=%d, calls=%d", repo.saved, calls.Load())
			}
		})
	}
}

type testQueue struct {
	mu       sync.Mutex
	progress []entity.JobProgress
	claims   int
	renews   int
	retries  int
	failures int
	renewErr error
	claim    *jobs.Claim
}

func (q *testQueue) Claim(context.Context, string, time.Duration) (*jobs.Claim, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.claims++
	claim := q.claim
	q.claim = nil
	return claim, nil
}

func (q *testQueue) Renew(context.Context, jobs.Lease, time.Duration) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.renews++
	return q.renewErr
}

func (q *testQueue) SetProgress(_ context.Context, _ jobs.Lease, progress entity.JobProgress) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.progress = append(q.progress, progress)
	return nil
}

func (q *testQueue) Retry(ctx context.Context, _ jobs.Lease, _ time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.retries++
	return ctx.Err()
}

func (q *testQueue) Fail(context.Context, jobs.Lease, entity.JobError) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.failures++
	return nil
}
