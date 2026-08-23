package job_test

import (
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

// TestNewJobDefaults verifies that NewJob sets the correct zero values and defaults.
func TestNewJobDefaults(t *testing.T) {
	j := job.NewJob("audit", nil)

	if j.ID == "" {
		t.Error("ID: got empty string, want non-empty UUID")
	}
	if j.Type != "audit" {
		t.Errorf("Type: got %q, want %q", j.Type, "audit")
	}
	if j.Status != job.StatusPending {
		t.Errorf("Status: got %q, want %q", j.Status, job.StatusPending)
	}
	if j.Attempts != 0 {
		t.Errorf("Attempts: got %d, want 0", j.Attempts)
	}
	if j.MaxAttempts != job.DefaultMaxAttempts {
		t.Errorf("MaxAttempts: got %d, want %d", j.MaxAttempts, job.DefaultMaxAttempts)
	}
}

// TestNewJobUniqueIDs verifies that each call to NewJob produces a distinct ID.
func TestNewJobUniqueIDs(t *testing.T) {
	j1 := job.NewJob("test", nil)
	j2 := job.NewJob("test", nil)
	if j1.ID == j2.ID {
		t.Errorf("NewJob: two consecutive calls returned the same ID %q", j1.ID)
	}
}
