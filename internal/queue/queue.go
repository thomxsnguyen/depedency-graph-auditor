package queue

import (
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

// Queue is an in-memory buffered channel that holds jobs until a worker is ready.
// Producers submit jobs and return immediately; workers block until a job is available.
type Queue struct {
	ch chan job.Job
}

// New creates a queue with the given buffer size.
// The buffer absorbs bursts from self-feeding expansion — a worker enqueuing
// child jobs won't block as long as the buffer has capacity.
func New(bufferSize int) *Queue {
	return &Queue{
		ch: make(chan job.Job, bufferSize),
	}
}

// Submit pushes a job onto the channel. Non-blocking up to the buffer size.
func (q *Queue) Submit(j job.Job) {
	q.ch <- j
}

// Dequeue returns the next job. Blocks until one is available or the channel is closed.
// The bool return is false when the channel is closed and drained, signaling the worker to stop.
func (q *Queue) Dequeue() (job.Job, bool) {
	j, ok := <-q.ch
	return j, ok
}

// Close closes the underlying channel, signaling workers that no more jobs will arrive.
func (q *Queue) Close() {
	close(q.ch)
}
