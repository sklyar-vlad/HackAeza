package calculation

import (
	"context"
	"testing"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
	"go.uber.org/zap"
)

type executorFunc func(context.Context, jobs.Lease) error

func (f executorFunc) Execute(ctx context.Context, lease jobs.Lease) error { return f(ctx, lease) }

func TestWorkerRenewsLeaseAndCancelsOnLossOrTimeout(t *testing.T) {
	for _, mode := range []string{"renew", "lost", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			options := DefaultOptions()
			options.LeaseDuration, options.OperationTimeout, options.JobTimeout = 120*time.Millisecond, 20*time.Millisecond, 200*time.Millisecond
			queue := &testQueue{}
			if mode == "lost" {
				queue.renewErr = entity.ErrLeaseLost
			}
			executor := executorFunc(func(ctx context.Context, _ jobs.Lease) error {
				if mode == "renew" {
					select {
					case <-time.After(110 * time.Millisecond):
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				<-ctx.Done()
				return ctx.Err()
			})
			worker, err := NewWorker(queue, executor, options, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			worker.process(ctx, jobs.Claim{})
			if queue.renews == 0 {
				t.Fatal("lease heartbeat did not run")
			}
			if (mode == "timeout" && queue.retries != 1) || (mode != "timeout" && queue.retries != 0) || queue.failures != 0 {
				t.Fatalf("incorrect completion: retries=%d, failures=%d", queue.retries, queue.failures)
			}
		})
	}
}

func TestWorkerShutdownWaitsAndRequeuesWithoutCancelledContext(t *testing.T) {
	queue := &testQueue{claim: &jobs.Claim{}}
	started := make(chan struct{})
	exited := make(chan struct{})
	executor := executorFunc(func(ctx context.Context, _ jobs.Lease) error {
		defer close(exited)
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	worker, err := NewWorker(queue, executor, DefaultOptions(), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx, "test-worker") }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not claim the job")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop")
	}
	select {
	case <-exited:
	default:
		t.Fatal("worker left its calculation running")
	}
	if queue.retries != 1 || queue.claims != 1 {
		t.Fatalf("shutdown: retries=%d, claims=%d", queue.retries, queue.claims)
	}
}
