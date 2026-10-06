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

func TestDirectPackageDependenciesAggregateBoundaryFactsAndIgnoreInternalAndImports(t *testing.T) {
	g := graph.New()
	a := graph.SymbolRef("a")
	b := graph.SymbolRef("b")
	c := graph.SymbolRef("c")
	plugin := graph.SymbolRef("plugin")
	for _, id := range []graph.SymbolRef{a, b, c, plugin} {
		mustAddPackagePathNode(t, g, testNode{ID: id, Kind: graph.NodePackage, Name: string(id)})
	}

	nodes := []testNode{
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

	edges := []testEdge{
		{From: "a::Start", To: "a::Helper1", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "a::Helper1", To: "a::Helper2", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "a::Helper2", To: "b::Target", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "a::Accept", To: "b::Contract", Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed},
		{From: "a::Return", To: "b::Base", Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed},
		{From: "a::Implement", To: "b::Contract", Kind: graph.EdgeImplements, Certainty: graph.RelationshipConfirmed},
		{From: "a::Embed", To: "b::Base", Kind: graph.EdgeEmbeds, Certainty: graph.RelationshipConfirmed},
		{From: "a::ToC", To: "c::Target", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: a, To: plugin, Kind: graph.EdgeImports, Certainty: graph.RelationshipConfirmed},
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
	a := graph.SymbolRef("a")
	b := graph.SymbolRef("b")
	c := graph.SymbolRef("c")
	d := graph.SymbolRef("d")
	for _, id := range []graph.SymbolRef{a, b, c, d} {
		mustAddPackagePathNode(t, g, testNode{ID: id, Kind: graph.NodePackage, Name: string(id)})
	}
	for _, id := range []graph.SymbolRef{"a::ToB", "a::ToC", "b::FromA", "b::ToD", "c::FromA", "c::ToD", "d::FromB", "d::FromC"} {
		value := string(id)
		parent := graph.SymbolRef(value[:1])
		mustAddPackagePathNode(t, g, testNode{ID: id, Kind: graph.NodeFunction, Name: value[3:], Parent: parent})
	}
	edges := []testEdge{
		{From: "a::ToB", To: "b::FromA", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "b::ToD", To: "d::FromB", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "a::ToC", To: "c::FromA", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "c::ToD", To: "d::FromC", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "b::FromA", To: "a::ToB", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed}, // malformed package cycle
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "routes.go", Offset: index}}
		mustAddPackagePathEdge(t, g, edges[index])
	}

	if exact := query.DependencyPaths(g, "a::ToB", "d::FromB"); len(exact) != 0 {
		t.Fatalf("disconnected B symbols unexpectedly formed exact path: %#v", exact)
	}
	wantPackages := [][]graph.SymbolRef{{a, b, d}, {a, c, d}}
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
	a := graph.SymbolRef("a")
	b := graph.SymbolRef("b")
	for _, id := range []graph.SymbolRef{a, b} {
		mustAddPackagePathNode(t, g, testNode{ID: id, Kind: graph.NodePackage, Name: string(id)})
	}
	for _, node := range []testNode{
		{ID: "a::Start", Kind: graph.NodeFunction, Name: "Start", Parent: a},
		{ID: "a::Helper1", Kind: graph.NodeFunction, Name: "Helper1", Parent: a},
		{ID: "a::Helper2", Kind: graph.NodeFunction, Name: "Helper2", Parent: a},
		{ID: "a::One", Kind: graph.NodeFunction, Name: "One", Parent: a},
		{ID: "a::Two", Kind: graph.NodeFunction, Name: "Two", Parent: a},
		{ID: "b::Target", Kind: graph.NodeFunction, Name: "Target", Parent: b},
	} {
		mustAddPackagePathNode(t, g, node)
	}
	edges := []testEdge{
		{From: "a::Start", To: "a::Helper1", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "a::Helper1", To: "a::Helper2", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "a::Helper2", To: "b::Target", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "a::Start", To: "a::One", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "a::Start", To: "a::Two", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "a::One", To: "b::Target", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "a::Two", To: "b::Target", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
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
	a := graph.SymbolRef("a")
	b := graph.SymbolRef("b")
	function := graph.SymbolRef("a::Run")
	for _, node := range []testNode{
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
	g, err := goanalyzer.Load(context.Background(), testutil.GoProjectDir(t), "./app", "./repository", "./service")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	app := graph.SymbolRef("example.com/shop/app")
	service := graph.SymbolRef("example.com/shop/service")
	repository := graph.SymbolRef("example.com/shop/repository")
	run := graph.SymbolRef("example.com/shop/app::Run")
	create := graph.SymbolRef("example.com/shop/service::Create")
	update := graph.SymbolRef("example.com/shop/service::Update")
	save := graph.SymbolRef("example.com/shop/repository::Save")
	want := []query.PackageDependencyPath{{
		Packages: []graph.SymbolRef{app, service, repository},
		Steps: []query.PackageDependency{
			{From: app, To: service, Certainty: graph.RelationshipConfirmed, Evidence: []query.Relationship{
				{From: run, To: create, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed, Evidence: graphEvidence(g, run, create, graph.EdgeCalls)},
				{From: run, To: update, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed, Evidence: graphEvidence(g, run, update, graph.EdgeCalls)},
			}},
			{From: service, To: repository, Certainty: graph.RelationshipConfirmed, Evidence: []query.Relationship{
				{From: create, To: save, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed, Evidence: graphEvidence(g, create, save, graph.EdgeCalls)},
				{From: update, To: save, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed, Evidence: graphEvidence(g, update, save, graph.EdgeCalls)},
			}},
		},
	}}
	if got := query.PackageDependencyPaths(g, app, repository); !reflect.DeepEqual(got, want) {
		t.Fatalf("fixture package paths = %#v, want %#v", got, want)
	}
	wantExact := []query.SemanticPath{
		{Steps: []query.SemanticStep{{From: run, To: create, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed}, {From: create, To: save, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed}}},
		{Steps: []query.SemanticStep{{From: run, To: update, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed}, {From: update, To: save, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed}}},
	}
	if got := query.DependencyPaths(g, run, save); !reflect.DeepEqual(got, wantExact) {
		t.Fatalf("fixture exact paths = %#v, want unchanged %#v", got, wantExact)
	}
}

func graphEvidence(g *graph.Graph, from, to graph.SymbolRef, kind graph.EdgeKind) []graph.Location {
	fromID, _ := g.Resolve(from)
	toID, _ := g.Resolve(to)
	for _, edge := range g.Outgoing(fromID, kind) {
		if edge.To == toID {
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

func mustAddPackagePathNode(t *testing.T, g *graph.Graph, node testNode) {
	t.Helper()
	if err := addTestNode(g, node); err != nil {
		t.Fatalf("AddNode(%q): %v", node.ID, err)
	}
}

func mustAddPackagePathEdge(t *testing.T, g *graph.Graph, edge testEdge) {
	t.Helper()
	if err := addTestEdge(g, edge); err != nil {
		t.Fatalf("AddEdge(%s, %q -> %q): %v", edge.Kind, edge.From, edge.To, err)
	}
}
