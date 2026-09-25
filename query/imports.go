package query

import (
	"sort"

	"nocv/graph"
)

// DirectImports returns represented packages imported directly by packageID.
// Import relationships are separate from semantic dependency navigation.
func DirectImports(g *graph.Graph, packageID graph.SymbolID) []Relationship {
	if !isPackageNode(g, packageID) {
		return nil
	}

	relationships := copyRelationships(g.Outgoing(packageID, graph.EdgeImports))
	sort.Slice(relationships, func(i, j int) bool {
		return relationships[i].To < relationships[j].To
	})
	return relationships
}

// DirectImporters returns represented packages that directly import packageID.
// Import relationships are separate from semantic dependent navigation.
func DirectImporters(g *graph.Graph, packageID graph.SymbolID) []Relationship {
	if !isPackageNode(g, packageID) {
		return nil
	}

	relationships := copyRelationships(g.Incoming(packageID, graph.EdgeImports))
	sort.Slice(relationships, func(i, j int) bool {
		return relationships[i].From < relationships[j].From
	})
	return relationships
}
