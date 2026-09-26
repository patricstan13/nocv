package query_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/query"
)

func TestInspectNodePackageSeparatesChildrenSemanticsAndImports(t *testing.T) {
	g, ids := typeDependencyFixture(t)
	inspection, ok := query.InspectNode(g, ids.pkg)
	if !ok || inspection.Package == nil || inspection.Type != nil || inspection.Function != nil {
		t.Fatalf("InspectNode(package) = %#v, %v", inspection, ok)
	}
	if inspection.Node.ID != ids.pkg || inspection.Node.Kind != graph.NodePackage {
		t.Errorf("package envelope node = %#v", inspection.Node)
	}
	if got, want := nodeIDs(inspection.Package.Types), []graph.SymbolID{ids.childInterface, ids.parentInterface, ids.service}; !reflect.DeepEqual(got, want) {
		t.Errorf("package types = %v, want %v", got, want)
	}
	if got, want := nodeIDs(inspection.Package.Functions), []graph.SymbolID{ids.run}; !reflect.DeepEqual(got, want) {
		t.Errorf("package functions = %v, want %v", got, want)
	}
	if len(inspection.Package.Dependencies) != 1 || inspection.Package.Dependencies[0].From != ids.pkg || inspection.Package.Dependencies[0].To != ids.otherPackage {
		t.Errorf("package semantic dependencies = %#v", inspection.Package.Dependencies)
	}
	if len(inspection.Package.Dependents) != 0 {
		t.Errorf("package semantic dependents = %#v, want empty", inspection.Package.Dependents)
	}
	if len(inspection.Package.Imports) != 1 || inspection.Package.Imports[0].To != ids.otherPackage {
		t.Errorf("package imports = %#v", inspection.Package.Imports)
	}
	if len(inspection.Package.Importers) != 0 {
		t.Errorf("package importers = %#v, want empty", inspection.Package.Importers)
	}

	incoming, ok := query.InspectNode(g, ids.otherPackage)
	if !ok || incoming.Package == nil || len(incoming.Package.Dependents) != 1 ||
		incoming.Package.Dependents[0].From != ids.pkg || incoming.Package.Dependents[0].To != ids.otherPackage {
		t.Fatalf("incoming package dependents = %#v, %v", incoming, ok)
	}
	if len(incoming.Package.Importers) != 1 || incoming.Package.Importers[0].From != ids.pkg {
		t.Errorf("incoming package importers = %#v", incoming.Package.Importers)
	}

	again, _ := query.InspectNode(g, ids.pkg)
	if !reflect.DeepEqual(again, inspection) {
		t.Fatalf("package inspection is not deterministic:\nfirst: %#v\nagain: %#v", inspection, again)
	}
}

