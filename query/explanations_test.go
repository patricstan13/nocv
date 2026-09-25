package query_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/query"
)

func TestWhyDependsOnExplainsTypeDependency(t *testing.T) {
	g, ids := dependencyFixture(t)
	beforeEdges := outgoingSnapshot(g)
	beforeDependencies := query.Dependencies(g, graph.NodeStruct, graph.NodeInterface)

	explanation, ok := query.WhyDependsOn(g, ids.service, ids.repository)
	if !ok {
		t.Fatal("WhyDependsOn(Service, Repository) returned no explanation")
	}
	if explanation.From != ids.service || explanation.To != ids.repository {
		t.Fatalf("explanation = %q -> %q", explanation.From, explanation.To)
	}
	if len(explanation.Evidence) != 3 {
		t.Fatalf("evidence = %#v, want three underlying calls", explanation.Evidence)
	}
	for _, evidence := range explanation.Evidence {
		if len(evidence.Locations) != 1 || evidence.Locations[0].File != "orders.go" || evidence.Locations[0].Offset <= 0 {
			t.Errorf("evidence locations for %q -> %q = %#v", evidence.From, evidence.To, evidence.Locations)
		}
	}

	if after := outgoingSnapshot(g); !reflect.DeepEqual(after, beforeEdges) {
		t.Fatalf("explanation mutated graph edges:\nbefore: %#v\nafter:  %#v", beforeEdges, after)
	}
	if after := query.Dependencies(g, graph.NodeStruct, graph.NodeInterface); !reflect.DeepEqual(after, beforeDependencies) {
		t.Fatalf("explanation changed dependency projection:\nbefore: %#v\nafter:  %#v", beforeDependencies, after)
	}
}

func TestWhyDependsOnExplainsPackageDependency(t *testing.T) {
	g, ids := dependencyFixture(t)
	explanation, ok := query.WhyDependsOn(g, ids.orders, ids.logging)
	if !ok {
		t.Fatal("WhyDependsOn(orders, logging) returned no explanation")
	}
	if len(explanation.Evidence) != 2 {
		t.Fatalf("package explanation evidence = %#v, want two calls", explanation.Evidence)
	}
}

func TestWhyDependsOnRejectsSuppressedMissingAndUnsupportedEndpoints(t *testing.T) {
	g, ids := dependencyFixture(t)
	for _, test := range []struct {
		name string
		from graph.SymbolID
		to   graph.SymbolID
	}{
		{name: "same package", from: ids.orders, to: ids.orders},
		{name: "same struct", from: ids.service, to: ids.service},
		{name: "missing source", from: "missing", to: ids.repository},
		{name: "missing target", from: ids.service, to: "missing"},
		{name: "mixed levels", from: ids.orders, to: ids.repository},
		{name: "function endpoints", from: ids.create, to: ids.save},
	} {
		t.Run(test.name, func(t *testing.T) {
			if explanation, ok := query.WhyDependsOn(g, test.from, test.to); ok {
				t.Fatalf("WhyDependsOn(%q, %q) = %#v, true; want no explanation", test.from, test.to, explanation)
			}
		})
	}
}

func TestWhyDependsOnDoesNotExplainTransitiveOnlyRelationship(t *testing.T) {
	g := graph.New()
	packages := []graph.SymbolID{"example.com/a", "example.com/x", "example.com/b"}
	for _, packageID := range packages {
		if err := g.AddNode(graph.Node{ID: packageID, Kind: graph.NodePackage, Name: string(packageID)}); err != nil {
			t.Fatal(err)
		}
		functionID := graph.ChildID(packageID, "Run")
		if err := g.AddNode(graph.Node{ID: functionID, Kind: graph.NodeFunction, Name: "Run", Parent: packageID}); err != nil {
			t.Fatal(err)
		}
	}
	for offset, endpoints := range [][2]graph.SymbolID{
		{graph.ChildID(packages[0], "Run"), graph.ChildID(packages[1], "Run")},
		{graph.ChildID(packages[1], "Run"), graph.ChildID(packages[2], "Run")},
	} {
		if err := g.AddEdge(graph.Edge{
			From:     endpoints[0],
			To:       endpoints[1],
			Kind:     graph.EdgeCalls,
			Evidence: []graph.Location{{File: "run.go", Offset: offset}},
		}); err != nil {
			t.Fatal(err)
		}
	}

	if explanation, ok := query.WhyDependsOn(g, packages[0], packages[2]); ok {
		t.Fatalf("transitive-only explanation = %#v, true; want none", explanation)
	}
}

func TestAnalyzerPipelineExplainsServiceRepositoryDependency(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), filepath.Join("..", "goanalyzer", "testdata", "project"), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	explanation, ok := query.WhyDependsOn(
		g,
		"example.com/shop/orders::Service",
		"example.com/shop/orders::Repository",
	)
	if !ok {
		t.Fatal("missing Service -> Repository explanation")
	}
	if len(explanation.Evidence) != 2 {
		t.Fatalf("Service -> Repository explanation = %#v", explanation)
	}
	wantCallers := map[graph.SymbolID]bool{
		"example.com/shop/orders::Service::ClosureCalls": true,
		"example.com/shop/orders::Service::Create":       true,
	}
	for _, evidence := range explanation.Evidence {
		if !wantCallers[evidence.From] ||
			evidence.To != "example.com/shop/orders::Repository::Save" ||
			len(evidence.Locations) != 1 {
			t.Errorf("unexpected Service -> Repository evidence: %#v", evidence)
		}
	}
}
