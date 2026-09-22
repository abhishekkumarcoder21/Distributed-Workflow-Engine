package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/distributed-workflow-engine/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type JobStore struct {
	pool *pgxpool.Pool
}

func NewJobStore(pool *pgxpool.Pool) *JobStore {
	return &JobStore{pool: pool}
}

func (s *JobStore) Create(ctx context.Context, req domain.CreateJobRequest) (*domain.Job, bool, error) {
	priority := domain.DefaultPriority
	if req.Priority != nil {
		priority = *req.Priority
	}
	maxAttempts := domain.DefaultMaxAttempts
	if req.MaxAttempts != nil {
		maxAttempts = *req.MaxAttempts
	}
	queue := domain.DefaultQueue
	if req.Queue != nil {
		queue = *req.Queue
	}

	payload := req.Payload
	if payload == nil {
		payload = json.RawMessage("{}")
	}

	if req.IdempotencyKey != nil {
		// Fast-path lookup
		existing, err := s.findByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err != nil {
			return nil, false, fmt.Errorf("checking idempotency key: %w", err)
		}
		if existing != nil {
			return existing, false, nil
		}

		// Insert with ON CONFLICT to protect against concurrent duplicate submissions
		job := &domain.Job{}
		err = s.pool.QueryRow(ctx, `
			INSERT INTO jobs (type, payload, priority, max_attempts, idempotency_key, queue)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (idempotency_key) DO NOTHING
			RETURNING id, type, payload, status, priority, max_attempts, attempt,
			          idempotency_key, error, worker_id, queue, run_at,
			          started_at, completed_at, created_at, updated_at`,
			req.Type, payload, priority, maxAttempts, req.IdempotencyKey, queue,
		).Scan(
			&job.ID, &job.Type, &job.Payload, &job.Status, &job.Priority,
			&job.MaxAttempts, &job.Attempt, &job.IdempotencyKey, &job.Error,
			&job.WorkerID, &job.Queue, &job.RunAt, &job.StartedAt,
			&job.CompletedAt, &job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Concurrent insert won the race; fetch the existing job
				existing, err := s.findByIdempotencyKey(ctx, *req.IdempotencyKey)
				if err != nil {
					return nil, false, fmt.Errorf("fetching conflicting job: %w", err)
				}
				if existing != nil {
					return existing, false, nil
				}
			}
			return nil, false, fmt.Errorf("inserting job: %w", err)
		}
		return job, true, nil
	}

	job := &domain.Job{}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO jobs (type, payload, priority, max_attempts, queue)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, type, payload, status, priority, max_attempts, attempt,
		          idempotency_key, error, worker_id, queue, run_at,
		          started_at, completed_at, created_at, updated_at`,
		req.Type, payload, priority, maxAttempts, queue,
	).Scan(
		&job.ID, &job.Type, &job.Payload, &job.Status, &job.Priority,
		&job.MaxAttempts, &job.Attempt, &job.IdempotencyKey, &job.Error,
		&job.WorkerID, &job.Queue, &job.RunAt, &job.StartedAt,
		&job.CompletedAt, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, false, fmt.Errorf("inserting job: %w", err)
	}
	return job, true, nil
}

func (s *JobStore) Get(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	job := &domain.Job{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, type, payload, status, priority, max_attempts, attempt,
		       idempotency_key, error, worker_id, queue, run_at,
		       started_at, completed_at, created_at, updated_at
		FROM jobs WHERE id = $1`, id,
	).Scan(
		&job.ID, &job.Type, &job.Payload, &job.Status, &job.Priority,
		&job.MaxAttempts, &job.Attempt, &job.IdempotencyKey, &job.Error,
		&job.WorkerID, &job.Queue, &job.RunAt, &job.StartedAt,
		&job.CompletedAt, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting job: %w", err)
	}
	return job, nil
}

func (s *JobStore) List(ctx context.Context, filter domain.JobFilter) ([]*domain.Job, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}

	where, args := buildWhereClause(filter)

	var total int
	countQuery := "SELECT COUNT(*) FROM jobs" + where
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting jobs: %w", err)
	}

	query := `SELECT id, type, payload, status, priority, max_attempts, attempt,
	                 idempotency_key, error, worker_id, queue, run_at,
	                 started_at, completed_at, created_at, updated_at
	          FROM jobs` + where +
		fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d OFFSET %d", filter.Limit, filter.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing jobs: %w", err)
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
			return nil, 0, fmt.Errorf("scanning job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, total, nil
}

