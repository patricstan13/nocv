// Package query derives architectural views from NOCV's fine-grained graph.
package query

import (
	"sort"

	"nocv/graph"
)

// CallEvidence identifies a stored call relationship that contributed to a
// projected dependency.
type CallEvidence struct {
	From      graph.SymbolID
	To        graph.SymbolID
	Locations []graph.Location
}

// Dependency is a derived relationship between two structural nodes.
// It is a query result and is never stored as a graph edge.
type Dependency struct {
	From     graph.SymbolID
	To       graph.SymbolID
	Evidence []CallEvidence
}

type dependencyKey struct {
	from graph.SymbolID
	to   graph.SymbolID
}

// Dependencies projects stored Calls edges to the nearest ancestor whose kind
// is one of levels. Pass NodeStruct and NodeInterface together for a type-level
// projection, or NodePackage for a package-level projection.
func Dependencies(g *graph.Graph, levels ...graph.NodeKind) []Dependency {
	if g == nil || !supportedLevels(levels) {
		return nil
	}

	byEndpoints := make(map[dependencyKey]*Dependency)
	for _, node := range g.Nodes() {
		if node.Kind != graph.NodeFunction {
			continue
		}
		for _, edge := range g.Outgoing(node.ID, graph.EdgeCalls) {
			from, fromOK := g.AncestorOfKind(edge.From, levels...)
			to, toOK := g.AncestorOfKind(edge.To, levels...)
			if !fromOK || !toOK || from == to {
				continue
			}

			key := dependencyKey{from: from, to: to}
			dependency := byEndpoints[key]
			if dependency == nil {
				dependency = &Dependency{From: from, To: to}
				byEndpoints[key] = dependency
			}
			dependency.Evidence = append(dependency.Evidence, CallEvidence{
				From:      edge.From,
				To:        edge.To,
				Locations: append([]graph.Location(nil), edge.Evidence...),
			})
		}
	}

	dependencies := make([]Dependency, 0, len(byEndpoints))
	for _, dependency := range byEndpoints {
		dependencies = append(dependencies, *dependency)
	}
	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i].From == dependencies[j].From {
			return dependencies[i].To < dependencies[j].To
		}
		return dependencies[i].From < dependencies[j].From
	})
	return dependencies
}

func supportedLevels(levels []graph.NodeKind) bool {
	if len(levels) == 0 {
		return false
	}
	for _, level := range levels {
		switch level {
		case graph.NodeStruct, graph.NodeInterface, graph.NodePackage:
		default:
			return false
		}
	}
	return true
}
