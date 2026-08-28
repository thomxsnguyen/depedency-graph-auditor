package worker

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/dlq"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/store"
)

const defaultPollInterval = 100 * time.Millisecond

var ErrShuttingDown = errors.New("worker pool is shutting down")

// Pool is a fixed number of goroutines that pull jobs from the queue and execute them.
// Workers are started once and run until the queue is closed.
type Pool struct {
	size      int
	queue     *queue.Queue
	handler   job.Handler
	workerWG  sync.WaitGroup
	pollerWG  sync.WaitGroup
	submitWG  sync.WaitGroup
	inFlight  atomic.Int64
	done      chan struct{}
	doneOnce  sync.Once                       // ensures close(done) is called exactly once
	backoff   func(attempt int) time.Duration // injectable; defaults to Backoff
	dlq       *dlq.DLQ                        // nil = log-only (Phase 2 behaviour)
	store     store.Store
	pollEvery time.Duration
	counted   sync.Map // job ID -> struct{} for jobs already counted in inFlight

	lifecycleMu  sync.Mutex
	started      bool
	stopping     bool
	accepting    atomic.Bool
	pollStop     chan struct{}
	pollStopOnce sync.Once
	shutdownOnce sync.Once
	shutdownDone chan struct{}
}

// Option is a functional option for configuring a Pool.
type Option func(*Pool)

// WithBackoff replaces the default Backoff function. Useful in tests to inject
// a zero-delay stub so the suite doesn't block waiting for real sleep durations.
func WithBackoff(fn func(attempt int) time.Duration) Option {
	return func(p *Pool) { p.backoff = fn }
}

// WithDLQ wires a DLQ into the pool. Exhausted jobs are published to it with
// StatusDeadLettered set. When not provided, exhausted jobs are only logged.
func WithDLQ(d *dlq.DLQ) Option {
	return func(p *Pool) { p.dlq = d }
}

// WithStore enables durable job lifecycle updates and scheduled retry polling.
func WithStore(s store.Store) Option {
	return func(p *Pool) { p.store = s }
}

// WithPollInterval configures how often the durable retry poller checks for
// jobs whose ScheduledAt time has elapsed. It is primarily useful in tests.
func WithPollInterval(interval time.Duration) Option {
	return func(p *Pool) { p.pollEvery = interval }
}

// New creates a worker pool with the given size, queue, and handler.
func New(size int, q *queue.Queue, h job.Handler) *Pool {
	return NewWithOptions(size, q, h)
}

// NewWithOptions creates a worker pool and applies any provided options.
func NewWithOptions(size int, q *queue.Queue, h job.Handler, opts ...Option) *Pool {
	p := &Pool{
		size:         size,
		queue:        q,
		handler:      h,
		done:         make(chan struct{}),
		backoff:      Backoff,
		pollEvery:    defaultPollInterval,
		pollStop:     make(chan struct{}),
		shutdownDone: make(chan struct{}),
	}
	for _, o := range opts {
		o(p)
	}
	p.inFlight.Store(int64(q.Len()))
	p.accepting.Store(true)
	return p
}

// Start launches `size` worker goroutines.
func (p *Pool) Start(ctx context.Context) {
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()
	if p.started || p.stopping {
		return
	}
	p.started = true

	if p.store != nil {
		p.pollerWG.Add(1)
		go p.pollLoop(ctx)
	}
	for i := 0; i < p.size; i++ {
		p.workerWG.Add(1)
		go p.workerLoop(ctx, i)
	}
}

// Wait blocks until the poller and all workers have exited.
func (p *Pool) Wait() {
	p.pollerWG.Wait()
	p.workerWG.Wait()
}

// Done returns a channel that is closed when all in-flight work is complete.
// This signals that the graph traversal is finished — no jobs are queued or running.
func (p *Pool) Done() <-chan struct{} {
	return p.done
}

// Submit accepts external work while the pool is running and tracks it until
// processing finishes. Once shutdown begins, new submissions are rejected.
func (p *Pool) Submit(j job.Job) error {
	p.lifecycleMu.Lock()
	if p.stopping {
		p.lifecycleMu.Unlock()
		return ErrShuttingDown
	}
	p.submitWG.Add(1)
	p.lifecycleMu.Unlock()
	defer p.submitWG.Done()

	p.counted.Store(j.ID, struct{}{})
	p.inFlight.Add(1)
	if err := p.queue.Submit(j); err != nil {
		p.counted.Delete(j.ID)
		p.inFlight.Add(-1)
		if errors.Is(err, queue.ErrClosed) {
			return ErrShuttingDown
		}
		return err
	}
	return nil
}

