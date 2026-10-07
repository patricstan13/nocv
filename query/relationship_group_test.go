package query_test

import (
	"reflect"
	"slices"
	"testing"

	"nocv/graph"
	"nocv/query"
)

func TestGroupRelationshipsGroupsCountsOrdersAndCopies(t *testing.T) {
	relationships := []query.Relationship{
		{
			From: "z::Use", To: "target::Contract", Kind: graph.EdgeImplements,
			Certainty: graph.RelationshipUncertain,
			Evidence:  []graph.Location{{File: "z.go", Offset: 4}},
		},
		{
			From: "b::Use", To: "target::Contract", Kind: graph.EdgeAccepts,
			Certainty: graph.RelationshipConfirmed,
			Evidence: []graph.Location{
				{File: "b.go", Offset: 7},
				{File: "b.go", Offset: 12},
			},
		},
		{
			From: "a::Use", To: "target::Contract", Kind: graph.EdgeAccepts,
			Certainty: graph.RelationshipConfirmed,
			Evidence:  []graph.Location{{File: "a.go", Offset: 3}},
		},
		{
			From: "c::Use", To: "target::Contract", Kind: graph.EdgeImplements,
			Certainty: graph.RelationshipConfirmed,
			Evidence:  []graph.Location{{File: "c.go", Offset: 8}},
		},
	}

	got := query.GroupRelationships(relationships)
	if len(got) != 3 {
		t.Fatalf("GroupRelationships() = %#v, want three groups", got)
	}
	wantKeys := []struct {
		kind      graph.EdgeKind
		certainty graph.RelationshipCertainty
		count     int
	}{
		{graph.EdgeAccepts, graph.RelationshipConfirmed, 2},
		{graph.EdgeImplements, graph.RelationshipConfirmed, 1},
		{graph.EdgeImplements, graph.RelationshipUncertain, 1},
	}
	for index, want := range wantKeys {
		if got[index].Kind != want.kind || got[index].Certainty != want.certainty || got[index].Count != want.count {
			t.Errorf("group %d = %#v, want kind %s, certainty %s, count %d", index, got[index], want.kind, want.certainty, want.count)
		}
	}
	if got[0].Example.From != "a::Use" {
		t.Errorf("accepts example = %#v, want lexically first relationship", got[0].Example)
	}

	reversed := slices.Clone(relationships)
	slices.Reverse(reversed)
	if reverseGroups := query.GroupRelationships(reversed); !reflect.DeepEqual(reverseGroups, got) {
		t.Errorf("groups depend on input order:\nforward: %#v\nreverse: %#v", got, reverseGroups)
	}

	got[0].Example.Evidence[0].Offset = 999
	if relationships[2].Evidence[0].Offset != 3 {
		t.Errorf("mutating example evidence changed input: %#v", relationships[2])
	}
}

func TestGroupRelationshipsUsesEvidenceToBreakExampleTies(t *testing.T) {
	relationships := []query.Relationship{
		{
			From: "a::Use", To: "b::Target", Kind: graph.EdgeCalls,
			Certainty: graph.RelationshipConfirmed,
			Evidence:  []graph.Location{{File: "later.go", Offset: 1}},
		},
		{
			From: "a::Use", To: "b::Target", Kind: graph.EdgeCalls,
			Certainty: graph.RelationshipConfirmed,
			Evidence:  []graph.Location{{File: "earlier.go", Offset: 2}},
		},
	}

	forward := query.GroupRelationships(relationships)
	slices.Reverse(relationships)
	reverse := query.GroupRelationships(relationships)
	if !reflect.DeepEqual(forward, reverse) {
		t.Fatalf("tied example depends on input order:\nforward: %#v\nreverse: %#v", forward, reverse)
	}
	if got := forward[0].Example.Evidence[0].File; got != "earlier.go" {
		t.Errorf("example evidence file = %q, want earlier.go", got)
	}
}

func TestGroupRelationshipsCountsRelationshipsNotEvidence(t *testing.T) {
	groups := query.GroupRelationships([]query.Relationship{{
		From: "a::Use", To: "b::Target", Kind: graph.EdgeAccepts,
		Certainty: graph.RelationshipConfirmed,
		Evidence: []graph.Location{
			{File: "use.go", Offset: 1},
			{File: "use.go", Offset: 5},
			{File: "use.go", Offset: 9},
		},
	}})
	if len(groups) != 1 || groups[0].Count != 1 {
		t.Fatalf("GroupRelationships() = %#v, want one relationship despite three evidence locations", groups)
	}
}

