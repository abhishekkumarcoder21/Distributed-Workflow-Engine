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

func BenchmarkRedisQueue_Enqueue(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	q := NewRedisQueue(client)
	defer q.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := &domain.Job{
			ID:       uuid.New(),
			Queue:    "default",
			Priority: (i % 3) + 1,
		}
		if err := q.Enqueue(ctx, job); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRedisQueue_Claim(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	q := NewRedisQueue(client)
	defer q.Close()

	ctx := context.Background()

	// Pre-populate queue with enough jobs
	for i := 0; i < b.N; i++ {
		job := &domain.Job{
			ID:       uuid.New(),
			Queue:    "default",
			Priority: (i % 3) + 1,
		}
		_ = q.Enqueue(ctx, job)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := q.Claim(ctx, "bench-worker", "default", 30*time.Second)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRedisQueue_Ack(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	q := NewRedisQueue(client)
	defer q.Close()

	ctx := context.Background()

	jobIDs := make([]uuid.UUID, b.N)
	for i := 0; i < b.N; i++ {
		jobIDs[i] = uuid.New()
		_ = q.Enqueue(ctx, &domain.Job{ID: jobIDs[i], Queue: "default", Priority: 2})
		_, _ = q.Claim(ctx, "bench-worker", "default", 30*time.Second)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := q.Ack(ctx, "default", jobIDs[i]); err != nil {
			b.Fatal(err)
		}
	}
}
