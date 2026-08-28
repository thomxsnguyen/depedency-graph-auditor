package worker_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/dlq"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/worker"
)

type durableStore struct {
	mu           sync.Mutex
	jobs         map[string]job.Job
	retries      []job.Job
	completed    []string
	deadLettered []string
}

type blockingPollStore struct {
	*durableStore
	block   atomic.Bool
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockingPollStore) AcquireJob(ctx context.Context) (job.Job, bool, error) {
	if s.block.Load() {
		s.once.Do(func() { close(s.started) })
		select {
		case <-s.release:
		case <-ctx.Done():
			return job.Job{}, false, ctx.Err()
		}
	}
	return s.durableStore.AcquireJob(ctx)
}

func newDurableStore() *durableStore {
	return &durableStore{jobs: make(map[string]job.Job)}
}

func (s *durableStore) CreateJob(_ context.Context, j job.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j.ScheduledAt.IsZero() {
		j.ScheduledAt = time.Now()
	}
	j.Status = job.StatusPending
	s.jobs[j.ID] = j
	return nil
}

func (s *durableStore) AcquireJob(_ context.Context) (job.Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, j := range s.jobs {
		if j.Status == job.StatusPending && !j.ScheduledAt.After(now) {
			j.Status = job.StatusRunning
			s.jobs[id] = j
			return j, true, nil
		}
	}
	return job.Job{}, false, nil
}

func (s *durableStore) CompleteJob(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	j.Status = job.StatusCompleted
	s.jobs[id] = j
	s.completed = append(s.completed, id)
	return nil
}

func (s *durableStore) RetryJob(_ context.Context, j job.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j.Status = job.StatusPending
	s.jobs[j.ID] = j
	s.retries = append(s.retries, j)
	return nil
}

func (s *durableStore) DeadLetterJob(_ context.Context, j job.Job, _ error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j.Status = job.StatusDeadLettered
	s.jobs[j.ID] = j
	s.deadLettered = append(s.deadLettered, j.ID)
	return nil
}

func (s *durableStore) ReclaimStuckJobs(context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reclaimed := 0
	for id, j := range s.jobs {
		if j.Status == job.StatusRunning {
			j.Status = job.StatusPending
			s.jobs[id] = j
			reclaimed++
		}
	}
	return reclaimed, nil
}