func TestGroupRelationshipsEmptyInput(t *testing.T) {
	if got := query.GroupRelationships(nil); got != nil {
		t.Errorf("GroupRelationships(nil) = %#v, want nil", got)
	}
	if got := query.GroupRelationships([]query.Relationship{}); got != nil {
		t.Errorf("GroupRelationships(empty) = %#v, want nil", got)
	}
}

func TestGroupRelationshipsSummarizesDirectDependents(t *testing.T) {
	g := graph.New()
	for _, node := range []testNode{
		{ID: "source", Kind: graph.NodePackage, Name: "source"},
		{ID: "target", Kind: graph.NodePackage, Name: "target"},
		{ID: "source::First", Kind: graph.NodeFunction, Name: "First", Parent: "source"},
		{ID: "source::Second", Kind: graph.NodeFunction, Name: "Second", Parent: "source"},
		{ID: "source::Concrete", Kind: graph.NodeStruct, Name: "Concrete", Parent: "source"},
		{ID: "target::Contract", Kind: graph.NodeInterface, Name: "Contract", Parent: "target"},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	for _, edge := range []testEdge{
		{From: "source::First", To: "target::Contract", Kind: graph.EdgeAccepts, Evidence: []graph.Location{{File: "first.go"}}},
		{From: "source::Second", To: "target::Contract", Kind: graph.EdgeAccepts, Evidence: []graph.Location{{File: "second.go"}}},
		{From: "source::Concrete", To: "target::Contract", Kind: graph.EdgeImplements, Evidence: []graph.Location{{File: "concrete.go"}}},
	} {
		if err := addTestEdge(g, edge); err != nil {
			t.Fatal(err)
		}
	}

	groups := query.GroupRelationships(query.DirectDependents(g, "target::Contract"))
	if len(groups) != 2 || groups[0].Kind != graph.EdgeAccepts || groups[0].Count != 2 || groups[1].Kind != graph.EdgeImplements || groups[1].Count != 1 {
		t.Fatalf("grouped direct dependents = %#v, want two accepts and one implements", groups)
	}
}

func TestGroupRelationshipsSummarizesProjectedPackageEvidence(t *testing.T) {
	g := graph.New()
	for _, node := range []testNode{
		{ID: "source", Kind: graph.NodePackage, Name: "source"},
		{ID: "target", Kind: graph.NodePackage, Name: "target"},
		{ID: "source::Call", Kind: graph.NodeFunction, Name: "Call", Parent: "source"},
		{ID: "source::Concrete", Kind: graph.NodeStruct, Name: "Concrete", Parent: "source"},
		{ID: "target::Run", Kind: graph.NodeFunction, Name: "Run", Parent: "target"},
		{ID: "target::Contract", Kind: graph.NodeInterface, Name: "Contract", Parent: "target"},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	for _, edge := range []testEdge{
		{
			From: "source::Call", To: "target::Run", Kind: graph.EdgeCalls,
			Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "call.go"}},
		},
		{
			From: "source::Concrete", To: "target::Contract", Kind: graph.EdgeImplements,
			Certainty: graph.RelationshipUncertain, Evidence: []graph.Location{{File: "concrete.go"}},
		},
	} {
		if err := addTestEdge(g, edge); err != nil {
			t.Fatal(err)
		}
	}

	dependencies := query.DirectPackageDependencies(g, "source")
	if len(dependencies) != 1 {
		t.Fatalf("DirectPackageDependencies(source) = %#v, want one dependency", dependencies)
	}
	if dependencies[0].Certainty != graph.RelationshipConfirmed {
		t.Fatalf("projected dependency certainty = %s, want confirmed", dependencies[0].Certainty)
	}
	groups := query.GroupRelationships(dependencies[0].Evidence)
	if len(groups) != 2 || groups[0].Kind != graph.EdgeCalls || groups[0].Certainty != graph.RelationshipConfirmed || groups[1].Kind != graph.EdgeImplements || groups[1].Certainty != graph.RelationshipUncertain {
		t.Fatalf("grouped package evidence = %#v, want confirmed calls and uncertain implements", groups)
	}
}
