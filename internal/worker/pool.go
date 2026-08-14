package worker

import (
	"context"
	"log"
	"sync"
	"sync/atomic"

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
}

// New creates a worker pool with the given size, queue, and handler.
func New(size int, q *queue.Queue, h job.Handler) *Pool {
	return &Pool{
		size:    size,
		queue:   q,
		handler: h,
		done:    make(chan struct{}),
	}
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
			log.Printf("worker %d: job %s failed: %v", id, j.ID, err)
			p.inFlight.Add(-1)
			p.checkDone()
			continue // Phase 1: log and move on, no retry
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
