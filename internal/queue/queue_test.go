package queue_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/dlq"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
)

type fakeStore struct {
	mu           sync.Mutex
	jobs         []job.Job
	createErr    error
	reclaimErr   error
	reclaimCalls int
}

func (s *fakeStore) CreateJob(_ context.Context, j job.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return s.createErr
	}
	s.jobs = append(s.jobs, j)
	return nil
}

func (s *fakeStore) AcquireJob(_ context.Context) (job.Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.jobs {
		if s.jobs[i].Status == job.StatusPending {
			s.jobs[i].Status = job.StatusRunning
			return s.jobs[i], true, nil
		}
	}
	return job.Job{}, false, nil
}

func (s *fakeStore) CompleteJob(context.Context, string) error           { return nil }
func (s *fakeStore) RetryJob(context.Context, job.Job) error             { return nil }
func (s *fakeStore) DeadLetterJob(context.Context, job.Job, error) error { return nil }

func (s *fakeStore) ReclaimStuckJobs(_ context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reclaimCalls++
	if s.reclaimErr != nil {
		return 0, s.reclaimErr
	}

	reclaimed := 0
	for i := range s.jobs {
		if s.jobs[i].Status == job.StatusRunning {
			s.jobs[i].Status = job.StatusPending
			reclaimed++
		}
	}
	return reclaimed, nil
}

func (s *fakeStore) DLQEntries(context.Context) ([]dlq.DLQEntry, error) {
	return nil, nil
}

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

func TestNewWithStoreReclaimsAndReloadsPendingJobs(t *testing.T) {
	store := &fakeStore{jobs: []job.Job{
		{ID: "pending", Type: "test", Status: job.StatusPending},
		{ID: "crashed", Type: "test", Status: job.StatusRunning},
	}}

	// A buffer smaller than the startup backlog must not block construction.
	q := queue.New(1, store)

	for _, wantID := range []string{"pending", "crashed"} {
		got, ok := q.Dequeue()
		if !ok {
			t.Fatalf("Dequeue returned ok=false for reloaded job %q", wantID)
		}
		if got.ID != wantID {
			t.Fatalf("Dequeue: got ID %q, want %q", got.ID, wantID)
		}
		if got.Status != job.StatusRunning {
			t.Fatalf("Dequeue: got status %q, want %q", got.Status, job.StatusRunning)
		}
	}
	if store.reclaimCalls != 1 {
		t.Fatalf("ReclaimStuckJobs calls: got %d, want 1", store.reclaimCalls)
	}
}

func TestSubmitWithStorePersistsBeforeAcquire(t *testing.T) {
	store := &fakeStore{}
	q := queue.New(1, store)
	want := makeJob("persisted")

	q.Submit(want)
	got, ok := q.Dequeue()

	if !ok {
		t.Fatal("Dequeue returned ok=false for a persisted job")
	}
	if got.ID != want.ID {
		t.Fatalf("Dequeue: got ID %q, want %q", got.ID, want.ID)
	}
	if got.Status != job.StatusRunning {
		t.Fatalf("Dequeue: got status %q, want %q", got.Status, job.StatusRunning)
	}
}

func TestSubmitWithStoreDoesNotQueueCreateFailure(t *testing.T) {
	store := &fakeStore{createErr: errors.New("database unavailable")}
	q := queue.New(1, store)

	q.Submit(makeJob("not-persisted"))
	q.Close()
	got, ok := q.Dequeue()

	if ok {
		t.Fatalf("Dequeue returned queued job after CreateJob failed: %+v", got)
	}
}

func TestDequeueSkipsSignalAlreadyAcquiredByPoller(t *testing.T) {
	store := &fakeStore{}
	q := queue.New(2, store)
	want := makeJob("poller-won")
	q.Submit(want)

	acquired, found, err := store.AcquireJob(context.Background())
	if err != nil || !found {
		t.Fatalf("poller AcquireJob: found=%v err=%v", found, err)
	}
	q.DispatchAcquired(acquired)

	got, ok := q.Dequeue()
	if !ok {
		t.Fatal("Dequeue returned ok=false after a stale wake-up signal")
	}
	if got.ID != want.ID || got.Status != job.StatusRunning {
		t.Fatalf("Dequeue: got %+v, want acquired job %q", got, want.ID)
	}
}
