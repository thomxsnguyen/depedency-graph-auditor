//go:build integration

package postgres_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
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

const defaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/jobqueue?sslmode=disable"

type integrationDB struct {
	url    string
	schema string
	pool   *pgxpool.Pool
}

func setupIntegrationDB(t *testing.T) *integrationDB {
	t.Helper()
	ctx := context.Background()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = defaultDatabaseURL
	}

	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("open Postgres admin pool: %v", err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Fatalf("Postgres is required for integration tests: %v", err)
	}

	schema := fmt.Sprintf("phase4_test_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		admin.Close()
		t.Fatalf("create test schema: %v", err)
	}

	pool, err := openSchemaPool(ctx, url, schema)
	if err != nil {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+identifier+" CASCADE")
		admin.Close()
		t.Fatalf("open test schema pool: %v", err)
	}

	_, filename, _, _ := runtime.Caller(0)
	migrationPath := filepath.Join(filepath.Dir(filename), "../../../db/migrations/001_jobs.sql")
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
	return &integrationDB{url: url, schema: schema, pool: pool}
}

func openSchemaPool(ctx context.Context, url, schema string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(url)
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

func runProcessHelper(t *testing.T, db *integrationDB, action, id string, scheduledAt time.Time) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestPhase4ProcessHelper$")
	cmd.Env = append(os.Environ(),
		"PHASE4_HELPER=1",
		"PHASE4_DATABASE_URL="+db.url,
		"PHASE4_SCHEMA="+db.schema,
		"PHASE4_ACTION="+action,
		"PHASE4_JOB_ID="+id,
		"PHASE4_SCHEDULED_AT="+scheduledAt.Format(time.RFC3339Nano),
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper process failed: %v\n%s", err, output)
	}
}

