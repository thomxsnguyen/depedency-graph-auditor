package gomod

import (
	"context"
	"fmt"
	"sort"

	"golang.org/x/mod/module"
)

// Coordinate identifies one exact module version.
type Coordinate struct {
	ModulePath string
	Version    string
}

// SelectedEdge is an edge between final selected coordinates. The root uses
// its module path with an empty version.
type SelectedEdge struct {
	From Coordinate
	To   Coordinate
}

// Selection is the deterministic final MVS build list and its selected edges.
type Selection struct {
	Modules []Coordinate
	Edges   []SelectedEdge
}

// RoundFetcher obtains metadata for one complete deterministic selection
// round. Queue-backed concurrency can implement this boundary without changing
// MVS behavior.
type RoundFetcher interface {
	FetchRound(context.Context, []Coordinate) (map[Coordinate]Metadata, error)
}

// Select resolves the fixed-point MVS build list over the requirement metadata
// supplied by fetch rounds. Requirement pruning is intentionally owned by the
// caller that supplies metadata to this boundary.
func Select(ctx context.Context, rootModulePath string, rootRequirements []Requirement, fetcher RoundFetcher) (Selection, error) {
	if err := module.CheckPath(rootModulePath); err != nil {
		return Selection{}, fmt.Errorf("MVS root module %q: %w", rootModulePath, err)
	}
	if err := validateRequirements("root", rootRequirements); err != nil {
		return Selection{}, err
	}

	metadataCache := make(map[Coordinate]Metadata)
	for {
		selected, err := computeSelected(rootRequirements, metadataCache)
		if err != nil {
			return Selection{}, err
		}
		missing := missingCoordinates(selected, metadataCache)
		if len(missing) == 0 {
			return buildSelection(rootModulePath, rootRequirements, selected, metadataCache)
		}
		if fetcher == nil {
			return Selection{}, fmt.Errorf("MVS metadata fetcher is required")
		}

		fetched, err := fetcher.FetchRound(ctx, append([]Coordinate(nil), missing...))
		if err != nil {
			return Selection{}, fmt.Errorf("MVS fetch round: %w", err)
		}
		if err := addFetchRound(missing, fetched, metadataCache); err != nil {
			return Selection{}, err
		}
	}
}

func computeSelected(rootRequirements []Requirement, metadataCache map[Coordinate]Metadata) (map[string]string, error) {
	selected := make(map[string]string)
	for _, requirement := range rootRequirements {
		if err := selectHigher(selected, requirement); err != nil {
			return nil, err
		}
	}
	coordinates := sortedMetadataCoordinates(metadataCache)
	for _, coordinate := range coordinates {
		for _, requirement := range metadataCache[coordinate].Requirements {
			if err := selectHigher(selected, requirement); err != nil {
				return nil, err
			}
		}
	}
	return selected, nil
}

func selectHigher(selected map[string]string, requirement Requirement) error {
	current, exists := selected[requirement.ModulePath]
	if !exists {
		selected[requirement.ModulePath] = requirement.Version
		return nil
	}
	comparison, err := CompareVersions(requirement.Version, current)
	if err != nil {
		return fmt.Errorf("MVS compare %s versions: %w", requirement.ModulePath, err)
	}
	if comparison > 0 || (comparison == 0 && requirement.Version > current) {
		selected[requirement.ModulePath] = requirement.Version
	}
	return nil
}

func missingCoordinates(selected map[string]string, metadataCache map[Coordinate]Metadata) []Coordinate {
	missing := make([]Coordinate, 0)
	for modulePath, version := range selected {
		coordinate := Coordinate{ModulePath: modulePath, Version: version}
		if _, exists := metadataCache[coordinate]; !exists {
			missing = append(missing, coordinate)
		}
	}
	sortCoordinates(missing)
	return missing
}

