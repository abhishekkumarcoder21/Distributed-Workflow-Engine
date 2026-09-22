package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type JobStatus string

const (
	JobStatusQueued     JobStatus = "QUEUED"
	JobStatusRunning    JobStatus = "RUNNING"
	JobStatusSucceeded  JobStatus = "SUCCEEDED"
	JobStatusFailed     JobStatus = "FAILED"
	JobStatusRetrying   JobStatus = "RETRYING"
	JobStatusCancelled  JobStatus = "CANCELLED"
	JobStatusTimeout    JobStatus = "TIMEOUT"
	JobStatusDeadLetter JobStatus = "DEAD_LETTER"
)

func (s JobStatus) IsTerminal() bool {
	switch s {
	case JobStatusSucceeded, JobStatusFailed, JobStatusCancelled, JobStatusTimeout, JobStatusDeadLetter:
		return true
	}
	return false
}

type Job struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	Type           string          `json:"type" db:"type"`
	Payload        json.RawMessage `json:"payload" db:"payload"`
	Status         JobStatus       `json:"status" db:"status"`
	Priority       int             `json:"priority" db:"priority"`
	MaxAttempts    int             `json:"max_attempts" db:"max_attempts"`
	Attempt        int             `json:"attempt" db:"attempt"`
	IdempotencyKey *string         `json:"idempotency_key,omitempty" db:"idempotency_key"`
	Error          *string         `json:"error,omitempty" db:"error"`
	WorkerID       *string         `json:"worker_id,omitempty" db:"worker_id"`
	Queue          string          `json:"queue" db:"queue"`
	RunAt          time.Time       `json:"run_at" db:"run_at"`
	StartedAt      *time.Time      `json:"started_at,omitempty" db:"started_at"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
}

type CreateJobRequest struct {
	Type           string          `json:"type"`
	Payload        json.RawMessage `json:"payload"`
	Priority       *int            `json:"priority,omitempty"`
	MaxAttempts    *int            `json:"max_attempts,omitempty"`
	IdempotencyKey *string         `json:"idempotency_key,omitempty"`
	Queue          *string         `json:"queue,omitempty"`
}

// Defaults applied during creation.
const (
	DefaultPriority    = 2 // medium
	DefaultMaxAttempts = 3
	DefaultQueue       = "default"
)

type JobFilter struct {
	Status   *JobStatus
	Type     *string
	Queue    *string
	Priority *int
	Limit    int
	Offset   int
}
