package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/calculation"
)

func workerOptions() (calculation.Options, error) {
	options := calculation.DefaultOptions()
	for _, field := range []struct {
		name   string
		target *int
	}{
		{"WORKER_PARALLELISM", &options.Parallelism},
		{"WORKER_BATCH_SIZE", &options.BatchSize},
		{"WORKER_MAX_BATCH_RECORDS", &options.MaxBatchRecords},
	} {
		if value, ok := os.LookupEnv(field.name); ok {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return options, fmt.Errorf("%s must be an integer", field.name)
			}
			*field.target = parsed
		}
	}
	for _, field := range []struct {
		name   string
		target *time.Duration
	}{
		{"WORKER_POLL_INTERVAL", &options.PollInterval},
		{"WORKER_LEASE_DURATION", &options.LeaseDuration},
		{"WORKER_JOB_TIMEOUT", &options.JobTimeout},
		{"WORKER_OPERATION_TIMEOUT", &options.OperationTimeout},
	} {
		if value, ok := os.LookupEnv(field.name); ok {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return options, fmt.Errorf("%s must be a duration such as 5s or 10m", field.name)
			}
			*field.target = parsed
		}
	}
	return options, options.Validate()
}
