package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"nocv/graph"
)

func TestClientRendersServerDataAsText(t *testing.T) {
	client, err := fs.ReadFile(assets, "static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	contents := string(client)
	if !strings.Contains(contents, "textContent") {
		t.Fatal("client does not use textContent")
	}
	for _, unsafe := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML"} {
		if strings.Contains(contents, unsafe) {
			t.Errorf("client uses unsafe HTML sink %q", unsafe)
		}
	}
}

func TestClientFreezesPhysicsAndPreservesSemanticEdgeDirection(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`packageNetwork.once("stabilizationIterationsDone", freezePhysics)`,
		`packageNetwork.once("stabilized", freezePhysics)`,
		`window.setTimeout(freezePhysics, 8000)`,
		`packageNetwork.setOptions({ physics: false })`,
		`from: edge.to`,
		`to: edge.from`,
		`semanticFrom: edge.from`,
		`semanticTo: edge.to`,
		`openPackageDependency(selected.semanticFrom, selected.semanticTo, selected, false, null)`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("client source lacks %q", required)
		}
	}
}

func TestClientUsesCompactProgressiveInspector(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`inspector.scrollTop = 0`,
		`packageDependencyItem(value, result.node.id)`,
		`packageDependentItem(value, result.node.id)`,
		`section("Imports", detail.imports, dependencyTargetItem)`,
		`contentsSummary(detail)`,
		`collapsibleRelationships(facts, 2, "Show evidence")`,
		`collapsibleRelationships(exact, 5, "Show relationships")`,
		`from + " depends on " + to`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("client source lacks %q", required)
		}
	}
	for _, exhaustive := range []string{`section("Types"`, `section("Functions"`, `section("Imports", detail.imports, relationshipItem)`} {
		if strings.Contains(client, exhaustive) {
			t.Errorf("client still renders exhaustive package detail with %q", exhaustive)
		}
	}
}

func TestClientNavigatesPackageRelationshipsBySemanticIdentity(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`function packageRelationshipItem(value, ref, originPackageRef)`,
		`button.title = ref`,
		`button.setAttribute("aria-label", label + " (" + ref + ")")`,
		`button.addEventListener("click", () => navigateToPackageDependency(value, originPackageRef))`,
		`return packageRelationshipItem(value, value.to, originPackageRef)`,
		`return packageRelationshipItem(value, value.from, originPackageRef)`,
		`function findPackageEdge(relationship)`,
		`edge.semanticFrom === relationship.from && edge.semanticTo === relationship.to`,
		`packageNetwork.setSelection({ edges: [edge.id] }`,
		`nodes: [edge.from, edge.to]`,
		`maxZoomLevel: 0.85`,
		`openPackageDependency(relationship.from, relationship.to, edge, true, originPackageRef)`,
		`showDependency(from, to)`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("client relationship navigation source lacks %q", required)
		}
	}
	for _, forbidden := range []string{`/api/relationship`, `/api/focus`, `/api/package-edge`} {
		if strings.Contains(client, forbidden) {
			t.Errorf("client relationship navigation source contains forbidden API %q", forbidden)
		}
	}
}

func TestClientProvidesOneLevelDependencyBackNavigation(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`let packageDependencyOriginRef = null`,
		`packageDependencyOriginRef = originPackageRef || null`,
		`if (packageDependencyOriginRef && packageLabels.has(packageDependencyOriginRef))`,
		`element("button", "← Back to " + label, "back-button dependency-back-button")`,
		`back.setAttribute("aria-label", "Back to package " + packageDependencyOriginRef)`,
		`back.addEventListener("click", returnToPackageOrigin)`,
		`function returnToPackageOrigin()`,
		`if (!originPackageRef || !packageLabels.has(originPackageRef)) return`,
		`selectPackage(originPackageRef, true)`,
		`function showPackage(id)`,
		`packageDependencyOriginRef = null`,
		`selectPackage(params.nodes[0], false)`,
		`openPackageDependency(selected.semanticFrom, selected.semanticTo, selected, false, null)`,
		`renderPackageDependencyInspection(currentPackageDependency.from, currentPackageDependency.to, currentPackageDependency.result)`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("client dependency Back source lacks %q", required)
		}
	}
	for _, forbidden := range []string{`navigationHistory`, `history.pushState`, `history.back`, `popstate`} {
		if strings.Contains(client, forbidden) {
			t.Errorf("client dependency Back source contains forbidden history mechanism %q", forbidden)
		}
	}
}

