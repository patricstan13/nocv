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

func TestDependencyPathsPreserveAllBranchesKindsAndOrdering(t *testing.T) {
	g, ids := branchingImpactFixture(t)
	before := outgoingSnapshot(g)

	want := []query.SemanticPath{
		{Steps: []query.SemanticStep{{From: ids.a, To: ids.changed, Kind: graph.EdgeAccepts}}},
		{Steps: []query.SemanticStep{{From: ids.a, To: ids.changed, Kind: graph.EdgeReturns}}},
		{Steps: []query.SemanticStep{
			{From: ids.a, To: ids.b, Kind: graph.EdgeCalls},
			{From: ids.b, To: ids.changed, Kind: graph.EdgeAccepts},
		}},
		{Steps: []query.SemanticStep{
			{From: ids.a, To: ids.c, Kind: graph.EdgeCalls},
			{From: ids.c, To: ids.changed, Kind: graph.EdgeReturns},
		}},
	}
	got := query.DependencyPaths(g, ids.a, ids.changed)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DependencyPaths(A, Changed) = %#v, want %#v", got, want)
	}
	if again := query.DependencyPaths(g, ids.a, ids.changed); !reflect.DeepEqual(again, want) {
		t.Fatalf("second DependencyPaths(A, Changed) = %#v, want deterministic %#v", again, want)
	}

	fromD := query.DependencyPaths(g, ids.d, ids.changed)
	if len(fromD) != 4 {
		t.Fatalf("DependencyPaths(D, Changed) = %#v, want four paths", fromD)
	}
	if len(fromD[0].Steps) != 2 || len(fromD[1].Steps) != 2 ||
		len(fromD[2].Steps) != 3 || len(fromD[3].Steps) != 3 {
		t.Fatalf("D path lengths = %d, %d, %d, %d; want 2, 2, 3, 3",
			len(fromD[0].Steps), len(fromD[1].Steps), len(fromD[2].Steps), len(fromD[3].Steps))
	}
	if after := outgoingSnapshot(g); !reflect.DeepEqual(after, before) {
		t.Fatalf("dependency path query mutated graph edges:\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func TestDependencyPathsTraverseEverySemanticKindAndMixedPaths(t *testing.T) {
	g := graph.New()
	pkgID := graph.PackageRef("example.com/mixed")
	a := graph.ChildRef(pkgID, "A")
	b := graph.ChildRef(pkgID, "B")
	c := graph.ChildRef(pkgID, "C")
	d := graph.ChildRef(pkgID, "D")
	implementation := graph.ChildRef(pkgID, "Implementation")
	returner := graph.ChildRef(pkgID, "Returner")
	for _, node := range []testNode{
		{ID: pkgID, Kind: graph.NodePackage, Name: "mixed"},
		{ID: a, Kind: graph.NodeFunction, Name: "A", Parent: pkgID},
		{ID: b, Kind: graph.NodeFunction, Name: "B", Parent: pkgID},
		{ID: c, Kind: graph.NodeInterface, Name: "C", Parent: pkgID},
		{ID: d, Kind: graph.NodeInterface, Name: "D", Parent: pkgID},
		{ID: implementation, Kind: graph.NodeStruct, Name: "Implementation", Parent: pkgID},
		{ID: returner, Kind: graph.NodeFunction, Name: "Returner", Parent: pkgID},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	edges := []testEdge{
		{From: a, To: b, Kind: graph.EdgeCalls},
		{From: b, To: c, Kind: graph.EdgeAccepts},
		{From: c, To: d, Kind: graph.EdgeEmbeds},
		{From: implementation, To: c, Kind: graph.EdgeImplements},
		{From: returner, To: c, Kind: graph.EdgeReturns},
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "mixed.go", Offset: index}}
		if err := addTestEdge(g, edges[index]); err != nil {
			t.Fatal(err)
		}
	}

	wantMixed := []query.SemanticPath{{Steps: []query.SemanticStep{
		{From: a, To: b, Kind: graph.EdgeCalls},
		{From: b, To: c, Kind: graph.EdgeAccepts},
		{From: c, To: d, Kind: graph.EdgeEmbeds},
	}}}
	if got := query.DependencyPaths(g, a, d); !reflect.DeepEqual(got, wantMixed) {
		t.Fatalf("mixed DependencyPaths(A, D) = %#v, want %#v", got, wantMixed)
	}

	tests := []struct {
		from graph.SymbolRef
		to   graph.SymbolRef
		kind graph.EdgeKind
	}{
		{from: a, to: b, kind: graph.EdgeCalls},
		{from: b, to: c, kind: graph.EdgeAccepts},
		{from: c, to: d, kind: graph.EdgeEmbeds},
		{from: implementation, to: c, kind: graph.EdgeImplements},
		{from: returner, to: c, kind: graph.EdgeReturns},
	}
	for _, test := range tests {
		paths := query.DependencyPaths(g, test.from, test.to)
		if len(paths) != 1 || len(paths[0].Steps) != 1 || paths[0].Steps[0] != (query.SemanticStep{From: test.from, To: test.to, Kind: test.kind}) {
			t.Errorf("DependencyPaths(%q, %q) = %#v, want one %s step", test.from, test.to, paths, test.kind)
		}
	}
}

