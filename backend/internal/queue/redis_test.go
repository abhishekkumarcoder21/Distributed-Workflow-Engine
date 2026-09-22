package queue

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/distributed-workflow-engine/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func setupTestQueue(t *testing.T) (*RedisQueue, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	q := NewRedisQueue(client)
	t.Cleanup(func() {
		q.Close()
		mr.Close()
	})

	return q, mr
}

func TestRedisQueue_EnqueueAndClaim(t *testing.T) {
	q, _ := setupTestQueue(t)
	ctx := context.Background()

	jobID := uuid.New()
	job := &domain.Job{
		ID:       jobID,
		Queue:    "default",
		Priority: 2,
	}

	if err := q.Enqueue(ctx, job); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	len, err := q.QueueLength(ctx, "default")
	if err != nil {
		t.Fatalf("queue length failed: %v", err)
	}
	if len != 1 {
		t.Fatalf("expected queue length 1, got %d", len)
	}

	claimedID, err := q.Claim(ctx, "worker-1", "default", 10*time.Second)
	if err != nil {
		t.Fatalf("claim failed: %v", err)
	}
	if claimedID != jobID {
		t.Fatalf("expected claimed ID %s, got %s", jobID, claimedID)
	}

	// Queue should now be 0, and in-progress should be 1
	len, _ = q.QueueLength(ctx, "default")
	if len != 0 {
		t.Errorf("expected queue length 0 after claim, got %d", len)
	}

	inProgress, err := q.InProgressCount(ctx, "default")
	if err != nil {
		t.Fatalf("in-progress count failed: %v", err)
	}
	if inProgress != 1 {
		t.Errorf("expected in-progress 1, got %d", inProgress)
	}
}

func TestRedisQueue_PriorityOrder(t *testing.T) {
	q, _ := setupTestQueue(t)
	ctx := context.Background()

	lowID := uuid.New()
	medID := uuid.New()
	highID := uuid.New()

	// Enqueue in reverse order: low, medium, high
	if err := q.Enqueue(ctx, &domain.Job{ID: lowID, Queue: "default", Priority: 3}); err != nil {
		t.Fatal(err)
	}
	if err := q.Enqueue(ctx, &domain.Job{ID: medID, Queue: "default", Priority: 2}); err != nil {
		t.Fatal(err)
	}
	if err := q.Enqueue(ctx, &domain.Job{ID: highID, Queue: "default", Priority: 1}); err != nil {
		t.Fatal(err)
	}

	// Claim 1: Should be high priority
	c1, err := q.Claim(ctx, "worker-1", "default", 10*time.Second)
	if err != nil {
		t.Fatalf("claim 1 failed: %v", err)
	}
	if c1 != highID {
		t.Errorf("expected high priority job %s, got %s", highID, c1)
	}

	// Claim 2: Should be medium priority
	c2, err := q.Claim(ctx, "worker-1", "default", 10*time.Second)
	if err != nil {
		t.Fatalf("claim 2 failed: %v", err)
	}
	if c2 != medID {
		t.Errorf("expected medium priority job %s, got %s", medID, c2)
	}

	// Claim 3: Should be low priority
	c3, err := q.Claim(ctx, "worker-1", "default", 10*time.Second)
	if err != nil {
		t.Fatalf("claim 3 failed: %v", err)
	}
	if c3 != lowID {
		t.Errorf("expected low priority job %s, got %s", lowID, c3)
	}

	// Claim 4: Queue empty
	_, err = q.Claim(ctx, "worker-1", "default", 10*time.Second)
	if err != ErrQueueEmpty {
		t.Errorf("expected ErrQueueEmpty, got %v", err)
	}
}

