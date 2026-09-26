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

func TestDirectPackageDependenciesAggregateBoundaryFactsAndIgnoreInternalAndImports(t *testing.T) {
	g := graph.New()
	a := graph.SymbolID("a")
	b := graph.SymbolID("b")
	c := graph.SymbolID("c")
	plugin := graph.SymbolID("plugin")
	for _, id := range []graph.SymbolID{a, b, c, plugin} {
		mustAddPackagePathNode(t, g, graph.Node{ID: id, Kind: graph.NodePackage, Name: string(id)})
	}

	nodes := []graph.Node{
		{ID: "a::Accept", Kind: graph.NodeFunction, Name: "Accept", Parent: a},
		{ID: "a::Embed", Kind: graph.NodeStruct, Name: "Embed", Parent: a},
		{ID: "a::Helper1", Kind: graph.NodeFunction, Name: "Helper1", Parent: a},
		{ID: "a::Helper2", Kind: graph.NodeFunction, Name: "Helper2", Parent: a},
		{ID: "a::Implement", Kind: graph.NodeStruct, Name: "Implement", Parent: a},
		{ID: "a::Return", Kind: graph.NodeFunction, Name: "Return", Parent: a},
		{ID: "a::Start", Kind: graph.NodeFunction, Name: "Start", Parent: a},
		{ID: "a::ToC", Kind: graph.NodeFunction, Name: "ToC", Parent: a},
		{ID: "b::Base", Kind: graph.NodeStruct, Name: "Base", Parent: b},
		{ID: "b::Contract", Kind: graph.NodeInterface, Name: "Contract", Parent: b},
		{ID: "b::Target", Kind: graph.NodeFunction, Name: "Target", Parent: b},
		{ID: "c::Target", Kind: graph.NodeFunction, Name: "Target", Parent: c},
	}
	for _, node := range nodes {
		mustAddPackagePathNode(t, g, node)
	}

	edges := []graph.Edge{
		{From: "a::Start", To: "a::Helper1", Kind: graph.EdgeCalls},
		{From: "a::Helper1", To: "a::Helper2", Kind: graph.EdgeCalls},
		{From: "a::Helper2", To: "b::Target", Kind: graph.EdgeCalls},
		{From: "a::Accept", To: "b::Contract", Kind: graph.EdgeAccepts},
		{From: "a::Return", To: "b::Base", Kind: graph.EdgeReturns},
		{From: "a::Implement", To: "b::Contract", Kind: graph.EdgeImplements},
		{From: "a::Embed", To: "b::Base", Kind: graph.EdgeEmbeds},
		{From: "a::ToC", To: "c::Target", Kind: graph.EdgeCalls},
		{From: a, To: plugin, Kind: graph.EdgeImports},
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "projection.go", Offset: index}}
		mustAddPackagePathEdge(t, g, edges[index])
	}

	before := importGraphSnapshot(g)
	got := query.DirectPackageDependencies(g, a)
	if len(got) != 2 || got[0].To != b || got[1].To != c {
		t.Fatalf("DirectPackageDependencies(a) = %#v, want sorted dependencies to b and c", got)
	}
	if len(got[0].Evidence) != 5 {
		t.Fatalf("a -> b evidence = %#v, want five cross-package semantic facts", got[0].Evidence)
	}
	wantKinds := map[graph.EdgeKind]bool{
		graph.EdgeCalls: true, graph.EdgeAccepts: true, graph.EdgeReturns: true,
		graph.EdgeImplements: true, graph.EdgeEmbeds: true,
	}
	for index, evidence := range got[0].Evidence {
		if evidence.From == "a::Start" || evidence.From == "a::Helper1" || evidence.Kind == graph.EdgeImports {
			t.Errorf("internal/import fact leaked into a -> b evidence: %#v", evidence)
		}
		if len(evidence.Evidence) != 1 || evidence.Evidence[0].File != "projection.go" {
			t.Errorf("boundary evidence lost source locations: %#v", evidence)
		}
		wantKinds[evidence.Kind] = false
		if index > 0 && relationshipOrderLess(evidence, got[0].Evidence[index-1]) {
			t.Errorf("evidence is not deterministic: %#v", got[0].Evidence)
		}
	}
	for kind, missing := range wantKinds {
		if missing {
			t.Errorf("missing %s evidence from package dependency", kind)
		}
	}
	if gotAgain := query.DirectPackageDependencies(g, a); !reflect.DeepEqual(gotAgain, got) {
		t.Fatalf("second direct package query = %#v, want deterministic %#v", gotAgain, got)
	}

	got[0].Evidence[0].Evidence[0].Offset = 999
	if after := importGraphSnapshot(g); !reflect.DeepEqual(after, before) {
		t.Fatalf("direct package query mutated graph:\nbefore: %#v\nafter:  %#v", before, after)
	}
	for _, dependencies := range [][]query.PackageDependency{
		query.DirectPackageDependencies(nil, a),
		query.DirectPackageDependencies(g, "missing"),
		query.DirectPackageDependencies(g, "a::Start"),
		query.DirectPackageDependencies(g, plugin),
	} {
		if len(dependencies) != 0 {
			t.Errorf("invalid or empty package dependencies = %#v, want empty", dependencies)
		}
	}
}