// Claim atomically picks the highest-priority queued job and assigns it to a worker.
// Uses SELECT ... FOR UPDATE SKIP LOCKED to avoid contention between workers.
func (s *JobStore) Claim(ctx context.Context, workerID string) (*domain.Job, error) {
	job := &domain.Job{}
	now := time.Now()
	err := s.pool.QueryRow(ctx, `
		UPDATE jobs SET
			status = 'RUNNING',
			worker_id = $1,
			attempt = attempt + 1,
			started_at = $2,
			updated_at = $2
		WHERE id = (
			SELECT id FROM jobs
			WHERE status = 'QUEUED' AND run_at <= $2
			ORDER BY priority ASC, run_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, type, payload, status, priority, max_attempts, attempt,
		          idempotency_key, error, worker_id, queue, run_at,
		          started_at, completed_at, created_at, updated_at`,
		workerID, now,
	).Scan(
		&job.ID, &job.Type, &job.Payload, &job.Status, &job.Priority,
		&job.MaxAttempts, &job.Attempt, &job.IdempotencyKey, &job.Error,
		&job.WorkerID, &job.Queue, &job.RunAt, &job.StartedAt,
		&job.CompletedAt, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("claiming job: %w", err)
	}
	return job, nil
}

// ClaimJob transitions a specific job (identified by ID, e.g. from Redis) to RUNNING.
// It returns nil, nil if the job is no longer in QUEUED or RETRYING status (e.g. cancelled).
func (s *JobStore) ClaimJob(ctx context.Context, id uuid.UUID, workerID string) (*domain.Job, error) {
	job := &domain.Job{}
	now := time.Now()
	err := s.pool.QueryRow(ctx, `
		UPDATE jobs SET
			status = 'RUNNING',
			worker_id = $1,
			attempt = attempt + 1,
			started_at = $2,
			updated_at = $2
		WHERE id = $3 AND status IN ('QUEUED', 'RETRYING')
		RETURNING id, type, payload, status, priority, max_attempts, attempt,
		          idempotency_key, error, worker_id, queue, run_at,
		          started_at, completed_at, created_at, updated_at`,
		workerID, now, id,
	).Scan(
		&job.ID, &job.Type, &job.Payload, &job.Status, &job.Priority,
		&job.MaxAttempts, &job.Attempt, &job.IdempotencyKey, &job.Error,
		&job.WorkerID, &job.Queue, &job.RunAt, &job.StartedAt,
		&job.CompletedAt, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("claiming job %s: %w", id, err)
	}
	return job, nil
}

