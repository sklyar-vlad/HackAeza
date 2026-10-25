package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sklyar-vlad/HackAeza/backend/internal/calculation"
	"github.com/sklyar-vlad/HackAeza/backend/internal/config"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
	"github.com/sklyar-vlad/HackAeza/backend/internal/platform/postgres"
	"github.com/sklyar-vlad/HackAeza/backend/internal/risk"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "create logger: %v\n", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err = run(ctx, logger)
	stop()
	if err != nil {
		logger.Error("worker stopped", zap.Error(err))
	}
	_ = logger.Sync()
	if err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *zap.Logger) error {
	options, err := workerOptions()
	if err != nil {
		return err
	}
	pool, err := postgres.New(ctx, config.NewConfig().Database)
	if err != nil {
		return err
	}
	defer pool.Close()
	queue := jobs.NewRepository(pool)
	repository := calculation.NewRepository(pool, queue, options.MaxBatchRecords)
	coordinator, err := calculation.NewCoordinator(repository, queue, risk.ScoreEngine{}, options)
	if err != nil {
		return err
	}
	worker, err := calculation.NewWorker(queue, coordinator, options, logger)
	if err != nil {
		return err
	}
	workerID := os.Getenv("WORKER_ID")
	if workerID == "" {
		workerID = rand.Text()
	}
	schedulerCtx, stopScheduler := context.WithCancel(ctx)
	schedulerDone := make(chan struct{})
	go func() {
		defer close(schedulerDone)
		calculation.NewTimeScheduler(queue, logger).Run(schedulerCtx)
	}()
	logger.Info("worker started", zap.String("workerId", workerID),
		zap.Int("parallelism", options.Parallelism), zap.Int("batchSize", options.BatchSize))
	err = worker.Run(ctx, workerID)
	stopScheduler()
	<-schedulerDone
	logger.Info("worker stopped gracefully")
	return err
}
