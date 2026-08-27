package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/depfile"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/dlq"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
	storepg "github.com/thomxsnguyen/mini-distributed-job-api/internal/store/postgres"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/worker"
)

const defaultShutdownTimeout = 30 * time.Second

func main() {
	// 1. Read a package.json path from CLI args.
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: auditor <path/to/package.json>\n")
		os.Exit(1)
	}
	pkgJSONPath := os.Args[1]
	shutdownTimeout, err := shutdownTimeoutFromEnv()
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}

	// 2. Parse it → extract direct dependencies (prod + dev by default).
	deps, err := depfile.ParsePackageJSON(pkgJSONPath, true)
	if err != nil {
		log.Fatalf("depfile: %v", err)
	}
	if len(deps) == 0 {
		fmt.Println("No dependencies found in package.json — nothing to audit.")
		return
	}

	// Derive the root name from the package.json "name" field if present,
	// otherwise fall back to the file path.
	rootName := rootNameFromFile(pkgJSONPath)

	// 3. Version ranges are resolved lazily by the registry client when each
	//    job is processed — no pre-flight resolution is needed here.

	// 4. Create infrastructure.
	workCtx := context.Background()
	signalCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required (see .env.example)")
	}
	dbPool, err := pgxpool.New(workCtx, databaseURL)
	if err != nil {
		log.Fatalf("postgres: open pool: %v", err)
	}
	defer dbPool.Close()
	if err := dbPool.Ping(workCtx); err != nil {
		log.Fatalf("postgres: ping: %v", err)
	}

	jobStore := storepg.New(dbPool)
	const bufferSize = 100
	q := queue.New(bufferSize, jobStore)
	deadLetters := dlq.New(jobStore)

	pkgStore := auditor.NewPackageStore()
	edgeStore := auditor.NewEdgeStore()
	reg := auditor.NewNpmClient()
	policy := auditor.LicensePolicy{}
	handler := auditor.NewAuditHandler(reg, policy, pkgStore, edgeStore)

	// 5. Seed the queue: one job per direct dependency.
	const poolSize = 10
	pool := worker.NewWithOptions(poolSize, q, handler,
		worker.WithStore(jobStore),
		worker.WithDLQ(deadLetters),
	)

	for _, d := range deps {
		payload, err := json.Marshal(auditor.AuditPayload{Name: d.Name, Version: d.VersionRange})
		if err != nil {
			log.Fatalf("marshal seed job for %s: %v", d.Name, err)
		}
		pool.Submit(job.NewJob("audit_package", payload))
	}

	// 6 & 7. Start the worker pool.
	pool.Start(workCtx)

	// 8. Wait for normal completion or a shutdown signal. The signal context is
	// deliberately not passed to the pool, so active handlers keep their work
	// context while the pool drains.
	select {
	case <-pool.Done():
	case <-signalCtx.Done():
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	if err := pool.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown deadline %s exceeded: %v", shutdownTimeout, err)
		os.Exit(1)
	}

	// 9 & 10. Generate and print the report.
	report := auditor.GenerateReport(pkgStore, edgeStore, rootName)
	fmt.Print(report.Summary)
}

func shutdownTimeoutFromEnv() (time.Duration, error) {
	value := os.Getenv("SHUTDOWN_TIMEOUT")
	if value == "" {
		return defaultShutdownTimeout, nil
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("SHUTDOWN_TIMEOUT %q is not a valid duration: %w", value, err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive, got %q", value)
	}
	return timeout, nil
}

// rootNameFromFile reads the "name" field from a package.json file.
// If the file cannot be read or has no "name", it falls back to the path string.
func rootNameFromFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return path
	}
	defer f.Close()

	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(f).Decode(&pkg); err != nil || pkg.Name == "" {
		return path
	}
	return pkg.Name
}
