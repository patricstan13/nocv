package query

import "nocv/graph"

// PackageImportViolation is one forbidden direct package import together with
// every source location where that import is declared.
type PackageImportViolation struct {
	From     graph.SymbolID
	To       graph.SymbolID
	Evidence []graph.Location
}

// CheckForbiddenPackageImport reports whether the exact direct import from ->
// to exists. It does not inspect transitive imports or semantic dependencies.
func CheckForbiddenPackageImport(
	g *graph.Graph,
	from graph.SymbolID,
	to graph.SymbolID,
) (PackageImportViolation, bool) {
	if !isPackageNode(g, from) || !isPackageNode(g, to) {
		return PackageImportViolation{}, false
	}

	for _, relationship := range DirectImports(g, from) {
		if relationship.To != to {
			continue
		}
		return PackageImportViolation{
			From:     relationship.From,
			To:       relationship.To,
			Evidence: append([]graph.Location(nil), relationship.Evidence...),
		}, true
	}
	return PackageImportViolation{}, false
}
