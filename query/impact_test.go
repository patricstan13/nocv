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

func TestImpactPreservesAllSimplePathsAndDeterministicOrdering(t *testing.T) {
	g, ids := branchingImpactFixture(t)
	before := outgoingSnapshot(g)

	got := query.Impact(g, ids.changed)
	want := []query.ImpactResult{
		{
			ID: ids.a,
			Paths: []query.ImpactPath{
				{Steps: []query.ImpactStep{{From: ids.a, To: ids.changed, Kind: graph.EdgeAccepts}}},
				{Steps: []query.ImpactStep{{From: ids.a, To: ids.changed, Kind: graph.EdgeReturns}}},
				{Steps: []query.ImpactStep{
					{From: ids.a, To: ids.b, Kind: graph.EdgeCalls},
					{From: ids.b, To: ids.changed, Kind: graph.EdgeAccepts},
				}},
				{Steps: []query.ImpactStep{
					{From: ids.a, To: ids.c, Kind: graph.EdgeCalls},
					{From: ids.c, To: ids.changed, Kind: graph.EdgeReturns},
				}},
			},
		},
		{ID: ids.b, Paths: []query.ImpactPath{{Steps: []query.ImpactStep{{From: ids.b, To: ids.changed, Kind: graph.EdgeAccepts}}}}},
		{ID: ids.c, Paths: []query.ImpactPath{{Steps: []query.ImpactStep{{From: ids.c, To: ids.changed, Kind: graph.EdgeReturns}}}}},
		{
			ID: ids.d,
			Paths: []query.ImpactPath{
				{Steps: []query.ImpactStep{
					{From: ids.d, To: ids.a, Kind: graph.EdgeCalls},
					{From: ids.a, To: ids.changed, Kind: graph.EdgeAccepts},
				}},
				{Steps: []query.ImpactStep{
					{From: ids.d, To: ids.a, Kind: graph.EdgeCalls},
					{From: ids.a, To: ids.changed, Kind: graph.EdgeReturns},
				}},
				{Steps: []query.ImpactStep{
					{From: ids.d, To: ids.a, Kind: graph.EdgeCalls},
					{From: ids.a, To: ids.b, Kind: graph.EdgeCalls},
					{From: ids.b, To: ids.changed, Kind: graph.EdgeAccepts},
				}},
				{Steps: []query.ImpactStep{
					{From: ids.d, To: ids.a, Kind: graph.EdgeCalls},
					{From: ids.a, To: ids.c, Kind: graph.EdgeCalls},
					{From: ids.c, To: ids.changed, Kind: graph.EdgeReturns},
				}},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Impact(Changed) = %#v, want %#v", got, want)
	}
	if again := query.Impact(g, ids.changed); !reflect.DeepEqual(again, want) {
		t.Fatalf("second Impact(Changed) = %#v, want deterministic %#v", again, want)
	}
	if after := outgoingSnapshot(g); !reflect.DeepEqual(after, before) {
		t.Fatalf("impact query mutated graph edges:\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func TestImpactTerminatesCyclesWithPathLocalState(t *testing.T) {
	g := graph.New()
	pkgID := graph.PackageID("example.com/cycle")
	a := graph.ChildID(pkgID, "A")
	b := graph.ChildID(pkgID, "B")
	c := graph.ChildID(pkgID, "C")
	for _, node := range []graph.Node{
		{ID: pkgID, Kind: graph.NodePackage, Name: "cycle"},
		{ID: a, Kind: graph.NodeFunction, Name: "A", Parent: pkgID},
		{ID: b, Kind: graph.NodeFunction, Name: "B", Parent: pkgID},
		{ID: c, Kind: graph.NodeFunction, Name: "C", Parent: pkgID},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatal(err)
		}
	}
	for offset, endpoints := range [][2]graph.SymbolID{{a, b}, {b, c}, {c, a}} {
		if err := g.AddEdge(graph.Edge{
			From:     endpoints[0],
			To:       endpoints[1],
			Kind:     graph.EdgeCalls,
			Evidence: []graph.Location{{File: "cycle.go", Offset: offset}},
		}); err != nil {
			t.Fatal(err)
		}
	}

	want := []query.ImpactResult{
		{ID: a, Paths: []query.ImpactPath{{Steps: []query.ImpactStep{{From: a, To: b, Kind: graph.EdgeCalls}, {From: b, To: c, Kind: graph.EdgeCalls}}}}},
		{ID: b, Paths: []query.ImpactPath{{Steps: []query.ImpactStep{{From: b, To: c, Kind: graph.EdgeCalls}}}}},
	}
	got := query.Impact(g, c)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Impact(C) = %#v, want %#v", got, want)
	}
	for _, result := range got {
		for _, path := range result.Paths {
			assertSimpleImpactPath(t, c, path)
		}
	}
}

func TestImpactTraversesEveryDirectSemanticKind(t *testing.T) {
	g := graph.New()
	pkgID := graph.PackageID("example.com/relationships")
	base := graph.ChildID(pkgID, "Base")
	child := graph.ChildID(pkgID, "Child")
	contract := graph.ChildID(pkgID, "Contract")
	implementation := graph.ChildID(pkgID, "Implementation")
	acceptor := graph.ChildID(pkgID, "Accept")
	returner := graph.ChildID(pkgID, "Return")
	callee := graph.ChildID(pkgID, "Callee")
	caller := graph.ChildID(pkgID, "Caller")
	contractMethod := graph.ChildID(contract, "Run")
	implementationMethod := graph.ChildID(implementation, "Run")
	for _, node := range []graph.Node{
		{ID: pkgID, Kind: graph.NodePackage, Name: "relationships"},
		{ID: base, Kind: graph.NodeStruct, Name: "Base", Parent: pkgID},
		{ID: child, Kind: graph.NodeStruct, Name: "Child", Parent: pkgID},
		{ID: contract, Kind: graph.NodeInterface, Name: "Contract", Parent: pkgID},
		{ID: implementation, Kind: graph.NodeStruct, Name: "Implementation", Parent: pkgID},
		{ID: acceptor, Kind: graph.NodeFunction, Name: "Accept", Parent: pkgID},
		{ID: returner, Kind: graph.NodeFunction, Name: "Return", Parent: pkgID},
		{ID: callee, Kind: graph.NodeFunction, Name: "Callee", Parent: pkgID},
		{ID: caller, Kind: graph.NodeFunction, Name: "Caller", Parent: pkgID},
		{ID: contractMethod, Kind: graph.NodeFunction, Name: "Run", Parent: contract},
		{ID: implementationMethod, Kind: graph.NodeFunction, Name: "Run", Parent: implementation},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatal(err)
		}
	}
	edges := []graph.Edge{
		{From: child, To: base, Kind: graph.EdgeEmbeds},
		{From: implementation, To: contract, Kind: graph.EdgeImplements},
		{From: implementationMethod, To: contractMethod, Kind: graph.EdgeImplements},
		{From: acceptor, To: contract, Kind: graph.EdgeAccepts},
		{From: returner, To: contract, Kind: graph.EdgeReturns},
		{From: caller, To: callee, Kind: graph.EdgeCalls},
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "relationships.go", Offset: index}}
		if err := g.AddEdge(edges[index]); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		changed graph.SymbolID
		from    graph.SymbolID
		kind    graph.EdgeKind
	}{
		{changed: base, from: child, kind: graph.EdgeEmbeds},
		{changed: contract, from: implementation, kind: graph.EdgeImplements},
		{changed: contract, from: acceptor, kind: graph.EdgeAccepts},
		{changed: contract, from: returner, kind: graph.EdgeReturns},
		{changed: contractMethod, from: implementationMethod, kind: graph.EdgeImplements},
		{changed: callee, from: caller, kind: graph.EdgeCalls},
	}
	for _, test := range tests {
		results := query.Impact(g, test.changed)
		if !hasOneStepImpact(results, test.from, test.changed, test.kind) {
			t.Errorf("Impact(%q) lacks %q -%s-> %q: %#v", test.changed, test.from, test.kind, test.changed, results)
		}
	}
}

