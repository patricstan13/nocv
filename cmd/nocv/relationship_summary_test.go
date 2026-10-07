package main

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"nocv/graph"
	"nocv/query"
)

func TestNodeInspectionGroupsDirectRelationshipsAndKeepsExactDrillDown(t *testing.T) {
	g := graph.New()
	factory := graph.SymbolRef("example.com/target::Factory")
	first := graph.SymbolRef("example.com/source::First")
	second := graph.SymbolRef("example.com/source::Second")
	returner := graph.SymbolRef("example.com/source::NewFactory")
	helper := graph.SymbolRef("example.com/target::Helper")
	for _, node := range []fixtureNode{
		{ID: "example.com/source", Kind: graph.NodePackage, Name: "source"},
		{ID: "example.com/target", Kind: graph.NodePackage, Name: "target"},
		{ID: factory, Kind: graph.NodeStruct, Name: "Factory", Parent: "example.com/target"},
		{ID: first, Kind: graph.NodeFunction, Name: "First", Parent: "example.com/source"},
		{ID: second, Kind: graph.NodeFunction, Name: "Second", Parent: "example.com/source"},
		{ID: returner, Kind: graph.NodeFunction, Name: "NewFactory", Parent: "example.com/source"},
		{ID: helper, Kind: graph.NodeFunction, Name: "Helper", Parent: "example.com/target"},
	} {
		if err := addFixtureNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	for _, edge := range []fixtureEdge{
		{
			From: first, To: factory, Kind: graph.EdgeAccepts,
			Evidence: []graph.Location{{File: "first.go", Offset: 1}, {File: "first.go", Offset: 9}},
		},
		{From: second, To: factory, Kind: graph.EdgeAccepts, Evidence: []graph.Location{{File: "second.go"}}},
		{From: returner, To: factory, Kind: graph.EdgeReturns, Evidence: []graph.Location{{File: "factory.go"}}},
		{From: first, To: helper, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "first.go", Offset: 12}}},
	} {
		if err := addFixtureEdge(g, edge); err != nil {
			t.Fatal(err)
		}
	}

	dependents := query.DirectDependents(g, factory)
	var inspection bytes.Buffer
	if err := executeCommand(&inspection, g, invocation{name: "node.inspect", pattern: "./...", values: []string{string(factory)}}); err != nil {
		t.Fatal(err)
	}
	inspectionOutput := inspection.String()
	for _, want := range []string{
		"Direct semantic dependents: 3",
		"accepts: 2",
		"returns: 1",
		"example: " + string(first),
		"example: " + string(returner),
		"Full list:",
		"nocv node dependents <pattern> " + string(factory),
	} {
		if !strings.Contains(inspectionOutput, want) {
			t.Errorf("node inspection lacks %q:\n%s", want, inspectionOutput)
		}
	}
	for _, evidenceFile := range []string{"first.go", "second.go", "factory.go"} {
		if strings.Contains(inspectionOutput, evidenceFile) {
			t.Errorf("node inspection dumped evidence file %q:\n%s", evidenceFile, inspectionOutput)
		}
	}
	groups := query.GroupRelationships(dependents)
	groupCount := 0
	for _, group := range groups {
		groupCount += group.Count
	}
	if groupCount != len(dependents) {
		t.Fatalf("rendered group count = %d, exact dependents = %d", groupCount, len(dependents))
	}

	var exactDependents bytes.Buffer
	if err := executeCommand(&exactDependents, g, invocation{name: "node.dependents", pattern: "./...", values: []string{string(factory)}}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []graph.SymbolRef{first, second, returner} {
		if !strings.Contains(exactDependents.String(), string(id)) {
			t.Errorf("exact dependents omit %s:\n%s", id, exactDependents.String())
		}
	}
	if got := len(query.DirectDependents(g, factory)); got != 3 {
		t.Fatalf("exact dependents count = %d, want 3", got)
	}
	if got := len(query.DirectDependents(g, factory)[0].Evidence); got != 2 {
		t.Fatalf("exact evidence count = %d, want 2", got)
	}

	dependencies := query.DirectDependencies(g, first)
	var dependencyInspection bytes.Buffer
	if err := executeCommand(&dependencyInspection, g, invocation{name: "node.inspect", pattern: "./...", values: []string{string(first)}}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Direct semantic dependencies: 2",
		"accepts: 1",
		"calls: 1",
		"nocv node dependencies <pattern> " + string(first),
	} {
		if !strings.Contains(dependencyInspection.String(), want) {
			t.Errorf("dependency inspection lacks %q:\n%s", want, dependencyInspection.String())
		}
	}
	var exactDependencies bytes.Buffer
	if err := executeCommand(&exactDependencies, g, invocation{name: "node.dependencies", pattern: "./...", values: []string{string(first)}}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(exactDependencies.String(), " -> "); got != len(dependencies) {
		t.Fatalf("exact dependency lines = %d, relationships = %d:\n%s", got, len(dependencies), exactDependencies.String())
	}
}

func TestNodeInspectionSeparatesRelationshipCertaintyGroups(t *testing.T) {
	g := graph.New()
	contract := graph.SymbolRef("example.com/target::Contract")
	if err := addFixtureNode(g, fixtureNode{ID: "example.com/source", Kind: graph.NodePackage, Name: "source"}); err != nil {
		t.Fatal(err)
	}
	if err := addFixtureNode(g, fixtureNode{ID: "example.com/target", Kind: graph.NodePackage, Name: "target"}); err != nil {
		t.Fatal(err)
	}
	if err := addFixtureNode(g, fixtureNode{ID: contract, Kind: graph.NodeInterface, Name: "Contract", Parent: "example.com/target"}); err != nil {
		t.Fatal(err)
	}
	for index := range 5 {
		implementation := graph.SymbolRef("example.com/source::Implementation" + strconv.Itoa(index))
		if err := addFixtureNode(g, fixtureNode{ID: implementation, Kind: graph.NodeStruct, Name: "Implementation", Parent: "example.com/source"}); err != nil {
			t.Fatal(err)
		}
		certainty := graph.RelationshipConfirmed
		if index >= 3 {
			certainty = graph.RelationshipUncertain
		}
		if err := addFixtureEdge(g, fixtureEdge{
			From: implementation, To: contract, Kind: graph.EdgeImplements, Certainty: certainty,
			Evidence: []graph.Location{{File: "implementation.go", Offset: index}},
		}); err != nil {
			t.Fatal(err)
		}
	}

	var output bytes.Buffer
	if err := executeCommand(&output, g, invocation{name: "node.inspect", pattern: "./...", values: []string{string(contract)}}); err != nil {
		t.Fatal(err)
	}
	want := []string{"Direct semantic dependents: 5", "implements: 3", "implements [uncertain]: 2"}
	for _, text := range want {
		if !strings.Contains(output.String(), text) {
			t.Errorf("mixed-certainty inspection lacks %q:\n%s", text, output.String())
		}
	}
	if strings.Contains(output.String(), "[confirmed]") || strings.Count(output.String(), "example:") != 2 {
		t.Errorf("mixed-certainty presentation is misleading:\n%s", output.String())
	}
}
