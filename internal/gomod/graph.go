package gomod

import (
	"sort"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
)

// UnknownLicense is used because public proxy .mod metadata contains no
// canonical license information.
const UnknownLicense = "UNKNOWN"

// MapSelection writes only the stable selected graph into the existing audit
// stores. Metadata handlers never write to these stores directly.
func MapSelection(selection Selection, packages *auditor.PackageStore, edges *auditor.EdgeStore, policy auditor.PolicyChecker) {
	modules := append([]Coordinate(nil), selection.Modules...)
	sortCoordinates(modules)
	for _, coordinate := range modules {
		metadata := auditor.PackageMetadata{
			Name:    coordinate.ModulePath,
			Version: coordinate.Version,
			License: UnknownLicense,
		}
		packages.Add(auditor.Package{
			Name:    metadata.Name,
			Version: metadata.Version,
			License: metadata.License,
			Verdict: policy.Check(metadata),
		})
	}
	edgeSet := make(map[SelectedEdge]struct{}, len(selection.Edges))
	for _, edge := range selection.Edges {
		edgeSet[edge] = struct{}{}
	}
	selectedEdges := make([]SelectedEdge, 0, len(edgeSet))
	for edge := range edgeSet {
		selectedEdges = append(selectedEdges, edge)
	}
	sort.Slice(selectedEdges, func(i, j int) bool {
		leftFromRoot := selectedEdges[i].From.Version == ""
		rightFromRoot := selectedEdges[j].From.Version == ""
		if leftFromRoot != rightFromRoot {
			return leftFromRoot
		}
		if comparison := compareCoordinates(selectedEdges[i].From, selectedEdges[j].From); comparison != 0 {
			return comparison < 0
		}
		return compareCoordinates(selectedEdges[i].To, selectedEdges[j].To) < 0
	})
	for _, edge := range selectedEdges {
		edges.Add(auditor.DependencyEdge{
			FromName:    edge.From.ModulePath,
			FromVersion: edge.From.Version,
			ToName:      edge.To.ModulePath,
			ToVersion:   edge.To.Version,
		})
	}
}
