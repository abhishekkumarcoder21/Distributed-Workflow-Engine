-- Workers table: tracks registered worker processes, their status, and heartbeat.
CREATE TABLE IF NOT EXISTS workers (
    id              TEXT PRIMARY KEY,
    status          TEXT NOT NULL DEFAULT 'ACTIVE', -- ACTIVE, DRAINING, OFFLINE
    last_heartbeat  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_count INTEGER NOT NULL DEFAULT 0,
    failed_count    INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for dead worker detection: active workers whose heartbeats are stale.
CREATE INDEX IF NOT EXISTS idx_workers_heartbeat ON workers (status, last_heartbeat)
    WHERE status = 'ACTIVE';
