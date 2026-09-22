-- Index for fast polling of jobs due for retry
CREATE INDEX IF NOT EXISTS idx_jobs_retrying ON jobs (run_at)
    WHERE status = 'RETRYING';
