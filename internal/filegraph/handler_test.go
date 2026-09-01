package filegraph

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	handler, err := NewHandler(root, index, store)
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
	handler, err := NewHandler(root, index, store)
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

func TestHandlerRejectsInvalidJobsAndPaths(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "src/App.ts", "")
	_, index, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(root, index, NewStore())
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

func TestReadSourceFileEnforcesSizeLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.ts")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", int(maxSourceFileBytes)+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readSourceFile(path); err == nil {
		t.Fatal("expected size-limit error")
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
