# Phase 4 — Durable Storage (Postgres)

Phase 4 upgrades the system from an in-memory toy to crash-safe infrastructure.
Jobs, their retry state, and dead-lettered entries are written to Postgres so
that a process restart does not silently discard in-flight work. A long audit
interrupted mid-run resumes from its frontier rather than starting over.

> **What's proven at the end of Phase 4:**
> A process killed mid-audit can be restarted and will continue from where it
> left off. No jobs are lost. The DLQ is queryable after a restart. Durable
> backoff (via `ScheduledAt`) replaces the in-memory sleep, so a scheduled
> retry also survives a crash.

---

## Local Development Setup

Phase 4 requires a running Postgres instance. Docker Compose is the recommended
approach — no native Postgres install needed.

### Files

```
docker-compose.yml    ← starts Postgres, mounts the migration, exposes 5432
.env.example          ← copy to .env; contains DATABASE_URL
db/
└── migrations/
    └── 001_jobs.sql  ← auto-run by Postgres on first container start
```

### `docker-compose.yml`

```yaml
services:
  postgres:
    image: postgres:16-alpine
    container_name: job-queue-pg
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: jobqueue
    ports:
      - "5432:5432"
    volumes:
      - pg_data:/var/lib/postgresql/data
      - ./db/migrations:/docker-entrypoint-initdb.d   # auto-runs *.sql on first start
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d jobqueue"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  pg_data:
```

### `.env.example` (copy to `.env`)

```
DATABASE_URL=postgres://postgres:postgres@localhost:5432/jobqueue?sslmode=disable
```

The `DATABASE_URL` value matches the `docker-compose.yml` credentials exactly.
Your Go code reads this variable to open the `pgx` / `database/sql` connection.

### How the migration runs

Postgres's official image automatically executes any `*.sql` files placed in
`/docker-entrypoint-initdb.d/` **on the very first startup** (i.e., when the
`pg_data` volume is empty). The Compose volume mount maps
`./db/migrations → /docker-entrypoint-initdb.d`, so `001_jobs.sql` creates the
`jobs` and `dlq` tables without any manual `psql` step.

> [!NOTE]
> The init scripts only run once. To re-run them (e.g., to reset the schema),
> delete the volume: `docker compose down -v` then `docker compose up -d`.

### Workflow commands

```bash
# First time (or after a reset)
docker compose up -d          # pulls image, creates volume, runs migration

# Day-to-day
docker compose up -d          # start Postgres in the background
docker compose stop           # pause without deleting data

# Inspect
docker compose ps             # check container is healthy
docker compose logs postgres  # view Postgres output

# Full reset (drops all data)
docker compose down -v
docker compose up -d

# Unit tests — no Postgres needed
go test -race ./...

# Integration tests — Postgres must be running
go test -race -tags=integration ./...
```

---

## Scope

### In scope

| Concern | Detail |
|---|---|
| **`jobs` table** | Persist every job: ID, type, payload, status, attempts, max_attempts, scheduled_at, created_at |
| **`dlq` table** | Persist every dead-lettered entry: job snapshot, error, dead_at |
| **Job store interface** | `Store` — abstraction over Postgres so tests can use a fake |
| **Queue backed by store** | On submit, write `pending`; on dequeue, mark `running`; on close/crash, `running` rows reclaimed as `pending` on next startup |
| **`ScheduledAt` field on Job** | Replaces `time.Sleep` in the retry path; worker writes `scheduled_at = now + backoff`, a poller picks the job up when `scheduled_at <= now` |
| **Startup reclamation** | On boot, any job in `running` state (from a previous crashed process) is reset to `pending` |
| **DLQ backed by store** | `DLQ.Publish` writes to the `dlq` table instead of in-memory slice; `DLQ.Entries` queries the table |

### Out of scope (later phases)

| Concern | Phase |
|---|---|
| DLQ replay / requeue API | Phase 4+ |
| Graceful shutdown with drain + timeout | Phase 5 |
| Horizontal scaling / multiple processes sharing one queue | Post-Phase 5 |

---

## What Changes From Phase 3

