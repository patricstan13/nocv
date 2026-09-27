package query

import (
	"sort"

	"nocv/graph"
)

// Relationship is one stored graph fact presented as direct navigation.
// Evidence is copied from the graph edge that established it.
type Relationship struct {
	From     graph.SymbolRef
	To       graph.SymbolRef
	Kind     graph.EdgeKind
	Evidence []graph.Location
}

var directDependencyKinds = []graph.EdgeKind{
	graph.EdgeCalls,
	graph.EdgeImplements,
	graph.EdgeEmbeds,
	graph.EdgeAccepts,
	graph.EdgeReturns,
}

// DirectDependencies returns supported semantic relationships leaving id.
// Structural containment and projected dependencies are not included.
func DirectDependencies(g *graph.Graph, id graph.SymbolRef) []Relationship {
	if g == nil {
		return nil
	}
	nodeID, exists := g.Resolve(id)
	if !exists {
		return nil
	}

	relationships := copyRelationships(g, g.Outgoing(nodeID, directDependencyKinds...))
	sort.Slice(relationships, func(i, j int) bool {
		if relationships[i].Kind != relationships[j].Kind {
			return relationships[i].Kind < relationships[j].Kind
		}
		return relationships[i].To < relationships[j].To
	})
	return relationships
}

// DirectDependents returns supported semantic relationships entering id.
// Structural containment and projected dependencies are not included.
func DirectDependents(g *graph.Graph, id graph.SymbolRef) []Relationship {
	if g == nil {
		return nil
	}
	nodeID, exists := g.Resolve(id)
	if !exists {
		return nil
	}

	relationships := copyRelationships(g, g.Incoming(nodeID, directDependencyKinds...))
	sort.Slice(relationships, func(i, j int) bool {
		if relationships[i].Kind != relationships[j].Kind {
			return relationships[i].Kind < relationships[j].Kind
		}
		return relationships[i].From < relationships[j].From
	})
	return relationships
}

func copyRelationships(g *graph.Graph, edges []*graph.Edge) []Relationship {
	if len(edges) == 0 {
		return nil
	}
	relationships := make([]Relationship, 0, len(edges))
	for _, edge := range edges {
		from, fromExists := g.Node(edge.From)
		to, toExists := g.Node(edge.To)
		if !fromExists || !toExists {
			continue
		}
		relationships = append(relationships, Relationship{
			From:     from.Ref,
			To:       to.Ref,
			Kind:     edge.Kind,
			Evidence: append([]graph.Location(nil), edge.Evidence...),
		})
	}
	return relationships
}
