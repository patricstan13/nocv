package query_test

import (
	"reflect"
	"testing"

	"nocv/graph"
	"nocv/query"
)

func TestDirectImportNavigationIsDeterministicDefensiveAndValidated(t *testing.T) {
	g, ids := importFixture(t)
	before := importGraphSnapshot(g)

	wantImports := []query.Relationship{
		{From: ids.app, To: ids.plugin, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Offset: 30}}},
		{From: ids.app, To: ids.repository, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Offset: 20}}},
		{From: ids.app, To: ids.service, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Offset: 10}, {File: "other.go", Offset: 5}}},
	}
	imports := query.DirectImports(g, ids.app)
	if !reflect.DeepEqual(imports, wantImports) {
		t.Fatalf("DirectImports(app) = %#v, want %#v", imports, wantImports)
	}
	wantImporters := []query.Relationship{
		{From: ids.app, To: ids.service, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Offset: 10}, {File: "other.go", Offset: 5}}},
		{From: ids.worker, To: ids.service, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "worker.go", Offset: 10}}},
	}
	if got := query.DirectImporters(g, ids.service); !reflect.DeepEqual(got, wantImporters) {
		t.Fatalf("DirectImporters(service) = %#v, want %#v", got, wantImporters)
	}

	imports[0].Evidence[0].Offset = 999
	if after := importGraphSnapshot(g); !reflect.DeepEqual(after, before) {
		t.Fatalf("import queries mutated graph:\nbefore: %#v\nafter:  %#v", before, after)
	}
	if again := query.DirectImports(g, ids.app); !reflect.DeepEqual(again, wantImports) {
		t.Fatalf("second DirectImports(app) = %#v, want %#v", again, wantImports)
	}

	for _, relationships := range [][]query.Relationship{
		query.DirectImports(nil, ids.app),
		query.DirectImporters(nil, ids.service),
		query.DirectImports(g, "missing"),
		query.DirectImporters(g, "missing"),
		query.DirectImports(g, ids.appFunction),
		query.DirectImporters(g, ids.serviceFunction),
	} {
		if len(relationships) != 0 {
			t.Errorf("invalid import navigation = %#v, want empty", relationships)
		}
	}
}

func TestImportsRemainSeparateFromSemanticQueries(t *testing.T) {
	g, ids := importFixture(t)

	if got := query.DirectImports(g, ids.app); !hasRelationship(got, ids.app, ids.plugin, graph.EdgeImports) {
		t.Fatalf("blank import missing from DirectImports: %#v", got)
	}
	if got := query.DirectDependencies(g, ids.app); len(got) != 0 {
		t.Fatalf("DirectDependencies(app) included imports: %#v", got)
	}
	if got := query.DirectDependents(g, ids.plugin); len(got) != 0 {
		t.Fatalf("DirectDependents(plugin) included blank import: %#v", got)
	}
	if got := query.DependencyPaths(g, ids.app, ids.plugin); len(got) != 0 {
		t.Fatalf("DependencyPaths(app, plugin) included blank import: %#v", got)
	}
	if got := query.TransitiveDependents(g, ids.plugin); len(got) != 0 {
		t.Fatalf("TransitiveDependents(plugin) included blank import: %#v", got)
	}
	if got := query.PackageDependencyPaths(g, ids.app, ids.plugin); len(got) != 0 {
		t.Fatalf("PackageDependencyPaths(app, plugin) included blank import: %#v", got)
	}
}

type importIDs struct {
	app             graph.SymbolRef
	plugin          graph.SymbolRef
	repository      graph.SymbolRef
	service         graph.SymbolRef
	worker          graph.SymbolRef
	appFunction     graph.SymbolRef
	serviceFunction graph.SymbolRef
}

func importFixture(t *testing.T) (*graph.Graph, importIDs) {
	t.Helper()
	ids := importIDs{
		app:             "example.com/project/app",
		plugin:          "example.com/project/plugin",
		repository:      "example.com/project/repository",
		service:         "example.com/project/service",
		worker:          "example.com/project/worker",
		appFunction:     "example.com/project/app::Run",
		serviceFunction: "example.com/project/service::Run",
	}
	g := graph.New()
	for _, node := range []testNode{
		{ID: ids.app, Kind: graph.NodePackage, Name: "app"},
		{ID: ids.plugin, Kind: graph.NodePackage, Name: "plugin"},
		{ID: ids.repository, Kind: graph.NodePackage, Name: "repository"},
		{ID: ids.service, Kind: graph.NodePackage, Name: "service"},
		{ID: ids.worker, Kind: graph.NodePackage, Name: "worker"},
		{ID: ids.appFunction, Kind: graph.NodeFunction, Name: "Run", Parent: ids.app},
		{ID: ids.serviceFunction, Kind: graph.NodeFunction, Name: "Run", Parent: ids.service},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}
	edges := []testEdge{
		{From: ids.app, To: ids.service, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Offset: 10}}},
		{From: ids.worker, To: ids.service, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "worker.go", Offset: 10}}},
		{From: ids.app, To: ids.repository, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Offset: 20}}},
		{From: ids.app, To: ids.plugin, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Offset: 30}}},
		{From: ids.app, To: ids.service, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "other.go", Offset: 5}}},
		{From: ids.appFunction, To: ids.serviceFunction, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "app.go", Offset: 50}}},
	}
	for _, edge := range edges {
		if err := addTestEdge(g, edge); err != nil {
			t.Fatalf("AddEdge(%s, %q -> %q): %v", edge.Kind, edge.From, edge.To, err)
		}
	}
	return g, ids
}

func importGraphSnapshot(g *graph.Graph) map[graph.SymbolRef][]*graph.Edge {
	snapshot := make(map[graph.SymbolRef][]*graph.Edge)
	for _, node := range g.Nodes() {
		snapshot[node.Ref] = g.Outgoing(node.ID)
	}
	return snapshot
}

func hasRelationship(relationships []query.Relationship, from, to graph.SymbolRef, kind graph.EdgeKind) bool {
	for _, relationship := range relationships {
		if relationship.From == from && relationship.To == to && relationship.Kind == kind {
			return true
		}
	}
	return false
}
