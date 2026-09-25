package query

import (
	"sort"
	"strings"

	"nocv/graph"
)

// PackageDependencyPath is one projected architectural route together with
// every exact semantic path that produced that package sequence.
type PackageDependencyPath struct {
	Packages []graph.SymbolID
	Evidence []SemanticPath
}

// PackageDependencyPaths derives all package-level dependency routes between
// two exact package nodes without storing or inventing package edges.
func PackageDependencyPaths(g *graph.Graph, from, to graph.SymbolID) []PackageDependencyPath {
	if g == nil || from == to || !isPackageNode(g, from) || !isPackageNode(g, to) {
		return nil
	}

	sources := packageSemanticSymbols(g, from)
	targets := packageSemanticSymbols(g, to)
	byPackages := make(map[string]*PackageDependencyPath)
	evidenceKeys := make(map[string]map[string]bool)

	for _, source := range sources {
		for _, target := range targets {
			for _, path := range DependencyPaths(g, source, target) {
				packages, ok := projectSemanticPathToPackages(g, path)
				if !ok || len(packages) < 2 || packages[0] != from || packages[len(packages)-1] != to {
					continue
				}

				key := packageSequenceKey(packages)
				projected := byPackages[key]
				if projected == nil {
					projected = &PackageDependencyPath{Packages: append([]graph.SymbolID(nil), packages...)}
					byPackages[key] = projected
					evidenceKeys[key] = make(map[string]bool)
				}
				evidenceKey := semanticPathKey(path.Steps)
				if evidenceKeys[key][evidenceKey] {
					continue
				}
				evidenceKeys[key][evidenceKey] = true
				projected.Evidence = append(projected.Evidence, path)
			}
		}
	}

	results := make([]PackageDependencyPath, 0, len(byPackages))
	for _, projected := range byPackages {
		sort.Slice(projected.Evidence, func(i, j int) bool {
			return semanticPathLess(projected.Evidence[i], projected.Evidence[j])
		})
		results = append(results, *projected)
	}
	sort.Slice(results, func(i, j int) bool {
		return packageSequenceLess(results[i].Packages, results[j].Packages)
	})
	return results
}

func isPackageNode(g *graph.Graph, id graph.SymbolID) bool {
	if g == nil {
		return false
	}
	node, exists := g.Node(id)
	return exists && node.Kind == graph.NodePackage
}

func packageSemanticSymbols(g *graph.Graph, packageID graph.SymbolID) []graph.SymbolID {
	var symbols []graph.SymbolID
	for _, node := range g.Nodes() {
		if node.Kind == graph.NodePackage {
			continue
		}
		owner, exists := g.AncestorOfKind(node.ID, graph.NodePackage)
		if exists && owner == packageID {
			symbols = append(symbols, node.ID)
		}
	}
	return symbols
}

func projectSemanticPathToPackages(g *graph.Graph, path SemanticPath) ([]graph.SymbolID, bool) {
	if len(path.Steps) == 0 {
		return nil, false
	}
	first, exists := g.AncestorOfKind(path.Steps[0].From, graph.NodePackage)
	if !exists {
		return nil, false
	}
	packages := []graph.SymbolID{first}
	for _, step := range path.Steps {
		owner, exists := g.AncestorOfKind(step.To, graph.NodePackage)
		if !exists {
			return nil, false
		}
		if owner != packages[len(packages)-1] {
			packages = append(packages, owner)
		}
	}
	return packages, true
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
