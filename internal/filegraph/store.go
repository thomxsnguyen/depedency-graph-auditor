package filegraph

import "sync"

// Store is the concurrent, in-memory result store for one file graph run.
type Store struct {
	mu          sync.RWMutex
	nodes       map[string]Node
	edges       map[Edge]struct{}
	diagnostics []Diagnostic
}

// NewStore returns an empty file graph store.
func NewStore() *Store {
	return &Store{
		nodes: make(map[string]Node),
		edges: make(map[Edge]struct{}),
	}
}

// AddNode inserts a file node if it is not already present.
func (s *Store) AddNode(node Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[node.Path] = node
}

// AddEdge inserts an exact file-import relationship once.
func (s *Store) AddEdge(edge Edge) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edges[edge] = struct{}{}
}

// AddDiagnostic records one mapping diagnostic.
func (s *Store) AddDiagnostic(diagnostic Diagnostic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.diagnostics = append(s.diagnostics, diagnostic)
}

// Nodes returns a snapshot of all file nodes.
func (s *Store) Nodes() []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nodes := make([]Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// Edges returns a snapshot of all file-import edges.
func (s *Store) Edges() []Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	edges := make([]Edge, 0, len(s.edges))
	for edge := range s.edges {
		edges = append(edges, edge)
	}
	return edges
}

// Diagnostics returns a snapshot of all mapping diagnostics.
func (s *Store) Diagnostics() []Diagnostic {
	s.mu.RLock()
	defer s.mu.RUnlock()
	diagnostics := make([]Diagnostic, len(s.diagnostics))
	copy(diagnostics, s.diagnostics)
	return diagnostics
}
