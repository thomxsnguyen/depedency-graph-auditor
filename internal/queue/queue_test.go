package queue_test

import (
	"testing"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
)

// makeJob is a convenience helper to create a minimal job with the given ID.
func makeJob(id string) job.Job {
	return job.Job{ID: id, Type: "test", Status: job.StatusPending}
}

// TestSubmitDequeue verifies the basic contract: a submitted job comes back
// from Dequeue intact.
func TestSubmitDequeue(t *testing.T) {
	q := queue.New(1)

	want := makeJob("job-1")
	q.Submit(want)

	got, ok := q.Dequeue()
	if !ok {
		t.Fatal("Dequeue returned ok=false on an open, non-empty queue")
	}
	if got.ID != want.ID {
		t.Fatalf("Dequeue: got ID %q, want %q", got.ID, want.ID)
	}
}

// TestDequeueOrderFIFO verifies that jobs are delivered in submission order.
func TestDequeueOrderFIFO(t *testing.T) {
	ids := []string{"a", "b", "c"}
	q := queue.New(len(ids))

	for _, id := range ids {
		q.Submit(makeJob(id))
	}

	for _, want := range ids {
		got, ok := q.Dequeue()
		if !ok {
			t.Fatalf("Dequeue returned ok=false, expected job %q", want)
		}
		if got.ID != want {
			t.Fatalf("Dequeue order: got %q, want %q", got.ID, want)
		}
	}
}

// TestCloseUnblocksDequeue verifies that Close() causes a blocking Dequeue to
// return (false, zero-value Job) rather than hanging forever.
func TestCloseUnblocksDequeue(t *testing.T) {
	q := queue.New(1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, ok := q.Dequeue()
		if ok {
			t.Errorf("Dequeue on closed queue: expected ok=false, got true")
		}
	}()

	// Give the goroutine time to block on Dequeue before closing.
	time.Sleep(10 * time.Millisecond)
	q.Close()

	select {
	case <-done:
		// goroutine exited — pass
	case <-time.After(time.Second):
		t.Fatal("Dequeue did not unblock within 1s after Close()")
	}
}

// TestCloseAndDrainReturnsAllJobs verifies that jobs submitted before Close are
// all delivered; subsequent Dequeue returns ok=false.
func TestCloseAndDrainReturnsAllJobs(t *testing.T) {
	q := queue.New(3)

	q.Submit(makeJob("x"))
	q.Submit(makeJob("y"))
	q.Close()

	j1, ok1 := q.Dequeue()
	j2, ok2 := q.Dequeue()
	_, ok3 := q.Dequeue() // channel drained + closed → false

	if !ok1 || j1.ID != "x" {
		t.Fatalf("first dequeue: got %+v ok=%v", j1, ok1)
	}
	if !ok2 || j2.ID != "y" {
		t.Fatalf("second dequeue: got %+v ok=%v", j2, ok2)
	}
	if ok3 {
		t.Fatal("third dequeue after close+drain: expected ok=false")
	}
}
