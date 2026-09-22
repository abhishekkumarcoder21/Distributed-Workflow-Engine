package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/distributed-workflow-engine/backend/internal/queue"
	"github.com/distributed-workflow-engine/backend/internal/store"
)

// RetryScheduler periodically checks PostgreSQL for RETRYING jobs whose run_at has passed,
// atomically updates their status to QUEUED, and enqueues them back into Redis.
type RetryScheduler struct {
	jobs      *store.JobStore
	q         queue.Queue
	interval  time.Duration
	batchSize int
	logger    *slog.Logger
}

func NewRetryScheduler(
	jobs *store.JobStore,
	q queue.Queue,
	interval time.Duration,
	batchSize int,
	logger *slog.Logger,
) *RetryScheduler {
	if interval <= 0 {
		interval = 1 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &RetryScheduler{
		jobs:      jobs,
		q:         q,
		interval:  interval,
		batchSize: batchSize,
		logger:    logger.With("component", "retry_scheduler"),
	}
}

// Run starts the scheduler loop and blocks until ctx is cancelled.
func (s *RetryScheduler) Run(ctx context.Context) error {
	s.logger.Info("retry scheduler started", "interval", s.interval)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("retry scheduler stopped")
			return nil
		case <-ticker.C:
			count, err := s.PollAndRequeue(ctx)
			if err != nil {
				s.logger.Error("error polling and requeuing retries", "error", err)
			} else if count > 0 {
				s.logger.Info("requeued due retry jobs", "count", count)
			}
		}
	}
}

// PollAndRequeue atomically claims due retrying jobs from PostgreSQL and enqueues them to Redis.
func (s *RetryScheduler) PollAndRequeue(ctx context.Context) (int, error) {
	jobs, err := s.jobs.RequeueDueRetries(ctx, s.batchSize)
	if err != nil {
		return 0, err
	}

	if len(jobs) == 0 {
		return 0, nil
	}

	enqueued := 0
	for _, job := range jobs {
		if err := s.q.Enqueue(ctx, job); err != nil {
			s.logger.Error("failed to enqueue retry job to redis", "job_id", job.ID, "error", err)
			continue
		}
		enqueued++
	}

	return enqueued, nil
}