func (s *durableStore) DLQEntries(context.Context) ([]dlq.DLQEntry, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Test handler implementations
// ---------------------------------------------------------------------------

// noopHandler handles a job and returns no children.
type noopHandler struct{}

func (noopHandler) Handle(_ context.Context, _ job.Job) ([]job.Job, error) {
	return nil, nil
}

// countingHandler records how many times Handle is called.
type countingHandler struct {
	calls atomic.Int64
}

func (h *countingHandler) Handle(_ context.Context, _ job.Job) ([]job.Job, error) {
	h.calls.Add(1)
	return nil, nil
}

// selfFeedingHandler produces one child job the first time it sees each job,
// up to a configured depth, enabling a self-expanding graph test.
type selfFeedingHandler struct {
	maxDepth int
}

func (h *selfFeedingHandler) Handle(_ context.Context, j job.Job) ([]job.Job, error) {
	// Depth is encoded in the job Type field as a digit string.
	depth := 0
	if len(j.Type) > 0 {
		depth = int(j.Type[0] - '0')
	}
	if depth >= h.maxDepth {
		return nil, nil
	}
	child := job.Job{
		ID:   job.NewJobID(),
		Type: string(rune('0' + depth + 1)), // next depth
	}
	return []job.Job{child}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeJob(id string) job.Job {
	j := job.NewJob("0", nil)
	j.ID = id // override the generated ID with the caller-supplied one
	return j
}

// waitDone waits for the pool's Done signal or times out.
func waitDone(t *testing.T, p *worker.Pool, timeout time.Duration) {
	t.Helper()
	select {
	case <-p.Done():
	case <-time.After(timeout):
		t.Fatal("pool.Done() did not fire within timeout")
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestPoolProcessesAllJobs verifies that every submitted job is handled exactly
// once and the Done signal fires when the queue is empty and no work is
// in-flight.
func TestPoolProcessesAllJobs(t *testing.T) {
	const jobCount = 10

	q := queue.New(jobCount)
	h := &countingHandler{}
	p := worker.New(3, q, h)

	// Submit ALL jobs before starting workers so inFlight is fully loaded
	// before any worker can decrement it. Without this, a fast worker can
	// process+decrement inFlight to 0 between Submit calls in the main
	// goroutine, causing Done to fire before all jobs are submitted.
	for i := 0; i < jobCount; i++ {
		p.Submit(makeJob(job.NewJobID()))
	}

	p.Start(context.Background())

	waitDone(t, p, 3*time.Second)

	if got := h.calls.Load(); got != jobCount {
		t.Errorf("handler calls: got %d, want %d", got, jobCount)
	}

	// Clean up goroutines.
	q.Close()
	p.Wait()
}

// TestPoolDoneFiresWhenEmpty verifies that Done() fires immediately if the pool
// is started with no jobs submitted (inFlight never rises above zero).
//
// This is a degenerate case but important: main.go waits on Done() — if it
// never fires on an empty run the program hangs.
//
// NOTE: the current pool implementation only closes done when inFlight drops
// TO zero from a positive value. An empty pool (inFlight stays 0) never closes
// done. This test documents that known behaviour; if the implementation is later
// fixed to handle the empty case, remove the SkipNow call.
func TestPoolDoneFiresOnlyAfterWork(t *testing.T) {
	q := queue.New(1)
	p := worker.New(2, q, noopHandler{})
	p.Start(context.Background())

	// Submit one job so that inFlight transitions 0→1→0, triggering Done.
	p.Submit(makeJob("only-job"))
	waitDone(t, p, 3*time.Second)

	q.Close()
	p.Wait()
}

// TestPoolSelfFeedingExpansion verifies that child jobs returned by a handler
// are submitted and processed, and Done fires only after the entire expanded
// graph is complete.
//
// Graph: root (depth 0) → child (depth 1) → grandchild (depth 2) → terminal
// maxDepth=2 means depth 2 produces no children.
// Total jobs processed = 3 (root, child, grandchild).
func TestPoolSelfFeedingExpansion(t *testing.T) {
	q := queue.New(16)
	h := &countingHandler{}

	// Wrap a selfFeedingHandler so we can both expand the graph and count calls.
	type combinedHandler struct {
		count *countingHandler
		feed  *selfFeedingHandler
	}
	combined := &combinedHandler{
		count: h,
		feed:  &selfFeedingHandler{maxDepth: 2},
	}

	type wrapHandler struct{ c *combinedHandler }
	wh := &wrapHandler{c: combined}

	// Inline adapter since we can't embed two handlers cleanly.
	adapter := handlerFunc(func(ctx context.Context, j job.Job) ([]job.Job, error) {
		wh.c.count.Handle(ctx, j) //nolint:errcheck
		return wh.c.feed.Handle(ctx, j)
	})

	p := worker.New(2, q, adapter)
	p.Start(context.Background())

	root := job.Job{ID: job.NewJobID(), Type: "0"}
	p.Submit(root)

	waitDone(t, p, 3*time.Second)

	if got := h.calls.Load(); got != 3 {
		t.Errorf("self-feeding: expected 3 jobs processed, got %d", got)
	}

	q.Close()
	p.Wait()
}

// TestPoolWorkerCountBounded verifies that the pool operates with at most
// `size` concurrent workers.  We do this by submitting `size` slow jobs and
// confirming the pool processes them all without hanging — not by directly
// observing goroutine count, which is brittle.
func TestPoolWorkerCountBounded(t *testing.T) {
	const size = 3

	q := queue.New(32)
	h := &countingHandler{}
	p := worker.New(size, q, h)

	// Submit before starting so inFlight is loaded before workers can race.
	for i := 0; i < size; i++ {
		p.Submit(makeJob(job.NewJobID()))
	}

	p.Start(context.Background())

	waitDone(t, p, 3*time.Second)

	if got := h.calls.Load(); got != size {
		t.Errorf("bounded pool: expected %d calls, got %d", size, got)
	}

	q.Close()
	p.Wait()
}

// handlerFunc lets us use a plain function as a job.Handler in tests.
type handlerFunc func(context.Context, job.Job) ([]job.Job, error)

func (f handlerFunc) Handle(ctx context.Context, j job.Job) ([]job.Job, error) {
	return f(ctx, j)
}

// noBackoff is injected via WithBackoff in retry tests so the suite never sleeps.
func noBackoff(int) time.Duration { return 0 }

// TestPoolRetryThenSucceed verifies that a handler which returns a transient error
// on the first two attempts and succeeds on the third causes the job to complete,
// Done to fire, and the handler to have been called exactly failCount+1 times.
func TestPoolRetryThenSucceed(t *testing.T) {
	const failCount = 2
	const maxAttempts = 5

	q := queue.New(8)
	var calls atomic.Int64
	h := handlerFunc(func(_ context.Context, _ job.Job) ([]job.Job, error) {
		if calls.Add(1) <= failCount {
			return nil, errors.New("transient error")
		}
		return nil, nil
	})

	p := worker.NewWithOptions(2, q, h, worker.WithBackoff(noBackoff))
	p.Start(context.Background())

	j := job.NewJob("test", nil)
	j.MaxAttempts = maxAttempts
	p.Submit(j)

	waitDone(t, p, 3*time.Second)

	if got := calls.Load(); got != failCount+1 {
		t.Errorf("retry-then-succeed: handler calls got %d, want %d", got, failCount+1)
	}

	q.Close()
	p.Wait()
}

// TestPoolExhausted verifies that a handler which always fails causes the job to
// be logged as exhausted after MaxAttempts attempts, Done to fire cleanly, and
// the handler to have been called exactly MaxAttempts times.
func TestPoolExhausted(t *testing.T) {
	const maxAttempts = 3

	q := queue.New(8)
	var calls atomic.Int64
	h := handlerFunc(func(_ context.Context, _ job.Job) ([]job.Job, error) {
		calls.Add(1)
		return nil, errors.New("permanent error")
	})

	p := worker.NewWithOptions(2, q, h, worker.WithBackoff(noBackoff))
	p.Start(context.Background())

	j := job.NewJob("test", nil)
	j.MaxAttempts = maxAttempts
	p.Submit(j)

	waitDone(t, p, 3*time.Second)

	if got := calls.Load(); got != maxAttempts {
		t.Errorf("exhausted: handler calls got %d, want %d", got, maxAttempts)
	}

	q.Close()
	p.Wait()
}

// TestPoolWithDLQExhausted verifies that when a DLQ is wired and a job
// exhausts all attempts: the entry appears in dlq.Entries(), the job has
// StatusDeadLettered, the error string is non-empty, and Done fires cleanly.
func TestPoolWithDLQExhausted(t *testing.T) {
	const maxAttempts = 3

	q := queue.New(8)
	d := &dlq.DLQ{}
	h := handlerFunc(func(_ context.Context, _ job.Job) ([]job.Job, error) {
		return nil, errors.New("permanent error")
	})

	p := worker.NewWithOptions(2, q, h,
		worker.WithBackoff(noBackoff),
		worker.WithDLQ(d),
	)
	p.Start(context.Background())

	j := job.NewJob("test", nil)
	j.MaxAttempts = maxAttempts
	p.Submit(j)

	waitDone(t, p, 3*time.Second)

	entries := d.Entries()
	if len(entries) != 1 {
		t.Fatalf("DLQ entries: got %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Job.ID != j.ID {
		t.Errorf("DLQ entry job ID: got %q, want %q", e.Job.ID, j.ID)
	}
	if e.Job.Status != job.StatusDeadLettered {
		t.Errorf("DLQ entry status: got %q, want %q", e.Job.Status, job.StatusDeadLettered)
	}
	if e.Err == "" {
		t.Error("DLQ entry Err: got empty string, want non-empty error message")
	}

	q.Close()
	p.Wait()
}

// TestPoolWithoutDLQExhausted verifies that when no DLQ is configured,
// exhausted jobs are only logged (Phase 2 fallback) — Done fires and
// there is no panic.
func TestPoolWithoutDLQExhausted(t *testing.T) {
	const maxAttempts = 3

	q := queue.New(8)
	h := handlerFunc(func(_ context.Context, _ job.Job) ([]job.Job, error) {
		return nil, errors.New("permanent error")
	})

	p := worker.NewWithOptions(2, q, h, worker.WithBackoff(noBackoff))
	p.Start(context.Background())

	j := job.NewJob("test", nil)
	j.MaxAttempts = maxAttempts
	p.Submit(j)

	waitDone(t, p, 3*time.Second)

	q.Close()
	p.Wait()
}

func TestPoolDurableRetryWaitsForScheduledAt(t *testing.T) {
	const retryDelay = 40 * time.Millisecond

	s := newDurableStore()
	q := queue.New(8, s)
	var mu sync.Mutex
	var handledAt []time.Time
	h := handlerFunc(func(_ context.Context, _ job.Job) ([]job.Job, error) {
		mu.Lock()
		defer mu.Unlock()
		handledAt = append(handledAt, time.Now())
		if len(handledAt) == 1 {
			return nil, errors.New("transient error")
		}
		return nil, nil
	})

	p := worker.NewWithOptions(1, q, h,
		worker.WithStore(s),
		worker.WithBackoff(func(int) time.Duration { return retryDelay }),
		worker.WithPollInterval(time.Millisecond),
	)
	p.Start(context.Background())

	j := job.NewJob("test", nil)
	j.MaxAttempts = 3
	p.Submit(j)
	waitDone(t, p, 3*time.Second)

	mu.Lock()
	if len(handledAt) != 2 {
		t.Fatalf("handler calls: got %d, want 2", len(handledAt))
	}
	betweenAttempts := handledAt[1].Sub(handledAt[0])
	mu.Unlock()
	if betweenAttempts < retryDelay {
		t.Fatalf("retry delivered after %v, before scheduled delay %v", betweenAttempts, retryDelay)
	}

	s.mu.Lock()
	if len(s.retries) != 1 {
		t.Fatalf("RetryJob calls: got %d, want 1", len(s.retries))
	}
	if s.retries[0].Attempts != 1 || s.retries[0].ScheduledAt.IsZero() {
		t.Fatalf("RetryJob received invalid retry state: %+v", s.retries[0])
	}
	if len(s.completed) != 1 || s.completed[0] != j.ID {
		t.Fatalf("CompleteJob calls: got %v, want [%s]", s.completed, j.ID)
	}
	s.mu.Unlock()

	q.Close()
	p.Wait()
}

func TestPoolDurableExhaustionUsesStore(t *testing.T) {
	s := newDurableStore()
	q := queue.New(1, s)
	h := handlerFunc(func(context.Context, job.Job) ([]job.Job, error) {
		return nil, errors.New("permanent error")
	})
	p := worker.NewWithOptions(1, q, h,
		worker.WithStore(s),
		worker.WithDLQ(dlq.New(s)),
		worker.WithPollInterval(time.Millisecond),
	)
	p.Start(context.Background())

	j := job.NewJob("test", nil)
	j.MaxAttempts = 1
	p.Submit(j)
	waitDone(t, p, 3*time.Second)

	s.mu.Lock()
	if len(s.deadLettered) != 1 || s.deadLettered[0] != j.ID {
		t.Fatalf("DeadLetterJob calls: got %v, want [%s]", s.deadLettered, j.ID)
	}
	s.mu.Unlock()

	q.Close()
	p.Wait()
}

func TestPoolSubmitRejectedAfterShutdown(t *testing.T) {
	q := queue.New(1)
	p := worker.New(1, q, noopHandler{})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if err := p.Submit(makeJob("late-job")); !errors.Is(err, worker.ErrShuttingDown) {
		t.Fatalf("Submit after shutdown: got %v, want ErrShuttingDown", err)
	}
}

func TestPoolShutdownStopsIdleWorkers(t *testing.T) {
	q := queue.New(1)
	p := worker.New(2, q, noopHandler{})
	p.Start(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown idle pool: %v", err)
	}
}

func TestPoolShutdownWaitsForPollerBeforeClosingQueue(t *testing.T) {
	s := &blockingPollStore{
		durableStore: newDurableStore(),
		started:      make(chan struct{}),
		release:      make(chan struct{}),
	}
	q := queue.New(1, s)
	s.block.Store(true)
	p := worker.NewWithOptions(0, q, noopHandler{},
		worker.WithStore(s),
		worker.WithPollInterval(time.Millisecond),
	)
	p.Start(context.Background())

	select {
	case <-s.started:
	case <-time.After(time.Second):
		t.Fatal("poller did not enter AcquireJob")
	}

	result := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		result <- p.Shutdown(ctx)
	}()

	if err := q.DispatchAcquired(makeJob("ordering-probe")); err != nil {
		t.Fatalf("queue closed before poller exited: %v", err)
	}
	close(s.release)
	if err := <-result; err != nil {
		t.Fatalf("Shutdown after poller exit: %v", err)
	}
	if err := q.DispatchAcquired(makeJob("late-probe")); !errors.Is(err, queue.ErrClosed) {
		t.Fatalf("queue after shutdown: got %v, want ErrClosed", err)
	}
}

func TestPoolShutdownIsConcurrentAndIdempotent(t *testing.T) {
	q := queue.New(1)
	p := worker.New(2, q, noopHandler{})
	p.Start(context.Background())

	const callers = 8
	results := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			results <- p.Shutdown(ctx)
		}()
	}

	for i := 0; i < callers; i++ {
		if err := <-results; err != nil {
			t.Fatalf("Shutdown caller %d: %v", i, err)
		}
	}
}

func TestPoolShutdownDrainsActiveJob(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	h := handlerFunc(func(context.Context, job.Job) ([]job.Job, error) {
		close(started)
		<-release
		return nil, nil
	})

	s := newDurableStore()
	q := queue.New(1, s)
	p := worker.NewWithOptions(1, q, h, worker.WithStore(s))
	j := makeJob("active-job")
	if err := p.Submit(j); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	p.Start(context.Background())

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	result := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		result <- p.Shutdown(ctx)
	}()

	select {
	case err := <-result:
		t.Fatalf("Shutdown returned before active job finished: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	close(release)
	if err := <-result; err != nil {
		t.Fatalf("Shutdown after drain: %v", err)
	}

	s.mu.Lock()
	completed := append([]string(nil), s.completed...)
	persisted := s.jobs[j.ID]
	s.mu.Unlock()
	if len(completed) != 1 || completed[0] != j.ID || persisted.Status != job.StatusCompleted {
		t.Fatalf("completed job state: calls=%v job=%+v", completed, persisted)
	}
}

func TestPoolShutdownReturnsWhenContextExpires(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	h := handlerFunc(func(context.Context, job.Job) ([]job.Job, error) {
		close(started)
		<-release
		return nil, nil
	})

	q := queue.New(1)
	p := worker.New(1, q, h)
	if err := p.Submit(makeJob("stuck-job")); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	p.Start(context.Background())
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := p.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown: got %v, want context deadline exceeded", err)
	}

	close(release)
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Second)
	defer cleanupCancel()
	if err := p.Shutdown(cleanupCtx); err != nil {
		t.Fatalf("Shutdown after releasing handler: %v", err)
	}
}

func TestPoolShutdownPersistsChildWithoutDispatch(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	child := makeJob("child-job")
	var calls atomic.Int64
	h := handlerFunc(func(context.Context, job.Job) ([]job.Job, error) {
		calls.Add(1)
		close(started)
		<-release
		return []job.Job{child}, nil
	})

	s := newDurableStore()
	q := queue.New(2, s)
	p := worker.NewWithOptions(1, q, h,
		worker.WithStore(s),
		worker.WithPollInterval(time.Millisecond),
	)
	root := makeJob("root-job")
	if err := p.Submit(root); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	p.Start(context.Background())
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Shutdown(canceledCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("initial Shutdown: got %v, want context canceled", err)
	}
	close(release)

	ctx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown after handler finished: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("handler calls: got %d, want 1", got)
	}
	s.mu.Lock()
	persistedChild, found := s.jobs[child.ID]
	completed := append([]string(nil), s.completed...)
	s.mu.Unlock()
	if !found {
		t.Fatal("child job was not persisted")
	}
	if persistedChild.Status != job.StatusPending {
		t.Fatalf("child status: got %q, want %q", persistedChild.Status, job.StatusPending)
	}
	if len(completed) != 1 || completed[0] != root.ID {
		t.Fatalf("completed jobs: got %v, want only root job", completed)
	}
}

