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

func TestCheckForbiddenPackageImportReturnsDirectEvidenceDefensively(t *testing.T) {
	g, ids := forbiddenImportFixture(t)
	beforeGraph := importGraphSnapshot(g)
	beforeImports := query.DirectImports(g, ids.a)
	beforeCycle := query.WouldCreateImportCycle(g, ids.d, ids.a)
	beforeSemantic := query.DirectDependencies(g, ids.dFunction)

	want := query.PackageImportViolation{
		From: ids.a,
		To:   ids.b,
		Evidence: []graph.Location{
			{File: "a.go", Offset: 10},
			{File: "other.go", Offset: 20},
		},
	}
	violation, exists := query.CheckForbiddenPackageImport(g, ids.a, ids.b)
	if !exists || !reflect.DeepEqual(violation, want) {
		t.Fatalf("CheckForbiddenPackageImport(A, B) = (%#v, %v), want (%#v, true)", violation, exists, want)
	}

	violation.Evidence[0].Offset = 999
	if again, ok := query.CheckForbiddenPackageImport(g, ids.a, ids.b); !ok || !reflect.DeepEqual(again, want) {
		t.Fatalf("second forbidden import check = (%#v, %v), want (%#v, true)", again, ok, want)
	}
	if after := importGraphSnapshot(g); !reflect.DeepEqual(after, beforeGraph) {
		t.Fatalf("forbidden import check mutated graph:\nbefore: %#v\nafter:  %#v", beforeGraph, after)
	}
	if got := query.DirectImports(g, ids.a); !reflect.DeepEqual(got, beforeImports) {
		t.Fatalf("DirectImports changed: got %#v, want %#v", got, beforeImports)
	}
	if got := query.WouldCreateImportCycle(g, ids.d, ids.a); !reflect.DeepEqual(got, beforeCycle) {
		t.Fatalf("WouldCreateImportCycle changed: got %#v, want %#v", got, beforeCycle)
	}
	if got := query.DirectDependencies(g, ids.dFunction); !reflect.DeepEqual(got, beforeSemantic) {
		t.Fatalf("semantic navigation changed: got %#v, want %#v", got, beforeSemantic)
	}
}

func TestCheckForbiddenPackageImportIsDirectOnlyAndIgnoresSemanticEdges(t *testing.T) {
	g, ids := forbiddenImportFixture(t)
	for _, endpoints := range [][2]graph.SymbolRef{
		{ids.a, ids.c}, // A reaches C in two import hops.
		{ids.a, ids.d}, // A reaches D in three import hops.
		{ids.d, ids.a}, // D has only a semantic Calls edge to A.
	} {
		if violation, exists := query.CheckForbiddenPackageImport(g, endpoints[0], endpoints[1]); exists {
			t.Errorf("CheckForbiddenPackageImport(%q, %q) = %#v, true; want no direct violation", endpoints[0], endpoints[1], violation)
		}
	}
}

func TestCheckForbiddenPackageImportValidatesExactPackagesAndSelfEdge(t *testing.T) {
	g, ids := forbiddenImportFixture(t)
	for _, endpoints := range [][2]graph.SymbolRef{
		{ids.a, ids.a},
		{"missing", ids.b},
		{ids.a, "missing"},
		{ids.aFunction, ids.b},
		{ids.a, ids.dFunction},
	} {
		if violation, exists := query.CheckForbiddenPackageImport(g, endpoints[0], endpoints[1]); exists {
			t.Errorf("invalid/absent direct import %q -> %q = %#v, true", endpoints[0], endpoints[1], violation)
		}
	}
	if violation, exists := query.CheckForbiddenPackageImport(nil, ids.a, ids.b); exists {
		t.Errorf("nil graph = %#v, true; want no violation", violation)
	}

	if err := addTestEdge(g, testEdge{
		From: ids.a, To: ids.a, Kind: graph.EdgeImports,
		Evidence: []graph.Location{{File: "malformed.go", Offset: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	if violation, exists := query.CheckForbiddenPackageImport(g, ids.a, ids.a); !exists || len(violation.Evidence) != 1 {
		t.Fatalf("stored self-import = (%#v, %v), want one direct violation", violation, exists)
	}
}

func TestCheckForbiddenPackageImportUsesAnalyzerImportFacts(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), filepath.Join("..", "goanalyzer", "testdata", "imports"), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	app := graph.SymbolRef("example.com/imports/app")
	for _, target := range []graph.SymbolRef{
		"example.com/imports/plugin",     // blank import
		"example.com/imports/repository", // aliased import
		"example.com/imports/helpers",    // dot import
	} {
		violation, exists := query.CheckForbiddenPackageImport(g, app, target)
		if !exists || len(violation.Evidence) != 1 {
			t.Errorf("analyzer import %q -> %q = (%#v, %v), want one-evidence violation", app, target, violation, exists)
		}
	}
	service, exists := query.CheckForbiddenPackageImport(g, app, "example.com/imports/service")
	if !exists || len(service.Evidence) != 2 {
		t.Fatalf("repeated service import = (%#v, %v), want one violation with two evidence locations", service, exists)
	}
}

type forbiddenImportIDs struct {
	a         graph.SymbolRef
	b         graph.SymbolRef
	c         graph.SymbolRef
	d         graph.SymbolRef
	aFunction graph.SymbolRef
	dFunction graph.SymbolRef
}

func forbiddenImportFixture(t *testing.T) (*graph.Graph, forbiddenImportIDs) {
	t.Helper()
	ids := forbiddenImportIDs{
		a:         "a",
		b:         "b",
		c:         "c",
		d:         "d",
		aFunction: "a::Run",
		dFunction: "d::Run",
	}
	g := graph.New()
	for _, id := range []graph.SymbolRef{ids.a, ids.b, ids.c, ids.d} {
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
	for _, edge := range []testEdge{
		{From: ids.a, To: ids.b, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "a.go", Offset: 10}}},
		{From: ids.a, To: ids.b, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "other.go", Offset: 20}}},
		{From: ids.b, To: ids.c, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "b.go", Offset: 10}}},
		{From: ids.c, To: ids.d, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "c.go", Offset: 10}}},
		{From: ids.dFunction, To: ids.aFunction, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "d.go", Offset: 10}}},
	} {
		if err := addTestEdge(g, edge); err != nil {
			t.Fatalf("AddEdge(%s, %q -> %q): %v", edge.Kind, edge.From, edge.To, err)
		}
	}
	return g, ids
}