func addFetchRound(requested []Coordinate, fetched map[Coordinate]Metadata, metadataCache map[Coordinate]Metadata) error {
	requestedSet := make(map[Coordinate]struct{}, len(requested))
	for _, coordinate := range requested {
		requestedSet[coordinate] = struct{}{}
	}
	fetchedCoordinates := make([]Coordinate, 0, len(fetched))
	for coordinate := range fetched {
		fetchedCoordinates = append(fetchedCoordinates, coordinate)
	}
	sortCoordinates(fetchedCoordinates)
	for _, coordinate := range fetchedCoordinates {
		if _, expected := requestedSet[coordinate]; !expected {
			return fmt.Errorf("MVS fetch round returned unexpected metadata for %s@%s", coordinate.ModulePath, coordinate.Version)
		}
		metadata := fetched[coordinate]
		if metadata.ModulePath != coordinate.ModulePath {
			return fmt.Errorf("MVS metadata for %s@%s declares module %q", coordinate.ModulePath, coordinate.Version, metadata.ModulePath)
		}
		if err := validateRequirements(coordinate.ModulePath+"@"+coordinate.Version, metadata.Requirements); err != nil {
			return err
		}
	}
	for _, coordinate := range requested {
		metadata, exists := fetched[coordinate]
		if !exists {
			return fmt.Errorf("MVS fetch round did not return metadata for %s@%s", coordinate.ModulePath, coordinate.Version)
		}
		metadataCache[coordinate] = metadata
	}
	return nil
}

func validateRequirements(owner string, requirements []Requirement) error {
	for _, requirement := range requirements {
		if err := ValidateCoordinate(requirement.ModulePath, requirement.Version); err != nil {
			return fmt.Errorf("MVS %s requirement %s@%s: %w", owner, requirement.ModulePath, requirement.Version, err)
		}
	}
	return nil
}

func buildSelection(rootModulePath string, rootRequirements []Requirement, selected map[string]string, metadataCache map[Coordinate]Metadata) (Selection, error) {
	modules := make([]Coordinate, 0, len(selected))
	for modulePath, version := range selected {
		modules = append(modules, Coordinate{ModulePath: modulePath, Version: version})
	}
	sortCoordinates(modules)

	edgeSet := make(map[SelectedEdge]struct{})
	root := Coordinate{ModulePath: rootModulePath}
	addSelectedEdges(edgeSet, root, rootRequirements, selected)
	for _, coordinate := range modules {
		metadata, exists := metadataCache[coordinate]
		if !exists {
			return Selection{}, fmt.Errorf("MVS selected metadata is missing for %s@%s", coordinate.ModulePath, coordinate.Version)
		}
		addSelectedEdges(edgeSet, coordinate, metadata.Requirements, selected)
	}
	edges := make([]SelectedEdge, 0, len(edgeSet))
	for edge := range edgeSet {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		leftFromRoot := edges[i].From == root
		rightFromRoot := edges[j].From == root
		if leftFromRoot != rightFromRoot {
			return leftFromRoot
		}
		if comparison := compareCoordinates(edges[i].From, edges[j].From); comparison != 0 {
			return comparison < 0
		}
		return compareCoordinates(edges[i].To, edges[j].To) < 0
	})
	return Selection{Modules: modules, Edges: edges}, nil
}

func addSelectedEdges(edges map[SelectedEdge]struct{}, from Coordinate, requirements []Requirement, selected map[string]string) {
	for _, requirement := range requirements {
		edges[SelectedEdge{
			From: from,
			To:   Coordinate{ModulePath: requirement.ModulePath, Version: selected[requirement.ModulePath]},
		}] = struct{}{}
	}
}

func sortedMetadataCoordinates(metadataCache map[Coordinate]Metadata) []Coordinate {
	coordinates := make([]Coordinate, 0, len(metadataCache))
	for coordinate := range metadataCache {
		coordinates = append(coordinates, coordinate)
	}
	sortCoordinates(coordinates)
	return coordinates
}

func sortCoordinates(coordinates []Coordinate) {
	sort.Slice(coordinates, func(i, j int) bool {
		return compareCoordinates(coordinates[i], coordinates[j]) < 0
	})
}

func compareCoordinates(left, right Coordinate) int {
	if left.ModulePath < right.ModulePath {
		return -1
	}
	if left.ModulePath > right.ModulePath {
		return 1
	}
	comparison := compareCanonicalVersions(left.Version, right.Version)
	if comparison != 0 {
		return comparison
	}
	if left.Version < right.Version {
		return -1
	}
	if left.Version > right.Version {
		return 1
	}
	return 0
}
