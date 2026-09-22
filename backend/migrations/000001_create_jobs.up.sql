-- Jobs table: the central record of every unit of work.
CREATE TABLE IF NOT EXISTS jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type            TEXT NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'QUEUED',
    priority        INTEGER NOT NULL DEFAULT 2,
    max_attempts    INTEGER NOT NULL DEFAULT 3,
    attempt         INTEGER NOT NULL DEFAULT 0,
    idempotency_key TEXT UNIQUE,
    error           TEXT,
    worker_id       TEXT,
    queue           TEXT NOT NULL DEFAULT 'default',
    run_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast lookup of jobs ready for execution: status=QUEUED, ordered by priority and time.
CREATE INDEX idx_jobs_queue_status ON jobs (queue, status, priority, run_at)
    WHERE status = 'QUEUED';

-- Dashboard queries filtering by status.
CREATE INDEX idx_jobs_status ON jobs (status);

-- Idempotency lookups (already covered by UNIQUE constraint, but explicit for clarity).
-- The UNIQUE constraint on idempotency_key creates an implicit unique index.

-- Type-based filtering.
CREATE INDEX idx_jobs_type ON jobs (type);
