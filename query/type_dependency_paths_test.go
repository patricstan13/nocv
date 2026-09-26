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

func TestDirectTypeDependenciesProjectOwnersAggregateKindsAndExcludeNonTypes(t *testing.T) {
	g, ids := typeDependencyFixture(t)
	beforeGraph := importGraphSnapshot(g)
	beforeExact := query.DirectDependencies(g, ids.serviceCreate)
	beforePackages := query.PackageDependencyPaths(g, ids.pkg, ids.otherPackage)

	dependencies := query.DirectTypeDependencies(g, ids.service)
	if len(dependencies) != 2 || dependencies[0].To != ids.contract || dependencies[1].To != ids.repository {
		t.Fatalf("DirectTypeDependencies(Service) = %#v, want sorted Contract and Repository dependencies", dependencies)
	}
	if len(dependencies[0].Evidence) != 2 {
		t.Fatalf("Service -> Contract evidence = %#v, want type- and method-level Implements", dependencies[0].Evidence)
	}
	if len(dependencies[1].Evidence) != 7 {
		t.Fatalf("Service -> Repository evidence = %#v, want seven aggregated boundary facts", dependencies[1].Evidence)
	}
	wantKinds := map[graph.EdgeKind]bool{
		graph.EdgeCalls: true, graph.EdgeEmbeds: true, graph.EdgeAccepts: true, graph.EdgeReturns: true,
	}
	for index, evidence := range dependencies[1].Evidence {
		if evidence.From == ids.run || evidence.To == ids.helper ||
			(evidence.From == ids.serviceCreate && evidence.To == ids.serviceValidate) || evidence.Kind == graph.EdgeImports {
			t.Errorf("package-function, same-type, or import fact leaked into type evidence: %#v", evidence)
		}
		if len(evidence.Evidence) != 1 || evidence.Evidence[0].File != "types.go" {
			t.Errorf("type evidence lost source location: %#v", evidence)
		}
		wantKinds[evidence.Kind] = false
		if index > 0 && relationshipOrderLess(evidence, dependencies[1].Evidence[index-1]) {
			t.Errorf("type evidence is not deterministic: %#v", dependencies[1].Evidence)
		}
	}
	for kind, missing := range wantKinds {
		if missing {
			t.Errorf("missing %s evidence from Service -> Repository", kind)
		}
	}

	interfaceDependencies := query.DirectTypeDependencies(g, ids.childInterface)
	if len(interfaceDependencies) != 1 || interfaceDependencies[0].To != ids.parentInterface ||
		len(interfaceDependencies[0].Evidence) != 1 || interfaceDependencies[0].Evidence[0].Kind != graph.EdgeEmbeds {
		t.Fatalf("interface embedding projection = %#v", interfaceDependencies)
	}
	contractDependencies := query.DirectTypeDependencies(g, ids.contract)
	if len(contractDependencies) != 1 || contractDependencies[0].To != ids.repository ||
		len(contractDependencies[0].Evidence) != 1 || contractDependencies[0].Evidence[0].Kind != graph.EdgeAccepts {
		t.Fatalf("interface -> struct projection = %#v", contractDependencies)
	}

	dependencies[1].Evidence[0].From = "mutated"
	dependencies[1].Evidence[0].Evidence[0].Offset = 999
	if again := query.DirectTypeDependencies(g, ids.service); len(again) != 2 || again[1].Evidence[0].From == "mutated" || again[1].Evidence[0].Evidence[0].Offset == 999 {
		t.Fatalf("type dependency result was not detached: %#v", again)
	}
	if after := importGraphSnapshot(g); !reflect.DeepEqual(after, beforeGraph) {
		t.Fatalf("type dependency query mutated graph:\nbefore: %#v\nafter:  %#v", beforeGraph, after)
	}
	if after := query.DirectDependencies(g, ids.serviceCreate); !reflect.DeepEqual(after, beforeExact) {
		t.Fatalf("exact direct dependencies changed: got %#v, want %#v", after, beforeExact)
	}
	if after := query.PackageDependencyPaths(g, ids.pkg, ids.otherPackage); !reflect.DeepEqual(after, beforePackages) {
		t.Fatalf("package view changed: got %#v, want %#v", after, beforePackages)
	}

	for _, got := range [][]query.TypeDependency{
		query.DirectTypeDependencies(nil, ids.service),
		query.DirectTypeDependencies(g, "missing"),
		query.DirectTypeDependencies(g, ids.pkg),
		query.DirectTypeDependencies(g, ids.run),
		query.DirectTypeDependencies(g, ids.serviceCreate),
	} {
		if len(got) != 0 {
			t.Errorf("invalid type root dependencies = %#v, want empty", got)
		}
	}
}

