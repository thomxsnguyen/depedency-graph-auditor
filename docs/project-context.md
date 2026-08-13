# Project Context: Durable Job Queue with a Self-Feeding Dependency Auditor

## Overview

This project is a durable job queue built in Go, together with a reference workload — a
dependency and license auditor — that exercises every part of the queue under realistic
conditions. The queue is the substance of the project: a reusable system for running work
in the background, reliably, off the thread that requested it. The auditor is the proof
that it works, chosen because it stresses each of the queue's hard parts rather than
leaving them as decoration.

A job queue is the machinery behind "do this work, but not right now, and not on the thread
that asked for it." A producer submits a description of some task, returns immediately, and
a pool of workers pulls tasks off and runs them independently. The requester never blocks;
the work happens in the background, and it survives failure. The engineering value of the
project lives almost entirely in that last clause — in the unhappy paths of failure,
retries, crash-safety, and shutdown — not in the trivial happy path of jobs going in and
workers running them.

## The core system

The queue is built from five concerns, layered in order. Each is a decision that can be
defended on its own terms.

### Producers and the queue

A producer submits a job and returns instantly. The queue holds pending jobs until a worker
is free. In Go this is a channel, or a channel in front of durable storage. The purpose is
decoupling: producers and workers each run at their own pace, and a burst of submissions is
absorbed by the queue rather than overwhelming the workers or whatever the jobs touch.

### The bounded worker pool

A fixed number of worker goroutines each loop: take a job, run it, repeat. *Bounded* is the
load-bearing word. Spawning a goroutine per job means a million jobs spawn a million
goroutines and exhaust memory or hammer the downstream. A fixed pool caps concurrency no
matter how deep the queue grows — the queue lengthens, the system stays stable. That is
backpressure, and the pool size is a genuine tradeoff: too few workers drain the queue
slowly, too many exhaust the downstream.

### Retries with exponential backoff and jitter

Jobs fail for transient reasons — a network blip, a downstream service briefly down.
Failed jobs are retried rather than lost, but retrying immediately and forever turns a small
outage into a self-inflicted denial of service. So retries back off: wait one second, then
two, then four, doubling each time, plus a small random offset so that jobs which failed
together do not all retry at the same instant and re-create the surge.

### The dead-letter queue

After a capped number of failures, a job is moved to a dead-letter queue — a holding pen
for permanently failed jobs — instead of retrying indefinitely. This keeps the live system
clear while preserving the failed work for later investigation.

### Graceful shutdown

On stop, the system does not drop in-flight work. It stops accepting new jobs, lets workers
finish what they are holding, and then exits, bounded by a timeout so that a single stuck
job cannot hang shutdown forever. This is the difference between "a restart loses work" and
"a restart is safe."

### The durability upgrade

Backing the queue with Postgres or disk instead of an in-memory channel means jobs survive a
crash. This is what turns the system from a toy into something shaped like real
infrastructure, and it is what makes long-running work resumable rather than restart-from-zero.

## Delivery semantics: the central insight

The strongest idea in the project is its delivery guarantee. A worker takes a job, does the
work, and then marks it done — it acknowledges, or "acks," the job. If the worker crashes
after doing the work but before acking, the job still looks pending and runs again on
restart. The system therefore guarantees *at-least-once* delivery: every job runs one or
more times, never zero.

The consequence is that jobs must be safe to run more than once. They are either idempotent
by nature — resizing an image twice produces the same image — or explicitly deduplicated
with a unique key, so that a second run is a no-op. Exactly-once delivery is not cheaply
achievable, so the correct design is to accept at-least-once and make the work safe to
repeat. Reasoning about this tradeoff is the clearest senior-level signal the project
produces.

## Reference workload: the dependency and license auditor

The auditor audits a software project's complete transitive dependency graph. Starting from
a root project, it resolves every package the project transitively depends on and checks each
for policy violations: outdated versions, disallowed or ambiguous licenses, and known
vulnerabilities. The workload is self-feeding — resolving one package discovers its direct
dependencies, each of which becomes a new job — so the queue drives its own expansion until
the graph is fully explored.

A single job is "resolve and audit package *P* at version *V*." A worker fetches *P*'s
metadata from the registry, evaluates it against policy, and enqueues a new job for each of
*P*'s direct dependencies that has not already been seen. The output is a directed graph:
nodes are audited packages annotated with a verdict, edges are the "depends on" relationships
discovered during resolution.

This workload is a good choice precisely because it makes each of the queue's mechanics
load-bearing rather than decorative:

Deduplication becomes a correctness requirement, not an optimization. Real dependency graphs
contain diamonds — two packages depending on a common library — and cycles, where a package
transitively depends back on itself. Without a shared "seen" set keyed by package and version,
the same package is enqueued repeatedly and cyclic graphs never terminate. The at-least-once
idempotency property is therefore what makes the traversal halt at all.

Bounded concurrency provides real backpressure, because resolution is I/O-bound against a
rate-limited registry. A fixed pool caps how many registry requests are in flight regardless
of how large the graph grows, so an enormous dependency closure expands the queue without
overwhelming the registry or the local machine.

Retries with backoff absorb genuine transient failures — registry timeouts, 5xx responses,
and rate-limit rejections — so a brief registry hiccup does not silently drop an entire
subtree of the graph. A package that repeatedly fails to resolve, because it was removed from
the registry or is malformed, lands in the dead-letter queue after the retry cap, so the
audit completes with a clear record of what could not be resolved instead of hanging on one
bad node.

Durability makes a large audit resumable: an interrupted run continues from its existing
frontier rather than restarting, and graceful shutdown persists the discovered graph so a
restart continues a partial audit cleanly.

The deliverable is the annotated graph plus a report: the packages violating policy, the
paths through the graph by which each was pulled in, and the packages that could not be
resolved. Because the graph assembles itself as the traversal proceeds — nodes appearing and
being colored by verdict, the queue visibly ballooning and then draining — the system's
progress and the pool's concurrency are directly observable, which makes it well suited to a
live demonstration.

## Build order

The system is built one layer at a time, testing each stage before starting the next, so that
there is always a working system rather than several half-built layers:

1. Bounded worker pool and the happy path — producers, the queue, and workers running jobs.
2. Retries with exponential backoff and jitter.
3. The dead-letter queue after a capped number of failures.
4. Durability — a Postgres- or disk-backed queue.
5. Graceful shutdown with drain and timeout.

The dependency-auditor workload can be introduced as the test job from the first stage
onward, since even a bounded pool running resolve-and-expand jobs demonstrates the
self-feeding expansion and the backpressure the pool provides.

## Defensibility checklist

Three questions should be airtight before the project is called defensible, each mapping to
one part of the design:

1. What happens if a worker crashes mid-job — answered by at-least-once delivery and
   idempotency or deduplication.
2. What stops a retry storm from taking down a struggling downstream — answered by
   exponential backoff, jitter, and the dead-letter queue cap.
3. How does the pool size create backpressure — answered by bounded concurrency, with the
   queue absorbing bursts.

## Framing

Described in full, the project reads as infrastructure rather than plumbing: a durable job
queue with a bounded worker pool, exponential-backoff retries, a dead-letter queue, and
graceful drain-on-shutdown, demonstrated by a self-feeding dependency and license auditor.
Every clause is a decision that was made deliberately and can be defended, which is the
direct answer to any doubt about whether the work is real engineering.
