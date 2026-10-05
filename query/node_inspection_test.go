package query_test

import (
	"context"
	"reflect"
	"testing"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/internal/testutil"
	"nocv/query"
)

func TestInspectNodePackageSeparatesChildrenSemanticsAndImports(t *testing.T) {
	g, ids := typeDependencyFixture(t)
	inspection, ok := query.InspectNode(g, ids.pkg)
	if !ok || inspection.Package == nil || inspection.Type != nil || inspection.Function != nil {
		t.Fatalf("InspectNode(package) = %#v, %v", inspection, ok)
	}
	if inspection.Node.Ref != ids.pkg || inspection.Node.Kind != graph.NodePackage {
		t.Errorf("package envelope node = %#v", inspection.Node)
	}
	if got, want := nodeIDs(inspection.Package.Types), []graph.SymbolRef{ids.childInterface, ids.parentInterface, ids.service}; !reflect.DeepEqual(got, want) {
		t.Errorf("package types = %v, want %v", got, want)
	}
	if got, want := nodeIDs(inspection.Package.Functions), []graph.SymbolRef{ids.run}; !reflect.DeepEqual(got, want) {
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
	wantMethods := []graph.SymbolRef{
		ids.serviceCreate,
		"example.com/types::Service::Delete",
		"example.com/types::Service::Update",
		ids.serviceValidate,
	}
	if got := summaryRefs(service.Type.Methods); !reflect.DeepEqual(got, wantMethods) {
		t.Errorf("Service methods = %v, want %v", got, wantMethods)
	}
	createSummary := service.Type.Methods[0]
	if createSummary.Ref != ids.serviceCreate || createSummary.Kind != graph.NodeFunction || createSummary.Name != "Create" ||
		createSummary.ParentRef != ids.service || createSummary.ParentName != "Service" {
		t.Errorf("Service.Create summary = %#v", createSummary)
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
	if service.Type.DirectDependencies[0].From.Ref != ids.service || service.Type.DirectDependencies[0].From.Name != "Service" ||
		service.Type.DirectDependencies[0].From.ParentRef != ids.pkg || service.Type.DirectDependencies[0].To.Ref != ids.contract ||
		service.Type.DirectDependencies[0].To.Kind != graph.NodeInterface || service.Type.DirectDependencies[0].To.ParentRef != ids.otherPackage {
		t.Errorf("Service direct exact dependency summaries = %#v", service.Type.DirectDependencies)
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
	if !ok || create.Function == nil || inspectionParentRef(g, create.Node) != ids.service {
		t.Fatalf("InspectNode(Service.Create) = %#v, %v", create, ok)
	}
	if len(create.Function.Calls) != 3 || len(create.Function.Accepts) != 1 || len(create.Function.Returns) != 1 ||
		len(create.Function.Implements) != 1 || len(create.Function.CalledBy) != 1 || len(create.Function.ImplementedBy) != 0 {
		t.Fatalf("Service.Create categories = %#v", create.Function)
	}
	if got, want := relationshipTargets(create.Function.Calls), []graph.SymbolRef{
		"example.com/other::Repository::Save", ids.helper, ids.serviceValidate,
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("Service.Create calls = %v, want %v", got, want)
	}
	if create.Function.Accepts[0].To.Ref != ids.repository || create.Function.Accepts[0].To.Name != "Repository" ||
		create.Function.Returns[0].To.Ref != ids.repository || create.Function.Implements[0].To.Ref != "example.com/other::Contract::Execute" {
		t.Errorf("Service.Create categorized endpoints = %#v", create.Function)
	}
	calledBy := create.Function.CalledBy[0]
	if calledBy.From.Ref != ids.run || calledBy.Kind != graph.EdgeCalls || calledBy.From.ParentRef != ids.pkg || calledBy.From.ParentName != "types" {
		t.Errorf("Service.Create called by = %#v", calledBy)
	}
	if got, want := create.Function.Calls[0].Evidence, []graph.Location{{File: "types.go", Offset: 0}}; !reflect.DeepEqual(got, want) {
		t.Errorf("Service.Create call evidence = %#v, want %#v", got, want)
	}

	interfaceMethod, ok := query.InspectNode(g, "example.com/other::Contract::Execute")
	if !ok || interfaceMethod.Function == nil || inspectionParentRef(g, interfaceMethod.Node) != ids.contract {
		t.Fatalf("InspectNode(Contract.Execute) = %#v, %v", interfaceMethod, ok)
	}
	if len(interfaceMethod.Function.Accepts) != 1 || interfaceMethod.Function.Accepts[0].Kind != graph.EdgeAccepts {
		t.Errorf("Contract.Execute accepts = %#v", interfaceMethod.Function.Accepts)
	}
	if len(interfaceMethod.Function.ImplementedBy) != 1 || interfaceMethod.Function.ImplementedBy[0].Kind != graph.EdgeImplements ||
		interfaceMethod.Function.ImplementedBy[0].To.ParentName != "Contract" {
		t.Errorf("Contract.Execute implemented by = %#v", interfaceMethod.Function.ImplementedBy)
	}
	contract, ok := query.InspectNode(g, ids.contract)
	if !ok || contract.Type == nil || len(contract.Type.Methods) != 1 {
		t.Fatalf("InspectNode(Contract) = %#v, %v", contract, ok)
	}
	interfaceSummary := contract.Type.Methods[0]
	if interfaceSummary.Ref != "example.com/other::Contract::Execute" || interfaceSummary.Name != "Execute" ||
		interfaceSummary.Kind != graph.NodeFunction || interfaceSummary.ParentRef != ids.contract || interfaceSummary.ParentName != "Contract" {
		t.Errorf("Contract.Execute method summary = %#v", interfaceSummary)
	}

	packageFunction, ok := query.InspectNode(g, ids.run)
	if !ok || packageFunction.Function == nil || inspectionParentRef(g, packageFunction.Node) != ids.pkg || len(packageFunction.Function.Calls) != 1 {
		t.Fatalf("InspectNode(Run) = %#v, %v", packageFunction, ok)
	}
}

func TestInspectNodeIntegratesDocumentationAndAnalyzerSemantics(t *testing.T) {
	documentationGraph, err := goanalyzer.Load(context.Background(), testutil.GoProjectDir(t), "./documentation/...")
	if err != nil {
		t.Fatalf("load documentation fixture: %v", err)
	}
	const documentationPackage = graph.SymbolRef("example.com/shop/documentation/service")

	packageInspection, ok := query.InspectNode(documentationGraph, documentationPackage)
	if !ok || packageInspection.Package == nil || packageInspection.Node.Documentation == "" {
		t.Fatalf("documentation package inspection = %#v, %v", packageInspection, ok)
	}
	service, ok := query.InspectNode(documentationGraph, documentationPackage+"::Service")
	if !ok || service.Type == nil || service.Node.Documentation != "Service coordinates order creation.\n\nIt validates requests before persistence." ||
		len(service.Type.Methods) != 1 || service.Type.Methods[0].Ref != documentationPackage+"::Service::Create" {
		t.Fatalf("documented Service inspection = %#v, %v", service, ok)
	}
	repository, ok := query.InspectNode(documentationGraph, documentationPackage+"::Repository")
	if !ok || repository.Type == nil || repository.Node.Documentation != "Repository defines persistence behavior." ||
		len(repository.Type.Methods) != 1 || repository.Type.Methods[0].Ref != documentationPackage+"::Repository::Save" ||
		repository.Type.Methods[0].ParentName != "Repository" {
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

	typeGraph, err := goanalyzer.Load(context.Background(), testutil.GoProjectDir(t), "./typeview/...")
	if err != nil {
		t.Fatalf("load typeview fixture: %v", err)
	}
	packageView, ok := query.InspectNode(typeGraph, "example.com/shop/typeview/service")
	if !ok || packageView.Package == nil || len(packageView.Package.Dependencies) != 1 || len(packageView.Package.Dependents) != 1 ||
		len(packageView.Package.Imports) != 1 || len(packageView.Package.Importers) != 1 {
		t.Fatalf("typeview package inspection = %#v, %v", packageView, ok)
	}
	if packageView.Node.Location != (graph.Location{}) {
		t.Errorf("typeview package inspection has declaration location %#v", packageView.Node.Location)
	}
	typeView, ok := query.InspectNode(typeGraph, "example.com/shop/typeview/service::Service")
	if !ok || typeView.Type == nil || len(typeView.Type.Methods) != 3 || len(typeView.Type.Dependencies) != 1 {
		t.Fatalf("typeview Service inspection = %#v, %v", typeView, ok)
	}
	if typeView.Node.Location.File == "" {
		t.Errorf("typeview Service inspection lost declaration location: %#v", typeView.Node.Location)
	}
	functionView, ok := query.InspectNode(typeGraph, "example.com/shop/typeview/service::Service::Create")
	if !ok || functionView.Function == nil || len(functionView.Function.Calls) != 1 || len(functionView.Function.Accepts) != 1 || len(functionView.Function.CalledBy) != 1 {
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
	beforeTransitiveDependents := query.TransitiveDependents(g, "example.com/other::Repository::Save")
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
	functionInspection.Function.Calls[0].Evidence[0].Offset = 903
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
	if freshFunction.Function.Calls[0].Evidence[0].Offset >= 900 {
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
	assertDeepEqual(t, "TransitiveDependents", query.TransitiveDependents(g, "example.com/other::Repository::Save"), beforeTransitiveDependents)
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

func nodeIDs(nodes []graph.Node) []graph.SymbolRef {
	ids := make([]graph.SymbolRef, len(nodes))
	for index, node := range nodes {
		ids[index] = node.Ref
	}
	return ids
}

func summaryRefs(summaries []query.SymbolSummary) []graph.SymbolRef {
	refs := make([]graph.SymbolRef, len(summaries))
	for index, summary := range summaries {
		refs[index] = summary.Ref
	}
	return refs
}

func relationshipTargets(relationships []query.SymbolRelationship) []graph.SymbolRef {
	refs := make([]graph.SymbolRef, len(relationships))
	for index, relationship := range relationships {
		refs[index] = relationship.To.Ref
	}
	return refs
}

func inspectionParentRef(g *graph.Graph, node graph.Node) graph.SymbolRef {
	parent, _ := g.Node(node.Parent)
	if parent == nil {
		return ""
	}
	return parent.Ref
}

func assertDeepEqual(t *testing.T, name string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s changed:\ngot:  %#v\nwant: %#v", name, got, want)
	}
}
