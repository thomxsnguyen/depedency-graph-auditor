# Bounded Worker Pool

The worker pool is a fixed number of goroutines that pull jobs from the queue and execute them. "Bounded" is the critical property — the pool size never changes regardless of how many jobs are waiting.

---

## Structure

```
Queue (channel)
    │
    ├──▶ Worker 1 ──▶ process(job) ──▶ ack/nack ──▶ loop back
    ├──▶ Worker 2 ──▶ process(job) ──▶ ack/nack ──▶ loop back
    ├──▶ Worker 3 ──▶ process(job) ──▶ ack/nack ──▶ loop back
    └──▶ ...
         Worker N ──▶ process(job) ──▶ ack/nack ──▶ loop back
```

Each worker runs the same loop forever:

```
1. Pull a job from the channel (block if empty)
2. Execute the job
3. On success → ack
   On failure → nack (triggers retry or dead-letter)
4. Go to 1
```

Workers are started once at system startup and run until shutdown. They are not created or destroyed per job.

---

## Why Bounded

### Unbounded: one goroutine per job

```
for job := range incomingJobs {
    go process(job)  // new goroutine every time
}
```

If 10,000 jobs arrive:
- 10,000 goroutines are spawned simultaneously
- 10,000 concurrent HTTP requests hit the downstream registry
- The registry rate-limits or rejects them
- Memory usage spikes from 10,000 active stacks
- The system is unstable under load — the exact moment it should be most stable

### Bounded: fixed pool

```
for i := 0; i < poolSize; i++ {
    go func() {
        for job := range jobQueue {
            process(job)
        }
    }()
}
```

If 10,000 jobs arrive with `poolSize = 10`:
- 10 goroutines are running, same as always
- 10 concurrent HTTP requests in flight at any moment
- 9,990 jobs wait in the queue
- The registry sees steady, predictable traffic
- Memory usage is constant regardless of queue depth

---

## Backpressure

Backpressure is the system's ability to say "slow down" without crashing. The bounded pool creates backpressure naturally:

```
Jobs arriving:    ████████████████████████  (fast burst)
Queue depth:      ░░░░░░░░░░████████████░░  (grows, then drains)
Workers active:   ██████████████████████████ (constant — always poolSize)
Registry load:    ██████████████████████████ (constant — always poolSize)
```

The queue **absorbs** the burst. The downstream **never sees it**. The system degrades gracefully — jobs take longer to complete, but nothing breaks.

Without backpressure, a burst of jobs translates directly into a burst of downstream requests, which is exactly when failures cascade.

---

## Pool Size Tradeoff

The pool size is a tuning knob with no single correct answer. It depends on what the jobs are doing and what the downstream can handle.

| Pool size | Queue drain speed | Downstream pressure | Risk |
|---|---|---|---|
| Too small (2) | Slow — jobs wait a long time | Very low | Audit takes unnecessarily long |
| Just right (10–20) | Reasonable pace | Within downstream limits | Sweet spot for most workloads |
| Too large (200) | Very fast | High — 200 concurrent requests | Registry rate-limits you, causing retries and backoffs that slow you down more than a smaller pool would |

### Factors that determine the right size

| Factor | Pushes pool size... |
|---|---|
| Downstream rate limit (e.g., 100 req/min) | Lower — stay under the limit |
| Job duration (e.g., 200ms per job) | Higher — more workers to maintain throughput |
| Available memory | Lower — each worker holds a job's data in memory |
| CPU cores available | Higher — more cores can run more goroutines truly in parallel |

### Diminishing returns

At some point, adding more workers **hurts** instead of helps. If the registry allows 100 requests/minute and each request takes 500ms, then ~1 worker can handle 2 jobs/second. 50 workers can handle 100 jobs/second — exactly the rate limit. Adding a 51st worker doesn't help; it just means one request gets rate-limited and has to retry.

---

## Worker Lifecycle

### Startup

Workers are spawned once at system initialization:

```
StartPool(size):
    for i := 0; i < size; i++:
        start goroutine running workerLoop(i)
```

### Running

Each worker loops independently. Multiple workers run in parallel on separate CPU cores. They share only the channel (which is thread-safe) and the database connection pool.

### Shutdown

On shutdown signal:
1. Close the channel — workers stop receiving new jobs
2. Each worker finishes its current in-progress job
3. Workers exit their loop and return
4. A timeout ensures a stuck worker doesn't block shutdown forever

---

## Design Decisions

### 1. Fixed pool, not auto-scaling

**Decision:** Pool size is set at startup and does not change.

**Tradeoff:** Simpler to reason about and debug. The system's concurrency is always exactly what you configured. Auto-scaling adds complexity — you need to decide when to scale up, when to scale down, and handle the risk of oscillation (scaling up triggers rate limits, which triggers scaling down, which causes a backlog, which triggers scaling up again). Fixed is correct for an MVP; auto-scaling is a future enhancement.

### 2. Workers pull, not push

**Decision:** Workers pull jobs from the channel when they're ready. The queue does not push jobs to workers.

**Tradeoff:** Pull-based means a worker only takes new work when it's finished with its current job. This is self-regulating — slow workers naturally take fewer jobs, fast workers take more. Push-based would require the queue to track which workers are busy, adding complexity with no benefit.

### 3. Shared channel, not per-worker queues

**Decision:** All workers read from a single shared channel.

**Tradeoff:** Simpler — one queue, one channel. Go channels are safe for multiple readers; exactly one worker receives each job. Per-worker queues would require a dispatcher to balance jobs across workers, adding complexity. A single channel with multiple readers is Go's idiomatic pattern for a worker pool.
