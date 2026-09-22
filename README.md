<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/Next.js-16-000000?style=for-the-badge&logo=next.js&logoColor=white" alt="Next.js" />
  <img src="https://img.shields.io/badge/PostgreSQL-16-4169E1?style=for-the-badge&logo=postgresql&logoColor=white" alt="PostgreSQL" />
  <img src="https://img.shields.io/badge/Redis-7-DC382D?style=for-the-badge&logo=redis&logoColor=white" alt="Redis" />
  <img src="https://img.shields.io/badge/Docker-Compose-2496ED?style=for-the-badge&logo=docker&logoColor=white" alt="Docker" />
  <img src="https://img.shields.io/badge/Prometheus-Metrics-E6522C?style=for-the-badge&logo=prometheus&logoColor=white" alt="Prometheus" />
</p>

<h1 align="center">⚡ Distributed Workflow Engine</h1>

<p align="center">
  <strong>A high-performance, fault-tolerant distributed job processor &amp; DAG workflow orchestrator</strong>
</p>

<p align="center">
  <em>Built with Go · PostgreSQL · Redis · Next.js</em>
</p>

<p align="center">
  <a href="#-quickstart"><strong>Quickstart</strong></a> ·
  <a href="#-architecture"><strong>Architecture</strong></a> ·
  <a href="#-features"><strong>Features</strong></a> ·
  <a href="#-api-reference"><strong>API Reference</strong></a> ·
  <a href="#-benchmarks"><strong>Benchmarks</strong></a>
</p>

---

## 🎯 Overview

A production-grade distributed workflow engine designed from the ground up with distributed systems fundamentals:

- **At-least-once delivery** with visibility timeout crash recovery
- **Zero-starvation priority scheduling** via weighted 6:3:1 slot rotation
- **Exponential backoff with full jitter** to prevent thundering herds
- **Atomic PostgreSQL state transitions** for data integrity
- **Kahn's topological sort** for DAG cycle detection ($O(V + E)$)
- **Real-time Next.js control plane** with live monitoring dashboard

---

## 🏗 Architecture

```
                           ┌─────────────────────────────┐
                           │    Next.js Control Plane     │
                           │    Dashboard (Port 3000)     │
                           └──────────────┬──────────────┘
                                          │ HTTP / JSON
                                          ▼
                           ┌─────────────────────────────┐
                           │      Go API & Scheduler     │
                           │         (Port 8080)         │
                           └───────┬─────────────┬───────┘
                                   │             │
                  State & Metadata │             │ Priority Queue Dispatch
                                   ▼             ▼
                       ┌──────────────┐   ┌──────────────┐
                       │  PostgreSQL  │   │ Redis (ZSET) │
                       │  Source of   │   │  Ephemeral   │
                       │    Truth     │   │ Priority Qs  │
                       └──────────────┘   └──────┬───────┘
                                                 │
                                   Claim / Ack   │  Visibility Timeout
                                                 ▼
                                  ┌────────────────────────┐
                                  │     Worker Clusters    │
                                  │  (Concurrency Pool,    │
                                  │   Graceful Draining)   │
                                  └────────────────────────┘
```

> **Dual-store design** — PostgreSQL is the source of truth for all job state, metadata, and audit trails. Redis serves as the ephemeral high-throughput dispatch layer for priority queue ordering, visibility timeouts, and worker heartbeats.

---

## ✨ Features

### ⚙️ Multi-Priority Queueing with Starvation Mitigation

Redis sorted sets (`queue:{name}:priority:{1,2,3}`) maintain strict FIFO ordering by timestamp within each priority tier. An **atomic Lua script** implements a 10-slot weighted cycle:

| Slots | Priority Searched First | Weight |
|:---:|---|:---:|
| 0 — 5 | 🔴 **High** (P1) | 6 |
| 6 — 8 | 🟡 **Medium** (P2) | 3 |
| 9 | 🟢 **Low** (P3) | 1 |

Lower-priority jobs always make progress, even under massive P1 floods — with zero latency waste on empty queues via instant fallback.

---

### 🔄 At-Least-Once Delivery & Crash Recovery

```
  ┌─────────┐    CLAIM     ┌──────────────┐    ACK     ┌───────────┐
  │  Queue   │ ──────────► │ In-Progress  │ ────────► │ Completed  │
  │  (ZSET)  │             │   (ZSET)     │           │  (PG)      │
  └─────────┘             └──────┬───────┘           └───────────┘
                                  │
                    Visibility     │  Timeout Expired?
                    Timeout        ▼
                           ┌──────────────┐
                           │  RequeueStale │ ──► Back to Queue
                           └──────────────┘
```

- Claimed jobs are atomically moved into an `in-progress` sorted set with score = `now + visibility_timeout`
- If a worker crashes before acknowledging, `RequeueStale` automatically recovers orphaned jobs
- **Zero data loss** — verified under chaos testing with 15 concurrent competing workers

---

### 📈 Exponential Backoff with Full Jitter

