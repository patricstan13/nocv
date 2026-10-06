package query_test

import (
	"testing"

	"nocv/graph"
	"nocv/query"
)

func TestCertaintySurvivesNavigationPathsAndProjections(t *testing.T) {
	g := graph.New()
	for _, node := range []testNode{
		{ID: "example.com/source", Kind: graph.NodePackage, Name: "source"},
		{ID: "example.com/target", Kind: graph.NodePackage, Name: "target"},
		{ID: "example.com/source::Concrete", Kind: graph.NodeStruct, Name: "Concrete", Parent: "example.com/source"},
		{ID: "example.com/source::Concrete::Run", Kind: graph.NodeFunction, Name: "Run", Parent: "example.com/source::Concrete"},
		{ID: "example.com/target::Contract", Kind: graph.NodeInterface, Name: "Contract", Parent: "example.com/target"},
		{ID: "example.com/target::Contract::Run", Kind: graph.NodeFunction, Name: "Run", Parent: "example.com/target::Contract"},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatal(err)
		}
	}

	uncertain := testEdge{
		From: "example.com/source::Concrete", To: "example.com/target::Contract",
		Kind: graph.EdgeImplements, Certainty: graph.RelationshipUncertain,
		Evidence: []graph.Location{{File: "source.go", Offset: 10}},
	}
	if err := addTestEdge(g, uncertain); err != nil {
		t.Fatal(err)
	}

	direct := query.DirectDependencies(g, uncertain.From)
	if len(direct) != 1 || direct[0].Certainty != graph.RelationshipUncertain {
		t.Fatalf("direct relationships = %#v, want one uncertain relationship", direct)
	}
	inspection, ok := query.InspectNode(g, uncertain.From)
	if !ok || inspection.Type == nil || len(inspection.Type.DirectDependencies) != 1 ||
		inspection.Type.DirectDependencies[0].Certainty != graph.RelationshipUncertain {
		t.Fatalf("node inspection lost certainty: %#v", inspection)
	}

	paths := query.DependencyPaths(g, uncertain.From, uncertain.To)
	if len(paths) != 1 || len(paths[0].Steps) != 1 ||
		paths[0].Steps[0].Certainty != graph.RelationshipUncertain {
		t.Fatalf("exact paths = %#v, want uncertain step", paths)
	}
	dependents := query.TransitiveDependents(g, uncertain.To)
	if len(dependents) != 1 || len(dependents[0].Paths) != 1 ||
		dependents[0].Paths[0].Steps[0].Certainty != graph.RelationshipUncertain {
		t.Fatalf("transitive dependents = %#v, want uncertain step", dependents)
	}

	assertProjectionCertainty(t, query.DirectPackageDependencies(g, "example.com/source"), graph.RelationshipUncertain, []graph.RelationshipCertainty{graph.RelationshipUncertain})
	assertTypeProjectionCertainty(t, query.DirectTypeDependencies(g, uncertain.From), graph.RelationshipUncertain, []graph.RelationshipCertainty{graph.RelationshipUncertain})

	confirmed := testEdge{
		From: "example.com/source::Concrete::Run", To: "example.com/target::Contract::Run",
		Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed,
		Evidence: []graph.Location{{File: "source.go", Offset: 20}},
	}
	if err := addTestEdge(g, confirmed); err != nil {
		t.Fatal(err)
	}

	packages := query.DirectPackageDependencies(g, "example.com/source")
	assertProjectionCertainty(t, packages, graph.RelationshipConfirmed, []graph.RelationshipCertainty{
		graph.RelationshipUncertain,
		graph.RelationshipConfirmed,
	})
	types := query.DirectTypeDependencies(g, uncertain.From)
	assertTypeProjectionCertainty(t, types, graph.RelationshipConfirmed, []graph.RelationshipCertainty{
		graph.RelationshipUncertain,
		graph.RelationshipConfirmed,
	})

	packageInspection := query.InspectPackageDependency(g, packages[0])
	if len(packageInspection.TypeDependencies) != 1 || len(packageInspection.ExactOnly) != 0 ||
		packageInspection.TypeDependencies[0].Certainty != graph.RelationshipConfirmed ||
		len(packageInspection.TypeDependencies[0].Evidence) != 2 {
		t.Fatalf("package inspection lost or duplicated certainty evidence: %#v", packageInspection)
	}
}

func assertProjectionCertainty(
	t *testing.T,
	dependencies []query.PackageDependency,
	want graph.RelationshipCertainty,
	wantEvidence []graph.RelationshipCertainty,
) {
	t.Helper()
	if len(dependencies) != 1 || dependencies[0].Certainty != want {
		t.Fatalf("package dependencies = %#v, want one %s dependency", dependencies, want)
	}
	assertEvidenceCertainties(t, dependencies[0].Evidence, wantEvidence)
}

func assertTypeProjectionCertainty(
	t *testing.T,
	dependencies []query.TypeDependency,
	want graph.RelationshipCertainty,
	wantEvidence []graph.RelationshipCertainty,
) {
	t.Helper()
	if len(dependencies) != 1 || dependencies[0].Certainty != want {
		t.Fatalf("type dependencies = %#v, want one %s dependency", dependencies, want)
	}
	assertEvidenceCertainties(t, dependencies[0].Evidence, wantEvidence)
}

func assertEvidenceCertainties(
	t *testing.T,
	evidence []query.Relationship,
	want []graph.RelationshipCertainty,
) {
	t.Helper()
	if len(evidence) != len(want) {
		t.Fatalf("evidence = %#v, want %d relationships", evidence, len(want))
	}
	got := make(map[graph.RelationshipCertainty]int)
	for _, relationship := range evidence {
		got[relationship.Certainty]++
	}
	for _, certainty := range want {
		got[certainty]--
	}
	for _, count := range got {
		if count != 0 {
			t.Fatalf("evidence certainties = %#v, want %#v", evidence, want)
		}
	}
}
