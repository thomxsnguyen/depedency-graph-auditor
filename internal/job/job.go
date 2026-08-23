package job

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Status represents the current state of a job in the queue.
type Status string

const (
	StatusPending      Status = "pending"
	StatusRunning      Status = "running"
	StatusCompleted    Status = "completed"
	StatusDeadLettered Status = "dead_lettered"
)

// DefaultMaxAttempts is the number of times a job will be tried before it is
// considered exhausted and dropped (Phase 3 will move it to a DLQ instead).
const DefaultMaxAttempts = 5

// Job is the generic unit of work submitted to the queue.
// It carries a type identifier, an arbitrary payload, and its current status.
type Job struct {
	ID          string
	Type        string
	Payload     json.RawMessage
	Status      Status
	Attempts    int       // how many times this job has been tried (0 = never run)
	MaxAttempts int       // maximum tries before the job is abandoned
	ScheduledAt time.Time // zero = run immediately; set by retry path for durable backoff
}

// NewJob constructs a Job with a fresh ID, pending status, and the default
// retry limit. Callers may override MaxAttempts after construction if needed.
func NewJob(jobType string, payload json.RawMessage) Job {
	return Job{
		ID:          NewJobID(),
		Type:        jobType,
		Payload:     payload,
		Status:      StatusPending,
		MaxAttempts: DefaultMaxAttempts,
	}
}

// Handler processes a job and returns zero or more new jobs to enqueue.
// The handler is unaware of the queue — it simply declares what follow-up
// work exists. The worker loop is responsible for enqueuing the returned jobs.
type Handler interface {
	Handle(ctx context.Context, j Job) ([]Job, error)
}

// NewJobID returns a new UUID string for use as a job ID.
func NewJobID() string {
	return uuid.New().String()
}
