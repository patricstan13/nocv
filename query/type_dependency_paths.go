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
	From      graph.SymbolRef
	To        graph.SymbolRef
	Certainty graph.RelationshipCertainty
	Evidence  []Relationship
}

// TypeDependencyPath is one simple route through the derived semantic type
// view. Each step carries evidence for exactly one type boundary.
type TypeDependencyPath struct {
	Types []graph.SymbolRef
	Steps []TypeDependency
}

type typeDependency struct {
	to         graph.NodeID
	dependency TypeDependency
}

// DirectTypeDependencies returns direct derived semantic dependencies from one
// exact represented struct or interface.
func DirectTypeDependencies(g *graph.Graph, typeID graph.SymbolRef) []TypeDependency {
	if g == nil {
		return nil
	}
	id, exists := g.Resolve(typeID)
	if !exists || !isTypeNode(g, typeID) {
		return nil
	}
	return publicTypeDependencies(typeDependencyView(g)[id])
}

// DirectTypeDependents returns direct derived type dependencies whose target
// is typeID. Dependencies retain their natural From -> To orientation.
func DirectTypeDependents(g *graph.Graph, typeID graph.SymbolRef) []TypeDependency {
	if !isTypeNode(g, typeID) {
		return nil
	}
	id, _ := g.Resolve(typeID)
	view := typeDependencyView(g)
	var dependents []TypeDependency
	for _, dependencies := range view {
		for _, dependency := range dependencies {
			if dependency.to == id {
				dependents = append(dependents, copyTypeDependency(dependency.dependency))
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
func TypeDependencyPaths(g *graph.Graph, from, to graph.SymbolRef) []TypeDependencyPath {
	if g == nil || from == to || !isTypeNode(g, from) || !isTypeNode(g, to) {
		return nil
	}

	view := typeDependencyView(g)
	fromID, _ := g.Resolve(from)
	toID, _ := g.Resolve(to)
	traversal := typeDependencyPathTraversal{
		graph:    g,
		view:     view,
		target:   toID,
		seen:     map[graph.NodeID]bool{fromID: true},
		pathKeys: make(map[string]bool),
	}
	walkTypeDependencyPaths(&traversal, fromID, []graph.SymbolRef{from}, nil)

	sort.Slice(traversal.paths, func(i, j int) bool {
		return typeSequenceLess(traversal.paths[i].Types, traversal.paths[j].Types)
	})
	return traversal.paths
}

type typeDependencyPathTraversal struct {
	graph    *graph.Graph
	view     map[graph.NodeID][]typeDependency
	target   graph.NodeID
	seen     map[graph.NodeID]bool
	pathKeys map[string]bool
	paths    []TypeDependencyPath
}

func walkTypeDependencyPaths(
	traversal *typeDependencyPathTraversal,
	current graph.NodeID,
	types []graph.SymbolRef,
	steps []TypeDependency,
) {
	for _, dependency := range traversal.view[current] {
		next := dependency.to
		if traversal.seen[next] {
			continue
		}
		nextNode, exists := traversal.graph.Node(next)
		if !exists {
			continue
		}
		nextTypes := append(append([]graph.SymbolRef(nil), types...), nextNode.Ref)
		nextSteps := appendTypeDependency(steps, dependency.dependency)
		if next == traversal.target {
			key := typeSequenceKey(nextTypes)
			if !traversal.pathKeys[key] {
				traversal.pathKeys[key] = true
				traversal.paths = append(traversal.paths, TypeDependencyPath{Types: nextTypes, Steps: nextSteps})
			}
			continue
		}

		traversal.seen[next] = true
		walkTypeDependencyPaths(traversal, next, nextTypes, nextSteps)
		delete(traversal.seen, next)
	}
}

func typeDependencyView(g *graph.Graph) map[graph.NodeID][]typeDependency {
	bySourceAndTarget := make(map[graph.NodeID]map[graph.NodeID]*TypeDependency)
	for _, node := range g.Nodes() {
		from, exists := typeOwner(g, node.ID)
		if !exists {
			continue
		}
		for _, edge := range g.Outgoing(node.ID, directDependencyKinds...) {
			to, exists := typeOwner(g, edge.To)
			if !exists || from == to {
				continue
			}
			fromExact, fromExists := g.Node(edge.From)
			toExact, toExists := g.Node(edge.To)
			if !fromExists || !toExists {
				continue
			}
			relationship := Relationship{
				From: fromExact.Ref, To: toExact.Ref, Kind: edge.Kind, Certainty: edge.Certainty,
				Evidence: append([]graph.Location(nil), edge.Evidence...),
			}
			fromType, fromExists := g.Node(from)
			toType, toExists := g.Node(to)
			if !fromExists || !toExists {
				continue
			}
			byTarget := bySourceAndTarget[from]
			if byTarget == nil {
				byTarget = make(map[graph.NodeID]*TypeDependency)
				bySourceAndTarget[from] = byTarget
			}
			dependency := byTarget[to]
			if dependency == nil {
				dependency = &TypeDependency{
					From: fromType.Ref, To: toType.Ref, Certainty: relationship.Certainty,
				}
				byTarget[to] = dependency
			} else {
				dependency.Certainty = mergeRelationshipCertainty(dependency.Certainty, relationship.Certainty)
			}
			dependency.Evidence = append(dependency.Evidence, relationship)
		}
	}

	view := make(map[graph.NodeID][]typeDependency, len(bySourceAndTarget))
	for from, byTarget := range bySourceAndTarget {
		dependencies := make([]typeDependency, 0, len(byTarget))
		for to, dependency := range byTarget {
			sort.Slice(dependency.Evidence, func(i, j int) bool {
				return semanticRelationshipLess(dependency.Evidence[i], dependency.Evidence[j])
			})
			dependencies = append(dependencies, typeDependency{to: to, dependency: *dependency})
		}
		sort.Slice(dependencies, func(i, j int) bool {
			return dependencies[i].dependency.To < dependencies[j].dependency.To
		})
		view[from] = dependencies
	}
	return view
}

func typeOwner(g *graph.Graph, id graph.NodeID) (graph.NodeID, bool) {
	if g == nil {
		return 0, false
	}
	node, exists := g.Node(id)
	if !exists {
		return 0, false
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
	return 0, false
}

func publicTypeDependencies(dependencies []typeDependency) []TypeDependency {
	result := make([]TypeDependency, len(dependencies))
	for index, dependency := range dependencies {
		result[index] = copyTypeDependency(dependency.dependency)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func isTypeNode(g *graph.Graph, id graph.SymbolRef) bool {
	if g == nil {
		return false
	}
	node, exists := g.NodeByRef(id)
	return exists && (node.Kind == graph.NodeStruct || node.Kind == graph.NodeInterface)
}

func appendTypeDependency(steps []TypeDependency, dependency TypeDependency) []TypeDependency {
	result := make([]TypeDependency, len(steps), len(steps)+1)
	for index, step := range steps {
		result[index] = copyTypeDependency(step)
	}
	return append(result, copyTypeDependency(dependency))
}

func copyTypeDependency(dependency TypeDependency) TypeDependency {
	copy := TypeDependency{From: dependency.From, To: dependency.To, Certainty: dependency.Certainty}
	copy.Evidence = make([]Relationship, len(dependency.Evidence))
	for index, relationship := range dependency.Evidence {
		copy.Evidence[index] = copyRelationship(relationship)
	}
	return copy
}

func typeSequenceLess(left, right []graph.SymbolRef) bool {
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

func typeSequenceKey(types []graph.SymbolRef) string {
	var key strings.Builder
	for _, typeID := range types {
		writeSemanticPathID(&key, typeID)
		key.WriteByte(';')
	}
	return key.String()
}
