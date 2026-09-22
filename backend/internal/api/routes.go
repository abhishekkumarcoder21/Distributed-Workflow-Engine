package api

import (
	"log/slog"
	"net/http"

	"github.com/distributed-workflow-engine/backend/internal/engine"
	"github.com/distributed-workflow-engine/backend/internal/queue"
	"github.com/distributed-workflow-engine/backend/internal/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(jobs *store.JobStore, workers *store.WorkerStore, workflows *store.WorkflowStore, q queue.Queue, workflowEngine *engine.WorkflowEngine, logger *slog.Logger) http.Handler {
	h := NewHandler(jobs, workers, workflows, q, workflowEngine, logger)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", h.HealthCheck)
	mux.Handle("GET /metrics", promhttp.Handler())

	// Jobs endpoints
	mux.HandleFunc("POST /api/v1/jobs", h.CreateJob)
	mux.HandleFunc("GET /api/v1/jobs", h.ListJobs)
	mux.HandleFunc("GET /api/v1/jobs/{id}", h.GetJob)
	mux.HandleFunc("POST /api/v1/jobs/{id}/cancel", h.CancelJob)
	mux.HandleFunc("POST /api/v1/jobs/{id}/retry", h.RetryJob)
	mux.HandleFunc("GET /api/v1/stats", h.GetStats)

	// Workflows endpoints
	mux.HandleFunc("POST /api/v1/workflows", h.CreateWorkflow)
	mux.HandleFunc("GET /api/v1/workflows", h.ListWorkflows)
	mux.HandleFunc("GET /api/v1/workflows/{id}", h.GetWorkflow)
	mux.HandleFunc("POST /api/v1/workflows/{id}/cancel", h.CancelWorkflow)

	// Workers & Queues endpoints
	mux.HandleFunc("GET /api/v1/workers", h.ListWorkers)
	mux.HandleFunc("GET /api/v1/queues", h.GetQueueStats)

	// Wrap with basic middleware.
	var handler http.Handler = mux
	handler = corsMiddleware(handler)
	handler = requestLogger(handler, logger)
	handler = recoverPanic(handler, logger)

	return handler
}

func requestLogger(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func recoverPanic(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic recovered", "error", err)
				respondError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
		w.Header().Set("Access-Control-Expose-Headers", "X-Idempotent-Replay")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
