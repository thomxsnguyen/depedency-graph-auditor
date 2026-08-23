package store

import (
	"context"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/dlq"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

// Store is the persistence contract for the job queue and DLQ.
type Store interface {
	CreateJob(ctx context.Context, j job.Job) error
	AcquireJob(ctx context.Context) (job.Job, bool, error)
	CompleteJob(ctx context.Context, id string) error
	RetryJob(ctx context.Context, j job.Job) error
	DeadLetterJob(ctx context.Context, j job.Job, err error) error

	ReclaimStuckJobs(ctx context.Context) (int, error)

	DLQEntries(ctx context.Context) ([]dlq.DLQEntry, error)
}