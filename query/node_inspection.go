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

// SymbolSummary identifies a graph symbol and its structural parent without
// exposing graph-internal node identity.
type SymbolSummary struct {
	Ref        graph.SymbolRef
	Kind       graph.NodeKind
	Name       string
	ParentRef  graph.SymbolRef
	ParentName string
}

// SymbolRelationship is an exact semantic relationship with both endpoints
// resolved for human-oriented inspection. Evidence remains supporting detail
// for the generic semantic fact stored by the graph.
type SymbolRelationship struct {
	From      SymbolSummary
	To        SymbolSummary
	Kind      graph.EdgeKind
	Certainty graph.RelationshipCertainty
	Evidence  []graph.Location
}

// TypeInspection describes direct methods, projected type relationships, and
// exact semantic relationships attached directly to the type declaration.
type TypeInspection struct {
	Methods            []SymbolSummary
	Dependencies       []TypeDependency
	Dependents         []TypeDependency
	DirectDependencies []SymbolRelationship
	DirectDependents   []SymbolRelationship
}

// FunctionInspection organizes the exact semantic relationships involving one
// package function, struct method, or interface method. Calls, Accepts,
// Returns, and Implements are outgoing; CalledBy and ImplementedBy are
// incoming.
type FunctionInspection struct {
	Calls         []SymbolRelationship
	CalledBy      []SymbolRelationship
	Accepts       []SymbolRelationship
	Returns       []SymbolRelationship
	Implements    []SymbolRelationship
	ImplementedBy []SymbolRelationship
}

// InspectNode returns a detached, kind-specific read model for one represented
// graph node.
func InspectNode(g *graph.Graph, id graph.SymbolRef) (NodeInspection, bool) {
	if g == nil {
		return NodeInspection{}, false
	}
	node, exists := g.NodeByRef(id)
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
		for _, childID := range g.Children(node.ID) {
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
			DirectDependencies: symbolRelationships(g, DirectDependencies(g, id)),
			DirectDependents:   symbolRelationships(g, DirectDependents(g, id)),
		}
		for _, childID := range g.Children(node.ID) {
			child, childExists := g.Node(childID)
			if childExists && child.Kind == graph.NodeFunction {
				detail.Methods = append(detail.Methods, symbolSummary(g, *child))
			}
		}
		sortSymbolSummaries(detail.Methods)
		inspection.Type = detail

	case graph.NodeFunction:
		detail := &FunctionInspection{}
		for _, relationship := range symbolRelationships(g, DirectDependencies(g, id)) {
			switch relationship.Kind {
			case graph.EdgeCalls:
				detail.Calls = append(detail.Calls, relationship)
			case graph.EdgeAccepts:
				detail.Accepts = append(detail.Accepts, relationship)
			case graph.EdgeReturns:
				detail.Returns = append(detail.Returns, relationship)
			case graph.EdgeImplements:
				detail.Implements = append(detail.Implements, relationship)
			}
		}
		for _, relationship := range symbolRelationships(g, DirectDependents(g, id)) {
			switch relationship.Kind {
			case graph.EdgeCalls:
				detail.CalledBy = append(detail.CalledBy, relationship)
			case graph.EdgeImplements:
				detail.ImplementedBy = append(detail.ImplementedBy, relationship)
			}
		}
		inspection.Function = detail

	default:
		return NodeInspection{}, false
	}
	return inspection, true
}

func symbolSummary(g *graph.Graph, node graph.Node) SymbolSummary {
	result := SymbolSummary{Ref: node.Ref, Kind: node.Kind, Name: node.Name}
	if parent, exists := g.Node(node.Parent); exists {
		result.ParentRef = parent.Ref
		result.ParentName = parent.Name
	}
	return result
}

func symbolRelationships(g *graph.Graph, relationships []Relationship) []SymbolRelationship {
	result := make([]SymbolRelationship, 0, len(relationships))
	for _, relationship := range relationships {
		from, fromExists := g.NodeByRef(relationship.From)
		to, toExists := g.NodeByRef(relationship.To)
		if !fromExists || !toExists {
			continue
		}
		result = append(result, SymbolRelationship{
			From:      symbolSummary(g, *from),
			To:        symbolSummary(g, *to),
			Kind:      relationship.Kind,
			Certainty: relationship.Certainty,
			Evidence:  append([]graph.Location(nil), relationship.Evidence...),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].From.Ref != result[j].From.Ref {
			return result[i].From.Ref < result[j].From.Ref
		}
		if result[i].To.Ref != result[j].To.Ref {
			return result[i].To.Ref < result[j].To.Ref
		}
		return result[i].Kind < result[j].Kind
	})
	return result
}

func sortSymbolSummaries(summaries []SymbolSummary) {
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Name != summaries[j].Name {
			return summaries[i].Name < summaries[j].Name
		}
		return summaries[i].Ref < summaries[j].Ref
	})
}

func sortNodes(nodes []graph.Node) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Name != nodes[j].Name {
			return nodes[i].Name < nodes[j].Name
		}
		return nodes[i].Ref < nodes[j].Ref
	})
}
