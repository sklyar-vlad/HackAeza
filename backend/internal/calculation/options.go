package calculation

import (
	"fmt"
	"time"
)

type Options struct {
	Parallelism      int
	BatchSize        int
	MaxBatchRecords  int
	PollInterval     time.Duration
	LeaseDuration    time.Duration
	JobTimeout       time.Duration
	OperationTimeout time.Duration
}

func DefaultOptions() Options {
	return Options{Parallelism: 4, BatchSize: 200, MaxBatchRecords: 50000,
		PollInterval: time.Second, LeaseDuration: 2 * time.Minute,
		JobTimeout: 10 * time.Minute, OperationTimeout: 30 * time.Second}
}

func (o Options) Validate() error {
	if o.Parallelism < 1 || o.Parallelism > 256 || o.BatchSize < 1 || o.BatchSize > 5000 || o.MaxBatchRecords < o.BatchSize || o.MaxBatchRecords > 1000000 {
		return fmt.Errorf("parallelism must be 1..256, batch size 1..5000, batch records batchSize..1000000")
	}
	if o.PollInterval < time.Millisecond || o.OperationTimeout < time.Millisecond || o.JobTimeout < o.OperationTimeout ||
		o.LeaseDuration/3 < o.OperationTimeout {
		return fmt.Errorf("worker durations must be positive; job timeout >= operation timeout; lease >= 3 * operation timeout")
	}
	return nil
}
