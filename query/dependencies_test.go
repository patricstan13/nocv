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

func TestTypeDependenciesAggregateCallsAndRetainEvidence(t *testing.T) {
	g, ids := dependencyFixture(t)
	before := outgoingSnapshot(g)

	dependencies := query.Dependencies(g, graph.NodeStruct, graph.NodeInterface)
	if len(dependencies) != 1 {
		t.Fatalf("Dependencies(type level) = %#v, want one dependency", dependencies)
	}
	dependency := dependencies[0]
	if dependency.From != ids.service || dependency.To != ids.repository {
		t.Fatalf("dependency = %q -> %q, want %q -> %q", dependency.From, dependency.To, ids.service, ids.repository)
	}
	if len(dependency.Evidence) != 3 {
		t.Fatalf("dependency evidence = %#v, want three underlying calls", dependency.Evidence)
	}
	wantCalls := map[callPair]bool{
		{from: ids.create, to: ids.save}:             true,
		{from: ids.deleteMethod, to: ids.deleteRepo}: true,
		{from: ids.update, to: ids.save}:             true,
	}
	for _, evidence := range dependency.Evidence {
		pair := callPair{from: evidence.From, to: evidence.To}
		if !wantCalls[pair] {
			t.Errorf("unexpected evidence relationship %q -> %q", evidence.From, evidence.To)
		}
		if len(evidence.Locations) != 1 || evidence.Locations[0].File != "orders.go" {
			t.Errorf("locations for %q -> %q = %#v", evidence.From, evidence.To, evidence.Locations)
		}
	}

	if after := outgoingSnapshot(g); !reflect.DeepEqual(after, before) {
		t.Fatalf("projection mutated semantic edges:\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func TestPackageDependenciesAggregateCrossPackageCalls(t *testing.T) {
	g, ids := dependencyFixture(t)
	dependencies := query.Dependencies(g, graph.NodePackage)
	if len(dependencies) != 1 {
		t.Fatalf("Dependencies(package level) = %#v, want one dependency", dependencies)
	}
	dependency := dependencies[0]
	if dependency.From != ids.orders || dependency.To != ids.logging {
		t.Fatalf("dependency = %q -> %q, want %q -> %q", dependency.From, dependency.To, ids.orders, ids.logging)
	}
	if len(dependency.Evidence) != 2 {
		t.Fatalf("dependency evidence = %#v, want two cross-package calls", dependency.Evidence)
	}
}

func TestUnsupportedProjectionReturnsNoDependencies(t *testing.T) {
	g, _ := dependencyFixture(t)
	if got := query.Dependencies(g, graph.NodeFunction); len(got) != 0 {
		t.Fatalf("Dependencies(function level) = %#v, want no projected dependencies", got)
	}
	if got := query.Dependencies(g); len(got) != 0 {
		t.Fatalf("Dependencies(without a level) = %#v, want no projected dependencies", got)
	}
}

func TestAnalyzerPipelineProjectsPackageDependency(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), filepath.Join("..", "goanalyzer", "testdata", "project"), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	dependencies := query.Dependencies(g, graph.NodePackage)
	for _, dependency := range dependencies {
		if dependency.From == "example.com/shop/orders" && dependency.To == "example.com/shop/logging" {
			if len(dependency.Evidence) != 2 {
				t.Fatalf("orders -> logging evidence = %#v, want two calls", dependency.Evidence)
			}
			return
		}
	}
	t.Fatalf("missing orders -> logging dependency; got %#v", dependencies)
}

type fixtureIDs struct {
	orders       graph.SymbolRef
	logging      graph.SymbolRef
	service      graph.SymbolRef
	repository   graph.SymbolRef
	create       graph.SymbolRef
	deleteMethod graph.SymbolRef
	update       graph.SymbolRef
	validate     graph.SymbolRef
	save         graph.SymbolRef
	deleteRepo   graph.SymbolRef
	process      graph.SymbolRef
	info         graph.SymbolRef
}

type callPair struct {
	from graph.SymbolRef
	to   graph.SymbolRef
}

func dependencyFixture(t *testing.T) (*graph.Graph, fixtureIDs) {
	t.Helper()
	ids := fixtureIDs{
		orders:       "example.com/shop/orders",
		logging:      "example.com/shop/logging",
		service:      "example.com/shop/orders::Service",
		repository:   "example.com/shop/orders::Repository",
		create:       "example.com/shop/orders::Service::Create",
		deleteMethod: "example.com/shop/orders::Service::Delete",
		update:       "example.com/shop/orders::Service::Update",
		validate:     "example.com/shop/orders::Service::Validate",
		save:         "example.com/shop/orders::Repository::Save",
		deleteRepo:   "example.com/shop/orders::Repository::Delete",
		process:      "example.com/shop/orders::Process",
		info:         "example.com/shop/logging::Info",
	}

	g := graph.New()
	for _, node := range []testNode{
		{ID: ids.orders, Kind: graph.NodePackage, Name: "orders"},
		{ID: ids.logging, Kind: graph.NodePackage, Name: "logging"},
		{ID: ids.service, Kind: graph.NodeStruct, Name: "Service", Parent: ids.orders},
		{ID: ids.repository, Kind: graph.NodeInterface, Name: "Repository", Parent: ids.orders},
		{ID: ids.create, Kind: graph.NodeFunction, Name: "Create", Parent: ids.service},
		{ID: ids.deleteMethod, Kind: graph.NodeFunction, Name: "Delete", Parent: ids.service},
		{ID: ids.update, Kind: graph.NodeFunction, Name: "Update", Parent: ids.service},
		{ID: ids.validate, Kind: graph.NodeFunction, Name: "Validate", Parent: ids.service},
		{ID: ids.save, Kind: graph.NodeFunction, Name: "Save", Parent: ids.repository},
		{ID: ids.deleteRepo, Kind: graph.NodeFunction, Name: "Delete", Parent: ids.repository},
		{ID: ids.process, Kind: graph.NodeFunction, Name: "Process", Parent: ids.orders},
		{ID: ids.info, Kind: graph.NodeFunction, Name: "Info", Parent: ids.logging},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	for index, pair := range []callPair{
		{from: ids.create, to: ids.save},
		{from: ids.deleteMethod, to: ids.deleteRepo},
		{from: ids.update, to: ids.save},
		{from: ids.create, to: ids.validate},
		{from: ids.process, to: ids.info},
		{from: ids.create, to: ids.info},
		{from: ids.process, to: ids.save},
	} {
		err := addTestEdge(g, testEdge{
			From: pair.from,
			To:   pair.to,
			Kind: graph.EdgeCalls,
			Evidence: []graph.Location{{
				File:   "orders.go",
				Offset: index + 1,
			}},
		})
		if err != nil {
			t.Fatalf("AddEdge(%q -> %q): %v", pair.from, pair.to, err)
		}
	}
	return g, ids
}

func outgoingSnapshot(g *graph.Graph) map[graph.SymbolRef][]*graph.Edge {
	snapshot := make(map[graph.SymbolRef][]*graph.Edge)
	for _, node := range g.Nodes() {
		if edges := g.Outgoing(node.ID); len(edges) > 0 {
			snapshot[node.Ref] = edges
		}
	}
	return snapshot
}
