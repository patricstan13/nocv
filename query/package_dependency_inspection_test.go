package query_test

import (
	"reflect"
	"testing"

	"nocv/graph"
	"nocv/query"
)

func TestInspectPackageDependencyPartitionsMixedEvidenceExactlyOnce(t *testing.T) {
	g := packageInspectionFixture(t)
	dependency := directPackageStep(t, g, "service", "repository")
	inspection := query.InspectPackageDependency(g, dependency)
	if !reflect.DeepEqual(inspection.Dependency, dependency) {
		t.Fatalf("detached dependency = %#v, want inspected edge %#v", inspection.Dependency, dependency)
	}

	if len(inspection.TypeDependencies) != 3 {
		t.Fatalf("TypeDependencies = %#v, want Child->Parent, Service->Repository, and Worker->Audit", inspection.TypeDependencies)
	}
	wantPairs := [][2]graph.SymbolRef{
		{"service::Child", "repository::Parent"},
		{"service::Service", "repository::Repository"},
		{"service::Worker", "repository::Audit"},
	}
	for index, pair := range wantPairs {
		dependency := inspection.TypeDependencies[index]
		if dependency.From != pair[0] || dependency.To != pair[1] {
			t.Errorf("type dependency %d = %s -> %s, want %s -> %s", index, dependency.From, dependency.To, pair[0], pair[1])
		}
	}
	if len(inspection.TypeDependencies[1].Evidence) != 4 {
		t.Fatalf("Service -> Repository evidence = %#v, want calls, accepts, and two Implements facts", inspection.TypeDependencies[1].Evidence)
	}
	for _, dependency := range inspection.TypeDependencies {
		for index := 1; index < len(dependency.Evidence); index++ {
			if relationshipOrderLess(dependency.Evidence[index], dependency.Evidence[index-1]) {
				t.Errorf("type evidence is not deterministic: %#v", dependency.Evidence)
			}
		}
	}
	if len(inspection.ExactOnly) != 2 {
		t.Fatalf("ExactOnly = %#v, want two package-function-boundary facts", inspection.ExactOnly)
	}
	if inspection.ExactOnly[0].From != "service::Migrate" || inspection.ExactOnly[1].To != "repository::helper" {
		t.Fatalf("ExactOnly ordering/content = %#v", inspection.ExactOnly)
	}
	assertInspectionPartition(t, dependency, inspection)
	assertInspectionHasNoImports(t, inspection)

	again := query.InspectPackageDependency(g, dependency)
	if !reflect.DeepEqual(again, inspection) {
		t.Fatalf("repeated inspection is not deterministic:\nfirst: %#v\nagain: %#v", inspection, again)
	}
}

func TestInspectPackageDependencyClassifiesPackageFunctionOnlyEvidence(t *testing.T) {
	g := packageInspectionFixture(t)
	dependency := directPackageStep(t, g, "app", "service")
	inspection := query.InspectPackageDependency(g, dependency)

	if len(inspection.TypeDependencies) != 0 {
		t.Fatalf("package-function-only TypeDependencies = %#v, want empty", inspection.TypeDependencies)
	}
	if len(inspection.ExactOnly) != 2 {
		t.Fatalf("package-function-only ExactOnly = %#v, want call and accepts facts", inspection.ExactOnly)
	}
	wantKinds := map[graph.EdgeKind]bool{graph.EdgeCalls: true, graph.EdgeAccepts: true}
	for _, relationship := range inspection.ExactOnly {
		wantKinds[relationship.Kind] = false
	}
	for kind, missing := range wantKinds {
		if missing {
			t.Errorf("ExactOnly lacks %s fact: %#v", kind, inspection.ExactOnly)
		}
	}
	assertInspectionPartition(t, dependency, inspection)
}

func TestInspectPackageDependencyFiltersToConcreteFullyTypeBackedEvidence(t *testing.T) {
	g := packageInspectionFixture(t)
	full := directPackageStep(t, g, "service", "repository")
	subset := query.PackageDependency{From: full.From, To: full.To}
	for _, relationship := range full.Evidence {
		if relationship.From != "service::Migrate" && relationship.To != "repository::helper" {
			subset.Evidence = append(subset.Evidence, relationship)
		}
	}

	inspection := query.InspectPackageDependency(g, subset)
	if len(inspection.TypeDependencies) != 3 || len(inspection.ExactOnly) != 0 {
		t.Fatalf("fully type-backed inspection = %#v", inspection)
	}
	assertInspectionPartition(t, subset, inspection)
	for _, typeDependency := range inspection.TypeDependencies {
		for _, relationship := range typeDependency.Evidence {
			if relationship.From == "service::Migrate" || relationship.To == "repository::helper" {
				t.Errorf("evidence outside concrete package dependency leaked into type classification: %#v", relationship)
			}
		}
	}
}

