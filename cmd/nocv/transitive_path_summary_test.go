package main

import (
	"bytes"
	"strings"
	"testing"

	"nocv/graph"
	"nocv/query"
)

func TestNodeInspectionSummarizesConvergentTransitivePathsAndKeepsExactDrillDown(t *testing.T) {
	g, ids := transitiveSummaryFixture(t, false)
	exactPaths := flattenedTransitivePaths(query.TransitiveDependents(g, ids.target))
	if len(exactPaths) != 3 {
		t.Fatalf("exact transitive paths = %#v, want 3", exactPaths)
	}
	for _, path := range exactPaths {
		if len(path.Steps) == 0 {
			t.Fatalf("exact query returned a zero-step path: %#v", path)
		}
	}

	var inspection bytes.Buffer
	if err := executeCommand(&inspection, g, invocation{name: "node.inspect", pattern: "./...", values: []string{string(ids.target)}}); err != nil {
		t.Fatal(err)
	}
	output := inspection.String()
	for _, want := range []string{
		"Transitive paths: 3",
		string(ids.shared) + " calls -> " + string(ids.target),
		"paths: 3",
		"Full paths:",
		"nocv node transitive-dependents <pattern> " + string(ids.target),
	} {
		if !strings.Contains(output, want) {
			t.Errorf("node inspection lacks %q:\n%s", want, output)
		}
	}
	for _, unwanted := range []string{
		string(ids.first) + " calls -> " + string(ids.shared),
		string(ids.second) + " calls -> " + string(ids.shared),
		"uncertain paths:",
		"[confirmed]",
		"terminal paths:",
	} {
		if strings.Contains(output, unwanted) {
			t.Errorf("node inspection unexpectedly contains %q:\n%s", unwanted, output)
		}
	}

	var exact bytes.Buffer
	if err := executeCommand(&exact, g, invocation{name: "node.transitive-dependents", pattern: "./...", values: []string{string(ids.target)}}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(exact.String(), "path "); got != len(exactPaths) {
		t.Fatalf("exact command rendered %d paths, want %d:\n%s", got, len(exactPaths), exact.String())
	}
	for _, id := range []graph.SymbolRef{ids.first, ids.second, ids.shared} {
		if !strings.Contains(exact.String(), string(id)) {
			t.Errorf("exact command omits %s:\n%s", id, exact.String())
		}
	}
}

func TestNodeInspectionAccountsForMultipleTransitiveFamiliesAndCertainty(t *testing.T) {
	g, ids := transitiveSummaryFixture(t, true)
	paths := flattenedTransitivePaths(query.TransitiveDependents(g, ids.target))
	level := query.GroupPathBranches(paths, nil, 0, query.PathFromEnd)
	if level.PathCount != 5 || level.UncertainPathCount != 3 || len(level.TerminalPathIndexes) != 0 || len(level.Branches) != 2 {
		t.Fatalf("transitive branch level = %#v, want five paths, three uncertain, and two families", level)
	}
	accounted := len(level.TerminalPathIndexes)
	for _, branch := range level.Branches {
		accounted += branch.PathCount
	}
	if accounted != len(paths) {
		t.Fatalf("displayed families account for %d paths, exact query has %d", accounted, len(paths))
	}

	var inspection bytes.Buffer
	if err := executeCommand(&inspection, g, invocation{name: "node.inspect", pattern: "./...", values: []string{string(ids.target)}}); err != nil {
		t.Fatal(err)
	}
	output := inspection.String()
	for _, want := range []string{
		"Transitive paths: 5",
		"uncertain paths: 3",
		string(ids.shared) + " calls -> " + string(ids.target),
		"paths: 3\n    uncertain paths: 1",
		string(ids.alternate) + " calls [uncertain] -> " + string(ids.target),
		"paths: 2\n    uncertain paths: 2",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("mixed transitive inspection lacks %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, string(ids.shared)+" calls [uncertain] -> ") || strings.Contains(output, "[confirmed]") {
		t.Errorf("branch-step and whole-path certainty were conflated:\n%s", output)
	}
}

type transitiveSummaryIDs struct {
	first     graph.SymbolRef
	second    graph.SymbolRef
	third     graph.SymbolRef
	shared    graph.SymbolRef
	alternate graph.SymbolRef
	target    graph.SymbolRef
}

func transitiveSummaryFixture(t *testing.T, multipleFamilies bool) (*graph.Graph, transitiveSummaryIDs) {
	t.Helper()
	ids := transitiveSummaryIDs{
		first:     "example.com/paths::EntryA",
		second:    "example.com/paths::EntryB",
		third:     "example.com/paths::EntryC",
		shared:    "example.com/paths::Shared",
		alternate: "example.com/paths::Alternate",
		target:    "example.com/paths::Target",
	}
	g := graph.New()
	if err := addFixtureNode(g, fixtureNode{ID: "example.com/paths", Kind: graph.NodePackage, Name: "paths"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []graph.SymbolRef{ids.first, ids.second, ids.third, ids.shared, ids.alternate, ids.target} {
		if err := addFixtureNode(g, fixtureNode{ID: id, Kind: graph.NodeFunction, Name: string(id), Parent: "example.com/paths"}); err != nil {
			t.Fatal(err)
		}
	}
	edges := []fixtureEdge{
		{From: ids.first, To: ids.shared, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "paths.go", Offset: 1}}},
		{From: ids.second, To: ids.shared, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "paths.go", Offset: 2}}},
		{From: ids.shared, To: ids.target, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "paths.go", Offset: 3}}},
	}
	if multipleFamilies {
		edges[0].Certainty = graph.RelationshipUncertain
		edges = append(edges,
			fixtureEdge{From: ids.third, To: ids.alternate, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "paths.go", Offset: 4}}},
			fixtureEdge{
				From: ids.alternate, To: ids.target, Kind: graph.EdgeCalls, Certainty: graph.RelationshipUncertain,
				Evidence: []graph.Location{{File: "paths.go", Offset: 5}},
			},
		)
	}
	for _, edge := range edges {
		if err := addFixtureEdge(g, edge); err != nil {
			t.Fatal(err)
		}
	}
	return g, ids
}

func flattenedTransitivePaths(results []query.TransitiveDependent) []query.SemanticPath {
	var paths []query.SemanticPath
	for _, result := range results {
		paths = append(paths, result.Paths...)
	}
	return paths
}
