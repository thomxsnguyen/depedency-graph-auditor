package gomod

import (
	"reflect"
	"sort"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
)

func TestMapSelectionWritesOnlyStableSelectedGraph(t *testing.T) {
	root := Coordinate{ModulePath: "example.com/root"}
	a := Coordinate{ModulePath: "example.com/a", Version: "v1.0.0"}
	shared := Coordinate{ModulePath: "example.com/shared", Version: "v1.5.0"}
	selection := Selection{
		Modules: []Coordinate{a, shared},
		Edges: []SelectedEdge{
			{From: root, To: a},
			{From: a, To: shared},
		},
	}
	packages := auditor.NewPackageStore()
	edges := auditor.NewEdgeStore()
	MapSelection(selection, packages, edges, auditor.LicensePolicy{})

	allPackages := packages.All()
	sort.Slice(allPackages, func(i, j int) bool { return allPackages[i].Name < allPackages[j].Name })
	wantPackages := []auditor.Package{
		{Name: a.ModulePath, Version: a.Version, License: UnknownLicense, Verdict: auditor.VerdictPolicyViolation},
		{Name: shared.ModulePath, Version: shared.Version, License: UnknownLicense, Verdict: auditor.VerdictPolicyViolation},
	}
	if !reflect.DeepEqual(allPackages, wantPackages) {
		t.Fatalf("packages: got %+v, want %+v", allPackages, wantPackages)
	}
	wantEdges := []auditor.DependencyEdge{
		{FromName: root.ModulePath, ToName: a.ModulePath, ToVersion: a.Version},
		{FromName: a.ModulePath, FromVersion: a.Version, ToName: shared.ModulePath, ToVersion: shared.Version},
	}
	if got := edges.All(); !reflect.DeepEqual(got, wantEdges) {
		t.Fatalf("edges: got %+v, want %+v", got, wantEdges)
	}
	if packages.Exists("example.com/shared", "v1.2.0") {
		t.Fatal("unselected lower version leaked into package store")
	}
}

func TestMapSelectionSortsAndDeduplicatesEdges(t *testing.T) {
	root := Coordinate{ModulePath: "example.com/root"}
	a := Coordinate{ModulePath: "example.com/a", Version: "v1.0.0"}
	b := Coordinate{ModulePath: "example.com/b", Version: "v1.0.0"}
	selection := Selection{
		Modules: []Coordinate{b, a},
		Edges: []SelectedEdge{
			{From: root, To: b},
			{From: root, To: a},
			{From: root, To: a},
		},
	}
	packages := auditor.NewPackageStore()
	edges := auditor.NewEdgeStore()
	MapSelection(selection, packages, edges, auditor.LicensePolicy{})

	if len(packages.All()) != 2 {
		t.Fatalf("packages: got %+v", packages.All())
	}
	wantEdges := []auditor.DependencyEdge{
		{FromName: root.ModulePath, ToName: a.ModulePath, ToVersion: a.Version},
		{FromName: root.ModulePath, ToName: b.ModulePath, ToVersion: b.Version},
	}
	if got := edges.All(); !reflect.DeepEqual(got, wantEdges) {
		t.Fatalf("edges: got %+v, want %+v", got, wantEdges)
	}
}
