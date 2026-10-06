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

func TestCheckForbiddenPackageDependencyPreservesProjectedRoutesAndEvidence(t *testing.T) {
	g, ids := forbiddenDependencyFixture(t)
	beforeGraph := importGraphSnapshot(g)
	beforePaths := query.PackageDependencyPaths(g, ids.app, ids.repository)
	beforeImport, beforeImportExists := query.CheckForbiddenPackageImport(g, ids.app, ids.plugin)
	beforeCycle := query.WouldCreateImportCycle(g, ids.plugin, ids.app)

	violation, exists := query.CheckForbiddenPackageDependency(g, ids.app, ids.repository)
	if !exists {
		t.Fatal("CheckForbiddenPackageDependency(app, repository) returned no violation")
	}
	if violation.From != ids.app || violation.To != ids.repository || !reflect.DeepEqual(violation.Paths, beforePaths) {
		t.Fatalf("dependency violation = %#v, want endpoints and paths %#v", violation, beforePaths)
	}
	wantRoutes := [][]graph.SymbolRef{
		{ids.app, ids.repository},
		{ids.app, ids.cache, ids.repository},
		{ids.app, ids.service, ids.repository},
	}
	if len(violation.Paths) != len(wantRoutes) {
		t.Fatalf("violation paths = %#v, want %d routes", violation.Paths, len(wantRoutes))
	}
	for index, want := range wantRoutes {
		if !reflect.DeepEqual(violation.Paths[index].Packages, want) {
			t.Errorf("route %d = %v, want %v", index, violation.Paths[index].Packages, want)
		}
	}
	if len(violation.Paths[2].Steps) != 2 || len(violation.Paths[2].Steps[0].Evidence) != 2 || len(violation.Paths[2].Steps[1].Evidence) != 2 {
		t.Fatalf("service route steps = %#v, want two hops with two grouped boundary facts each", violation.Paths[2].Steps)
	}
	if !containsSemanticKind(violation.Paths[2], graph.EdgeAccepts) {
		t.Fatalf("service route lacks non-Calls evidence: %#v", violation.Paths[2])
	}

	violation.Paths[0].Packages[0] = "mutated"
	violation.Paths[0].Steps[0].Evidence[0].From = "mutated"
	if again, ok := query.CheckForbiddenPackageDependency(g, ids.app, ids.repository); !ok || !reflect.DeepEqual(again.Paths, beforePaths) {
		t.Fatalf("second dependency check = (%#v, %v), want detached paths %#v", again, ok, beforePaths)
	}
	if after := importGraphSnapshot(g); !reflect.DeepEqual(after, beforeGraph) {
		t.Fatalf("forbidden dependency check mutated graph:\nbefore: %#v\nafter:  %#v", beforeGraph, after)
	}
	if got := query.PackageDependencyPaths(g, ids.app, ids.repository); !reflect.DeepEqual(got, beforePaths) {
		t.Fatalf("PackageDependencyPaths changed: got %#v, want %#v", got, beforePaths)
	}
	if got, ok := query.CheckForbiddenPackageImport(g, ids.app, ids.plugin); ok != beforeImportExists || !reflect.DeepEqual(got, beforeImport) {
		t.Fatalf("CheckForbiddenPackageImport changed: got (%#v, %v), want (%#v, %v)", got, ok, beforeImport, beforeImportExists)
	}
	if got := query.WouldCreateImportCycle(g, ids.plugin, ids.app); !reflect.DeepEqual(got, beforeCycle) {
		t.Fatalf("WouldCreateImportCycle changed: got %#v, want %#v", got, beforeCycle)
	}
}

func TestForbiddenImportAndSemanticDependencyRulesRemainDistinct(t *testing.T) {
	g, ids := forbiddenDependencyFixture(t)

	if _, exists := query.CheckForbiddenPackageDependency(g, ids.app, ids.repository); !exists {
		t.Fatal("semantic-only app -> repository did not produce dependency violation")
	}
	if violation, exists := query.CheckForbiddenPackageImport(g, ids.app, ids.repository); exists {
		t.Fatalf("semantic-only app -> repository produced import violation: %#v", violation)
	}

	if _, exists := query.CheckForbiddenPackageImport(g, ids.app, ids.plugin); !exists {
		t.Fatal("import-only app -> plugin did not produce import violation")
	}
	if violation, exists := query.CheckForbiddenPackageDependency(g, ids.app, ids.plugin); exists {
		t.Fatalf("import-only app -> plugin produced semantic violation: %#v", violation)
	}
}

