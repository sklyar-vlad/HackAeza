package calculation

import (
	"context"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
	"go.uber.org/zap"
)

type TimeQueue interface {
	ScheduleTime(context.Context) (*jobs.Accepted, error)
}

type TimeScheduler struct {
	queue  TimeQueue
	logger *zap.Logger
}

func NewTimeScheduler(queue TimeQueue, logger *zap.Logger) *TimeScheduler {
	return &TimeScheduler{queue: queue, logger: logger}
}

// Run belongs in the worker's signal context, alongside its existing claim loop.
// It only schedules work; the normal coordinator must execute and publish it.
// Starting it on every worker is safe, including immediately after a restart.
func (s *TimeScheduler) Run(ctx context.Context) {
	s.run(ctx, 30*time.Second)
}

func (s *TimeScheduler) run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		tickCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		accepted, err := s.queue.ScheduleTime(tickCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			s.logger.Error("schedule time recalculation", zap.Error(err))
		} else if accepted != nil {
			s.logger.Info("time recalculation queued", zap.String("jobId", string(accepted.JobID)))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
