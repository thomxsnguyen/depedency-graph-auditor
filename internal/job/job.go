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
	StatusPending        Status = "pending"
	StatusRunning        Status = "running"
	StatusWaiting        Status = "waiting"
	StatusRetryScheduled Status = "retry_scheduled"
	StatusCompleted      Status = "completed"
	StatusFailed         Status = "failed"
	StatusDeadLettered   Status = "dead_lettered"
	StatusCancelled      Status = "cancelled"
)

// DefaultMaxAttempts is the number of times a job will be tried before it is
// considered exhausted and dropped (Phase 3 will move it to a DLQ instead).
const DefaultMaxAttempts = 5

// Job is the generic unit of work submitted to the queue.
// It carries a type identifier, an arbitrary payload, and its current status.
type Job struct {
	ID                string          `json:"id"`
	Type              string          `json:"type"`
	Payload           json.RawMessage `json:"payload,omitempty"`
	Status            Status          `json:"status"`
	Attempts          int             `json:"attempts"`
	MaxAttempts       int             `json:"maxAttempts"`
	ScheduledAt       time.Time       `json:"scheduledAt"`
	RootJobID         string          `json:"rootJobId,omitempty"`
	ParentJobID       string          `json:"parentJobId,omitempty"`
	Internal          bool            `json:"internal,omitempty"`
	IdempotencyKey    string          `json:"-"`
	RequestHash       string          `json:"-"`
	LockedBy          string          `json:"lockedBy,omitempty"`
	LeaseToken        string          `json:"-"`
	LockedUntil       *time.Time      `json:"lockedUntil,omitempty"`
	HeartbeatAt       *time.Time      `json:"heartbeatAt,omitempty"`
	CancelRequestedAt *time.Time      `json:"cancelRequestedAt,omitempty"`
	LastErrorKind     ErrorKind       `json:"lastErrorKind,omitempty"`
	LastError         string          `json:"lastError,omitempty"`
	CreatedAt         time.Time       `json:"createdAt"`
	StartedAt         *time.Time      `json:"startedAt,omitempty"`
	CompletedAt       *time.Time      `json:"completedAt,omitempty"`
	RetriedFromJobID  string          `json:"retriedFromJobId,omitempty"`
	ReplayedFromJobID string          `json:"replayedFromJobId,omitempty"`
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
