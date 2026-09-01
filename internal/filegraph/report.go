package filegraph

import (
	"encoding/json"
	"fmt"
	"sort"
)

// GenerateReport snapshots and deterministically orders one file graph.
func GenerateReport(root string, store *Store) Report {
	report := Report{
		Root:        root,
		Nodes:       store.Nodes(),
		Edges:       store.Edges(),
		Diagnostics: store.Diagnostics(),
	}
	sort.Slice(report.Nodes, func(i, j int) bool {
		return report.Nodes[i].Path < report.Nodes[j].Path
	})
	sort.Slice(report.Edges, func(i, j int) bool {
		if report.Edges[i].From == report.Edges[j].From {
			return report.Edges[i].To < report.Edges[j].To
		}
		return report.Edges[i].From < report.Edges[j].From
	})
	sort.Slice(report.Diagnostics, func(i, j int) bool {
		left, right := report.Diagnostics[i], report.Diagnostics[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Import != right.Import {
			return left.Import < right.Import
		}
		return left.Message < right.Message
	})
	return report
}

// MarshalReport renders indented UTF-8 JSON ending with one newline.
func MarshalReport(report Report) ([]byte, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("filegraph: encode report: %w", err)
	}
	return append(data, '\n'), nil
}
