package query

import (
	"sort"

	"nocv/graph"
)

// DependencyPaths returns every distinct simple semantic dependency path from
// one exact represented symbol to another. It does not project endpoints.
func DependencyPaths(g *graph.Graph, from, to graph.SymbolRef) []SemanticPath {
	if g == nil || from == to {
		return nil
	}
	fromID, exists := g.Resolve(from)
	if !exists {
		return nil
	}
	toID, exists := g.Resolve(to)
	if !exists {
		return nil
	}

	var paths []SemanticPath
	pathKeys := make(map[string]bool)
	seen := map[graph.NodeID]bool{fromID: true}

	var walk func(graph.NodeID, []SemanticStep)
	walk = func(current graph.NodeID, steps []SemanticStep) {
		for _, edge := range g.Outgoing(current, directDependencyKinds...) {
			next := edge.To
			if seen[next] {
				continue
			}
			fromNode, fromExists := g.Node(edge.From)
			toNode, toExists := g.Node(edge.To)
			if !fromExists || !toExists {
				continue
			}

			nextSteps := append([]SemanticStep(nil), steps...)
			nextSteps = append(nextSteps, SemanticStep{
				From: fromNode.Ref,
				To:   toNode.Ref,
				Kind: edge.Kind,
			})
			if next == toID {
				key := semanticPathKey(nextSteps)
				if !pathKeys[key] {
					pathKeys[key] = true
					paths = append(paths, SemanticPath{Steps: nextSteps})
				}
				continue
			}

			seen[next] = true
			walk(next, nextSteps)
			delete(seen, next)
		}
	}
	walk(fromID, nil)

	sort.Slice(paths, func(i, j int) bool {
		return semanticPathLess(paths[i], paths[j])
	})
	return paths
}
