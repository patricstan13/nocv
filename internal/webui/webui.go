// Package webui serves NOCV's read-only package dependency explorer.
package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/query"
)

//go:embed static
var assets embed.FS

type packageGraph struct {
	Nodes []packageNode `json:"nodes"`
	Edges []packageEdge `json:"edges"`
}

type packageNode struct {
	ID    graph.SymbolRef `json:"id"`
	Label string          `json:"label"`
}

type packageEdge struct {
	ID   string          `json:"id"`
	From graph.SymbolRef `json:"from"`
	To   graph.SymbolRef `json:"to"`
}

type nodeInspection struct {
	Node     nodeInfo            `json:"node"`
	Package  *packageInspection  `json:"package,omitempty"`
	Type     *typeInspection     `json:"type,omitempty"`
	Function *functionInspection `json:"function,omitempty"`
}

type nodeInfo struct {
	ID            graph.SymbolRef `json:"id"`
	Name          string          `json:"name"`
	Kind          string          `json:"kind"`
	Parent        graph.SymbolRef `json:"parent,omitempty"`
	ParentName    string          `json:"parentName,omitempty"`
	ParentKind    string          `json:"parentKind,omitempty"`
	Location      *location       `json:"location,omitempty"`
	Documentation string          `json:"documentation,omitempty"`
}

type location struct {
	File   string `json:"file"`
	Offset int    `json:"offset"`
}

type packageInspection struct {
	Types        []nodeInfo          `json:"types"`
	Functions    []nodeInfo          `json:"functions"`
	Dependencies []packageDependency `json:"dependencies"`
	Dependents   []packageDependency `json:"dependents"`
	Imports      []relationship      `json:"imports"`
	Importers    []relationship      `json:"importers"`
}

type typeInspection struct {
	Methods            []symbolSummary      `json:"methods"`
	Dependencies       []typeDependency     `json:"dependencies"`
	Dependents         []typeDependency     `json:"dependents"`
	DirectDependencies []symbolRelationship `json:"directDependencies"`
	DirectDependents   []symbolRelationship `json:"directDependents"`
}

type functionInspection struct {
	Calls         []symbolRelationship `json:"calls"`
	CalledBy      []symbolRelationship `json:"calledBy"`
	Accepts       []symbolRelationship `json:"accepts"`
	Returns       []symbolRelationship `json:"returns"`
	Implements    []symbolRelationship `json:"implements"`
	ImplementedBy []symbolRelationship `json:"implementedBy"`
	Signature     *callableSignature   `json:"signature,omitempty"`
}

type symbolSummary struct {
	Ref        graph.SymbolRef `json:"ref"`
	Kind       string          `json:"kind"`
	Name       string          `json:"name"`
	ParentRef  graph.SymbolRef `json:"parentRef,omitempty"`
	ParentName string          `json:"parentName,omitempty"`
}

type symbolRelationship struct {
	From     symbolSummary `json:"from"`
	To       symbolSummary `json:"to"`
	Kind     string        `json:"kind"`
	Evidence []location    `json:"evidence"`
}

type packageDependency struct {
	From     graph.SymbolRef `json:"from"`
	To       graph.SymbolRef `json:"to"`
	Evidence []relationship  `json:"evidence"`
}

type typeDependency struct {
	From     graph.SymbolRef `json:"from"`
	To       graph.SymbolRef `json:"to"`
	Evidence []relationship  `json:"evidence"`
}

type relationship struct {
	From     graph.SymbolRef `json:"from"`
	To       graph.SymbolRef `json:"to"`
	Kind     string          `json:"kind"`
	Evidence []location      `json:"evidence"`
}

type dependencyInspection struct {
	Dependency       packageDependency `json:"dependency"`
	TypeDependencies []typeDependency  `json:"typeDependencies"`
	ExactOnly        []relationship    `json:"exactOnly"`
}

// Handler returns a self-contained HTTP handler over one retained Go analysis.
// Compiler state remains private to goanalyzer; HTTP responses contain only
// detached read models.
func Handler(analysis *goanalyzer.Analysis) http.Handler {
	var g *graph.Graph
	if analysis != nil {
		g = analysis.Graph()
	}
	return handler(analysis, g)
}

