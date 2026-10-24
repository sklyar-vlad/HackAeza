package entity

import "time"

type JobID string

type JobKind string

const (
	JobModelSettings  JobKind = "model_settings"
	JobSourceSnapshot JobKind = "source_snapshot"
)

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
)

type JobAccepted struct {
	JobID       JobID       `json:"jobId"`
	StatusURL   string      `json:"statusUrl"`
	CreatedAt   time.Time   `json:"createdAt"`
	Calculation Calculation `json:"calculation"`
}

type JobProgress struct {
	ProcessedCustomers int64 `json:"processedCustomers"`
	TotalCustomers     int64 `json:"totalCustomers"`
}

type FieldError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type JobError struct {
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	RequestID string       `json:"requestId"`
	Details   []FieldError `json:"details,omitempty"`
}

// JobResult is the closed OpenAPI oneOf. Storage decodes the concrete result by JobKind.
// Only successful jobs have a result; queued/running jobs have neither result nor error.
type JobResult interface {
	jobResult()
}

func (ModelSettingsResponse) jobResult() {}
func (SnapshotResult) jobResult()        {}

type CalculationJob struct {
	JobID      JobID        `json:"jobId"`
	Kind       JobKind      `json:"kind"`
	Status     JobStatus    `json:"status"`
	CreatedAt  time.Time    `json:"createdAt"`
	StartedAt  *time.Time   `json:"startedAt"`
	FinishedAt *time.Time   `json:"finishedAt"`
	Attempt    int          `json:"attempt"`
	Progress   *JobProgress `json:"progress,omitempty"`
	Result     JobResult    `json:"result,omitempty"`
	Error      *JobError    `json:"error,omitempty"`
}