func TestInspectPackageDependencyReturnsDetachedResultsAndHandlesInvalidInput(t *testing.T) {
	g := packageInspectionFixture(t)
	pathsBefore := query.PackageDependencyPaths(g, "service", "repository")
	dependency := pathsBefore[0].Steps[0]
	inputBefore := copyPackageDependencyForTest(dependency)
	inspection := query.InspectPackageDependency(g, dependency)

	inspection.Dependency.Evidence[0].From = "mutated dependency"
	inspection.Dependency.Evidence[0].Evidence[0].Offset = 900
	inspection.TypeDependencies[0].From = "mutated type"
	inspection.TypeDependencies[0].Evidence[0].Evidence[0].Offset = 901
	inspection.ExactOnly[0].From = "mutated exact"
	inspection.ExactOnly[0].Evidence[0].Offset = 902

	if !reflect.DeepEqual(dependency, inputBefore) {
		t.Fatalf("inspection mutated input dependency:\ngot:  %#v\nwant: %#v", dependency, inputBefore)
	}
	if after := query.PackageDependencyPaths(g, "service", "repository"); !reflect.DeepEqual(after, pathsBefore) {
		t.Fatalf("inspection mutation affected package paths:\ngot:  %#v\nwant: %#v", after, pathsBefore)
	}
	again := query.InspectPackageDependency(g, dependency)
	if again.Dependency.Evidence[0].From == "mutated dependency" ||
		again.TypeDependencies[0].From == "mutated type" || again.ExactOnly[0].From == "mutated exact" ||
		again.Dependency.Evidence[0].Evidence[0].Offset >= 900 ||
		again.TypeDependencies[0].Evidence[0].Evidence[0].Offset >= 900 || again.ExactOnly[0].Evidence[0].Offset >= 900 {
		t.Fatalf("future inspection retained returned-result mutation: %#v", again)
	}

	invalid := []query.PackageDependency{
		{},
		{From: "service", To: "repository"},
		{From: "service", To: "service", Evidence: dependency.Evidence},
		{From: "missing", To: "repository", Evidence: dependency.Evidence},
		{From: "service::Service", To: "repository", Evidence: dependency.Evidence},
		{From: "service", To: "repository::Repository", Evidence: dependency.Evidence},
	}
	if got := query.InspectPackageDependency(nil, dependency); !reflect.DeepEqual(got, query.PackageDependencyInspection{}) {
		t.Fatalf("nil graph inspection = %#v, want zero result", got)
	}
	for _, malformed := range invalid {
		if got := query.InspectPackageDependency(g, malformed); !reflect.DeepEqual(got, query.PackageDependencyInspection{}) {
			t.Errorf("invalid inspection for %#v = %#v, want zero result", malformed, got)
		}
	}
}

type inspectionRelationshipID struct {
	from graph.SymbolRef
	to   graph.SymbolRef
	kind graph.EdgeKind
}

func assertInspectionPartition(t *testing.T, dependency query.PackageDependency, inspection query.PackageDependencyInspection) {
	t.Helper()
	want := make(map[inspectionRelationshipID]query.Relationship, len(dependency.Evidence))
	for _, relationship := range dependency.Evidence {
		want[inspectionID(relationship)] = relationship
	}
	counts := make(map[inspectionRelationshipID]int, len(want))
	check := func(relationship query.Relationship) {
		id := inspectionID(relationship)
		counts[id]++
		original, exists := want[id]
		if !exists {
			t.Errorf("classified relationship absent from package evidence: %#v", relationship)
		} else if !reflect.DeepEqual(relationship, original) {
			t.Errorf("classified relationship lost evidence:\ngot:  %#v\nwant: %#v", relationship, original)
		}
	}
	for _, dependency := range inspection.TypeDependencies {
		for _, relationship := range dependency.Evidence {
			check(relationship)
		}
	}
	for _, relationship := range inspection.ExactOnly {
		check(relationship)
	}
	if len(counts) != len(want) {
		t.Errorf("classified identity set = %#v, want %#v", counts, want)
	}
	for id := range want {
		if counts[id] != 1 {
			t.Errorf("package evidence %#v classified %d times, want exactly once", id, counts[id])
		}
	}
}

