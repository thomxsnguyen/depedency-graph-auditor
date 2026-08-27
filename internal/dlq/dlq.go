package dlq

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

// Store is the persistence subset required by the DLQ. It is declared here so
// the DLQ does not import the broader store package and create an import cycle.
type Store interface {
	DeadLetterJob(ctx context.Context, j job.Job, err error) error
	DLQEntries(ctx context.Context) ([]DLQEntry, error)
}

// DLQEntry records an exhausted job and the reason it was dead-lettered.
type DLQEntry struct {
	Job    job.Job
	Err    string    // final error message
	DeadAt time.Time // wall-clock time the job was quarantined
}

// DLQ stores exhausted jobs in Postgres when configured with a Store and uses
// a thread-safe in-memory fallback otherwise.
type DLQ struct {
	mu      sync.Mutex
	entries []DLQEntry
	store   Store
}

// New creates a DLQ backed by the provided durable store. Passing nil retains
// the in-memory behavior used by unit tests and earlier phases.
func New(store Store) *DLQ {
	return &DLQ{store: store}
}

// Publish appends an exhausted job to the DLQ. Safe for concurrent use.
func (d *DLQ) Publish(j job.Job, err error) {
	if d.store != nil {
		if storeErr := d.store.DeadLetterJob(context.Background(), j, err); storeErr != nil {
			log.Printf("dlq: persist job %s: %v", j.ID, storeErr)
		}
		return
	}

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
	if d.store != nil {
		entries, err := d.store.DLQEntries(context.Background())
		if err != nil {
			log.Printf("dlq: query entries: %v", err)
			return nil
		}
		return entries
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]DLQEntry, len(d.entries))
	copy(out, d.entries)
	return out
}