func TestInspectNodeTypeAndFunctionUseProjectedAndExactSemantics(t *testing.T) {
	g, ids := typeDependencyFixture(t)
	service, ok := query.InspectNode(g, ids.service)
	if !ok || service.Type == nil || service.Package != nil || service.Function != nil {
		t.Fatalf("InspectNode(Service) = %#v, %v", service, ok)
	}
	wantMethods := []graph.SymbolID{
		ids.serviceCreate,
		"example.com/types::Service::Delete",
		"example.com/types::Service::Update",
		ids.serviceValidate,
	}
	if got := nodeIDs(service.Type.Methods); !reflect.DeepEqual(got, wantMethods) {
		t.Errorf("Service methods = %v, want %v", got, wantMethods)
	}
	if len(service.Type.Dependencies) != 2 || service.Type.Dependencies[0].To != ids.contract || service.Type.Dependencies[1].To != ids.repository {
		t.Errorf("Service type dependencies = %#v", service.Type.Dependencies)
	}
	if len(service.Type.Dependents) != 0 {
		t.Errorf("Service type dependents = %#v, want empty", service.Type.Dependents)
	}
	if len(service.Type.DirectDependencies) != 2 {
		t.Errorf("Service direct exact dependencies = %#v, want Implements and Embeds", service.Type.DirectDependencies)
	}
	if len(service.Type.DirectDependents) != 0 {
		t.Errorf("Service direct exact dependents = %#v, want empty", service.Type.DirectDependents)
	}

	repository, ok := query.InspectNode(g, ids.repository)
	if !ok || repository.Type == nil || len(repository.Type.Dependents) != 2 ||
		repository.Type.Dependents[0].From != ids.contract || repository.Type.Dependents[1].From != ids.service {
		t.Fatalf("Repository type dependents = %#v, %v", repository, ok)
	}

	create, ok := query.InspectNode(g, ids.serviceCreate)
	if !ok || create.Function == nil || create.Node.Parent != ids.service {
		t.Fatalf("InspectNode(Service.Create) = %#v, %v", create, ok)
	}
	wantOutgoingKinds := map[graph.EdgeKind]bool{
		graph.EdgeCalls: true, graph.EdgeImplements: true, graph.EdgeAccepts: true, graph.EdgeReturns: true,
	}
	for _, relationship := range create.Function.Dependencies {
		wantOutgoingKinds[relationship.Kind] = false
	}
	for kind, missing := range wantOutgoingKinds {
		if missing {
			t.Errorf("Service.Create dependencies lack %s: %#v", kind, create.Function.Dependencies)
		}
	}
	if len(create.Function.Dependents) != 1 || create.Function.Dependents[0].From != ids.run || create.Function.Dependents[0].Kind != graph.EdgeCalls {
		t.Errorf("Service.Create dependents = %#v", create.Function.Dependents)
	}

	interfaceMethod, ok := query.InspectNode(g, "example.com/other::Contract::Execute")
	if !ok || interfaceMethod.Function == nil || interfaceMethod.Node.Parent != ids.contract {
		t.Fatalf("InspectNode(Contract.Execute) = %#v, %v", interfaceMethod, ok)
	}
	if len(interfaceMethod.Function.Dependencies) != 1 || interfaceMethod.Function.Dependencies[0].Kind != graph.EdgeAccepts {
		t.Errorf("Contract.Execute dependencies = %#v", interfaceMethod.Function.Dependencies)
	}
	if len(interfaceMethod.Function.Dependents) != 1 || interfaceMethod.Function.Dependents[0].Kind != graph.EdgeImplements {
		t.Errorf("Contract.Execute dependents = %#v", interfaceMethod.Function.Dependents)
	}

	packageFunction, ok := query.InspectNode(g, ids.run)
	if !ok || packageFunction.Function == nil || packageFunction.Node.Parent != ids.pkg || len(packageFunction.Function.Dependencies) != 1 {
		t.Fatalf("InspectNode(Run) = %#v, %v", packageFunction, ok)
	}
}

