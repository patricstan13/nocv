package query_test

import (
	"context"
	"testing"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/internal/testutil"
	"nocv/query"
)

func TestFieldTypeFlowsThroughSemanticDependencyViews(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), testutil.GoProjectDir(t), "./fielddeps/...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	const (
		consumerPkg = graph.SymbolRef("example.com/shop/fielddeps/consumer")
		modelPkg    = graph.SymbolRef("example.com/shop/fielddeps/model")
		direct      = graph.SymbolRef("example.com/shop/fielddeps/consumer::Direct")
		nested      = graph.SymbolRef("example.com/shop/fielddeps/consumer::Nested")
		node        = graph.SymbolRef("example.com/shop/fielddeps/consumer::Node")
		modelB      = graph.SymbolRef("example.com/shop/fielddeps/model::B")
		modelBox    = graph.SymbolRef("example.com/shop/fielddeps/model::Box")
		modelC      = graph.SymbolRef("example.com/shop/fielddeps/model::C")
	)

	exact := query.DirectDependencies(g, direct)
	if len(exact) != 1 || exact[0].Kind != graph.EdgeFieldType || exact[0].To != modelB {
		t.Fatalf("Direct dependencies for Direct = %#v, want FieldType -> B", exact)
	}
	dependents := query.DirectDependents(g, modelB)
	if len(dependents) == 0 || !containsRelationship(dependents, direct, modelB, graph.EdgeFieldType) {
		t.Fatalf("Direct dependents for B = %#v, want Direct FieldType fact", dependents)
	}
	nodeInspection, exists := query.InspectNode(g, direct)
	if !exists || nodeInspection.Type == nil || len(nodeInspection.Type.DirectDependencies) != 1 || nodeInspection.Type.DirectDependencies[0].Kind != graph.EdgeFieldType {
		t.Fatalf("Direct node inspection = %#v, want exact FieldType relationship", nodeInspection)
	}
	transitive := query.TransitiveDependents(g, modelB)
	if !containsTransitiveFieldDependent(transitive, direct) {
		t.Fatalf("transitive dependents for B = %#v, want Direct through FieldType", transitive)
	}

	typeDependencies := query.DirectTypeDependencies(g, nested)
	if len(typeDependencies) != 3 || typeDependencies[0].To != modelB || typeDependencies[1].To != modelBox || typeDependencies[2].To != modelC {
		t.Fatalf("Nested type dependencies = %#v, want B, Box, and C", typeDependencies)
	}
	for _, dependency := range typeDependencies {
		if len(dependency.Evidence) != 1 || dependency.Evidence[0].Kind != graph.EdgeFieldType {
			t.Errorf("type dependency lacks FieldType evidence: %#v", dependency)
		}
	}

	packageDependencies := query.DirectPackageDependencies(g, consumerPkg)
	if len(packageDependencies) != 1 || packageDependencies[0].To != modelPkg {
		t.Fatalf("consumer package dependencies = %#v, want field-only dependency on model", packageDependencies)
	}
	for _, evidence := range packageDependencies[0].Evidence {
		if evidence.Kind != graph.EdgeFieldType {
			t.Errorf("non-field semantic fact leaked into field-only package dependency: %#v", evidence)
		}
	}

	packageInspection := query.InspectPackageDependency(g, packageDependencies[0])
	if len(packageInspection.TypeDependencies) != 5 || len(packageInspection.ExactOnly) != 0 {
		t.Fatalf("field package inspection = %#v, want five type dependencies and no exact-only facts", packageInspection)
	}
	paths := query.PackageDependencyPaths(g, consumerPkg, modelPkg)
	if len(paths) != 1 || len(paths[0].Steps) != 1 {
		t.Fatalf("field-only package paths = %#v, want one direct path", paths)
	}
	typePaths := query.TypeDependencyPaths(g, nested, modelC)
	if len(typePaths) != 1 || len(typePaths[0].Steps) != 1 || typePaths[0].Steps[0].Evidence[0].Kind != graph.EdgeFieldType {
		t.Fatalf("field-only type paths = %#v, want one direct FieldType path", typePaths)
	}
	if paths := query.DependencyPaths(g, direct, modelB); len(paths) != 1 || paths[0].Steps[0].Kind != graph.EdgeFieldType {
		t.Fatalf("exact field dependency paths = %#v, want direct FieldType path", paths)
	}
	// The graph permits a recursive field's exact self edge, while the existing
	// type projection intentionally suppresses same-type dependencies.
	if dependencies := query.DirectTypeDependencies(g, node); len(dependencies) != 0 {
		t.Fatalf("recursive Node projected a same-type dependency: %#v", dependencies)
	}
}

func containsRelationship(relationships []query.Relationship, from, to graph.SymbolRef, kind graph.EdgeKind) bool {
	for _, relationship := range relationships {
		if relationship.From == from && relationship.To == to && relationship.Kind == kind {
			return true
		}
	}
	return false
}

func containsTransitiveFieldDependent(dependents []query.TransitiveDependent, id graph.SymbolRef) bool {
	for _, dependent := range dependents {
		if dependent.ID == id && len(dependent.Paths) == 1 && len(dependent.Paths[0].Steps) == 1 && dependent.Paths[0].Steps[0].Kind == graph.EdgeFieldType {
			return true
		}
	}
	return false
}
