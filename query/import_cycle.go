package query

import (
	"sort"
	"strings"

	"nocv/graph"
)

// ImportPath is one simple path through stored package import relationships.
type ImportPath struct {
	Packages []graph.SymbolRef
}

// ImportCycleCheck describes whether a proposed direct package import would
// close one or more existing import paths.
type ImportCycleCheck struct {
	WouldCycle bool
	Paths      []ImportPath
}

// WouldCreateImportCycle reports whether adding from -> to would create a Go
// package import cycle. For distinct endpoints, Paths contains every existing
// simple import path from to back to from. A valid self-import is treated as a
// cycle with a trivial one-package path.
func WouldCreateImportCycle(g *graph.Graph, from, to graph.SymbolRef) ImportCycleCheck {
	if !isPackageNode(g, from) || !isPackageNode(g, to) {
		return ImportCycleCheck{}
	}
	if from == to {
		return ImportCycleCheck{
			WouldCycle: true,
			Paths:      []ImportPath{{Packages: []graph.SymbolRef{from}}},
		}
	}

	paths := importPaths(g, to, from)
	return ImportCycleCheck{WouldCycle: len(paths) > 0, Paths: paths}
}

func importPaths(g *graph.Graph, from, to graph.SymbolRef) []ImportPath {
	fromID, fromExists := g.Resolve(from)
	toID, toExists := g.Resolve(to)
	if !fromExists || !toExists {
		return nil
	}
	traversal := importPathTraversal{
		graph:    g,
		target:   toID,
		seen:     map[graph.NodeID]bool{fromID: true},
		pathKeys: make(map[string]bool),
	}
	walkImportPaths(&traversal, fromID, []graph.SymbolRef{from})

	sort.Slice(traversal.paths, func(i, j int) bool {
		return importPathLess(traversal.paths[i].Packages, traversal.paths[j].Packages)
	})
	return traversal.paths
}

type importPathTraversal struct {
	graph    *graph.Graph
	target   graph.NodeID
	seen     map[graph.NodeID]bool
	pathKeys map[string]bool
	paths    []ImportPath
}

func walkImportPaths(traversal *importPathTraversal, current graph.NodeID, packages []graph.SymbolRef) {
	for _, edge := range traversal.graph.Outgoing(current, graph.EdgeImports) {
		next := edge.To
		if traversal.seen[next] {
			continue
		}
		nextNode, exists := traversal.graph.Node(next)
		if !exists || nextNode.Kind != graph.NodePackage {
			continue
		}
		nextPackages := append(append([]graph.SymbolRef(nil), packages...), nextNode.Ref)
		if next == traversal.target {
			key := importPathKey(nextPackages)
			if !traversal.pathKeys[key] {
				traversal.pathKeys[key] = true
				traversal.paths = append(traversal.paths, ImportPath{Packages: nextPackages})
			}
			continue
		}

		traversal.seen[next] = true
		walkImportPaths(traversal, next, nextPackages)
		delete(traversal.seen, next)
	}
}

func importPathLess(left, right []graph.SymbolRef) bool {
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

func importPathKey(packages []graph.SymbolRef) string {
	var key strings.Builder
	for _, packageID := range packages {
		key.WriteString(string(packageID))
		key.WriteByte(0)
	}
	return key.String()
}
