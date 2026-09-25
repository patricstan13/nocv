package query

import (
	"sort"

	"nocv/graph"
)

// ImpactResult identifies one potentially affected node and every distinct
// simple semantic dependency path from that node to the changed node.
type ImpactResult struct {
	ID    graph.SymbolID
	Paths []SemanticPath
}

// ImpactPath and ImpactStep retain the impact-specific API names while impact
// and exact dependency explanation share one neutral path representation.
type ImpactPath = SemanticPath
type ImpactStep = SemanticStep

// Impact returns all represented nodes with a semantic dependency path to id.
// Paths are simple: no SymbolID is visited more than once within one path.
func Impact(g *graph.Graph, id graph.SymbolID) []ImpactResult {
	if g == nil {
		return nil
	}
	if _, exists := g.Node(id); !exists {
		return nil
	}

	byID := make(map[graph.SymbolID]*ImpactResult)
	pathKeys := make(map[graph.SymbolID]map[string]bool)
	seen := map[graph.SymbolID]bool{id: true}

	var walk func(graph.SymbolID, []SemanticStep)
	walk = func(current graph.SymbolID, path []SemanticStep) {
		for _, relationship := range DirectDependents(g, current) {
			dependent := relationship.From
			if seen[dependent] {
				continue
			}

			nextPath := make([]SemanticStep, len(path)+1)
			nextPath[0] = SemanticStep{
				From: relationship.From,
				To:   relationship.To,
				Kind: relationship.Kind,
			}
			copy(nextPath[1:], path)

			key := semanticPathKey(nextPath)
			keys := pathKeys[dependent]
			if keys == nil {
				keys = make(map[string]bool)
				pathKeys[dependent] = keys
			}
			if keys[key] {
				continue
			}
			keys[key] = true

			result := byID[dependent]
			if result == nil {
				result = &ImpactResult{ID: dependent}
				byID[dependent] = result
			}
			result.Paths = append(result.Paths, SemanticPath{Steps: nextPath})

			seen[dependent] = true
			walk(dependent, nextPath)
			delete(seen, dependent)
		}
	}
	walk(id, nil)

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
