package query

import (
	"sort"

	"nocv/graph"
)

// PackageDependencyInspection classifies the exact evidence beneath one
// direct derived package dependency. Every dependency fact belongs to either
// one type dependency or ExactOnly.
type PackageDependencyInspection struct {
	Dependency       PackageDependency
	TypeDependencies []TypeDependency
	ExactOnly        []Relationship
}

type relationshipIdentity struct {
	from graph.SymbolRef
	to   graph.SymbolRef
	kind graph.EdgeKind
}

// InspectPackageDependency classifies one direct package dependency into
// existing type-view relationships and exact facts with no type projection.
func InspectPackageDependency(g *graph.Graph, dependency PackageDependency) PackageDependencyInspection {
	if g == nil || len(dependency.Evidence) == 0 || dependency.From == dependency.To ||
		!isPackageNode(g, dependency.From) || !isPackageNode(g, dependency.To) {
		return PackageDependencyInspection{}
	}

	inspection := PackageDependencyInspection{Dependency: copyPackageDependency(dependency)}
	packageEvidence := make(map[relationshipIdentity]Relationship, len(dependency.Evidence))
	for _, relationship := range dependency.Evidence {
		packageEvidence[identityOf(relationship)] = relationship
	}
	matched := make(map[relationshipIdentity]bool, len(packageEvidence))

	for _, typeID := range typesInPackage(g, dependency.From) {
		for _, typeDependency := range DirectTypeDependencies(g, typeID) {
			target, exists := g.NodeByRef(typeDependency.To)
			if !exists {
				continue
			}
			parent, parentExists := g.Node(target.Parent)
			if !parentExists || parent.Ref != dependency.To {
				continue
			}

			filtered := TypeDependency{From: typeDependency.From, To: typeDependency.To}
			for _, relationship := range typeDependency.Evidence {
				identity := identityOf(relationship)
				packageRelationship, exists := packageEvidence[identity]
				if !exists || matched[identity] {
					continue
				}
				filtered.Evidence = append(filtered.Evidence, copyRelationship(packageRelationship))
				matched[identity] = true
			}
			if len(filtered.Evidence) != 0 {
				inspection.TypeDependencies = append(inspection.TypeDependencies, filtered)
			}
		}
	}

	sort.Slice(inspection.TypeDependencies, func(i, j int) bool {
		if inspection.TypeDependencies[i].From != inspection.TypeDependencies[j].From {
			return inspection.TypeDependencies[i].From < inspection.TypeDependencies[j].From
		}
		return inspection.TypeDependencies[i].To < inspection.TypeDependencies[j].To
	})
	for _, relationship := range dependency.Evidence {
		if !matched[identityOf(relationship)] {
			inspection.ExactOnly = append(inspection.ExactOnly, copyRelationship(relationship))
		}
	}
	sort.Slice(inspection.ExactOnly, func(i, j int) bool {
		return semanticRelationshipLess(inspection.ExactOnly[i], inspection.ExactOnly[j])
	})
	return inspection
}

func identityOf(relationship Relationship) relationshipIdentity {
	return relationshipIdentity{from: relationship.From, to: relationship.To, kind: relationship.Kind}
}

func typesInPackage(g *graph.Graph, packageID graph.SymbolRef) []graph.SymbolRef {
	var types []graph.SymbolRef
	packageNode, exists := g.NodeByRef(packageID)
	if !exists {
		return nil
	}
	for _, id := range g.Children(packageNode.ID) {
		node, exists := g.Node(id)
		if exists && (node.Kind == graph.NodeStruct || node.Kind == graph.NodeInterface) {
			types = append(types, node.Ref)
		}
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	return types
}

func copyRelationship(relationship Relationship) Relationship {
	copy := relationship
	copy.Evidence = append([]graph.Location(nil), relationship.Evidence...)
	return copy
}
