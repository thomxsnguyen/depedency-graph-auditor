package auditor_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

// ---------------------------------------------------------------------------
// Mock registry
// ---------------------------------------------------------------------------

// mockRegistry is a deterministic, in-memory RegistryClient for use in tests.
// Configure Packages to map "name@version" → PackageMetadata that FetchPackage
// will return. Set Err to simulate a registry failure.
type mockRegistry struct {
	Packages map[string]*auditor.PackageMetadata
	Err      error
}

func (m *mockRegistry) FetchPackage(_ context.Context, name, version string) (*auditor.PackageMetadata, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	key := name + "@" + version
	meta, ok := m.Packages[key]
	if !ok {
		return nil, errors.New("mock: package not found: " + key)
	}
	return meta, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newAuditJob builds a minimal "audit_package" job for the given name@version.
func newAuditJob(t *testing.T, name, version string) job.Job {
	t.Helper()
	payload, err := json.Marshal(auditor.AuditPayload{Name: name, Version: version})
	if err != nil {
		t.Fatalf("newAuditJob marshal: %v", err)
	}
	return job.Job{
		ID:      job.NewJobID(),
		Type:    "audit_package",
		Payload: payload,
		Status:  job.StatusPending,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestHandleHappyPath verifies that a package with two unseen dependencies
// produces two child jobs, one stored package node, and two stored edges.
func TestHandleHappyPath(t *testing.T) {
	reg := &mockRegistry{
		Packages: map[string]*auditor.PackageMetadata{
			"express@4.18.2": {
				Name:    "express",
				Version: "4.18.2",
				License: "MIT",
				Dependencies: map[string]string{
					"body-parser": "1.20.1",
					"qs":          "6.11.0",
				},
			},
		},
	}
	pkgs := auditor.NewPackageStore()
	edges := auditor.NewEdgeStore()
	h := auditor.NewAuditHandler(reg, auditor.LicensePolicy{}, pkgs, edges)

	j := newAuditJob(t, "express", "4.18.2")
	newJobs, err := h.Handle(context.Background(), j)

	if err != nil {
		t.Fatalf("Handle: unexpected error: %v", err)
	}
	if len(newJobs) != 2 {
		t.Fatalf("Handle: expected 2 child jobs, got %d", len(newJobs))
	}
	if len(pkgs.All()) != 1 {
		t.Fatalf("Handle: expected 1 package in store, got %d", len(pkgs.All()))
	}
	if len(edges.All()) != 2 {
		t.Fatalf("Handle: expected 2 edges in store, got %d", len(edges.All()))
	}
}

// TestHandleVerdictPolicyViolation verifies that a disallowed license produces
// a VerdictPolicyViolation on the stored package.
func TestHandleVerdictPolicyViolation(t *testing.T) {
	reg := &mockRegistry{
		Packages: map[string]*auditor.PackageMetadata{
			"evil-lib@0.1.0": {
				Name:         "evil-lib",
				Version:      "0.1.0",
				License:      "GPL-3.0",
				Dependencies: map[string]string{},
			},
		},
	}
	pkgs := auditor.NewPackageStore()
	h := auditor.NewAuditHandler(reg, auditor.LicensePolicy{}, pkgs, auditor.NewEdgeStore())

	_, err := h.Handle(context.Background(), newAuditJob(t, "evil-lib", "0.1.0"))
	if err != nil {
		t.Fatalf("Handle: unexpected error: %v", err)
	}

	all := pkgs.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 package, got %d", len(all))
	}
	if all[0].Verdict != auditor.VerdictPolicyViolation {
		t.Errorf("verdict: got %q, want %q", all[0].Verdict, auditor.VerdictPolicyViolation)
	}
}

// TestHandleDeduplicatesAlreadySeenPackage verifies that when the package is
// already in the store, Handle returns nil jobs and nil error — no duplicate
// edges are written and no child jobs are produced.
func TestHandleDeduplicatesAlreadySeenPackage(t *testing.T) {
	reg := &mockRegistry{
		Packages: map[string]*auditor.PackageMetadata{
			"lodash@4.17.21": {
				Name:         "lodash",
				Version:      "4.17.21",
				License:      "MIT",
				Dependencies: map[string]string{"dep-a": "1.0.0"},
			},
		},
	}
	pkgs := auditor.NewPackageStore()
	edges := auditor.NewEdgeStore()
	// Pre-populate the store to simulate a prior worker having audited lodash.
	pkgs.Add(auditor.Package{Name: "lodash", Version: "4.17.21"})

	h := auditor.NewAuditHandler(reg, auditor.LicensePolicy{}, pkgs, edges)
	newJobs, err := h.Handle(context.Background(), newAuditJob(t, "lodash", "4.17.21"))

	if err != nil {
		t.Fatalf("Handle (dedup): unexpected error: %v", err)
	}
	if newJobs != nil {
		t.Errorf("Handle (dedup): expected nil child jobs, got %v", newJobs)
	}
	if len(edges.All()) != 0 {
		t.Errorf("Handle (dedup): expected 0 edges written, got %d", len(edges.All()))
	}
}

// TestHandleSkipsChildJobForSeenDependency verifies that when a direct
// dependency is already in the package store, no child job is created for it —
// but its edge is still recorded.
func TestHandleSkipsChildJobForSeenDependency(t *testing.T) {
	reg := &mockRegistry{
		Packages: map[string]*auditor.PackageMetadata{
			"a@1.0.0": {
				Name:    "a",
				Version: "1.0.0",
				License: "MIT",
				Dependencies: map[string]string{
					"b": "2.0.0", // already seen
					"c": "3.0.0", // new
				},
			},
		},
	}
	pkgs := auditor.NewPackageStore()
	edges := auditor.NewEdgeStore()
	// Pre-populate b as already audited.
	pkgs.Add(auditor.Package{Name: "b", Version: "2.0.0"})

	h := auditor.NewAuditHandler(reg, auditor.LicensePolicy{}, pkgs, edges)
	newJobs, err := h.Handle(context.Background(), newAuditJob(t, "a", "1.0.0"))

	if err != nil {
		t.Fatalf("Handle: unexpected error: %v", err)
	}
	// Only one new job — for c@3.0.0; b@2.0.0 is skipped.
	if len(newJobs) != 1 {
		t.Fatalf("Handle: expected 1 child job, got %d", len(newJobs))
	}
	// Both edges must still be recorded.
	if len(edges.All()) != 2 {
		t.Errorf("Handle: expected 2 edges, got %d", len(edges.All()))
	}
}

// TestHandleRegistryErrorPropagates verifies that a registry failure is
// returned as an error and nothing is written to the stores.
func TestHandleRegistryErrorPropagates(t *testing.T) {
	reg := &mockRegistry{Err: errors.New("registry unavailable")}
	pkgs := auditor.NewPackageStore()
	edges := auditor.NewEdgeStore()
	h := auditor.NewAuditHandler(reg, auditor.LicensePolicy{}, pkgs, edges)

	_, err := h.Handle(context.Background(), newAuditJob(t, "some-pkg", "1.0.0"))

	if err == nil {
		t.Fatal("Handle: expected error from failing registry, got nil")
	}
	if len(pkgs.All()) != 0 {
		t.Errorf("Handle: expected 0 packages after registry error, got %d", len(pkgs.All()))
	}
}

// TestHandleBadPayloadReturnsError verifies that a malformed JSON payload
// returns an error without calling the registry.
func TestHandleBadPayloadReturnsError(t *testing.T) {
	// reg.Err is nil but Packages is empty — if registry is called, it will error
	// with "package not found", but we expect the payload error to come first.
	reg := &mockRegistry{Packages: map[string]*auditor.PackageMetadata{}}
	h := auditor.NewAuditHandler(reg, auditor.LicensePolicy{}, auditor.NewPackageStore(), auditor.NewEdgeStore())

	bad := job.Job{
		ID:      job.NewJobID(),
		Type:    "audit_package",
		Payload: []byte("not-valid-json"),
	}
	_, err := h.Handle(context.Background(), bad)
	if err == nil {
		t.Fatal("Handle: expected unmarshal error, got nil")
	}
}