func TestDependencyPathsTerminateCyclesWithoutSuppressingBranches(t *testing.T) {
	g := graph.New()
	pkgID := graph.PackageRef("example.com/cycle")
	a := graph.ChildRef(pkgID, "A")
	b := graph.ChildRef(pkgID, "B")
	c := graph.ChildRef(pkgID, "C")
	d := graph.ChildRef(pkgID, "D")
	for _, node := range []testNode{
		{ID: pkgID, Kind: graph.NodePackage, Name: "cycle"},
		{ID: a, Kind: graph.NodeFunction, Name: "A", Parent: pkgID},
		{ID: b, Kind: graph.NodeFunction, Name: "B", Parent: pkgID},
		{ID: c, Kind: graph.NodeFunction, Name: "C", Parent: pkgID},
		{ID: d, Kind: graph.NodeFunction, Name: "D", Parent: pkgID},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	for index, endpoints := range [][2]graph.SymbolRef{{a, b}, {b, c}, {c, a}, {a, d}, {d, c}} {
		if err := addTestEdge(g, testEdge{
			From: endpoints[0], To: endpoints[1], Kind: graph.EdgeCalls,
			Evidence: []graph.Location{{File: "cycle.go", Offset: index}},
		}); err != nil {
			t.Fatal(err)
		}
	}

	paths := query.DependencyPaths(g, a, c)
	if len(paths) != 2 {
		t.Fatalf("DependencyPaths(A, C) = %#v, want branches through B and D", paths)
	}
	for _, path := range paths {
		assertSimpleDependencyPath(t, a, c, path)
	}
	if got := query.DependencyPaths(g, a, a); len(got) != 0 {
		t.Fatalf("DependencyPaths(A, A) = %#v, want no identity or cyclic paths", got)
	}
}

func TestDependencyPathsRejectInvalidEndpointsAndProjection(t *testing.T) {
	g := graph.New()
	pkgA := graph.PackageRef("example.com/a")
	pkgB := graph.PackageRef("example.com/b")
	typeA := graph.ChildRef(pkgA, "Service")
	typeB := graph.ChildRef(pkgB, "Repository")
	caller := graph.ChildRef(typeA, "Call")
	callee := graph.ChildRef(typeB, "Save")
	for _, node := range []testNode{
		{ID: pkgA, Kind: graph.NodePackage, Name: "a"},
		{ID: pkgB, Kind: graph.NodePackage, Name: "b"},
		{ID: typeA, Kind: graph.NodeStruct, Name: "Service", Parent: pkgA},
		{ID: typeB, Kind: graph.NodeInterface, Name: "Repository", Parent: pkgB},
		{ID: caller, Kind: graph.NodeFunction, Name: "Call", Parent: typeA},
		{ID: callee, Kind: graph.NodeFunction, Name: "Save", Parent: typeB},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	if err := addTestEdge(g, testEdge{
		From: caller, To: callee, Kind: graph.EdgeCalls,
		Evidence: []graph.Location{{File: "call.go"}},
	}); err != nil {
		t.Fatal(err)
	}

	if got := query.DependencyPaths(g, caller, callee); len(got) != 1 {
		t.Fatalf("exact function DependencyPaths = %#v, want one", got)
	}
	for _, paths := range [][]query.SemanticPath{
		query.DependencyPaths(nil, caller, callee),
		query.DependencyPaths(g, "missing", callee),
		query.DependencyPaths(g, caller, "missing"),
		query.DependencyPaths(g, caller, caller),
		query.DependencyPaths(g, typeA, caller),
		query.DependencyPaths(g, caller, typeA),
		query.DependencyPaths(g, typeA, typeB),
		query.DependencyPaths(g, pkgA, pkgB),
	} {
		if len(paths) != 0 {
			t.Errorf("invalid, structural, or projected DependencyPaths = %#v, want empty", paths)
		}
	}
}

func TestAnalyzerPipelineExplainsRunCreateToOrder(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), filepath.Join("..", "goanalyzer", "testdata", "project"), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	runCreate := graph.SymbolRef("example.com/shop/orders::RunCreate")
	create := graph.SymbolRef("example.com/shop/orders::Service::Create")
	repositorySave := graph.SymbolRef("example.com/shop/orders::Repository::Save")
	order := graph.SymbolRef("example.com/shop/orders::Order")
	want := []query.SemanticPath{
		{Steps: []query.SemanticStep{
			{From: runCreate, To: create, Kind: graph.EdgeCalls},
			{From: create, To: order, Kind: graph.EdgeAccepts},
		}},
		{Steps: []query.SemanticStep{
			{From: runCreate, To: create, Kind: graph.EdgeCalls},
			{From: create, To: repositorySave, Kind: graph.EdgeCalls},
			{From: repositorySave, To: order, Kind: graph.EdgeAccepts},
		}},
	}
	if got := query.DependencyPaths(g, runCreate, order); !reflect.DeepEqual(got, want) {
		t.Fatalf("DependencyPaths(RunCreate, Order) = %#v, want %#v", got, want)
	}
}

func assertSimpleDependencyPath(
	t *testing.T,
	from, to graph.SymbolRef,
	path query.SemanticPath,
) {
	t.Helper()
	if len(path.Steps) == 0 || path.Steps[0].From != from || path.Steps[len(path.Steps)-1].To != to {
		t.Fatalf("path does not connect %q to %q: %#v", from, to, path)
	}
	seen := map[graph.SymbolRef]bool{from: true}
	for index, step := range path.Steps {
		if index > 0 && path.Steps[index-1].To != step.From {
			t.Fatalf("disconnected path: %#v", path)
		}
		if seen[step.To] {
			t.Fatalf("path repeats %q: %#v", step.To, path)
		}
		seen[step.To] = true
	}
}
