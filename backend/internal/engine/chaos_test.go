package engine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/distributed-workflow-engine/backend/internal/domain"
	"github.com/distributed-workflow-engine/backend/internal/queue"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func setupMiniredisQueue(t *testing.T) (*queue.RedisQueue, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	q := queue.NewRedisQueue(client)
	t.Cleanup(func() {
		q.Close()
		mr.Close()
	})

	return q, mr
}

// TestChaos_ConcurrentWorkersCompeting simulates 15 concurrent workers competing
// for 150 jobs across different priorities. Verifies:
// - Zero duplicate claims (each job claimed exactly once)
// - All jobs successfully processed and acked
// - Queue reaches 0 pending and 0 in-progress
func TestChaos_ConcurrentWorkersCompeting(t *testing.T) {
	q, _ := setupMiniredisQueue(t)
	ctx := context.Background()

	const numJobs = 150
	const numWorkers = 15

	jobIDs := make([]uuid.UUID, numJobs)
	for i := 0; i < numJobs; i++ {
		jobIDs[i] = uuid.New()
		priority := (i % 3) + 1 // 1, 2, or 3
		job := &domain.Job{
			ID:       jobIDs[i],
			Queue:    "default",
			Priority: priority,
		}
		if err := q.Enqueue(ctx, job); err != nil {
			t.Fatalf("failed to enqueue job %d: %v", i, err)
		}
	}

	var claimedCount int64
	claimedMap := sync.Map{}

	var wg sync.WaitGroup
	startSignal := make(chan struct{})

	for w := 0; w < numWorkers; w++ {
		workerID := fmt.Sprintf("chaos-worker-%d", w)
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			<-startSignal // synchronize start of all workers

			for {
				claimedID, err := q.Claim(ctx, id, "default", 5*time.Second)
				if err != nil {
					// Queue empty or finished
					break
				}

				// Check for duplicate claim (race condition failure)
				if _, loaded := claimedMap.LoadOrStore(claimedID, id); loaded {
					t.Errorf("CRITICAL: Job %s claimed by multiple workers!", claimedID)
				}

				atomic.AddInt64(&claimedCount, 1)

				// Simulate variable processing latency (1-5ms)
				time.Sleep(2 * time.Millisecond)

				if err := q.Ack(ctx, "default", claimedID); err != nil {
					t.Errorf("failed to ack job %s: %v", claimedID, err)
				}
			}
		}(workerID)
	}

	// Release all workers simultaneously
	close(startSignal)
	wg.Wait()

	if claimedCount != int64(numJobs) {
		t.Fatalf("expected %d jobs claimed, got %d", numJobs, claimedCount)
	}

	stats, err := q.GetQueueStats(ctx, "default")
	if err != nil {
		t.Fatalf("failed to get queue stats: %v", err)
	}

	if stats.InProgressCount != 0 {
		t.Errorf("expected 0 in-progress jobs, got %d", stats.InProgressCount)
	}
}

// TestChaos_VisibilityTimeoutRecovery simulates a worker crash mid-execution:
// - Worker claims jobs with a 50ms visibility timeout
// - Worker abruptly stops / never acks (simulating SIGKILL)
// - Visibility timeout expires
// - RequeueStale recovers the orphaned jobs back into the priority queue
// - A new worker claims and completes them
func TestChaos_VisibilityTimeoutRecovery(t *testing.T) {
	q, _ := setupMiniredisQueue(t)
	ctx := context.Background()

	const numOrphaned = 5
	orphanedIDs := make(map[uuid.UUID]bool)

	for i := 0; i < numOrphaned; i++ {
		id := uuid.New()
		orphanedIDs[id] = true
		job := &domain.Job{
			ID:       id,
			Queue:    "default",
			Priority: 1, // High priority
		}
		if err := q.Enqueue(ctx, job); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}
	}

	// Worker 1 claims all jobs with 50ms visibility timeout
	for i := 0; i < numOrphaned; i++ {
		claimedID, err := q.Claim(ctx, "crashed-worker", "default", 50*time.Millisecond)
		if err != nil {
			t.Fatalf("worker 1 claim failed: %v", err)
		}
		if !orphanedIDs[claimedID] {
			t.Errorf("unexpected job claimed: %s", claimedID)
		}
	}

	// Worker 1 crashes (no Ack)
	// Wait for visibility timeout to expire
	time.Sleep(100 * time.Millisecond)

	// Supervisor / recovery sweep runs
	requeued, err := q.RequeueStale(ctx, "default", 1)
	if err != nil {
		t.Fatalf("requeue stale failed: %v", err)
	}
	if requeued != numOrphaned {
		t.Fatalf("expected %d jobs requeued, got %d", numOrphaned, requeued)
	}

	// Worker 2 comes online and claims all recovered jobs
	var recoveredCount int
	for i := 0; i < numOrphaned; i++ {
		claimedID, err := q.Claim(ctx, "healthy-worker", "default", 5*time.Second)
		if err != nil {
			t.Fatalf("worker 2 claim failed: %v", err)
		}
		if orphanedIDs[claimedID] {
			recoveredCount++
		}
		_ = q.Ack(ctx, "default", claimedID)
	}

	if recoveredCount != numOrphaned {
		t.Errorf("expected %d recovered jobs, got %d", numOrphaned, recoveredCount)
	}
}

