package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/distributed-workflow-engine/backend/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Clean the jobs table before each test.
	if _, err := pool.Exec(ctx, "DELETE FROM jobs"); err != nil {
		pool.Close()
		t.Fatalf("failed to clean jobs table: %v", err)
	}

	return pool, func() { pool.Close() }
}

func TestJobStore_CreateAndGet(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	s := NewJobStore(pool)
	ctx := context.Background()

	req := domain.CreateJobRequest{
		Type:    "send_email",
		Payload: json.RawMessage(`{"recipient":"test@example.com"}`),
	}

	job, created, err := s.Create(ctx, req)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !created {
		t.Errorf("expected created = true for new job")
	}
	if job.Type != "send_email" {
		t.Errorf("type = %q, want send_email", job.Type)
	}
	if job.Status != domain.JobStatusQueued {
		t.Errorf("status = %q, want QUEUED", job.Status)
	}
	if job.Priority != domain.DefaultPriority {
		t.Errorf("priority = %d, want %d", job.Priority, domain.DefaultPriority)
	}

	got, err := s.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != job.ID {
		t.Errorf("id mismatch")
	}
}

func TestJobStore_IdempotencyKey(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	s := NewJobStore(pool)
	ctx := context.Background()

	key := "signup-user-123"
	req := domain.CreateJobRequest{
		Type:           "send_email",
		Payload:        json.RawMessage(`{}`),
		IdempotencyKey: &key,
	}

	first, c1, err := s.Create(ctx, req)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if !c1 {
		t.Errorf("expected c1 = true for first insert")
	}

	second, c2, err := s.Create(ctx, req)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if c2 {
		t.Errorf("expected c2 = false for duplicate insert")
	}

	if first.ID != second.ID {
		t.Errorf("idempotency failed: got different IDs %s and %s", first.ID, second.ID)
	}
}

func TestJobStore_Claim(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	s := NewJobStore(pool)
	ctx := context.Background()

	// Create two jobs with different priorities.
	highPriority := 1
	req1 := domain.CreateJobRequest{
		Type:     "send_email",
		Payload:  json.RawMessage(`{}`),
		Priority: &highPriority,
	}
	lowPriority := 3
	req2 := domain.CreateJobRequest{
		Type:     "generate_report",
		Payload:  json.RawMessage(`{}`),
		Priority: &lowPriority,
	}

	if _, _, err := s.Create(ctx, req2); err != nil {
		t.Fatalf("create low priority: %v", err)
	}
	if _, _, err := s.Create(ctx, req1); err != nil {
		t.Fatalf("create high priority: %v", err)
	}

	// Claim should pick the high-priority job first.
	claimed, err := s.Claim(ctx, "test-worker")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected a job, got nil")
	}
	if claimed.Type != "send_email" {
		t.Errorf("expected high-priority job (send_email), got %s", claimed.Type)
	}
	if claimed.Status != domain.JobStatusRunning {
		t.Errorf("status = %q, want RUNNING", claimed.Status)
	}
	if claimed.Attempt != 1 {
		t.Errorf("attempt = %d, want 1", claimed.Attempt)
	}
}

func TestJobStore_Cancel(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	s := NewJobStore(pool)
	ctx := context.Background()

	job, _, err := s.Create(ctx, domain.CreateJobRequest{
		Type:    "send_email",
		Payload: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := s.Cancel(ctx, job.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	got, err := s.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != domain.JobStatusCancelled {
		t.Errorf("status = %q, want CANCELLED", got.Status)
	}

	// Cancelling an already-cancelled job should fail.
	if err := s.Cancel(ctx, job.ID); err != ErrJobNotCancellable {
		t.Errorf("expected ErrJobNotCancellable, got %v", err)
	}
}

func TestJobStore_List(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	s := NewJobStore(pool)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, _, err := s.Create(ctx, domain.CreateJobRequest{
			Type:    "send_email",
			Payload: json.RawMessage(`{}`),
		}); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	jobs, total, err := s.List(ctx, domain.JobFilter{Limit: 3})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(jobs) != 3 {
		t.Errorf("len = %d, want 3", len(jobs))
	}
}
