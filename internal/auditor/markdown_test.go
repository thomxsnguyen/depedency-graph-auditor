package auditor_test

import (
	"strings"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
)

func TestGenerateMarkdownReportRequiresReport(t *testing.T) {
	_, err := auditor.GenerateMarkdownReport(auditor.MarkdownReportInput{})
	if err == nil {
		t.Fatal("GenerateMarkdownReport returned nil error without a report")
	}
}

func TestGenerateMarkdownReportEmptyGraph(t *testing.T) {
	got, err := auditor.GenerateMarkdownReport(auditor.MarkdownReportInput{
		Root:   "empty-app",
		Report: &auditor.Report{},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"# Dependency Audit Report",
		"- Root: `empty-app`",
		"- Packages scanned: 0",
		"- Policy violations: 0",
		"```mermaid\ngraph TD\n```",
		"## Packages",
		"## Policy Violations",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateMarkdownReportRendersCompleteGraphAndPackages(t *testing.T) {
	packages := []auditor.Package{
		{Name: "shared", Version: "3.0.0", License: "MIT", Verdict: auditor.VerdictPass},
		{Name: "alpha", Version: "1.0.0", License: "Apache-2.0", Verdict: auditor.VerdictPass},
		{Name: "beta", Version: "2.0.0", License: "ISC", Verdict: auditor.VerdictPass},
	}
	edges := []auditor.DependencyEdge{
		{FromName: "beta", FromVersion: "2.0.0", ToName: "shared", ToVersion: "3.0.0"},
		{FromName: "alpha", FromVersion: "1.0.0", ToName: "shared", ToVersion: "3.0.0"},
	}

	got, err := auditor.GenerateMarkdownReport(auditor.MarkdownReportInput{
		Root:     "app",
		Packages: packages,
		Edges:    edges,
		Report:   &auditor.Report{TotalPackages: 3},
	})
	if err != nil {
		t.Fatal(err)
	}

	wants := []string{
		"n0[\"alpha@1.0.0\"]",
		"n1[\"beta@2.0.0\"]",
		"n2[\"shared@3.0.0\"]",
		"n0 --> n2",
		"n1 --> n2",
		"| `alpha` | `1.0.0` | `Apache-2.0` | `pass` |",
		"| `shared` | `3.0.0` | `MIT` | `pass` |",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateMarkdownReportDeduplicatesEdgesAndHandlesCycle(t *testing.T) {
	edgeAB := auditor.DependencyEdge{FromName: "a", FromVersion: "1", ToName: "b", ToVersion: "1"}
	edgeBA := auditor.DependencyEdge{FromName: "b", FromVersion: "1", ToName: "a", ToVersion: "1"}

	got, err := auditor.GenerateMarkdownReport(auditor.MarkdownReportInput{
		Root:   "app",
		Edges:  []auditor.DependencyEdge{edgeBA, edgeAB, edgeAB},
		Report: &auditor.Report{},
	})
	if err != nil {
		t.Fatal(err)
	}

	if count := strings.Count(got, "n0 --> n1"); count != 1 {
		t.Fatalf("a-to-b edge count: got %d, want 1\n%s", count, got)
	}
	if count := strings.Count(got, "n1 --> n0"); count != 1 {
		t.Fatalf("b-to-a edge count: got %d, want 1\n%s", count, got)
	}
}

func TestGenerateMarkdownReportIncludesIsolatedAndMissingEndpointNodes(t *testing.T) {
	got, err := auditor.GenerateMarkdownReport(auditor.MarkdownReportInput{
		Root: "app",
		Packages: []auditor.Package{
			{Name: "isolated", Version: "1.0.0", Verdict: auditor.VerdictPass},
		},
		Edges: []auditor.DependencyEdge{
			{FromName: "known-only-by-edge", FromVersion: "2.0.0", ToName: "missing-metadata", ToVersion: "3.0.0"},
		},
		Report: &auditor.Report{TotalPackages: 1},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, label := range []string{
		"isolated@1.0.0",
		"known-only-by-edge@2.0.0",
		"missing-metadata@3.0.0",
	} {
		if !strings.Contains(got, label) {
			t.Errorf("report missing node label %q:\n%s", label, got)
		}
	}
}

func TestGenerateMarkdownReportEscapesMermaidAndTableContent(t *testing.T) {
	got, err := auditor.GenerateMarkdownReport(auditor.MarkdownReportInput{
		Root: "root|line\nnext`",
		Packages: []auditor.Package{
			{
				Name:    "@scope/pkg\"\\\nnext",
				Version: "1|2`",
				License: "MIT|custom\nline",
				Verdict: auditor.VerdictPass,
			},
		},
		Report: &auditor.Report{TotalPackages: 1},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"- Root: `root\\|line next&#96;`",
		"n0[\"@scope/pkg\\\"\\\\ next@1|2`\"]",
		"| `@scope/pkg\"\\ next` | `1\\|2&#96;` | `MIT\\|custom line` | `pass` |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("escaped report missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateMarkdownReportRendersSortedViolations(t *testing.T) {
	violations := []auditor.PackageViolation{
		{
			Package: auditor.Package{Name: "zeta", Version: "2.0.0", License: "", Verdict: auditor.VerdictPolicyViolation},
		},
		{
			Package: auditor.Package{Name: "alpha", Version: "1.0.0", License: "GPL-3.0", Verdict: auditor.VerdictPolicyViolation},
			Path:    []string{"app", "alpha@1.0.0"},
		},
	}
	got, err := auditor.GenerateMarkdownReport(auditor.MarkdownReportInput{
		Root:   "app",
		Report: &auditor.Report{PolicyViolations: violations},
	})
	if err != nil {
		t.Fatal(err)
	}

	alpha := "| `alpha@1.0.0` | `GPL-3.0` | `app → alpha@1.0.0` |"
	zeta := "| `zeta@2.0.0` | Not declared | Not available |"
	alphaIndex := strings.Index(got, alpha)
	zetaIndex := strings.Index(got, zeta)
	if alphaIndex == -1 || zetaIndex == -1 {
		t.Fatalf("violation rows missing:\n%s", got)
	}
	if alphaIndex >= zetaIndex {
		t.Fatalf("violations are not sorted by package:\n%s", got)
	}
}

func TestGenerateMarkdownReportIsDeterministicAndDoesNotMutateInput(t *testing.T) {
	alpha := auditor.Package{Name: "alpha", Version: "1", Verdict: auditor.VerdictPass}
	beta := auditor.Package{Name: "beta", Version: "1", Verdict: auditor.VerdictPass}
	edge := auditor.DependencyEdge{FromName: "alpha", FromVersion: "1", ToName: "beta", ToVersion: "1"}
	report := &auditor.Report{TotalPackages: 2}
	packages := []auditor.Package{beta, alpha}
	edges := []auditor.DependencyEdge{edge, edge}

	first, err := auditor.GenerateMarkdownReport(auditor.MarkdownReportInput{
		Root: "app", Packages: packages, Edges: edges, Report: report,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := auditor.GenerateMarkdownReport(auditor.MarkdownReportInput{
		Root: "app", Packages: []auditor.Package{alpha, beta}, Edges: []auditor.DependencyEdge{edge}, Report: report,
	})
	if err != nil {
		t.Fatal(err)
	}

	if first != second {
		t.Fatalf("output differs for equivalent input ordering:\nFIRST:\n%s\nSECOND:\n%s", first, second)
	}
	if packages[0] != beta || len(edges) != 2 {
		t.Fatal("renderer mutated caller-owned input slices")
	}
}
