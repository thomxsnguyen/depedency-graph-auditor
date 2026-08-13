# Retry Engine

When a job fails for a transient reason — a network timeout, a 5xx response, a rate-limit rejection — the retry engine decides **when** to try again and **when to stop trying**.

---

## The Problem Retries Solve

Without retries, a single network blip permanently loses a job. The registry was down for 2 seconds, and an entire subtree of the dependency graph is silently missing from the audit.

But retrying naively creates worse problems:

| Retry strategy | What happens |
|---|---|
| **No retries** | One blip = lost work |
| **Retry immediately** | You hammer the already-struggling downstream, making the outage worse |
| **Retry immediately, forever** | A permanently broken job loops infinitely, wasting a worker slot forever |

The retry engine solves all three: retry **with delays**, and **give up** after a cap.

---

## Exponential Backoff

Each retry waits longer than the last. The delay doubles every attempt:

```
Attempt 1 fails → wait 1 second
Attempt 2 fails → wait 2 seconds
Attempt 3 fails → wait 4 seconds
Attempt 4 fails → wait 8 seconds
Attempt 5 fails → wait 16 seconds
```

**Formula:**

```
delay = baseDelay × 2^(attempt - 1)
```

With a `baseDelay` of 1 second:

| Attempt | Delay |
|---|---|
| 1 | 1s |
| 2 | 2s |
| 3 | 4s |
| 4 | 8s |
| 5 | 16s |

### Why exponential, not linear?

Linear (wait 1s, 2s, 3s, 4s...) backs off too slowly. If the downstream needs minutes to recover, linear retries still hammer it every few seconds. Exponential backs off aggressively — after a few attempts, you're waiting long enough to give the downstream real breathing room.

---

## Jitter

Jitter adds a **random offset** to the delay so that jobs which failed at the same time don't all retry at the same instant.

### The thundering herd problem

Without jitter:

```
10 jobs all fail at t=0
All 10 retry at t=1s   → burst of 10 requests
All 10 fail again
All 10 retry at t=3s   → burst of 10 requests
...the bursts re-create the exact problem that caused the failures
```

With jitter:

```
10 jobs all fail at t=0
Job 1 retries at t=0.8s
Job 2 retries at t=1.3s
Job 3 retries at t=0.6s
Job 4 retries at t=1.1s
...retries are spread out, downstream sees steady trickle instead of bursts
```

### Formula

```
delay = (baseDelay × 2^(attempt - 1)) + random(0, jitterMax)
```

The random component ensures no two jobs retry at the same instant, even if they failed at the same instant.

---

## Retry Cap

After a fixed number of attempts, the job stops retrying and moves to the dead-letter queue.

```
MaxRetries = 5

Attempt 1 → fail → retry
Attempt 2 → fail → retry
Attempt 3 → fail → retry
Attempt 4 → fail → retry
Attempt 5 → fail → dead-letter queue (stop trying)
```

### Why cap?

Some failures are not transient. A package was deleted from the registry. The URL is permanently wrong. Without a cap, this job retries forever — doubling its delay each time but never succeeding, occupying a worker slot on every attempt for eternity.

The cap says: "If you've failed 5 times, you're probably not going to succeed on attempt 6. Stop wasting resources and preserve the failure for someone to investigate."

---

## How It Integrates

When a worker nacks a job, the retry engine runs this logic:

```
OnFailure(job, error):
    job.Attempts += 1

    if job.Attempts >= job.MaxRetries:
        job.Status = DeadLettered
        → move to dead-letter queue
        → done

    delay = baseDelay × 2^(job.Attempts - 1) + random(0, jitterMax)
    job.Status = Pending
    job.ScheduledAt = now + delay

    → save to Postgres
    → after delay, push back onto the channel
```

The `ScheduledAt` field prevents the job from being picked up before its backoff delay has elapsed. Workers skip jobs where `ScheduledAt` is in the future.

---

## Design Decisions

### 1. Exponential backoff, not linear or fixed

**Decision:** Delays double each attempt.

**Tradeoff:** Exponential backs off aggressively, which means later retries wait a long time (attempt 8 = ~4 minutes). This is intentional — if the downstream hasn't recovered after 7 attempts, you need to give it real time, not keep poking every few seconds. The downside is that a job which would succeed on a quick retry waits longer than necessary on later attempts.

### 2. Full jitter, not no jitter

**Decision:** Add a random offset to every delay.

**Tradeoff:** Slightly less predictable timing (harder to reason about exact retry schedules in logs), but eliminates thundering herd — the scenario where correlated retries re-create the surge that caused the original failure. The randomness is what breaks the correlation.

### 3. Retry cap with dead-letter, not infinite retries

**Decision:** After `MaxRetries` failures, stop retrying and move to the dead-letter queue.

**Tradeoff:** A job that would eventually succeed on attempt 6 is permanently shelved. But the alternative — infinite retries — means a single permanently broken job consumes worker time forever. The dead-letter queue preserves the failure for manual investigation without blocking the live system.

### 4. Delay via ScheduledAt, not sleep

**Decision:** The backoff delay is stored as a `ScheduledAt` timestamp in Postgres, not implemented as a `time.Sleep()` in the worker goroutine.

**Tradeoff:** If the system crashes during a backoff wait, the delay survives — on restart, the job's `ScheduledAt` is still in the future and it won't be picked up early. A `time.Sleep()` in a goroutine would be lost on crash, and the job would retry immediately on restart, defeating the backoff.
