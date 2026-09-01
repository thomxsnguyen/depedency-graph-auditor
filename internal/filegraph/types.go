package filegraph

// Node is one JavaScript or TypeScript source file in the project.
type Node struct {
	Path string `json:"path"`
}

// Edge records that From imports To.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Diagnostic records a source file that could not be fully mapped.
type Diagnostic struct {
	Path    string `json:"path"`
	Import  string `json:"import,omitempty"`
	Message string `json:"message"`
}

// Report is the deterministic JSON representation of a file graph.
type Report struct {
	Root        string       `json:"root"`
	Nodes       []Node       `json:"nodes"`
	Edges       []Edge       `json:"edges"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
