package analyzer

import "fmt"

// Registry is an immutable ordered collection of source-language analyzers.
type Registry struct {
	analyzers []Analyzer
}

// NewRegistry constructs a registry for concurrent read-only selection.
func NewRegistry(analyzers ...Analyzer) (*Registry, error) {
	registered := make([]Analyzer, len(analyzers))
	for index, candidate := range analyzers {
		if candidate == nil {
			return nil, fmt.Errorf("filegraph: analyzer %d is nil", index)
		}
		registered[index] = candidate
	}
	return &Registry{analyzers: registered}, nil
}

// AnalyzerFor selects the only analyzer that supports path.
func (r *Registry) AnalyzerFor(path string) (Analyzer, bool, error) {
	if r == nil {
		return nil, false, fmt.Errorf("filegraph: analyzer registry is required")
	}
	var selected Analyzer
	for _, candidate := range r.analyzers {
		if !candidate.Supports(path) {
			continue
		}
		if selected != nil {
			return nil, false, fmt.Errorf("filegraph: multiple analyzers support %q", path)
		}
		selected = candidate
	}
	return selected, selected != nil, nil
}
