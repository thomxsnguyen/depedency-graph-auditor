-- Phase 4: Durable job queue schema

CREATE TABLE IF NOT EXISTS jobs (
    id            TEXT        PRIMARY KEY,
    type          TEXT        NOT NULL,
    payload       JSONB,
    status        TEXT        NOT NULL DEFAULT 'pending',
    attempts      INT         NOT NULL DEFAULT 0,
    max_attempts  INT         NOT NULL DEFAULT 5,
    scheduled_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Covers the hot polling query:
-- SELECT ... FROM jobs WHERE status = 'pending' AND scheduled_at <= NOW()
CREATE INDEX IF NOT EXISTS idx_jobs_status_scheduled
    ON jobs (status, scheduled_at)
    WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS dlq (
    id         BIGSERIAL   PRIMARY KEY,
    job_id     TEXT        NOT NULL,
    job_type   TEXT        NOT NULL,
    payload    JSONB,
    attempts   INT         NOT NULL,
    error      TEXT        NOT NULL,
    dead_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
