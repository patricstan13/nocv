package webui

import (
	"encoding/json"
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

func TestDependencyAPISeparatesTypeAndExactOnlyEvidence(t *testing.T) {
	handler := Handler(webFixture(t))

	typeBacked := request(t, handler, http.MethodGet, "/api/package-dependency?from=example.com%2Fservice&to=example.com%2Frepository")
	if typeBacked.Code != http.StatusOK {
		t.Fatalf("type-backed status = %d, body = %s", typeBacked.Code, typeBacked.Body.String())
	}
	var typeResult dependencyInspection
	decode(t, typeBacked, &typeResult)
	if len(typeResult.TypeDependencies) != 1 || len(typeResult.TypeDependencies[0].Evidence) != 2 || len(typeResult.ExactOnly) != 0 {
		t.Errorf("type-backed inspection = %#v", typeResult)
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
		app        = graph.SymbolID("example.com/app")
		service    = graph.SymbolID("example.com/service")
		repository = graph.SymbolID("example.com/repository")
		importOnly = graph.SymbolID("example.com/import-only")
		appRun     = graph.SymbolID("example.com/app::Run")
		serviceT   = graph.SymbolID("example.com/service::Service")
		serviceRun = graph.SymbolID("example.com/service::Service::Create")
		repoT      = graph.SymbolID("example.com/repository::Repository")
		repoSave   = graph.SymbolID("example.com/repository::Repository::Save")
	)
	g := graph.New()
	nodes := []graph.Node{
		{ID: app, Kind: graph.NodePackage, Name: "app", Documentation: "Package app starts the workflow."},
		{ID: service, Kind: graph.NodePackage, Name: "service", Documentation: "Package service owns application behavior."},
		{ID: repository, Kind: graph.NodePackage, Name: "repository"},
		{ID: importOnly, Kind: graph.NodePackage, Name: "importonly"},
		{ID: appRun, Kind: graph.NodeFunction, Name: "Run", Parent: app, Location: graph.Location{File: "app.go", Offset: 40}},
		{ID: serviceT, Kind: graph.NodeStruct, Name: "Service", Parent: service, Location: graph.Location{File: "service.go", Offset: 20}},
		{ID: serviceRun, Kind: graph.NodeFunction, Name: "Create", Parent: serviceT, Location: graph.Location{File: "service.go", Offset: 80}},
		{ID: repoT, Kind: graph.NodeInterface, Name: "Repository", Parent: repository, Location: graph.Location{File: "repository.go", Offset: 15}},
		{ID: repoSave, Kind: graph.NodeFunction, Name: "Save", Parent: repoT, Location: graph.Location{File: "repository.go", Offset: 45}},
	}
	for _, node := range nodes {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("AddNode(%s): %v", node.ID, err)
		}
	}
	edges := []graph.Edge{
		{From: app, To: service, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Offset: 8}}},
		{From: app, To: importOnly, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Offset: 16}}},
		{From: appRun, To: serviceRun, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "app.go", Offset: 70}}},
		{From: serviceRun, To: repoSave, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "service.go", Offset: 110}}},
		{From: serviceRun, To: repoT, Kind: graph.EdgeAccepts, Evidence: []graph.Location{{File: "service.go", Offset: 90}}},
	}
	for _, edge := range edges {
		if err := g.AddEdge(edge); err != nil {
			t.Fatalf("AddEdge(%s): %v", edge.Kind, err)
		}
	}
	return g
}