func assertInspectionHasNoImports(t *testing.T, inspection query.PackageDependencyInspection) {
	t.Helper()
	for _, relationship := range inspection.Dependency.Evidence {
		if relationship.Kind == graph.EdgeImports {
			t.Errorf("import leaked into inspected package dependency: %#v", relationship)
		}
	}
	for _, dependency := range inspection.TypeDependencies {
		for _, relationship := range dependency.Evidence {
			if relationship.Kind == graph.EdgeImports {
				t.Errorf("import leaked into type classification: %#v", relationship)
			}
		}
	}
	for _, relationship := range inspection.ExactOnly {
		if relationship.Kind == graph.EdgeImports {
			t.Errorf("import leaked into exact-only classification: %#v", relationship)
		}
	}
}

func inspectionID(relationship query.Relationship) inspectionRelationshipID {
	return inspectionRelationshipID{from: relationship.From, to: relationship.To, kind: relationship.Kind}
}

func directPackageStep(t *testing.T, g *graph.Graph, from, to graph.SymbolRef) query.PackageDependency {
	t.Helper()
	paths := query.PackageDependencyPaths(g, from, to)
	if len(paths) != 1 || len(paths[0].Steps) != 1 {
		t.Fatalf("PackageDependencyPaths(%s, %s) = %#v, want one direct step", from, to, paths)
	}
	return paths[0].Steps[0]
}

func copyPackageDependencyForTest(dependency query.PackageDependency) query.PackageDependency {
	copy := dependency
	copy.Evidence = append([]query.Relationship(nil), dependency.Evidence...)
	for index := range copy.Evidence {
		copy.Evidence[index].Evidence = append([]graph.Location(nil), dependency.Evidence[index].Evidence...)
	}
	return copy
}

func packageInspectionFixture(t *testing.T) *graph.Graph {
	t.Helper()
	g := graph.New()
	for _, node := range []testNode{
		{ID: "app", Kind: graph.NodePackage, Name: "app"},
		{ID: "service", Kind: graph.NodePackage, Name: "service"},
		{ID: "repository", Kind: graph.NodePackage, Name: "repository"},
		{ID: "app::Run", Kind: graph.NodeFunction, Name: "Run", Parent: "app"},
		{ID: "service::Child", Kind: graph.NodeInterface, Name: "Child", Parent: "service"},
		{ID: "service::Service", Kind: graph.NodeStruct, Name: "Service", Parent: "service"},
		{ID: "service::Worker", Kind: graph.NodeStruct, Name: "Worker", Parent: "service"},
		{ID: "service::Migrate", Kind: graph.NodeFunction, Name: "Migrate", Parent: "service"},
		{ID: "service::Service::Create", Kind: graph.NodeFunction, Name: "Create", Parent: "service::Service"},
		{ID: "service::Worker::Sync", Kind: graph.NodeFunction, Name: "Sync", Parent: "service::Worker"},
		{ID: "repository::Parent", Kind: graph.NodeInterface, Name: "Parent", Parent: "repository"},
		{ID: "repository::Repository", Kind: graph.NodeInterface, Name: "Repository", Parent: "repository"},
		{ID: "repository::Audit", Kind: graph.NodeStruct, Name: "Audit", Parent: "repository"},
		{ID: "repository::Repository::Save", Kind: graph.NodeFunction, Name: "Save", Parent: "repository::Repository"},
		{ID: "repository::Audit::Record", Kind: graph.NodeFunction, Name: "Record", Parent: "repository::Audit"},
		{ID: "repository::helper", Kind: graph.NodeFunction, Name: "helper", Parent: "repository"},
	} {
		if err := addTestNode(g, node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}
	for index, edge := range []testEdge{
		{From: "app::Run", To: "service::Service::Create", Kind: graph.EdgeCalls},
		{From: "app::Run", To: "service::Service", Kind: graph.EdgeAccepts},
		{From: "service::Migrate", To: "repository::Repository::Save", Kind: graph.EdgeCalls},
		{From: "service::Service::Create", To: "repository::Repository::Save", Kind: graph.EdgeCalls},
		{From: "service::Service::Create", To: "repository::Repository", Kind: graph.EdgeAccepts},
		{From: "service::Service::Create", To: "repository::helper", Kind: graph.EdgeCalls},
		{From: "service::Service", To: "repository::Repository", Kind: graph.EdgeImplements},
		{From: "service::Service::Create", To: "repository::Repository::Save", Kind: graph.EdgeImplements},
		{From: "service::Worker::Sync", To: "repository::Audit::Record", Kind: graph.EdgeCalls},
		{From: "service::Child", To: "repository::Parent", Kind: graph.EdgeEmbeds},
		{From: "service", To: "repository", Kind: graph.EdgeImports},
	} {
		edge.Evidence = []graph.Location{{File: "inspection.go", Offset: index}}
		if err := addTestEdge(g, edge); err != nil {
			t.Fatalf("AddEdge(%s, %q -> %q): %v", edge.Kind, edge.From, edge.To, err)
		}
	}
	return g
}
