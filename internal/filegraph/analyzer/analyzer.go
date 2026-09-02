// Package analyzer defines the language-neutral source-file analysis contract.
package analyzer

import "context"

// Index is the immutable repository file lookup exposed to analyzers.
type Index interface {
	Has(path string) bool
	Paths() []string
}

// FileContext identifies one source file in a repository snapshot.
type FileContext struct {
	Root  string
	Path  string
	Index Index
}

// Dependency is one normalized file-to-file relationship.
type Dependency struct {
	Target     string
	Kind       string
	Confidence string
}

// Diagnostic is one source reference that could not be mapped completely.
type Diagnostic struct {
	Reference string
	Message   string
}

// Result is the normalized output from one language analyzer.
type Result struct {
	Dependencies []Dependency
	Diagnostics  []Diagnostic
}

// Analyzer extracts and resolves file dependencies for one source language.
type Analyzer interface {
	Supports(path string) bool
	Analyze(ctx context.Context, file FileContext) (Result, error)
}
