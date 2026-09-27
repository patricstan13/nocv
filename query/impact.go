package query

import (
	"sort"

	"nocv/graph"
)

// ImpactResult identifies one potentially affected node and every distinct
// simple semantic dependency path from that node to the changed node.
type ImpactResult struct {
	ID    graph.SymbolRef
	Paths []SemanticPath
}

// ImpactPath and ImpactStep retain the impact-specific API names while impact
// and exact dependency explanation share one neutral path representation.
type ImpactPath = SemanticPath
type ImpactStep = SemanticStep

// Impact returns all represented nodes with a semantic dependency path to id.
// Paths are simple: no node is visited more than once within one path.
func Impact(g *graph.Graph, id graph.SymbolRef) []ImpactResult {
	if g == nil {
		return nil
	}
	idNode, exists := g.Resolve(id)
	if !exists {
		return nil
	}

	byID := make(map[graph.SymbolRef]*ImpactResult)
	pathKeys := make(map[graph.SymbolRef]map[string]bool)
	seen := map[graph.NodeID]bool{idNode: true}

	var walk func(graph.NodeID, []SemanticStep)
	walk = func(current graph.NodeID, path []SemanticStep) {
		for _, edge := range g.Incoming(current, directDependencyKinds...) {
			dependent := edge.From
			if seen[dependent] {
				continue
			}
			fromNode, fromExists := g.Node(edge.From)
			toNode, toExists := g.Node(edge.To)
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
			keys := pathKeys[fromNode.Ref]
			if keys == nil {
				keys = make(map[string]bool)
				pathKeys[fromNode.Ref] = keys
			}
			if keys[key] {
				continue
			}
			keys[key] = true

			result := byID[fromNode.Ref]
			if result == nil {
				result = &ImpactResult{ID: fromNode.Ref}
				byID[fromNode.Ref] = result
			}
			result.Paths = append(result.Paths, SemanticPath{Steps: nextPath})

			seen[dependent] = true
			walk(dependent, nextPath)
			delete(seen, dependent)
		}
	}
	walk(idNode, nil)

	results := make([]ImpactResult, 0, len(byID))
	for _, result := range byID {
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
