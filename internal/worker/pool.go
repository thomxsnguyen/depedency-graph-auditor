package worker

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/dlq"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/store"
)

const defaultPollInterval = 100 * time.Millisecond

// Pool is a fixed number of goroutines that pull jobs from the queue and execute them.
// Workers are started once and run until the queue is closed.
type Pool struct {
	size      int
	queue     *queue.Queue
	handler   job.Handler
	wg        sync.WaitGroup
	inFlight  atomic.Int64
	done      chan struct{}
	doneOnce  sync.Once                       // ensures close(done) is called exactly once
	backoff   func(attempt int) time.Duration // injectable; defaults to Backoff
	dlq       *dlq.DLQ                        // nil = log-only (Phase 2 behaviour)
	store     store.Store
	pollEvery time.Duration
	counted   sync.Map // job ID -> struct{} for jobs already counted in inFlight
}

// Option is a functional option for configuring a Pool.
type Option func(*Pool)

// WithBackoff replaces the default Backoff function. Useful in tests to inject
// a zero-delay stub so the suite doesn't block waiting for real sleep durations.
func WithBackoff(fn func(attempt int) time.Duration) Option {
	return func(p *Pool) { p.backoff = fn }
}

// WithDLQ wires a DLQ into the pool. Exhausted jobs are published to it with
// StatusDeadLettered set. When not provided, exhausted jobs are only logged.
func WithDLQ(d *dlq.DLQ) Option {
	return func(p *Pool) { p.dlq = d }
}

// WithStore enables durable job lifecycle updates and scheduled retry polling.
func WithStore(s store.Store) Option {
	return func(p *Pool) { p.store = s }
}

// WithPollInterval configures how often the durable retry poller checks for
// jobs whose ScheduledAt time has elapsed. It is primarily useful in tests.
func WithPollInterval(interval time.Duration) Option {
	return func(p *Pool) { p.pollEvery = interval }
}

// New creates a worker pool with the given size, queue, and handler.
func New(size int, q *queue.Queue, h job.Handler) *Pool {
	return NewWithOptions(size, q, h)
}

// NewWithOptions creates a worker pool and applies any provided options.
func NewWithOptions(size int, q *queue.Queue, h job.Handler, opts ...Option) *Pool {
	p := &Pool{
		size:      size,
		queue:     q,
		handler:   h,
		done:      make(chan struct{}),
		backoff:   Backoff,
		pollEvery: defaultPollInterval,
	}
	for _, o := range opts {
		o(p)
	}
	p.inFlight.Store(int64(q.Len()))
	return p
}

// Start launches `size` worker goroutines.
func (p *Pool) Start(ctx context.Context) {
	if p.store != nil {
		p.wg.Add(1)
		go p.pollLoop(ctx)
	}
	for i := 0; i < p.size; i++ {
		p.wg.Add(1)
		go p.workerLoop(ctx, i)
	}
}

// Wait blocks until all workers have exited.
func (p *Pool) Wait() {
	p.wg.Wait()
}

// Done returns a channel that is closed when all in-flight work is complete.
// This signals that the graph traversal is finished — no jobs are queued or running.
func (p *Pool) Done() <-chan struct{} {
	return p.done
}

// Submit wraps the queue's Submit with in-flight tracking.
// The counter increments here and decrements when the worker finishes processing.
func (p *Pool) Submit(j job.Job) {
	p.counted.Store(j.ID, struct{}{})
	p.inFlight.Add(1)
	p.queue.Submit(j)
}

// workerLoop is the main loop for each worker goroutine.
func (p *Pool) workerLoop(ctx context.Context, id int) {
	defer p.wg.Done()

	for {
		j, ok := p.queue.Dequeue()
		if !ok {
			return // channel closed, exit
		}

		// Startup-reclaimed jobs were included in the queue length before their
		// IDs were known to the pool. Record the ID now so a later retry is not
		// counted a second time by the poller.
		p.counted.Store(j.ID, struct{}{})
		j.Status = job.StatusRunning
		newJobs, err := p.handler.Handle(ctx, j)

		if err != nil {
			j.Attempts++
			if j.Attempts < j.MaxAttempts {
				delay := p.backoff(j.Attempts)
				log.Printf("worker %d: job %s failed (attempt %d/%d), retrying in %v: %v",
					id, j.ID, j.Attempts, j.MaxAttempts, delay, err)
				if p.store != nil {
					j.ScheduledAt = time.Now().Add(delay)
					if retryErr := p.store.RetryJob(ctx, j); retryErr != nil {
						log.Printf("worker %d: schedule retry for job %s: %v", id, j.ID, retryErr)
					}
				} else {
					time.Sleep(delay)
					p.queue.Submit(j)
				}
				continue
			}
			// Attempts exhausted — quarantine in DLQ if configured, then release.
			j.Status = job.StatusDeadLettered
			if p.dlq != nil {
				p.dlq.Publish(j, err)
			} else if p.store != nil {
				if deadErr := p.store.DeadLetterJob(ctx, j, err); deadErr != nil {
					log.Printf("worker %d: persist dead-lettered job %s: %v", id, j.ID, deadErr)
				}
			}
			log.Printf("worker %d: job %s dead-lettered after %d attempts: %v",
				id, j.ID, j.Attempts, err)
			p.counted.Delete(j.ID)
			p.inFlight.Add(-1)
			p.checkDone()
			continue
		}

		j.Status = job.StatusCompleted
		if p.store != nil {
			if err := p.store.CompleteJob(ctx, j.ID); err != nil {
				log.Printf("worker %d: complete job %s: %v", id, j.ID, err)
			}
		}

		// Enqueue child jobs before decrementing this job's counter.
		// Each child increments inFlight, so the counter stays positive
		// as long as there's still work to do.
		for _, newJob := range newJobs {
			p.Submit(newJob)
		}

		p.counted.Delete(j.ID)
		p.inFlight.Add(-1)
		p.checkDone()
	}
}

// pollLoop releases pending jobs once their durable ScheduledAt timestamp is
// eligible. Jobs already submitted or retried by this process remain counted
// in inFlight; jobs discovered after a restart are counted when first acquired.
func (p *Pool) pollLoop(ctx context.Context) {
	defer p.wg.Done()
	ticker := time.NewTicker(p.pollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.done:
			return
		case <-ticker.C:
			p.dispatchReady(ctx)
		}
	}
}

func (p *Pool) dispatchReady(ctx context.Context) {
	for {
		j, found, err := p.store.AcquireJob(ctx)
		if err != nil {
			log.Printf("worker: poll scheduled jobs: %v", err)
			return
		}
		if !found {
			return
		}

		if _, alreadyCounted := p.counted.LoadOrStore(j.ID, struct{}{}); !alreadyCounted {
			p.inFlight.Add(1)
		}
		p.queue.DispatchAcquired(j)
	}
}

// checkDone closes the done channel if no jobs are in-flight.
// sync.Once guarantees the channel is closed exactly once even if multiple
// workers call checkDone concurrently when inFlight hits zero.
func (p *Pool) checkDone() {
	if p.inFlight.Load() == 0 {
		p.doneOnce.Do(func() { close(p.done) })
	}
}
