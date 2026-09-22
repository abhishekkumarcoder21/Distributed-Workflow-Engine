package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/distributed-workflow-engine/backend/internal/domain"
	"github.com/distributed-workflow-engine/backend/internal/engine"
	"github.com/distributed-workflow-engine/backend/internal/metrics"
	"github.com/distributed-workflow-engine/backend/internal/queue"
	"github.com/distributed-workflow-engine/backend/internal/store"
	"github.com/google/uuid"
)

type Handler struct {
	jobs           *store.JobStore
	workers        *store.WorkerStore
	workflows      *store.WorkflowStore
	workflowEngine *engine.WorkflowEngine
	q              queue.Queue
	logger         *slog.Logger
}

func NewHandler(jobs *store.JobStore, workers *store.WorkerStore, workflows *store.WorkflowStore, q queue.Queue, workflowEngine *engine.WorkflowEngine, logger *slog.Logger) *Handler {
	return &Handler{
		jobs:           jobs,
		workers:        workers,
		workflows:      workflows,
		q:              q,
		workflowEngine: workflowEngine,
		logger:         logger,
	}
}

func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Type == "" {
		respondError(w, http.StatusBadRequest, "type is required")
		return
	}
	if req.Priority != nil && (*req.Priority < 1 || *req.Priority > 3) {
		respondError(w, http.StatusBadRequest, "priority must be 1 (high), 2 (medium), or 3 (low)")
		return
	}
	if req.MaxAttempts != nil && *req.MaxAttempts < 1 {
		respondError(w, http.StatusBadRequest, "max_attempts must be at least 1")
		return
	}

	// Support Idempotency-Key HTTP header if not present in JSON body
	if req.IdempotencyKey == nil {
		if key := r.Header.Get("Idempotency-Key"); key != "" {
			req.IdempotencyKey = &key
		}
	}

	// Validate idempotency key length if provided
	if req.IdempotencyKey != nil {
		if len(*req.IdempotencyKey) == 0 || len(*req.IdempotencyKey) > 128 {
			respondError(w, http.StatusBadRequest, "idempotency_key must be between 1 and 128 characters")
			return
		}
	}

	job, created, err := h.jobs.Create(r.Context(), req)
	if err != nil {
		h.logger.Error("failed to create job", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	if created {
		metrics.RecordJobCreated(job.Type)
		if h.q != nil {
			if err := h.q.Enqueue(r.Context(), job); err != nil {
				h.logger.Error("failed to enqueue job in redis", "id", job.ID, "error", err)
			}
		}
		h.logger.Info("job created", "id", job.ID, "type", job.Type)
		respondJSON(w, http.StatusCreated, job)
		return
	}

	// Idempotent replay: job already exists, return 200 OK with X-Idempotent-Replay header
	w.Header().Set("X-Idempotent-Replay", "true")
	h.logger.Info("idempotent job replay", "id", job.ID, "key", *req.IdempotencyKey)
	respondJSON(w, http.StatusOK, job)
}

func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	job, err := h.jobs.Get(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get job", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get job")
		return
	}
	if job == nil {
		respondError(w, http.StatusNotFound, "job not found")
		return
	}

	respondJSON(w, http.StatusOK, job)
}

func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	filter := domain.JobFilter{
		Limit:  parseQueryInt(r, "limit", 50),
		Offset: parseQueryInt(r, "offset", 0),
	}

	if s := r.URL.Query().Get("status"); s != "" {
		status := domain.JobStatus(s)
		filter.Status = &status
	}
	if t := r.URL.Query().Get("type"); t != "" {
		filter.Type = &t
	}
	if q := r.URL.Query().Get("queue"); q != "" {
		filter.Queue = &q
	}
	if p := r.URL.Query().Get("priority"); p != "" {
		if pv, err := strconv.Atoi(p); err == nil {
			filter.Priority = &pv
		}
	}

	jobs, total, err := h.jobs.List(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list jobs", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}

	respondList(w, jobs, total, filter.Limit, filter.Offset)
}

func (h *Handler) CancelJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	if err := h.jobs.Cancel(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrJobNotCancellable) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		h.logger.Error("failed to cancel job", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to cancel job")
		return
	}

	if h.q != nil {
		_ = h.q.Ack(r.Context(), domain.DefaultQueue, id)
	}

	// Return the updated job.
	job, err := h.jobs.Get(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get cancelled job", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to retrieve cancelled job")
		return
	}

	h.logger.Info("job cancelled", "id", id)
	respondJSON(w, http.StatusOK, job)
}

