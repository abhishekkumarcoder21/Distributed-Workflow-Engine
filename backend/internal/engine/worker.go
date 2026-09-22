package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/distributed-workflow-engine/backend/internal/domain"
	"github.com/distributed-workflow-engine/backend/internal/metrics"
	"github.com/distributed-workflow-engine/backend/internal/queue"
	"github.com/distributed-workflow-engine/backend/internal/store"
	"github.com/google/uuid"
)

// HandlerFunc is the signature for job execution functions.
type HandlerFunc func(ctx context.Context, payload json.RawMessage) error

type Worker struct {
	id                string
	queueName         string
	jobs              *store.JobStore
	workers           *store.WorkerStore
	q                 queue.Queue
	handlers          map[string]HandlerFunc
	pollInterval      time.Duration
	jobTimeout        time.Duration
	visibilityTimeout time.Duration
	heartbeatInterval time.Duration
	concurrency       int
	sem               chan struct{}
	wg                sync.WaitGroup
	logger            *slog.Logger

	mu sync.Mutex
}

func NewWorker(
	id string,
	jobs *store.JobStore,
	workers *store.WorkerStore,
	q queue.Queue,
	pollInterval, jobTimeout, visibilityTimeout, heartbeatInterval time.Duration,
	concurrency int,
	logger *slog.Logger,
) *Worker {
	if visibilityTimeout <= 0 {
		visibilityTimeout = 30 * time.Second
	}
	if heartbeatInterval <= 0 {
		heartbeatInterval = 10 * time.Second
	}
	if concurrency <= 0 {
		concurrency = 5
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &Worker{
		id:                id,
		queueName:         domain.DefaultQueue,
		jobs:              jobs,
		workers:           workers,
		q:                 q,
		handlers:          make(map[string]HandlerFunc),
		pollInterval:      pollInterval,
		jobTimeout:        jobTimeout,
		visibilityTimeout: visibilityTimeout,
		heartbeatInterval: heartbeatInterval,
		concurrency:       concurrency,
		sem:               make(chan struct{}, concurrency),
		logger:            logger.With("worker_id", id),
	}
}

func (w *Worker) SetQueueName(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queueName = name
}

func (w *Worker) RegisterHandler(jobType string, fn HandlerFunc) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[jobType] = fn
}

// Run starts the worker loop and blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("worker starting", "queue", w.queueName, "concurrency", w.concurrency)

	// Register worker in PostgreSQL
	if w.workers != nil {
		if err := w.workers.Register(ctx, w.id); err != nil {
			w.logger.Warn("failed to register worker in database", "error", err)
		}
	}

	// Start worker heartbeat in background
	go w.heartbeatLoop(ctx)

	// Start stale job recovery loop in background
	go w.staleRecoveryLoop(ctx)

	defer func() {
		w.logger.Info("worker entering DRAINING state, waiting for in-flight jobs...")
		if w.workers != nil {
			_ = w.workers.SetStatus(context.Background(), w.id, domain.WorkerStatusDraining)
		}

		// Wait for all in-flight jobs to complete
		w.wg.Wait()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if w.workers != nil {
			_ = w.workers.SetStatus(shutdownCtx, w.id, domain.WorkerStatusOffline)
		}
		if w.q != nil {
			if err := w.q.RemoveWorker(shutdownCtx, w.id); err != nil {
				w.logger.Warn("failed to remove worker heartbeat on shutdown", "error", err)
			}
		}
		w.logger.Info("worker stopped cleanly")
	}()

	for {
		// Acquire a concurrency slot before claiming a job
		select {
		case <-ctx.Done():
			return nil
		case w.sem <- struct{}{}:
		}

		if w.q == nil {
			<-w.sem
			return nil
		}

		claimedID, err := w.q.Claim(ctx, w.id, w.queueName, w.visibilityTimeout)
		if err != nil {
			<-w.sem // release concurrency slot
			if errors.Is(err, queue.ErrQueueEmpty) {
				w.sleep(ctx)
				continue
			}
			w.logger.Error("failed to claim job from queue", "error", err)
			w.sleep(ctx)
			continue
		}

		w.wg.Add(1)
		go func(jobID uuid.UUID) {
			defer func() {
				<-w.sem
				w.wg.Done()
			}()
			w.processJob(ctx, jobID)
		}(claimedID)
	}
}

