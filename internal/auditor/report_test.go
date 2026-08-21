package auditor_test

import (
	"strings"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildGraph is a convenience helper that populates a PackageStore and
// EdgeStore from a compact description, then returns them alongside an
// EdgeStore for use in GenerateReport.
//
// pkgs: slice of {name, version, license, verdict} tuples
// edges: slice of {fromName, fromVersion, toName, toVersion} tuples
func buildGraph(
	pkgRows []auditor.Package,
	edgeRows []auditor.DependencyEdge,
) (*auditor.PackageStore, *auditor.EdgeStore) {
	pkgs := auditor.NewPackageStore()
	edges := auditor.NewEdgeStore()
	for _, p := range pkgRows {
		pkgs.Add(p)
	}
	for _, e := range edgeRows {
		edges.Add(e)
	}
	return pkgs, edges
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestReportTotalPackages verifies TotalPackages equals the number of packages
// in the store regardless of verdict.
func TestReportTotalPackages(t *testing.T) {
	pkgs, edges := buildGraph(
		[]auditor.Package{
			{Name: "a", Version: "1.0.0", License: "MIT", Verdict: auditor.VerdictPass},
			{Name: "b", Version: "2.0.0", License: "MIT", Verdict: auditor.VerdictPass},
			{Name: "c", Version: "3.0.0", License: "GPL-3.0", Verdict: auditor.VerdictPolicyViolation},
		},
		nil,
	)
	r := auditor.GenerateReport(pkgs, edges, "root")
	if r.TotalPackages != 3 {
		t.Errorf("TotalPackages: got %d, want 3", r.TotalPackages)
	}
}

// TestReportNoViolations verifies that a clean graph produces an empty
// PolicyViolations slice and a summary mentioning 0 violations.
func TestReportNoViolations(t *testing.T) {
	pkgs, edges := buildGraph(
		[]auditor.Package{
			{Name: "express", Version: "4.18.2", License: "MIT", Verdict: auditor.VerdictPass},
		},
		nil,
	)
	r := auditor.GenerateReport(pkgs, edges, "my-app")
	if len(r.PolicyViolations) != 0 {
		t.Errorf("PolicyViolations: expected 0, got %d", len(r.PolicyViolations))
	}
	if !strings.Contains(r.Summary, "Policy violations: 0") {
		t.Errorf("Summary missing 'Policy violations: 0':\n%s", r.Summary)
	}
	if !strings.Contains(r.Summary, "Clean: 1 packages passed all checks.") {
		t.Errorf("Summary missing clean count:\n%s", r.Summary)
	}
}

// TestReportViolationCount verifies the number of PolicyViolations matches the
// number of packages with VerdictPolicyViolation.
func TestReportViolationCount(t *testing.T) {
	pkgs, edges := buildGraph(
		[]auditor.Package{
			{Name: "good", Version: "1.0.0", License: "MIT", Verdict: auditor.VerdictPass},
			{Name: "bad1", Version: "1.0.0", License: "GPL-3.0", Verdict: auditor.VerdictPolicyViolation},
			{Name: "bad2", Version: "2.0.0", License: "WTFPL", Verdict: auditor.VerdictPolicyViolation},
		},
		nil,
	)
	r := auditor.GenerateReport(pkgs, edges, "root")
	if len(r.PolicyViolations) != 2 {
		t.Errorf("PolicyViolations: expected 2, got %d", len(r.PolicyViolations))
	}
}

// TestReportPathLinearChain verifies path computation for a simple linear graph:
//
//	root → a@1.0.0 → b@2.0.0 (violation)
//
// Expected path: [root, a@1.0.0, b@2.0.0]
func TestReportPathLinearChain(t *testing.T) {
	pkgs, edges := buildGraph(
		[]auditor.Package{
			{Name: "a", Version: "1.0.0", License: "MIT", Verdict: auditor.VerdictPass},
			{Name: "b", Version: "2.0.0", License: "GPL-3.0", Verdict: auditor.VerdictPolicyViolation},
		},
		[]auditor.DependencyEdge{
			{FromName: "root", FromVersion: "", ToName: "a", ToVersion: "1.0.0"},
			{FromName: "a", FromVersion: "1.0.0", ToName: "b", ToVersion: "2.0.0"},
		},
	)
	r := auditor.GenerateReport(pkgs, edges, "root")

	if len(r.PolicyViolations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(r.PolicyViolations))
	}
	path := r.PolicyViolations[0].Path
	wantPath := []string{"root", "a@1.0.0", "b@2.0.0"}
	if !pathEqual(path, wantPath) {
		t.Errorf("path: got %v, want %v", path, wantPath)
	}
}

// TestReportPathDiamondDependency verifies path computation when a violation is
// reachable via two routes (diamond graph). Any valid path from root to the
// violation is acceptable — we assert the path starts at root and ends at the
// violation node.
//
//	root → a@1.0.0 → c@3.0.0 (violation)
//	root → b@2.0.0 → c@3.0.0
func TestReportPathDiamondDependency(t *testing.T) {
	pkgs, edges := buildGraph(
		[]auditor.Package{
			{Name: "a", Version: "1.0.0", License: "MIT", Verdict: auditor.VerdictPass},
			{Name: "b", Version: "2.0.0", License: "MIT", Verdict: auditor.VerdictPass},
			{Name: "c", Version: "3.0.0", License: "GPL-3.0", Verdict: auditor.VerdictPolicyViolation},
		},
		[]auditor.DependencyEdge{
			{FromName: "root", FromVersion: "", ToName: "a", ToVersion: "1.0.0"},
			{FromName: "root", FromVersion: "", ToName: "b", ToVersion: "2.0.0"},
			{FromName: "a", FromVersion: "1.0.0", ToName: "c", ToVersion: "3.0.0"},
			{FromName: "b", FromVersion: "2.0.0", ToName: "c", ToVersion: "3.0.0"},
		},
	)
	r := auditor.GenerateReport(pkgs, edges, "root")

	if len(r.PolicyViolations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(r.PolicyViolations))
	}
	path := r.PolicyViolations[0].Path
	if path[0] != "root" {
		t.Errorf("path must start at root, got %q", path[0])
	}
	if path[len(path)-1] != "c@3.0.0" {
		t.Errorf("path must end at violation node, got %q", path[len(path)-1])
	}
}

