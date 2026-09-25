package query

import (
	"sort"

	"nocv/graph"
)

// DependencyPaths returns every distinct simple semantic dependency path from
// one exact represented symbol to another. It does not project endpoints.
func DependencyPaths(g *graph.Graph, from, to graph.SymbolID) []SemanticPath {
	if g == nil || from == to {
		return nil
	}
	if _, exists := g.Node(from); !exists {
		return nil
	}
	if _, exists := g.Node(to); !exists {
		return nil
	}

	var paths []SemanticPath
	pathKeys := make(map[string]bool)
	seen := map[graph.SymbolID]bool{from: true}

	var walk func(graph.SymbolID, []SemanticStep)
	walk = func(current graph.SymbolID, steps []SemanticStep) {
		for _, relationship := range DirectDependencies(g, current) {
			next := relationship.To
			if seen[next] {
				continue
			}

			nextSteps := append([]SemanticStep(nil), steps...)
			nextSteps = append(nextSteps, SemanticStep{
				From: relationship.From,
				To:   relationship.To,
				Kind: relationship.Kind,
			})
			if next == to {
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
	walk(from, nil)

	sort.Slice(paths, func(i, j int) bool {
		return semanticPathLess(paths[i], paths[j])
	})
	return paths
}