func (w *Worker) processJob(ctx context.Context, jobID uuid.UUID) {
	// Transition job to RUNNING in PostgreSQL.
	job, err := w.jobs.ClaimJob(ctx, jobID, w.id)
	if err != nil {
		w.logger.Error("failed to mark job RUNNING in store", "job_id", jobID, "error", err)
		return
	}

	if job == nil {
		w.logger.Info("job no longer eligible for execution (cancelled or already processed)", "job_id", jobID)
		if err := w.q.Ack(ctx, w.queueName, jobID); err != nil {
			w.logger.Warn("failed to ack inactive job", "job_id", jobID, "error", err)
		}
		return
	}

	w.mu.Lock()
	handler, ok := w.handlers[job.Type]
	w.mu.Unlock()

	if !ok {
		errMsg := fmt.Sprintf("no handler registered for job type %q", job.Type)
		w.logger.Error(errMsg, "job_id", job.ID)
		if err := w.jobs.MarkFailed(ctx, job.ID, errMsg); err != nil {
			w.logger.Error("failed to mark job failed", "job_id", job.ID, "error", err)
		}
		if w.workers != nil {
			_ = w.workers.IncrementFailed(ctx, w.id)
		}
		_ = w.q.Ack(ctx, w.queueName, job.ID)
		return
	}

	w.logger.Info("executing job", "job_id", job.ID, "type", job.Type, "attempt", job.Attempt)

	startTime := time.Now()
	jobCtx, cancel := context.WithTimeout(ctx, w.jobTimeout)
	execErr := handler(jobCtx, job.Payload)
	cancel()
	duration := time.Since(startTime)

	if execErr != nil {
		if errors.Is(execErr, context.DeadlineExceeded) {
			w.logger.Error("job execution timed out", "job_id", job.ID, "timeout", w.jobTimeout)
		} else {
			w.logger.Error("job execution failed", "job_id", job.ID, "error", execErr)
		}

		if w.workers != nil {
			_ = w.workers.IncrementFailed(ctx, w.id)
		}

		delay := CalculateBackoff(job.Attempt, DefaultBaseDelay, DefaultMaxDelay)
		nextRun := time.Now().Add(delay)

		newStatus, err := w.jobs.RecordFailure(ctx, job.ID, execErr.Error(), nextRun)
		if err != nil {
			w.logger.Error("failed to record job failure", "job_id", job.ID, "error", err)
			metrics.RecordJobFinished(job.Type, string(domain.JobStatusFailed), duration)
		} else {
			metrics.RecordJobFinished(job.Type, string(newStatus), duration)
			if newStatus == domain.JobStatusRetrying {
				w.logger.Info("job scheduled for retry", "job_id", job.ID, "attempt", job.Attempt, "retry_in", delay)
			} else {
				w.logger.Warn("job exhausted max attempts, moved to dead-letter", "job_id", job.ID, "attempt", job.Attempt)
			}
		}
	} else {
		w.logger.Info("job succeeded", "job_id", job.ID)
		if err := w.jobs.MarkSucceeded(ctx, job.ID); err != nil {
			w.logger.Error("failed to mark job succeeded", "job_id", job.ID, "error", err)
		}
		metrics.RecordJobFinished(job.Type, string(domain.JobStatusSucceeded), duration)
		if w.workers != nil {
			_ = w.workers.IncrementCompleted(ctx, w.id)
		}
	}

	// Always ack in Redis once the attempt is resolved in PostgreSQL
	if err := w.q.Ack(ctx, w.queueName, job.ID); err != nil {
		w.logger.Error("failed to ack job in queue", "job_id", job.ID, "error", err)
	}
}

func (w *Worker) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(w.heartbeatInterval)
	defer ticker.Stop()

	ttl := w.heartbeatInterval * 3

	// Initial heartbeat immediately
	if w.q != nil {
		if err := w.q.Heartbeat(ctx, w.id, ttl); err != nil {
			w.logger.Warn("initial redis heartbeat failed", "error", err)
		}
	}
	if w.workers != nil {
		_ = w.workers.Heartbeat(ctx, w.id)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if w.q != nil {
				if err := w.q.Heartbeat(ctx, w.id, ttl); err != nil {
					w.logger.Warn("worker redis heartbeat failed", "error", err)
				}
			}
			if w.workers != nil {
				if err := w.workers.Heartbeat(ctx, w.id); err != nil {
					w.logger.Warn("worker db heartbeat failed", "error", err)
				}
			}
		}
	}
}

func (w *Worker) staleRecoveryLoop(ctx context.Context) {
	if w.q == nil {
		return
	}
	ticker := time.NewTicker(w.visibilityTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := w.q.RequeueStale(ctx, w.queueName, domain.DefaultPriority)
			if err != nil {
				w.logger.Warn("requeuing stale jobs failed", "error", err)
			} else if count > 0 {
				w.logger.Info("requeued stale jobs from expired visibility timeout", "count", count)
			}
		}
	}
}

func (w *Worker) sleep(ctx context.Context) {
	t := time.NewTimer(w.pollInterval)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
