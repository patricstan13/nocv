package main

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/internal/testutil"
	"nocv/query"
)

func TestDependencyInspectionDrillsThroughMixedTransitiveFixture(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), testutil.GoProjectDir(t), "./typeview/...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	app := graph.SymbolRef("example.com/shop/typeview/app")
	repository := graph.SymbolRef("example.com/shop/typeview/repository")
	serviceType := graph.SymbolRef("example.com/shop/typeview/service::Service")

	beforePackages := query.PackageDependencyPaths(g, app, repository)
	beforeTypes := query.DirectTypeDependencies(g, serviceType)
	beforeExact := query.DirectDependencies(g, "example.com/shop/typeview/service::Service::Create")

	var first bytes.Buffer
	printDependencyInspection(&first, g, app, repository)
	output := first.String()
	for _, want := range []string{
		"Dependency inspection:",
		"PACKAGE PATH 1",
		"PACKAGE PATH 2",
		"example.com/shop/typeview/app -> example.com/shop/typeview/service",
		"example.com/shop/typeview/service -> example.com/shop/typeview/repository",
		"example.com/shop/typeview/app::Run calls -> example.com/shop/typeview/service::Service::Create",
		"example.com/shop/typeview/service::Service -> example.com/shop/typeview/repository::Repository",
		"TYPE",
		"EXACT ONLY",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("inspection output lacks %q:\n%s", want, output)
		}
	}
	packageOnly := "example.com/shop/typeview/app::Run calls -> example.com/shop/typeview/service::Service::Create"
	if count := strings.Count(output, packageOnly); count != 1 {
		t.Errorf("package-function evidence rendered %d times, want only package evidence once:\n%s", count, output)
	}
	typeBacked := "example.com/shop/typeview/service::Service::Create calls -> example.com/shop/typeview/repository::Repository::Save"
	if count := strings.Count(output, typeBacked); count != 1 {
		t.Errorf("type-backed evidence rendered %d times, want one classified occurrence:\n%s", count, output)
	}
	if strings.Contains(output, " imports -> ") {
		t.Errorf("import leaked into semantic inspection:\n%s", output)
	}

	var second bytes.Buffer
	printDependencyInspection(&second, g, app, repository)
	if second.String() != output {
		t.Fatalf("inspection rendering is not deterministic:\nfirst:\n%s\nsecond:\n%s", output, second.String())
	}
	if after := query.PackageDependencyPaths(g, app, repository); !reflect.DeepEqual(after, beforePackages) {
		t.Fatalf("package paths changed after inspection: %#v", after)
	}
	if after := query.DirectTypeDependencies(g, serviceType); !reflect.DeepEqual(after, beforeTypes) {
		t.Fatalf("type dependencies changed after inspection: %#v", after)
	}
	if after := query.DirectDependencies(g, "example.com/shop/typeview/service::Service::Create"); !reflect.DeepEqual(after, beforeExact) {
		t.Fatalf("exact dependencies changed after inspection: %#v", after)
	}
}

func TestDependencyInspectionShowsMultipleTypesInterfaceAndPackageFunctionEvidence(t *testing.T) {
	g := inspectionFixture(t)
	paths := query.PackageDependencyPaths(g, "service", "repository")
	if len(paths) != 1 || len(paths[0].Steps) != 1 {
		t.Fatalf("package paths = %#v, want one direct step", paths)
	}
	dependencies := query.InspectPackageDependency(g, paths[0].Steps[0]).TypeDependencies
	if len(dependencies) != 2 {
		t.Fatalf("InspectPackageDependency() type dependencies = %#v, want two relationships", dependencies)
	}
	if dependencies[0].From != "service::Service" || dependencies[0].To != "repository::Repository" || len(dependencies[0].Evidence) != 3 {
		t.Fatalf("first dependency = %#v, want Service -> Repository with three exact facts", dependencies[0])
	}
	if dependencies[1].From != "service::Worker" || dependencies[1].To != "repository::Audit" || len(dependencies[1].Evidence) != 1 {
		t.Fatalf("second dependency = %#v, want Worker -> Audit", dependencies[1])
	}

	var out bytes.Buffer
	printDependencyInspection(&out, g, "service", "repository")
	output := out.String()
	for _, want := range []string{
		"service::Migrate calls -> repository::Repository::Save",
		"service::Service -> repository::Repository",
		"service::Service implements -> repository::Repository",
		"service::Service::Create implements -> repository::Repository::Save",
		"service::Worker -> repository::Audit",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("inspection output lacks %q:\n%s", want, output)
		}
	}
	if strings.Count(output, "service::Migrate calls -> repository::Repository::Save") != 1 {
		t.Errorf("package-function fact should remain package-only:\n%s", output)
	}
	if strings.Contains(output, " imports -> ") {
		t.Errorf("import leaked into inspection:\n%s", output)
	}
}

