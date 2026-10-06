package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/internal/testutil"
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

func TestClientFormatsLocationsAsLineAndColumn(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`value.line && value.column ? ":" + value.line + ":" + value.column`,
		`location.line && location.column ? ":" + location.line + ":" + location.column`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("client source lacks line-and-column formatting %q", required)
		}
	}
	if strings.Contains(client, "location.offset") || strings.Contains(client, "value.offset") {
		t.Error("client still formats byte offsets as source locations")
	}
}

func TestClientShowsPersistentPartialAnalysisIndicator(t *testing.T) {
	markup := readAsset(t, "static/index.html")
	client := readAsset(t, "static/app.js")
	styles := readAsset(t, "static/app.css")
	for source, required := range map[string][]string{
		markup: {`id="analysis-status"`, `role="status"`, `hidden`},
		client: {`getJSON("/api/status")`, `result.status !== "partial"`, `"Partial analysis"`, `analysisStatusIndicator.hidden = false`},
		styles: {`.analysis-status`, `.analysis-status[hidden]`},
	} {
		for _, fragment := range required {
			if !strings.Contains(source, fragment) {
				t.Errorf("status UI source lacks %q", fragment)
			}
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
		`nonEmptySection("Types", detail.types, packageContentItem)`,
		`nonEmptySection("Functions", detail.functions, packageContentItem)`,
		`nonEmptySection("Go imports (" + (detail.imports || []).length + ")", detail.imports, dependencyTargetItem)`,
		`nonEmptySection("Imported by (" + (detail.importers || []).length + ")", detail.importers, dependencySourceItem)`,
		`const details = element("details", undefined, className || "advanced-disclosure")`,
		`parts.push(disclosure("Advanced", advanced))`,
		`collapsibleRelationships(facts, 2, "Show evidence")`,
		`collapsibleRelationships(exact, 5, "Show relationships")`,
		`semanticFrom + " depends on " + semanticTo`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("client source lacks %q", required)
		}
	}
	for _, exhaustive := range []string{`contentsSummary(detail)`, `+ " types"`, `+ " functions"`, `parts.push(...section("Imports"`, `parts.push(...section("Imported by"`} {
		if strings.Contains(client, exhaustive) {
			t.Errorf("client still renders exhaustive package detail with %q", exhaustive)
		}
	}
}

func TestClientRendersPackageContentsAsNavigableSymbols(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`function packageContentItem(node)`,
		`return symbolItem({`,
		`ref: node.id`,
		`kind: node.kind`,
		`name: node.name`,
		`nonEmptySection("Types", detail.types, packageContentItem)`,
		`nonEmptySection("Functions", detail.functions, packageContentItem)`,
		`button.addEventListener("click", () => inspectSymbol(summary.ref))`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("package content navigation source lacks %q", required)
		}
	}
}

func TestClientNavigatesSemanticDependencyEndpointsToPackages(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`const semanticFrom = result.dependency.from`,
		`const semanticTo = result.dependency.to`,
		`section("From", [semanticFrom], packageEndpointItem)`,
		`section("To", [semanticTo], packageEndpointItem)`,
		`function packageEndpointItem(ref)`,
		`button.addEventListener("click", () => selectPackage(ref, true))`,
		`function selectPackage(ref, focusNode)`,
		`showPackage(ref)`,
		`packageDependencyOriginRef = null`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("dependency endpoint navigation source lacks %q", required)
		}
	}
}

func TestClientRendersNavigableSymbolInspectors(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`function symbolDisplayName(summary)`,
		`return summary.parentName + "." + summary.name`,
		`function renderSymbolButton(summary)`,
		`button.title = summary.ref`,
		`button.addEventListener("click", () => inspectSymbol(summary.ref))`,
		`nonEmptySection("Methods", detail.methods, symbolItem)`,
		`button.addEventListener("click", () => inspectSymbol(ref))`,
		`const method = result.node.parentKind === "struct" || result.node.parentKind === "interface"`,
		`function callableHeading(result)`,
		`return method ? result.node.parentName + "." + result.node.name : result.node.name`,
		`parts.push(element("h3", "Owner"))`,
		`ref: result.node.parent`,
		`name: result.node.parentName`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("symbol inspector source lacks %q", required)
		}
	}
}