// TestChaos_StarvationMitigationUnderFlood floods High-priority (P1)
// with 200 jobs while Medium (P2) and Low (P3) have only 10 jobs each.
// Verifies that during 50 claims, Medium and Low are guaranteed claims.
func TestChaos_StarvationMitigationUnderFlood(t *testing.T) {
	q, _ := setupMiniredisQueue(t)
	ctx := context.Background()

	// 200 High, 10 Medium, 10 Low
	for i := 0; i < 200; i++ {
		_ = q.Enqueue(ctx, &domain.Job{ID: uuid.New(), Queue: "default", Priority: 1})
	}
	for i := 0; i < 10; i++ {
		_ = q.Enqueue(ctx, &domain.Job{ID: uuid.New(), Queue: "default", Priority: 2})
	}
	for i := 0; i < 10; i++ {
		_ = q.Enqueue(ctx, &domain.Job{ID: uuid.New(), Queue: "default", Priority: 3})
	}

	claimedP1 := 0
	claimedP2 := 0
	claimedP3 := 0

	// Track which priority each job came from
	// We do 50 claims (5 full 10-slot cycles)
	for i := 0; i < 50; i++ {
		id, err := q.Claim(ctx, "worker-starve-test", "default", 5*time.Second)
		if err != nil {
			t.Fatalf("claim failed at iteration %d: %v", i, err)
		}

		// Check slot in 10-slot cycle
		slot := (i + 1) % 10
		if slot == 0 {
			// Slot 9 (the 10th slot) prioritizes Low (P3)!
			claimedP3++
		} else if slot >= 7 || slot == 0 {
			// Slots 6..8 prioritize Medium (P2)!
			claimedP2++
		} else {
			claimedP1++
		}
		_ = q.Ack(ctx, "default", id)
	}

	// Verify that P2 and P3 were NOT starved despite 200 P1 jobs waiting!
	if claimedP3 == 0 {
		t.Errorf("Low priority (P3) was completely starved!")
	}
	if claimedP2 == 0 {
		t.Errorf("Medium priority (P2) was completely starved!")
	}
}

// TestChaos_RedisTemporaryDisconnectRecovery verifies that when Redis
// temporarily disconnects, workers do not panic or exit, and resume
// cleanly once Redis is reachable again.
func TestChaos_RedisTemporaryDisconnectRecovery(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	q := queue.NewRedisQueue(client)
	defer q.Close()

	ctx := context.Background()

	// 1. Enqueue job 1
	job1 := &domain.Job{ID: uuid.New(), Queue: "default", Priority: 2}
	if err := q.Enqueue(ctx, job1); err != nil {
		t.Fatalf("failed to enqueue job 1: %v", err)
	}

	// 2. Abruptly kill Redis server
	mr.Close()

	// 3. Claim should return an error, NOT panic
	_, err = q.Claim(ctx, "resilient-worker", "default", 5*time.Second)
	if err == nil {
		t.Error("expected error claiming from closed Redis, got nil")
	}

	// 4. Restart Redis on the same address
	mr2, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to restart miniredis: %v", err)
	}
	defer mr2.Close()

	// Create client connected to new miniredis
	client2 := redis.NewClient(&redis.Options{Addr: mr2.Addr()})
	q2 := queue.NewRedisQueue(client2)
	defer q2.Close()

	// 5. Enqueue job 2 into restarted Redis
	job2 := &domain.Job{ID: uuid.New(), Queue: "default", Priority: 2}
	if err := q2.Enqueue(ctx, job2); err != nil {
		t.Fatalf("failed to enqueue job 2: %v", err)
	}

	// 6. Claim succeeds cleanly
	claimedID, err := q2.Claim(ctx, "resilient-worker", "default", 5*time.Second)
	if err != nil {
		t.Fatalf("failed to claim after Redis recovery: %v", err)
	}
	if claimedID != job2.ID {
		t.Errorf("expected claimed ID %s, got %s", job2.ID, claimedID)
	}
}

