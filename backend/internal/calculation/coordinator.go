package calculation

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
	"github.com/sklyar-vlad/HackAeza/backend/internal/risk"
)

type Coordinator struct {
	repository Repository
	queue      jobs.Queue
	engine     risk.Engine
	options    Options
}

// The engine is shared by the pool and must treat inputs/settings as read-only.
func NewCoordinator(repository Repository, queue jobs.Queue, engine risk.Engine, options Options) (*Coordinator, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if repository == nil || queue == nil || engine == nil {
		return nil, fmt.Errorf("repository, queue and risk engine are required")
	}
	return &Coordinator{repository: repository, queue: queue, engine: engine, options: options}, nil
}

func (c *Coordinator) Execute(ctx context.Context, lease jobs.Lease) error {
	opCtx, cancel := c.operation(ctx)
	// PostgreSQL fixes one timestamp for the complete run, independent of host clocks.
	run, err := c.repository.Prepare(opCtx, lease, time.Time{})
	cancel()
	if err != nil {
		return err
	}
	if run.Target.CalculatedAt == nil {
		return fmt.Errorf("calculation timestamp is missing")
	}
	summary := newSummary(run)
	if err := c.progress(ctx, lease, 0, run.Counts.Accounts); err != nil {
		return err
	}
	var after entity.ID
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		opCtx, cancel = c.operation(ctx)
		inputs, err := c.repository.Customers(opCtx, run, after, c.options.BatchSize)
		cancel()
		if err != nil {
			return err
		}
		if len(inputs) == 0 {
			break
		}
		if len(inputs) > c.options.BatchSize {
			return fmt.Errorf("repository exceeded the customer batch size")
		}
		for _, input := range inputs {
			if input.Account.ID <= after {
				return fmt.Errorf("customer pagination did not advance")
			}
			after = input.Account.ID
		}
		results, err := c.calculateBatch(ctx, inputs, run)
		if err != nil {
			return err
		}
		for _, result := range results {
			if err := addCustomer(&summary, result); err != nil {
				return err
			}
		}
		opCtx, cancel = c.operation(ctx)
		err = c.repository.SaveResults(opCtx, run, results)
		cancel()
		if err != nil {
			return err
		}
		if err := c.progress(ctx, lease, summary.TotalCustomers, run.Counts.Accounts); err != nil {
			return err
		}
	}
	if summary.TotalCustomers != run.Counts.Accounts || summary.TotalServices != run.Counts.Services {
		return fmt.Errorf("calculated counts do not match the prepared snapshot")
	}
	if summary.Loss.IsPartial && summary.Loss.UnassessedRevenueCents == summary.Loss.RenewalRevenueCents {
		summary.Loss.AmountCents = nil
	}
	opCtx, cancel = c.operation(ctx)
	defer cancel()
	_, err = c.repository.Publish(opCtx, run, summary, run.Warnings)
	return err
}

func (c *Coordinator) calculateBatch(ctx context.Context, inputs []risk.Input, run Run) ([]entity.CustomerDetail, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	results := make([]entity.CustomerDetail, len(inputs))
	var next atomic.Int64
	var workers sync.WaitGroup
	for range min(c.options.Parallelism, len(inputs)) {
		workers.Go(func() {
			defer func() {
				if failure := recover(); failure != nil {
					cancel(fmt.Errorf("risk engine panicked: %v", failure))
				}
			}()
			for ctx.Err() == nil {
				index := int(next.Add(1) - 1)
				if index >= len(inputs) {
					return
				}
				result, err := c.engine.Calculate(inputs[index], run.Settings, *run.Target.CalculatedAt)
				if err != nil {
					cancel(fmt.Errorf("calculate customer %d: %w", inputs[index].Account.ID, err))
					return
				}
				if result.ID != inputs[index].Account.ID || len(result.Services) != len(inputs[index].Services) {
					cancel(fmt.Errorf("risk engine returned an incomplete customer"))
					return
				}
				results[index] = result
			}
		})
	}
	workers.Wait()
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Coordinator) operation(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.options.OperationTimeout)
}

func (c *Coordinator) progress(ctx context.Context, lease jobs.Lease, processed, total int64) error {
	opCtx, cancel := c.operation(ctx)
	defer cancel()
	return c.queue.SetProgress(opCtx, lease, entity.JobProgress{ProcessedCustomers: processed, TotalCustomers: total})
}
