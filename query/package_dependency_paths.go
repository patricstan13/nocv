package query

import (
	"sort"
	"strings"

	"nocv/graph"
)

// PackageDependency is one derived direct semantic dependency between two
// packages. Evidence contains every stored cross-package semantic fact that
// establishes the package relationship.
type PackageDependency struct {
	From     graph.SymbolRef
	To       graph.SymbolRef
	Evidence []Relationship
}

// PackageDependencyPath is one simple route through the derived semantic
// package view. Each step carries evidence for exactly one package boundary.
type PackageDependencyPath struct {
	Packages []graph.SymbolRef
	Steps    []PackageDependency
}

type packageDependency struct {
	to         graph.NodeID
	dependency PackageDependency
}

// DirectPackageDependencies returns direct derived semantic package
// dependencies originating from one exact package.
func DirectPackageDependencies(g *graph.Graph, packageID graph.SymbolRef) []PackageDependency {
	if g == nil {
		return nil
	}
	id, exists := g.Resolve(packageID)
	if !exists || !isPackageNode(g, packageID) {
		return nil
	}
	return publicPackageDependencies(packageDependencyView(g)[id])
}

// DirectPackageDependents returns direct derived semantic package dependencies
// whose target is packageID. Dependencies retain their natural From -> To
// orientation.
func DirectPackageDependents(g *graph.Graph, packageID graph.SymbolRef) []PackageDependency {
	if !isPackageNode(g, packageID) {
		return nil
	}
	id, _ := g.Resolve(packageID)
	view := packageDependencyView(g)
	var dependents []PackageDependency
	for _, dependencies := range view {
		for _, dependency := range dependencies {
			if dependency.to == id {
				dependents = append(dependents, copyPackageDependency(dependency.dependency))
			}
		}
	}
	sort.Slice(dependents, func(i, j int) bool {
		return dependents[i].From < dependents[j].From
	})
	return dependents
}

// PackageDependencyPaths returns every distinct simple route through the
// derived semantic package view from one exact package to another.
func PackageDependencyPaths(g *graph.Graph, from, to graph.SymbolRef) []PackageDependencyPath {
	if g == nil || from == to || !isPackageNode(g, from) || !isPackageNode(g, to) {
		return nil
	}

	view := packageDependencyView(g)
	fromID, _ := g.Resolve(from)
	toID, _ := g.Resolve(to)
	var paths []PackageDependencyPath
	pathKeys := make(map[string]bool)
	seen := map[graph.NodeID]bool{fromID: true}

	var walk func(graph.NodeID, []graph.SymbolRef, []PackageDependency)
	walk = func(current graph.NodeID, packages []graph.SymbolRef, steps []PackageDependency) {
		for _, dependency := range view[current] {
			next := dependency.to
			if seen[next] {
				continue
			}
			nextNode, exists := g.Node(next)
			if !exists {
				continue
			}
			nextPackages := append(append([]graph.SymbolRef(nil), packages...), nextNode.Ref)
			nextSteps := appendPackageDependency(steps, dependency.dependency)
			if next == toID {
				key := packageSequenceKey(nextPackages)
				if !pathKeys[key] {
					pathKeys[key] = true
					paths = append(paths, PackageDependencyPath{Packages: nextPackages, Steps: nextSteps})
				}
				continue
			}

			seen[next] = true
			walk(next, nextPackages, nextSteps)
			delete(seen, next)
		}
	}
	walk(fromID, []graph.SymbolRef{from}, nil)

	sort.Slice(paths, func(i, j int) bool {
		return packageSequenceLess(paths[i].Packages, paths[j].Packages)
	})
	return paths
}

func packageDependencyView(g *graph.Graph) map[graph.NodeID][]packageDependency {
	bySourceAndTarget := make(map[graph.NodeID]map[graph.NodeID]*PackageDependency)
	for _, node := range g.Nodes() {
		if node.Kind == graph.NodePackage {
			continue
		}
		fromID, exists := g.AncestorOfKind(node.ID, graph.NodePackage)
		if !exists {
			continue
		}
		for _, edge := range g.Outgoing(node.ID, directDependencyKinds...) {
			toID, exists := g.AncestorOfKind(edge.To, graph.NodePackage)
			if !exists || fromID == toID {
				continue
			}
			fromExact, fromExactExists := g.Node(edge.From)
			toExact, toExactExists := g.Node(edge.To)
			if !fromExactExists || !toExactExists {
				continue
			}
			relationship := Relationship{From: fromExact.Ref, To: toExact.Ref, Kind: edge.Kind, Evidence: append([]graph.Location(nil), edge.Evidence...)}
			fromPackage, fromExists := g.Node(fromID)
			toPackage, toExists := g.Node(toID)
			if !fromExists || !toExists {
				continue
			}
			byTarget := bySourceAndTarget[fromID]
			if byTarget == nil {
				byTarget = make(map[graph.NodeID]*PackageDependency)
				bySourceAndTarget[fromID] = byTarget
			}
			dependency := byTarget[toID]
			if dependency == nil {
				dependency = &PackageDependency{From: fromPackage.Ref, To: toPackage.Ref}
				byTarget[toID] = dependency
			}
			dependency.Evidence = append(dependency.Evidence, relationship)
		}
	}

	view := make(map[graph.NodeID][]packageDependency, len(bySourceAndTarget))
	for from, byTarget := range bySourceAndTarget {
		dependencies := make([]packageDependency, 0, len(byTarget))
		for to, dependency := range byTarget {
			sort.Slice(dependency.Evidence, func(i, j int) bool {
				return semanticRelationshipLess(dependency.Evidence[i], dependency.Evidence[j])
			})
			dependencies = append(dependencies, packageDependency{to: to, dependency: *dependency})
		}
		sort.Slice(dependencies, func(i, j int) bool {
			return dependencies[i].dependency.To < dependencies[j].dependency.To
		})
		view[from] = dependencies
	}
	return view
}

func publicPackageDependencies(dependencies []packageDependency) []PackageDependency {
	result := make([]PackageDependency, len(dependencies))
	for index, dependency := range dependencies {
		result[index] = copyPackageDependency(dependency.dependency)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func semanticRelationshipLess(left, right Relationship) bool {
	if left.From != right.From {
		return left.From < right.From
	}
	if left.Kind != right.Kind {
		return left.Kind < right.Kind
	}
	return left.To < right.To
}

func appendPackageDependency(steps []PackageDependency, dependency PackageDependency) []PackageDependency {
	result := make([]PackageDependency, len(steps), len(steps)+1)
	for index, step := range steps {
		result[index] = copyPackageDependency(step)
	}
	return append(result, copyPackageDependency(dependency))
}

func copyPackageDependency(dependency PackageDependency) PackageDependency {
	copy := PackageDependency{From: dependency.From, To: dependency.To}
	copy.Evidence = make([]Relationship, len(dependency.Evidence))
	for index, relationship := range dependency.Evidence {
		copy.Evidence[index] = copyRelationship(relationship)
	}
	return copy
}

func isPackageNode(g *graph.Graph, id graph.SymbolRef) bool {
	if g == nil {
		return false
	}
	node, exists := g.NodeByRef(id)
	return exists && node.Kind == graph.NodePackage
}

func packageSequenceLess(left, right []graph.SymbolRef) bool {
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

func packageSequenceKey(packages []graph.SymbolRef) string {
	var key strings.Builder
	for _, packageID := range packages {
		writeSemanticPathID(&key, packageID)
		key.WriteByte(';')
	}
	return key.String()
}
