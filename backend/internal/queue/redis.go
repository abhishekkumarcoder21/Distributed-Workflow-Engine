package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/distributed-workflow-engine/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	defaultQueueName = "default"
	workersActiveKey = "workers:active"
)

// claimScript atomically inspects queues with weighted starvation mitigation,
// claims the chosen job, and moves it into the in-progress sorted set with a visibility deadline.
var claimScript = redis.NewScript(`
local in_progress_key = KEYS[4]
local counter_key = KEYS[5]
local deadline = tonumber(ARGV[1])
local w1 = tonumber(ARGV[2]) or 6
local w2 = tonumber(ARGV[3]) or 3
local w3 = tonumber(ARGV[4]) or 1
local total_w = w1 + w2 + w3

local count = redis.call('INCR', counter_key)
local slot = count % total_w

-- Determine search order based on weighted slot to prevent starvation
local search_order
if slot < w1 then
    search_order = {KEYS[1], KEYS[2], KEYS[3]}
elseif slot < (w1 + w2) then
    search_order = {KEYS[2], KEYS[1], KEYS[3]}
else
    search_order = {KEYS[3], KEYS[1], KEYS[2]}
end

for _, q_key in ipairs(search_order) do
    local items = redis.call('ZRANGE', q_key, 0, 0)
    if #items > 0 then
        local job_id = items[1]
        redis.call('ZREM', q_key, job_id)
        redis.call('ZADD', in_progress_key, deadline, job_id)
        return job_id
    end
end

return nil
`)

// requeueStaleScript atomically moves jobs whose visibility timeout has expired
// back to the priority queue.
var requeueStaleScript = redis.NewScript(`
local in_progress_key = KEYS[1]
local target_queue_key = KEYS[2]
local now = tonumber(ARGV[1])

local stale_jobs = redis.call('ZRANGEBYSCORE', in_progress_key, '-inf', now)
local count = 0
for _, job_id in ipairs(stale_jobs) do
    redis.call('ZREM', in_progress_key, job_id)
    redis.call('ZADD', target_queue_key, now, job_id)
    count = count + 1
end

return count
`)

type RedisQueue struct {
	client *redis.Client
}

func NewRedisQueue(client *redis.Client) *RedisQueue {
	return &RedisQueue{client: client}
}

func (q *RedisQueue) Enqueue(ctx context.Context, job *domain.Job) error {
	if job == nil || job.ID == uuid.Nil {
		return ErrInvalidJob
	}

	queueName := job.Queue
	if queueName == "" {
		queueName = defaultQueueName
	}

	priority := job.Priority
	if priority < 1 || priority > 3 {
		priority = domain.DefaultPriority
	}

	key := priorityQueueKey(queueName, priority)
	score := float64(time.Now().UnixMilli())

	err := q.client.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: job.ID.String(),
	}).Err()
	if err != nil {
		return fmt.Errorf("enqueuing job %s: %w", job.ID, err)
	}

	return nil
}

func (q *RedisQueue) Claim(ctx context.Context, workerID string, queueName string, visibilityTimeout time.Duration) (uuid.UUID, error) {
	return q.ClaimWithWeights(ctx, workerID, queueName, visibilityTimeout, 6, 3, 1)
}

func (q *RedisQueue) ClaimWithWeights(ctx context.Context, workerID string, queueName string, visibilityTimeout time.Duration, w1, w2, w3 int) (uuid.UUID, error) {
	if queueName == "" {
		queueName = defaultQueueName
	}
	if visibilityTimeout <= 0 {
		visibilityTimeout = 30 * time.Second
	}
	if w1 <= 0 {
		w1 = 6
	}
	if w2 <= 0 {
		w2 = 3
	}
	if w3 <= 0 {
		w3 = 1
	}

	keys := []string{
		priorityQueueKey(queueName, 1),
		priorityQueueKey(queueName, 2),
		priorityQueueKey(queueName, 3),
		inProgressKey(queueName),
		claimCounterKey(queueName),
	}

	deadline := time.Now().Add(visibilityTimeout).UnixMilli()

	result, err := claimScript.Run(ctx, q.client, keys, deadline, w1, w2, w3).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return uuid.Nil, ErrQueueEmpty
		}
		return uuid.Nil, fmt.Errorf("claiming job: %w", err)
	}

	jobIDStr, ok := result.(string)
	if !ok || jobIDStr == "" {
		return uuid.Nil, ErrQueueEmpty
	}

	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parsing claimed job id %q: %w", jobIDStr, err)
	}

	return jobID, nil
}

