package query

import (
	"sort"
	"strings"

	"nocv/graph"
)

// TypeDependency is one derived direct semantic dependency between represented
// structs or interfaces. Evidence contains every stored semantic fact that
// establishes the type relationship.
type TypeDependency struct {
	From     graph.SymbolID
	To       graph.SymbolID
	Evidence []Relationship
}

// TypeDependencyPath is one simple route through the derived semantic type
// view. Each step carries evidence for exactly one type boundary.
type TypeDependencyPath struct {
	Types []graph.SymbolID
	Steps []TypeDependency
}

// DirectTypeDependencies returns direct derived semantic dependencies from one
// exact represented struct or interface.
func DirectTypeDependencies(g *graph.Graph, typeID graph.SymbolID) []TypeDependency {
	if !isTypeNode(g, typeID) {
		return nil
	}
	return copyTypeDependencies(typeDependencyView(g)[typeID])
}

// DirectTypeDependents returns direct derived type dependencies whose target
// is typeID. Dependencies retain their natural From -> To orientation.
func DirectTypeDependents(g *graph.Graph, typeID graph.SymbolID) []TypeDependency {
	if !isTypeNode(g, typeID) {
		return nil
	}
	view := typeDependencyView(g)
	var dependents []TypeDependency
	for _, dependencies := range view {
		for _, dependency := range dependencies {
			if dependency.To == typeID {
				dependents = append(dependents, copyTypeDependency(dependency))
			}
		}
	}
	sort.Slice(dependents, func(i, j int) bool {
		return dependents[i].From < dependents[j].From
	})
	return dependents
}

// TypeDependencyPaths returns every distinct simple route through the derived
// semantic type view between two exact represented types.
func TypeDependencyPaths(g *graph.Graph, from, to graph.SymbolID) []TypeDependencyPath {
	if g == nil || from == to || !isTypeNode(g, from) || !isTypeNode(g, to) {
		return nil
	}

	view := typeDependencyView(g)
	var paths []TypeDependencyPath
	pathKeys := make(map[string]bool)
	seen := map[graph.SymbolID]bool{from: true}

	var walk func(graph.SymbolID, []graph.SymbolID, []TypeDependency)
	walk = func(current graph.SymbolID, types []graph.SymbolID, steps []TypeDependency) {
		for _, dependency := range view[current] {
			next := dependency.To
			if seen[next] {
				continue
			}
			nextTypes := append(append([]graph.SymbolID(nil), types...), next)
			nextSteps := appendTypeDependency(steps, dependency)
			if next == to {
				key := typeSequenceKey(nextTypes)
				if !pathKeys[key] {
					pathKeys[key] = true
					paths = append(paths, TypeDependencyPath{Types: nextTypes, Steps: nextSteps})
				}
				continue
			}

			seen[next] = true
			walk(next, nextTypes, nextSteps)
			delete(seen, next)
		}
	}
	walk(from, []graph.SymbolID{from}, nil)

	sort.Slice(paths, func(i, j int) bool {
		return typeSequenceLess(paths[i].Types, paths[j].Types)
	})
	return paths
}

func typeDependencyView(g *graph.Graph) map[graph.SymbolID][]TypeDependency {
	bySourceAndTarget := make(map[graph.SymbolID]map[graph.SymbolID]*TypeDependency)
	for _, node := range g.Nodes() {
		from, exists := typeOwner(g, node.ID)
		if !exists {
			continue
		}
		for _, relationship := range DirectDependencies(g, node.ID) {
			to, exists := typeOwner(g, relationship.To)
			if !exists || from == to {
				continue
			}
			byTarget := bySourceAndTarget[from]
			if byTarget == nil {
				byTarget = make(map[graph.SymbolID]*TypeDependency)
				bySourceAndTarget[from] = byTarget
			}
			dependency := byTarget[to]
			if dependency == nil {
				dependency = &TypeDependency{From: from, To: to}
				byTarget[to] = dependency
			}
			dependency.Evidence = append(dependency.Evidence, relationship)
		}
	}

	view := make(map[graph.SymbolID][]TypeDependency, len(bySourceAndTarget))
	for from, byTarget := range bySourceAndTarget {
		dependencies := make([]TypeDependency, 0, len(byTarget))
		for _, dependency := range byTarget {
			sort.Slice(dependency.Evidence, func(i, j int) bool {
				return semanticRelationshipLess(dependency.Evidence[i], dependency.Evidence[j])
			})
			dependencies = append(dependencies, *dependency)
		}
		sort.Slice(dependencies, func(i, j int) bool {
			return dependencies[i].To < dependencies[j].To
		})
		view[from] = dependencies
	}
	return view
}

func typeOwner(g *graph.Graph, id graph.SymbolID) (graph.SymbolID, bool) {
	if g == nil {
		return "", false
	}
	node, exists := g.Node(id)
	if !exists {
		return "", false
	}
	switch node.Kind {
	case graph.NodeStruct, graph.NodeInterface:
		return node.ID, true
	case graph.NodeFunction:
		parent, exists := g.Node(node.Parent)
		if exists && (parent.Kind == graph.NodeStruct || parent.Kind == graph.NodeInterface) {
			return parent.ID, true
		}
	}
	return "", false
}

func isTypeNode(g *graph.Graph, id graph.SymbolID) bool {
	if g == nil {
		return false
	}
	node, exists := g.Node(id)
	return exists && (node.Kind == graph.NodeStruct || node.Kind == graph.NodeInterface)
}

func appendTypeDependency(steps []TypeDependency, dependency TypeDependency) []TypeDependency {
	result := make([]TypeDependency, len(steps), len(steps)+1)
	for index, step := range steps {
		result[index] = copyTypeDependency(step)
	}
	return append(result, copyTypeDependency(dependency))
}

func copyTypeDependencies(dependencies []TypeDependency) []TypeDependency {
	if len(dependencies) == 0 {
		return nil
	}
	result := make([]TypeDependency, len(dependencies))
	for index, dependency := range dependencies {
		result[index] = copyTypeDependency(dependency)
	}
	return result
}

func copyTypeDependency(dependency TypeDependency) TypeDependency {
	copy := TypeDependency{From: dependency.From, To: dependency.To}
	copy.Evidence = make([]Relationship, len(dependency.Evidence))
	for index, relationship := range dependency.Evidence {
		copy.Evidence[index] = copyRelationship(relationship)
	}
	return copy
}

func typeSequenceLess(left, right []graph.SymbolID) bool {
	if len(left) != len(right) {
		return len(left) < len(right)
	}
	for index := range left {
		if left[index] != right[index] {
			return left[index] < right[index]
		}
	}
	return false
}

func typeSequenceKey(types []graph.SymbolID) string {
	var key strings.Builder
	for _, typeID := range types {
		writeSemanticPathID(&key, typeID)
		key.WriteByte(';')
	}
	return key.String()
}