func graphHandler(g *graph.Graph) http.Handler {
	return handler(nil, g)
}

func handler(analysis *goanalyzer.Analysis, g *graph.Graph) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/packages", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		if g == nil {
			writeError(w, http.StatusInternalServerError, "graph is unavailable")
			return
		}
		response := packageGraph{Nodes: []packageNode{}, Edges: []packageEdge{}}
		for _, node := range g.Nodes() {
			if node.Kind != graph.NodePackage {
				continue
			}
			response.Nodes = append(response.Nodes, packageNode{ID: node.Ref, Label: node.Name})
			for _, dependency := range query.DirectPackageDependencies(g, node.Ref) {
				response.Edges = append(response.Edges, packageEdge{
					ID:   edgeID(dependency.From, dependency.To),
					From: dependency.From,
					To:   dependency.To,
				})
			}
		}
		writeJSON(w, http.StatusOK, response)
	}))
	mux.HandleFunc("/api/node", getOnly(func(w http.ResponseWriter, r *http.Request) {
		id := graph.SymbolRef(r.URL.Query().Get("id"))
		if id == "" {
			writeError(w, http.StatusBadRequest, "missing id query parameter")
			return
		}
		inspection, ok := query.InspectNode(g, id)
		if !ok {
			writeError(w, http.StatusNotFound, "node not found")
			return
		}
		var signature *query.CallableSignature
		if inspection.Function != nil && analysis != nil {
			current, err := analysis.CallableSignature(id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			signature = &current
		}
		writeJSON(w, http.StatusOK, presentNodeInspection(g, inspection, signature))
	}))
	mux.HandleFunc("/api/package-dependency", getOnly(func(w http.ResponseWriter, r *http.Request) {
		from := graph.SymbolRef(r.URL.Query().Get("from"))
		to := graph.SymbolRef(r.URL.Query().Get("to"))
		if from == "" || to == "" {
			writeError(w, http.StatusBadRequest, "missing from or to query parameter")
			return
		}
		var direct *query.PackageDependency
		for _, dependency := range query.DirectPackageDependencies(g, from) {
			if dependency.To == to {
				copy := dependency
				direct = &copy
				break
			}
		}
		if direct == nil {
			writeError(w, http.StatusNotFound, "direct package dependency not found")
			return
		}
		writeJSON(w, http.StatusOK, presentDependencyInspection(query.InspectPackageDependency(g, *direct)))
	}))
	mux.HandleFunc("/api/parameter-impact", postOnly(func(w http.ResponseWriter, r *http.Request) {
		handleParameterImpact(w, r, analysis)
	}))

	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/", getOnly(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		contents, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "interface is unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(contents)
	}))
	return mux
}

func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		next(w, r)
	}
}

func postOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		next(w, r)
	}
}

func edgeID(from, to graph.SymbolRef) string {
	return fmt.Sprintf("%d:%s>%d:%s", len(from), from, len(to), to)
}

func presentNodeInspection(g *graph.Graph, source query.NodeInspection, signature *query.CallableSignature) nodeInspection {
	result := nodeInspection{Node: presentNode(g, source.Node)}
	if source.Package != nil {
		result.Package = &packageInspection{
			Types:        presentNodes(g, source.Package.Types),
			Functions:    presentNodes(g, source.Package.Functions),
			Dependencies: presentPackageDependencies(source.Package.Dependencies),
			Dependents:   presentPackageDependencies(source.Package.Dependents),
			Imports:      presentRelationships(source.Package.Imports),
			Importers:    presentRelationships(source.Package.Importers),
		}
	}
	if source.Type != nil {
		result.Type = &typeInspection{
			Methods:            presentSymbolSummaries(source.Type.Methods),
			Dependencies:       presentTypeDependencies(source.Type.Dependencies),
			Dependents:         presentTypeDependencies(source.Type.Dependents),
			DirectDependencies: presentSymbolRelationships(source.Type.DirectDependencies),
			DirectDependents:   presentSymbolRelationships(source.Type.DirectDependents),
		}
	}
	if source.Function != nil {
		result.Function = &functionInspection{
			Calls:         presentSymbolRelationships(source.Function.Calls),
			CalledBy:      presentSymbolRelationships(source.Function.CalledBy),
			Accepts:       presentSymbolRelationships(source.Function.Accepts),
			Returns:       presentSymbolRelationships(source.Function.Returns),
			Implements:    presentSymbolRelationships(source.Function.Implements),
			ImplementedBy: presentSymbolRelationships(source.Function.ImplementedBy),
		}
		if signature != nil {
			presented := presentCallableSignature(*signature)
			result.Function.Signature = &presented
		}
	}
	return result
}