func (q *RedisQueue) Ack(ctx context.Context, queueName string, jobID uuid.UUID) error {
	if queueName == "" {
		queueName = defaultQueueName
	}
	key := inProgressKey(queueName)
	err := q.client.ZRem(ctx, key, jobID.String()).Err()
	if err != nil {
		return fmt.Errorf("acking job %s: %w", jobID, err)
	}
	return nil
}

func (q *RedisQueue) RequeueStale(ctx context.Context, queueName string, defaultPriority int) (int, error) {
	if queueName == "" {
		queueName = defaultQueueName
	}
	if defaultPriority < 1 || defaultPriority > 3 {
		defaultPriority = domain.DefaultPriority
	}

	keys := []string{
		inProgressKey(queueName),
		priorityQueueKey(queueName, defaultPriority),
	}
	now := time.Now().UnixMilli()

	result, err := requeueStaleScript.Run(ctx, q.client, keys, now).Result()
	if err != nil {
		return 0, fmt.Errorf("requeuing stale jobs: %w", err)
	}

	count, ok := result.(int64)
	if !ok {
		return 0, nil
	}
	return int(count), nil
}

func (q *RedisQueue) Heartbeat(ctx context.Context, workerID string, ttl time.Duration) error {
	if workerID == "" {
		return errors.New("empty worker ID")
	}
	pipe := q.client.Pipeline()
	key := workerHeartbeatKey(workerID)
	pipe.Set(ctx, key, time.Now().Format(time.RFC3339), ttl)
	pipe.SAdd(ctx, workersActiveKey, workerID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("writing heartbeat for worker %s: %w", workerID, err)
	}
	return nil
}

func (q *RedisQueue) RemoveWorker(ctx context.Context, workerID string) error {
	if workerID == "" {
		return nil
	}
	pipe := q.client.Pipeline()
	pipe.Del(ctx, workerHeartbeatKey(workerID))
	pipe.SRem(ctx, workersActiveKey, workerID)
	_, err := pipe.Exec(ctx)
	return err
}

func (q *RedisQueue) QueueLength(ctx context.Context, queueName string) (int64, error) {
	if queueName == "" {
		queueName = defaultQueueName
	}
	var total int64
	for p := 1; p <= 3; p++ {
		count, err := q.client.ZCard(ctx, priorityQueueKey(queueName, p)).Result()
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func (q *RedisQueue) InProgressCount(ctx context.Context, queueName string) (int64, error) {
	if queueName == "" {
		queueName = defaultQueueName
	}
	return q.client.ZCard(ctx, inProgressKey(queueName)).Result()
}

func (q *RedisQueue) GetQueueStats(ctx context.Context, queueName string) (*QueueStats, error) {
	if queueName == "" {
		queueName = defaultQueueName
	}
	stats := &QueueStats{
		QueueName:      queueName,
		PriorityCounts: make(map[int]int64),
	}

	for p := 1; p <= 3; p++ {
		c, err := q.client.ZCard(ctx, priorityQueueKey(queueName, p)).Result()
		if err != nil {
			return nil, fmt.Errorf("reading priority %d count: %w", p, err)
		}
		stats.PriorityCounts[p] = c
	}

	inProg, err := q.client.ZCard(ctx, inProgressKey(queueName)).Result()
	if err != nil {
		return nil, fmt.Errorf("reading in-progress count: %w", err)
	}
	stats.InProgressCount = inProg

	return stats, nil
}

func (q *RedisQueue) Close() error {
	return q.client.Close()
}

func priorityQueueKey(queueName string, priority int) string {
	return fmt.Sprintf("queue:%s:priority:%d", queueName, priority)
}

func inProgressKey(queueName string) string {
	return fmt.Sprintf("queue:%s:in-progress", queueName)
}

func workerHeartbeatKey(workerID string) string {
	return fmt.Sprintf("worker:%s:heartbeat", workerID)
}

func claimCounterKey(queueName string) string {
	return fmt.Sprintf("queue:%s:claim_counter", queueName)
}