// Shutdown stops intake and polling, closes the queue after the poller exits,
// and waits for workers to drain. The shutdown sequence continues if a caller's
// context expires, so another caller can still wait for the same terminal state.
func (p *Pool) Shutdown(ctx context.Context) error {
	p.shutdownOnce.Do(func() {
		p.lifecycleMu.Lock()
		p.stopping = true
		p.accepting.Store(false)
		p.pollStopOnce.Do(func() { close(p.pollStop) })
		p.lifecycleMu.Unlock()

		go func() {
			p.pollerWG.Wait()
			p.submitWG.Wait()
			p.queue.Close()
			p.workerWG.Wait()
			close(p.shutdownDone)
		}()
	})

	select {
	case <-p.shutdownDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// workerLoop is the main loop for each worker goroutine.
func (p *Pool) workerLoop(ctx context.Context, id int) {
	defer p.workerWG.Done()

	for {
		j, ok := p.queue.Dequeue()
		if !ok {
			return // channel closed, exit
		}

		// Startup-reclaimed jobs were included in the queue length before their
		// IDs were known to the pool. Record the ID now so a later retry is not
		// counted a second time by the poller.
		p.counted.Store(j.ID, struct{}{})
		j.Status = job.StatusRunning
		newJobs, err := p.handler.Handle(ctx, j)

		if err != nil {
			j.Attempts++
			if j.Attempts < j.MaxAttempts {
				delay := p.backoff(j.Attempts)
				log.Printf("worker %d: job %s failed (attempt %d/%d), retrying in %v: %v",
					id, j.ID, j.Attempts, j.MaxAttempts, delay, err)
				if p.store != nil {
					j.ScheduledAt = time.Now().Add(delay)
					if retryErr := p.store.RetryJob(ctx, j); retryErr != nil {
						log.Printf("worker %d: schedule retry for job %s: %v", id, j.ID, retryErr)
					}
				} else {
					time.Sleep(delay)
					if submitErr := p.queue.Submit(j); submitErr != nil {
						log.Printf("worker %d: requeue job %s: %v", id, j.ID, submitErr)
					}
				}
				continue
			}
			// Attempts exhausted — quarantine in DLQ if configured, then release.
			j.Status = job.StatusDeadLettered
			if p.dlq != nil {
				p.dlq.Publish(j, err)
			} else if p.store != nil {
				if deadErr := p.store.DeadLetterJob(ctx, j, err); deadErr != nil {
					log.Printf("worker %d: persist dead-lettered job %s: %v", id, j.ID, deadErr)
				}
			}
			log.Printf("worker %d: job %s dead-lettered after %d attempts: %v",
				id, j.ID, j.Attempts, err)
			p.counted.Delete(j.ID)
			p.inFlight.Add(-1)
			p.checkDone()
			continue
		}

		j.Status = job.StatusCompleted
		if p.store != nil {
			if err := p.store.CompleteJob(ctx, j.ID); err != nil {
				log.Printf("worker %d: complete job %s: %v", id, j.ID, err)
			}
		}

		// Enqueue child jobs before decrementing this job's counter.
		// Each child increments inFlight, so the counter stays positive
		// as long as there's still work to do.
		for _, newJob := range newJobs {
			p.submitChild(ctx, id, newJob)
		}

		p.counted.Delete(j.ID)
		p.inFlight.Add(-1)
		p.checkDone()
	}
}

// pollLoop releases pending jobs once their durable ScheduledAt timestamp is
// eligible. Jobs already submitted or retried by this process remain counted
// in inFlight; jobs discovered after a restart are counted when first acquired.
func (p *Pool) pollLoop(ctx context.Context) {
	defer p.pollerWG.Done()
	ticker := time.NewTicker(p.pollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.pollStop:
			return
		case <-p.done:
			return
		case <-ticker.C:
			p.dispatchReady(ctx)
		}
	}
}

func (p *Pool) dispatchReady(ctx context.Context) {
	for {
		select {
		case <-p.pollStop:
			return
		default:
		}

		j, found, err := p.store.AcquireJob(ctx)
		if err != nil {
			log.Printf("worker: poll scheduled jobs: %v", err)
			return
		}
		if !found {
			return
		}

		if _, alreadyCounted := p.counted.LoadOrStore(j.ID, struct{}{}); !alreadyCounted {
			p.inFlight.Add(1)
		}
		if err := p.queue.DispatchAcquired(j); err != nil {
			log.Printf("worker: dispatch acquired job %s: %v", j.ID, err)
			return
		}
	}
}

// submitChild keeps self-feeding work in the current process while running.
// During shutdown it persists the next frontier without dispatching it, so a
// later process can resume the graph without extending this process's drain.
func (p *Pool) submitChild(ctx context.Context, workerID int, j job.Job) {
	if p.accepting.Load() {
		var err error
		if p.store != nil {
			err = p.persistChildForDispatch(ctx, j)
		} else {
			err = p.Submit(j)
		}
		if err == nil {
			return
		} else if !errors.Is(err, ErrShuttingDown) {
			log.Printf("worker %d: submit child job %s: %v", workerID, j.ID, err)
			return
		}
	}

	if err := p.queue.Persist(ctx, j); err != nil {
		log.Printf("worker %d: persist child job %s during shutdown: %v", workerID, j.ID, err)
	}
}

// persistChildForDispatch records durable child work without making the worker
// block on the bounded dispatch queue. The poller is the sole dispatcher for
// these jobs, while inFlight keeps Done open until they have been processed.
func (p *Pool) persistChildForDispatch(ctx context.Context, j job.Job) error {
	p.lifecycleMu.Lock()
	if p.stopping {
		p.lifecycleMu.Unlock()
		return ErrShuttingDown
	}
	p.submitWG.Add(1)
	p.lifecycleMu.Unlock()
	defer p.submitWG.Done()

	p.counted.Store(j.ID, struct{}{})
	p.inFlight.Add(1)
	if err := p.queue.Persist(ctx, j); err != nil {
		p.counted.Delete(j.ID)
		p.inFlight.Add(-1)
		return err
	}
	return nil
}

// checkDone closes the done channel if no jobs are in-flight.
// sync.Once guarantees the channel is closed exactly once even if multiple
// workers call checkDone concurrently when inFlight hits zero.
func (p *Pool) checkDone() {
	if p.inFlight.Load() == 0 {
		p.doneOnce.Do(func() { close(p.done) })
	}
}