func TestClientRendersCategorizedFunctionRelationshipsWithEvidenceDisclosure(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`nonEmptySection("Calls", detail.calls, (value) => symbolRelationshipItem(value, false))`,
		`nonEmptySection("Called by", detail.calledBy, (value) => symbolRelationshipItem(value, true))`,
		`nonEmptySection("Accepts", detail.accepts, (value) => symbolRelationshipItem(value, false))`,
		`nonEmptySection("Returns", detail.returns, (value) => symbolRelationshipItem(value, false))`,
		`nonEmptySection("Implements", detail.implements, (value) => symbolRelationshipItem(value, false))`,
		`nonEmptySection("Implemented by", detail.implementedBy, (value) => symbolRelationshipItem(value, true))`,
		`const summary = incoming ? relationship.from : relationship.to`,
		`function evidenceDisclosure(evidence)`,
		`return disclosure("Evidence (" + evidence.length + ")", [list], "evidence-disclosure")`,
		`element("p", "No semantic relationships.", "muted semantic-empty")`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("categorized function inspector source lacks %q", required)
		}
	}
}

func TestClientProtectsInspectorFromStaleRequests(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`let inspectorRequestGeneration = 0`,
		`function beginInspectorRequest(label)`,
		`const generation = ++inspectorRequestGeneration`,
		`element("h2", "Loading " + (label || "selection") + "…")`,
		`async function inspectSymbol(ref)`,
		`const generation = beginInspectorRequest(typeLabel(ref))`,
		`if (generation !== inspectorRequestGeneration) return`,
		`renderNodeInspection(result)`,
		`function showPackage(id)`,
		`inspectSymbol(id)`,
		`inspectSymbol(params.nodes[0])`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("stale-request protection source lacks %q", required)
		}
	}
	if strings.Count(client, `if (generation !== inspectorRequestGeneration) return`) < 4 {
		t.Error("node and dependency success/error paths must all reject stale responses")
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
		`renderPackageDependencyInspection(currentPackageDependency.result)`,
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
		`inspectSymbol(params.nodes[0])`,
		`getJSON("/api/node?id=" + encodeURIComponent(ref))`,
		`disclosure("Advanced · Exact relationships", exactParts)`,
		`backToPackages.addEventListener("click", showPackageGraph)`,
		`packageSearchControl.setEnabled(false)`,
		`packageSearchControl.setEnabled(true)`,
		`packageNetwork.redraw()`,
		`renderPackageDependencyInspection(currentPackageDependency.result)`,
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

func TestClientProvidesSignatureChangeWorkflow(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`let inspectorState = { kind: "symbol", inspection: null, proposal: null, result: null }`,
		`renderWorkflowButton("Analyze signature change"`,
		`result.function.signature.parameters.map((parameter) => parameter.type.display)`,
		`result.function.signature.results || []`,
		`"+ Add parameter"`,
		`"+ Add result"`,
		`"Remove parameter " + (index + 1)`,
		`"Remove result " + (index + 1)`,
		`"Final parameter is variadic"`,
		`renderWorkflowButton("Analyze"`,
		`renderWorkflowButton("Cancel"`,
		`postJSON("/api/signature-impact"`,
		`callable: state.inspection.node.id`,
		`parameters: parameters.map((type) => ({ type }))`,
		`results: results.map((type) => ({ type }))`,
		`variadic: state.proposal.variadic`,
		`renderWorkflowButton("Edit proposed signature"`,
		`renderWorkflowButton("Back to " + callableKind(state.inspection)`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("signature-change workflow source lacks %q", required)
		}
	}
	if strings.Count(client, `renderWorkflowButton("Analyze signature change"`) != 1 {
		t.Error("signature-change action must be defined only by the function/method inspector")
	}
}

func TestClientRendersSignatureImpactReadModel(t *testing.T) {
	client := readAsset(t, "static/app.js")
	for _, required := range []string{
		`element("h3", "Call-site impact")`,
		`site.compatibility === "incompatible"`,
		`site.compatibility === "compatible"`,
		`site.compatibility === "unknown"`,
		`"Show compatible call sites (" + compatible.length + ")"`,
		`renderSymbolButton(site.caller)`,
		`element("h3", "Compiler consequences")`,
		`No new compile/type-check diagnostics were observed in the affected scope.`,
		`consequence.classification === "uncertain"`,
		`renderSymbolButton(consequence.symbol)`,
		`element("h3", "Contract impact")`,
		`renderSymbolButton(contract.concrete)`,
		`renderSymbolButton(contract.interface)`,
		`element("h3", "Structural impact")`,
		`structural.exposure === "pointer only"`,
		`name: "*" + structural.type.name`,
		`renderSymbolButton(structural.originMethod)`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("signature-impact rendering source lacks %q", required)
		}
	}
}

