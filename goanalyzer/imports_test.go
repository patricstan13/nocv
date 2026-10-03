package goanalyzer

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"nocv/graph"
	"nocv/internal/testutil"
)

func TestLoadDiscoversRepresentedPackageImports(t *testing.T) {
	g, err := Load(context.Background(), testutil.GoProjectDir(t), "./imports/...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	app := graph.SymbolRef("example.com/shop/imports/app")
	want := map[graph.SymbolRef]int{
		"example.com/shop/imports/helpers":    1,
		"example.com/shop/imports/plugin":     1,
		"example.com/shop/imports/repository": 1,
		"example.com/shop/imports/service":    2,
	}
	edges := outgoingByRef(g, app, graph.EdgeImports)
	if len(edges) != len(want) {
		t.Fatalf("app imports = %#v, want %d represented imports", edges, len(want))
	}
	for _, edge := range edges {
		wantEvidence, exists := want[edge.To]
		if !exists {
			t.Errorf("unexpected import edge %#v", edge)
			continue
		}
		if len(edge.Evidence) != wantEvidence {
			t.Errorf("%s evidence = %#v, want %d locations", edge.To, edge.Evidence, wantEvidence)
		}
		for _, evidence := range edge.Evidence {
			if evidence.File == "" || evidence.Offset < 0 {
				t.Errorf("%s has invalid evidence %#v", edge.To, evidence)
			}
		}
	}

	service := outgoingByRef(g, app, graph.EdgeImports)
	for _, edge := range service {
		if edge.To != "example.com/shop/imports/service" {
			continue
		}
		if !strings.HasSuffix(edge.Evidence[0].File, filepath.Join("app", "a.go")) ||
			!strings.HasSuffix(edge.Evidence[1].File, filepath.Join("app", "b.go")) {
			t.Fatalf("service evidence order/files = %#v, want a.go then b.go", edge.Evidence)
		}
	}

	if _, exists := nodeByRef(g, "fmt"); exists {
		t.Fatal("Load() created a synthetic node for external package fmt")
	}
	if edges := outgoingByRef(g, app, graph.EdgeImports); containsImportTarget(edges, "fmt") {
		t.Fatalf("external fmt import was stored: %#v", edges)
	}

	importers := incomingByRef(g, "example.com/shop/imports/service", graph.EdgeImports)
	if len(importers) != 2 || importers[0].From != app || importers[1].From != "example.com/shop/imports/worker" {
		t.Fatalf("service importers = %#v, want app and worker", importers)
	}
}

func containsImportTarget(edges []*symbolEdge, target graph.SymbolRef) bool {
	for _, edge := range edges {
		if edge.To == target {
			return true
		}
	}
	return false
}
