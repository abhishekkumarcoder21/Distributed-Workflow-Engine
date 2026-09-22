package store

import (
	"context"
	"fmt"
	"time"

	"github.com/distributed-workflow-engine/backend/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkerStore struct {
	pool *pgxpool.Pool
}

func NewWorkerStore(pool *pgxpool.Pool) *WorkerStore {
	return &WorkerStore{pool: pool}
}

// Register inserts or reactivates a worker in PostgreSQL.
func (s *WorkerStore) Register(ctx context.Context, id string) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO workers (id, status, last_heartbeat, started_at, updated_at)
		VALUES ($1, 'ACTIVE', $2, $2, $2)
		ON CONFLICT (id) DO UPDATE SET
			status = 'ACTIVE',
			last_heartbeat = $2,
			updated_at = $2`,
		id, now,
	)
	if err != nil {
		return fmt.Errorf("registering worker %s: %w", id, err)
	}
	return nil
}

// Heartbeat updates the last_heartbeat timestamp for a worker.
func (s *WorkerStore) Heartbeat(ctx context.Context, id string) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE workers SET last_heartbeat = $2, updated_at = $2
		WHERE id = $1`,
		id, now,
	)
	if err != nil {
		return fmt.Errorf("updating heartbeat for worker %s: %w", id, err)
	}
	return nil
}

// SetStatus updates a worker's status (ACTIVE, DRAINING, OFFLINE).
func (s *WorkerStore) SetStatus(ctx context.Context, id string, status domain.WorkerStatus) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE workers SET status = $2, updated_at = $3
		WHERE id = $1`,
		id, status, now,
	)
	if err != nil {
		return fmt.Errorf("updating status for worker %s: %w", id, err)
	}
	return nil
}

// IncrementCompleted increments the worker's completed_count.
func (s *WorkerStore) IncrementCompleted(ctx context.Context, id string) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE workers SET completed_count = completed_count + 1, updated_at = $2
		WHERE id = $1`,
		id, now,
	)
	return err
}

// IncrementFailed increments the worker's failed_count.
func (s *WorkerStore) IncrementFailed(ctx context.Context, id string) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE workers SET failed_count = failed_count + 1, updated_at = $2
		WHERE id = $1`,
		id, now,
	)
	return err
}

// List returns all registered workers ordered by last heartbeat.
func (s *WorkerStore) List(ctx context.Context) ([]*domain.Worker, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, status, last_heartbeat, started_at, completed_count, failed_count, created_at, updated_at
		FROM workers
		ORDER BY last_heartbeat DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing workers: %w", err)
	}
	defer rows.Close()

	var workers []*domain.Worker
	for rows.Next() {
		w := &domain.Worker{}
		if err := rows.Scan(
			&w.ID, &w.Status, &w.LastHeartbeat, &w.StartedAt,
			&w.CompletedCount, &w.FailedCount, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning worker: %w", err)
		}
		workers = append(workers, w)
	}
	return workers, nil
}

// FindDeadWorkers returns workers whose status is ACTIVE but last_heartbeat is older than timeout.
func (s *WorkerStore) FindDeadWorkers(ctx context.Context, timeout time.Duration) ([]*domain.Worker, error) {
	cutoff := time.Now().Add(-timeout)
	rows, err := s.pool.Query(ctx, `
		SELECT id, status, last_heartbeat, started_at, completed_count, failed_count, created_at, updated_at
		FROM workers
		WHERE status = 'ACTIVE' AND last_heartbeat < $1`,
		cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("finding dead workers: %w", err)
	}
	defer rows.Close()

	var workers []*domain.Worker
	for rows.Next() {
		w := &domain.Worker{}
		if err := rows.Scan(
			&w.ID, &w.Status, &w.LastHeartbeat, &w.StartedAt,
			&w.CompletedCount, &w.FailedCount, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning dead worker: %w", err)
		}
		workers = append(workers, w)
	}
	return workers, nil
}

// RecoverDeadWorkerJobs finds all RUNNING jobs associated with a dead worker,
// resets their status to QUEUED, clears their worker_id, and returns them for re-enqueuing.
func (s *WorkerStore) RecoverDeadWorkerJobs(ctx context.Context, deadWorkerID string) ([]*domain.Job, error) {
	now := time.Now()
	rows, err := s.pool.Query(ctx, `
		UPDATE jobs SET
			status = 'QUEUED',
			worker_id = NULL,
			updated_at = $2
		WHERE worker_id = $1 AND status = 'RUNNING'
		RETURNING id, type, payload, status, priority, max_attempts, attempt,
		          idempotency_key, error, worker_id, queue, run_at,
		          started_at, completed_at, created_at, updated_at`,
		deadWorkerID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("recovering jobs for dead worker %s: %w", deadWorkerID, err)
	}
	defer rows.Close()

	var jobs []*domain.Job
	for rows.Next() {
		job := &domain.Job{}
		if err := rows.Scan(
			&job.ID, &job.Type, &job.Payload, &job.Status, &job.Priority,
			&job.MaxAttempts, &job.Attempt, &job.IdempotencyKey, &job.Error,
			&job.WorkerID, &job.Queue, &job.RunAt, &job.StartedAt,
			&job.CompletedAt, &job.CreatedAt, &job.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning recovered job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}