func TestImpactExcludesStructuralAndProjectedRelationships(t *testing.T) {
	g := graph.New()
	pkgA := graph.PackageID("example.com/a")
	pkgB := graph.PackageID("example.com/b")
	typeA := graph.ChildID(pkgA, "Service")
	typeB := graph.ChildID(pkgB, "Repository")
	caller := graph.ChildID(typeA, "Call")
	callee := graph.ChildID(typeB, "Save")
	for _, node := range []graph.Node{
		{ID: pkgA, Kind: graph.NodePackage, Name: "a"},
		{ID: pkgB, Kind: graph.NodePackage, Name: "b"},
		{ID: typeA, Kind: graph.NodeStruct, Name: "Service", Parent: pkgA},
		{ID: typeB, Kind: graph.NodeInterface, Name: "Repository", Parent: pkgB},
		{ID: caller, Kind: graph.NodeFunction, Name: "Call", Parent: typeA},
		{ID: callee, Kind: graph.NodeFunction, Name: "Save", Parent: typeB},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatal(err)
		}
	}
	if err := g.AddEdge(graph.Edge{
		From: caller, To: callee, Kind: graph.EdgeCalls,
		Evidence: []graph.Location{{File: "call.go"}},
	}); err != nil {
		t.Fatal(err)
	}

	if got := query.Impact(g, typeA); len(got) != 0 {
		t.Fatalf("structural children affected parent: %#v", got)
	}
	if got := query.Impact(g, pkgB); len(got) != 0 {
		t.Fatalf("projected call dependency affected package: %#v", got)
	}
	if got := query.Impact(g, callee); len(got) != 1 || got[0].ID != caller {
		t.Fatalf("Impact(callee) = %#v, want only concrete caller", got)
	}
}

