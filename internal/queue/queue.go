package queue

import (
	"context"
	"log"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/store"
)

// Queue is an in-memory buffered channel that holds jobs until a worker is ready.
// Producers submit jobs and return immediately; workers block until a job is available.
type Queue struct {
	ch    chan job.Job
	store store.Store
}

// New creates a queue with the given buffer size.
// The buffer absorbs bursts from self-feeding expansion — a worker enqueuing
// child jobs won't block as long as the buffer has capacity.
func New(bufferSize int, stores ...store.Store) *Queue {
	if len(stores) == 0 || stores[0] == nil {
		return &Queue{ch: make(chan job.Job, bufferSize)}
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
		ch:    make(chan job.Job, capacity),
		store: stores[0],
	}
	for _, j := range pending {
		q.ch <- j
	}

	return q
}

// Submit pushes a job onto the channel. Non-blocking up to the buffer size.
func (q *Queue) Submit(j job.Job) {
	if q.store != nil {
		if err := q.store.CreateJob(context.Background(), j); err != nil {
			log.Printf("queue: create job %s: %v", j.ID, err)
			return
		}
		q.ch <- job.Job{}
		return
	}
	q.ch <- j
}

// Dequeue returns the next job. Blocks until one is available or the channel is closed.
// The bool return is false when the channel is closed and drained, signaling the worker to stop.
func (q *Queue) Dequeue() (job.Job, bool) {
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
		return job.Job{}, false
	}
	if !found {
		return job.Job{}, false
	}
	return acquired, true
}

// DispatchAcquired pushes a job that has already been atomically acquired from
// the store into the worker dispatch channel. It does not create a new row.
func (q *Queue) DispatchAcquired(j job.Job) {
	q.ch <- j
}

// Len returns the number of jobs currently waiting in the dispatch channel.
func (q *Queue) Len() int {
	return len(q.ch)
}

// Close closes the underlying channel, signaling workers that no more jobs will arrive.
func (q *Queue) Close() {
	close(q.ch)
}
