//go:build integration

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/dlq"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
	storepg "github.com/thomxsnguyen/mini-distributed-job-api/internal/store/postgres"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/worker"
)

const (
	phase5DefaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/jobqueue?sslmode=disable"
	phase5TimeoutExitCode    = 23
)

type phase5IntegrationDB struct {
	url    string
	schema string
	pool   *pgxpool.Pool
}

func setupPhase5IntegrationDB(t *testing.T) *phase5IntegrationDB {
	t.Helper()
	ctx := context.Background()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = phase5DefaultDatabaseURL
	}

	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open Postgres admin pool: %v", err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Fatalf("Postgres is required for integration tests: %v", err)
	}

	schema := fmt.Sprintf("phase5_test_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		admin.Close()
		t.Fatalf("create test schema: %v", err)
	}

	pool, err := openPhase5SchemaPool(ctx, databaseURL, schema)
	if err != nil {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+identifier+" CASCADE")
		admin.Close()
		t.Fatalf("open test schema pool: %v", err)
	}

	_, filename, _, _ := runtime.Caller(0)
	migrationPath := filepath.Join(filepath.Dir(filename), "../../db/migrations/001_jobs.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+identifier+" CASCADE")
		admin.Close()
	})
	return &phase5IntegrationDB{url: databaseURL, schema: schema, pool: pool}
}

func openPhase5SchemaPool(ctx context.Context, databaseURL, schema string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

type phase5HandlerFunc func(context.Context, job.Job) ([]job.Job, error)

func (f phase5HandlerFunc) Handle(ctx context.Context, j job.Job) ([]job.Job, error) {
	return f(ctx, j)
}

type phase5SmokeRegistry struct{}

func (phase5SmokeRegistry) FetchPackage(context.Context, string, string) (*auditor.PackageMetadata, error) {
	return &auditor.PackageMetadata{
		Name:         "phase5-smoke",
		Version:      "1.0.0",
		License:      "MIT",
		Dependencies: map[string]string{},
	}, nil
}

func phase5HelperCommand(db *phase5IntegrationDB, mode, rootID, childID string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestPhase5ProcessHelper$")
	cmd.Env = append(os.Environ(),
		"PHASE5_HELPER=1",
		"PHASE5_DATABASE_URL="+db.url,
		"PHASE5_SCHEMA="+db.schema,
		"PHASE5_MODE="+mode,
		"PHASE5_ROOT_ID="+rootID,
		"PHASE5_CHILD_ID="+childID,
	)
	if mode == "timeout" {
		cmd.Env = append(cmd.Env, "SHUTDOWN_TIMEOUT=100ms")
	} else {
		cmd.Env = append(cmd.Env, "SHUTDOWN_TIMEOUT=2s")
	}
	if mode == "retry" {
		cmd.Env = append(cmd.Env, "PHASE5_RETRY_DELAY=1s")
	}
	return cmd
}

type phase5ProcessResult struct {
	err     error
	stderr  string
	elapsed time.Duration
}

func runSignaledPhase5Helper(
	t *testing.T,
	db *phase5IntegrationDB,
	mode, rootID, childID string,
	sig os.Signal,
) phase5ProcessResult {
	t.Helper()
	cmd := phase5HelperCommand(db, mode, rootID, childID)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("helper stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "READY" {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("helper did not become ready: stdout=%q stderr=%s", scanner.Text(), stderr.String())
	}

	started := time.Now()
	if err := cmd.Process.Signal(sig); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("signal helper: %v", err)
	}
	waitErr := cmd.Wait()
	return phase5ProcessResult{err: waitErr, stderr: stderr.String(), elapsed: time.Since(started)}
}

func runPhase5HelperWithoutSignal(t *testing.T, db *phase5IntegrationDB, mode, rootID string) {
	t.Helper()
	cmd := phase5HelperCommand(db, mode, rootID, "")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper failed: %v\n%s", err, output)
	}
}