| Component | Change |
|---|---|
| `internal/job/job.go` | Add `ScheduledAt time.Time` field |
| `internal/store/store.go` | **[NEW]** `Store` interface |
| `internal/store/postgres/postgres.go` | **[NEW]** Postgres implementation of `Store` |
| `internal/queue/queue.go` | **[MODIFY]** Accept a `Store`; submit/dequeue/reclaim through it |
| `internal/dlq/dlq.go` | **[MODIFY]** Accept a `Store`; `Publish` and `Entries` go through it |
| `internal/worker/pool.go` | **[MODIFY]** Replace `time.Sleep` + `queue.Submit` retry with `store.SetScheduledAt` |
| `db/migrations/001_jobs.sql` | **[NEW]** Schema for `jobs` and `dlq` tables |

---

## Project Layout

```
internal/
├── job/
│   └── job.go                  ← [MODIFY] add ScheduledAt time.Time field
├── store/
│   ├── store.go                ← [NEW] Store interface
│   └── postgres/
│       └── postgres.go         ← [NEW] Postgres implementation
├── queue/
│   └── queue.go                ← [MODIFY] accept Store; persist submit/dequeue/reclaim
├── dlq/
│   └── dlq.go                  ← [MODIFY] accept Store; Publish/Entries via DB
└── worker/
    └── pool.go                 ← [MODIFY] retry writes ScheduledAt instead of sleeping
db/
└── migrations/
    └── 001_jobs.sql            ← [NEW] jobs + dlq schema
```

---

## Component Specifications

### 1. Schema (`db/migrations/001_jobs.sql`)

```sql
CREATE TABLE jobs (
    id            TEXT        PRIMARY KEY,
    type          TEXT        NOT NULL,
    payload       JSONB,
    status        TEXT        NOT NULL DEFAULT 'pending',
    attempts      INT         NOT NULL DEFAULT 0,
    max_attempts  INT         NOT NULL DEFAULT 5,
    scheduled_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_jobs_status_scheduled ON jobs (status, scheduled_at)
    WHERE status = 'pending';

CREATE TABLE dlq (
    id         BIGSERIAL   PRIMARY KEY,
    job_id     TEXT        NOT NULL,
    job_type   TEXT        NOT NULL,
    payload    JSONB,
    attempts   INT         NOT NULL,
    error      TEXT        NOT NULL,
    dead_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

The partial index on `(status, scheduled_at)` covers the hot polling query:
`SELECT ... FROM jobs WHERE status = 'pending' AND scheduled_at <= NOW()`.

---

### 2. Job struct update (`internal/job/job.go`)

```go
type Job struct {
    ID          string
    Type        string
    Payload     json.RawMessage
    Status      Status
    Attempts    int
    MaxAttempts int
    ScheduledAt time.Time  // ← NEW: zero value means "run immediately"
}
```

`ScheduledAt` is zero for all newly created jobs (run immediately). The worker
sets it to `now + Backoff(attempt)` on retry instead of sleeping.

---

### 3. Store interface (`internal/store/store.go`)

```go
// Store is the persistence contract for the job queue and DLQ.
// The Postgres implementation writes to the DB; a fake is used in tests.
type Store interface {
    // Job lifecycle
    CreateJob(ctx context.Context, j job.Job) error
    AcquireJob(ctx context.Context) (job.Job, bool, error)  // marks status=running
    CompleteJob(ctx context.Context, id string) error
    RetryJob(ctx context.Context, j job.Job) error           // updates attempts + scheduled_at
    DeadLetterJob(ctx context.Context, j job.Job, err error) error

    // Startup
    ReclaimStuckJobs(ctx context.Context) (int, error)  // running → pending

    // DLQ
    DLQEntries(ctx context.Context) ([]dlq.DLQEntry, error)
}
```

The interface is the boundary between the queue/pool and Postgres. Tests
implement a simple in-memory fake — no database required.

---

### 4. Queue update (`internal/queue/queue.go`)

The channel remains the dispatch mechanism (workers still block on `Dequeue`),
but every state transition is now mirrored to the store:

| Operation | Before (Phase 3) | After (Phase 4) |
|---|---|---|
| `Submit` | `ch <- j` | `store.CreateJob` then `ch <- j` |
| `Dequeue` | `<-ch` | `<-ch` then `store.AcquireJob` (marks running) |
| Startup | nothing | `store.ReclaimStuckJobs` resets running → pending, reloads pending into channel |

`ReclaimStuckJobs` is called once at startup before any workers are started.
It finds jobs left in `running` state by a previously crashed process and
resets them to `pending` so they re-enter the channel.

---

### 5. Worker pool update (`internal/worker/pool.go`)

The retry path changes from sleep-then-requeue to write-ScheduledAt:

```go
// Phase 3 (before)
delay := p.backoff(j.Attempts)
time.Sleep(delay)
p.queue.Submit(j)  // re-queue directly

