package query_test

import (
	"context"
	"reflect"
	"testing"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/internal/testutil"
	"nocv/query"
)

func TestDirectDependenciesPreserveKindsEvidenceAndOrder(t *testing.T) {
	g, ids := navigationFixture(t)
	before := outgoingSnapshot(g)

	want := []query.Relationship{
		{From: ids.create, To: ids.next, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 50}}},
		{From: ids.create, To: ids.order, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 30}, {File: "project.go", Offset: 31}}},
		{From: ids.create, To: ids.repository, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 20}}},
		{From: ids.create, To: ids.repository, Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 10}}},
	}
	got := query.DirectDependencies(g, ids.create)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DirectDependencies(Create) = %#v, want %#v", got, want)
	}

	// Mutating the presentation result cannot mutate stored graph evidence.
	got[0].Evidence[0].Offset = 999
	if after := outgoingSnapshot(g); !reflect.DeepEqual(after, before) {
		t.Fatalf("direct navigation mutated graph edges:\nbefore: %#v\nafter:  %#v", before, after)
	}
	if again := query.DirectDependencies(g, ids.create); !reflect.DeepEqual(again, want) {
		t.Fatalf("second DirectDependencies(Create) = %#v, want %#v", again, want)
	}
}

func TestDirectDependenciesCoverImplementsAndEmbeds(t *testing.T) {
	g, ids := navigationFixture(t)
	if got, want := query.DirectDependencies(g, ids.service), []query.Relationship{
		{From: ids.service, To: ids.repository, Kind: graph.EdgeImplements, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 70}}},
		{From: ids.service, To: ids.base, Kind: graph.EdgeEmbeds, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 60}}},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DirectDependencies(Service) = %#v, want %#v", got, want)
	}
	if got, want := query.DirectDependencies(g, ids.concreteSave), []query.Relationship{
		{From: ids.concreteSave, To: ids.interfaceSave, Kind: graph.EdgeImplements, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 80}}},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DirectDependencies(Service.Save) = %#v, want %#v", got, want)
	}
}

func TestDirectDependentsCoverEverySupportedRelationshipKind(t *testing.T) {
	g, ids := navigationFixture(t)
	tests := []struct {
		name string
		id   graph.SymbolRef
		want []query.Relationship
	}{
		{
			name: "calls",
			id:   ids.next,
			want: []query.Relationship{{From: ids.create, To: ids.next, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 50}}}},
		},
		{
			name: "accepts evidence",
			id:   ids.order,
			want: []query.Relationship{{From: ids.create, To: ids.order, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 30}, {File: "project.go", Offset: 31}}}},
		},
		{
			name: "same source and target retain different kinds",
			id:   ids.repository,
			want: []query.Relationship{
				{From: ids.service, To: ids.repository, Kind: graph.EdgeImplements, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 70}}},
				{From: ids.create, To: ids.repository, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 20}}},
				{From: ids.create, To: ids.repository, Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 10}}},
			},
		},
		{
			name: "embeds",
			id:   ids.base,
			want: []query.Relationship{{From: ids.service, To: ids.base, Kind: graph.EdgeEmbeds, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 60}}}},
		},
		{
			name: "method implements",
			id:   ids.interfaceSave,
			want: []query.Relationship{{From: ids.concreteSave, To: ids.interfaceSave, Kind: graph.EdgeImplements, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 80}}}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := query.DirectDependents(g, test.id); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("DirectDependents(%q) = %#v, want %#v", test.id, got, test.want)
			}
		})
	}
}

func TestDirectNavigationIsNeitherTransitiveStructuralNorProjected(t *testing.T) {
	g, ids := navigationFixture(t)

	dependencies := query.DirectDependencies(g, ids.create)
	for _, relationship := range dependencies {
		if relationship.To == ids.final {
			t.Fatalf("transitive target %q appeared in direct dependencies: %#v", ids.final, dependencies)
		}
		if relationship.To == ids.service {
			t.Fatalf("structural parent %q appeared in direct dependencies: %#v", ids.service, dependencies)
		}
	}
	dependents := query.DirectDependents(g, ids.final)
	if len(dependents) != 1 || dependents[0].From != ids.next {
		t.Fatalf("DirectDependents(Final) = %#v, want only Next", dependents)
	}
	if got := query.DirectDependencies(g, ids.pkg); len(got) != 0 {
		t.Fatalf("package acquired invented projected dependencies: %#v", got)
	}
}

func TestDirectNavigationHandlesNilAndMissingNodes(t *testing.T) {
	g, _ := navigationFixture(t)
	for _, relationships := range [][]query.Relationship{
		query.DirectDependencies(nil, "missing"),
		query.DirectDependents(nil, "missing"),
		query.DirectDependencies(g, "missing"),
		query.DirectDependents(g, "missing"),
	} {
		if len(relationships) != 0 {
			t.Fatalf("missing or nil navigation = %#v, want empty", relationships)
		}
	}
}