func TestClientSearchesRanksAndFocusesExistingPackages(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`const SEARCH_RESULT_LIMIT = 10`,
		`function searchPackages(packageNodes, query)`,
		`query.trim().toLowerCase()`,
		`if (label === normalized) rank = 0`,
		`else if (label.startsWith(normalized)) rank = 1`,
		`else if (label.includes(normalized)) rank = 2`,
		`else if (ref.includes(normalized)) rank = 3`,
		`compareText(left.label, right.label) || compareText(left.ref, right.ref)`,
		`ranked.slice(0, SEARCH_RESULT_LIMIT)`,
		`event.key === "ArrowDown"`,
		`event.key === "ArrowUp"`,
		`event.key === "Enter"`,
		`event.key === "Escape"`,
		`selectPackage(node.id, true)`,
		`packageNetwork.selectNodes([ref], true)`,
		`packageNetwork.focus(ref`,
		`showPackage(ref)`,
		`packageSearchControl = setupPackageSearch(packageNodes)`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("client search source lacks %q", required)
		}
	}
	for _, mutation := range []string{"nodes.remove(", "nodes.clear(", "nodes.update(", "edges.remove(", "edges.clear(", "edges.update("} {
		if strings.Contains(client, mutation) {
			t.Errorf("search source may mutate graph data through %q", mutation)
		}
	}
}

func TestEmbeddedAssetsPreserveCanvasSizingAndHaveNoMissingSourceMap(t *testing.T) {
	index := readAsset(t, "static/index.html")
	if !strings.Contains(index, "dependency → dependent") {
		t.Fatal("package explorer legend does not explain visual edge direction")
	}
	for _, required := range []string{`id="package-search-input"`, `placeholder="Search packages..."`, `role="combobox"`, `role="listbox"`} {
		if !strings.Contains(index, required) {
			t.Errorf("package explorer search markup lacks %q", required)
		}
	}
	stylesheet := readAsset(t, "static/app.css")
	for _, required := range []string{
		`main { display: grid; grid-template-columns: minmax(0, 1fr) 380px; height: calc(100vh - 76px); min-height: 0; }`,
		`#graph-pane { position: relative; width: 100%; height: 100%; min-width: 0; min-height: 0; overflow: hidden;`,
		`.network-canvas { position: absolute; inset: 0; width: 100%; height: 100%; min-width: 0; min-height: 0; overflow: hidden; }`,
	} {
		if !strings.Contains(stylesheet, required) {
			t.Errorf("stylesheet lacks canvas sizing rule %q", required)
		}
	}
	bundle := readAsset(t, "static/vis-network.min.js")
	if strings.Contains(bundle, "sourceMappingURL") {
		t.Fatal("vendored vis-network bundle still references an unavailable source map")
	}
}

func TestClientProvidesContextualTypeDrilldown(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`function showTypeDrilldown(result)`,
		`const dependencies = result.typeDependencies || []`,
		`const byRef = new Map()`,
		`id: dependency.from`,
		`id: dependency.to`,
		`from: dependency.to`,
		`to: dependency.from`,
		`semanticFrom: dependency.from`,
		`semanticTo: dependency.to`,
		`evidence: dependency.evidence || []`,
		`const activeTypeNetwork = new vis.Network(typeNetworkElement`,
		`typeNetwork = activeTypeNetwork`,
		`activeTypeNetwork.once("stabilizationIterationsDone", freezeTypePhysics)`,
		`activeTypeNetwork.setOptions({ physics: false })`,
		`showTypeDependency(typeEdges.get(params.edges[0]))`,
		`getJSON("/api/node?id=" + encodeURIComponent(id))`,
		`collapsibleRelationships(exact, 2, "Show exact relationships")`,
		`backToPackages.addEventListener("click", showPackageGraph)`,
		`packageSearchControl.setEnabled(false)`,
		`packageSearchControl.setEnabled(true)`,
		`packageNetwork.redraw()`,
		`renderPackageDependencyInspection(currentPackageDependency.from, currentPackageDependency.to, currentPackageDependency.result)`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("client type drilldown source lacks %q", required)
		}
	}
	for _, forbidden := range []string{`/api/types`, `history.pushState`, `history.replaceState`} {
		if strings.Contains(client, forbidden) {
			t.Errorf("client type drilldown source contains forbidden global navigation %q", forbidden)
		}
	}
	if strings.Count(client, `new vis.Network(networkElement`) != 1 {
		t.Error("package network should be constructed exactly once")
	}
}