func runAndKillHelper(t *testing.T, db *integrationDB, id string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestPhase4ProcessHelper$")
	cmd.Env = append(os.Environ(),
		"PHASE4_HELPER=1",
		"PHASE4_DATABASE_URL="+db.url,
		"PHASE4_SCHEMA="+db.schema,
		"PHASE4_ACTION=running",
		"PHASE4_JOB_ID="+id,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "READY" {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("crash helper did not become ready: %s", stderr.String())
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("crash helper exited normally; expected forced termination")
	}
}

func TestPhase4ProcessHelper(t *testing.T) {
	if os.Getenv("PHASE4_HELPER") != "1" {
		return
	}
	ctx := context.Background()
	pool, err := openSchemaPool(ctx, os.Getenv("PHASE4_DATABASE_URL"), os.Getenv("PHASE4_SCHEMA"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	s := storepg.New(pool)
	j := job.NewJob("integration", json.RawMessage(`{"source":"helper"}`))
	j.ID = os.Getenv("PHASE4_JOB_ID")
	if err := s.CreateJob(ctx, j); err != nil {
		t.Fatal(err)
	}
	j, found, err := s.AcquireJob(ctx)
	if err != nil || !found {
		t.Fatalf("acquire helper job: found=%v err=%v", found, err)
	}

	switch os.Getenv("PHASE4_ACTION") {
	case "running":
		fmt.Println("READY")
		select {}
	case "retry":
		scheduledAt, err := time.Parse(time.RFC3339Nano, os.Getenv("PHASE4_SCHEDULED_AT"))
		if err != nil {
			t.Fatal(err)
		}
		j.Attempts = 1
		j.ScheduledAt = scheduledAt
		if err := s.RetryJob(ctx, j); err != nil {
			t.Fatal(err)
		}
	case "dlq":
		j.Attempts = j.MaxAttempts
		persistentDLQ := dlq.New(s)
		persistentDLQ.Publish(j, errors.New("persistent failure"))
		if len(persistentDLQ.Entries()) != 1 {
			t.Fatal("persistent DLQ entry was not written")
		}
	default:
		t.Fatalf("unknown helper action %q", os.Getenv("PHASE4_ACTION"))
	}
}

type handlerFunc func(context.Context, job.Job) ([]job.Job, error)

func (f handlerFunc) Handle(ctx context.Context, j job.Job) ([]job.Job, error) {
	return f(ctx, j)
}

func waitDone(t *testing.T, p *worker.Pool) {
	t.Helper()
	select {
	case <-p.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("pool did not finish")
	}
}

func runPool(t *testing.T, p *worker.Pool, q *queue.Queue) {
	t.Helper()
	p.Start(context.Background())
	waitDone(t, p)
	q.Close()
	p.Wait()
}

func TestCrashAndResume(t *testing.T) {
	db := setupIntegrationDB(t)
	id := fmt.Sprintf("crash-%d", time.Now().UnixNano())
	runAndKillHelper(t, db, id)

	s := storepg.New(db.pool)
	q := queue.New(4, s)
	var calls int
	p := worker.NewWithOptions(1, q, handlerFunc(func(context.Context, job.Job) ([]job.Job, error) {
		calls++
		return nil, nil
	}), worker.WithStore(s), worker.WithPollInterval(5*time.Millisecond))
	runPool(t, p, q)

	var status job.Status
	if err := db.pool.QueryRow(context.Background(), "SELECT status FROM jobs WHERE id=$1", id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || status != job.StatusCompleted {
		t.Fatalf("resumed job calls=%d status=%q", calls, status)
	}
}

func TestDurableBackoffSurvivesRestart(t *testing.T) {
	db := setupIntegrationDB(t)
	id := fmt.Sprintf("backoff-%d", time.Now().UnixNano())
	scheduledAt := time.Now().Add(300 * time.Millisecond)
	runProcessHelper(t, db, "retry", id, scheduledAt)

	s := storepg.New(db.pool)
	q := queue.New(4, s)
	var mu sync.Mutex
	var handledAt time.Time
	p := worker.NewWithOptions(1, q, handlerFunc(func(context.Context, job.Job) ([]job.Job, error) {
		mu.Lock()
		handledAt = time.Now()
		mu.Unlock()
		return nil, nil
	}), worker.WithStore(s), worker.WithPollInterval(5*time.Millisecond))
	runPool(t, p, q)

	mu.Lock()
	actual := handledAt
	mu.Unlock()
	if actual.Before(scheduledAt) {
		t.Fatalf("job handled at %v before scheduled_at %v", actual, scheduledAt)
	}
	var attempts int
	var status job.Status
	if err := db.pool.QueryRow(context.Background(),
		"SELECT attempts, status FROM jobs WHERE id=$1", id,
	).Scan(&attempts, &status); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || status != job.StatusCompleted {
		t.Fatalf("durable retry attempts=%d status=%q", attempts, status)
	}
}

func TestDLQPersistsAcrossRestart(t *testing.T) {
	db := setupIntegrationDB(t)
	id := fmt.Sprintf("dlq-%d", time.Now().UnixNano())
	runProcessHelper(t, db, "dlq", id, time.Time{})

	entries := dlq.New(storepg.New(db.pool)).Entries()
	if len(entries) != 1 {
		t.Fatalf("DLQ entries=%d, want 1", len(entries))
	}
	if entries[0].Job.ID != id || entries[0].Err != "persistent failure" {
		t.Fatalf("unexpected DLQ entry: %+v", entries[0])
	}
}

type smokeRegistry struct{}

func (smokeRegistry) FetchPackage(context.Context, string, string) (*auditor.PackageMetadata, error) {
	return &auditor.PackageMetadata{
		Name:         "smoke-package",
		Version:      "1.0.0",
		License:      "MIT",
		Dependencies: map[string]string{},
	}, nil
}

func TestAuditorDurableStorageSmoke(t *testing.T) {
	db := setupIntegrationDB(t)
	s := storepg.New(db.pool)
	q := queue.New(4, s)
	packages := auditor.NewPackageStore()
	handler := auditor.NewAuditHandler(
		smokeRegistry{},
		auditor.LicensePolicy{},
		packages,
		auditor.NewEdgeStore(),
	)
	p := worker.NewWithOptions(1, q, handler,
		worker.WithStore(s),
		worker.WithDLQ(dlq.New(s)),
		worker.WithPollInterval(5*time.Millisecond),
	)
	payload, err := json.Marshal(auditor.AuditPayload{Name: "smoke-package", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	j := job.NewJob("audit_package", payload)
	p.Submit(j)
	runPool(t, p, q)

	if len(packages.All()) != 1 {
		t.Fatalf("audited packages=%d, want 1", len(packages.All()))
	}
	var status job.Status
	if err := db.pool.QueryRow(context.Background(), "SELECT status FROM jobs WHERE id=$1", j.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != job.StatusCompleted {
		t.Fatalf("durable smoke job status=%q", status)
	}
}