// Phase 4 (after)
j.ScheduledAt = time.Now().Add(p.backoff(j.Attempts))
p.store.RetryJob(ctx, j)  // writes attempts + scheduled_at, status=pending
// inFlight is NOT decremented — the poller will re-deliver the job
```

A **poll loop** goroutine (started alongside the workers) queries the store
for `pending` jobs with `scheduled_at <= now` and pushes them into the queue
channel. The `inFlight` counter is not decremented on retry — the job is still
considered in-flight until it either completes or exhausts attempts.

#### Why replace sleep with ScheduledAt?

| | Phase 3 (sleep) | Phase 4 (ScheduledAt) |
|---|---|---|
| Crash during sleep | Job is lost | Job row has `scheduled_at` set; poller picks it up on restart |
| Worker goroutine blocked | Yes, for up to 30s | No — goroutine is free immediately |
| Backoff survives restart | No | Yes |

---

## Retry Flow with Durable Backoff (End-to-End)

```mermaid
sequenceDiagram
    participant P as Pool
    participant S as Store (Postgres)
    participant Poll as Poll Loop
    participant W as Worker
    participant H as Handler

    P->>S: CreateJob (status=pending, scheduled_at=now)
    Poll->>S: SELECT pending WHERE scheduled_at <= now
    S-->>Poll: job
    Poll->>W: channel ← job
    W->>S: AcquireJob (status=running)
    W->>H: Handle(job)
    H-->>W: error

    note over W: attempts++ → 1 < MaxAttempts
    W->>S: RetryJob (status=pending, scheduled_at=now+backoff(1))
    note over W: inFlight unchanged — job still counted

    Poll->>S: SELECT pending WHERE scheduled_at <= now+backoff
    S-->>Poll: job (after delay elapses)
    Poll->>W: channel ← job
    W->>H: Handle(job)
    H-->>W: success
    W->>S: CompleteJob (status=completed)
    note over W: inFlight-- → checkDone()
```

---

## Testing Strategy

### Unit tests (fake store, no Postgres required)

| Component | What's tested |
|---|---|
| `Store` fake | All interface methods behave correctly in memory |
| `ReclaimStuckJobs` | Jobs in `running` are reset to `pending`; `pending`/`completed` are untouched |
| `RetryJob` | `scheduled_at` and `attempts` are updated; status becomes `pending` |
| `AcquireJob` | Returns a job and marks it `running`; returns `false` when none available |
| `DeadLetterJob` | Job row gets `dead_lettered`; DLQ table gets an entry |
| Queue reclaim integration | After reclaim, previously-running jobs re-enter the channel |
| Pool retry path | Worker calls `RetryJob` instead of sleeping; `inFlight` unchanged |

### Integration tests (real Postgres via `testcontainers-go` or `pgtest`)

| Test | What's verified |
|---|---|
| Crash-and-resume | Kill pool mid-run, restart, verify all jobs complete |
| Durable backoff | Job fails, process restarts before backoff elapses, job retried after `scheduled_at` |
| DLQ persistence | Exhausted job appears in `dlq` table after restart |

### What is NOT tested

- Multi-process coordination (out of scope until post-Phase 5).
- Migration rollback.

---

## Exit Criteria

Phase 4 is done when:

- [ ] A process killed mid-audit can be restarted; `ReclaimStuckJobs` resets
      `running` jobs to `pending` and the audit completes correctly
- [ ] A job that fails and has `scheduled_at` set in the future is not
      re-delivered until that time has elapsed
- [ ] A failed job's `scheduled_at` and `attempts` survive a process restart
- [ ] The `dlq` table contains entries for exhausted jobs after a restart
- [ ] `go test -race ./...` passes with the fake store — no Postgres required
      for the unit test suite
- [ ] The integration crash-and-resume test passes against a real Postgres
      instance (via testcontainers or a local dev database)
- [ ] The auditor smoke test passes end-to-end — durable storage is transparent
      when no crashes occur
