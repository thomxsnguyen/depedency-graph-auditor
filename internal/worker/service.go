package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

type ServiceOptions struct {
	Concurrency      int
	PollInterval     time.Duration
	LeaseDuration    time.Duration
	HeartbeatEvery   time.Duration
	RecoveryInterval time.Duration
	ShutdownTimeout  time.Duration
	Logger           *log.Logger
}

type Service struct {
	store    job.ServiceStore
	workerID string
	handlers map[string]job.ServiceHandler
	options  ServiceOptions
}

func NewService(store job.ServiceStore, workerID string, handlers map[string]job.ServiceHandler, options ServiceOptions) *Service {
	if options.Concurrency <= 0 {
		options.Concurrency = 10
	}
	if options.PollInterval <= 0 {
		options.PollInterval = 250 * time.Millisecond
	}
	if options.LeaseDuration <= 0 {
		options.LeaseDuration = 30 * time.Second
	}
	if options.HeartbeatEvery <= 0 {
		options.HeartbeatEvery = 10 * time.Second
	}
	if options.RecoveryInterval <= 0 {
		options.RecoveryInterval = 5 * time.Second
	}
	if options.ShutdownTimeout <= 0 {
		options.ShutdownTimeout = 25 * time.Second
	}
	if options.Logger == nil {
		options.Logger = log.Default()
	}
	return &Service{store: store, workerID: workerID, handlers: handlers, options: options}
}

func (s *Service) Run(ctx context.Context) error {
	if s.options.HeartbeatEvery*2 >= s.options.LeaseDuration {
		return fmt.Errorf("worker heartbeat interval must be less than half the lease duration")
	}
	if err := s.store.RegisterWorker(ctx, s.workerID); err != nil {
		return err
	}
	var workers sync.WaitGroup
	for index := 0; index < s.options.Concurrency; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			s.claimLoop(ctx)
		}()
	}
	maintenanceDone := make(chan struct{})
	go func() {
		defer close(maintenanceDone)
		s.maintenanceLoop(ctx)
	}()
	workers.Wait()
	<-maintenanceDone
	return nil
}

func (s *Service) claimLoop(ctx context.Context) {
	ticker := time.NewTicker(s.options.PollInterval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		value, found, err := s.store.Claim(ctx, s.workerID, s.options.LeaseDuration)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.options.Logger.Printf("claim job: %v", err)
		} else if found {
			s.process(ctx, value)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) maintenanceLoop(ctx context.Context) {
	heartbeat := time.NewTicker(s.options.HeartbeatEvery)
	recovery := time.NewTicker(s.options.RecoveryInterval)
	defer heartbeat.Stop()
	defer recovery.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if err := s.store.WorkerHeartbeat(ctx, s.workerID); err != nil && ctx.Err() == nil {
				s.options.Logger.Printf("worker heartbeat: %v", err)
			}
		case <-recovery.C:
			if _, err := s.store.ReclaimExpired(ctx); err != nil && ctx.Err() == nil {
				s.options.Logger.Printf("reclaim expired jobs: %v", err)
			}
		}
	}
}

type handlerResponse struct {
	result job.HandlerResult
	err    error
}

func (s *Service) process(runCtx context.Context, value job.Job) {
	handler, exists := s.handlers[value.Type]
	if !exists {
		_ = s.store.Fail(context.Background(), value, job.ErrorPermanent,
			"unsupported job type", time.Time{})
		return
	}
	handlerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	response := make(chan handlerResponse, 1)
	go func() {
		result, err := handler.Handle(handlerCtx, value)
		response <- handlerResponse{result: result, err: err}
	}()
	ticker := time.NewTicker(s.options.HeartbeatEvery)
	defer ticker.Stop()
	var shutdown <-chan time.Time
	var shutdownTimer *time.Timer
	defer func() {
		if shutdownTimer != nil {
			shutdownTimer.Stop()
		}
	}()
	for {
		select {
		case <-runCtx.Done():
			if shutdown == nil {
				shutdownTimer = time.NewTimer(s.options.ShutdownTimeout)
				shutdown = shutdownTimer.C
				runCtx = context.Background()
			}
		case <-shutdown:
			cancel()
			_ = s.store.Fail(context.Background(), value, job.ErrorTransient, "worker shutdown interrupted job", time.Now())
			return
		case outcome := <-response:
			if outcome.err == nil {
				if err := s.store.Complete(context.Background(), value, outcome.result); err != nil && !errors.Is(err, job.ErrLeaseLost) {
					s.options.Logger.Printf("complete job %s: %v", value.ID, err)
				}
				return
			}
			kind := job.KindOf(outcome.err)
			retryAt := time.Time{}
			if kind == job.ErrorTransient {
				retryAt = time.Now().Add(Backoff(value.Attempts))
			}
			if err := s.store.Fail(context.Background(), value, kind, outcome.err.Error(), retryAt); err != nil && !errors.Is(err, job.ErrLeaseLost) {
				s.options.Logger.Printf("fail job %s: %v", value.ID, err)
			}
			return
		case <-ticker.C:
			owned, err := s.store.Heartbeat(context.Background(), value, s.options.LeaseDuration)
			if err != nil {
				s.options.Logger.Printf("heartbeat job %s: %v", value.ID, err)
			}
			if !owned {
				cancel()
				_ = s.store.Fail(context.Background(), value, job.ErrorCancelled, "job cancellation requested", time.Time{})
				return
			}
		}
	}
}