func readAsset(t *testing.T, name string) string {
	t.Helper()
	contents, err := fs.ReadFile(assets, name)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func TestHandlerServesEmbeddedInterfaceAndAssets(t *testing.T) {
	handler := Handler(webFixture(t))

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/", contentType: "text/html", contains: "NOCV Package Explorer"},
		{path: "/static/app.css", contentType: "text/css", contains: "#inspector"},
		{path: "/static/app.js", contentType: "text/javascript", contains: "api/package-dependency"},
		{path: "/static/vis-network.min.js", contentType: "text/javascript", contains: "@version 10.1.0"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			response := request(t, handler, http.MethodGet, test.path)
			body, err := io.ReadAll(response.Result().Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", test.path, response.Code)
			}
			if !strings.Contains(response.Header().Get("Content-Type"), test.contentType) {
				t.Errorf("GET %s content type = %q, want containing %q", test.path, response.Header().Get("Content-Type"), test.contentType)
			}
			if !strings.Contains(string(body), test.contains) {
				t.Errorf("GET %s body does not contain %q", test.path, test.contains)
			}
		})
	}
}

func TestPackagesAPIIsDeterministicAndExcludesImports(t *testing.T) {
	handler := Handler(webFixture(t))
	first := request(t, handler, http.MethodGet, "/api/packages")
	second := request(t, handler, http.MethodGet, "/api/packages")
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", first.Code, first.Body.String())
	}
	if contentType := first.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", contentType)
	}
	if second.Body.String() != first.Body.String() {
		t.Fatalf("responses differ:\nfirst: %s\nsecond: %s", first.Body.String(), second.Body.String())
	}

	var result packageGraph
	decode(t, first, &result)
	wantNodes := []packageNode{
		{ID: "example.com/app", Label: "app"},
		{ID: "example.com/import-only", Label: "importonly"},
		{ID: "example.com/repository", Label: "repository"},
		{ID: "example.com/service", Label: "service"},
	}
	wantEdges := []packageEdge{
		{ID: edgeID("example.com/app", "example.com/service"), From: "example.com/app", To: "example.com/service"},
		{ID: edgeID("example.com/service", "example.com/repository"), From: "example.com/service", To: "example.com/repository"},
	}
	if !reflect.DeepEqual(result.Nodes, wantNodes) {
		t.Errorf("nodes = %#v, want %#v", result.Nodes, wantNodes)
	}
	if !reflect.DeepEqual(result.Edges, wantEdges) {
		t.Errorf("edges = %#v, want semantic-only %#v", result.Edges, wantEdges)
	}
}

func TestPackagesAPIPreservesRefsForDuplicateLabels(t *testing.T) {
	g := graph.New()
	for _, node := range []webNode{
		{ID: "example.com/a/internal", Kind: graph.NodePackage, Name: "internal"},
		{ID: "example.com/b/internal", Kind: graph.NodePackage, Name: "internal"},
	} {
		if err := addWebNode(g, node); err != nil {
			t.Fatal(err)
		}
	}

	response := request(t, Handler(g), http.MethodGet, "/api/packages")
	var result packageGraph
	decode(t, response, &result)
	want := []packageNode{
		{ID: "example.com/a/internal", Label: "internal"},
		{ID: "example.com/b/internal", Label: "internal"},
	}
	if response.Code != http.StatusOK || !reflect.DeepEqual(result.Nodes, want) {
		t.Fatalf("duplicate-label packages = status %d, nodes %#v; want %#v", response.Code, result.Nodes, want)
	}
}

