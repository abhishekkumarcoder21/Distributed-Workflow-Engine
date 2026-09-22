package queue

import (
	"context"
	"errors"
	"time"

	"github.com/distributed-workflow-engine/backend/internal/domain"
	"github.com/google/uuid"
)

var (
	// ErrQueueEmpty is returned when no jobs are available to claim.
	ErrQueueEmpty = errors.New("queue is empty")
	// ErrJobNotFound is returned when a job is not found.
	ErrJobNotFound = errors.New("job not found in queue")
	// ErrInvalidJob is returned when job attributes are invalid for queuing.
	ErrInvalidJob = errors.New("invalid job for queuing")
)

// Queue defines the interface for job dispatch and worker liveness tracking.
type Queue interface {
	// Enqueue pushes a job into the Redis priority queue.
	Enqueue(ctx context.Context, job *domain.Job) error

	// Claim atomically claims the highest priority job from the queue and moves it to in-progress.
	// Returns ErrQueueEmpty if no jobs are available.
	Claim(ctx context.Context, workerID string, queueName string, visibilityTimeout time.Duration) (uuid.UUID, error)

	// Ack removes the job from the in-progress set upon completion.
	Ack(ctx context.Context, queueName string, jobID uuid.UUID) error

	// RequeueStale finds jobs in in-progress whose visibility timeout has expired and moves them back to the priority queue.
	RequeueStale(ctx context.Context, queueName string, defaultPriority int) (int, error)

	// Heartbeat refreshes the worker's heartbeat in Redis with a TTL.
	Heartbeat(ctx context.Context, workerID string, ttl time.Duration) error

	// RemoveWorker removes worker heartbeat on shutdown.
	RemoveWorker(ctx context.Context, workerID string) error

	// QueueLength returns the total number of waiting jobs in a queue.
	QueueLength(ctx context.Context, queueName string) (int64, error)

	// InProgressCount returns the number of jobs currently in-progress.
	InProgressCount(ctx context.Context, queueName string) (int64, error)

	// GetQueueStats returns a breakdown of jobs waiting by priority and jobs in-progress.
	GetQueueStats(ctx context.Context, queueName string) (*QueueStats, error)

	// Close closes any underlying network connections.
	Close() error
}

type QueueStats struct {
	QueueName       string        `json:"queue_name"`
	PriorityCounts  map[int]int64 `json:"priority_counts"`
	InProgressCount int64         `json:"in_progress_count"`
}
