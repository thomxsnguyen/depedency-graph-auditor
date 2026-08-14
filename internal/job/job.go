package job

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// Status represents the current state of a job in the queue.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
)

// Job is the generic unit of work submitted to the queue.
// It carries a type identifier, an arbitrary payload, and its current status.
type Job struct {
	ID      string
	Type    string
	Payload json.RawMessage
	Status  Status
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
