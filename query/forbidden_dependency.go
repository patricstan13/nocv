package query

import "nocv/graph"

// PackageDependencyViolation is one forbidden semantic package dependency
// together with every projected package route and exact semantic explanation.
type PackageDependencyViolation struct {
	From  graph.SymbolID
	To    graph.SymbolID
	Paths []PackageDependencyPath
}

// CheckForbiddenPackageDependency reports whether PackageDependencyPaths
// contains at least one semantic dependency route from -> to.
func CheckForbiddenPackageDependency(
	g *graph.Graph,
	from graph.SymbolID,
	to graph.SymbolID,
) (PackageDependencyViolation, bool) {
	paths := PackageDependencyPaths(g, from, to)
	if len(paths) == 0 {
		return PackageDependencyViolation{}, false
	}
	return PackageDependencyViolation{From: from, To: to, Paths: paths}, true
}