func TestClientGuardsSignatureImpactRequestsAgainstStaleResults(t *testing.T) {
	client := readAsset(t, "static/app.js")
	start := strings.Index(client, "async function requestSignatureImpact(state)")
	end := strings.Index(client[start:], "function renderCallSiteProblem")
	if start < 0 || end < 0 {
		t.Fatal("requestSignatureImpact helper is missing")
	}
	requestSource := client[start : start+end]
	for _, required := range []string{
		`const generation = ++inspectorRequestGeneration`,
		`if (generation !== inspectorRequestGeneration) return`,
		`renderSignatureImpact({ ...state, kind: "signature-result", result })`,
		`renderSignatureChangeEditor(state, error.message)`,
	} {
		if !strings.Contains(requestSource, required) {
			t.Errorf("signature-impact stale guard lacks %q", required)
		}
	}
	if strings.Count(requestSource, `if (generation !== inspectorRequestGeneration) return`) != 2 {
		t.Error("signature-impact success and error paths must both reject stale responses")
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
	handler := graphHandler(webFixture(t))

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
	handler := graphHandler(webFixture(t))
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
		{ID: edgeID("example.com/app", "example.com/service"), From: "example.com/app", To: "example.com/service", Certainty: "confirmed"},
		{ID: edgeID("example.com/service", "example.com/repository"), From: "example.com/service", To: "example.com/repository", Certainty: "confirmed"},
	}
	if !reflect.DeepEqual(result.Nodes, wantNodes) {
		t.Errorf("nodes = %#v, want %#v", result.Nodes, wantNodes)
	}
	if !reflect.DeepEqual(result.Edges, wantEdges) {
		t.Errorf("edges = %#v, want semantic-only %#v", result.Edges, wantEdges)
	}
}

