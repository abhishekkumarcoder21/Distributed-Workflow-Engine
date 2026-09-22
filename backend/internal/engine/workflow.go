package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/distributed-workflow-engine/backend/internal/domain"
	"github.com/distributed-workflow-engine/backend/internal/queue"
	"github.com/distributed-workflow-engine/backend/internal/store"
	"github.com/google/uuid"
)

type WorkflowEngine struct {
	workflows    *store.WorkflowStore
	jobs         *store.JobStore
	q            queue.Queue
	logger       *slog.Logger
	pollInterval time.Duration
}

func NewWorkflowEngine(workflows *store.WorkflowStore, jobs *store.JobStore, q queue.Queue, logger *slog.Logger) *WorkflowEngine {
	return &WorkflowEngine{
		workflows:    workflows,
		jobs:         jobs,
		q:            q,
		logger:       logger.With("component", "workflow_engine"),
		pollInterval: 1 * time.Second,
	}
}

// Start initiates a workflow: sets its status to RUNNING and schedules root tasks.
func (e *WorkflowEngine) Start(ctx context.Context, workflowID uuid.UUID) error {
	if err := e.workflows.UpdateStatus(ctx, workflowID, domain.WorkflowStatusRunning, nil); err != nil {
		return fmt.Errorf("setting workflow status to RUNNING: %w", err)
	}
	return e.AdvanceWorkflow(ctx, workflowID)
}

// AdvanceWorkflow inspects all tasks in the workflow:
// - Updates tasks whose backed Jobs have completed, failed, or cancelled.
// - Detects workflow failure if any task fails.
// - Detects workflow success if all tasks succeed.
// - Schedules any newly ready tasks (tasks whose dependencies all SUCCEEDED).
func (e *WorkflowEngine) AdvanceWorkflow(ctx context.Context, workflowID uuid.UUID) error {
	if e.workflows == nil {
		return fmt.Errorf("workflow store is not configured")
	}

	wf, err := e.workflows.Get(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("getting workflow: %w", err)
	}
	if wf == nil {
		return fmt.Errorf("workflow %s not found", workflowID)
	}

	if wf.Status.IsTerminal() {
		return nil
	}

	var failedTask *domain.WorkflowTask
	allSucceeded := true

	for i := range wf.Tasks {
		task := &wf.Tasks[i]

		// If task is currently RUNNING and has a JobID, check job status
		if task.Status == domain.WorkflowTaskStatusRunning && task.JobID != nil && e.jobs != nil {
			job, err := e.jobs.Get(ctx, *task.JobID)
			if err == nil && job != nil {
				switch job.Status {
				case domain.JobStatusSucceeded:
					_ = e.workflows.UpdateTask(ctx, task.ID, domain.WorkflowTaskStatusSucceeded, nil, nil)
					task.Status = domain.WorkflowTaskStatusSucceeded
				case domain.JobStatusDeadLetter, domain.JobStatusFailed:
					_ = e.workflows.UpdateTask(ctx, task.ID, domain.WorkflowTaskStatusFailed, nil, job.Error)
					task.Status = domain.WorkflowTaskStatusFailed
					task.Error = job.Error
					failedTask = task
				case domain.JobStatusCancelled:
					_ = e.workflows.UpdateTask(ctx, task.ID, domain.WorkflowTaskStatusCancelled, nil, nil)
					task.Status = domain.WorkflowTaskStatusCancelled
				}
			}
		}

		if task.Status == domain.WorkflowTaskStatusFailed {
			failedTask = task
			allSucceeded = false
		} else if task.Status != domain.WorkflowTaskStatusSucceeded {
			allSucceeded = false
		}
	}

	// 1. If any task failed, fail the whole workflow and cancel remaining pending tasks
	if failedTask != nil {
		errMsg := fmt.Sprintf("task %q failed", failedTask.Name)
		if failedTask.Error != nil && *failedTask.Error != "" {
			errMsg = fmt.Sprintf("task %q failed: %s", failedTask.Name, *failedTask.Error)
		}
		e.logger.Warn("workflow failed due to task failure", "workflow_id", workflowID, "task", failedTask.Name, "error", errMsg)
		_ = e.workflows.UpdateStatus(ctx, workflowID, domain.WorkflowStatusFailed, &errMsg)

		for _, t := range wf.Tasks {
			if t.Status == domain.WorkflowTaskStatusPending {
				_ = e.workflows.UpdateTask(ctx, t.ID, domain.WorkflowTaskStatusCancelled, nil, nil)
			}
		}
		return nil
	}

	// 2. If all tasks succeeded, succeed the workflow
	if allSucceeded && len(wf.Tasks) > 0 {
		e.logger.Info("workflow completed successfully", "workflow_id", workflowID)
		return e.workflows.UpdateStatus(ctx, workflowID, domain.WorkflowStatusSucceeded, nil)
	}

	// 3. Find newly ready tasks whose dependencies have all SUCCEEDED
	readyTasks, err := e.workflows.GetReadyTasks(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("getting ready tasks: %w", err)
	}

	for _, task := range readyTasks {
		idempotencyKey := fmt.Sprintf("wf:%s:task:%s", workflowID, task.Name)
		priority := domain.DefaultPriority
		jobReq := domain.CreateJobRequest{
			Type:           task.Type,
			Payload:        task.Payload,
			Priority:       &priority,
			IdempotencyKey: &idempotencyKey,
		}

		if e.jobs == nil {
			continue
		}

		job, created, err := e.jobs.Create(ctx, jobReq)
		if err != nil {
			e.logger.Error("failed to create job for task", "workflow_id", workflowID, "task", task.Name, "error", err)
			continue
		}

		if err := e.workflows.UpdateTask(ctx, task.ID, domain.WorkflowTaskStatusRunning, &job.ID, nil); err != nil {
			e.logger.Error("failed to update task status to RUNNING", "task_id", task.ID, "error", err)
			continue
		}

		if created && e.q != nil {
			if err := e.q.Enqueue(ctx, job); err != nil {
				e.logger.Error("failed to enqueue job for task", "job_id", job.ID, "error", err)
			}
		}
		e.logger.Info("scheduled workflow task", "workflow_id", workflowID, "task", task.Name, "job_id", job.ID)
	}

	return nil
}

// Run starts the background workflow monitoring loop.
func (e *WorkflowEngine) Run(ctx context.Context) {
	e.logger.Info("workflow engine poller started", "interval", e.pollInterval)
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("workflow engine poller stopped")
			return
		case <-ticker.C:
			e.pollRunningWorkflows(ctx)
		}
	}
}

func (e *WorkflowEngine) pollRunningWorkflows(ctx context.Context) {
	if e.workflows == nil {
		return
	}

	runningStatus := domain.WorkflowStatusRunning
	workflows, _, err := e.workflows.List(ctx, domain.WorkflowFilter{
		Status: &runningStatus,
		Limit:  50,
	})
	if err != nil {
		e.logger.Error("failed to list running workflows", "error", err)
		return
	}

	for _, wf := range workflows {
		if err := e.AdvanceWorkflow(ctx, wf.ID); err != nil {
			e.logger.Error("failed to advance workflow", "workflow_id", wf.ID, "error", err)
		}
	}
}