func TestNodeAPIUsesInspectNodePresentation(t *testing.T) {
	handler := Handler(webFixture(t))
	response := request(t, handler, http.MethodGet, "/api/node?id=example.com%2Fapp")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var result nodeInspection
	decode(t, response, &result)
	if result.Node.ID != "example.com/app" || result.Node.Kind != "package" || result.Node.Documentation != "Package app starts the workflow." {
		t.Errorf("node = %#v", result.Node)
	}
	if result.Node.Location != nil {
		t.Errorf("package location = %#v, want omitted", result.Node.Location)
	}
	if result.Package == nil || len(result.Package.Functions) != 1 || result.Package.Functions[0].ID != "example.com/app::Run" {
		t.Fatalf("package functions = %#v", result.Package)
	}
	if len(result.Package.Dependencies) != 1 || result.Package.Dependencies[0].To != "example.com/service" {
		t.Errorf("semantic dependencies = %#v", result.Package.Dependencies)
	}
	if len(result.Package.Imports) != 2 || result.Package.Imports[0].To != "example.com/import-only" {
		t.Errorf("imports = %#v", result.Package.Imports)
	}
}

func TestNodeAPIPreservesDependencyDirectionForRelationshipNavigation(t *testing.T) {
	response := request(t, Handler(webFixture(t)), http.MethodGet, "/api/node?id=example.com%2Fservice")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var result nodeInspection
	decode(t, response, &result)
	if result.Package == nil {
		t.Fatal("service package inspection is missing")
	}
	if len(result.Package.Dependencies) != 1 || len(result.Package.Dependents) != 1 {
		t.Fatalf("service relationships = dependencies %#v, dependents %#v", result.Package.Dependencies, result.Package.Dependents)
	}
	dependency := result.Package.Dependencies[0]
	if dependency.From != "example.com/service" || dependency.To != "example.com/repository" {
		t.Errorf("service dependency = %s -> %s, want semantic direction service -> repository", dependency.From, dependency.To)
	}
	dependent := result.Package.Dependents[0]
	if dependent.From != "example.com/app" || dependent.To != "example.com/service" {
		t.Errorf("service dependent = %s -> %s, want semantic direction app -> service", dependent.From, dependent.To)
	}
}

func TestNodeAPISerializesSymbolCentricTypeAndFunctionInspection(t *testing.T) {
	handler := Handler(webFixture(t))

	typeResponse := request(t, handler, http.MethodGet, "/api/node?id=example.com%2Fservice%3A%3AService")
	if typeResponse.Code != http.StatusOK {
		t.Fatalf("type status = %d, body = %s", typeResponse.Code, typeResponse.Body.String())
	}
	var typeResult nodeInspection
	decode(t, typeResponse, &typeResult)
	if typeResult.Type == nil || len(typeResult.Type.Methods) != 1 {
		t.Fatalf("type inspection = %#v", typeResult.Type)
	}
	method := typeResult.Type.Methods[0]
	if method.Ref != "example.com/service::Service::Create" || method.Kind != "function" || method.Name != "Create" ||
		method.ParentRef != "example.com/service::Service" || method.ParentName != "Service" {
		t.Errorf("method summary = %#v", method)
	}
	if len(typeResult.Type.DirectDependencies) != 1 || len(typeResult.Type.DirectDependents) != 0 {
		t.Fatalf("type exact relationships = %#v / %#v", typeResult.Type.DirectDependencies, typeResult.Type.DirectDependents)
	}
	direct := typeResult.Type.DirectDependencies[0]
	if direct.From.Ref != "example.com/service::Service" || direct.To.Ref != "example.com/service::Base" ||
		direct.To.ParentRef != "example.com/service" || direct.Kind != "embeds" {
		t.Errorf("type direct relationship = %#v", direct)
	}

	functionResponse := request(t, handler, http.MethodGet, "/api/node?id=example.com%2Fservice%3A%3AService%3A%3ACreate")
	if functionResponse.Code != http.StatusOK {
		t.Fatalf("function status = %d, body = %s", functionResponse.Code, functionResponse.Body.String())
	}
	var functionResult nodeInspection
	decode(t, functionResponse, &functionResult)
	detail := functionResult.Function
	if detail == nil || len(detail.Calls) != 1 || len(detail.CalledBy) != 1 || len(detail.Accepts) != 1 {
		t.Fatalf("function inspection = %#v", detail)
	}
	call := detail.Calls[0]
	if call.From.Ref != "example.com/service::Service::Create" || call.From.ParentName != "Service" ||
		call.To.Ref != "example.com/repository::Repository::Save" || call.To.ParentRef != "example.com/repository::Repository" ||
		call.Kind != "calls" || len(call.Evidence) != 1 || call.Evidence[0].Offset != 110 {
		t.Errorf("call relationship = %#v", call)
	}
	if detail.CalledBy[0].From.Ref != "example.com/app::Run" || detail.CalledBy[0].From.ParentRef != "example.com/app" {
		t.Errorf("called-by relationship = %#v", detail.CalledBy[0])
	}
	if detail.Accepts[0].To.Ref != "example.com/repository::Repository" || detail.Accepts[0].To.Kind != "interface" {
		t.Errorf("accepts relationship = %#v", detail.Accepts[0])
	}
}