func TestRelationshipCertaintyIsExplicitInAPIModels(t *testing.T) {
	g := graph.New()
	for _, node := range []webNode{
		{ID: "example.com/source", Kind: graph.NodePackage, Name: "source"},
		{ID: "example.com/target", Kind: graph.NodePackage, Name: "target"},
		{ID: "example.com/source::Concrete", Kind: graph.NodeStruct, Name: "Concrete", Parent: "example.com/source"},
		{ID: "example.com/target::Contract", Kind: graph.NodeInterface, Name: "Contract", Parent: "example.com/target"},
	} {
		if err := addWebNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	if err := addWebEdge(g, webEdge{
		From: "example.com/source::Concrete", To: "example.com/target::Contract",
		Kind: graph.EdgeImplements, Certainty: graph.RelationshipUncertain,
		Evidence: []graph.Location{{File: "source.go", Line: 3, Column: 1}},
	}); err != nil {
		t.Fatal(err)
	}

	handler := graphHandler(g)
	packagesResponse := request(t, handler, http.MethodGet, "/api/packages")
	if packagesResponse.Code != http.StatusOK {
		t.Fatalf("packages status = %d, body = %s", packagesResponse.Code, packagesResponse.Body.String())
	}
	var packages packageGraph
	decode(t, packagesResponse, &packages)
	if len(packages.Edges) != 1 || packages.Edges[0].Certainty != "uncertain" {
		t.Fatalf("package edges = %#v, want explicit uncertain certainty", packages.Edges)
	}

	nodeResponse := request(t, handler, http.MethodGet, "/api/node?id=example.com/source::Concrete")
	if nodeResponse.Code != http.StatusOK {
		t.Fatalf("node status = %d, body = %s", nodeResponse.Code, nodeResponse.Body.String())
	}
	var node nodeInspection
	decode(t, nodeResponse, &node)
	if node.Type == nil || len(node.Type.Dependencies) != 1 ||
		node.Type.Dependencies[0].Certainty != "uncertain" ||
		len(node.Type.DirectDependencies) != 1 ||
		node.Type.DirectDependencies[0].Certainty != "uncertain" {
		t.Fatalf("node inspection lost certainty: %#v", node)
	}
}

func TestAnalyzerProducedUncertainImplementationReachesAPI(t *testing.T) {
	dir := t.TempDir()
	for name, contents := range map[string]string{
		"go.mod": "module example.com/webcertainty\n\ngo 1.22\n",
		"certainty.go": `package webcertainty

type Contract interface { Required() }
type MissingAlias = MissingDependency
type Candidate struct { MissingAlias }
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), dir, ".")
	if err != nil {
		t.Fatal(err)
	}

	response := request(t, Handler(analysis), http.MethodGet, "/api/node?id=example.com%2Fwebcertainty::Candidate")
	if response.Code != http.StatusOK {
		t.Fatalf("node status = %d, body = %s", response.Code, response.Body.String())
	}
	var node nodeInspection
	decode(t, response, &node)
	if node.Type == nil || len(node.Type.DirectDependencies) != 1 ||
		node.Type.DirectDependencies[0].Kind != "implements" ||
		node.Type.DirectDependencies[0].Certainty != "uncertain" {
		t.Fatalf("analyzer-produced implementation response = %#v", node)
	}
}

func TestAnalysisStatusAPIReportsCompleteAndPartial(t *testing.T) {
	tests := []struct {
		name     string
		analysis *goanalyzer.Analysis
		want     analysisStatus
	}{
		{
			name:     "complete",
			analysis: parameterImpactAnalysis(t),
			want:     analysisStatus{Status: "complete", Reasons: []analysisStatusReason{}},
		},
		{
			name:     "partial",
			analysis: statusAnalysis(t),
			want: analysisStatus{Status: "partial", Reasons: []analysisStatusReason{
				{Kind: "incomplete type information", Package: "example.com/shop/status/brokenone"},
				{Kind: "incomplete type information", Package: "example.com/shop/status/brokentwo"},
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, Handler(test.analysis), http.MethodGet, "/api/status")
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var got analysisStatus
			decode(t, response, &got)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("analysis status = %#v, want %#v", got, test.want)
			}
		})
	}

	wrongMethod := request(t, Handler(parameterImpactAnalysis(t)), http.MethodPost, "/api/status")
	if wrongMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", wrongMethod.Code)
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

	response := request(t, graphHandler(g), http.MethodGet, "/api/packages")
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
	handler := graphHandler(webFixture(t))
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
	response := request(t, graphHandler(webFixture(t)), http.MethodGet, "/api/node?id=example.com%2Fservice")
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
	handler := graphHandler(webFixture(t))

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
	if functionResult.Node.Parent != "example.com/service::Service" || functionResult.Node.ParentName != "Service" || functionResult.Node.ParentKind != "struct" {
		t.Errorf("function parent presentation = %#v", functionResult.Node)
	}
	if functionResult.Node.Location == nil || functionResult.Node.Location.Line != 8 || functionResult.Node.Location.Column != 18 {
		t.Errorf("function location = %#v, want service.go:8:18", functionResult.Node.Location)
	}
	detail := functionResult.Function
	if detail == nil || len(detail.Calls) != 1 || len(detail.CalledBy) != 1 || len(detail.Accepts) != 1 {
		t.Fatalf("function inspection = %#v", detail)
	}
	call := detail.Calls[0]
	if call.From.Ref != "example.com/service::Service::Create" || call.From.ParentName != "Service" ||
		call.To.Ref != "example.com/repository::Repository::Save" || call.To.ParentRef != "example.com/repository::Repository" ||
		call.Kind != "calls" || len(call.Evidence) != 1 || call.Evidence[0].Line != 11 || call.Evidence[0].Column != 3 {
		t.Errorf("call relationship = %#v", call)
	}
	if strings.Contains(functionResponse.Body.String(), `"offset"`) {
		t.Errorf("function response exposes byte offset: %s", functionResponse.Body.String())
	}
	if detail.CalledBy[0].From.Ref != "example.com/app::Run" || detail.CalledBy[0].From.ParentRef != "example.com/app" {
		t.Errorf("called-by relationship = %#v", detail.CalledBy[0])
	}
	if detail.Accepts[0].To.Ref != "example.com/repository::Repository" || detail.Accepts[0].To.Kind != "interface" {
		t.Errorf("accepts relationship = %#v", detail.Accepts[0])
	}
}

func TestAnalysisBackedNodeAPIIncludesCallableSignature(t *testing.T) {
	analysis := parameterImpactAnalysis(t)
	handler := Handler(analysis)

	response := request(t, handler, http.MethodGet, "/api/node?id=example.com%2Fshop%2Fparameterimpact%3A%3AVariadic")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var result nodeInspection
	decode(t, response, &result)
	if result.Function == nil || result.Function.Signature == nil {
		t.Fatalf("function signature is missing: %#v", result.Function)
	}
	signature := result.Function.Signature
	if !signature.Variadic || len(signature.Parameters) != 1 || signature.Parameters[0].Type.Display != "string" {
		t.Fatalf("signature = %#v, want variadic string", signature)
	}

	resultMethod := request(t, handler, http.MethodGet, "/api/node?id=example.com%2Fshop%2Fparameterimpact%3A%3AResultBase%3A%3AFetch")
	var resultMethodInspection nodeInspection
	decode(t, resultMethod, &resultMethodInspection)
	if resultMethod.Code != http.StatusOK || resultMethodInspection.Function == nil || resultMethodInspection.Function.Signature == nil ||
		len(resultMethodInspection.Function.Signature.Results) != 1 || resultMethodInspection.Function.Signature.Results[0].Type.Display != "Item" {
		t.Fatalf("result signature response = status %d, result %#v", resultMethod.Code, resultMethodInspection)
	}

	qualified := request(t, handler, http.MethodGet, "/api/node?id=example.com%2Fshop%2Fparameterimpact%3A%3ACallsMethod")
	var qualifiedResult nodeInspection
	decode(t, qualified, &qualifiedResult)
	if qualified.Code != http.StatusOK || qualifiedResult.Function == nil || qualifiedResult.Function.Signature == nil {
		t.Fatalf("qualified signature response = status %d, result %#v", qualified.Code, qualifiedResult)
	}
	parameters := qualifiedResult.Function.Signature.Parameters
	if len(parameters) != 2 || parameters[0].Type.Display != "Service" || parameters[1].Type.Display != "ID" {
		t.Fatalf("editable qualified signature = %#v, want package-local Go expressions", parameters)
	}

	interfaceMethod := request(t, handler, http.MethodGet, "/api/node?id=example.com%2Fshop%2Fparameterimpact%3A%3AStore%3A%3ASave")
	var interfaceResult nodeInspection
	decode(t, interfaceMethod, &interfaceResult)
	if interfaceMethod.Code != http.StatusOK || interfaceResult.Function == nil || interfaceResult.Function.Signature == nil {
		t.Fatalf("interface method lacks callable signature: status %d, result %#v", interfaceMethod.Code, interfaceResult)
	}
}

func TestSignatureImpactAPIUsesAnalysisReadModel(t *testing.T) {
	analysis := parameterImpactAnalysis(t)
	handler := Handler(analysis)

	tests := []struct {
		name       string
		body       string
		assertions func(*testing.T, signatureImpact)
	}{
		{
			name: "call sites and contracts",
			body: `{"callable":"example.com/shop/parameterimpact::Service::Save","parameters":[{"type":"string"}],"variadic":false}`,
			assertions: func(t *testing.T, result signatureImpact) {
				if len(result.CallSites) == 0 || len(result.Contracts) == 0 {
					t.Fatalf("impact lacks call sites or contracts: %#v", result)
				}
				if result.CallSites[0].Compatibility == "" || result.Contracts[0].Kind != "lost implementation" {
					t.Fatalf("impact presentation = %#v", result)
				}
			},
		},
		{
			name: "structural exposure",
			body: `{"callable":"example.com/shop/parameterimpact::PromotionBase::Change","parameters":[{"type":"string"}]}`,
			assertions: func(t *testing.T, result signatureImpact) {
				if len(result.Structural) == 0 {
					t.Fatalf("impact lacks structural consequences: %#v", result)
				}
				if result.Structural[0].Exposure == "" || result.Structural[0].OriginMethod.Ref == "" {
					t.Fatalf("structural presentation = %#v", result.Structural)
				}
			},
		},
		{
			name: "results and compiler consequences",
			body: `{"callable":"example.com/shop/parameterimpact::ResultBase::Fetch","parameters":[{"type":"ID"}],"results":[{"type":"*Item"}]}`,
			assertions: func(t *testing.T, result signatureImpact) {
				if len(result.After.Results) != 1 || result.After.Results[0].Type.Display == "" || len(result.Compiler.AffectedPackages) == 0 || len(result.Compiler.Consequences) == 0 {
					t.Fatalf("result impact presentation = %#v", result)
				}
			},
		},
		{
			name: "variadic",
			body: `{"callable":"example.com/shop/parameterimpact::Variadic","parameters":[{"type":"string"}],"variadic":true}`,
			assertions: func(t *testing.T, result signatureImpact) {
				if !result.After.Variadic || len(result.CallSites) != 0 || len(result.Compiler.Consequences) != 0 {
					t.Fatalf("variadic impact = %#v", result)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := requestBody(t, handler, http.MethodPost, "/api/signature-impact", test.body)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var result signatureImpact
			decode(t, response, &result)
			test.assertions(t, result)
		})
	}
}

func TestSignatureImpactAPIErrorsAreConcise(t *testing.T) {
	handler := Handler(parameterImpactAnalysis(t))
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "invalid symbol", body: `{"callable":"example.com/shop/parameterimpact::Missing","parameters":[]}`, want: "unknown symbol"},
		{name: "invalid type", body: `{"callable":"example.com/shop/parameterimpact::UseID","parameters":[{"type":"DoesNotExist"}]}`, want: "resolve proposed parameter 1 type"},
		{name: "invalid result type", body: `{"callable":"example.com/shop/parameterimpact::UseID","parameters":[{"type":"ID"}],"results":[{"type":"DoesNotExist"}]}`, want: "resolve proposed result 1 type"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := requestBody(t, handler, http.MethodPost, "/api/signature-impact", test.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
			}
			var result map[string]string
			decode(t, response, &result)
			if !strings.Contains(result["error"], test.want) {
				t.Fatalf("error = %q, want containing %q", result["error"], test.want)
			}
		})
	}

	wrongMethod := request(t, handler, http.MethodGet, "/api/signature-impact")
	if wrongMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", wrongMethod.Code)
	}
}

func TestDependencyAPISeparatesTypeAndExactOnlyEvidence(t *testing.T) {
	handler := graphHandler(webFixture(t))

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
	handler := graphHandler(webFixture(t))
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
	return requestBody(t, handler, method, path, "")
}

func requestBody(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
	return response
}

func parameterImpactAnalysis(t *testing.T) *goanalyzer.Analysis {
	t.Helper()
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), testutil.GoProjectDir(t), "./parameterimpact")
	if err != nil {
		t.Fatal(err)
	}
	return analysis
}

func statusAnalysis(t *testing.T) *goanalyzer.Analysis {
	t.Helper()
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), testutil.PartialGoProject(t), "./status/...")
	if err != nil {
		t.Fatal(err)
	}
	return analysis
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
		{ID: appRun, Kind: graph.NodeFunction, Name: "Run", Parent: app, Location: graph.Location{File: "app.go", Line: 4, Column: 1, Offset: 40}},
		{ID: serviceT, Kind: graph.NodeStruct, Name: "Service", Parent: service, Location: graph.Location{File: "service.go", Line: 2, Column: 6, Offset: 20}},
		{ID: serviceBase, Kind: graph.NodeStruct, Name: "Base", Parent: service, Location: graph.Location{File: "service.go", Line: 3, Column: 6, Offset: 30}},
		{ID: serviceRun, Kind: graph.NodeFunction, Name: "Create", Parent: serviceT, Location: graph.Location{File: "service.go", Line: 8, Column: 18, Offset: 80}},
		{ID: repoT, Kind: graph.NodeInterface, Name: "Repository", Parent: repository, Location: graph.Location{File: "repository.go", Line: 2, Column: 6, Offset: 15}},
		{ID: repoSave, Kind: graph.NodeFunction, Name: "Save", Parent: repoT, Location: graph.Location{File: "repository.go", Line: 4, Column: 2, Offset: 45}},
	}
	for _, node := range nodes {
		if err := addWebNode(g, node); err != nil {
			t.Fatalf("AddNode(%s): %v", node.ID, err)
		}
	}
	edges := []webEdge{
		{From: app, To: service, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Line: 1, Column: 8, Offset: 8}}},
		{From: app, To: importOnly, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "app.go", Line: 2, Column: 8, Offset: 16}}},
		{From: appRun, To: serviceRun, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "app.go", Line: 7, Column: 2, Offset: 70}}},
		{From: serviceRun, To: repoSave, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "service.go", Line: 11, Column: 3, Offset: 110}}},
		{From: serviceRun, To: repoT, Kind: graph.EdgeAccepts, Evidence: []graph.Location{{File: "service.go", Line: 9, Column: 4, Offset: 90}}},
		{From: serviceT, To: serviceBase, Kind: graph.EdgeEmbeds, Evidence: []graph.Location{{File: "service.go", Line: 3, Column: 2, Offset: 35}}},
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
	From      graph.SymbolRef
	To        graph.SymbolRef
	Kind      graph.EdgeKind
	Certainty graph.RelationshipCertainty
	Evidence  []graph.Location
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
	certainty := edge.Certainty
	if certainty == graph.RelationshipCertaintyUnknown {
		certainty = graph.RelationshipConfirmed
	}
	return g.AddEdge(graph.Edge{
		From: from, To: to, Kind: edge.Kind, Certainty: certainty,
		Evidence: edge.Evidence,
	})
}
