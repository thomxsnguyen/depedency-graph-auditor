package worker

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
)

// Pool is a fixed number of goroutines that pull jobs from the queue and execute them.
// Workers are started once and run until the queue is closed.
type Pool struct {
	size     int
	queue    *queue.Queue
	handler  job.Handler
	wg       sync.WaitGroup
	inFlight atomic.Int64
	done     chan struct{}
	backoff  func(attempt int) time.Duration // injectable; defaults to Backoff
}

// Option is a functional option for configuring a Pool.
type Option func(*Pool)

// WithBackoff replaces the default Backoff function. Useful in tests to inject
// a zero-delay stub so the suite doesn't block waiting for real sleep durations.
func WithBackoff(fn func(attempt int) time.Duration) Option {
	return func(p *Pool) { p.backoff = fn }
}

// New creates a worker pool with the given size, queue, and handler.
func New(size int, q *queue.Queue, h job.Handler) *Pool {
	return NewWithOptions(size, q, h)
}

// NewWithOptions creates a worker pool and applies any provided options.
func NewWithOptions(size int, q *queue.Queue, h job.Handler, opts ...Option) *Pool {
	p := &Pool{
		size:    size,
		queue:   q,
		handler: h,
		done:    make(chan struct{}),
		backoff: Backoff,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Start launches `size` worker goroutines.
func (p *Pool) Start(ctx context.Context) {
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

		j.Status = job.StatusRunning
		newJobs, err := p.handler.Handle(ctx, j)

		if err != nil {
			j.Attempts++
			if j.Attempts < j.MaxAttempts {
				delay := p.backoff(j.Attempts)
				log.Printf("worker %d: job %s failed (attempt %d/%d), retrying in %v: %v",
					id, j.ID, j.Attempts, j.MaxAttempts, delay, err)
				time.Sleep(delay)
				p.queue.Submit(j) // re-queue directly — inFlight counter is unchanged
				continue
			}
			// Attempts exhausted — log and release. Phase 3 upgrades this to a DLQ.
			log.Printf("worker %d: job %s exhausted after %d attempts: %v",
				id, j.ID, j.Attempts, err)
			p.inFlight.Add(-1)
			p.checkDone()
			continue
		}

		j.Status = job.StatusCompleted

		// Enqueue child jobs before decrementing this job's counter.
		// Each child increments inFlight, so the counter stays positive
		// as long as there's still work to do.
		for _, newJob := range newJobs {
			p.Submit(newJob)
		}

		p.inFlight.Add(-1)
		p.checkDone()
	}
}

// checkDone closes the done channel if no jobs are in-flight.
func (p *Pool) checkDone() {
	if p.inFlight.Load() == 0 {
		select {
		case <-p.done:
			// already closed
		default:
			close(p.done)
		}
	}
}
