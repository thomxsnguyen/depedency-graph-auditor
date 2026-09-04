-- Distributed job service lifecycle, leases, history, and results.

ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS root_job_id TEXT,
    ADD COLUMN IF NOT EXISTS parent_job_id TEXT,
    ADD COLUMN IF NOT EXISTS internal BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT,
    ADD COLUMN IF NOT EXISTS request_hash TEXT,
    ADD COLUMN IF NOT EXISTS locked_by TEXT,
    ADD COLUMN IF NOT EXISTS lease_token TEXT,
    ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cancel_requested_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_error_kind TEXT,
    ADD COLUMN IF NOT EXISTS last_error TEXT,
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retried_from_job_id TEXT,
    ADD COLUMN IF NOT EXISTS replayed_from_job_id TEXT;

UPDATE jobs SET root_job_id = id WHERE root_job_id IS NULL;
ALTER TABLE jobs ALTER COLUMN root_job_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_idempotency_key
    ON jobs (idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_claimable
    ON jobs (scheduled_at, created_at, id)
    WHERE status IN ('pending', 'retry_scheduled');
CREATE INDEX IF NOT EXISTS idx_jobs_expired_leases
    ON jobs (locked_until)
    WHERE status = 'running';
CREATE INDEX IF NOT EXISTS idx_jobs_root_status
    ON jobs (root_job_id, status);

CREATE TABLE IF NOT EXISTS job_attempts (
    id            BIGSERIAL PRIMARY KEY,
    job_id        TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    attempt       INT NOT NULL,
    worker_id     TEXT NOT NULL,
    lease_token   TEXT NOT NULL,
    status        TEXT NOT NULL,
    started_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at   TIMESTAMPTZ,
    error_kind    TEXT,
    error         TEXT,
    UNIQUE (job_id, attempt)
);

CREATE TABLE IF NOT EXISTS job_events (
    id            BIGSERIAL PRIMARY KEY,
    job_id        TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    event_type    TEXT NOT NULL,
    attempt       INT,
    worker_id     TEXT,
    details       JSONB,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_job_events_job_time
    ON job_events (job_id, occurred_at, id);

CREATE TABLE IF NOT EXISTS job_results (
    job_id         TEXT PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
    schema_version INT NOT NULL DEFAULT 1,
    result         JSONB NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS workers (
    id             TEXT PRIMARY KEY,
    started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    heartbeat_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_results (
    id             BIGSERIAL PRIMARY KEY,
    root_job_id    TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    ecosystem      TEXT NOT NULL DEFAULT 'npm',
    package_name   TEXT NOT NULL,
    package_version TEXT NOT NULL,
    license        TEXT NOT NULL,
    verdict        TEXT NOT NULL,
    parent_name    TEXT,
    parent_version TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE audit_results
    ADD COLUMN IF NOT EXISTS ecosystem TEXT NOT NULL DEFAULT 'npm';
CREATE UNIQUE INDEX IF NOT EXISTS idx_audit_results_coordinate_parent
    ON audit_results (
        root_job_id,
        ecosystem,
        package_name,
        package_version,
        COALESCE(parent_name, ''),
        COALESCE(parent_version, '')
    );

CREATE TABLE IF NOT EXISTS audit_relationships (
    root_job_id     TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    ecosystem      TEXT NOT NULL,
    parent_name    TEXT NOT NULL,
    parent_version TEXT NOT NULL DEFAULT '',
    child_name     TEXT NOT NULL,
    child_version  TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (root_job_id, ecosystem, parent_name, parent_version, child_name, child_version)
);

ALTER TABLE dlq
    ADD COLUMN IF NOT EXISTS error_kind TEXT,
    ADD COLUMN IF NOT EXISTS root_job_id TEXT,
    ADD COLUMN IF NOT EXISTS parent_job_id TEXT,
    ADD COLUMN IF NOT EXISTS replayed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS replayed_as_job_id TEXT;
