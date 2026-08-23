package dlq

import (
	"sync"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

// DLQEntry records an exhausted job and the reason it was dead-lettered.
type DLQEntry struct {
	Job    job.Job
	Err    string    // final error message
	DeadAt time.Time // wall-clock time the job was quarantined
}

// DLQ is a thread-safe in-memory store for exhausted jobs.
// Phase 4 will back this with Postgres so entries survive crashes.
type DLQ struct {
	mu      sync.Mutex
	entries []DLQEntry
}

// Publish appends an exhausted job to the DLQ. Safe for concurrent use.
func (d *DLQ) Publish(j job.Job, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries = append(d.entries, DLQEntry{
		Job:    j,
		Err:    err.Error(),
		DeadAt: time.Now(),
	})
}

// Entries returns a snapshot copy of all dead-lettered entries.
// The caller receives its own slice; mutations do not affect the DLQ.
func (d *DLQ) Entries() []DLQEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]DLQEntry, len(d.entries))
	copy(out, d.entries)
	return out
}
