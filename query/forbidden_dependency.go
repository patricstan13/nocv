package query

import "nocv/graph"

// PackageDependencyViolation is one forbidden semantic package dependency
// together with every package route and its hop-level boundary evidence.
type PackageDependencyViolation struct {
	From  graph.SymbolRef
	To    graph.SymbolRef
	Paths []PackageDependencyPath
}

// CheckForbiddenPackageDependency reports whether PackageDependencyPaths
// contains at least one semantic dependency route from -> to.
func CheckForbiddenPackageDependency(
	g *graph.Graph,
	from graph.SymbolRef,
	to graph.SymbolRef,
) (PackageDependencyViolation, bool) {
	paths := PackageDependencyPaths(g, from, to)
	if len(paths) == 0 {
		return PackageDependencyViolation{}, false
	}
	return PackageDependencyViolation{From: from, To: to, Paths: paths}, true
}
