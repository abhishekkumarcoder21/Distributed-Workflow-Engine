package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/distributed-workflow-engine/backend/internal/domain"
	"github.com/distributed-workflow-engine/backend/internal/queue"
	"github.com/distributed-workflow-engine/backend/internal/store"
)

// WorkerSupervisor monitors worker liveness in PostgreSQL and Redis.
// When an ACTIVE worker has stopped heartbeating beyond deadTimeout,
// the supervisor marks it OFFLINE, deregisters it from Redis, and re-enqueues
// any jobs that were left in RUNNING state back into the Redis queue.
type WorkerSupervisor struct {
	workers     *store.WorkerStore
	q           queue.Queue
	checkPeriod time.Duration
	deadTimeout time.Duration
	logger      *slog.Logger
}

func NewWorkerSupervisor(
	workers *store.WorkerStore,
	q queue.Queue,
	checkPeriod, deadTimeout time.Duration,
	logger *slog.Logger,
) *WorkerSupervisor {
	if checkPeriod <= 0 {
		checkPeriod = 10 * time.Second
	}
	if deadTimeout <= 0 {
		deadTimeout = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &WorkerSupervisor{
		workers:     workers,
		q:           q,
		checkPeriod: checkPeriod,
		deadTimeout: deadTimeout,
		logger:      logger.With("component", "worker_supervisor"),
	}
}

// Run starts the supervisor loop and blocks until ctx is cancelled.
func (s *WorkerSupervisor) Run(ctx context.Context) error {
	s.logger.Info("worker supervisor started", "check_period", s.checkPeriod, "dead_timeout", s.deadTimeout)
	ticker := time.NewTicker(s.checkPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("worker supervisor stopped")
			return nil
		case <-ticker.C:
			if err := s.ReapDeadWorkers(ctx); err != nil {
				s.logger.Error("error reaping dead workers", "error", err)
			}
		}
	}
}

// ReapDeadWorkers finds inactive workers, marks them OFFLINE, and reclaims their jobs.
func (s *WorkerSupervisor) ReapDeadWorkers(ctx context.Context) error {
	if s.workers == nil {
		return nil
	}

	deadWorkers, err := s.workers.FindDeadWorkers(ctx, s.deadTimeout)
	if err != nil {
		return err
	}

	for _, w := range deadWorkers {
		s.logger.Warn("detected dead worker", "worker_id", w.ID, "last_heartbeat", w.LastHeartbeat)

		// Mark worker OFFLINE
		if err := s.workers.SetStatus(ctx, w.ID, domain.WorkerStatusOffline); err != nil {
			s.logger.Error("failed to mark worker OFFLINE", "worker_id", w.ID, "error", err)
		}

		// Remove from Redis
		if s.q != nil {
			_ = s.q.RemoveWorker(ctx, w.ID)
		}

		// Recover orphaned RUNNING jobs
		recoveredJobs, err := s.workers.RecoverDeadWorkerJobs(ctx, w.ID)
		if err != nil {
			s.logger.Error("failed to recover dead worker jobs", "worker_id", w.ID, "error", err)
			continue
		}

		if len(recoveredJobs) > 0 {
			s.logger.Info("recovered orphaned jobs from dead worker", "worker_id", w.ID, "count", len(recoveredJobs))
			if s.q != nil {
				for _, job := range recoveredJobs {
					if err := s.q.Enqueue(ctx, job); err != nil {
						s.logger.Error("failed to re-enqueue recovered job", "job_id", job.ID, "error", err)
					}
				}
			}
		}
	}

	return nil
}