func TestDependencyAPISeparatesTypeAndExactOnlyEvidence(t *testing.T) {
	handler := Handler(webFixture(t))

	typeBacked := request(t, handler, http.MethodGet, "/api/package-dependency?from=example.com%2Fservice&to=example.com%2Frepository")
	if typeBacked.Code != http.StatusOK {
		t.Fatalf("type-backed status = %d, body = %s", typeBacked.Code, typeBacked.Body.String())
	}
	var typeResult dependencyInspection
	decode(t, typeBacked, &typeResult)
	if len(typeResult.TypeDependencies) != 1 || len(typeResult.TypeDependencies[0].Evidence) != 2 || len(typeResult.ExactOnly) != 0 {
		t.Fatalf("type-backed inspection = %#v", typeResult)
	}
	typeDependency := typeResult.TypeDependencies[0]
	if typeDependency.From != "example.com/service::Service" || typeDependency.To != "example.com/repository::Repository" {
		t.Errorf("type dependency endpoints = %s -> %s, want full symbol refs", typeDependency.From, typeDependency.To)
	}

	exact := request(t, handler, http.MethodGet, "/api/package-dependency?from=example.com%2Fapp&to=example.com%2Fservice")
	var exactResult dependencyInspection
	decode(t, exact, &exactResult)
	if exact.Code != http.StatusOK || len(exactResult.TypeDependencies) != 0 || len(exactResult.ExactOnly) != 1 || exactResult.ExactOnly[0].Kind != "calls" {
		t.Errorf("exact-only inspection status = %d, result = %#v", exact.Code, exactResult)
	}
}

func TestAPIErrorsAreSmallJSONResponses(t *testing.T) {
	handler := Handler(webFixture(t))
	tests := []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodGet, path: "/api/node", status: http.StatusBadRequest},
		{method: http.MethodGet, path: "/api/node?id=missing", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/api/package-dependency?from=example.com%2Fapp", status: http.StatusBadRequest},
		{method: http.MethodGet, path: "/api/package-dependency?from=example.com%2Fapp&to=example.com%2Fimport-only", status: http.StatusNotFound},
		{method: http.MethodPost, path: "/api/packages", status: http.StatusMethodNotAllowed},
	}
	for _, test := range tests {
		response := request(t, handler, test.method, test.path)
		if response.Code != test.status {
			t.Errorf("%s %s status = %d, want %d", test.method, test.path, response.Code, test.status)
		}
		if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Errorf("%s content type = %q", test.path, response.Header().Get("Content-Type"))
		}
		var body map[string]string
		decode(t, response, &body)
		if body["error"] == "" || len(body) != 1 {
			t.Errorf("%s body = %#v", test.path, body)
		}
	}

	unavailable := request(t, Handler(nil), http.MethodGet, "/api/packages")
	if unavailable.Code != http.StatusInternalServerError || unavailable.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("unavailable graph response = status %d, content type %q", unavailable.Code, unavailable.Header().Get("Content-Type"))
	}
	var body map[string]string
	decode(t, unavailable, &body)
	if body["error"] == "" || len(body) != 1 {
		t.Errorf("unavailable graph body = %#v", body)
	}
}

func request(t *testing.T, handler http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, path, nil))
	return response
}

func decode(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode %q: %v", response.Body.String(), err)
	}
}

