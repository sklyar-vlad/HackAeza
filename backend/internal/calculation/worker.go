package calculation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
	"go.uber.org/zap"
)

type Executor interface {
	Execute(context.Context, jobs.Lease) error
}

type Worker struct {
	queue    jobs.Queue
	executor Executor
	options  Options
	logger   *zap.Logger
}

func NewWorker(queue jobs.Queue, executor Executor, options Options, logger *zap.Logger) (*Worker, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if queue == nil || executor == nil || logger == nil {
		return nil, fmt.Errorf("queue, executor and logger are required")
	}
	return &Worker{queue: queue, executor: executor, options: options, logger: logger}, nil
}

func (w *Worker) Run(ctx context.Context, workerID string) error {
	if workerID == "" {
		return fmt.Errorf("worker ID is required")
	}
	for ctx.Err() == nil {
		opCtx, cancel := context.WithTimeout(ctx, w.options.OperationTimeout)
		claim, err := w.queue.Claim(opCtx, workerID, w.options.LeaseDuration)
		cancel()
		if err != nil && ctx.Err() == nil {
			w.logger.Error("claim calculation job", zap.Error(err))
		} else if claim != nil {
			w.process(ctx, *claim)
			continue
		}
		timer := time.NewTimer(w.options.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	return nil
}

func (w *Worker) process(ctx context.Context, claim jobs.Claim) {
	jobCtx, timeoutCancel := context.WithTimeout(ctx, w.options.JobTimeout)
	runCtx, cancelRun := context.WithCancelCause(jobCtx)
	heartbeatCtx, stopHeartbeat := context.WithCancel(runCtx)
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		w.heartbeat(heartbeatCtx, claim.Lease, cancelRun)
	}()
	w.logger.Info("calculation started", zap.String("jobId", string(claim.Lease.JobID)), zap.Int("attempt", claim.Job.Attempt))
	err := w.executor.Execute(runCtx, claim.Lease)
	stopHeartbeat()
	<-heartbeatDone
	if err != nil && context.Cause(runCtx) != nil {
		err = context.Cause(runCtx)
	}
	cancelRun(nil)
	timeoutCancel()
	if err == nil {
		w.logger.Info("calculation published", zap.String("jobId", string(claim.Lease.JobID)))
		return
	}
	w.logger.Warn("calculation interrupted", zap.String("jobId", string(claim.Lease.JobID)), zap.Error(err))
	if errors.Is(err, entity.ErrLeaseLost) {
		return
	}
	// Shutdown cancels calculation work, but still releases its lease for a retry.
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), w.options.OperationTimeout)
	defer cancel()
	var releaseErr error
	if errors.Is(err, ErrBatchRecordLimit) || errors.Is(err, entity.ErrVersionConflict) {
		releaseErr = w.queue.Fail(cleanupCtx, claim.Lease, entity.JobError{
			Code: "CALCULATION_FAILED", Message: "Расчёт не выполнен: проверьте ограничения пакета и версии данных. Предыдущие результаты сохранены.",
		})
	} else {
		releaseErr = w.queue.Retry(cleanupCtx, claim.Lease, time.Now().UTC())
	}
	if releaseErr != nil && !errors.Is(releaseErr, entity.ErrLeaseLost) {
		w.logger.Error("release calculation job", zap.String("jobId", string(claim.Lease.JobID)), zap.Error(releaseErr))
	}
}

func (w *Worker) heartbeat(ctx context.Context, lease jobs.Lease, cancel context.CancelCauseFunc) {
	ticker := time.NewTicker(w.options.LeaseDuration / 3)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			opCtx, stop := context.WithTimeout(ctx, w.options.OperationTimeout)
			err := w.queue.Renew(opCtx, lease, w.options.LeaseDuration)
			stop()
			if err != nil {
				if ctx.Err() == nil {
					cancel(fmt.Errorf("renew calculation lease: %w", err))
				}
				return
			}
		}
	}
}
