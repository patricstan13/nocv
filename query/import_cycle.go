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
	var paths []ImportPath
	pathKeys := make(map[string]bool)
	seen := map[graph.NodeID]bool{fromID: true}

	var walk func(graph.NodeID, []graph.SymbolRef)
	walk = func(current graph.NodeID, packages []graph.SymbolRef) {
		for _, edge := range g.Outgoing(current, graph.EdgeImports) {
			next := edge.To
			if seen[next] {
				continue
			}
			nextNode, exists := g.Node(next)
			if !exists || nextNode.Kind != graph.NodePackage {
				continue
			}
			nextPackages := append(append([]graph.SymbolRef(nil), packages...), nextNode.Ref)
			if next == toID {
				key := importPathKey(nextPackages)
				if !pathKeys[key] {
					pathKeys[key] = true
					paths = append(paths, ImportPath{Packages: nextPackages})
				}
				continue
			}

			seen[next] = true
			walk(next, nextPackages)
			delete(seen, next)
		}
	}
	walk(fromID, []graph.SymbolRef{from})

	sort.Slice(paths, func(i, j int) bool {
		return importPathLess(paths[i].Packages, paths[j].Packages)
	})
	return paths
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
