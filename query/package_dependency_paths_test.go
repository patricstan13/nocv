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

func TestPackageDependencyPathsGroupEvidenceCollapseHopsAndPreserveRoutes(t *testing.T) {
	g := graph.New()
	app := graph.SymbolID("example.com/app")
	cache := graph.SymbolID("example.com/cache")
	repository := graph.SymbolID("example.com/repository")
	service := graph.SymbolID("example.com/service")
	for _, packageID := range []graph.SymbolID{app, cache, repository, service} {
		mustAddPackagePathNode(t, g, graph.Node{ID: packageID, Kind: graph.NodePackage, Name: string(packageID)})
	}

	function := func(packageID graph.SymbolID, name string) graph.SymbolID {
		id := graph.ChildID(packageID, name)
		mustAddPackagePathNode(t, g, graph.Node{ID: id, Kind: graph.NodeFunction, Name: name, Parent: packageID})
		return id
	}
	aDirect := function(app, "Direct")
	aGrouped := function(app, "Grouped")
	aDistinct := function(app, "Distinct")
	aThree := function(app, "Three")
	aRevisit := function(app, "Revisit")
	appAgain := function(app, "Again")
	cacheRoute := function(cache, "Route")
	cacheThree := function(cache, "Three")
	repositorySave := function(repository, "Save")
	serviceFirst := function(service, "First")
	serviceInternal := function(service, "Internal")
	serviceSecond := function(service, "Second")
	serviceThree := function(service, "Three")
	serviceRevisit := function(service, "Revisit")

	for index, endpoints := range [][2]graph.SymbolID{
		{aDirect, repositorySave},
		{aGrouped, serviceFirst},
		{serviceFirst, serviceInternal},
		{serviceInternal, repositorySave},
		{aGrouped, serviceSecond},
		{serviceSecond, repositorySave},
		{aDistinct, cacheRoute},
		{cacheRoute, repositorySave},
		{aThree, serviceThree},
		{serviceThree, cacheThree},
		{cacheThree, repositorySave},
		{aRevisit, serviceRevisit},
		{serviceRevisit, appAgain},
		{appAgain, repositorySave},
	} {
		mustAddPackagePathEdge(t, g, graph.Edge{
			From: endpoints[0], To: endpoints[1], Kind: graph.EdgeCalls,
			Evidence: []graph.Location{{File: "routes.go", Offset: index}},
		})
	}

	before := outgoingSnapshot(g)
	wantPackages := [][]graph.SymbolID{
		{app, repository},
		{app, cache, repository},
		{app, service, repository},
		{app, service, app, repository},
		{app, service, cache, repository},
	}
	wantEvidenceCounts := []int{2, 1, 2, 1, 1}
	got := query.PackageDependencyPaths(g, app, repository)
	if len(got) != len(wantPackages) {
		t.Fatalf("PackageDependencyPaths(app, repository) = %#v, want %d routes", got, len(wantPackages))
	}
	for index := range wantPackages {
		if !reflect.DeepEqual(got[index].Packages, wantPackages[index]) {
			t.Errorf("route %d packages = %v, want %v", index, got[index].Packages, wantPackages[index])
		}
		if len(got[index].Evidence) != wantEvidenceCounts[index] {
			t.Errorf("route %d evidence = %#v, want %d paths", index, got[index].Evidence, wantEvidenceCounts[index])
		}
	}

	grouped := got[2]
	if len(grouped.Evidence[0].Steps) != 2 || len(grouped.Evidence[1].Steps) != 3 {
		t.Fatalf("grouped service evidence = %#v, want two- and three-step exact paths", grouped.Evidence)
	}
	if gotAgain := query.PackageDependencyPaths(g, app, repository); !reflect.DeepEqual(gotAgain, got) {
		t.Fatalf("second package path query = %#v, want deterministic %#v", gotAgain, got)
	}
	if after := outgoingSnapshot(g); !reflect.DeepEqual(after, before) {
		t.Fatalf("package path projection mutated graph edges:\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func TestPackageDependencyPathsUseEverySemanticKindAndMixedPaths(t *testing.T) {
	g := graph.New()
	app := graph.SymbolID("example.com/app")
	repository := graph.SymbolID("example.com/repository")
	service := graph.SymbolID("example.com/service")
	for _, packageID := range []graph.SymbolID{app, repository, service} {
		mustAddPackagePathNode(t, g, graph.Node{ID: packageID, Kind: graph.NodePackage, Name: string(packageID)})
	}

	appStruct := graph.ChildID(app, "Implementation")
	appEmbedded := graph.ChildID(app, "Embedded")
	appCall := graph.ChildID(app, "Call")
	appAccept := graph.ChildID(app, "Accept")
	appReturn := graph.ChildID(app, "Return")
	appMixed := graph.ChildID(app, "Mixed")
	repositoryInterface := graph.ChildID(repository, "Contract")
	repositoryStruct := graph.ChildID(repository, "Base")
	repositoryFunction := graph.ChildID(repository, "Save")
	serviceFunction := graph.ChildID(service, "Create")
	for _, node := range []graph.Node{
		{ID: appStruct, Kind: graph.NodeStruct, Name: "Implementation", Parent: app},
		{ID: appEmbedded, Kind: graph.NodeStruct, Name: "Embedded", Parent: app},
		{ID: appCall, Kind: graph.NodeFunction, Name: "Call", Parent: app},
		{ID: appAccept, Kind: graph.NodeFunction, Name: "Accept", Parent: app},
		{ID: appReturn, Kind: graph.NodeFunction, Name: "Return", Parent: app},
		{ID: appMixed, Kind: graph.NodeFunction, Name: "Mixed", Parent: app},
		{ID: repositoryInterface, Kind: graph.NodeInterface, Name: "Contract", Parent: repository},
		{ID: repositoryStruct, Kind: graph.NodeStruct, Name: "Base", Parent: repository},
		{ID: repositoryFunction, Kind: graph.NodeFunction, Name: "Save", Parent: repository},
		{ID: serviceFunction, Kind: graph.NodeFunction, Name: "Create", Parent: service},
	} {
		mustAddPackagePathNode(t, g, node)
	}

	edges := []graph.Edge{
		{From: appStruct, To: repositoryInterface, Kind: graph.EdgeImplements},
		{From: appEmbedded, To: repositoryStruct, Kind: graph.EdgeEmbeds},
		{From: appCall, To: repositoryFunction, Kind: graph.EdgeCalls},
		{From: appAccept, To: repositoryInterface, Kind: graph.EdgeAccepts},
		{From: appReturn, To: repositoryInterface, Kind: graph.EdgeReturns},
		{From: appMixed, To: serviceFunction, Kind: graph.EdgeCalls},
		{From: serviceFunction, To: repositoryInterface, Kind: graph.EdgeAccepts},
	}
	for index := range edges {
		edges[index].Evidence = []graph.Location{{File: "relationships.go", Offset: index}}
		mustAddPackagePathEdge(t, g, edges[index])
	}

	got := query.PackageDependencyPaths(g, app, repository)
	if len(got) != 2 {
		t.Fatalf("PackageDependencyPaths(app, repository) = %#v, want direct and service routes", got)
	}
	if !reflect.DeepEqual(got[0].Packages, []graph.SymbolID{app, repository}) || len(got[0].Evidence) != 5 {
		t.Fatalf("direct package route = %#v, want five semantic kinds", got[0])
	}
	wantKinds := map[graph.EdgeKind]bool{
		graph.EdgeCalls: true, graph.EdgeImplements: true, graph.EdgeEmbeds: true,
		graph.EdgeAccepts: true, graph.EdgeReturns: true,
	}
	for _, evidence := range got[0].Evidence {
		if len(evidence.Steps) != 1 || !wantKinds[evidence.Steps[0].Kind] {
			t.Errorf("unexpected direct semantic evidence: %#v", evidence)
		}
		delete(wantKinds, evidence.Steps[0].Kind)
	}
	if len(wantKinds) != 0 {
		t.Errorf("missing projected semantic kinds: %v", wantKinds)
	}
	wantMixed := query.PackageDependencyPath{
		Packages: []graph.SymbolID{app, service, repository},
		Evidence: []query.SemanticPath{{Steps: []query.SemanticStep{
			{From: appMixed, To: serviceFunction, Kind: graph.EdgeCalls},
			{From: serviceFunction, To: repositoryInterface, Kind: graph.EdgeAccepts},
		}}},
	}
	if !reflect.DeepEqual(got[1], wantMixed) {
		t.Fatalf("mixed package route = %#v, want %#v", got[1], wantMixed)
	}
}

func TestPackageDependencyPathsValidateExactPackageEndpoints(t *testing.T) {
	g := graph.New()
	app := graph.SymbolID("example.com/app")
	repository := graph.SymbolID("example.com/repository")
	appFunction := graph.ChildID(app, "Run")
	repositoryFunction := graph.ChildID(repository, "Save")
	for _, node := range []graph.Node{
		{ID: app, Kind: graph.NodePackage, Name: "app"},
		{ID: repository, Kind: graph.NodePackage, Name: "repository"},
		{ID: appFunction, Kind: graph.NodeFunction, Name: "Run", Parent: app},
		{ID: repositoryFunction, Kind: graph.NodeFunction, Name: "Save", Parent: repository},
	} {
		mustAddPackagePathNode(t, g, node)
	}

	for _, paths := range [][]query.PackageDependencyPath{
		query.PackageDependencyPaths(nil, app, repository),
		query.PackageDependencyPaths(g, "missing", repository),
		query.PackageDependencyPaths(g, app, "missing"),
		query.PackageDependencyPaths(g, appFunction, repository),
		query.PackageDependencyPaths(g, app, repositoryFunction),
		query.PackageDependencyPaths(g, app, app),
		query.PackageDependencyPaths(g, app, repository),
	} {
		if len(paths) != 0 {
			t.Errorf("invalid, identical, structural-only, or disconnected package paths = %#v, want empty", paths)
		}
	}
}

func TestAnalyzerPipelineProjectsGroupedPackageDependencyPath(t *testing.T) {
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
		Evidence: []query.SemanticPath{
			{Steps: []query.SemanticStep{{From: run, To: create, Kind: graph.EdgeCalls}, {From: create, To: save, Kind: graph.EdgeCalls}}},
			{Steps: []query.SemanticStep{{From: run, To: update, Kind: graph.EdgeCalls}, {From: update, To: save, Kind: graph.EdgeCalls}}},
		},
	}}
	if got := query.PackageDependencyPaths(g, app, repository); !reflect.DeepEqual(got, want) {
		t.Fatalf("fixture package paths = %#v, want %#v", got, want)
	}

	// Exact symbol paths remain the evidence source rather than projected graph edges.
	if got := query.DependencyPaths(g, run, save); !reflect.DeepEqual(got, want[0].Evidence) {
		t.Fatalf("fixture exact paths = %#v, want grouped evidence %#v", got, want[0].Evidence)
	}
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