func TestImpactHandlesNilAndMissingNodes(t *testing.T) {
	g, ids := branchingImpactFixture(t)
	if got := query.Impact(nil, ids.changed); len(got) != 0 {
		t.Fatalf("Impact(nil) = %#v, want empty", got)
	}
	if got := query.Impact(g, "missing"); len(got) != 0 {
		t.Fatalf("Impact(missing) = %#v, want empty", got)
	}
}

func TestAnalyzerPipelineProvidesTransitiveImpactPaths(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), filepath.Join("..", "goanalyzer", "testdata", "project"), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	results := query.Impact(g, "example.com/shop/orders::Order")
	runCreateID := graph.SymbolID("example.com/shop/orders::RunCreate")
	var runCreate *query.ImpactResult
	for index := range results {
		if results[index].ID == runCreateID {
			runCreate = &results[index]
			break
		}
	}
	if runCreate == nil {
		t.Fatalf("Order impact lacks RunCreate: %#v", results)
	}
	if len(runCreate.Paths) != 2 {
		t.Fatalf("RunCreate impact paths = %#v, want call chains of lengths two and three", runCreate.Paths)
	}
	if len(runCreate.Paths[0].Steps) != 2 || len(runCreate.Paths[1].Steps) != 3 {
		t.Fatalf("RunCreate path lengths = %d and %d, want 2 and 3", len(runCreate.Paths[0].Steps), len(runCreate.Paths[1].Steps))
	}
}

