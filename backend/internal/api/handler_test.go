package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_HealthCheck(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.HealthCheck(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Data["status"])
	}
}

func TestHandler_CreateJob_Validation(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid JSON",
			body:       "{invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing type",
			body:       `{"payload": {}}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid priority (too high)",
			body:       `{"type": "email", "priority": 5}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid priority (too low)",
			body:       `{"type": "email", "priority": 0}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid max_attempts (zero)",
			body:       `{"type": "email", "max_attempts": 0}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid idempotency_key (empty)",
			body:       `{"type": "email", "idempotency_key": ""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid idempotency_key (too long > 128 chars)",
			body:       `{"type": "email", "idempotency_key": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(tc.body))
			w := httptest.NewRecorder()

			h.CreateJob(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d, body: %s", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}

	// Test header-based Idempotency-Key validation
	t.Run("invalid Idempotency-Key header (too long)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(`{"type": "email"}`))
		req.Header.Set("Idempotency-Key", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		w := httptest.NewRecorder()

		h.CreateJob(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
		}
	})
}

func TestHandler_GetJob_InvalidID(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	w := httptest.NewRecorder()

	h.GetJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_CancelJob_InvalidID(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/invalid/cancel", nil)
	req.SetPathValue("id", "invalid")
	w := httptest.NewRecorder()

	h.CancelJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_RetryJob_InvalidID(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/invalid/retry", nil)
	req.SetPathValue("id", "invalid")
	w := httptest.NewRecorder()

	h.RetryJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_ListWorkers_Empty(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workers", nil)
	w := httptest.NewRecorder()

	h.ListWorkers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_GetQueueStats_NilQueue(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/queues", nil)
	w := httptest.NewRecorder()

	h.GetQueueStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_CreateWorkflow_Validation(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid JSON",
			body:       "{invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing workflow name",
			body:       `{"tasks": [{"name": "t1", "type": "email"}]}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty tasks",
			body:       `{"name": "test_wf", "tasks": []}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "cycle in DAG",
			body:       `{"name": "cycle_wf", "tasks": [{"name": "A", "type": "t", "dependencies": ["B"]}, {"name": "B", "type": "t", "dependencies": ["A"]}]}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewBufferString(tc.body))
			w := httptest.NewRecorder()

			h.CreateWorkflow(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d, body: %s", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandler_GetWorkflow_InvalidID(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	w := httptest.NewRecorder()

	h.GetWorkflow(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_CancelWorkflow_InvalidID(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/invalid/cancel", nil)
	req.SetPathValue("id", "invalid")
	w := httptest.NewRecorder()

	h.CancelWorkflow(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}




