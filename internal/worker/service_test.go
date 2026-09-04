package worker

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

type serviceStoreStub struct {
	mu        sync.Mutex
	claimed   bool
	completed int
	failed    int
}

func (s *serviceStoreStub) Submit(context.Context, job.Submission) (job.Job, bool, error) {
	return job.Job{}, false, nil
}
func (s *serviceStoreStub) Get(context.Context, string) (job.Detail, error) { return job.Detail{}, nil }
func (s *serviceStoreStub) List(context.Context, job.ListFilter) (job.ListPage, error) {
	return job.ListPage{}, nil
}
func (s *serviceStoreStub) Counts(context.Context) (job.Counts, error)      { return job.Counts{}, nil }
func (s *serviceStoreStub) Metrics(context.Context) (job.Metrics, error)    { return job.Metrics{}, nil }
func (s *serviceStoreStub) Cancel(context.Context, string) (job.Job, error) { return job.Job{}, nil }
func (s *serviceStoreStub) Retry(context.Context, string, int) (job.Job, error) {
	return job.Job{}, nil
}
func (s *serviceStoreStub) ListDLQ(context.Context, int, string) (job.DLQPage, error) {
	return job.DLQPage{}, nil
}
func (s *serviceStoreStub) ReplayDLQ(context.Context, int64, int) (job.Job, error) {
	return job.Job{}, nil
}
func (s *serviceStoreStub) RegisterWorker(context.Context, string) error  { return nil }
func (s *serviceStoreStub) WorkerHeartbeat(context.Context, string) error { return nil }
func (s *serviceStoreStub) Claim(ctx context.Context, worker string, _ time.Duration) (job.Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimed {
		return job.Job{}, false, nil
	}
	s.claimed = true
	return job.Job{ID: "one", RootJobID: "one", Type: "demo", Payload: json.RawMessage(`{}`), Status: job.StatusRunning, Attempts: 1, MaxAttempts: 1, LockedBy: worker, LeaseToken: "token"}, true, nil
}
func (s *serviceStoreStub) Heartbeat(context.Context, job.Job, time.Duration) (bool, error) {
	return true, nil
}
func (s *serviceStoreStub) Complete(context.Context, job.Job, job.HandlerResult) error {
	s.mu.Lock()
	s.completed++
	s.mu.Unlock()
	return nil
}
func (s *serviceStoreStub) Fail(context.Context, job.Job, job.ErrorKind, string, time.Time) error {
	s.mu.Lock()
	s.failed++
	s.mu.Unlock()
	return nil
}
func (s *serviceStoreStub) ReclaimExpired(context.Context) (int, error) { return 0, nil }

type handlerStub struct{ err error }

func (h handlerStub) Handle(context.Context, job.Job) (job.HandlerResult, error) {
	return job.HandlerResult{Result: json.RawMessage(`{"ok":true}`)}, h.err
}

type blockingHandler struct{}

func (blockingHandler) Handle(ctx context.Context, _ job.Job) (job.HandlerResult, error) {
	<-ctx.Done()
	return job.HandlerResult{}, ctx.Err()
}

func TestServiceProcessesClaimedJob(t *testing.T) {
	store := &serviceStoreStub{}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	service := NewService(store, "worker-test", map[string]job.ServiceHandler{"demo": handlerStub{}}, ServiceOptions{
		Concurrency: 1, PollInterval: time.Millisecond, LeaseDuration: 20 * time.Millisecond, HeartbeatEvery: 5 * time.Millisecond, RecoveryInterval: 5 * time.Millisecond,
	})
	if err := service.Run(ctx); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.completed != 1 || store.failed != 0 {
		t.Fatalf("completed=%d failed=%d", store.completed, store.failed)
	}
}

func TestServiceStopsClaimingAndBoundsShutdown(t *testing.T) {
	store := &serviceStoreStub{}
	ctx, cancel := context.WithCancel(context.Background())
	service := NewService(store, "worker-test", map[string]job.ServiceHandler{"demo": blockingHandler{}}, ServiceOptions{
		Concurrency: 1, PollInterval: time.Millisecond, LeaseDuration: 40 * time.Millisecond,
		HeartbeatEvery: 5 * time.Millisecond, RecoveryInterval: 5 * time.Millisecond,
		ShutdownTimeout: 10 * time.Millisecond,
	})
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	time.Sleep(5 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("worker did not stop within its shutdown deadline")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.completed != 0 || store.failed != 1 {
		t.Fatalf("completed=%d failed=%d", store.completed, store.failed)
	}
}
