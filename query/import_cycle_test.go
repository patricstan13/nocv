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

func TestWouldCreateImportCyclePreservesEverySimpleReversePath(t *testing.T) {
	g, ids := importCycleFixture(t)
	before := importGraphSnapshot(g)

	want := query.ImportCycleCheck{
		WouldCycle: true,
		Paths: []query.ImportPath{
			{Packages: []graph.SymbolRef{ids.a, ids.d}},
			{Packages: []graph.SymbolRef{ids.a, ids.b, ids.d}},
			{Packages: []graph.SymbolRef{ids.a, ids.c, ids.d}},
			{Packages: []graph.SymbolRef{ids.a, ids.b, ids.x, ids.d}},
			{Packages: []graph.SymbolRef{ids.a, ids.c, ids.x, ids.d}},
		},
	}
	got := query.WouldCreateImportCycle(g, ids.d, ids.a)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WouldCreateImportCycle(D, A) = %#v, want %#v", got, want)
	}
	if again := query.WouldCreateImportCycle(g, ids.d, ids.a); !reflect.DeepEqual(again, want) {
		t.Fatalf("second cycle check = %#v, want deterministic %#v", again, want)
	}
	if after := importGraphSnapshot(g); !reflect.DeepEqual(after, before) {
		t.Fatalf("cycle check mutated graph:\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func TestWouldCreateImportCycleIgnoresSemanticEdgesAndReportsNoPath(t *testing.T) {
	g, ids := importCycleFixture(t)

	// D semantically calls A, but no stored import path leads from D to A.
	if got := query.WouldCreateImportCycle(g, ids.a, ids.d); got.WouldCycle || len(got.Paths) != 0 {
		t.Fatalf("semantic edge affected import cycle result: %#v", got)
	}
}

func TestWouldCreateImportCycleIsSafeOnExistingCycle(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.SymbolRef{"p", "q", "r"} {
		if err := addTestNode(g, testNode{ID: id, Kind: graph.NodePackage, Name: string(id)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, endpoints := range [][2]graph.SymbolRef{{"p", "q"}, {"q", "p"}, {"q", "r"}} {
		if err := addTestEdge(g, testEdge{From: endpoints[0], To: endpoints[1], Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "fixture.go"}}}); err != nil {
			t.Fatal(err)
		}
	}

	want := query.ImportCycleCheck{WouldCycle: true, Paths: []query.ImportPath{{Packages: []graph.SymbolRef{"p", "q", "r"}}}}
	if got := query.WouldCreateImportCycle(g, "r", "p"); !reflect.DeepEqual(got, want) {
		t.Fatalf("cycle-safe result = %#v, want %#v", got, want)
	}
}

func TestWouldCreateImportCycleValidatesEndpointsAndTreatsSelfImportAsCycle(t *testing.T) {
	g, ids := importCycleFixture(t)
	for _, got := range []query.ImportCycleCheck{
		query.WouldCreateImportCycle(nil, ids.a, ids.d),
		query.WouldCreateImportCycle(g, "missing", ids.d),
		query.WouldCreateImportCycle(g, ids.a, "missing"),
		query.WouldCreateImportCycle(g, ids.aFunction, ids.d),
		query.WouldCreateImportCycle(g, ids.a, ids.dFunction),
	} {
		if got.WouldCycle || len(got.Paths) != 0 {
			t.Errorf("invalid endpoint result = %#v, want zero result", got)
		}
	}

	wantSelf := query.ImportCycleCheck{
		WouldCycle: true,
		Paths:      []query.ImportPath{{Packages: []graph.SymbolRef{ids.a}}},
	}
	if got := query.WouldCreateImportCycle(g, ids.a, ids.a); !reflect.DeepEqual(got, wantSelf) {
		t.Fatalf("self-import result = %#v, want %#v", got, wantSelf)
	}
}

func TestWouldCreateImportCycleFindsNOCVReversePaths(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), filepath.Join(".."), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	result := query.WouldCreateImportCycle(g, "nocv/graph", "nocv/cmd/nocv")
	if !result.WouldCycle {
		t.Fatal("graph -> cmd/nocv was not identified as cycle-producing")
	}
	want := []graph.SymbolRef{"nocv/cmd/nocv", "nocv/query", "nocv/graph"}
	for _, path := range result.Paths {
		if reflect.DeepEqual(path.Packages, want) {
			return
		}
	}
	t.Fatalf("NOCV reverse paths = %#v, want path %#v", result.Paths, want)
}

type importCycleIDs struct {
	a         graph.SymbolRef
	b         graph.SymbolRef
	c         graph.SymbolRef
	d         graph.SymbolRef
	x         graph.SymbolRef
	aFunction graph.SymbolRef
	dFunction graph.SymbolRef
}

func importCycleFixture(t *testing.T) (*graph.Graph, importCycleIDs) {
	t.Helper()
	ids := importCycleIDs{
		a:         "a",
		b:         "b",
		c:         "c",
		d:         "d",
		x:         "x",
		aFunction: "a::Run",
		dFunction: "d::Run",
	}
	g := graph.New()
	for _, id := range []graph.SymbolRef{ids.a, ids.b, ids.c, ids.d, ids.x} {
		if err := addTestNode(g, testNode{ID: id, Kind: graph.NodePackage, Name: string(id)}); err != nil {
			t.Fatalf("AddNode(%q): %v", id, err)
		}
	}
	for _, node := range []testNode{
		{ID: ids.aFunction, Kind: graph.NodeFunction, Name: "Run", Parent: ids.a},
		{ID: ids.dFunction, Kind: graph.NodeFunction, Name: "Run", Parent: ids.d},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	imports := [][2]graph.SymbolRef{
		{ids.a, ids.d},
		{ids.a, ids.b},
		{ids.a, ids.c},
		{ids.b, ids.d},
		{ids.c, ids.d},
		{ids.b, ids.x},
		{ids.c, ids.x},
		{ids.x, ids.d},
		{ids.a, ids.d}, // repeated fact exercises defensive path deduplication
	}
	for index, endpoints := range imports {
		if err := addTestEdge(g, testEdge{
			From: endpoints[0], To: endpoints[1], Kind: graph.EdgeImports,
			Evidence: []graph.Location{{File: "fixture.go", Offset: index}},
		}); err != nil {
			t.Fatalf("AddEdge(imports, %q -> %q): %v", endpoints[0], endpoints[1], err)
		}
	}
	if err := addTestEdge(g, testEdge{
		From: ids.dFunction, To: ids.aFunction, Kind: graph.EdgeCalls,
		Evidence: []graph.Location{{File: "fixture.go", Offset: 100}},
	}); err != nil {
		t.Fatal(err)
	}
	return g, ids
}