func TestTypeDependencyPathsTraverseDisconnectedMethodsAndPreserveRoutes(t *testing.T) {
	g, ids := typePathFixture(t)

	if exact := query.DependencyPaths(g, ids.serviceCreate, ids.storePut); len(exact) != 0 {
		t.Fatalf("disconnected Repository methods unexpectedly formed exact path: %#v", exact)
	}
	wantTypes := [][]graph.SymbolID{
		{ids.service, ids.cache, ids.store},
		{ids.service, ids.repository, ids.store},
	}
	paths := query.TypeDependencyPaths(g, ids.service, ids.store)
	if len(paths) != len(wantTypes) {
		t.Fatalf("TypeDependencyPaths(Service, Store) = %#v, want two routes", paths)
	}
	for index, path := range paths {
		if !reflect.DeepEqual(path.Types, wantTypes[index]) {
			t.Errorf("path %d types = %v, want %v", index, path.Types, wantTypes[index])
		}
		if len(path.Steps) != len(path.Types)-1 {
			t.Errorf("path %d has %d steps for %d types", index, len(path.Steps), len(path.Types))
		}
		for stepIndex, step := range path.Steps {
			if step.From != path.Types[stepIndex] || step.To != path.Types[stepIndex+1] || len(step.Evidence) != 1 {
				t.Errorf("path %d step %d violates type-hop invariant: %#v", index, stepIndex, step)
			}
		}
	}
	if again := query.TypeDependencyPaths(g, ids.service, ids.store); !reflect.DeepEqual(again, paths) {
		t.Fatalf("second type path query = %#v, want deterministic %#v", again, paths)
	}

	paths[0].Types[0] = "mutated"
	paths[0].Steps[0].Evidence[0].Evidence[0].Offset = 999
	if again := query.TypeDependencyPaths(g, ids.service, ids.store); again[0].Types[0] == "mutated" || again[0].Steps[0].Evidence[0].Evidence[0].Offset == 999 {
		t.Fatalf("type path result was not detached: %#v", again)
	}

	for _, got := range [][]query.TypeDependencyPath{
		query.TypeDependencyPaths(nil, ids.service, ids.store),
		query.TypeDependencyPaths(g, "missing", ids.store),
		query.TypeDependencyPaths(g, ids.service, "missing"),
		query.TypeDependencyPaths(g, ids.serviceCreate, ids.store),
		query.TypeDependencyPaths(g, ids.service, ids.storePut),
		query.TypeDependencyPaths(g, ids.service, ids.service),
	} {
		if len(got) != 0 {
			t.Errorf("invalid or identical type paths = %#v, want empty", got)
		}
	}
}

func TestAnalyzerPipelineProjectsTypeView(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), filepath.Join("..", "goanalyzer", "testdata", "typeview"), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	service := graph.SymbolID("example.com/typeview/service::Service")
	repository := graph.SymbolID("example.com/typeview/repository::Repository")
	store := graph.SymbolID("example.com/typeview/store::Store")
	create := graph.SymbolID("example.com/typeview/service::Service::Create")
	save := graph.SymbolID("example.com/typeview/repository::Repository::Save")
	put := graph.SymbolID("example.com/typeview/store::Store::Put")
	run := graph.SymbolID("example.com/typeview/app::Run")

	dependencies := query.DirectTypeDependencies(g, service)
	if len(dependencies) != 1 || dependencies[0].To != repository || len(dependencies[0].Evidence) != 3 {
		t.Fatalf("analyzer Service type dependencies = %#v, want one Repository edge with three facts", dependencies)
	}
	for _, evidence := range dependencies[0].Evidence {
		if evidence.From == "example.com/typeview/service::Service::Internal" {
			t.Errorf("internal same-type call leaked into analyzer type evidence: %#v", evidence)
		}
	}
	if got := query.DirectTypeDependencies(g, run); len(got) != 0 {
		t.Fatalf("package function acquired type dependencies: %#v", got)
	}
	if got := query.DependencyPaths(g, create, put); len(got) != 0 {
		t.Fatalf("disconnected analyzer methods formed exact path: %#v", got)
	}
	paths := query.TypeDependencyPaths(g, service, store)
	if len(paths) != 1 || !reflect.DeepEqual(paths[0].Types, []graph.SymbolID{service, repository, store}) {
		t.Fatalf("analyzer type paths = %#v, want Service -> Repository -> Store", paths)
	}
	if exact := query.DependencyPaths(g, create, save); len(exact) != 1 || len(exact[0].Steps) != 1 {
		t.Fatalf("exact Create -> Save fact changed: %#v", exact)
	}
	packagePaths := query.PackageDependencyPaths(g, "example.com/typeview/service", "example.com/typeview/repository")
	if len(packagePaths) != 1 {
		t.Fatalf("package projection lacks service -> repository: %#v", packagePaths)
	}
}