func TestSelfAnalysisAttributesImpactClosureCall(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), filepath.Join(".."), "./...")
	if err != nil {
		t.Fatalf("Load(NOCV): %v", err)
	}

	impactID := graph.SymbolID("nocv/query::Impact")
	directDependentsID := graph.SymbolID("nocv/query::DirectDependents")
	dependencies := query.DirectDependencies(g, impactID)
	foundCall := false
	for _, relationship := range dependencies {
		if relationship.Kind == graph.EdgeCalls && relationship.To == directDependentsID {
			foundCall = len(relationship.Evidence) == 1 && relationship.Evidence[0].File != ""
			break
		}
	}
	if !foundCall {
		t.Fatalf("Impact direct dependencies lack closure call to DirectDependents: %#v", dependencies)
	}

	if !hasOneStepImpact(query.Impact(g, directDependentsID), impactID, directDependentsID, graph.EdgeCalls) {
		t.Fatal("impact analysis does not consume Impact -> DirectDependents closure call")
	}

	packagePaths := query.PackageDependencyPaths(g, "nocv/cmd/nocv", "nocv/query")
	if len(packagePaths) == 0 {
		t.Fatal("self-analysis lacks cmd/nocv -> query package dependency paths")
	}
}

type branchingImpactIDs struct {
	changed graph.SymbolID
	a       graph.SymbolID
	b       graph.SymbolID
	c       graph.SymbolID
	d       graph.SymbolID
}

func branchingImpactFixture(t *testing.T) (*graph.Graph, branchingImpactIDs) {
	t.Helper()
	ids := branchingImpactIDs{
		changed: "example.com/impact::Changed",
		a:       "example.com/impact::A",
		b:       "example.com/impact::B",
		c:       "example.com/impact::C",
		d:       "example.com/impact::D",
	}
	g := graph.New()
	pkgID := graph.PackageID("example.com/impact")
	for _, node := range []graph.Node{
		{ID: pkgID, Kind: graph.NodePackage, Name: "impact"},
		{ID: ids.changed, Kind: graph.NodeInterface, Name: "Changed", Parent: pkgID},
		{ID: ids.a, Kind: graph.NodeFunction, Name: "A", Parent: pkgID},
		{ID: ids.b, Kind: graph.NodeFunction, Name: "B", Parent: pkgID},
		{ID: ids.c, Kind: graph.NodeFunction, Name: "C", Parent: pkgID},
		{ID: ids.d, Kind: graph.NodeFunction, Name: "D", Parent: pkgID},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatal(err)
		}
	}
	edges := []graph.Edge{
		{From: ids.d, To: ids.a, Kind: graph.EdgeCalls},
		{From: ids.a, To: ids.c, Kind: graph.EdgeCalls},
		{From: ids.a, To: ids.changed, Kind: graph.EdgeReturns},
		{From: ids.c, To: ids.changed, Kind: graph.EdgeReturns},
		{From: ids.a, To: ids.b, Kind: graph.EdgeCalls},
		{From: ids.b, To: ids.changed, Kind: graph.EdgeAccepts},
		{From: ids.a, To: ids.changed, Kind: graph.EdgeAccepts},
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "impact.go", Offset: index}}
		if err := g.AddEdge(edges[index]); err != nil {
			t.Fatal(err)
		}
	}
	return g, ids
}

func assertSimpleImpactPath(t *testing.T, changed graph.SymbolID, path query.ImpactPath) {
	t.Helper()
	if len(path.Steps) == 0 {
		t.Fatal("impact path has no steps")
	}
	seen := make(map[graph.SymbolID]bool)
	for index, step := range path.Steps {
		if index > 0 && path.Steps[index-1].To != step.From {
			t.Fatalf("disconnected impact path: %#v", path)
		}
		if seen[step.From] {
			t.Fatalf("impact path repeats %q: %#v", step.From, path)
		}
		seen[step.From] = true
	}
	last := path.Steps[len(path.Steps)-1].To
	if seen[last] || last != changed {
		t.Fatalf("impact path repeats or does not end at %q: %#v", changed, path)
	}
}

func hasOneStepImpact(
	results []query.ImpactResult,
	from, to graph.SymbolID,
	kind graph.EdgeKind,
) bool {
	for _, result := range results {
		if result.ID != from {
			continue
		}
		for _, path := range result.Paths {
			if len(path.Steps) == 1 && path.Steps[0] == (query.ImpactStep{From: from, To: to, Kind: kind}) {
				return true
			}
		}
	}
	return false
}
