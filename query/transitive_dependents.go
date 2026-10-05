package query

import (
	"sort"

	"nocv/graph"
)

// TransitiveDependent identifies one node that directly or recursively relies
// on the selected node, together with every distinct simple dependency path.
type TransitiveDependent struct {
	ID    graph.SymbolRef
	Paths []SemanticPath
}

// TransitiveDependents returns every represented node with a semantic
// dependency path to id. Paths are simple: no node is visited more than once
// within one path.
func TransitiveDependents(g *graph.Graph, id graph.SymbolRef) []TransitiveDependent {
	if g == nil {
		return nil
	}
	idNode, exists := g.Resolve(id)
	if !exists {
		return nil
	}

	traversal := transitiveDependentTraversal{
		graph:    g,
		byID:     make(map[graph.SymbolRef]*TransitiveDependent),
		pathKeys: make(map[graph.SymbolRef]map[string]bool),
		seen:     map[graph.NodeID]bool{idNode: true},
	}
	walkTransitiveDependents(&traversal, idNode, nil)

	results := make([]TransitiveDependent, 0, len(traversal.byID))
	for _, result := range traversal.byID {
		sort.Slice(result.Paths, func(i, j int) bool {
			return semanticPathLess(result.Paths[i], result.Paths[j])
		})
		results = append(results, *result)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})
	return results
}

type transitiveDependentTraversal struct {
	graph    *graph.Graph
	byID     map[graph.SymbolRef]*TransitiveDependent
	pathKeys map[graph.SymbolRef]map[string]bool
	seen     map[graph.NodeID]bool
}

func walkTransitiveDependents(
	traversal *transitiveDependentTraversal,
	current graph.NodeID,
	path []SemanticStep,
) {
	for _, edge := range traversal.graph.Incoming(current, directDependencyKinds...) {
		dependent := edge.From
		if traversal.seen[dependent] {
			continue
		}
		fromNode, fromExists := traversal.graph.Node(edge.From)
		toNode, toExists := traversal.graph.Node(edge.To)
		if !fromExists || !toExists {
			continue
		}

		nextPath := make([]SemanticStep, len(path)+1)
		nextPath[0] = SemanticStep{
			From: fromNode.Ref,
			To:   toNode.Ref,
			Kind: edge.Kind,
		}
		copy(nextPath[1:], path)

		key := semanticPathKey(nextPath)
		keys := traversal.pathKeys[fromNode.Ref]
		if keys == nil {
			keys = make(map[string]bool)
			traversal.pathKeys[fromNode.Ref] = keys
		}
		if keys[key] {
			continue
		}
		keys[key] = true

		result := traversal.byID[fromNode.Ref]
		if result == nil {
			result = &TransitiveDependent{ID: fromNode.Ref}
			traversal.byID[fromNode.Ref] = result
		}
		result.Paths = append(result.Paths, SemanticPath{Steps: nextPath})

		traversal.seen[dependent] = true
		walkTransitiveDependents(traversal, dependent, nextPath)
		delete(traversal.seen, dependent)
	}
}
