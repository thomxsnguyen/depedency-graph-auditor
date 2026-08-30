package gomod

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fixtureRoundFetcher struct {
	metadata map[Coordinate]Metadata
	rounds   [][]Coordinate
	counts   map[Coordinate]int
	err      error
}

func (f *fixtureRoundFetcher) FetchRound(_ context.Context, coordinates []Coordinate) (map[Coordinate]Metadata, error) {
	f.rounds = append(f.rounds, append([]Coordinate(nil), coordinates...))
	if f.err != nil {
		return nil, f.err
	}
	if f.counts == nil {
		f.counts = make(map[Coordinate]int)
	}
	result := make(map[Coordinate]Metadata, len(coordinates))
	for _, coordinate := range coordinates {
		f.counts[coordinate]++
		if metadata, exists := f.metadata[coordinate]; exists {
			result[coordinate] = metadata
		}
	}
	return result, nil
}

func TestSelectSingleVersionAndTransitiveFixedPoint(t *testing.T) {
	a := coordinate("example.com/a", "v1.0.0")
	b := coordinate("example.com/b", "v1.2.0")
	fetcher := &fixtureRoundFetcher{metadata: map[Coordinate]Metadata{
		a: metadata(a, requirement(b)),
		b: metadata(b),
	}}
	selection, err := Select(context.Background(), "example.com/root", "1.16", []Requirement{requirement(a)}, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	assertCoordinates(t, selection.Modules, []Coordinate{a, b})
	if len(fetcher.rounds) != 2 || !reflect.DeepEqual(fetcher.rounds[0], []Coordinate{a}) || !reflect.DeepEqual(fetcher.rounds[1], []Coordinate{b}) {
		t.Fatalf("rounds: got %+v", fetcher.rounds)
	}
}

func TestSelectDiamondChoosesHighestSharedVersion(t *testing.T) {
	a := coordinate("example.com/a", "v1.0.0")
	b := coordinate("example.com/b", "v1.0.0")
	sharedLow := coordinate("example.com/shared", "v1.2.0")
	sharedHigh := coordinate("example.com/shared", "v1.5.0")
	fetcher := &fixtureRoundFetcher{metadata: map[Coordinate]Metadata{
		a:          metadata(a, requirement(sharedLow)),
		b:          metadata(b, requirement(sharedHigh)),
		sharedHigh: metadata(sharedHigh),
	}}
	selection, err := Select(context.Background(), "example.com/root", "1.16", []Requirement{requirement(b), requirement(a)}, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	assertCoordinates(t, selection.Modules, []Coordinate{a, b, sharedHigh})
	if fetcher.counts[sharedLow] != 0 || fetcher.counts[sharedHigh] != 1 {
		t.Fatalf("shared fetch counts: low=%d high=%d", fetcher.counts[sharedLow], fetcher.counts[sharedHigh])
	}
	wantEdges := []SelectedEdge{
		{From: coordinate("example.com/root", ""), To: a},
		{From: coordinate("example.com/root", ""), To: b},
		{From: a, To: sharedHigh},
		{From: b, To: sharedHigh},
	}
	if !reflect.DeepEqual(selection.Edges, wantEdges) {
		t.Fatalf("edges: got %+v, want %+v", selection.Edges, wantEdges)
	}
}

func TestSelectPromotionExcludesFetchedLowerVersion(t *testing.T) {
	a := coordinate("example.com/a", "v1.0.0")
	bridge := coordinate("example.com/bridge", "v1.0.0")
	sharedLow := coordinate("example.com/shared", "v1.2.0")
	sharedHigh := coordinate("example.com/shared", "v1.5.0")
	fetcher := &fixtureRoundFetcher{metadata: map[Coordinate]Metadata{
		a:          metadata(a, requirement(bridge), requirement(sharedLow)),
		bridge:     metadata(bridge, requirement(sharedHigh)),
		sharedLow:  metadata(sharedLow),
		sharedHigh: metadata(sharedHigh),
	}}
	selection, err := Select(context.Background(), "example.com/root", "1.16", []Requirement{requirement(a)}, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	assertCoordinates(t, selection.Modules, []Coordinate{a, bridge, sharedHigh})
	if fetcher.counts[sharedLow] != 1 || fetcher.counts[sharedHigh] != 1 {
		t.Fatalf("fetch counts: low=%d high=%d", fetcher.counts[sharedLow], fetcher.counts[sharedHigh])
	}
	for _, edge := range selection.Edges {
		if edge.From == sharedLow || edge.To == sharedLow {
			t.Fatalf("lower selected version leaked into final edge: %+v", edge)
		}
	}
	wantRedirected := SelectedEdge{From: a, To: sharedHigh}
	if !containsEdge(selection.Edges, wantRedirected) {
		t.Fatalf("missing redirected selected edge %+v in %+v", wantRedirected, selection.Edges)
	}
}

func TestSelectUsesGoOrderingAndKeepsMajorPathsDistinct(t *testing.T) {
	producerA := coordinate("example.com/a", "v1.0.0")
	producerB := coordinate("example.com/b", "v1.0.0")
	pseudoLow := coordinate("example.com/shared", "v0.0.0-20240101120000-abcdefabcdef")
	pseudoHigh := coordinate("example.com/shared", "v0.0.0-20240201120000-abcdefabcdef")
	moduleV1 := coordinate("example.com/module", "v1.9.0")
	moduleV2 := coordinate("example.com/module/v2", "v2.1.0")
	fetcher := &fixtureRoundFetcher{metadata: map[Coordinate]Metadata{
		producerA:  metadata(producerA, requirement(pseudoLow)),
		producerB:  metadata(producerB, requirement(pseudoHigh)),
		pseudoHigh: metadata(pseudoHigh),
		moduleV1:   metadata(moduleV1),
		moduleV2:   metadata(moduleV2),
	}}
	selection, err := Select(context.Background(), "example.com/root", "1.16", []Requirement{
		requirement(producerA), requirement(producerB), requirement(moduleV2), requirement(moduleV1),
	}, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	assertCoordinates(t, selection.Modules, []Coordinate{producerA, producerB, moduleV1, moduleV2, pseudoHigh})
}

func TestSelectIsIndependentOfRequirementAndFetchMapOrder(t *testing.T) {
	a := coordinate("example.com/a", "v1.0.0")
	b := coordinate("example.com/b", "v1.0.0")
	c := coordinate("example.com/c", "v1.0.0")
	fixtures := map[Coordinate]Metadata{
		a: metadata(a, requirement(c)),
		b: metadata(b, requirement(c)),
		c: metadata(c),
	}
	first, err := Select(context.Background(), "example.com/root", "1.16", []Requirement{requirement(a), requirement(b)}, &fixtureRoundFetcher{metadata: fixtures})
	if err != nil {
		t.Fatal(err)
	}
	secondFixtures := map[Coordinate]Metadata{c: fixtures[c], b: fixtures[b], a: fixtures[a]}
	second, err := Select(context.Background(), "example.com/root", "1.16", []Requirement{requirement(b), requirement(a)}, &fixtureRoundFetcher{metadata: secondFixtures})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("selection changed with input order:\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

func TestSelectBreaksSemanticVersionTiesDeterministically(t *testing.T) {
	plain := coordinate("example.com/module", "v1.2.3")
	incompatible := coordinate("example.com/module", "v1.2.3+incompatible")
	fixtures := map[Coordinate]Metadata{incompatible: metadata(incompatible)}
	first, err := Select(context.Background(), "example.com/root", "1.16", []Requirement{requirement(plain), requirement(incompatible)}, &fixtureRoundFetcher{metadata: fixtures})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Select(context.Background(), "example.com/root", "1.16", []Requirement{requirement(incompatible), requirement(plain)}, &fixtureRoundFetcher{metadata: fixtures})
	if err != nil {
		t.Fatal(err)
	}
	assertCoordinates(t, first.Modules, []Coordinate{incompatible})
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("selection changed across equivalent version order: first=%+v second=%+v", first, second)
	}
}

func TestSelectDeduplicatesCoordinatesAndEdges(t *testing.T) {
	a := coordinate("example.com/a", "v1.0.0")
	fetcher := &fixtureRoundFetcher{metadata: map[Coordinate]Metadata{a: metadata(a)}}
	selection, err := Select(context.Background(), "example.com/root", "1.16", []Requirement{requirement(a), requirement(a)}, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	assertCoordinates(t, selection.Modules, []Coordinate{a})
	if fetcher.counts[a] != 1 || len(selection.Edges) != 1 {
		t.Fatalf("count=%d edges=%+v", fetcher.counts[a], selection.Edges)
	}
}

func TestSelectEmptyBuildListNeedsNoFetcher(t *testing.T) {
	selection, err := Select(context.Background(), "example.com/root", "1.23", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(selection.Modules) != 0 || len(selection.Edges) != 0 {
		t.Fatalf("selection: %+v", selection)
	}
}

func TestSelectPrunesTransitiveMetadataForModernGraph(t *testing.T) {
	a := coordinate("example.com/a", "v1.0.0")
	b := coordinate("example.com/b", "v1.0.0")
	c := coordinate("example.com/c", "v1.0.0")
	fetcher := &fixtureRoundFetcher{metadata: map[Coordinate]Metadata{
		a: {ModulePath: a.ModulePath, GoVersion: "1.20", Requirements: []Requirement{requirement(b)}},
		b: {ModulePath: b.ModulePath, GoVersion: "1.20", Requirements: []Requirement{requirement(c)}},
	}}
	selection, err := Select(context.Background(), "example.com/root", "1.23", []Requirement{requirement(a)}, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	assertCoordinates(t, selection.Modules, []Coordinate{a, b})
	if fetcher.counts[a] != 1 || fetcher.counts[b] != 1 || fetcher.counts[c] != 0 {
		t.Fatalf("fetch counts: a=%d b=%d", fetcher.counts[a], fetcher.counts[b])
	}
	if !containsEdge(selection.Edges, SelectedEdge{From: a, To: b}) {
		t.Fatalf("immediate pruned-graph edge missing: %+v", selection.Edges)
	}
	if containsEdge(selection.Edges, SelectedEdge{From: b, To: c}) {
		t.Fatalf("pruned requirement edge leaked into selection: %+v", selection.Edges)
	}
}

func TestSelectDiscoversLegacyClosureBehindModernDependency(t *testing.T) {
	modern := coordinate("example.com/modern", "v1.0.0")
	legacy := coordinate("example.com/legacy", "v1.0.0")
	child := coordinate("example.com/child", "v1.0.0")
	fetcher := &fixtureRoundFetcher{metadata: map[Coordinate]Metadata{
		modern: {ModulePath: modern.ModulePath, GoVersion: "1.20", Requirements: []Requirement{requirement(legacy)}},
		legacy: {ModulePath: legacy.ModulePath, GoVersion: "1.16", Requirements: []Requirement{requirement(child)}},
		child:  metadata(child),
	}}
	selection, err := Select(context.Background(), "example.com/root", "1.23", []Requirement{requirement(modern)}, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	assertCoordinates(t, selection.Modules, []Coordinate{child, legacy, modern})
	if fetcher.counts[modern] != 1 || fetcher.counts[legacy] != 1 || fetcher.counts[child] != 1 {
		t.Fatalf("fetch counts: modern=%d legacy=%d child=%d", fetcher.counts[modern], fetcher.counts[legacy], fetcher.counts[child])
	}
	if !containsEdge(selection.Edges, SelectedEdge{From: legacy, To: child}) {
		t.Fatalf("legacy closure edge missing: %+v", selection.Edges)
	}
}

func TestSelectLoadsFullClosureThroughLegacyDependency(t *testing.T) {
	modern := coordinate("example.com/modern", "v1.0.0")
	legacy := coordinate("example.com/legacy", "v1.0.0")
	shared := coordinate("example.com/shared", "v1.0.0")
	child := coordinate("example.com/child", "v1.0.0")
	fetcher := &fixtureRoundFetcher{metadata: map[Coordinate]Metadata{
		modern: {ModulePath: modern.ModulePath, GoVersion: "1.20", Requirements: []Requirement{requirement(shared)}},
		legacy: {ModulePath: legacy.ModulePath, GoVersion: "1.16", Requirements: []Requirement{requirement(shared)}},
		shared: {ModulePath: shared.ModulePath, GoVersion: "1.20", Requirements: []Requirement{requirement(child)}},
		child:  metadata(child),
	}}
	selection, err := Select(context.Background(), "example.com/root", "1.23", []Requirement{requirement(modern), requirement(legacy)}, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	assertCoordinates(t, selection.Modules, []Coordinate{child, legacy, modern, shared})
	for _, coordinate := range []Coordinate{modern, legacy, shared, child} {
		if fetcher.counts[coordinate] != 1 {
			t.Fatalf("fetch count for %+v: got %d, want 1", coordinate, fetcher.counts[coordinate])
		}
	}
	if !containsEdge(selection.Edges, SelectedEdge{From: shared, To: child}) {
		t.Fatalf("legacy closure edge missing: %+v", selection.Edges)
	}
}

func TestSelectPrePruningRootLoadsFullClosure(t *testing.T) {
	modern := coordinate("example.com/modern", "v1.0.0")
	child := coordinate("example.com/child", "v1.0.0")
	fetcher := &fixtureRoundFetcher{metadata: map[Coordinate]Metadata{
		modern: {ModulePath: modern.ModulePath, GoVersion: "1.23", Requirements: []Requirement{requirement(child)}},
		child:  metadata(child),
	}}
	selection, err := Select(context.Background(), "example.com/root", "1.16", []Requirement{requirement(modern)}, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	assertCoordinates(t, selection.Modules, []Coordinate{child, modern})
	if fetcher.counts[child] != 1 {
		t.Fatalf("child fetch count: got %d, want 1", fetcher.counts[child])
	}
}

func TestSelectRejectsIncompleteOrInvalidRounds(t *testing.T) {
	a := coordinate("example.com/a", "v1.0.0")
	tests := []struct {
		name    string
		fetcher RoundFetcher
		wantErr string
	}{
		{name: "nil fetcher", wantErr: "fetcher is required"},
		{name: "fetch failure", fetcher: &fixtureRoundFetcher{err: errors.New("unavailable")}, wantErr: "unavailable"},
		{name: "missing result", fetcher: &fixtureRoundFetcher{metadata: map[Coordinate]Metadata{}}, wantErr: "did not return metadata"},
		{name: "module mismatch", fetcher: &fixtureRoundFetcher{metadata: map[Coordinate]Metadata{a: {ModulePath: "example.com/other"}}}, wantErr: "declares module"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Select(context.Background(), "example.com/root", "1.16", []Requirement{requirement(a)}, tt.fetcher)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error: got %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func coordinate(modulePath, version string) Coordinate {
	return Coordinate{ModulePath: modulePath, Version: version}
}

func requirement(coordinate Coordinate) Requirement {
	return Requirement{ModulePath: coordinate.ModulePath, Version: coordinate.Version}
}

func metadata(coordinate Coordinate, requirements ...Requirement) Metadata {
	return Metadata{ModulePath: coordinate.ModulePath, Requirements: requirements}
}

func assertCoordinates(t *testing.T, got, want []Coordinate) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("coordinates: got %+v, want %+v", got, want)
	}
}

func containsEdge(edges []SelectedEdge, target SelectedEdge) bool {
	for _, edge := range edges {
		if edge == target {
			return true
		}
	}
	return false
}
