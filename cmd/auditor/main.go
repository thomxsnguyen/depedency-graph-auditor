package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/depfile"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/dlq"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
	storepg "github.com/thomxsnguyen/mini-distributed-job-api/internal/store/postgres"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/worker"
)

func main() {
	// 1. Read a package.json path from CLI args.
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: auditor <path/to/package.json>\n")
		os.Exit(1)
	}
	pkgJSONPath := os.Args[1]

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
	ctx := context.Background()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required (see .env.example)")
	}
	dbPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("postgres: open pool: %v", err)
	}
	defer dbPool.Close()
	if err := dbPool.Ping(ctx); err != nil {
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
	pool.Start(ctx)

	// 8. Block until inFlight drops to zero (graph fully traversed).
	<-pool.Done()

	// Close the queue so worker goroutines exit, then wait for them.
	q.Close()
	pool.Wait()

	// 9 & 10. Generate and print the report.
	report := auditor.GenerateReport(pkgStore, edgeStore, rootName)
	fmt.Print(report.Summary)
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