func (h *Handler) RetryJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	job, err := h.jobs.ManualRetry(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrJobNotRetryable) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		h.logger.Error("failed to manually retry job", "id", id, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to retry job")
		return
	}

	// Enqueue the job back into Redis
	if h.q != nil {
		if err := h.q.Enqueue(r.Context(), job); err != nil {
			h.logger.Error("failed to enqueue retried job into redis", "id", id, "error", err)
		}
	}

	h.logger.Info("job manually retried", "id", id)
	respondJSON(w, http.StatusOK, job)
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	counts, err := h.jobs.StatusCounts(r.Context())
	if err != nil {
		h.logger.Error("failed to get stats", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	// Count active workers
	var activeWorkers int
	if h.workers != nil {
		workers, err := h.workers.List(r.Context())
		if err == nil {
			for _, wk := range workers {
				if wk.Status == domain.WorkerStatusActive {
					activeWorkers++
				}
			}
		}
	}

	// Build the structured response the frontend expects
	total := 0
	for _, c := range counts {
		total += c
	}

	stats := map[string]int{
		"total_jobs":       total,
		"queued_jobs":      counts[domain.JobStatusQueued],
		"running_jobs":     counts[domain.JobStatusRunning],
		"succeeded_jobs":   counts[domain.JobStatusSucceeded],
		"failed_jobs":      counts[domain.JobStatusFailed],
		"retrying_jobs":    counts[domain.JobStatusRetrying],
		"dead_letter_jobs": counts[domain.JobStatusDeadLetter],
		"cancelled_jobs":   counts[domain.JobStatusCancelled],
		"active_workers":   activeWorkers,
	}

	respondJSON(w, http.StatusOK, stats)
}

func (h *Handler) ListWorkers(w http.ResponseWriter, r *http.Request) {
	if h.workers == nil {
		respondJSON(w, http.StatusOK, []*domain.Worker{})
		return
	}

	workers, err := h.workers.List(r.Context())
	if err != nil {
		h.logger.Error("failed to list workers", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to list workers")
		return
	}

	respondJSON(w, http.StatusOK, workers)
}

func (h *Handler) GetQueueStats(w http.ResponseWriter, r *http.Request) {
	if h.q == nil {
		respondJSON(w, http.StatusOK, map[string]string{"message": "queue not initialized"})
		return
	}

	queueName := r.URL.Query().Get("queue")
	if queueName == "" {
		queueName = domain.DefaultQueue
	}

	stats, err := h.q.GetQueueStats(r.Context(), queueName)
	if err != nil {
		h.logger.Error("failed to get queue stats", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get queue stats")
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func parseQueryInt(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func (h *Handler) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "workflow name is required")
		return
	}

	if err := domain.ValidateDAG(req.Tasks); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	wf, err := h.workflows.Create(r.Context(), req)
	if err != nil {
		h.logger.Error("failed to create workflow", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to create workflow")
		return
	}

	// Start workflow execution immediately
	if h.workflowEngine != nil {
		metrics.RecordWorkflow(string(domain.WorkflowStatusRunning))
		if err := h.workflowEngine.Start(r.Context(), wf.ID); err != nil {
			h.logger.Error("failed to start workflow", "workflow_id", wf.ID, "error", err)
		}
	}

	h.logger.Info("workflow created and started", "id", wf.ID, "name", wf.Name, "tasks", len(wf.Tasks))
	respondJSON(w, http.StatusCreated, wf)
}

func (h *Handler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid workflow ID")
		return
	}

	wf, err := h.workflows.Get(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get workflow", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get workflow")
		return
	}
	if wf == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	respondJSON(w, http.StatusOK, wf)
}

func (h *Handler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	filter := domain.WorkflowFilter{
		Limit:  parseQueryInt(r, "limit", 20),
		Offset: parseQueryInt(r, "offset", 0),
	}

	if s := r.URL.Query().Get("status"); s != "" {
		st := domain.WorkflowStatus(s)
		filter.Status = &st
	}

	workflows, total, err := h.workflows.List(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list workflows", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to list workflows")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"workflows": workflows,
		"total":     total,
		"limit":     filter.Limit,
		"offset":    filter.Offset,
	})
}

func (h *Handler) CancelWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid workflow ID")
		return
	}

	if err := h.workflows.Cancel(r.Context(), id); err != nil {
		h.logger.Error("failed to cancel workflow", "id", id, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to cancel workflow")
		return
	}

	h.logger.Info("workflow cancelled", "id", id)
	respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

