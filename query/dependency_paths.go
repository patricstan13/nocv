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

	traversal := dependencyPathTraversal{
		graph:    g,
		target:   toID,
		seen:     map[graph.NodeID]bool{fromID: true},
		pathKeys: make(map[string]bool),
	}
	walkDependencyPaths(&traversal, fromID, nil)

	sort.Slice(traversal.paths, func(i, j int) bool {
		return semanticPathLess(traversal.paths[i], traversal.paths[j])
	})
	return traversal.paths
}

type dependencyPathTraversal struct {
	graph    *graph.Graph
	target   graph.NodeID
	seen     map[graph.NodeID]bool
	pathKeys map[string]bool
	paths    []SemanticPath
}

func walkDependencyPaths(traversal *dependencyPathTraversal, current graph.NodeID, steps []SemanticStep) {
	for _, edge := range traversal.graph.Outgoing(current, directDependencyKinds...) {
		next := edge.To
		if traversal.seen[next] {
			continue
		}
		fromNode, fromExists := traversal.graph.Node(edge.From)
		toNode, toExists := traversal.graph.Node(edge.To)
		if !fromExists || !toExists {
			continue
		}

		nextSteps := append([]SemanticStep(nil), steps...)
		nextSteps = append(nextSteps, SemanticStep{
			From: fromNode.Ref,
			To:   toNode.Ref,
			Kind: edge.Kind,
		})
		if next == traversal.target {
			key := semanticPathKey(nextSteps)
			if !traversal.pathKeys[key] {
				traversal.pathKeys[key] = true
				traversal.paths = append(traversal.paths, SemanticPath{Steps: nextSteps})
			}
			continue
		}

		traversal.seen[next] = true
		walkDependencyPaths(traversal, next, nextSteps)
		delete(traversal.seen, next)
	}
}
