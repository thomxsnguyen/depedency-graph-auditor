package filegraph

import (
	"reflect"
	"sync"
	"testing"
)

func TestRepositoryIndexIsNormalizedImmutableAndSorted(t *testing.T) {
	input := []string{"src/z.py", "src\\a.ts", "src/a.ts", "../escape.py", "/absolute.py"}
	index := NewRepositoryIndex(input)
	input[0] = "changed.py"

	want := []string{"src/a.ts", "src/z.py"}
	if got := index.Paths(); !reflect.DeepEqual(got, want) {
		t.Fatalf("paths: got %v, want %v", got, want)
	}
	paths := index.Paths()
	paths[0] = "mutated.py"
	if got := index.Paths(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Paths exposed internal state: %v", got)
	}
	if !index.Has("src/a.ts") || index.Has("mutated.py") {
		t.Fatalf("unexpected index membership")
	}
}

func TestRepositoryIndexSupportsConcurrentReads(t *testing.T) {
	index := NewRepositoryIndex([]string{"src/app.ts", "src/app.py"})
	var readers sync.WaitGroup
	for range 20 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			if !index.Has("src/app.ts") || len(index.Paths()) != 2 {
				t.Errorf("unexpected concurrent index read")
			}
		}()
	}
	readers.Wait()
}
