-- Workflows table: represents a multi-task DAG workflow.
CREATE TABLE IF NOT EXISTS workflows (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'PENDING',
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Workflow tasks table: individual steps in a workflow, backed by a Job when scheduled.
CREATE TABLE IF NOT EXISTS workflow_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    job_id          UUID REFERENCES jobs(id) ON DELETE SET NULL,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'PENDING',
    dependencies    TEXT[] NOT NULL DEFAULT '{}',
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_workflow_task_name UNIQUE (workflow_id, name)
);

CREATE INDEX idx_workflows_status ON workflows (status);
CREATE INDEX idx_workflow_tasks_workflow_id ON workflow_tasks (workflow_id);
CREATE INDEX idx_workflow_tasks_job_id ON workflow_tasks (job_id) WHERE job_id IS NOT NULL;
CREATE INDEX idx_workflow_tasks_status ON workflow_tasks (workflow_id, status);
