package main

import (
	"bytes"
	"strings"
	"testing"

	"nocv/graph"
	"nocv/query"
)

func TestPackageDependencyInspectionSummarizesAllPathFamiliesAndKeepsExactDrillDown(t *testing.T) {
	g, from, to := packagePathSummaryFixture(t)
	paths := query.PackageDependencyPaths(g, from, to)
	level := query.GroupPackagePathBranches(paths, nil, 0)
	if level.PathCount != 6 || level.UncertainPathCount != 4 || len(level.TerminalPathIndexes) != 0 || len(level.Branches) != 3 {
		t.Fatalf("package path level = %#v, want six paths, four uncertain, no terminals, and three families", level)
	}
	accounted := len(level.TerminalPathIndexes)
	for _, branch := range level.Branches {
		accounted += branch.PathCount
	}
	if accounted != len(paths) {
		t.Fatalf("displayed families account for %d paths, exact query has %d", accounted, len(paths))
	}

	var inspection bytes.Buffer
	if err := executeCommand(&inspection, g, invocation{
		name: "go.package.inspect-dependency", pattern: "./...", values: []string{string(from), string(to)},
	}); err != nil {
		t.Fatal(err)
	}
	output := inspection.String()
	for _, want := range []string{
		"Direct dependency:",
		"example.com/summary/a::Direct calls -> example.com/summary/e::FromA",
		"Package paths: 6",
		"uncertain paths: 4",
		string(from) + " -> example.com/summary/b\n    paths: 2\n    uncertain paths: 1",
		string(from) + " -> [uncertain] example.com/summary/c\n    paths: 3\n    uncertain paths: 3",
		string(from) + " -> " + string(to) + "\n    paths: 1",
		"Full paths:",
		"nocv go package dependency-paths ./... " + string(from) + " " + string(to),
	} {
		if !strings.Contains(output, want) {
			t.Errorf("package dependency inspection lacks %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "[confirmed]") || strings.Contains(output, "terminal paths:") {
		t.Errorf("confirmed or zero-terminal noise appeared in inspection:\n%s", output)
	}

	summary := output[strings.Index(output, "Package paths:"):]
	bIndex := strings.Index(summary, string(from)+" -> example.com/summary/b")
	cIndex := strings.Index(summary, string(from)+" -> [uncertain] example.com/summary/c")
	eIndex := strings.Index(summary, string(from)+" -> "+string(to))
	if bIndex < 0 || cIndex < 0 || eIndex < 0 || !(bIndex < cIndex && cIndex < eIndex) {
		t.Errorf("families are not in query semantic order:\n%s", output)
	}

	for _, unwanted := range []string{"evidence:", "summary/a::ToCOne", "summary/a::ToCTwo", "path 1:"} {
		if strings.Contains(summary, unwanted) {
			t.Errorf("family summary dumped exact path or evidence %q:\n%s", unwanted, summary)
		}
	}

	var exact bytes.Buffer
	if err := executeCommand(&exact, g, invocation{
		name: "go.package.dependency-paths", pattern: "./...", values: []string{string(from), string(to)},
	}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(exact.String(), "path "); got != len(paths) {
		t.Fatalf("exact command rendered %d paths, want %d:\n%s", got, len(paths), exact.String())
	}
	for _, want := range []string{"example.com/summary/a::ToCOne", "example.com/summary/a::ToCTwo", "evidence:"} {
		if !strings.Contains(exact.String(), want) {
			t.Errorf("exact command omits %q:\n%s", want, exact.String())
		}
	}
}

func TestPrintPackagePathSummaryAccountsForTerminalsAndKeepsConfirmedOutputQuiet(t *testing.T) {
	confirmed := []query.PackageDependencyPath{
		{Steps: []query.PackageDependency{{From: "A", To: "B", Certainty: graph.RelationshipConfirmed}}},
		{Steps: []query.PackageDependency{{From: "A", To: "C", Certainty: graph.RelationshipConfirmed}}},
	}
	var confirmedOutput bytes.Buffer
	printPackagePathSummary(&confirmedOutput, confirmed, "./...", "A", "C")
	if strings.Contains(confirmedOutput.String(), "uncertain paths:") || strings.Contains(confirmedOutput.String(), "[confirmed]") {
		t.Fatalf("all-confirmed summary is noisy:\n%s", confirmedOutput.String())
	}

	withTerminal := append([]query.PackageDependencyPath{{}}, confirmed...)
	var terminalOutput bytes.Buffer
	printPackagePathSummary(&terminalOutput, withTerminal, "./...", "A", "C")
	for _, want := range []string{"Package paths: 3", "terminal paths: 1", "A -> B", "A -> C"} {
		if !strings.Contains(terminalOutput.String(), want) {
			t.Errorf("terminal summary lacks %q:\n%s", want, terminalOutput.String())
		}
	}
}

func TestPrintPackagePathSummarySeparatesBranchStepCertainty(t *testing.T) {
	paths := []query.PackageDependencyPath{
		{Steps: []query.PackageDependency{{From: "A", To: "B", Certainty: graph.RelationshipConfirmed}}},
		{Steps: []query.PackageDependency{{From: "A", To: "B", Certainty: graph.RelationshipUncertain}}},
	}
	var out bytes.Buffer
	printPackagePathSummary(&out, paths, "./...", "A", "B")
	output := out.String()
	if strings.Count(output, "  A ->") != 2 || !strings.Contains(output, "A -> B") || !strings.Contains(output, "A -> [uncertain] B") {
		t.Fatalf("confirmed and uncertain branch steps were not rendered separately:\n%s", output)
	}
}

func packagePathSummaryFixture(t *testing.T) (*graph.Graph, graph.SymbolRef, graph.SymbolRef) {
	t.Helper()
	packages := []graph.SymbolRef{
		"example.com/summary/a",
		"example.com/summary/b",
		"example.com/summary/c",
		"example.com/summary/d",
		"example.com/summary/e",
		"example.com/summary/f",
	}
	g := graph.New()
	for _, packageID := range packages {
		if err := addFixtureNode(g, fixtureNode{ID: packageID, Kind: graph.NodePackage, Name: string(packageID)}); err != nil {
			t.Fatal(err)
		}
	}

	edges := []fixtureEdge{
		packageBoundary("ToB", packages[0], "FromA", packages[1], graph.EdgeCalls, graph.RelationshipConfirmed, 1),
		packageBoundary("ToE", packages[1], "FromB", packages[4], graph.EdgeCalls, graph.RelationshipConfirmed, 2),
		packageBoundary("ToD", packages[1], "FromB", packages[3], graph.EdgeCalls, graph.RelationshipUncertain, 3),
		packageBoundary("ToE", packages[3], "FromD", packages[4], graph.EdgeCalls, graph.RelationshipConfirmed, 4),
		packageBoundary("ToCOne", packages[0], "FromAOne", packages[2], graph.EdgeCalls, graph.RelationshipUncertain, 5),
		packageBoundary("ToCTwo", packages[0], "FromATwo", packages[2], graph.EdgeCalls, graph.RelationshipUncertain, 6),
		packageBoundary("ToE", packages[2], "FromC", packages[4], graph.EdgeCalls, graph.RelationshipConfirmed, 7),
		packageBoundary("ToD", packages[2], "FromC", packages[3], graph.EdgeCalls, graph.RelationshipConfirmed, 8),
		packageBoundary("ToF", packages[2], "FromC", packages[5], graph.EdgeCalls, graph.RelationshipConfirmed, 9),
		packageBoundary("ToE", packages[5], "FromF", packages[4], graph.EdgeCalls, graph.RelationshipConfirmed, 10),
		packageBoundary("Direct", packages[0], "FromA", packages[4], graph.EdgeCalls, graph.RelationshipConfirmed, 11),
	}
	for _, edge := range edges {
		for _, node := range []fixtureNode{
			{ID: edge.From, Kind: graph.NodeFunction, Name: string(edge.From), Parent: parentPackage(edge.From, packages)},
			{ID: edge.To, Kind: graph.NodeFunction, Name: string(edge.To), Parent: parentPackage(edge.To, packages)},
		} {
			if _, exists := g.NodeByRef(node.ID); !exists {
				if err := addFixtureNode(g, node); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	for _, edge := range edges {
		if err := addFixtureEdge(g, edge); err != nil {
			t.Fatal(err)
		}
	}
	return g, packages[0], packages[4]
}

func packageBoundary(
	fromName string,
	fromPackage graph.SymbolRef,
	toName string,
	toPackage graph.SymbolRef,
	kind graph.EdgeKind,
	certainty graph.RelationshipCertainty,
	offset int,
) fixtureEdge {
	return fixtureEdge{
		From: fromPackage + graph.SymbolRef("::"+fromName),
		To:   toPackage + graph.SymbolRef("::"+toName),
		Kind: kind, Certainty: certainty,
		Evidence: []graph.Location{{File: "summary.go", Offset: offset}},
	}
}

func parentPackage(id graph.SymbolRef, packages []graph.SymbolRef) graph.SymbolRef {
	for _, packageID := range packages {
		if strings.HasPrefix(string(id), string(packageID)+"::") {
			return packageID
		}
	}
	return ""
}
