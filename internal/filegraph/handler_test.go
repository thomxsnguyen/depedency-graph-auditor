package filegraph

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	fileanalyzer "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/javascript"
	pythonanalyzer "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/analyzer/python"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

func TestHandlerRecordsResolvedEdgesAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "src/App.tsx", `import "./Button"; import "./missing"; import React from "react";`)
	writeFixture(t, root, "src/Button.tsx", "")
	paths, index, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore()
	for _, path := range paths {
		store.AddNode(Node{Path: path})
	}
	handler, err := NewHandler(root, index, testAnalyzerRegistry(t), store)
	if err != nil {
		t.Fatal(err)
	}
	absoluteRoot, _ := filepath.Abs(root)
	queued, err := NewJob(absoluteRoot, "src/App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	children, err := handler.Handle(context.Background(), queued)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 0 {
		t.Fatalf("children: got %d, want 0", len(children))
	}
	if got := store.Edges(); len(got) != 1 || got[0] != (Edge{From: "src/App.tsx", To: "src/Button.tsx"}) {
		t.Fatalf("edges: %+v", got)
	}
	if got := store.Diagnostics(); len(got) != 1 || got[0].Import != "./missing" {
		t.Fatalf("diagnostics: %+v", got)
	}
}

func TestHandlerRecordsExtractionFailuresWithoutRetry(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "src/App.ts", `import "./unterminated`)
	paths, index, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore()
	handler, err := NewHandler(root, index, testAnalyzerRegistry(t), store)
	if err != nil {
		t.Fatal(err)
	}
	absoluteRoot, _ := filepath.Abs(root)
	queued, _ := NewJob(absoluteRoot, paths[0])
	if _, err := handler.Handle(context.Background(), queued); err != nil {
		t.Fatalf("deterministic extraction error should complete: %v", err)
	}
	if got := store.Diagnostics(); len(got) != 1 || !strings.Contains(got[0].Message, "unterminated") {
		t.Fatalf("diagnostics: %+v", got)
	}
}

func TestHandlerRecordsPythonFileEdgesAndIgnoresExternalImports(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "src/pc_diagnostic/__init__.py", "")
	writeFixture(t, root, "src/pc_diagnostic/main.py", `
from pc_diagnostic.models import Snapshot
from pc_diagnostic.missing import Missing
import psutil
`)
	writeFixture(t, root, "src/pc_diagnostic/models.py", "")
	paths, index, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore()
	for _, path := range paths {
		store.AddNode(Node{Path: path})
	}
	handler, err := NewHandler(root, index, testAnalyzerRegistry(t), store)
	if err != nil {
		t.Fatal(err)
	}
	absoluteRoot, _ := filepath.Abs(root)
	queued, _ := NewJob(absoluteRoot, "src/pc_diagnostic/main.py")
	if _, err := handler.Handle(context.Background(), queued); err != nil {
		t.Fatal(err)
	}

	edges := store.Edges()
	if len(edges) != 1 || edges[0] != (Edge{From: "src/pc_diagnostic/main.py", To: "src/pc_diagnostic/models.py"}) {
		t.Fatalf("edges: %+v", edges)
	}
	diagnostics := store.Diagnostics()
	if len(diagnostics) != 1 || diagnostics[0].Import != "pc_diagnostic.missing" {
		t.Fatalf("diagnostics: %+v", diagnostics)
	}
}

func TestHandlerRejectsInvalidJobsAndPaths(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "src/App.ts", "")
	_, index, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(root, index, testAnalyzerRegistry(t), NewStore())
	if err != nil {
		t.Fatal(err)
	}
	absoluteRoot, _ := filepath.Abs(root)

	tests := []job.Job{
		job.NewJob("audit_package", json.RawMessage(`{}`)),
		job.NewJob(JobType, json.RawMessage(`{`)),
		job.NewJob(JobType, mustPayload(t, Payload{Root: absoluteRoot, Path: "../secret.ts"})),
		job.NewJob(JobType, mustPayload(t, Payload{Root: t.TempDir(), Path: "src/App.ts"})),
		job.NewJob(JobType, mustPayload(t, Payload{Root: absoluteRoot, Path: "src/missing.ts"})),
	}
	for _, queued := range tests {
		if _, err := handler.Handle(context.Background(), queued); err == nil {
			t.Fatalf("expected rejection for %+v", queued)
		}
	}
}

func mustPayload(t *testing.T, payload Payload) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testAnalyzerRegistry(t *testing.T) *fileanalyzer.Registry {
	t.Helper()
	registry, err := fileanalyzer.NewRegistry(javascript.New(), pythonanalyzer.New())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}