Failed jobs transition to `RETRYING` with randomized exponential delays:

$$\text{delay} = \text{rand}\Big(0,\ \min\big(\text{maxDelay},\ \text{baseDelay} \times 2^{\text{attempt}-1}\big)\Big)$$

- Prevents the **thundering herd problem** when shared dependencies fail
- Permanently failing jobs move to `DEAD_LETTER` for manual inspection and replay
- Configurable `baseDelay` (default: 1s) and `maxDelay` (default: 1h)

---

### 🔀 DAG Workflow Orchestration

Define multi-step workflows with task dependencies as a Directed Acyclic Graph:

```json
{
  "name": "data-pipeline",
  "tasks": [
    { "name": "extract",   "type": "etl.extract",   "payload": {} },
    { "name": "transform", "type": "etl.transform", "payload": {}, "dependencies": ["extract"] },
    { "name": "validate",  "type": "etl.validate",  "payload": {}, "dependencies": ["extract"] },
    { "name": "load",      "type": "etl.load",      "payload": {}, "dependencies": ["transform", "validate"] }
  ]
}
```

- **Pre-submission cycle detection** via Kahn's algorithm
- **Automatic parallelism** — independent root tasks run simultaneously
- **Cascading failure handling** — if any task fails, remaining pending tasks are cancelled
- **Idempotent task execution** — each task creates a job with workflow-scoped idempotency keys

---

### 🛡️ Idempotency & Deduplication

```
Client ──► POST /api/v1/jobs
           Header: Idempotency-Key: payment-xyz-123

           ┌─ New Key? ──► INSERT ... ON CONFLICT DO NOTHING ──► 201 Created
           └─ Existing? ──► Return cached result ──► 200 OK + X-Idempotent-Replay: true
```

Both **HTTP header** (`Idempotency-Key`) and **JSON body** (`idempotency_key`) are supported. PostgreSQL's atomic `INSERT ... ON CONFLICT` prevents duplicate execution races — no double-enqueue, no double-execution.

---

### 🖥️ Real-Time Control Plane Dashboard

An interactive Next.js dashboard providing full observability:

| Tab | Capabilities |
|---|---|
| **Overview** | System-wide stats, job status distribution, queue backlog depth |
| **Workflows** | Create, monitor, and cancel DAG workflows with task-level state tracking |
| **Jobs** | Filter by status/priority, inspect payloads, manual retry & cancellation |
| **Workers** | Live heartbeat monitoring, completed/failed counters, draining status |
| **Queues** | Priority-level backlog (P1/P2/P3), in-progress counts |

---

### 📊 Production Observability

Prometheus-compatible metrics exporter at `/metrics`:

| Metric | Type | Description |
|---|---|---|
| `workflow_engine_jobs_total` | Counter | Jobs by type and terminal status |
| `workflow_engine_jobs_duration_seconds` | Histogram | Job execution durations |
| `workflow_engine_queue_depth` | Gauge | Queue backlog by priority |
| `workflow_engine_workers_active` | Gauge | Active worker count |
| `workflow_engine_workflows_total` | Counter | Workflows by status |

---

## 🚀 Quickstart

### One Command with Docker Compose

```bash
docker compose up --build
```