func presentDependencyInspection(source query.PackageDependencyInspection) dependencyInspection {
	return dependencyInspection{
		Dependency:       presentPackageDependency(source.Dependency),
		TypeDependencies: presentTypeDependencies(source.TypeDependencies),
		ExactOnly:        presentRelationships(source.ExactOnly),
	}
}

func presentNode(g *graph.Graph, source graph.Node) nodeInfo {
	result := nodeInfo{
		ID: source.Ref, Name: source.Name, Kind: source.Kind.String(),
		Documentation: source.Documentation,
	}
	if parent, exists := g.Node(source.Parent); exists {
		result.Parent = parent.Ref
		result.ParentName = parent.Name
		result.ParentKind = parent.Kind.String()
	}
	if source.Location.File != "" || source.Location.Offset != 0 {
		result.Location = &location{File: source.Location.File, Offset: source.Location.Offset}
	}
	return result
}

func presentNodes(g *graph.Graph, source []graph.Node) []nodeInfo {
	result := make([]nodeInfo, 0, len(source))
	for _, node := range source {
		result = append(result, presentNode(g, node))
	}
	return result
}

func presentSymbolSummaries(source []query.SymbolSummary) []symbolSummary {
	result := make([]symbolSummary, 0, len(source))
	for _, summary := range source {
		result = append(result, presentSymbolSummary(summary))
	}
	return result
}

func presentSymbolSummary(source query.SymbolSummary) symbolSummary {
	return symbolSummary{
		Ref: source.Ref, Kind: source.Kind.String(), Name: source.Name,
		ParentRef: source.ParentRef, ParentName: source.ParentName,
	}
}

func presentSymbolRelationships(source []query.SymbolRelationship) []symbolRelationship {
	result := make([]symbolRelationship, 0, len(source))
	for _, item := range source {
		evidence := make([]location, 0, len(item.Evidence))
		for _, itemLocation := range item.Evidence {
			evidence = append(evidence, location{File: itemLocation.File, Offset: itemLocation.Offset})
		}
		result = append(result, symbolRelationship{
			From: presentSymbolSummary(item.From), To: presentSymbolSummary(item.To),
			Kind: item.Kind.String(), Evidence: evidence,
		})
	}
	return result
}

func presentPackageDependencies(source []query.PackageDependency) []packageDependency {
	result := make([]packageDependency, 0, len(source))
	for _, dependency := range source {
		result = append(result, presentPackageDependency(dependency))
	}
	return result
}

func presentPackageDependency(source query.PackageDependency) packageDependency {
	return packageDependency{From: source.From, To: source.To, Evidence: presentRelationships(source.Evidence)}
}

func presentTypeDependencies(source []query.TypeDependency) []typeDependency {
	result := make([]typeDependency, 0, len(source))
	for _, dependency := range source {
		result = append(result, typeDependency{
			From: dependency.From, To: dependency.To, Evidence: presentRelationships(dependency.Evidence),
		})
	}
	return result
}

func presentRelationships(source []query.Relationship) []relationship {
	result := make([]relationship, 0, len(source))
	for _, item := range source {
		evidence := make([]location, 0, len(item.Evidence))
		for _, itemLocation := range item.Evidence {
			evidence = append(evidence, location{File: itemLocation.File, Offset: itemLocation.Offset})
		}
		result = append(result, relationship{
			From: item.From, To: item.To, Kind: item.Kind.String(), Evidence: evidence,
		})
	}
	return result
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{Error: message})
}
