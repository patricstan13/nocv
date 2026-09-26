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
	From     graph.SymbolID
	To       graph.SymbolID
	Evidence []Relationship
}

// PackageDependencyPath is one simple route through the derived semantic
// package view. Each step carries evidence for exactly one package boundary.
type PackageDependencyPath struct {
	Packages []graph.SymbolID
	Steps    []PackageDependency
}

// DirectPackageDependencies returns direct derived semantic package
// dependencies originating from one exact package.
func DirectPackageDependencies(g *graph.Graph, packageID graph.SymbolID) []PackageDependency {
	if !isPackageNode(g, packageID) {
		return nil
	}
	return copyPackageDependencies(packageDependencyView(g)[packageID])
}

// DirectPackageDependents returns direct derived semantic package dependencies
// whose target is packageID. Dependencies retain their natural From -> To
// orientation.
func DirectPackageDependents(g *graph.Graph, packageID graph.SymbolID) []PackageDependency {
	if !isPackageNode(g, packageID) {
		return nil
	}
	view := packageDependencyView(g)
	var dependents []PackageDependency
	for _, dependencies := range view {
		for _, dependency := range dependencies {
			if dependency.To == packageID {
				dependents = append(dependents, copyPackageDependency(dependency))
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
func PackageDependencyPaths(g *graph.Graph, from, to graph.SymbolID) []PackageDependencyPath {
	if g == nil || from == to || !isPackageNode(g, from) || !isPackageNode(g, to) {
		return nil
	}

	view := packageDependencyView(g)
	var paths []PackageDependencyPath
	pathKeys := make(map[string]bool)
	seen := map[graph.SymbolID]bool{from: true}

	var walk func(graph.SymbolID, []graph.SymbolID, []PackageDependency)
	walk = func(current graph.SymbolID, packages []graph.SymbolID, steps []PackageDependency) {
		for _, dependency := range view[current] {
			next := dependency.To
			if seen[next] {
				continue
			}
			nextPackages := append(append([]graph.SymbolID(nil), packages...), next)
			nextSteps := appendPackageDependency(steps, dependency)
			if next == to {
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
	walk(from, []graph.SymbolID{from}, nil)

	sort.Slice(paths, func(i, j int) bool {
		return packageSequenceLess(paths[i].Packages, paths[j].Packages)
	})
	return paths
}

func packageDependencyView(g *graph.Graph) map[graph.SymbolID][]PackageDependency {
	bySourceAndTarget := make(map[graph.SymbolID]map[graph.SymbolID]*PackageDependency)
	for _, node := range g.Nodes() {
		if node.Kind == graph.NodePackage {
			continue
		}
		from, exists := g.AncestorOfKind(node.ID, graph.NodePackage)
		if !exists {
			continue
		}
		for _, relationship := range DirectDependencies(g, node.ID) {
			to, exists := g.AncestorOfKind(relationship.To, graph.NodePackage)
			if !exists || from == to {
				continue
			}
			byTarget := bySourceAndTarget[from]
			if byTarget == nil {
				byTarget = make(map[graph.SymbolID]*PackageDependency)
				bySourceAndTarget[from] = byTarget
			}
			dependency := byTarget[to]
			if dependency == nil {
				dependency = &PackageDependency{From: from, To: to}
				byTarget[to] = dependency
			}
			dependency.Evidence = append(dependency.Evidence, relationship)
		}
	}

	view := make(map[graph.SymbolID][]PackageDependency, len(bySourceAndTarget))
	for from, byTarget := range bySourceAndTarget {
		dependencies := make([]PackageDependency, 0, len(byTarget))
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

func copyPackageDependencies(dependencies []PackageDependency) []PackageDependency {
	if len(dependencies) == 0 {
		return nil
	}
	result := make([]PackageDependency, len(dependencies))
	for index, dependency := range dependencies {
		result[index] = copyPackageDependency(dependency)
	}
	return result
}

func copyPackageDependency(dependency PackageDependency) PackageDependency {
	copy := PackageDependency{From: dependency.From, To: dependency.To}
	copy.Evidence = make([]Relationship, len(dependency.Evidence))
	for index, relationship := range dependency.Evidence {
		copy.Evidence[index] = copyRelationship(relationship)
	}
	return copy
}

func isPackageNode(g *graph.Graph, id graph.SymbolID) bool {
	if g == nil {
		return false
	}
	node, exists := g.Node(id)
	return exists && node.Kind == graph.NodePackage
}

func packageSequenceLess(left, right []graph.SymbolID) bool {
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

func packageSequenceKey(packages []graph.SymbolID) string {
	var key strings.Builder
	for _, packageID := range packages {
		writeSemanticPathID(&key, packageID)
		key.WriteByte(';')
	}
	return key.String()
}
