package auditor

import "sync"

// Package is an audited node in the dependency graph.
type Package struct {
	Name    string
	Version string
	License string
	Verdict Verdict
}

// DependencyEdge is a directed "depends on" relationship in the graph.
type DependencyEdge struct {
	FromName    string
	FromVersion string
	ToName      string
	ToVersion   string
}

// PackageStore is a thread-safe set of audited packages keyed by "name@version".
// It doubles as the deduplication registry — a package present here has already
// been seen and does not need to be enqueued again.
type PackageStore struct {
	mu       sync.RWMutex
	packages map[string]Package
}

// NewPackageStore returns an empty PackageStore.
func NewPackageStore() *PackageStore {
	return &PackageStore{
		packages: make(map[string]Package),
	}
}

func key(name, version string) string {
	return name + "@" + version
}

// Add inserts a package. Returns true if the package was new, false if it already existed.
// The bool signals whether the caller should continue to write edges and enqueue children.
func (s *PackageStore) Add(p Package) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(p.Name, p.Version)
	if _, exists := s.packages[k]; exists {
		return false
	}
	s.packages[k] = p
	return true
}

// Exists reports whether (name, version) has already been seen.
func (s *PackageStore) Exists(name, version string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.packages[key(name, version)]
	return ok
}

// All returns a snapshot of every audited package (for report generation).
func (s *PackageStore) All() []Package {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Package, 0, len(s.packages))
	for _, p := range s.packages {
		out = append(out, p)
	}
	return out
}

// EdgeStore records dependency relationships in the graph.
// It is append-only; all writes use a plain Mutex because there is nothing to read-optimise.
type EdgeStore struct {
	mu    sync.Mutex
	edges []DependencyEdge
}

// NewEdgeStore returns an empty EdgeStore.
func NewEdgeStore() *EdgeStore {
	return &EdgeStore{}
}

// Add records an edge.
func (s *EdgeStore) Add(e DependencyEdge) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edges = append(s.edges, e)
}

// All returns a snapshot of every edge (for report generation and path walking).
func (s *EdgeStore) All() []DependencyEdge {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]DependencyEdge, len(s.edges))
	copy(out, s.edges)
	return out
}
