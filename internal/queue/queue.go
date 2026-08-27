package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/store"
)

var (
	// ErrClosed is returned when dispatch is attempted after queue shutdown.
	ErrClosed = errors.New("queue is closed")
	// ErrNoStore is returned when durable-only persistence is requested from an
	// in-memory queue.
	ErrNoStore = errors.New("queue has no durable store")
)

// Queue is an in-memory buffered channel that holds jobs until a worker is ready.
// Producers submit jobs and return immediately; workers block until a job is available.
type Queue struct {
	ch          chan job.Job
	store       store.Store
	lifecycleMu sync.RWMutex
	closing     chan struct{}
	closeOnce   sync.Once
	closed      bool
}

// New creates a queue with the given buffer size.
// The buffer absorbs bursts from self-feeding expansion — a worker enqueuing
// child jobs won't block as long as the buffer has capacity.
func New(bufferSize int, stores ...store.Store) *Queue {
	if len(stores) == 0 || stores[0] == nil {
		return &Queue{
			ch:      make(chan job.Job, bufferSize),
			closing: make(chan struct{}),
		}
	}

	if _, err := stores[0].ReclaimStuckJobs(context.Background()); err != nil {
		log.Printf("queue: reclaim stuck jobs: %v", err)
	}

	// Reload every pending job that is currently eligible. AcquireJob is the
	// store interface's atomic pending -> running operation, so reloaded jobs
	// are already acquired when workers later receive them from the channel.
	var pending []job.Job
	for {
		j, found, err := stores[0].AcquireJob(context.Background())
		if err != nil {
			log.Printf("queue: reload pending jobs: %v", err)
			break
		}
		if !found {
			break
		}
		pending = append(pending, j)
	}

	// Startup must not block when more jobs are reloaded than the configured
	// burst buffer can hold; workers are not running yet to drain the channel.
	capacity := bufferSize
	if len(pending) > capacity {
		capacity = len(pending)
	}
	q := &Queue{
		ch:      make(chan job.Job, capacity),
		store:   stores[0],
		closing: make(chan struct{}),
	}
	for _, j := range pending {
		q.ch <- j
	}

	return q
}

// Submit persists and dispatches a job while the queue is open.
func (q *Queue) Submit(j job.Job) error {
	q.lifecycleMu.RLock()
	defer q.lifecycleMu.RUnlock()
	if q.closed {
		return ErrClosed
	}

	queued := j
	if q.store != nil {
		if err := q.store.CreateJob(context.Background(), j); err != nil {
			return fmt.Errorf("queue: create job %s: %w", j.ID, err)
		}
		queued = job.Job{}
	}

	select {
	case <-q.closing:
		return ErrClosed
	case q.ch <- queued:
		return nil
	}
}

// Persist writes a pending job to durable storage without dispatching it.
// It remains available after Close so workers can preserve children discovered
// while a process is draining.
func (q *Queue) Persist(ctx context.Context, j job.Job) error {
	if q.store == nil {
		return ErrNoStore
	}
	if err := q.store.CreateJob(ctx, j); err != nil {
		return fmt.Errorf("queue: persist job %s: %w", j.ID, err)
	}
	return nil
}

// Dequeue returns the next job. Blocks until one is available or the channel is closed.
// The bool return is false when the channel is closed and drained, signaling the worker to stop.
func (q *Queue) Dequeue() (job.Job, bool) {
	for {
		queued, ok := <-q.ch
		if !ok || q.store == nil {
			return queued, ok
		}
		if queued.ID != "" && queued.Status == job.StatusRunning {
			return queued, true
		}

		acquired, found, err := q.store.AcquireJob(context.Background())
		if err != nil {
			log.Printf("queue: acquire job: %v", err)
			continue
		}
		if !found {
			// The poller may have acquired this signal's row first and queued
			// the acquired job behind it. Ignore the stale signal and keep
			// waiting; only a closed channel stops a worker.
			continue
		}
		return acquired, true
	}
}

// DispatchAcquired pushes a job that has already been atomically acquired from
// the store into the worker dispatch channel. It does not create a new row.
func (q *Queue) DispatchAcquired(j job.Job) error {
	q.lifecycleMu.RLock()
	defer q.lifecycleMu.RUnlock()
	if q.closed {
		return ErrClosed
	}

	select {
	case <-q.closing:
		return ErrClosed
	case q.ch <- j:
		return nil
	}
}

// Len returns the number of jobs currently waiting in the dispatch channel.
func (q *Queue) Len() int {
	return len(q.ch)
}

// Close idempotently closes dispatch after all active senders have stopped.
func (q *Queue) Close() {
	q.closeOnce.Do(func() {
		// Closing this signal first releases senders blocked on a full channel.
		close(q.closing)
		q.lifecycleMu.Lock()
		q.closed = true
		close(q.ch)
		q.lifecycleMu.Unlock()
	})
}