// RevertToQueued resets a job's status from RUNNING back to QUEUED and clears its worker_id.
func (s *JobStore) RevertToQueued(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE jobs SET status = 'QUEUED', worker_id = NULL, updated_at = $2
		WHERE id = $1 AND status = 'RUNNING'`, id, now)
	return err
}

// RecordFailure records a failure for a job. If attempt < max_attempts, the job transitions
// to RETRYING with the calculated nextRunAt. If attempts are exhausted, it transitions to DEAD_LETTER.
func (s *JobStore) RecordFailure(ctx context.Context, id uuid.UUID, jobErr string, nextRunAt time.Time) (domain.JobStatus, error) {
	now := time.Now()
	var status domain.JobStatus
	err := s.pool.QueryRow(ctx, `
		UPDATE jobs SET
			status = CASE
				WHEN attempt < max_attempts THEN 'RETRYING'
				ELSE 'DEAD_LETTER'
			END,
			run_at = CASE
				WHEN attempt < max_attempts THEN $2
				ELSE run_at
			END,
			completed_at = CASE
				WHEN attempt >= max_attempts THEN $3
				ELSE NULL
			END,
			error = $4,
			worker_id = NULL,
			updated_at = $3
		WHERE id = $1
		RETURNING status`,
		id, nextRunAt, now, jobErr,
	).Scan(&status)
	if err != nil {
		return "", fmt.Errorf("recording failure for job %s: %w", id, err)
	}
	return status, nil
}

// RequeueDueRetries finds jobs in RETRYING state whose run_at <= now,
// transitions them to QUEUED, and returns them for enqueuing into Redis.
func (s *JobStore) RequeueDueRetries(ctx context.Context, limit int) ([]*domain.Job, error) {
	if limit <= 0 {
		limit = 50
	}
	now := time.Now()
	query := `
		UPDATE jobs SET
			status = 'QUEUED',
			updated_at = $1
		WHERE id = ANY(
			SELECT id FROM jobs
			WHERE status = 'RETRYING' AND run_at <= $1
			ORDER BY run_at ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, type, payload, status, priority, max_attempts, attempt,
		          idempotency_key, error, worker_id, queue, run_at,
		          started_at, completed_at, created_at, updated_at`

	rows, err := s.pool.Query(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("requeuing due retries: %w", err)
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
			return nil, fmt.Errorf("scanning requeued job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// ManualRetry resets a DEAD_LETTER or FAILED job to QUEUED with attempt count reset to 0.
func (s *JobStore) ManualRetry(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	now := time.Now()
	job := &domain.Job{}
	err := s.pool.QueryRow(ctx, `
		UPDATE jobs SET
			status = 'QUEUED',
			attempt = 0,
			error = NULL,
			worker_id = NULL,
			run_at = $2,
			completed_at = NULL,
			updated_at = $2
		WHERE id = $1 AND status IN ('DEAD_LETTER', 'FAILED')
		RETURNING id, type, payload, status, priority, max_attempts, attempt,
		          idempotency_key, error, worker_id, queue, run_at,
		          started_at, completed_at, created_at, updated_at`,
		id, now,
	).Scan(
		&job.ID, &job.Type, &job.Payload, &job.Status, &job.Priority,
		&job.MaxAttempts, &job.Attempt, &job.IdempotencyKey, &job.Error,
		&job.WorkerID, &job.Queue, &job.RunAt, &job.StartedAt,
		&job.CompletedAt, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJobNotRetryable
		}
		return nil, fmt.Errorf("retrying job %s: %w", id, err)
	}
	return job, nil
}

var ErrJobNotRetryable = errors.New("job cannot be retried (must be in DEAD_LETTER or FAILED state)")

func (s *JobStore) MarkSucceeded(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE jobs SET status = 'SUCCEEDED', completed_at = $2, updated_at = $2, error = NULL
		WHERE id = $1`, id, now)
	return err
}

func (s *JobStore) MarkFailed(ctx context.Context, id uuid.UUID, jobErr string) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE jobs SET status = 'FAILED', completed_at = $2, updated_at = $2, error = $3
		WHERE id = $1`, id, now, jobErr)
	return err
}

func (s *JobStore) Cancel(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result, err := s.pool.Exec(ctx, `
		UPDATE jobs SET status = 'CANCELLED', completed_at = $2, updated_at = $2
		WHERE id = $1 AND status IN ('QUEUED', 'RUNNING')`, id, now)
	if err != nil {
		return fmt.Errorf("cancelling job: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrJobNotCancellable
	}
	return nil
}

// StatusCounts returns a count of jobs grouped by status, for dashboard summaries.
func (s *JobStore) StatusCounts(ctx context.Context) (map[domain.JobStatus]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT status, COUNT(*) FROM jobs GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[domain.JobStatus]int)
	for rows.Next() {
		var status domain.JobStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, nil
}

func (s *JobStore) findByIdempotencyKey(ctx context.Context, key string) (*domain.Job, error) {
	job := &domain.Job{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, type, payload, status, priority, max_attempts, attempt,
		       idempotency_key, error, worker_id, queue, run_at,
		       started_at, completed_at, created_at, updated_at
		FROM jobs WHERE idempotency_key = $1`, key,
	).Scan(
		&job.ID, &job.Type, &job.Payload, &job.Status, &job.Priority,
		&job.MaxAttempts, &job.Attempt, &job.IdempotencyKey, &job.Error,
		&job.WorkerID, &job.Queue, &job.RunAt, &job.StartedAt,
		&job.CompletedAt, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

var ErrJobNotCancellable = errors.New("job cannot be cancelled (not in QUEUED or RUNNING state)")

func buildWhereClause(f domain.JobFilter) (string, []any) {
	var conditions []string
	var args []any
	argIdx := 1

	if f.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *f.Status)
		argIdx++
	}
	if f.Type != nil {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *f.Type)
		argIdx++
	}
	if f.Queue != nil {
		conditions = append(conditions, fmt.Sprintf("queue = $%d", argIdx))
		args = append(args, *f.Queue)
		argIdx++
	}
	if f.Priority != nil {
		conditions = append(conditions, fmt.Sprintf("priority = $%d", argIdx))
		args = append(args, *f.Priority)
		argIdx++
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}
