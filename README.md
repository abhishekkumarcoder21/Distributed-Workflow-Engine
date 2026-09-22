# Distributed Workflow Engine

A high-performance, fault-tolerant distributed background job processor and DAG workflow orchestration engine built in Go, backed by PostgreSQL and Redis, featuring an interactive Next.js control plane dashboard.

Designed with distributed systems fundamentals: **at-least-once delivery**, **zero-starvation priority scheduling**, **exponential backoff with full jitter**, **atomic PostgreSQL state transitions**, **visibility timeout crash recovery**, and **Kahn's topological DAG cycle detection**.

---

## Architecture Overview

```
                          ┌───────────────────────────┐
                          │   Next.js Control Plane   │
                          │   Dashboard (Port 3000)   │
                          └─────────────┬─────────────┘
                                        │ HTTP / JSON
                                        ▼
                          ┌───────────────────────────┐
                          │     Go API & Scheduler    │
                          │        (Port 8080)        │
                          └──────┬─────────────┬──────┘
                                 │             │
                State & Metadata │             │ Priority Queue Dispatch
                                 ▼             ▼
                     ┌──────────────┐       ┌──────────────┐
                     │  PostgreSQL  │       │ Redis (ZSET) │
                     │ Source of    │       │ Ephemeral    │
                     │ Truth        │       │ Priority Qs  │
                     └──────────────┘       └──────┬───────┘
                                                   │
                                     Claim / Ack   │ Visibility Timeout Heartbeat
                                                   ▼
                                    ┌───────────────────────┐
                                    │    Worker Clusters    │
                                    │ (Concurrency Pool,    │
                                    │  Graceful Draining)   │
                                    └───────────────────────┘
```

---

## Key Engineering Features

1. **Multi-Priority Queueing with Starvation Mitigation (6:3:1)**:
   - Redis sorted sets (`queue:{name}:priority:{1,2,3}`) maintain strict FIFO ordering by timestamp per priority level.
   - An atomic Lua script executes a 10-slot cycle: Slots 0..5 (High priority first), Slots 6..8 (Medium priority first), Slot 9 (Low priority first).
   - Guarantees lower-priority progress even under massive high-priority floods with zero latency waste (instant fallback).

2. **At-Least-Once Delivery & Visibility Timeout Recovery**:
   - Claimed jobs are atomically moved into an in-progress sorted set (`queue:{name}:in-progress`) with score = `now + visibility_timeout`.
   - If a worker crashes before acknowledging, `RequeueStale` automatically recovers orphaned jobs back to the priority queue without data loss.

3. **Exponential Backoff with Full Jitter**:
   - Failed jobs transition to `RETRYING` with randomized exponential delays:
     $$\text{delay} = \text{rand}(0, \min(\text{maxDelay}, \text{baseDelay} \times 2^{\text{attempt}-1}))$$
   - Prevents the thundering herd problem when third-party services experience outages.
   - Permanently failing jobs move to `DEAD_LETTER` for manual inspection and replay.

4. **DAG Workflow Orchestration Engine**:
   - Define multi-step workflows with task dependencies.
   - Pre-submission cycle detection via Kahn's algorithm ($O(V + E)$).
   - Automatic concurrency: independent root tasks run simultaneously, downstream tasks schedule immediately upon parent completion.

5. **Atomic Idempotency & Deduplication**:
   - Both HTTP header (`Idempotency-Key`) and body (`idempotency_key`) supported.
   - PostgreSQL atomic `INSERT ... ON CONFLICT (idempotency_key) DO NOTHING` prevents duplicate execution races. Replayed requests return `200 OK` with `X-Idempotent-Replay: true` without duplicate Redis enqueue.

6. **Next.js Real-Time Control Plane**:
   - Live visual DAG viewer, job filtering, payload inspector, manual retry/cancellation, worker heartbeat monitoring, and queue backlog metrics.

7. **Production Observability**:
   - Prometheus metrics exporter (`/metrics`) recording job throughput, execution durations, queue depths, and worker counts.

---

## Quickstart with Docker Compose

Start the entire distributed cluster (PostgreSQL, Redis, API, Worker, and Next.js Dashboard) with a single command:

```bash
docker compose up --build
```

- **Dashboard**: [http://localhost:3000](http://localhost:3000)
- **API Server**: [http://localhost:8080](http://localhost:8080)
- **Prometheus Metrics**: [http://localhost:8080/metrics](http://localhost:8080/metrics)
- **Health Check**: [http://localhost:8080/health](http://localhost:8080/health)

---

## Local Development

### Prerequisites
- Go 1.22+
- Node.js 20+
- PostgreSQL 16 & Redis 7 (or running via `docker compose up postgres redis`)

### 1. Run the Backend API
```bash
cd backend
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/workflow_engine?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"
go run ./cmd/api
```

### 2. Run Worker Processes
```bash
cd backend
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/workflow_engine?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"
export CONCURRENCY=5
go run ./cmd/worker
```

### 3. Run the Dashboard
```bash
cd frontend
npm install
npm run dev
```
Open [http://localhost:3000](http://localhost:3000).

---

## Performance Benchmarks

Measured on a 12th Gen Intel Core i7-1260P (16 threads):

| Operation | Latency / Op | Single-Thread Throughput |
|---|---|---|
| **DAG Validation (10 Nodes)** | **3.21 μs** | **~310,800 ops/sec** |
| **DAG Validation (100 Nodes)** | **34.35 μs** | **~29,100 ops/sec** |
| **Redis Enqueue (`ZADD`)** | **38.58 μs** | **~25,900 ops/sec** |
| **Atomic Lua Claim (6:3:1)** | **635.48 μs** | **~1,570 ops/sec** |
| **Redis Ack (`ZREM`)** | **78.54 μs** | **~12,730 ops/sec** |

See [`docs/benchmarks.md`](docs/benchmarks.md) for full benchmark methodology and alloc statistics.

---

## Running Automated & Chaos Tests

```bash
cd backend

# Run all unit, integration, and failure recovery tests
go test -v ./...

# Run the Chaos Test suite (concurrent competing workers, crash recovery, starvation mitigation)
go test -v -run "TestChaos_" ./internal/engine/...

# Run performance benchmarks with memory allocations
go test -bench="." -benchmem ./internal/domain ./internal/queue
```

---

## API Reference Summary

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/jobs` | Submit a new job (supports `Idempotency-Key` header) |
| `GET` | `/api/v1/jobs` | List jobs with status, priority, and pagination filters |
| `GET` | `/api/v1/jobs/{id}` | Get full job state, payload, attempts, and error details |
| `POST` | `/api/v1/jobs/{id}/cancel` | Cancel a queued or running job |
| `POST` | `/api/v1/jobs/{id}/retry` | Re-enqueue a failed or dead-letter job |
| `POST` | `/api/v1/workflows` | Submit and validate a DAG workflow |
| `GET` | `/api/v1/workflows` | List workflows with status filters |
| `GET` | `/api/v1/workflows/{id}` | Get workflow details with all task states |
| `POST` | `/api/v1/workflows/{id}/cancel` | Cancel a workflow and its active tasks |
| `GET` | `/api/v1/workers` | List active, draining, and offline workers |
| `GET` | `/api/v1/queues` | Get queue backlog grouped by priority (P1, P2, P3) |
| `GET` | `/api/v1/stats` | System-wide execution statistics |
| `GET` | `/metrics` | Prometheus metrics scrape endpoint |
| `GET` | `/health` | Service health status |

---

## License
MIT
#   D i s t r i b u t e d - W o r k f l o w - E n g i n e  
 