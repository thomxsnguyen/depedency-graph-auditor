package gomod

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/worker"
)

type fakeMetadataClient struct {
	mu        sync.Mutex
	metadata  map[Coordinate]Metadata
	failures  map[Coordinate]int
	permanent map[Coordinate]error
	calls     map[Coordinate]int
}

type drainingMetadataClient struct {
	coordinate Coordinate
	started    chan struct{}
	release    chan struct{}
}

func (c *drainingMetadataClient) Fetch(ctx context.Context, modulePath, version string) (Metadata, error) {
	close(c.started)
	select {
	case <-c.release:
		return metadata(c.coordinate), nil
	case <-ctx.Done():
		return Metadata{}, fmt.Errorf("work context canceled before drain: %w", ctx.Err())
	}
}

func (c *fakeMetadataClient) Fetch(_ context.Context, modulePath, version string) (Metadata, error) {
	coordinate := Coordinate{ModulePath: modulePath, Version: version}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.calls == nil {
		c.calls = make(map[Coordinate]int)
	}
	c.calls[coordinate]++
	if err := c.permanent[coordinate]; err != nil {
		return Metadata{}, err
	}
	if c.failures[coordinate] > 0 {
		c.failures[coordinate]--
		return Metadata{}, errors.New("transient proxy failure")
	}
	metadata, exists := c.metadata[coordinate]
	if !exists {
		return Metadata{}, errors.New("missing fixture")
	}
	return metadata, nil
}

func (c *fakeMetadataClient) callCount(coordinate Coordinate) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[coordinate]
}

func TestQueueRoundFetcherReturnsCompleteMetadata(t *testing.T) {
	a := Coordinate{ModulePath: "example.com/a", Version: "v1.0.0"}
	b := Coordinate{ModulePath: "example.com/b", Version: "v1.0.0"}
	client := &fakeMetadataClient{metadata: map[Coordinate]Metadata{a: metadata(a), b: metadata(b)}}
	fetcher := queueFetcherForTest(client)

	results, err := fetcher.FetchRound(context.Background(), []Coordinate{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[a].ModulePath != a.ModulePath || results[b].ModulePath != b.ModulePath {
		t.Fatalf("results: %+v", results)
	}
	if client.callCount(a) != 1 || client.callCount(b) != 1 {
		t.Fatalf("calls: a=%d b=%d", client.callCount(a), client.callCount(b))
	}
}

func TestQueueRoundFetcherUsesWorkerRetries(t *testing.T) {
	coordinate := Coordinate{ModulePath: "example.com/retry", Version: "v1.0.0"}
	client := &fakeMetadataClient{
		metadata: map[Coordinate]Metadata{coordinate: metadata(coordinate)},
		failures: map[Coordinate]int{coordinate: 2},
	}
	fetcher := queueFetcherForTest(client)

	results, err := fetcher.FetchRound(context.Background(), []Coordinate{coordinate})
	if err != nil {
		t.Fatal(err)
	}
	if results[coordinate].ModulePath != coordinate.ModulePath || client.callCount(coordinate) != 3 {
		t.Fatalf("results=%+v calls=%d", results, client.callCount(coordinate))
	}
}

func TestQueueRoundFetcherRejectsIncompleteDeadLetteredRound(t *testing.T) {
	coordinate := Coordinate{ModulePath: "example.com/fail", Version: "v1.0.0"}
	client := &fakeMetadataClient{permanent: map[Coordinate]error{coordinate: &ProxyError{
		Kind: ErrorNotFound, ModulePath: coordinate.ModulePath, Version: coordinate.Version, StatusCode: 404,
	}}}
	fetcher := queueFetcherForTest(client)

	_, err := fetcher.FetchRound(context.Background(), []Coordinate{coordinate})
	if err == nil || !strings.Contains(err.Error(), "not_found") {
		t.Fatalf("error: got %v", err)
	}
	if client.callCount(coordinate) != 1 {
		t.Fatalf("calls: got %d, want 1 for permanent failure", client.callCount(coordinate))
	}
}

func TestQueueRoundFetcherRejectsExhaustedTransientRound(t *testing.T) {
	coordinate := Coordinate{ModulePath: "example.com/exhausted", Version: "v1.0.0"}
	client := &fakeMetadataClient{
		metadata: map[Coordinate]Metadata{coordinate: metadata(coordinate)},
		failures: map[Coordinate]int{coordinate: 10},
	}
	fetcher := queueFetcherForTest(client)

	_, err := fetcher.FetchRound(context.Background(), []Coordinate{coordinate})
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("error: got %v", err)
	}
	if client.callCount(coordinate) != 5 {
		t.Fatalf("calls: got %d, want 5", client.callCount(coordinate))
	}
}

func TestQueueRoundFetcherDrainsActiveWorkOnCallerCancellation(t *testing.T) {
	coordinate := Coordinate{ModulePath: "example.com/drain", Version: "v1.0.0"}
	client := &drainingMetadataClient{
		coordinate: coordinate,
		started:    make(chan struct{}),
		release:    make(chan struct{}),
	}
	fetcher := queueFetcherForTest(client)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := fetcher.FetchRound(ctx, []Coordinate{coordinate})
		result <- err
	}()

	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("metadata request did not start")
	}
	cancel()
	time.Sleep(10 * time.Millisecond)
	close(client.release)
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error: got %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("fetch round did not finish after draining active work")
	}
}

