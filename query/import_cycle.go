package query

import (
	"sort"
	"strings"

	"nocv/graph"
)

// ImportPath is one simple path through stored package import relationships.
type ImportPath struct {
	Packages []graph.SymbolID
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
func WouldCreateImportCycle(g *graph.Graph, from, to graph.SymbolID) ImportCycleCheck {
	if !isPackageNode(g, from) || !isPackageNode(g, to) {
		return ImportCycleCheck{}
	}
	if from == to {
		return ImportCycleCheck{
			WouldCycle: true,
			Paths:      []ImportPath{{Packages: []graph.SymbolID{from}}},
		}
	}

	paths := importPaths(g, to, from)
	return ImportCycleCheck{WouldCycle: len(paths) > 0, Paths: paths}
}

func importPaths(g *graph.Graph, from, to graph.SymbolID) []ImportPath {
	var paths []ImportPath
	pathKeys := make(map[string]bool)
	seen := map[graph.SymbolID]bool{from: true}

	var walk func(graph.SymbolID, []graph.SymbolID)
	walk = func(current graph.SymbolID, packages []graph.SymbolID) {
		for _, relationship := range DirectImports(g, current) {
			next := relationship.To
			if seen[next] {
				continue
			}
			nextPackages := append(append([]graph.SymbolID(nil), packages...), next)
			if next == to {
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
	walk(from, []graph.SymbolID{from})

	sort.Slice(paths, func(i, j int) bool {
		return importPathLess(paths[i].Packages, paths[j].Packages)
	})
	return paths
}

func importPathLess(left, right []graph.SymbolID) bool {
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

func importPathKey(packages []graph.SymbolID) string {
	var key strings.Builder
	for _, packageID := range packages {
		key.WriteString(string(packageID))
		key.WriteByte(0)
	}
	return key.String()
}
