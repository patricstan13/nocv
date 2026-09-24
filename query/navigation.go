package query

import (
	"sort"

	"nocv/graph"
)

// Relationship is one stored semantic fact presented as direct dependency
// navigation. Evidence is copied from the graph edge that established it.
type Relationship struct {
	From     graph.SymbolID
	To       graph.SymbolID
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
func DirectDependencies(g *graph.Graph, id graph.SymbolID) []Relationship {
	if g == nil {
		return nil
	}
	if _, exists := g.Node(id); !exists {
		return nil
	}

	relationships := copyRelationships(g.Outgoing(id, directDependencyKinds...))
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
func DirectDependents(g *graph.Graph, id graph.SymbolID) []Relationship {
	if g == nil {
		return nil
	}
	if _, exists := g.Node(id); !exists {
		return nil
	}

	relationships := copyRelationships(g.Incoming(id, directDependencyKinds...))
	sort.Slice(relationships, func(i, j int) bool {
		if relationships[i].Kind != relationships[j].Kind {
			return relationships[i].Kind < relationships[j].Kind
		}
		return relationships[i].From < relationships[j].From
	})
	return relationships
}

func copyRelationships(edges []*graph.Edge) []Relationship {
	if len(edges) == 0 {
		return nil
	}
	relationships := make([]Relationship, 0, len(edges))
	for _, edge := range edges {
		relationships = append(relationships, Relationship{
			From:     edge.From,
			To:       edge.To,
			Kind:     edge.Kind,
			Evidence: append([]graph.Location(nil), edge.Evidence...),
		})
	}
	return relationships
}
