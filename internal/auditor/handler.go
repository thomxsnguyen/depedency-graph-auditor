package auditor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

// AuditPayload is the JSON body carried by every "audit_package" job.
type AuditPayload struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// AuditHandler implements job.Handler. Each call resolves and audits one (name, version)
// pair and returns new jobs for every direct dependency not yet in the package store.
//
// The handler is intentionally unaware of the queue — it only returns the child jobs;
// the worker loop is responsible for submitting them.
type AuditHandler struct {
	registry     RegistryClient
	policy       PolicyChecker
	packageStore *PackageStore
	edgeStore    *EdgeStore
}

// NewAuditHandler constructs an AuditHandler with the provided dependencies.
func NewAuditHandler(
	reg RegistryClient,
	pol PolicyChecker,
	pkgs *PackageStore,
	edges *EdgeStore,
) *AuditHandler {
	return &AuditHandler{
		registry:     reg,
		policy:       pol,
		packageStore: pkgs,
		edgeStore:    edges,
	}
}

// Handle runs the 5-step resolve-and-audit logic for the given job.
//
//  1. Parse the payload to extract (name, version).
//  2. Fetch metadata from the registry — the I/O-bound step.
//  3. Evaluate the package against policy to produce a verdict.
//  4. Save the node to the package store (dedup: if already present, skip remaining steps).
//  5. Save edges and return new jobs for unseen direct dependencies.
func (h *AuditHandler) Handle(ctx context.Context, j job.Job) ([]job.Job, error) {
	// Step 1 — parse payload.
	var p AuditPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return nil, fmt.Errorf("auditor: unmarshal payload for job %s: %w", j.ID, err)
	}

	// Step 2 — fetch metadata from the registry.
	meta, err := h.registry.FetchPackage(ctx, p.Name, p.Version)
	if err != nil {
		return nil, fmt.Errorf("auditor: fetch %s@%s: %w", p.Name, p.Version, err)
	}

	// Step 3 — audit against policy.
	verdict := h.policy.Check(*meta)

	// Step 4 — save the node.
	// Add returns false if the package was already present. In that case another
	// worker has already (or is about to) write the edges for this node, so we
	// stop here to avoid duplicate edges and spurious child jobs. This is the
	// primary deduplication point for the graph traversal.
	pkg := Package{
		Name:    meta.Name,
		Version: meta.Version,
		License: meta.License,
		Verdict: verdict,
	}
	if !h.packageStore.Add(pkg) {
		return nil, nil // already seen — nothing more to do
	}

	// Step 5 — save edges and enqueue children.
	var newJobs []job.Job
	for depName, depVersion := range meta.Dependencies {
		// Record the dependency edge regardless of whether the child is new.
		h.edgeStore.Add(DependencyEdge{
			FromName:    meta.Name,
			FromVersion: meta.Version,
			ToName:      depName,
			ToVersion:   depVersion,
		})

		// Enqueue a new audit job only if this dependency has not been seen yet.
		// The check-then-enqueue window is an acceptable at-least-once race in
		// Phase 1: a duplicate job produces a no-op at step 4 above.
		if !h.packageStore.Exists(depName, depVersion) {
			payload, err := json.Marshal(AuditPayload{Name: depName, Version: depVersion})
			if err != nil {
				// Marshalling a plain struct cannot fail in practice, but be explicit.
				return nil, fmt.Errorf("auditor: marshal child payload for %s@%s: %w", depName, depVersion, err)
			}
			newJobs = append(newJobs, job.Job{
				ID:      job.NewJobID(),
				Type:    "audit_package",
				Payload: payload,
				Status:  job.StatusPending,
			})
		}
	}

	return newJobs, nil
}