func webFixture(t *testing.T) *graph.Graph {
	t.Helper()
	const (
		app         = graph.SymbolRef("example.com/app")
		service     = graph.SymbolRef("example.com/service")
		repository  = graph.SymbolRef("example.com/repository")
		importOnly  = graph.SymbolRef("example.com/import-only")
		appRun      = graph.SymbolRef("example.com/app::Run")
		serviceT    = graph.SymbolRef("example.com/service::Service")
		serviceBase = graph.SymbolRef("example.com/service::Base")
		serviceRun  = graph.SymbolRef("example.com/service::Service::Create")
		repoT       = graph.SymbolRef("example.com/repository::Repository")
		repoSave    = graph.SymbolRef("example.com/repository::Repository::Save")
	)
	g := graph.New()
	nodes := []webNode{
		{ID: app, Kind: graph.NodePackage, Name: "app", Documentation: "Package app starts the workflow."},
		{ID: service, Kind: graph.NodePackage, Name: "service", Documentation: "Package service owns application behavior."},
		{ID: repository, Kind: graph.NodePackage, Name: "repository"},
		{ID: importOnly, Kind: graph.NodePackage, Name: "importonly"},
		{ID: appRun, Kind: graph.NodeFunction, Name: "Run", Parent: app, Location: graph.Location{File: "app.go", Offset: 40}},
		{ID: serviceT, Kind: graph.NodeStruct, Name: "Service", Parent: service, Location: graph.Location{File: "service.go", Offset: 20}},
		{ID: serviceBase, Kind: graph.NodeStruct, Name: "Base", Parent: service, Location: graph.Location{File: "service.go", Offset: 30}},
		{ID: serviceRun, Kind: graph.NodeFunction, Name: "Create", Parent: serviceT, Location: graph.Location{File: "service.go", Offset: 80}},
		{ID: repoT, Kind: graph.NodeInterface, Name: "Repository", Parent: repository, Location: graph.Location{File: "repository.go", Offset: 15}},
		{ID: repoSave, Kind: graph.NodeFunction, Name: "Save", Parent: repoT, Location: graph.Location{File: "repository.go", Offset: 45}},
	}
	for _, node := range nodes {
		if err := addWebNode(g, node); err != nil {
			t.Fatalf("AddNode(%s): %v", node.ID, err)
		}
	}
	edges := []webEdge{
		{From: app, To: service, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Offset: 8}}},
		{From: app, To: importOnly, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Offset: 16}}},
		{From: appRun, To: serviceRun, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "app.go", Offset: 70}}},
		{From: serviceRun, To: repoSave, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "service.go", Offset: 110}}},
		{From: serviceRun, To: repoT, Kind: graph.EdgeAccepts, Evidence: []graph.Location{{File: "service.go", Offset: 90}}},
		{From: serviceT, To: serviceBase, Kind: graph.EdgeEmbeds, Evidence: []graph.Location{{File: "service.go", Offset: 35}}},
	}
	for _, edge := range edges {
		if err := addWebEdge(g, edge); err != nil {
			t.Fatalf("AddEdge(%s): %v", edge.Kind, err)
		}
	}
	return g
}

type webNode struct {
	ID            graph.SymbolRef
	Kind          graph.NodeKind
	Name          string
	Parent        graph.SymbolRef
	Location      graph.Location
	Documentation string
}

type webEdge struct {
	From     graph.SymbolRef
	To       graph.SymbolRef
	Kind     graph.EdgeKind
	Evidence []graph.Location
}

func addWebNode(g *graph.Graph, node webNode) error {
	var parent graph.NodeID
	if node.Parent != "" {
		var exists bool
		parent, exists = g.Resolve(node.Parent)
		if !exists {
			return fmt.Errorf("parent %q does not exist", node.Parent)
		}
	}
	_, err := g.AddNode(graph.Node{
		Ref: node.ID, Kind: node.Kind, Name: node.Name, Parent: parent,
		Location: node.Location, Documentation: node.Documentation,
	})
	return err
}

func addWebEdge(g *graph.Graph, edge webEdge) error {
	from, fromExists := g.Resolve(edge.From)
	to, toExists := g.Resolve(edge.To)
	if !fromExists || !toExists {
		return fmt.Errorf("edge endpoint missing: %q -> %q", edge.From, edge.To)
	}
	return g.AddEdge(graph.Edge{From: from, To: to, Kind: edge.Kind, Evidence: edge.Evidence})
}