type typeDependencyIDs struct {
	pkg             graph.SymbolID
	otherPackage    graph.SymbolID
	service         graph.SymbolID
	repository      graph.SymbolID
	contract        graph.SymbolID
	childInterface  graph.SymbolID
	parentInterface graph.SymbolID
	serviceCreate   graph.SymbolID
	serviceValidate graph.SymbolID
	run             graph.SymbolID
	helper          graph.SymbolID
}

func typeDependencyFixture(t *testing.T) (*graph.Graph, typeDependencyIDs) {
	t.Helper()
	ids := typeDependencyIDs{
		pkg:             "example.com/types",
		otherPackage:    "example.com/other",
		service:         "example.com/types::Service",
		repository:      "example.com/other::Repository",
		contract:        "example.com/other::Contract",
		childInterface:  "example.com/types::Child",
		parentInterface: "example.com/types::Parent",
		serviceCreate:   "example.com/types::Service::Create",
		serviceValidate: "example.com/types::Service::validate",
		run:             "example.com/types::Run",
		helper:          "example.com/other::helper",
	}
	g := graph.New()
	for _, node := range []graph.Node{
		{ID: ids.pkg, Kind: graph.NodePackage, Name: "types"},
		{ID: ids.otherPackage, Kind: graph.NodePackage, Name: "other"},
		{ID: ids.service, Kind: graph.NodeStruct, Name: "Service", Parent: ids.pkg},
		{ID: ids.repository, Kind: graph.NodeStruct, Name: "Repository", Parent: ids.otherPackage},
		{ID: ids.contract, Kind: graph.NodeInterface, Name: "Contract", Parent: ids.otherPackage},
		{ID: ids.childInterface, Kind: graph.NodeInterface, Name: "Child", Parent: ids.pkg},
		{ID: ids.parentInterface, Kind: graph.NodeInterface, Name: "Parent", Parent: ids.pkg},
		{ID: ids.serviceCreate, Kind: graph.NodeFunction, Name: "Create", Parent: ids.service},
		{ID: "example.com/types::Service::Update", Kind: graph.NodeFunction, Name: "Update", Parent: ids.service},
		{ID: "example.com/types::Service::Delete", Kind: graph.NodeFunction, Name: "Delete", Parent: ids.service},
		{ID: ids.serviceValidate, Kind: graph.NodeFunction, Name: "validate", Parent: ids.service},
		{ID: "example.com/other::Repository::Save", Kind: graph.NodeFunction, Name: "Save", Parent: ids.repository},
		{ID: "example.com/other::Repository::Update", Kind: graph.NodeFunction, Name: "Update", Parent: ids.repository},
		{ID: "example.com/other::Repository::Delete", Kind: graph.NodeFunction, Name: "Delete", Parent: ids.repository},
		{ID: "example.com/other::Contract::Execute", Kind: graph.NodeFunction, Name: "Execute", Parent: ids.contract},
		{ID: ids.run, Kind: graph.NodeFunction, Name: "Run", Parent: ids.pkg},
		{ID: ids.helper, Kind: graph.NodeFunction, Name: "helper", Parent: ids.otherPackage},
	} {
		mustAddPackagePathNode(t, g, node)
	}
	edges := []graph.Edge{
		{From: ids.serviceCreate, To: "example.com/other::Repository::Save", Kind: graph.EdgeCalls},
		{From: "example.com/types::Service::Update", To: "example.com/other::Repository::Update", Kind: graph.EdgeCalls},
		{From: "example.com/types::Service::Delete", To: "example.com/other::Repository::Delete", Kind: graph.EdgeCalls},
		{From: ids.serviceCreate, To: ids.repository, Kind: graph.EdgeAccepts},
		{From: ids.serviceCreate, To: ids.repository, Kind: graph.EdgeReturns},
		{From: ids.service, To: ids.repository, Kind: graph.EdgeEmbeds},
		{From: ids.serviceValidate, To: "example.com/other::Repository::Save", Kind: graph.EdgeCalls},
		{From: ids.service, To: ids.contract, Kind: graph.EdgeImplements},
		{From: ids.serviceCreate, To: "example.com/other::Contract::Execute", Kind: graph.EdgeImplements},
		{From: "example.com/other::Contract::Execute", To: ids.repository, Kind: graph.EdgeAccepts},
		{From: ids.childInterface, To: ids.parentInterface, Kind: graph.EdgeEmbeds},
		{From: ids.serviceCreate, To: ids.serviceValidate, Kind: graph.EdgeCalls},
		{From: ids.run, To: ids.serviceCreate, Kind: graph.EdgeCalls},
		{From: ids.serviceCreate, To: ids.helper, Kind: graph.EdgeCalls},
		{From: ids.pkg, To: ids.otherPackage, Kind: graph.EdgeImports},
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "types.go", Offset: index}}
		mustAddPackagePathEdge(t, g, edges[index])
	}
	return g, ids
}