func TestDependencyInspectionStopsWhenNoPackageDependencyExists(t *testing.T) {
	g, ids := cliFixture(t)
	var out bytes.Buffer
	printDependencyInspection(&out, g, ids.packageB, ids.packageA)
	output := out.String()
	if !strings.Contains(output, "PACKAGE\n  no dependency") {
		t.Fatalf("missing clean no-dependency result:\n%s", output)
	}
	if strings.Contains(output, "HOP") || strings.Contains(output, "TYPE") || strings.Contains(output, "EXACT") {
		t.Fatalf("no-dependency inspection attempted lower-level drilldown:\n%s", output)
	}
}

func TestNodeInspectionOmitsPackageLocationButKeepsDeclarationLocations(t *testing.T) {
	g, err := goanalyzer.Load(context.Background(), testutil.GoProjectDir(t), "./typeview/...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	var packageOutput bytes.Buffer
	printNodeInspection(&packageOutput, g, "example.com/shop/typeview/service")
	if !strings.Contains(packageOutput.String(), "Kind:\n  package") {
		t.Fatalf("package inspection lacks package kind:\n%s", packageOutput.String())
	}
	if strings.Contains(packageOutput.String(), "Location:") || strings.Contains(packageOutput.String(), "service.go:") {
		t.Fatalf("package inspection rendered a declaration location:\n%s", packageOutput.String())
	}

	for id, wantLocation := range map[graph.SymbolRef]string{
		"example.com/shop/typeview/service::Service":         "service.go:5:6",
		"example.com/shop/typeview/service::Service::Create": "service.go:7:16",
	} {
		var declarationOutput bytes.Buffer
		printNodeInspection(&declarationOutput, g, id)
		if !strings.Contains(declarationOutput.String(), "Location:") || !strings.Contains(declarationOutput.String(), wantLocation) {
			t.Errorf("declaration inspection %s lacks source location:\n%s", id, declarationOutput.String())
		}
	}
}

func inspectionFixture(t *testing.T) *graph.Graph {
	t.Helper()
	g := graph.New()
	for _, node := range []fixtureNode{
		{ID: "service", Kind: graph.NodePackage, Name: "service"},
		{ID: "repository", Kind: graph.NodePackage, Name: "repository"},
		{ID: "service::Service", Kind: graph.NodeStruct, Name: "Service", Parent: "service"},
		{ID: "service::Worker", Kind: graph.NodeStruct, Name: "Worker", Parent: "service"},
		{ID: "repository::Repository", Kind: graph.NodeInterface, Name: "Repository", Parent: "repository"},
		{ID: "repository::Audit", Kind: graph.NodeStruct, Name: "Audit", Parent: "repository"},
		{ID: "service::Migrate", Kind: graph.NodeFunction, Name: "Migrate", Parent: "service"},
		{ID: "service::Service::Create", Kind: graph.NodeFunction, Name: "Create", Parent: "service::Service"},
		{ID: "service::Worker::Sync", Kind: graph.NodeFunction, Name: "Sync", Parent: "service::Worker"},
		{ID: "repository::Repository::Save", Kind: graph.NodeFunction, Name: "Save", Parent: "repository::Repository"},
		{ID: "repository::Audit::Record", Kind: graph.NodeFunction, Name: "Record", Parent: "repository::Audit"},
	} {
		if err := addFixtureNode(g, node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}
	for index, edge := range []fixtureEdge{
		{From: "service::Migrate", To: "repository::Repository::Save", Kind: graph.EdgeCalls},
		{From: "service::Service::Create", To: "repository::Repository::Save", Kind: graph.EdgeCalls},
		{From: "service::Service", To: "repository::Repository", Kind: graph.EdgeImplements},
		{From: "service::Service::Create", To: "repository::Repository::Save", Kind: graph.EdgeImplements},
		{From: "service::Worker::Sync", To: "repository::Audit::Record", Kind: graph.EdgeCalls},
		{From: "service", To: "repository", Kind: graph.EdgeImports},
	} {
		edge.Evidence = []graph.Location{{File: "inspection.go", Offset: index}}
		if err := addFixtureEdge(g, edge); err != nil {
			t.Fatalf("AddEdge(%s, %q -> %q): %v", edge.Kind, edge.From, edge.To, err)
		}
	}
	return g
}
