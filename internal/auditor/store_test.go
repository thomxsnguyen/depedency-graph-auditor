package auditor_test

import (
	"sync"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
)

// ---------------------------------------------------------------------------
// PackageStore
// ---------------------------------------------------------------------------

func TestPackageStoreAddNewReturnsTrue(t *testing.T) {
	s := auditor.NewPackageStore()
	p := auditor.Package{Name: "express", Version: "4.18.2", License: "MIT", Verdict: auditor.VerdictPass}

	if !s.Add(p) {
		t.Error("Add of new package: expected true, got false")
	}
}

func TestPackageStoreAddDuplicateReturnsFalse(t *testing.T) {
	s := auditor.NewPackageStore()
	p := auditor.Package{Name: "express", Version: "4.18.2", License: "MIT", Verdict: auditor.VerdictPass}

	s.Add(p)
	if s.Add(p) {
		t.Error("Add of duplicate package: expected false, got true")
	}
}

func TestPackageStoreExistsAfterAdd(t *testing.T) {
	s := auditor.NewPackageStore()
	p := auditor.Package{Name: "lodash", Version: "4.17.21"}

	if s.Exists("lodash", "4.17.21") {
		t.Error("Exists before Add: expected false")
	}
	s.Add(p)
	if !s.Exists("lodash", "4.17.21") {
		t.Error("Exists after Add: expected true")
	}
}

func TestPackageStoreExistsDifferentVersionFalse(t *testing.T) {
	s := auditor.NewPackageStore()
	s.Add(auditor.Package{Name: "lodash", Version: "4.17.21"})

	if s.Exists("lodash", "4.0.0") {
		t.Error("Exists for different version: expected false")
	}
}

func TestPackageStoreAllReturnsSnapshot(t *testing.T) {
	s := auditor.NewPackageStore()
	s.Add(auditor.Package{Name: "a", Version: "1.0.0"})
	s.Add(auditor.Package{Name: "b", Version: "2.0.0"})

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("All: expected 2 packages, got %d", len(all))
	}
}

// TestPackageStoreConcurrentSafe verifies there are no data races under
// concurrent Add/Exists from multiple goroutines.  Run with -race.
func TestPackageStoreConcurrentSafe(t *testing.T) {
	s := auditor.NewPackageStore()
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			p := auditor.Package{Name: "shared", Version: "1.0.0"}
			s.Add(p)
			s.Exists("shared", "1.0.0")
			s.All()
		}()
	}
	wg.Wait()

	// Exactly one Add must have succeeded; the rest are dedup'd.
	all := s.All()
	if len(all) != 1 {
		t.Fatalf("concurrent Add: expected exactly 1 package, got %d", len(all))
	}
}

// ---------------------------------------------------------------------------
// EdgeStore
// ---------------------------------------------------------------------------

func TestEdgeStoreAddAndAll(t *testing.T) {
	s := auditor.NewEdgeStore()

	e1 := auditor.DependencyEdge{FromName: "root", FromVersion: "1.0.0", ToName: "a", ToVersion: "2.0.0"}
	e2 := auditor.DependencyEdge{FromName: "root", FromVersion: "1.0.0", ToName: "b", ToVersion: "3.0.0"}

	s.Add(e1)
	s.Add(e2)

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("EdgeStore.All: expected 2 edges, got %d", len(all))
	}
}

// TestEdgeStoreAllReturnsIndependentSnapshot verifies that mutating the slice
// returned by All does not affect the internal store.
func TestEdgeStoreAllReturnsIndependentSnapshot(t *testing.T) {
	s := auditor.NewEdgeStore()
	s.Add(auditor.DependencyEdge{FromName: "a", ToName: "b"})

	snap := s.All()
	snap[0].ToName = "MUTATED"

	snap2 := s.All()
	if snap2[0].ToName == "MUTATED" {
		t.Error("EdgeStore.All snapshot was not independent of internal state")
	}
}

// TestEdgeStoreConcurrentSafe verifies there are no data races under concurrent
// Add calls.  Run with -race.
func TestEdgeStoreConcurrentSafe(t *testing.T) {
	s := auditor.NewEdgeStore()
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			s.Add(auditor.DependencyEdge{FromName: "a", ToName: "b"})
		}()
	}
	wg.Wait()

	if len(s.All()) != goroutines {
		t.Fatalf("concurrent Add: expected %d edges, got %d", goroutines, len(s.All()))
	}
}