func TestCheckForbiddenPackageDependencyValidatesExactDistinctPackages(t *testing.T) {
	g, ids := forbiddenDependencyFixture(t)
	for _, endpoints := range [][2]graph.SymbolRef{
		{ids.repository, ids.app}, // no semantic path
		{ids.app, ids.app},
		{"missing", ids.repository},
		{ids.app, "missing"},
		{ids.appRun, ids.repository},
		{ids.app, ids.repositorySave},
	} {
		if violation, exists := query.CheckForbiddenPackageDependency(g, endpoints[0], endpoints[1]); exists {
			t.Errorf("invalid, identical, or disconnected endpoints %q -> %q = %#v, true", endpoints[0], endpoints[1], violation)
		}
	}
	if violation, exists := query.CheckForbiddenPackageDependency(nil, ids.app, ids.repository); exists {
		t.Errorf("nil graph = %#v, true; want no violation", violation)
	}
}

func TestCheckForbiddenPackageDependencyFindsNOCVPaths(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), filepath.Join(".."), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	violation, exists := query.CheckForbiddenPackageDependency(g, "nocv/cmd/nocv", "nocv/graph")
	if !exists || len(violation.Paths) == 0 {
		t.Fatalf("NOCV cmd/nocv -> graph = (%#v, %v), want semantic violation", violation, exists)
	}
	if violation, exists := query.CheckForbiddenPackageDependency(g, "nocv/query", "nocv/goanalyzer"); exists {
		t.Fatalf("NOCV query -> goanalyzer = %#v, true; want no violation", violation)
	}
}

type forbiddenDependencyIDs struct {
	app            graph.SymbolRef
	cache          graph.SymbolRef
	plugin         graph.SymbolRef
	repository     graph.SymbolRef
	service        graph.SymbolRef
	appRun         graph.SymbolRef
	repositorySave graph.SymbolRef
}

func forbiddenDependencyFixture(t *testing.T) (*graph.Graph, forbiddenDependencyIDs) {
	t.Helper()
	ids := forbiddenDependencyIDs{
		app:            "example.com/app",
		cache:          "example.com/cache",
		plugin:         "example.com/plugin",
		repository:     "example.com/repository",
		service:        "example.com/service",
		appRun:         "example.com/app::Run",
		repositorySave: "example.com/repository::Save",
	}
	g := graph.New()
	for _, id := range []graph.SymbolRef{ids.app, ids.cache, ids.plugin, ids.repository, ids.service} {
		mustAddPackagePathNode(t, g, testNode{ID: id, Kind: graph.NodePackage, Name: string(id)})
	}
	nodes := []testNode{
		{ID: ids.appRun, Kind: graph.NodeFunction, Name: "Run", Parent: ids.app},
		{ID: "example.com/app::Alternate", Kind: graph.NodeFunction, Name: "Alternate", Parent: ids.app},
		{ID: "example.com/app::Direct", Kind: graph.NodeFunction, Name: "Direct", Parent: ids.app},
		{ID: "example.com/cache::Get", Kind: graph.NodeFunction, Name: "Get", Parent: ids.cache},
		{ID: "example.com/repository::Contract", Kind: graph.NodeInterface, Name: "Contract", Parent: ids.repository},
		{ID: ids.repositorySave, Kind: graph.NodeFunction, Name: "Save", Parent: ids.repository},
		{ID: "example.com/service::Create", Kind: graph.NodeFunction, Name: "Create", Parent: ids.service},
		{ID: "example.com/service::Update", Kind: graph.NodeFunction, Name: "Update", Parent: ids.service},
	}
	for _, node := range nodes {
		mustAddPackagePathNode(t, g, node)
	}
	edges := []testEdge{
		{From: "example.com/app::Direct", To: ids.repositorySave, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: ids.appRun, To: "example.com/cache::Get", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "example.com/cache::Get", To: ids.repositorySave, Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: ids.appRun, To: "example.com/service::Create", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "example.com/service::Create", To: "example.com/repository::Contract", Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed},
		{From: "example.com/app::Alternate", To: "example.com/service::Update", Kind: graph.EdgeCalls, Certainty: graph.RelationshipConfirmed},
		{From: "example.com/service::Update", To: "example.com/repository::Contract", Kind: graph.EdgeAccepts, Certainty: graph.RelationshipConfirmed},
		{From: ids.app, To: ids.plugin, Kind: graph.EdgeImports, Certainty: graph.RelationshipConfirmed},
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "fixture.go", Offset: index}}
		mustAddPackagePathEdge(t, g, edges[index])
	}
	return g, ids
}

func containsSemanticKind(path query.PackageDependencyPath, kind graph.EdgeKind) bool {
	for _, step := range path.Steps {
		for _, evidence := range step.Evidence {
			if evidence.Kind == kind {
				return true
			}
		}
	}
	return false
}
