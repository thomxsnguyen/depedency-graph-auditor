package dlq_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/dlq"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

// TestDLQPublish verifies that Publish stores an entry with the correct job,
// error string, and a non-zero DeadAt timestamp.
func TestDLQPublish(t *testing.T) {
	d := &dlq.DLQ{}
	j := job.NewJob("test", nil)
	e := errors.New("permanent failure")

	d.Publish(j, e)

	entries := d.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries: got %d, want 1", len(entries))
	}
	got := entries[0]
	if got.Job.ID != j.ID {
		t.Errorf("Entry.Job.ID: got %q, want %q", got.Job.ID, j.ID)
	}
	if got.Err != e.Error() {
		t.Errorf("Entry.Err: got %q, want %q", got.Err, e.Error())
	}
	if got.DeadAt.IsZero() {
		t.Error("Entry.DeadAt: got zero time, want non-zero")
	}
}

// TestDLQEntriesCopy verifies that the slice returned by Entries is a copy —
// mutating it must not affect subsequent calls to Entries.
func TestDLQEntriesCopy(t *testing.T) {
	d := &dlq.DLQ{}
	d.Publish(job.NewJob("test", nil), errors.New("err"))

	first := d.Entries()
	first[0].Err = "mutated"

	second := d.Entries()
	if second[0].Err == "mutated" {
		t.Error("Entries() returned internal slice; mutation affected the DLQ")
	}
}

// TestDLQConcurrent verifies that N goroutines publishing simultaneously
// produce exactly N entries with no data races.
func TestDLQConcurrent(t *testing.T) {
	const n = 50
	d := &dlq.DLQ{}

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			d.Publish(job.NewJob("test", nil), errors.New("concurrent error"))
		}()
	}
	wg.Wait()

	entries := d.Entries()
	if len(entries) != n {
		t.Errorf("concurrent Publish: got %d entries, want %d", len(entries), n)
	}
}
