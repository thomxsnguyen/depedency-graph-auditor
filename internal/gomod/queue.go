package gomod

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/dlq"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
	storecontract "github.com/thomxsnguyen/mini-distributed-job-api/internal/store"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/worker"
)

const (
	goMetadataJobType = "go_module_metadata"
	goQueueBufferSize = 100
	goWorkerPoolSize  = 10
	goShutdownTimeout = 30 * time.Second
)

// MetadataPayload is the durable job body for one selected module coordinate.
type MetadataPayload struct {
	ModulePath string `json:"module_path"`
	Version    string `json:"version"`
	Round      int    `json:"round"`
}

type metadataClient interface {
	Fetch(context.Context, string, string) (Metadata, error)
}

// QueueRoundFetcher executes each MVS metadata round through the existing
// bounded, retryable worker queue.
type QueueRoundFetcher struct {
	client metadataClient
	store  storecontract.Store
	dlq    *dlq.DLQ

	mu          sync.Mutex
	round       int
	shutdown    time.Duration
	poolOptions []worker.Option
}

// NewQueueRoundFetcher constructs the production queue adapter.
func NewQueueRoundFetcher(client *Client, jobStore storecontract.Store, deadLetters *dlq.DLQ) *QueueRoundFetcher {
	return &QueueRoundFetcher{client: client, store: jobStore, dlq: deadLetters}
}

// SetShutdownTimeout applies the CLI's existing bounded-shutdown setting.
func (f *QueueRoundFetcher) SetShutdownTimeout(timeout time.Duration) {
	f.shutdown = timeout
}

// FetchRound fetches every requested coordinate to a terminal state. It never
// returns partial metadata as a successful round.
func (f *QueueRoundFetcher) FetchRound(ctx context.Context, coordinates []Coordinate) (map[Coordinate]Metadata, error) {
	if f.client == nil {
		return nil, fmt.Errorf("Go metadata client is required")
	}
	if len(coordinates) == 0 {
		return map[Coordinate]Metadata{}, nil
	}
	f.mu.Lock()
	f.round++
	round := f.round
	f.mu.Unlock()

	collector := &metadataCollector{
		results:  make(map[Coordinate]Metadata, len(coordinates)),
		failures: make(map[Coordinate]error),
	}
	for start := 0; start < len(coordinates); start += goQueueBufferSize {
		end := start + goQueueBufferSize
		if end > len(coordinates) {
			end = len(coordinates)
		}
		if err := f.runChunk(ctx, round, coordinates[start:end], collector); err != nil {
			return nil, fmt.Errorf("Go metadata round %d: %w", round, err)
		}
		if coordinate, err, exists := collector.firstFailure(coordinates[start:end]); exists {
			return nil, fmt.Errorf("Go metadata round %d failed for %s@%s: %w", round, coordinate.ModulePath, coordinate.Version, err)
		}
	}

	results := collector.snapshot()
	for _, coordinate := range coordinates {
		if _, exists := results[coordinate]; !exists {
			return nil, fmt.Errorf("Go metadata round %d incomplete: no metadata for %s@%s", round, coordinate.ModulePath, coordinate.Version)
		}
	}
	return results, nil
}