// TestPhase5ProcessHelper is invoked in a subprocess by the integration tests.
// It follows the same signal/work/shutdown context separation as the CLI while
// using deterministic handlers instead of external npm requests.
func TestPhase5ProcessHelper(t *testing.T) {
	if os.Getenv("PHASE5_HELPER") != "1" {
		return
	}

	workCtx := context.Background()
	dbPool, err := openPhase5SchemaPool(
		workCtx,
		os.Getenv("PHASE5_DATABASE_URL"),
		os.Getenv("PHASE5_SCHEMA"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer dbPool.Close()

	signalCtx, stopSignals := signalContext()
	defer stopSignals()
	jobStore := storepg.New(dbPool)
	q := queue.New(4, jobStore)
	mode := os.Getenv("PHASE5_MODE")
	rootID := os.Getenv("PHASE5_ROOT_ID")
	childID := os.Getenv("PHASE5_CHILD_ID")

	var release chan struct{}
	var handler job.Handler
	if mode == "smoke" {
		handler = auditor.NewAuditHandler(
			phase5SmokeRegistry{},
			auditor.LicensePolicy{},
			auditor.NewPackageStore(),
			auditor.NewEdgeStore(),
		)
	} else {
		release = make(chan struct{})
		handler = phase5HandlerFunc(func(_ context.Context, j job.Job) ([]job.Job, error) {
			if j.ID != rootID {
				return nil, nil
			}
			fmt.Println("READY")
			<-release
			switch mode {
			case "clean", "timeout":
				return nil, nil
			case "child":
				child := job.NewJob("phase5-child", json.RawMessage(`{"source":"shutdown"}`))
				child.ID = childID
				return []job.Job{child}, nil
			case "retry":
				return nil, errors.New("retry during shutdown")
			default:
				return nil, fmt.Errorf("unknown helper mode %q", mode)
			}
		})
	}

	opts := []worker.Option{
		worker.WithStore(jobStore),
		worker.WithDLQ(dlq.New(jobStore)),
		worker.WithPollInterval(5 * time.Millisecond),
	}
	if mode == "retry" {
		retryDelay, parseErr := time.ParseDuration(os.Getenv("PHASE5_RETRY_DELAY"))
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		opts = append(opts, worker.WithBackoff(func(int) time.Duration { return retryDelay }))
	}
	p := worker.NewWithOptions(1, q, handler, opts...)

	var root job.Job
	if mode == "smoke" {
		payload, marshalErr := json.Marshal(auditor.AuditPayload{Name: "phase5-smoke", Version: "1.0.0"})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		root = job.NewJob("audit_package", payload)
	} else {
		root = job.NewJob("phase5-"+mode, json.RawMessage(`{"source":"helper"}`))
	}
	root.ID = rootID
	root.ScheduledAt = time.Now().Add(-time.Second)
	if err := p.Submit(root); err != nil {
		t.Fatal(err)
	}
	p.Start(workCtx)

	signaled := false
	select {
	case <-p.Done():
	case <-signalCtx.Done():
		signaled = true
	}
	if signaled && mode != "timeout" {
		go func() {
			time.Sleep(50 * time.Millisecond)
			close(release)
		}()
	}

	timeout, err := shutdownTimeoutFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := p.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown deadline %s exceeded: %v\n", timeout, err)
		os.Exit(phase5TimeoutExitCode)
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func phase5JobState(t *testing.T, db *phase5IntegrationDB, id string) (job.Status, int, time.Time) {
	t.Helper()
	var status job.Status
	var attempts int
	var scheduledAt time.Time
	if err := db.pool.QueryRow(context.Background(), `
		SELECT status, attempts, scheduled_at FROM jobs WHERE id=$1
	`, id).Scan(&status, &attempts, &scheduledAt); err != nil {
		t.Fatalf("query job %s: %v", id, err)
	}
	return status, attempts, scheduledAt
}

func runPhase5RestartPool(t *testing.T, db *phase5IntegrationDB, handler job.Handler) {
	t.Helper()
	s := storepg.New(db.pool)
	q := queue.New(4, s)
	p := worker.NewWithOptions(1, q, handler,
		worker.WithStore(s),
		worker.WithPollInterval(5*time.Millisecond),
	)
	p.Start(context.Background())
	select {
	case <-p.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("restarted pool did not finish")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown restarted pool: %v", err)
	}
}

func testPhase5CleanSignal(t *testing.T, sig os.Signal) {
	t.Helper()
	db := setupPhase5IntegrationDB(t)
	rootID := fmt.Sprintf("clean-%d", time.Now().UnixNano())
	result := runSignaledPhase5Helper(t, db, "clean", rootID, "", sig)
	if result.err != nil {
		t.Fatalf("clean-drain helper exited with error: %v\n%s", result.err, result.stderr)
	}
	status, _, _ := phase5JobState(t, db, rootID)
	if status != job.StatusCompleted {
		t.Fatalf("clean-drain job status=%q, want %q", status, job.StatusCompleted)
	}
}

func TestPhase5SIGTERMCleanDrain(t *testing.T) {
	testPhase5CleanSignal(t, syscall.SIGTERM)
}

func TestPhase5SIGINTCleanDrain(t *testing.T) {
	testPhase5CleanSignal(t, os.Interrupt)
}

func TestPhase5TimeoutAndRestart(t *testing.T) {
	db := setupPhase5IntegrationDB(t)
	rootID := fmt.Sprintf("timeout-%d", time.Now().UnixNano())
	result := runSignaledPhase5Helper(t, db, "timeout", rootID, "", syscall.SIGTERM)
	exitErr, ok := result.err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != phase5TimeoutExitCode {
		t.Fatalf("timeout helper error=%v, want exit code %d\n%s", result.err, phase5TimeoutExitCode, result.stderr)
	}
	if result.elapsed < 75*time.Millisecond || result.elapsed > time.Second {
		t.Fatalf("timeout helper exited after %v, want bounded shutdown near 100ms", result.elapsed)
	}
	status, _, _ := phase5JobState(t, db, rootID)
	if status != job.StatusRunning {
		t.Fatalf("interrupted job status=%q, want %q", status, job.StatusRunning)
	}

	var calls atomic.Int64
	runPhase5RestartPool(t, db, phase5HandlerFunc(func(context.Context, job.Job) ([]job.Job, error) {
		calls.Add(1)
		return nil, nil
	}))
	status, _, _ = phase5JobState(t, db, rootID)
	if calls.Load() != 1 || status != job.StatusCompleted {
		t.Fatalf("reclaimed job calls=%d status=%q", calls.Load(), status)
	}
}

func TestPhase5ChildFrontierPreserved(t *testing.T) {
	db := setupPhase5IntegrationDB(t)
	rootID := fmt.Sprintf("parent-%d", time.Now().UnixNano())
	childID := fmt.Sprintf("child-%d", time.Now().UnixNano())
	result := runSignaledPhase5Helper(t, db, "child", rootID, childID, syscall.SIGTERM)
	if result.err != nil {
		t.Fatalf("child helper exited with error: %v\n%s", result.err, result.stderr)
	}
	rootStatus, _, _ := phase5JobState(t, db, rootID)
	childStatus, _, _ := phase5JobState(t, db, childID)
	if rootStatus != job.StatusCompleted || childStatus != job.StatusPending {
		t.Fatalf("shutdown frontier root=%q child=%q", rootStatus, childStatus)
	}

	var calls atomic.Int64
	handledIDs := make(chan string, 1)
	runPhase5RestartPool(t, db, phase5HandlerFunc(func(_ context.Context, j job.Job) ([]job.Job, error) {
		handledIDs <- j.ID
		calls.Add(1)
		return nil, nil
	}))
	handledID := <-handledIDs
	childStatus, _, _ = phase5JobState(t, db, childID)
	if calls.Load() != 1 || handledID != childID || childStatus != job.StatusCompleted {
		t.Fatalf("restarted child calls=%d id=%q status=%q", calls.Load(), handledID, childStatus)
	}
}

func TestPhase5DurableRetryDuringShutdown(t *testing.T) {
	db := setupPhase5IntegrationDB(t)
	rootID := fmt.Sprintf("retry-%d", time.Now().UnixNano())
	result := runSignaledPhase5Helper(t, db, "retry", rootID, "", syscall.SIGTERM)
	if result.err != nil {
		t.Fatalf("retry helper exited with error: %v\n%s", result.err, result.stderr)
	}
	status, attempts, scheduledAt := phase5JobState(t, db, rootID)
	if status != job.StatusPending || attempts != 1 {
		t.Fatalf("retry state status=%q attempts=%d", status, attempts)
	}

	var mu sync.Mutex
	var handledAt time.Time
	runPhase5RestartPool(t, db, phase5HandlerFunc(func(context.Context, job.Job) ([]job.Job, error) {
		mu.Lock()
		handledAt = time.Now()
		mu.Unlock()
		return nil, nil
	}))
	mu.Lock()
	actualHandledAt := handledAt
	mu.Unlock()
	if actualHandledAt.Before(scheduledAt) {
		t.Fatalf("retry handled at %v before scheduled_at %v", actualHandledAt, scheduledAt)
	}
	status, attempts, _ = phase5JobState(t, db, rootID)
	if status != job.StatusCompleted || attempts != 1 {
		t.Fatalf("completed retry status=%q attempts=%d", status, attempts)
	}
}

func TestPhase5AuditorSmoke(t *testing.T) {
	db := setupPhase5IntegrationDB(t)
	rootID := fmt.Sprintf("smoke-%d", time.Now().UnixNano())
	runPhase5HelperWithoutSignal(t, db, "smoke", rootID)
	status, _, _ := phase5JobState(t, db, rootID)
	if status != job.StatusCompleted {
		t.Fatalf("auditor smoke job status=%q, want %q", status, job.StatusCompleted)
	}
}
