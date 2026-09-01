package filegraph

import (
	"sync"
	"testing"
)

func TestStoreDeduplicatesNodesAndEdgesDuringConcurrentWrites(t *testing.T) {
	store := NewStore()
	var writers sync.WaitGroup
	for range 20 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			store.AddNode(Node{Path: "src/App.tsx"})
			store.AddEdge(Edge{From: "src/App.tsx", To: "src/Button.tsx"})
			store.AddDiagnostic(Diagnostic{Path: "src/App.tsx", Message: "diagnostic"})
		}()
	}
	writers.Wait()

	if got := len(store.Nodes()); got != 1 {
		t.Fatalf("nodes: got %d, want 1", got)
	}
	if got := len(store.Edges()); got != 1 {
		t.Fatalf("edges: got %d, want 1", got)
	}
	if got := len(store.Diagnostics()); got != 20 {
		t.Fatalf("diagnostics: got %d, want 20", got)
	}
}

func TestStoreSnapshotsDoNotExposeInternalCollections(t *testing.T) {
	store := NewStore()
	store.AddNode(Node{Path: "src/App.tsx"})
	store.AddEdge(Edge{From: "src/App.tsx", To: "src/Button.tsx"})
	store.AddDiagnostic(Diagnostic{Path: "src/App.tsx", Message: "message"})

	nodes := store.Nodes()
	edges := store.Edges()
	diagnostics := store.Diagnostics()
	nodes[0].Path = "changed"
	edges[0].From = "changed"
	diagnostics[0].Path = "changed"

	if store.Nodes()[0].Path != "src/App.tsx" || store.Edges()[0].From != "src/App.tsx" || store.Diagnostics()[0].Path != "src/App.tsx" {
		t.Fatal("snapshot mutation changed the store")
	}
}
