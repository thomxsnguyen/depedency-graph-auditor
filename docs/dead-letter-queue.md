# Dead-Letter Queue (DLQ)

The dead-letter queue is a holding pen for jobs that have permanently failed. After a job exhausts its retry cap, it moves here instead of retrying forever. The live queue stays clear; the failed work is preserved for investigation.

---

## The Problem the DLQ Solves

Without a DLQ, a permanently failing job has two bad outcomes:

| Strategy | What happens |
|---|---|
| **Retry forever** | The job retries indefinitely, consuming a worker slot on every attempt, never succeeding. A single bad package blocks resources forever. |
| **Drop it** | The job disappears. The audit report is silently incomplete — a missing package with no record it was ever attempted. |

The DLQ is the third option: **stop retrying, but don't lose the evidence.**

---

## How It Works

```
Job fails → retry engine checks attempt count

    Attempts < MaxRetries?
        → Yes: schedule retry with backoff (stays in main queue)
        → No:  move to dead-letter queue

Dead-lettered job:
    - Status set to "DeadLettered"
    - Error message preserved
    - Attempt count preserved
    - Original payload preserved
    - Job stays in Postgres, queryable and inspectable
```

A dead-lettered job is **inert** — no worker will pick it up. It sits in the database until someone investigates.

---

## What's Stored

A dead-lettered job retains everything needed to understand why it failed and to reprocess it later if desired:

| Field | Purpose |
|---|---|
| `ID` | Which job |
| `Payload` | The original input (e.g., `{"name": "deleted-pkg", "version": "1.0.0"}`) |
| `Attempts` | How many times it was tried |
| `Error` | The last failure message (e.g., `"404 Not Found"`, `"connection timeout"`) |
| `CreatedAt` | When the job was first submitted |
| `UpdatedAt` | When it was dead-lettered |

---

## What the DLQ Enables

### 1. The audit completes

Without the DLQ, a permanently failing job either blocks the system (infinite retries) or silently disappears (dropped). With the DLQ, the failing job is removed from the live queue, the remaining jobs continue, and the audit finishes.

### 2. A clear failure record

The audit report can include a section: "These packages could not be resolved." Each entry comes directly from the DLQ — the package name, version, and the reason it failed. Nothing is silently missing.

### 3. Manual investigation

After the audit, an operator can query the DLQ:

```sql
SELECT payload, error, attempts FROM jobs WHERE status = 'DeadLettered';
```

This shows exactly what failed and why — was the package deleted from the registry? Was it a persistent network issue? Was the version malformed?

### 4. Reprocessing

If the root cause is fixed (e.g., the registry is back up, a missing package is republished), dead-lettered jobs can be moved back to `Pending` and retried:

```sql
UPDATE jobs SET status = 'Pending', attempts = 0
WHERE status = 'DeadLettered';
```

The queue picks them up and processes them as normal.

---

## Design Decisions

### 1. DLQ as a status, not a separate table

**Decision:** Dead-lettered jobs live in the same `jobs` table with `status = 'DeadLettered'`. There is no separate DLQ table.

**Tradeoff:** Simpler schema — one table, one set of queries. The DLQ is just a filtered view of the jobs table. A separate table would provide physical isolation (DLQ can't interfere with live query performance) but adds complexity with no meaningful benefit at this scale.

### 2. Preserve the full job, not just the error

**Decision:** The entire job — payload, attempt count, error, timestamps — is kept intact when dead-lettered.

**Tradeoff:** Uses more storage than recording only the error, but makes reprocessing possible without recreating the job from scratch. The storage cost is negligible since dead-lettered jobs are a small fraction of total jobs.

### 3. Dead-letter is final until manual intervention

**Decision:** Once a job is dead-lettered, it never automatically retries. Only a manual action (operator query or API call) can move it back to `Pending`.

**Tradeoff:** A job that would succeed if retried one more time stays dead-lettered until someone notices. But automatic recovery from the DLQ would undermine its purpose — the DLQ exists precisely to stop retrying jobs that keep failing. If the system automatically retried dead-lettered jobs, it would just be a higher retry cap, not a DLQ.