func TestInspectNodeIntegratesDocumentationAndAnalyzerSemantics(t *testing.T) {
	documentationGraph, err := goanalyzer.Load(context.Background(), filepath.Join("..", "goanalyzer", "testdata", "documentation"), "./...")
	if err != nil {
		t.Fatalf("load documentation fixture: %v", err)
	}
	const documentationPackage = graph.SymbolID("example.com/documentation/service")

	packageInspection, ok := query.InspectNode(documentationGraph, documentationPackage)
	if !ok || packageInspection.Package == nil || packageInspection.Node.Documentation == "" {
		t.Fatalf("documentation package inspection = %#v, %v", packageInspection, ok)
	}
	service, ok := query.InspectNode(documentationGraph, documentationPackage+"::Service")
	if !ok || service.Type == nil || service.Node.Documentation != "Service coordinates order creation.\n\nIt validates requests before persistence." ||
		len(service.Type.Methods) != 1 || service.Type.Methods[0].Documentation != "Create persists a new order." {
		t.Fatalf("documented Service inspection = %#v, %v", service, ok)
	}
	repository, ok := query.InspectNode(documentationGraph, documentationPackage+"::Repository")
	if !ok || repository.Type == nil || repository.Node.Documentation != "Repository defines persistence behavior." ||
		len(repository.Type.Methods) != 1 || repository.Type.Methods[0].Documentation != "Save persists an entity." {
		t.Fatalf("documented Repository inspection = %#v, %v", repository, ok)
	}
	interfaceMethod, ok := query.InspectNode(documentationGraph, documentationPackage+"::Repository::Save")
	if !ok || interfaceMethod.Function == nil || interfaceMethod.Node.Documentation != "Save persists an entity." {
		t.Fatalf("documented Repository.Save inspection = %#v, %v", interfaceMethod, ok)
	}
	run, ok := query.InspectNode(documentationGraph, documentationPackage+"::Run")
	if !ok || run.Function == nil || run.Node.Documentation != "Run starts the application." {
		t.Fatalf("documented Run inspection = %#v, %v", run, ok)
	}

	typeGraph, err := goanalyzer.Load(context.Background(), filepath.Join("..", "goanalyzer", "testdata", "typeview"), "./...")
	if err != nil {
		t.Fatalf("load typeview fixture: %v", err)
	}
	packageView, ok := query.InspectNode(typeGraph, "example.com/typeview/service")
	if !ok || packageView.Package == nil || len(packageView.Package.Dependencies) != 1 || len(packageView.Package.Dependents) != 1 ||
		len(packageView.Package.Imports) != 1 || len(packageView.Package.Importers) != 1 {
		t.Fatalf("typeview package inspection = %#v, %v", packageView, ok)
	}
	if packageView.Node.Location != (graph.Location{}) {
		t.Errorf("typeview package inspection has declaration location %#v", packageView.Node.Location)
	}
	typeView, ok := query.InspectNode(typeGraph, "example.com/typeview/service::Service")
	if !ok || typeView.Type == nil || len(typeView.Type.Methods) != 3 || len(typeView.Type.Dependencies) != 1 {
		t.Fatalf("typeview Service inspection = %#v, %v", typeView, ok)
	}
	if typeView.Node.Location.File == "" {
		t.Errorf("typeview Service inspection lost declaration location: %#v", typeView.Node.Location)
	}
	functionView, ok := query.InspectNode(typeGraph, "example.com/typeview/service::Service::Create")
	if !ok || functionView.Function == nil || len(functionView.Function.Dependencies) != 2 || len(functionView.Function.Dependents) != 1 {
		t.Fatalf("typeview Service.Create inspection = %#v, %v", functionView, ok)
	}
	if functionView.Node.Location.File == "" {
		t.Errorf("typeview Service.Create inspection lost declaration location: %#v", functionView.Node.Location)
	}
}