func TestPackageDependencyPathsTraversePackageViewAcrossDisconnectedSymbols(t *testing.T) {
	g := graph.New()
	a := graph.SymbolID("a")
	b := graph.SymbolID("b")
	c := graph.SymbolID("c")
	d := graph.SymbolID("d")
	for _, id := range []graph.SymbolID{a, b, c, d} {
		mustAddPackagePathNode(t, g, graph.Node{ID: id, Kind: graph.NodePackage, Name: string(id)})
	}
	for _, id := range []graph.SymbolID{"a::ToB", "a::ToC", "b::FromA", "b::ToD", "c::FromA", "c::ToD", "d::FromB", "d::FromC"} {
		value := string(id)
		parent := graph.SymbolID(value[:1])
		mustAddPackagePathNode(t, g, graph.Node{ID: id, Kind: graph.NodeFunction, Name: value[3:], Parent: parent})
	}
	edges := []graph.Edge{
		{From: "a::ToB", To: "b::FromA", Kind: graph.EdgeCalls},
		{From: "b::ToD", To: "d::FromB", Kind: graph.EdgeCalls},
		{From: "a::ToC", To: "c::FromA", Kind: graph.EdgeCalls},
		{From: "c::ToD", To: "d::FromC", Kind: graph.EdgeCalls},
		{From: "b::FromA", To: "a::ToB", Kind: graph.EdgeCalls}, // malformed package cycle
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "routes.go", Offset: index}}
		mustAddPackagePathEdge(t, g, edges[index])
	}

	if exact := query.DependencyPaths(g, "a::ToB", "d::FromB"); len(exact) != 0 {
		t.Fatalf("disconnected B symbols unexpectedly formed exact path: %#v", exact)
	}
	wantPackages := [][]graph.SymbolID{{a, b, d}, {a, c, d}}
	paths := query.PackageDependencyPaths(g, a, d)
	if len(paths) != len(wantPackages) {
		t.Fatalf("PackageDependencyPaths(a, d) = %#v, want two routes", paths)
	}
	for index, path := range paths {
		if !reflect.DeepEqual(path.Packages, wantPackages[index]) {
			t.Errorf("path %d packages = %v, want %v", index, path.Packages, wantPackages[index])
		}
		if len(path.Steps) != len(path.Packages)-1 {
			t.Errorf("path %d has %d steps for %d packages", index, len(path.Steps), len(path.Packages))
		}
		for stepIndex, step := range path.Steps {
			if step.From != path.Packages[stepIndex] || step.To != path.Packages[stepIndex+1] || len(step.Evidence) != 1 {
				t.Errorf("path %d step %d violates hop invariant: %#v", index, stepIndex, step)
			}
		}
	}
	if again := query.PackageDependencyPaths(g, a, d); !reflect.DeepEqual(again, paths) {
		t.Fatalf("second package path query = %#v, want deterministic %#v", again, paths)
	}
}

func TestPackageViewCollapsesInternalChainsWithoutChangingExactPaths(t *testing.T) {
	g := graph.New()
	a := graph.SymbolID("a")
	b := graph.SymbolID("b")
	for _, id := range []graph.SymbolID{a, b} {
		mustAddPackagePathNode(t, g, graph.Node{ID: id, Kind: graph.NodePackage, Name: string(id)})
	}
	for _, node := range []graph.Node{
		{ID: "a::Start", Kind: graph.NodeFunction, Name: "Start", Parent: a},
		{ID: "a::Helper1", Kind: graph.NodeFunction, Name: "Helper1", Parent: a},
		{ID: "a::Helper2", Kind: graph.NodeFunction, Name: "Helper2", Parent: a},
		{ID: "a::One", Kind: graph.NodeFunction, Name: "One", Parent: a},
		{ID: "a::Two", Kind: graph.NodeFunction, Name: "Two", Parent: a},
		{ID: "b::Target", Kind: graph.NodeFunction, Name: "Target", Parent: b},
	} {
		mustAddPackagePathNode(t, g, node)
	}
	edges := []graph.Edge{
		{From: "a::Start", To: "a::Helper1", Kind: graph.EdgeCalls},
		{From: "a::Helper1", To: "a::Helper2", Kind: graph.EdgeCalls},
		{From: "a::Helper2", To: "b::Target", Kind: graph.EdgeCalls},
		{From: "a::Start", To: "a::One", Kind: graph.EdgeCalls},
		{From: "a::Start", To: "a::Two", Kind: graph.EdgeCalls},
		{From: "a::One", To: "b::Target", Kind: graph.EdgeCalls},
		{From: "a::Two", To: "b::Target", Kind: graph.EdgeCalls},
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "internal.go", Offset: index}}
		mustAddPackagePathEdge(t, g, edges[index])
	}

	exact := query.DependencyPaths(g, "a::Start", "b::Target")
	if len(exact) != 3 || len(exact[2].Steps) != 3 {
		t.Fatalf("exact DependencyPaths changed: %#v, want two two-step and one three-step path", exact)
	}
	dependencies := query.DirectPackageDependencies(g, a)
	if len(dependencies) != 1 || len(dependencies[0].Evidence) != 3 {
		t.Fatalf("direct package view = %#v, want one a -> b edge with three boundary facts", dependencies)
	}
	for _, evidence := range dependencies[0].Evidence {
		if evidence.From == "a::Start" || evidence.From == "a::Helper1" {
			t.Errorf("internal approach leaked into boundary evidence: %#v", evidence)
		}
	}
}

