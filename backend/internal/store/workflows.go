package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/distributed-workflow-engine/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkflowStore struct {
	pool *pgxpool.Pool
}

func NewWorkflowStore(pool *pgxpool.Pool) *WorkflowStore {
	return &WorkflowStore{pool: pool}
}

// Create creates a workflow and its tasks within a single database transaction.
func (s *WorkflowStore) Create(ctx context.Context, req domain.CreateWorkflowRequest) (*domain.WorkflowWithTasks, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	wf := domain.Workflow{}
	err = tx.QueryRow(ctx, `
		INSERT INTO workflows (name, status)
		VALUES ($1, $2)
		RETURNING id, name, status, error, created_at, started_at, completed_at, updated_at
	`, req.Name, domain.WorkflowStatusPending).Scan(
		&wf.ID, &wf.Name, &wf.Status, &wf.Error,
		&wf.CreatedAt, &wf.StartedAt, &wf.CompletedAt, &wf.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting workflow: %w", err)
	}

	tasks := make([]domain.WorkflowTask, 0, len(req.Tasks))
	for _, tr := range req.Tasks {
		payload := tr.Payload
		if payload == nil {
			payload = json.RawMessage("{}")
		}
		deps := tr.Dependencies
		if deps == nil {
			deps = []string{}
		}

		t := domain.WorkflowTask{}
		err = tx.QueryRow(ctx, `
			INSERT INTO workflow_tasks (workflow_id, name, type, payload, status, dependencies)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, workflow_id, job_id, name, type, payload, status, dependencies, error, created_at, started_at, completed_at, updated_at
		`, wf.ID, tr.Name, tr.Type, payload, domain.WorkflowTaskStatusPending, deps).Scan(
			&t.ID, &t.WorkflowID, &t.JobID, &t.Name, &t.Type, &t.Payload,
			&t.Status, &t.Dependencies, &t.Error,
			&t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting task %q: %w", tr.Name, err)
		}
		tasks = append(tasks, t)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing workflow transaction: %w", err)
	}

	return &domain.WorkflowWithTasks{
		Workflow: wf,
		Tasks:    tasks,
	}, nil
}

// Get retrieves a workflow and all its tasks by ID.
func (s *WorkflowStore) Get(ctx context.Context, id uuid.UUID) (*domain.WorkflowWithTasks, error) {
	wf := domain.Workflow{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, status, error, created_at, started_at, completed_at, updated_at
		FROM workflows
		WHERE id = $1
	`, id).Scan(
		&wf.ID, &wf.Name, &wf.Status, &wf.Error,
		&wf.CreatedAt, &wf.StartedAt, &wf.CompletedAt, &wf.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying workflow: %w", err)
	}

	tasks, err := s.GetTasksByWorkflowID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("querying workflow tasks: %w", err)
	}

	return &domain.WorkflowWithTasks{
		Workflow: wf,
		Tasks:    tasks,
	}, nil
}

// GetTasksByWorkflowID returns all tasks belonging to a workflow.
func (s *WorkflowStore) GetTasksByWorkflowID(ctx context.Context, workflowID uuid.UUID) ([]domain.WorkflowTask, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, workflow_id, job_id, name, type, payload, status, dependencies, error, created_at, started_at, completed_at, updated_at
		FROM workflow_tasks
		WHERE workflow_id = $1
		ORDER BY created_at ASC
	`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("querying tasks: %w", err)
	}
	defer rows.Close()

	var tasks []domain.WorkflowTask
	for rows.Next() {
		var t domain.WorkflowTask
		if err := rows.Scan(
			&t.ID, &t.WorkflowID, &t.JobID, &t.Name, &t.Type, &t.Payload,
			&t.Status, &t.Dependencies, &t.Error,
			&t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tasks: %w", err)
	}
	if tasks == nil {
		tasks = []domain.WorkflowTask{}
	}
	return tasks, nil
}

// GetReadyTasks finds tasks in PENDING status whose dependencies are all SUCCEEDED.
func (s *WorkflowStore) GetReadyTasks(ctx context.Context, workflowID uuid.UUID) ([]domain.WorkflowTask, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.workflow_id, t.job_id, t.name, t.type, t.payload, t.status, t.dependencies, t.error, t.created_at, t.started_at, t.completed_at, t.updated_at
		FROM workflow_tasks t
		WHERE t.workflow_id = $1 AND t.status = 'PENDING'
		  AND NOT EXISTS (
		      SELECT 1
		      FROM unnest(t.dependencies) dep_name
		      JOIN workflow_tasks dep ON dep.workflow_id = t.workflow_id AND dep.name = dep_name
		      WHERE dep.status != 'SUCCEEDED'
		  )
		ORDER BY t.created_at ASC
	`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("querying ready tasks: %w", err)
	}
	defer rows.Close()

	var tasks []domain.WorkflowTask
	for rows.Next() {
		var t domain.WorkflowTask
		if err := rows.Scan(
			&t.ID, &t.WorkflowID, &t.JobID, &t.Name, &t.Type, &t.Payload,
			&t.Status, &t.Dependencies, &t.Error,
			&t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning ready task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating ready tasks: %w", err)
	}
	if tasks == nil {
		tasks = []domain.WorkflowTask{}
	}
	return tasks, nil
}

// GetTaskByJobID finds the workflow task associated with a specific Job ID.
func (s *WorkflowStore) GetTaskByJobID(ctx context.Context, jobID uuid.UUID) (*domain.WorkflowTask, error) {
	t := &domain.WorkflowTask{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, workflow_id, job_id, name, type, payload, status, dependencies, error, created_at, started_at, completed_at, updated_at
		FROM workflow_tasks
		WHERE job_id = $1
	`, jobID).Scan(
		&t.ID, &t.WorkflowID, &t.JobID, &t.Name, &t.Type, &t.Payload,
		&t.Status, &t.Dependencies, &t.Error,
		&t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying task by job_id: %w", err)
	}
	return t, nil
}