func TestInspectNodeIsDetachedAndDoesNotChangeExistingQueries(t *testing.T) {
	g, ids := typeDependencyFixture(t)
	beforeGraph := importGraphSnapshot(g)
	beforeExact := query.DirectDependencies(g, ids.serviceCreate)
	beforeExactDependents := query.DirectDependents(g, ids.repository)
	beforePackages := query.DirectPackageDependencies(g, ids.pkg)
	beforePackageDependents := query.DirectPackageDependents(g, ids.otherPackage)
	beforeTypes := query.DirectTypeDependencies(g, ids.service)
	beforeTypeDependents := query.DirectTypeDependents(g, ids.repository)
	beforePackagePaths := query.PackageDependencyPaths(g, ids.pkg, ids.otherPackage)
	beforeTypePaths := query.TypeDependencyPaths(g, ids.service, ids.repository)
	beforeExactPaths := query.DependencyPaths(g, ids.serviceCreate, "example.com/other::Repository::Save")
	beforeImpact := query.Impact(g, "example.com/other::Repository::Save")
	beforeDrilldown := query.InspectPackageDependency(g, beforePackagePaths[0].Steps[0])

	inspection, ok := query.InspectNode(g, ids.pkg)
	if !ok {
		t.Fatal("InspectNode(package) returned false")
	}
	inspection.Node.Name = "mutated"
	inspection.Package.Types[0].Name = "mutated child"
	inspection.Package.Dependencies[0].Evidence[0].Evidence[0].Offset = 900
	inspection.Package.Imports[0].Evidence[0].Offset = 901
	typeInspection, _ := query.InspectNode(g, ids.service)
	typeInspection.Type.Methods[0].Name = "mutated method"
	typeInspection.Type.Dependencies[0].Evidence[0].Evidence[0].Offset = 902
	functionInspection, _ := query.InspectNode(g, ids.serviceCreate)
	functionInspection.Function.Dependencies[0].Evidence[0].Offset = 903
	packageDependents := query.DirectPackageDependents(g, ids.otherPackage)
	packageDependents[0].Evidence[0].Evidence[0].Offset = 904
	typeDependents := query.DirectTypeDependents(g, ids.repository)
	typeDependents[0].Evidence[0].Evidence[0].Offset = 905
	freshPackage, _ := query.InspectNode(g, ids.pkg)
	if freshPackage.Node.Name == "mutated" || freshPackage.Package.Types[0].Name == "mutated child" {
		t.Fatalf("package inspection retained returned-node mutation: %#v", freshPackage)
	}
	freshType, _ := query.InspectNode(g, ids.service)
	if freshType.Type.Methods[0].Name == "mutated method" || freshType.Type.Dependencies[0].Evidence[0].Evidence[0].Offset >= 900 {
		t.Fatalf("type inspection retained returned-result mutation: %#v", freshType)
	}
	freshFunction, _ := query.InspectNode(g, ids.serviceCreate)
	if freshFunction.Function.Dependencies[0].Evidence[0].Offset >= 900 {
		t.Fatalf("function inspection retained returned-result mutation: %#v", freshFunction)
	}

	if after := importGraphSnapshot(g); !reflect.DeepEqual(after, beforeGraph) {
		t.Fatalf("node inspection mutated graph:\nbefore: %#v\nafter:  %#v", beforeGraph, after)
	}
	assertDeepEqual(t, "DirectDependencies", query.DirectDependencies(g, ids.serviceCreate), beforeExact)
	assertDeepEqual(t, "DirectDependents", query.DirectDependents(g, ids.repository), beforeExactDependents)
	assertDeepEqual(t, "DirectPackageDependencies", query.DirectPackageDependencies(g, ids.pkg), beforePackages)
	assertDeepEqual(t, "DirectPackageDependents", query.DirectPackageDependents(g, ids.otherPackage), beforePackageDependents)
	assertDeepEqual(t, "DirectTypeDependencies", query.DirectTypeDependencies(g, ids.service), beforeTypes)
	assertDeepEqual(t, "DirectTypeDependents", query.DirectTypeDependents(g, ids.repository), beforeTypeDependents)
	assertDeepEqual(t, "PackageDependencyPaths", query.PackageDependencyPaths(g, ids.pkg, ids.otherPackage), beforePackagePaths)
	assertDeepEqual(t, "TypeDependencyPaths", query.TypeDependencyPaths(g, ids.service, ids.repository), beforeTypePaths)
	assertDeepEqual(t, "DependencyPaths", query.DependencyPaths(g, ids.serviceCreate, "example.com/other::Repository::Save"), beforeExactPaths)
	assertDeepEqual(t, "Impact", query.Impact(g, "example.com/other::Repository::Save"), beforeImpact)
	assertDeepEqual(t, "InspectPackageDependency", query.InspectPackageDependency(g, beforePackagePaths[0].Steps[0]), beforeDrilldown)

	if _, ok := query.InspectNode(nil, ids.pkg); ok {
		t.Error("InspectNode(nil) returned true")
	}
	if _, ok := query.InspectNode(g, "missing"); ok {
		t.Error("InspectNode(missing) returned true")
	}
	for _, got := range [][]query.PackageDependency{
		query.DirectPackageDependents(nil, ids.otherPackage),
		query.DirectPackageDependents(g, "missing"),
		query.DirectPackageDependents(g, ids.service),
	} {
		if len(got) != 0 {
			t.Errorf("invalid package dependents = %#v, want empty", got)
		}
	}
	for _, got := range [][]query.TypeDependency{
		query.DirectTypeDependents(nil, ids.repository),
		query.DirectTypeDependents(g, "missing"),
		query.DirectTypeDependents(g, ids.pkg),
		query.DirectTypeDependents(g, ids.serviceCreate),
	} {
		if len(got) != 0 {
			t.Errorf("invalid type dependents = %#v, want empty", got)
		}
	}
}

func nodeIDs(nodes []graph.Node) []graph.SymbolID {
	ids := make([]graph.SymbolID, len(nodes))
	for index, node := range nodes {
		ids[index] = node.ID
	}
	return ids
}

func assertDeepEqual(t *testing.T, name string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s changed:\ngot:  %#v\nwant: %#v", name, got, want)
	}
}