func TestPackageDependencyPathsValidateExactPackageEndpoints(t *testing.T) {
	g := graph.New()
	a := graph.SymbolID("a")
	b := graph.SymbolID("b")
	function := graph.SymbolID("a::Run")
	for _, node := range []graph.Node{
		{ID: a, Kind: graph.NodePackage, Name: "a"},
		{ID: b, Kind: graph.NodePackage, Name: "b"},
		{ID: function, Kind: graph.NodeFunction, Name: "Run", Parent: a},
	} {
		mustAddPackagePathNode(t, g, node)
	}
	for _, paths := range [][]query.PackageDependencyPath{
		query.PackageDependencyPaths(nil, a, b),
		query.PackageDependencyPaths(g, "missing", b),
		query.PackageDependencyPaths(g, a, "missing"),
		query.PackageDependencyPaths(g, function, b),
		query.PackageDependencyPaths(g, a, function),
		query.PackageDependencyPaths(g, a, a),
		query.PackageDependencyPaths(g, a, b),
	} {
		if len(paths) != 0 {
			t.Errorf("invalid, identical, or disconnected package paths = %#v, want empty", paths)
		}
	}
}

func TestAnalyzerPipelineBuildsPackageViewWithoutChangingExactPaths(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), filepath.Join("..", "goanalyzer", "testdata", "project"), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	app := graph.SymbolID("example.com/shop/app")
	service := graph.SymbolID("example.com/shop/service")
	repository := graph.SymbolID("example.com/shop/repository")
	run := graph.SymbolID("example.com/shop/app::Run")
	create := graph.SymbolID("example.com/shop/service::Create")
	update := graph.SymbolID("example.com/shop/service::Update")
	save := graph.SymbolID("example.com/shop/repository::Save")
	want := []query.PackageDependencyPath{{
		Packages: []graph.SymbolID{app, service, repository},
		Steps: []query.PackageDependency{
			{From: app, To: service, Evidence: []query.Relationship{
				{From: run, To: create, Kind: graph.EdgeCalls, Evidence: graphEvidence(g, run, create, graph.EdgeCalls)},
				{From: run, To: update, Kind: graph.EdgeCalls, Evidence: graphEvidence(g, run, update, graph.EdgeCalls)},
			}},
			{From: service, To: repository, Evidence: []query.Relationship{
				{From: create, To: save, Kind: graph.EdgeCalls, Evidence: graphEvidence(g, create, save, graph.EdgeCalls)},
				{From: update, To: save, Kind: graph.EdgeCalls, Evidence: graphEvidence(g, update, save, graph.EdgeCalls)},
			}},
		},
	}}
	if got := query.PackageDependencyPaths(g, app, repository); !reflect.DeepEqual(got, want) {
		t.Fatalf("fixture package paths = %#v, want %#v", got, want)
	}
	wantExact := []query.SemanticPath{
		{Steps: []query.SemanticStep{{From: run, To: create, Kind: graph.EdgeCalls}, {From: create, To: save, Kind: graph.EdgeCalls}}},
		{Steps: []query.SemanticStep{{From: run, To: update, Kind: graph.EdgeCalls}, {From: update, To: save, Kind: graph.EdgeCalls}}},
	}
	if got := query.DependencyPaths(g, run, save); !reflect.DeepEqual(got, wantExact) {
		t.Fatalf("fixture exact paths = %#v, want unchanged %#v", got, wantExact)
	}
}

func graphEvidence(g *graph.Graph, from, to graph.SymbolID, kind graph.EdgeKind) []graph.Location {
	for _, edge := range g.Outgoing(from, kind) {
		if edge.To == to {
			return edge.Evidence
		}
	}
	return nil
}

func relationshipOrderLess(left, right query.Relationship) bool {
	if left.From != right.From {
		return left.From < right.From
	}
	if left.Kind != right.Kind {
		return left.Kind < right.Kind
	}
	return left.To < right.To
}

func mustAddPackagePathNode(t *testing.T, g *graph.Graph, node graph.Node) {
	t.Helper()
	if err := g.AddNode(node); err != nil {
		t.Fatalf("AddNode(%q): %v", node.ID, err)
	}
}

func mustAddPackagePathEdge(t *testing.T, g *graph.Graph, edge graph.Edge) {
	t.Helper()
	if err := g.AddEdge(edge); err != nil {
		t.Fatalf("AddEdge(%s, %q -> %q): %v", edge.Kind, edge.From, edge.To, err)
	}
}
