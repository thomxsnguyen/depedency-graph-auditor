package worker_test

import (
	"testing"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/worker"
)

// TestBackoffAttempt1Window verifies that attempt 1 always falls in [0, 1s).
func TestBackoffAttempt1Window(t *testing.T) {
	for i := 0; i < 200; i++ {
		d := worker.Backoff(1)
		if d < 0 {
			t.Fatalf("Backoff(1): got negative duration %v", d)
		}
		if d >= time.Second {
			t.Fatalf("Backoff(1): got %v, want < 1s", d)
		}
	}
}

// TestBackoffAttempt5PlusCapped verifies that attempt 5+ is capped at [0, 30s).
func TestBackoffAttempt5PlusCapped(t *testing.T) {
	for _, attempt := range []int{5, 6, 10, 100} {
		for i := 0; i < 50; i++ {
			d := worker.Backoff(attempt)
			if d < 0 {
				t.Fatalf("Backoff(%d): got negative duration %v", attempt, d)
			}
			if d >= 30*time.Second {
				t.Fatalf("Backoff(%d): got %v, want < 30s", attempt, d)
			}
		}
	}
}

// TestBackoffNeverNegative verifies that no attempt ever returns a negative duration.
func TestBackoffNeverNegative(t *testing.T) {
	for attempt := 1; attempt <= 10; attempt++ {
		for i := 0; i < 50; i++ {
			if d := worker.Backoff(attempt); d < 0 {
				t.Fatalf("Backoff(%d): got negative duration %v", attempt, d)
			}
		}
	}
}

// TestBackoffJitterProducesVariety verifies that repeated calls with the same
// attempt return more than one distinct value, confirming jitter is active.
// (Probabilistic — extremely unlikely to fail with 100 samples of a [0,4s) window.)
func TestBackoffJitterProducesVariety(t *testing.T) {
	seen := make(map[time.Duration]struct{})
	for i := 0; i < 100; i++ {
		seen[worker.Backoff(3)] = struct{}{}
	}
	if len(seen) < 2 {
		t.Error("Backoff(3): all 100 calls returned the same value; jitter appears broken")
	}
}