func TestRedisQueue_Ack(t *testing.T) {
	q, _ := setupTestQueue(t)
	ctx := context.Background()

	jobID := uuid.New()
	if err := q.Enqueue(ctx, &domain.Job{ID: jobID, Queue: "default", Priority: 2}); err != nil {
		t.Fatal(err)
	}

	_, err := q.Claim(ctx, "worker-1", "default", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if err := q.Ack(ctx, "default", jobID); err != nil {
		t.Fatalf("ack failed: %v", err)
	}

	inProgress, err := q.InProgressCount(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	if inProgress != 0 {
		t.Errorf("expected 0 in-progress after ack, got %d", inProgress)
	}
}

func TestRedisQueue_RequeueStale(t *testing.T) {
	q, _ := setupTestQueue(t)
	ctx := context.Background()

	jobID := uuid.New()
	if err := q.Enqueue(ctx, &domain.Job{ID: jobID, Queue: "default", Priority: 2}); err != nil {
		t.Fatal(err)
	}

	// Claim with 100 millisecond visibility timeout
	_, err := q.Claim(ctx, "worker-1", "default", 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for visibility timeout to expire
	time.Sleep(150 * time.Millisecond)

	requeued, err := q.RequeueStale(ctx, "default", 2)
	if err != nil {
		t.Fatalf("requeue stale failed: %v", err)
	}
	if requeued != 1 {
		t.Fatalf("expected 1 job requeued, got %d", requeued)
	}

	// The job should now be reclaimable again!
	reclaimedID, err := q.Claim(ctx, "worker-2", "default", 10*time.Second)
	if err != nil {
		t.Fatalf("reclaim failed: %v", err)
	}
	if reclaimedID != jobID {
		t.Errorf("expected reclaimed job %s, got %s", jobID, reclaimedID)
	}
}

func TestRedisQueue_Heartbeat(t *testing.T) {
	q, mr := setupTestQueue(t)
	ctx := context.Background()

	workerID := "worker-alpha"
	if err := q.Heartbeat(ctx, workerID, 5*time.Second); err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}

	if !mr.Exists("worker:worker-alpha:heartbeat") {
		t.Error("expected heartbeat key to exist")
	}

	if err := q.RemoveWorker(ctx, workerID); err != nil {
		t.Fatalf("remove worker failed: %v", err)
	}

	if mr.Exists("worker:worker-alpha:heartbeat") {
		t.Error("expected heartbeat key to be deleted")
	}
}

func TestRedisQueue_StarvationMitigation(t *testing.T) {
	q, _ := setupTestQueue(t)
	ctx := context.Background()

	// Enqueue 15 high-priority jobs
	for i := 0; i < 15; i++ {
		if err := q.Enqueue(ctx, &domain.Job{ID: uuid.New(), Queue: "default", Priority: 1}); err != nil {
			t.Fatal(err)
		}
	}

	// Enqueue 1 medium-priority job and 1 low-priority job
	medID := uuid.New()
	lowID := uuid.New()
	if err := q.Enqueue(ctx, &domain.Job{ID: medID, Queue: "default", Priority: 2}); err != nil {
		t.Fatal(err)
	}
	if err := q.Enqueue(ctx, &domain.Job{ID: lowID, Queue: "default", Priority: 3}); err != nil {
		t.Fatal(err)
	}

	// Claim 10 jobs using weights (6, 3, 1)
	claimed := make(map[uuid.UUID]bool)
	for i := 0; i < 10; i++ {
		id, err := q.ClaimWithWeights(ctx, "worker-1", "default", 10*time.Second, 6, 3, 1)
		if err != nil {
			t.Fatalf("claim %d failed: %v", i, err)
		}
		claimed[id] = true
	}

	// Both medium and low priority jobs MUST have been claimed during this 10-claim cycle,
	// proving that high priority did not starve them!
	if !claimed[medID] {
		t.Errorf("starvation detected: medium priority job %s was not claimed in 10-claim cycle", medID)
	}
	if !claimed[lowID] {
		t.Errorf("starvation detected: low priority job %s was not claimed in 10-claim cycle", lowID)
	}
}

func TestRedisQueue_GetQueueStats(t *testing.T) {
	q, _ := setupTestQueue(t)
	ctx := context.Background()

	_ = q.Enqueue(ctx, &domain.Job{ID: uuid.New(), Queue: "default", Priority: 1})
	_ = q.Enqueue(ctx, &domain.Job{ID: uuid.New(), Queue: "default", Priority: 1})
	_ = q.Enqueue(ctx, &domain.Job{ID: uuid.New(), Queue: "default", Priority: 2})
	_ = q.Enqueue(ctx, &domain.Job{ID: uuid.New(), Queue: "default", Priority: 3})

	// Claim one job so in-progress count is 1
	_, _ = q.Claim(ctx, "worker-1", "default", 10*time.Second)

	stats, err := q.GetQueueStats(ctx, "default")
	if err != nil {
		t.Fatalf("get queue stats failed: %v", err)
	}

	if stats.InProgressCount != 1 {
		t.Errorf("expected 1 in-progress, got %d", stats.InProgressCount)
	}
	totalWaiting := stats.PriorityCounts[1] + stats.PriorityCounts[2] + stats.PriorityCounts[3]
	if totalWaiting != 3 {
		t.Errorf("expected 3 waiting jobs, got %d", totalWaiting)
	}
}

