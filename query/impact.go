package query

import (
	"sort"
	"strconv"
	"strings"

	"nocv/graph"
)

// ImpactResult identifies one potentially affected node and every distinct
// simple semantic dependency path from that node to the changed node.
type ImpactResult struct {
	ID    graph.SymbolID
	Paths []ImpactPath
}

// ImpactPath is one explanation of why a node may be affected.
type ImpactPath struct {
	Steps []ImpactStep
}

// ImpactStep is one semantic relationship in ordinary dependency direction.
type ImpactStep struct {
	From graph.SymbolID
	To   graph.SymbolID
	Kind graph.EdgeKind
}

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

	var walk func(graph.SymbolID, []ImpactStep)
	walk = func(current graph.SymbolID, path []ImpactStep) {
		for _, relationship := range DirectDependents(g, current) {
			dependent := relationship.From
			if seen[dependent] {
				continue
			}

			nextPath := make([]ImpactStep, len(path)+1)
			nextPath[0] = ImpactStep{
				From: relationship.From,
				To:   relationship.To,
				Kind: relationship.Kind,
			}
			copy(nextPath[1:], path)

			key := impactPathKey(nextPath)
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
			result.Paths = append(result.Paths, ImpactPath{Steps: nextPath})

			seen[dependent] = true
			walk(dependent, nextPath)
			delete(seen, dependent)
		}
	}
	walk(id, nil)

	results := make([]ImpactResult, 0, len(byID))
	for _, result := range byID {
		sort.Slice(result.Paths, func(i, j int) bool {
			return impactPathLess(result.Paths[i], result.Paths[j])
		})
		results = append(results, *result)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})
	return results
}

func impactPathLess(left, right ImpactPath) bool {
	if len(left.Steps) != len(right.Steps) {
		return len(left.Steps) < len(right.Steps)
	}
	for index := range left.Steps {
		leftStep := left.Steps[index]
		rightStep := right.Steps[index]
		if leftStep.From != rightStep.From {
			return leftStep.From < rightStep.From
		}
		if leftStep.Kind != rightStep.Kind {
			return leftStep.Kind < rightStep.Kind
		}
		if leftStep.To != rightStep.To {
			return leftStep.To < rightStep.To
		}
	}
	return false
}

func impactPathKey(steps []ImpactStep) string {
	var key strings.Builder
	for _, step := range steps {
		writePathID(&key, step.From)
		key.WriteByte('/')
		key.WriteString(strconv.FormatUint(uint64(step.Kind), 10))
		key.WriteByte('/')
		writePathID(&key, step.To)
		key.WriteByte(';')
	}
	return key.String()
}

func writePathID(builder *strings.Builder, id graph.SymbolID) {
	value := string(id)
	builder.WriteString(strconv.Itoa(len(value)))
	builder.WriteByte(':')
	builder.WriteString(value)
}