func (f *QueueRoundFetcher) runChunk(ctx context.Context, round int, coordinates []Coordinate, collector *metadataCollector) error {
	var q *queue.Queue
	if f.store == nil {
		q = queue.New(goQueueBufferSize)
	} else {
		q = queue.New(goQueueBufferSize, f.store)
	}
	deadLetters := f.dlq
	if deadLetters == nil {
		deadLetters = dlq.New(f.store)
	}
	handler := &metadataHandler{client: f.client, round: round, collector: collector}
	options := []worker.Option{worker.WithDLQ(deadLetters)}
	if f.store != nil {
		options = append(options, worker.WithStore(f.store))
	}
	options = append(options, f.poolOptions...)
	pool := worker.NewWithOptions(goWorkerPoolSize, q, handler, options...)

	for _, coordinate := range coordinates {
		payload, err := json.Marshal(MetadataPayload{ModulePath: coordinate.ModulePath, Version: coordinate.Version, Round: round})
		if err != nil {
			q.Close()
			return fmt.Errorf("marshal %s@%s job: %w", coordinate.ModulePath, coordinate.Version, err)
		}
		if err := pool.Submit(job.NewJob(goMetadataJobType, payload)); err != nil {
			q.Close()
			return fmt.Errorf("submit %s@%s job: %w", coordinate.ModulePath, coordinate.Version, err)
		}
	}
	// A caller cancellation requests bounded shutdown, but active durable jobs
	// retain their work context so the pool can drain them before returning.
	pool.Start(context.WithoutCancel(ctx))

	var terminalErr error
	select {
	case <-pool.Done():
	case <-ctx.Done():
		terminalErr = ctx.Err()
	}
	shutdownTimeout := f.shutdown
	if shutdownTimeout <= 0 {
		shutdownTimeout = goShutdownTimeout
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := pool.Shutdown(shutdownContext); err != nil && terminalErr == nil {
		terminalErr = fmt.Errorf("shutdown: %w", err)
	}
	return terminalErr
}

type metadataHandler struct {
	client    metadataClient
	round     int
	collector *metadataCollector
}

func (h *metadataHandler) Handle(ctx context.Context, queuedJob job.Job) ([]job.Job, error) {
	if queuedJob.Type != goMetadataJobType {
		return nil, fmt.Errorf("Go metadata handler: unsupported job type %q", queuedJob.Type)
	}
	var payload MetadataPayload
	if err := json.Unmarshal(queuedJob.Payload, &payload); err != nil {
		return nil, fmt.Errorf("Go metadata handler: decode payload: %w", err)
	}
	if payload.Round != h.round {
		return nil, fmt.Errorf("Go metadata handler: job round %d does not match active round %d", payload.Round, h.round)
	}
	if err := ValidateCoordinate(payload.ModulePath, payload.Version); err != nil {
		return nil, fmt.Errorf("Go metadata handler: invalid coordinate: %w", err)
	}
	coordinate := Coordinate{ModulePath: payload.ModulePath, Version: payload.Version}
	if err, exists := h.collector.failure(coordinate); exists {
		return nil, err
	}
	metadata, err := h.client.Fetch(ctx, payload.ModulePath, payload.Version)
	if err != nil {
		var classified interface{ Retryable() bool }
		if errors.As(err, &classified) && !classified.Retryable() {
			h.collector.fail(coordinate, err)
		}
		return nil, err
	}
	h.collector.put(coordinate, metadata)
	return nil, nil
}

type metadataCollector struct {
	mu       sync.Mutex
	results  map[Coordinate]Metadata
	failures map[Coordinate]error
}

func (c *metadataCollector) put(coordinate Coordinate, metadata Metadata) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.results == nil {
		c.results = make(map[Coordinate]Metadata)
	}
	c.results[coordinate] = metadata
}

func (c *metadataCollector) snapshot() map[Coordinate]Metadata {
	c.mu.Lock()
	defer c.mu.Unlock()
	results := make(map[Coordinate]Metadata, len(c.results))
	for coordinate, metadata := range c.results {
		results[coordinate] = metadata
	}
	return results
}

func (c *metadataCollector) fail(coordinate Coordinate, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failures == nil {
		c.failures = make(map[Coordinate]error)
	}
	c.failures[coordinate] = err
}

func (c *metadataCollector) failure(coordinate Coordinate) (error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	err, exists := c.failures[coordinate]
	return err, exists
}

func (c *metadataCollector) firstFailure(coordinates []Coordinate) (Coordinate, error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ordered := append([]Coordinate(nil), coordinates...)
	sortCoordinates(ordered)
	for _, coordinate := range ordered {
		if err, exists := c.failures[coordinate]; exists {
			return coordinate, err, true
		}
	}
	return Coordinate{}, nil, false
}