func TestAnalyzerPipelineProvidesDirectNavigation(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), testutil.GoProjectDir(t), "./app", "./logging", "./orders", "./repository", "./service")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	transformID := graph.SymbolRef("example.com/shop/orders::Service::Transform")
	wantDependencies := []query.Relationship{
		{From: transformID, To: "example.com/shop/orders::Order", Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed},
		{From: transformID, To: "example.com/shop/orders::Repository", Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed},
		{From: transformID, To: "example.com/shop/orders::Repository", Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed},
	}
	gotDependencies := query.DirectDependencies(g, transformID)
	if len(gotDependencies) != len(wantDependencies) {
		t.Fatalf("DirectDependencies(Service.Transform) = %#v, want three", gotDependencies)
	}
	for index, want := range wantDependencies {
		got := gotDependencies[index]
		if got.From != want.From || got.To != want.To || got.Kind != want.Kind || len(got.Evidence) != 1 {
			t.Errorf("dependency %d = %#v, want endpoints/kind %#v with one evidence location", index, got, want)
		}
	}

	repositoryID := graph.SymbolRef("example.com/shop/orders::Repository")
	dependents := query.DirectDependents(g, repositoryID)
	wantRelationships := map[relationshipKey]bool{
		{from: "example.com/shop/orders::PostgresRepository", kind: graph.EdgeImplements}: true,
		{from: "example.com/shop/orders::Service::Transform", kind: graph.EdgeAccepts}:    true,
		{from: "example.com/shop/orders::Processor::Process", kind: graph.EdgeAccepts}:    true,
		{from: "example.com/shop/orders::LoadPair", kind: graph.EdgeReturns}:              true,
		{from: "example.com/shop/orders::Service::Transform", kind: graph.EdgeReturns}:    true,
	}
	for key := range wantRelationships {
		found := false
		for _, relationship := range dependents {
			if relationship.From == key.from && relationship.Kind == key.kind && relationship.To == repositoryID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing Repository dependent %#v; got %#v", key, dependents)
		}
	}
}

type navigationIDs struct {
	pkg           graph.SymbolRef
	service       graph.SymbolRef
	base          graph.SymbolRef
	order         graph.SymbolRef
	repository    graph.SymbolRef
	create        graph.SymbolRef
	next          graph.SymbolRef
	final         graph.SymbolRef
	concreteSave  graph.SymbolRef
	interfaceSave graph.SymbolRef
}

type relationshipKey struct {
	from graph.SymbolRef
	kind graph.EdgeKind
}

func navigationFixture(t *testing.T) (*graph.Graph, navigationIDs) {
	t.Helper()
	ids := navigationIDs{
		pkg:           "example.com/project",
		service:       "example.com/project::Service",
		base:          "example.com/project::Base",
		order:         "example.com/project::Order",
		repository:    "example.com/project::Repository",
		create:        "example.com/project::Service::Create",
		next:          "example.com/project::Next",
		final:         "example.com/project::Final",
		concreteSave:  "example.com/project::Service::Save",
		interfaceSave: "example.com/project::Repository::Save",
	}
	g := graph.New()
	for _, node := range []testNode{
		{ID: ids.pkg, Kind: graph.NodePackage, Name: "project"},
		{ID: ids.service, Kind: graph.NodeStruct, Name: "Service", Parent: ids.pkg},
		{ID: ids.base, Kind: graph.NodeStruct, Name: "Base", Parent: ids.pkg},
		{ID: ids.order, Kind: graph.NodeStruct, Name: "Order", Parent: ids.pkg},
		{ID: ids.repository, Kind: graph.NodeInterface, Name: "Repository", Parent: ids.pkg},
		{ID: ids.create, Kind: graph.NodeFunction, Name: "Create", Parent: ids.service},
		{ID: ids.next, Kind: graph.NodeFunction, Name: "Next", Parent: ids.pkg},
		{ID: ids.final, Kind: graph.NodeFunction, Name: "Final", Parent: ids.pkg},
		{ID: ids.concreteSave, Kind: graph.NodeFunction, Name: "Save", Parent: ids.service},
		{ID: ids.interfaceSave, Kind: graph.NodeFunction, Name: "Save", Parent: ids.repository},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	edges := []testEdge{
		{From: ids.create, To: ids.repository, Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 10}}},
		{From: ids.create, To: ids.repository, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 20}}},
		{From: ids.create, To: ids.order, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 30}}},
		{From: ids.create, To: ids.order, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 31}}},
		{From: ids.next, To: ids.final, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 40}}},
		{From: ids.create, To: ids.next, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 50}}},
		{From: ids.service, To: ids.base, Kind: graph.EdgeEmbeds, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 60}}},
		{From: ids.service, To: ids.repository, Kind: graph.EdgeImplements, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 70}}},
		{From: ids.concreteSave, To: ids.interfaceSave, Kind: graph.EdgeImplements, Certainty: graph.RelationshipConfirmed, Evidence: []graph.Location{{File: "project.go", Offset: 80}}},
	}
	for _, edge := range edges {
		if err := addTestEdge(g, edge); err != nil {
			t.Fatalf("AddEdge(%s, %q -> %q): %v", edge.Kind, edge.From, edge.To, err)
		}
	}
	return g, ids
}
