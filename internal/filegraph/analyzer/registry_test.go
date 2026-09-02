package analyzer

import (
	"context"
	"strings"
	"sync"
	"testing"
)

type testAnalyzer struct {
	extension string
}

func (a testAnalyzer) Supports(path string) bool {
	return strings.HasSuffix(path, a.extension)
}

func (testAnalyzer) Analyze(context.Context, FileContext) (Result, error) {
	return Result{}, nil
}

func TestRegistrySelectsOneAnalyzer(t *testing.T) {
	javascript := testAnalyzer{extension: ".ts"}
	python := testAnalyzer{extension: ".py"}
	registry, err := NewRegistry(javascript, python)
	if err != nil {
		t.Fatal(err)
	}

	selected, found, err := registry.AnalyzerFor("src/app.py")
	if err != nil || !found || selected != python {
		t.Fatalf("selection: analyzer=%v found=%t err=%v", selected, found, err)
	}
	if selected, found, err := registry.AnalyzerFor("README.md"); err != nil || found || selected != nil {
		t.Fatalf("unsupported selection: analyzer=%v found=%t err=%v", selected, found, err)
	}
}

func TestRegistryRejectsAmbiguousSelection(t *testing.T) {
	registry, err := NewRegistry(testAnalyzer{extension: ".ts"}, testAnalyzer{extension: ".ts"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := registry.AnalyzerFor("src/app.ts"); err == nil {
		t.Fatal("expected ambiguous analyzer error")
	}
}

func TestRegistryRejectsNilAnalyzer(t *testing.T) {
	if _, err := NewRegistry(nil); err == nil {
		t.Fatal("expected nil analyzer error")
	}
}

func TestRegistrySupportsConcurrentSelection(t *testing.T) {
	registry, err := NewRegistry(testAnalyzer{extension: ".ts"}, testAnalyzer{extension: ".py"})
	if err != nil {
		t.Fatal(err)
	}
	var readers sync.WaitGroup
	for range 20 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			if _, found, err := registry.AnalyzerFor("src/app.ts"); err != nil || !found {
				t.Errorf("concurrent selection: found=%t err=%v", found, err)
			}
		}()
	}
	readers.Wait()
}