func TestQueueRoundFetcherChunksAtBoundedQueueCapacity(t *testing.T) {
	coordinates := make([]Coordinate, goQueueBufferSize+1)
	fixtures := make(map[Coordinate]Metadata, len(coordinates))
	for index := range coordinates {
		coordinate := Coordinate{ModulePath: fmt.Sprintf("example.com/module%03d", index), Version: "v1.0.0"}
		coordinates[index] = coordinate
		fixtures[coordinate] = metadata(coordinate)
	}
	client := &fakeMetadataClient{metadata: fixtures}
	fetcher := queueFetcherForTest(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := fetcher.FetchRound(ctx, coordinates)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(coordinates) {
		t.Fatalf("results: got %d, want %d", len(results), len(coordinates))
	}
}

func TestQueuedSelectionMapsOnlyFinalPromotedBuildList(t *testing.T) {
	a := Coordinate{ModulePath: "example.com/a", Version: "v1.0.0"}
	b := Coordinate{ModulePath: "example.com/b", Version: "v1.0.0"}
	sharedLow := Coordinate{ModulePath: "example.com/shared", Version: "v1.2.0"}
	sharedHigh := Coordinate{ModulePath: "example.com/shared", Version: "v1.5.0"}
	client := &fakeMetadataClient{metadata: map[Coordinate]Metadata{
		a:          metadata(a, Requirement{ModulePath: sharedLow.ModulePath, Version: sharedLow.Version}),
		sharedLow:  metadata(sharedLow, Requirement{ModulePath: b.ModulePath, Version: b.Version}),
		b:          metadata(b, Requirement{ModulePath: sharedHigh.ModulePath, Version: sharedHigh.Version}),
		sharedHigh: metadata(sharedHigh),
	}}

	selection, err := Select(context.Background(), "example.com/root", "1.16", []Requirement{
		{ModulePath: a.ModulePath, Version: a.Version},
	}, queueFetcherForTest(client))
	if err != nil {
		t.Fatal(err)
	}
	packages := auditor.NewPackageStore()
	edges := auditor.NewEdgeStore()
	MapSelection(selection, packages, edges, auditor.LicensePolicy{})

	if client.callCount(sharedLow) != 1 || client.callCount(sharedHigh) != 1 {
		t.Fatalf("promotion fetches: low=%d high=%d", client.callCount(sharedLow), client.callCount(sharedHigh))
	}
	if packages.Exists(sharedLow.ModulePath, sharedLow.Version) {
		t.Fatal("fetched lower version leaked into final graph")
	}
	if !packages.Exists(sharedHigh.ModulePath, sharedHigh.Version) {
		t.Fatal("promoted version missing from final graph")
	}
	for _, edge := range edges.All() {
		if edge.ToName == sharedLow.ModulePath && edge.ToVersion == sharedLow.Version {
			t.Fatalf("lower-version edge leaked into final graph: %+v", edge)
		}
	}
}

func TestMetadataHandlerValidatesJobPayload(t *testing.T) {
	coordinate := Coordinate{ModulePath: "example.com/module", Version: "v1.0.0"}
	client := &fakeMetadataClient{metadata: map[Coordinate]Metadata{coordinate: metadata(coordinate)}}
	handler := &metadataHandler{client: client, round: 2, collector: &metadataCollector{results: make(map[Coordinate]Metadata)}}
	payload, err := json.Marshal(MetadataPayload{ModulePath: coordinate.ModulePath, Version: coordinate.Version, Round: 1})
	if err != nil {
		t.Fatal(err)
	}
	tests := []job.Job{
		{ID: "wrong-type", Type: "audit_package", Payload: payload},
		{ID: "bad-json", Type: goMetadataJobType, Payload: []byte("{")},
		{ID: "wrong-round", Type: goMetadataJobType, Payload: payload},
	}
	for _, queuedJob := range tests {
		if _, err := handler.Handle(context.Background(), queuedJob); err == nil {
			t.Fatalf("job %s: expected validation error", queuedJob.ID)
		}
	}
}

func queueFetcherForTest(client metadataClient) *QueueRoundFetcher {
	return &QueueRoundFetcher{
		client: client,
		poolOptions: []worker.Option{
			worker.WithBackoff(func(int) time.Duration { return 0 }),
		},
	}
}