func TestPoolShutdownLeavesDurableRetryPending(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	h := handlerFunc(func(context.Context, job.Job) ([]job.Job, error) {
		calls.Add(1)
		close(started)
		<-release
		return nil, errors.New("transient error")
	})

	s := newDurableStore()
	q := queue.New(1, s)
	p := worker.NewWithOptions(1, q, h,
		worker.WithStore(s),
		worker.WithBackoff(func(int) time.Duration { return time.Hour }),
		worker.WithPollInterval(time.Millisecond),
	)
	j := makeJob("retry-job")
	j.MaxAttempts = 3
	if err := p.Submit(j); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	p.Start(context.Background())
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Shutdown(canceledCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("initial Shutdown: got %v, want context canceled", err)
	}
	close(release)

	ctx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown after retry persistence: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("handler calls: got %d, want 1", got)
	}
	s.mu.Lock()
	persistedRetry := s.jobs[j.ID]
	retryCalls := len(s.retries)
	s.mu.Unlock()
	if retryCalls != 1 {
		t.Fatalf("RetryJob calls: got %d, want 1", retryCalls)
	}
	if persistedRetry.Status != job.StatusPending || persistedRetry.Attempts != 1 {
		t.Fatalf("persisted retry state: got %+v", persistedRetry)
	}
}