| Service | URL |
|---|---|
| 🖥️ **Dashboard** | [http://localhost:3000](http://localhost:3000) |
| ⚡ **API Server** | [http://localhost:8080](http://localhost:8080) |
| 📊 **Prometheus Metrics** | [http://localhost:8080/metrics](http://localhost:8080/metrics) |
| 💚 **Health Check** | [http://localhost:8080/health](http://localhost:8080/health) |

---

## 🛠️ Local Development

### Prerequisites

| Tool | Version |
|---|---|
| Go | 1.22+ |
| Node.js | 20+ |
| PostgreSQL | 16 |
| Redis | 7 |

> **Tip:** Run `docker compose up postgres redis` to spin up only the infrastructure services.

### 1️⃣ Run the Backend API

```bash
cd backend
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/workflow_engine?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"
go run ./cmd/api
```

### 2️⃣ Run Worker Processes

```bash
cd backend
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/workflow_engine?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"
export CONCURRENCY=5
go run ./cmd/worker
```

### 3️⃣ Run the Dashboard

```bash
cd frontend
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) 🎉

---

## 📋 API Reference

### Jobs

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/jobs` | Submit a new job (supports `Idempotency-Key` header) |
| `GET` | `/api/v1/jobs` | List jobs with `status`, `type`, `queue`, `priority` filters |
| `GET` | `/api/v1/jobs/{id}` | Get job state, payload, attempts, and error details |
| `POST` | `/api/v1/jobs/{id}/cancel` | Cancel a queued or running job |
| `POST` | `/api/v1/jobs/{id}/retry` | Re-enqueue a failed or dead-letter job |

### Workflows

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/workflows` | Submit and validate a DAG workflow |
| `GET` | `/api/v1/workflows` | List workflows with status filters |
| `GET` | `/api/v1/workflows/{id}` | Get workflow details with all task states |
| `POST` | `/api/v1/workflows/{id}/cancel` | Cancel a workflow and its pending tasks |

### System

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/workers` | List active, draining, and offline workers |
| `GET` | `/api/v1/queues` | Queue backlog grouped by priority (P1, P2, P3) |
| `GET` | `/api/v1/stats` | System-wide execution statistics |
| `GET` | `/metrics` | Prometheus metrics scrape endpoint |
| `GET` | `/health` | Service health status |

### Example: Submit a Job

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: unique-key-123" \
  -d '{
    "type": "email.send",
    "payload": { "to": "user@example.com", "subject": "Hello!" },
    "priority": 1,
    "max_attempts": 5
  }'
```

### Example: Create a DAG Workflow

```bash
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "name": "onboarding-pipeline",
    "tasks": [
      { "name": "create-account", "type": "account.create", "payload": {} },
      { "name": "send-welcome",   "type": "email.welcome",  "payload": {}, "dependencies": ["create-account"] },
      { "name": "provision-infra", "type": "infra.setup",   "payload": {}, "dependencies": ["create-account"] },
      { "name": "notify-team",    "type": "slack.notify",   "payload": {}, "dependencies": ["send-welcome", "provision-infra"] }
    ]
  }'
```

---

## ⚡ Benchmarks

Measured on a 12th Gen Intel Core i7-1260P (16 threads) · Go 1.27 · `testing.B -benchmem`

### DAG Validation (Kahn's Algorithm)

| Graph Scale | Latency / Op | Throughput | Memory / Op | Allocs / Op |
|---|---|---|---|---|
| 10 Nodes | **3.21 μs** | ~310,800 ops/sec | 2,216 B | 27 |
| 50 Nodes | **14.79 μs** | ~67,600 ops/sec | 9,608 B | 107 |
| 100 Nodes | **34.35 μs** | ~29,100 ops/sec | 18,920 B | 207 |

### Redis Queue Operations

| Operation | Latency / Op | Throughput | Memory / Op | Allocs / Op |
|---|---|---|---|---|
| Enqueue (`ZADD`) | **38.58 μs** | ~25,900 ops/sec | 1,112 B | 28 |
| Atomic Lua Claim (6:3:1) | **635.48 μs** | ~1,570 ops/sec | 271,917 B | 879 |
| Acknowledge (`ZREM`) | **78.54 μs** | ~12,730 ops/sec | 650 B | 21 |

### Chaos Testing Results

| Test | Result |
|---|---|
| 15 concurrent workers, 150 jobs | ✅ **Zero duplicate claims** |
| P1 flood (200 jobs), P2/P3 starvation check | ✅ **All priorities served** (6:3:1 verified) |
| Worker crash recovery | ✅ **Orphaned jobs recovered < 100ms** |

📖 Full benchmark methodology: [`docs/benchmarks.md`](docs/benchmarks.md)

---

## 🧪 Testing

```bash
cd backend

# Run all unit, integration, and failure recovery tests with race detection
go test -v -race ./...

# Run chaos test suite (concurrent workers, crash recovery, starvation mitigation)
go test -v -run "TestChaos_" ./internal/engine/...

# Run performance benchmarks with memory allocation stats
go test -bench="." -benchmem ./internal/domain ./internal/queue
```

---

## 📁 Project Structure

```
.
├── backend/
│   ├── cmd/
│   │   ├── api/               # API server entrypoint
│   │   └── worker/            # Worker process entrypoint
│   ├── internal/
│   │   ├── api/               # HTTP handlers, routing, middleware
│   │   ├── config/            # Environment configuration
│   │   ├── domain/            # Core types (Job, Workflow, Worker)
│   │   ├── engine/            # Scheduler, Worker, Supervisor, Backoff, DAG engine
│   │   ├── metrics/           # Prometheus instrumentation
│   │   ├── queue/             # Redis queue (Lua scripts, priority dispatch)
│   │   └── store/             # PostgreSQL persistence layer
│   └── migrations/            # SQL schema migrations (embedded)
├── frontend/
│   └── src/
│       ├── app/               # Next.js app router
│       ├── components/        # Dashboard UI components
│       └── lib/               # API client
├── deployments/docker/        # Dockerfiles (API, Worker, Dashboard)
├── docs/                      # Benchmark documentation
├── .github/workflows/         # CI pipeline (lint, test, benchmark)
└── docker-compose.yml         # Full-stack orchestration
```

---

## 🔧 CI/CD

GitHub Actions pipeline runs on every push and pull request to `main`:

| Job | Checks |
|---|---|
| **Backend** | `go vet` · Race-detected tests · Benchmark suite |
| **Frontend** | `npm ci` · Production build & type check |

Infrastructure services (PostgreSQL 16 + Redis 7) are provisioned as GitHub Actions service containers.

---

## 📄 License

MIT

---

<p align="center">
  <sub>Built with ❤️ using Go, PostgreSQL, Redis, and Next.js</sub>
</p>