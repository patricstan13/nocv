package query

import (
	"sort"

	"nocv/graph"
)

// NodeInspection is a detached consumer view of one represented node. Exactly
// one kind-specific detail field is populated.
type NodeInspection struct {
	Node     graph.Node
	Package  *PackageInspection
	Type     *TypeInspection
	Function *FunctionInspection
}

// PackageInspection describes direct package contents and relationships.
// Imports remain separate from derived semantic package dependencies.
type PackageInspection struct {
	Types        []graph.Node
	Functions    []graph.Node
	Dependencies []PackageDependency
	Dependents   []PackageDependency
	Imports      []Relationship
	Importers    []Relationship
}

// TypeInspection describes direct methods, projected type relationships, and
// exact semantic relationships attached directly to the type declaration.
type TypeInspection struct {
	Methods            []graph.Node
	Dependencies       []TypeDependency
	Dependents         []TypeDependency
	DirectDependencies []Relationship
	DirectDependents   []Relationship
}

// FunctionInspection describes exact direct semantic relationships involving
// one package function, struct method, or interface method.
type FunctionInspection struct {
	Dependencies []Relationship
	Dependents   []Relationship
}

// InspectNode returns a detached, kind-specific read model for one represented
// graph node.
func InspectNode(g *graph.Graph, id graph.SymbolID) (NodeInspection, bool) {
	if g == nil {
		return NodeInspection{}, false
	}
	node, exists := g.Node(id)
	if !exists {
		return NodeInspection{}, false
	}
	inspection := NodeInspection{Node: *node}

	switch node.Kind {
	case graph.NodePackage:
		detail := &PackageInspection{
			Dependencies: DirectPackageDependencies(g, id),
			Dependents:   DirectPackageDependents(g, id),
			Imports:      DirectImports(g, id),
			Importers:    DirectImporters(g, id),
		}
		for _, childID := range g.Children(id) {
			child, childExists := g.Node(childID)
			if !childExists {
				continue
			}
			switch child.Kind {
			case graph.NodeStruct, graph.NodeInterface:
				detail.Types = append(detail.Types, *child)
			case graph.NodeFunction:
				detail.Functions = append(detail.Functions, *child)
			}
		}
		sortNodes(detail.Types)
		sortNodes(detail.Functions)
		inspection.Package = detail

	case graph.NodeStruct, graph.NodeInterface:
		detail := &TypeInspection{
			Dependencies:       DirectTypeDependencies(g, id),
			Dependents:         DirectTypeDependents(g, id),
			DirectDependencies: DirectDependencies(g, id),
			DirectDependents:   DirectDependents(g, id),
		}
		for _, childID := range g.Children(id) {
			child, childExists := g.Node(childID)
			if childExists && child.Kind == graph.NodeFunction {
				detail.Methods = append(detail.Methods, *child)
			}
		}
		sortNodes(detail.Methods)
		inspection.Type = detail

	case graph.NodeFunction:
		inspection.Function = &FunctionInspection{
			Dependencies: DirectDependencies(g, id),
			Dependents:   DirectDependents(g, id),
		}

	default:
		return NodeInspection{}, false
	}
	return inspection, true
}

func sortNodes(nodes []graph.Node) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Name != nodes[j].Name {
			return nodes[i].Name < nodes[j].Name
		}
		return nodes[i].ID < nodes[j].ID
	})
}