type typePathIDs struct {
	service       graph.SymbolID
	repository    graph.SymbolID
	cache         graph.SymbolID
	store         graph.SymbolID
	serviceCreate graph.SymbolID
	storePut      graph.SymbolID
}

func typePathFixture(t *testing.T) (*graph.Graph, typePathIDs) {
	t.Helper()
	ids := typePathIDs{
		service:       "types::Service",
		repository:    "types::Repository",
		cache:         "types::Cache",
		store:         "types::Store",
		serviceCreate: "types::Service::Create",
		storePut:      "types::Store::Put",
	}
	g := graph.New()
	mustAddPackagePathNode(t, g, graph.Node{ID: "types", Kind: graph.NodePackage, Name: "types"})
	for _, node := range []graph.Node{
		{ID: ids.service, Kind: graph.NodeStruct, Name: "Service", Parent: "types"},
		{ID: ids.repository, Kind: graph.NodeStruct, Name: "Repository", Parent: "types"},
		{ID: ids.cache, Kind: graph.NodeStruct, Name: "Cache", Parent: "types"},
		{ID: ids.store, Kind: graph.NodeStruct, Name: "Store", Parent: "types"},
		{ID: ids.serviceCreate, Kind: graph.NodeFunction, Name: "Create", Parent: ids.service},
		{ID: "types::Service::Cache", Kind: graph.NodeFunction, Name: "Cache", Parent: ids.service},
		{ID: "types::Repository::Save", Kind: graph.NodeFunction, Name: "Save", Parent: ids.repository},
		{ID: "types::Repository::Update", Kind: graph.NodeFunction, Name: "Update", Parent: ids.repository},
		{ID: "types::Cache::Get", Kind: graph.NodeFunction, Name: "Get", Parent: ids.cache},
		{ID: "types::Cache::Write", Kind: graph.NodeFunction, Name: "Write", Parent: ids.cache},
		{ID: ids.storePut, Kind: graph.NodeFunction, Name: "Put", Parent: ids.store},
	} {
		mustAddPackagePathNode(t, g, node)
	}
	edges := []graph.Edge{
		{From: ids.serviceCreate, To: "types::Repository::Save", Kind: graph.EdgeCalls},
		{From: "types::Repository::Update", To: ids.storePut, Kind: graph.EdgeCalls},
		{From: "types::Service::Cache", To: "types::Cache::Get", Kind: graph.EdgeCalls},
		{From: "types::Cache::Write", To: ids.storePut, Kind: graph.EdgeCalls},
		{From: "types::Repository::Save", To: ids.serviceCreate, Kind: graph.EdgeCalls}, // malformed type cycle
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "paths.go", Offset: index}}
		mustAddPackagePathEdge(t, g, edges[index])
	}
	return g, ids
}
