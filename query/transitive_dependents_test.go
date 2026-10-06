package query_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/internal/testutil"
	"nocv/query"
)

func TestTransitiveDependentsPreserveAllSimplePathsAndDeterministicOrdering(t *testing.T) {
	g, ids := branchingDependentsFixture(t)
	before := outgoingSnapshot(g)

	got := query.TransitiveDependents(g, ids.changed)
	want := []query.TransitiveDependent{
		{
			ID: ids.a,
			Paths: []query.SemanticPath{
				{Steps: []query.SemanticStep{{From: ids.a, To: ids.changed, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed}}},
				{Steps: []query.SemanticStep{{From: ids.a, To: ids.changed, Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed}}},
				{Steps: []query.SemanticStep{
					{From: ids.a, To: ids.b, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
					{From: ids.b, To: ids.changed, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed},
				}},
				{Steps: []query.SemanticStep{
					{From: ids.a, To: ids.c, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
					{From: ids.c, To: ids.changed, Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed},
				}},
			},
		},
		{ID: ids.b, Paths: []query.SemanticPath{{Steps: []query.SemanticStep{{From: ids.b, To: ids.changed, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed}}}}},
		{ID: ids.c, Paths: []query.SemanticPath{{Steps: []query.SemanticStep{{From: ids.c, To: ids.changed, Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed}}}}},
		{
			ID: ids.d,
			Paths: []query.SemanticPath{
				{Steps: []query.SemanticStep{
					{From: ids.d, To: ids.a, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
					{From: ids.a, To: ids.changed, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed},
				}},
				{Steps: []query.SemanticStep{
					{From: ids.d, To: ids.a, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
					{From: ids.a, To: ids.changed, Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed},
				}},
				{Steps: []query.SemanticStep{
					{From: ids.d, To: ids.a, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
					{From: ids.a, To: ids.b, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
					{From: ids.b, To: ids.changed, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed},
				}},
				{Steps: []query.SemanticStep{
					{From: ids.d, To: ids.a, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
					{From: ids.a, To: ids.c, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
					{From: ids.c, To: ids.changed, Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed},
				}},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TransitiveDependents(Changed) = %#v, want %#v", got, want)
	}
	if again := query.TransitiveDependents(g, ids.changed); !reflect.DeepEqual(again, want) {
		t.Fatalf("second TransitiveDependents(Changed) = %#v, want deterministic %#v", again, want)
	}
	if after := outgoingSnapshot(g); !reflect.DeepEqual(after, before) {
		t.Fatalf("transitive dependents query mutated graph edges:\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func TestTransitiveDependentsTerminatesCyclesWithPathLocalState(t *testing.T) {
	g := graph.New()
	pkgID := graph.PackageRef("example.com/cycle")
	a := graph.ChildRef(pkgID, "A")
	b := graph.ChildRef(pkgID, "B")
	c := graph.ChildRef(pkgID, "C")
	for _, node := range []testNode{
		{ID: pkgID, Kind: graph.NodePackage, Name: "cycle"},
		{ID: a, Kind: graph.NodeFunction, Name: "A", Parent: pkgID},
		{ID: b, Kind: graph.NodeFunction, Name: "B", Parent: pkgID},
		{ID: c, Kind: graph.NodeFunction, Name: "C", Parent: pkgID},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	for offset, endpoints := range [][2]graph.SymbolRef{{a, b}, {b, c}, {c, a}} {
		if err := addTestEdge(g, testEdge{
			From:     endpoints[0],
			To:       endpoints[1],
			Kind:     graph.EdgeCalls,
			Evidence: []graph.Location{{File: "cycle.go", Offset: offset}},
		}); err != nil {
			t.Fatal(err)
		}
	}

	want := []query.TransitiveDependent{
		{ID: a, Paths: []query.SemanticPath{{Steps: []query.SemanticStep{{From: a, To: b, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed}, {From: b, To: c, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed}}}}},
		{ID: b, Paths: []query.SemanticPath{{Steps: []query.SemanticStep{{From: b, To: c, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed}}}}},
	}
	got := query.TransitiveDependents(g, c)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TransitiveDependents(C) = %#v, want %#v", got, want)
	}
	for _, result := range got {
		for _, path := range result.Paths {
			assertSimpleDependentPath(t, c, path)
		}
	}
}

func TestDirectAndTransitiveDependentsSpecifyTraversalDepth(t *testing.T) {
	g := graph.New()
	pkg := graph.PackageRef("example.com/depth")
	a := graph.ChildRef(pkg, "A")
	b := graph.ChildRef(pkg, "B")
	c := graph.ChildRef(pkg, "C")
	for _, node := range []testNode{
		{ID: pkg, Kind: graph.NodePackage, Name: "depth"},
		{ID: a, Kind: graph.NodeFunction, Name: "A", Parent: pkg},
		{ID: b, Kind: graph.NodeFunction, Name: "B", Parent: pkg},
		{ID: c, Kind: graph.NodeFunction, Name: "C", Parent: pkg},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	for offset, endpoints := range [][2]graph.SymbolRef{{b, a}, {c, b}} {
		if err := addTestEdge(g, testEdge{
			From: endpoints[0], To: endpoints[1], Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed,
			Evidence: []graph.Location{{File: "depth.go", Offset: offset}},
		}); err != nil {
			t.Fatal(err)
		}
	}

	direct := query.DirectDependents(g, a)
	if len(direct) != 1 || direct[0].From != b {
		t.Fatalf("DirectDependents(A) = %#v, want only B", direct)
	}
	transitive := query.TransitiveDependents(g, a)
	if len(transitive) != 2 || transitive[0].ID != b || transitive[1].ID != c {
		t.Fatalf("TransitiveDependents(A) = %#v, want B and C", transitive)
	}
}

func TestTransitiveDependentsTraversesEveryDirectSemanticKind(t *testing.T) {
	g := graph.New()
	pkgID := graph.PackageRef("example.com/relationships")
	base := graph.ChildRef(pkgID, "Base")
	child := graph.ChildRef(pkgID, "Child")
	contract := graph.ChildRef(pkgID, "Contract")
	implementation := graph.ChildRef(pkgID, "Implementation")
	acceptor := graph.ChildRef(pkgID, "Accept")
	returner := graph.ChildRef(pkgID, "Return")
	callee := graph.ChildRef(pkgID, "Callee")
	caller := graph.ChildRef(pkgID, "Caller")
	contractMethod := graph.ChildRef(contract, "Run")
	implementationMethod := graph.ChildRef(implementation, "Run")
	for _, node := range []testNode{
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
		if err := addTestNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	edges := []testEdge{
		{From: child, To: base, Kind: graph.EdgeEmbeds, Certainty: graph.RelationshipConfirmed},
		{From: implementation, To: contract, Kind: graph.EdgeImplements, Certainty: graph.RelationshipConfirmed},
		{From: implementationMethod, To: contractMethod, Kind: graph.EdgeImplements, Certainty: graph.RelationshipConfirmed},
		{From: acceptor, To: contract, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed},
		{From: returner, To: contract, Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed},
		{From: caller, To: callee, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "relationships.go", Offset: index}}
		if err := addTestEdge(g, edges[index]); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		changed graph.SymbolRef
		from    graph.SymbolRef
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
		results := query.TransitiveDependents(g, test.changed)
		if !hasOneStepDependent(results, test.from, test.changed, test.kind) {
			t.Errorf("TransitiveDependents(%q) lacks %q -%s-> %q: %#v", test.changed, test.from, test.kind, test.changed, results)
		}
	}
}

func TestTransitiveDependentsExcludesStructuralAndProjectedRelationships(t *testing.T) {
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
		From: caller, To: callee, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed,
		Evidence: []graph.Location{{File: "call.go"}},
	}); err != nil {
		t.Fatal(err)
	}

	if got := query.TransitiveDependents(g, typeA); len(got) != 0 {
		t.Fatalf("structural children affected parent: %#v", got)
	}
	if got := query.TransitiveDependents(g, pkgB); len(got) != 0 {
		t.Fatalf("projected call dependency affected package: %#v", got)
	}
	if got := query.TransitiveDependents(g, callee); len(got) != 1 || got[0].ID != caller {
		t.Fatalf("TransitiveDependents(callee) = %#v, want only concrete caller", got)
	}
}

func TestTransitiveDependentsHandlesNilAndMissingNodes(t *testing.T) {
	g, ids := branchingDependentsFixture(t)
	if got := query.TransitiveDependents(nil, ids.changed); len(got) != 0 {
		t.Fatalf("TransitiveDependents(nil) = %#v, want empty", got)
	}
	if got := query.TransitiveDependents(g, "missing"); len(got) != 0 {
		t.Fatalf("TransitiveDependents(missing) = %#v, want empty", got)
	}
}

func TestAnalyzerPipelineProvidesTransitiveDependentPaths(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), testutil.GoProjectDir(t), "./orders", "./logging")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	results := query.TransitiveDependents(g, "example.com/shop/orders::Order")
	runCreateID := graph.SymbolRef("example.com/shop/orders::RunCreate")
	var runCreate *query.TransitiveDependent
	for index := range results {
		if results[index].ID == runCreateID {
			runCreate = &results[index]
			break
		}
	}
	if runCreate == nil {
		t.Fatalf("Order transitive dependents lack RunCreate: %#v", results)
	}
	if len(runCreate.Paths) != 2 {
		t.Fatalf("RunCreate transitive dependent paths = %#v, want call chains of lengths two and three", runCreate.Paths)
	}
	if len(runCreate.Paths[0].Steps) != 2 || len(runCreate.Paths[1].Steps) != 3 {
		t.Fatalf("RunCreate path lengths = %d and %d, want 2 and 3", len(runCreate.Paths[0].Steps), len(runCreate.Paths[1].Steps))
	}
}

func TestSelfAnalysisAttributesTransitiveDependentsClosureCall(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), filepath.Join(".."), "./...")
	if err != nil {
		t.Fatalf("Load(NOCV): %v", err)
	}

	dependentsID := graph.SymbolRef("nocv/query::TransitiveDependents")
	closureTarget := graph.SymbolRef("nocv/query::semanticPathLess")
	dependencies := query.DirectDependencies(g, dependentsID)
	foundCall := false
	for _, relationship := range dependencies {
		if relationship.Kind == graph.EdgeCalls && relationship.To == closureTarget {
			foundCall = len(relationship.Evidence) == 1 && relationship.Evidence[0].File != ""
			break
		}
	}
	if !foundCall {
		t.Fatalf("TransitiveDependents direct dependencies lack closure call to semanticPathLess: %#v", dependencies)
	}

	if !hasOneStepDependent(query.TransitiveDependents(g, closureTarget), dependentsID, closureTarget, graph.EdgeCalls) {
		t.Fatal("transitive dependents query does not consume TransitiveDependents -> semanticPathLess closure call")
	}

	packagePaths := query.PackageDependencyPaths(g, "nocv/cmd/nocv", "nocv/query")
	if len(packagePaths) == 0 {
		t.Fatal("self-analysis lacks cmd/nocv -> query package dependency paths")
	}
}

type branchingDependentsIDs struct {
	changed graph.SymbolRef
	a       graph.SymbolRef
	b       graph.SymbolRef
	c       graph.SymbolRef
	d       graph.SymbolRef
}

func branchingDependentsFixture(t *testing.T) (*graph.Graph, branchingDependentsIDs) {
	t.Helper()
	ids := branchingDependentsIDs{
		changed: "example.com/dependents::Changed",
		a:       "example.com/dependents::A",
		b:       "example.com/dependents::B",
		c:       "example.com/dependents::C",
		d:       "example.com/dependents::D",
	}
	g := graph.New()
	pkgID := graph.PackageRef("example.com/dependents")
	for _, node := range []testNode{
		{ID: pkgID, Kind: graph.NodePackage, Name: "dependents"},
		{ID: ids.changed, Kind: graph.NodeInterface, Name: "Changed", Parent: pkgID},
		{ID: ids.a, Kind: graph.NodeFunction, Name: "A", Parent: pkgID},
		{ID: ids.b, Kind: graph.NodeFunction, Name: "B", Parent: pkgID},
		{ID: ids.c, Kind: graph.NodeFunction, Name: "C", Parent: pkgID},
		{ID: ids.d, Kind: graph.NodeFunction, Name: "D", Parent: pkgID},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	edges := []testEdge{
		{From: ids.d, To: ids.a, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: ids.a, To: ids.c, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: ids.a, To: ids.changed, Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed},
		{From: ids.c, To: ids.changed, Kind: graph.EdgeReturns, Certainty: graph.RelationshipConfirmed},
		{From: ids.a, To: ids.b, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: ids.b, To: ids.changed, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed},
		{From: ids.a, To: ids.changed, Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed},
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "dependents.go", Offset: index}}
		if err := addTestEdge(g, edges[index]); err != nil {
			t.Fatal(err)
		}
	}
	return g, ids
}

func assertSimpleDependentPath(t *testing.T, changed graph.SymbolRef, path query.SemanticPath) {
	t.Helper()
	if len(path.Steps) == 0 {
		t.Fatal("transitive dependent path has no steps")
	}
	seen := make(map[graph.SymbolRef]bool)
	for index, step := range path.Steps {
		if index > 0 && path.Steps[index-1].To != step.From {
			t.Fatalf("disconnected transitive dependent path: %#v", path)
		}
		if seen[step.From] {
			t.Fatalf("transitive dependent path repeats %q: %#v", step.From, path)
		}
		seen[step.From] = true
	}
	last := path.Steps[len(path.Steps)-1].To
	if seen[last] || last != changed {
		t.Fatalf("transitive dependent path repeats or does not end at %q: %#v", changed, path)
	}
}

func hasOneStepDependent(
	results []query.TransitiveDependent,
	from, to graph.SymbolRef,
	kind graph.EdgeKind,
) bool {
	for _, result := range results {
		if result.ID != from {
			continue
		}
		for _, path := range result.Paths {
			if len(path.Steps) == 1 && path.Steps[0] == (query.SemanticStep{From: from, To: to, Kind: kind, Certainty: graph.RelationshipConfirmed}) {
				return true
			}
		}
	}
	return false
}