// TestReportSummaryFormat verifies the human-readable summary contains the
// expected sections from the spec.
func TestReportSummaryFormat(t *testing.T) {
	pkgs, edges := buildGraph(
		[]auditor.Package{
			{Name: "good", Version: "1.0.0", License: "MIT", Verdict: auditor.VerdictPass},
			{Name: "evil", Version: "0.1.0", License: "GPL-3.0", Verdict: auditor.VerdictPolicyViolation},
		},
		[]auditor.DependencyEdge{
			{FromName: "my-app", FromVersion: "", ToName: "evil", ToVersion: "0.1.0"},
		},
	)
	r := auditor.GenerateReport(pkgs, edges, "my-app")

	checks := []string{
		"=== Dependency Audit Report ===",
		"Root: my-app",
		"Packages scanned: 2",
		"Policy violations: 1",
		"evil@0.1.0",
		"GPL-3.0",
		"license not in allowlist",
		"Clean: 1 packages passed all checks.",
	}
	for _, want := range checks {
		if !strings.Contains(r.Summary, want) {
			t.Errorf("Summary missing %q:\n%s", want, r.Summary)
		}
	}
}

// TestReportEmptyLicenseSummaryReason verifies that a package with no declared
// license reports "no license declared" in the summary.
func TestReportEmptyLicenseSummaryReason(t *testing.T) {
	pkgs, edges := buildGraph(
		[]auditor.Package{
			{Name: "sketchy", Version: "2.3.1", License: "", Verdict: auditor.VerdictPolicyViolation},
		},
		nil,
	)
	r := auditor.GenerateReport(pkgs, edges, "root")
	if !strings.Contains(r.Summary, "no license declared") {
		t.Errorf("Summary should mention 'no license declared' for empty license:\n%s", r.Summary)
	}
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

func pathEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