// UpdateStatus updates the workflow status. Sets started_at when RUNNING and completed_at when terminal.
func (s *WorkflowStore) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.WorkflowStatus, errStr *string) error {
	query := `
		UPDATE workflows
		SET status = $2,
		    error = COALESCE($3, error),
		    started_at = CASE WHEN $2 = 'RUNNING' AND started_at IS NULL THEN NOW() ELSE started_at END,
		    completed_at = CASE WHEN $2 IN ('SUCCEEDED', 'FAILED', 'CANCELLED') THEN NOW() ELSE completed_at END,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query, id, status, errStr)
	if err != nil {
		return fmt.Errorf("updating workflow status: %w", err)
	}
	return nil
}

// UpdateTask updates the status and job_id of a workflow task.
func (s *WorkflowStore) UpdateTask(ctx context.Context, taskID uuid.UUID, status domain.WorkflowTaskStatus, jobID *uuid.UUID, errStr *string) error {
	query := `
		UPDATE workflow_tasks
		SET status = $2,
		    job_id = COALESCE($3, job_id),
		    error = COALESCE($4, error),
		    started_at = CASE WHEN $2 = 'RUNNING' AND started_at IS NULL THEN NOW() ELSE started_at END,
		    completed_at = CASE WHEN $2 IN ('SUCCEEDED', 'FAILED', 'CANCELLED') THEN NOW() ELSE completed_at END,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query, taskID, status, jobID, errStr)
	if err != nil {
		return fmt.Errorf("updating workflow task: %w", err)
	}
	return nil
}

// List returns a paginated list of workflows, optionally filtered by status.
func (s *WorkflowStore) List(ctx context.Context, filter domain.WorkflowFilter) ([]domain.Workflow, int, error) {
	var whereClauses []string
	var args []any
	argIdx := 1

	if filter.Status != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM workflows %s", whereSQL)
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting workflows: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, name, status, error, created_at, started_at, completed_at, updated_at
		FROM workflows
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying workflows: %w", err)
	}
	defer rows.Close()

	var workflows []domain.Workflow
	for rows.Next() {
		var wf domain.Workflow
		if err := rows.Scan(
			&wf.ID, &wf.Name, &wf.Status, &wf.Error,
			&wf.CreatedAt, &wf.StartedAt, &wf.CompletedAt, &wf.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning workflow: %w", err)
		}
		workflows = append(workflows, wf)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating workflows: %w", err)
	}

	if workflows == nil {
		workflows = []domain.Workflow{}
	}

	return workflows, total, nil
}

// Cancel cancels a workflow and all of its pending or running tasks.
func (s *WorkflowStore) Cancel(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting cancel transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Update workflow
	_, err = tx.Exec(ctx, `
		UPDATE workflows
		SET status = 'CANCELLED', completed_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status IN ('PENDING', 'RUNNING')
	`, id)
	if err != nil {
		return fmt.Errorf("cancelling workflow: %w", err)
	}

	// Update pending / running tasks
	_, err = tx.Exec(ctx, `
		UPDATE workflow_tasks
		SET status = 'CANCELLED', completed_at = NOW(), updated_at = NOW()
		WHERE workflow_id = $1 AND status IN ('PENDING', 'RUNNING')
	`, id)
	if err != nil {
		return fmt.Errorf("cancelling workflow tasks: %w", err)
	}

	return tx.Commit(ctx)
}
