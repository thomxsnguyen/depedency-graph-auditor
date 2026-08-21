package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/depfile"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
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
	const bufferSize = 100
	q := queue.New(bufferSize)

	pkgStore := auditor.NewPackageStore()
	edgeStore := auditor.NewEdgeStore()
	reg := auditor.NewNpmClient()
	policy := auditor.LicensePolicy{}
	handler := auditor.NewAuditHandler(reg, policy, pkgStore, edgeStore)

	// 5. Seed the queue: one job per direct dependency.
	const poolSize = 10
	pool := worker.New(poolSize, q, handler)

	for _, d := range deps {
		payload, err := json.Marshal(auditor.AuditPayload{Name: d.Name, Version: d.VersionRange})
		if err != nil {
			log.Fatalf("marshal seed job for %s: %v", d.Name, err)
		}
		pool.Submit(job.Job{
			ID:      job.NewJobID(),
			Type:    "audit_package",
			Payload: payload,
			Status:  job.StatusPending,
		})
	}

	// 6 & 7. Start the worker pool.
	ctx := context.Background()
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
