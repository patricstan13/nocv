package goanalyzer

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"nocv/graph"
	"nocv/internal/testutil"
)

func TestLoadDiscoversFieldTypeRelationships(t *testing.T) {
	g, err := Load(context.Background(), testutil.GoProjectDir(t), "./fielddeps/...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	const (
		modelB     = graph.SymbolRef("example.com/shop/fielddeps/model::B")
		modelC     = graph.SymbolRef("example.com/shop/fielddeps/model::C")
		modelBox   = graph.SymbolRef("example.com/shop/fielddeps/model::Box")
		direct     = graph.SymbolRef("example.com/shop/fielddeps/consumer::Direct")
		containers = graph.SymbolRef("example.com/shop/fielddeps/consumer::Containers")
		nested     = graph.SymbolRef("example.com/shop/fielddeps/consumer::Nested")
		node       = graph.SymbolRef("example.com/shop/fielddeps/consumer::Node")
		embedded   = graph.SymbolRef("example.com/shop/fielddeps/model::Embedded")
	)

	assertRelationship(t, g, direct, modelB, graph.EdgeFieldType, 1)
	directEdge := outgoingByRef(g, direct, graph.EdgeFieldType)[0]
	if got := directEdge.Evidence[0]; got.Line != 6 || got.Column != 8 || !strings.HasSuffix(got.File, "/fielddeps/consumer/consumer.go") {
		t.Errorf("direct field evidence = %#v, want consumer.go:6:8", got)
	}

	assertRelationship(t, g, containers, modelB, graph.EdgeFieldType, 7)
	containerEdge := outgoingByRef(g, containers, graph.EdgeFieldType)[0]
	var evidenceLines []int
	for _, evidence := range containerEdge.Evidence {
		evidenceLines = append(evidenceLines, evidence.Line)
	}
	if want := []int{10, 11, 12, 13, 14, 15, 16}; !reflect.DeepEqual(evidenceLines, want) {
		t.Errorf("container evidence lines = %v, want %v", evidenceLines, want)
	}

	assertRelationship(t, g, nested, modelBox, graph.EdgeFieldType, 1)
	assertRelationship(t, g, nested, modelB, graph.EdgeFieldType, 2)
	assertRelationship(t, g, nested, modelC, graph.EdgeFieldType, 1)
	if edges := outgoingByRef(g, nested, graph.EdgeFieldType); len(edges) != 3 {
		t.Errorf("nested field relationships = %#v, want only Box, B, and C", edges)
	}
	if edges := outgoingByRef(g, nested, graph.EdgeAccepts, graph.EdgeReturns); len(edges) != 0 {
		t.Errorf("function-valued field produced callable signature relationships: %#v", edges)
	}

	assertRelationship(t, g, embedded, modelB, graph.EdgeEmbeds, 1)
	if edges := outgoingByRef(g, embedded, graph.EdgeFieldType); len(edges) != 0 {
		t.Errorf("embedded field also produced FieldType: %#v", edges)
	}

	assertRelationship(t, g, node, node, graph.EdgeFieldType, 1)
	if edges := outgoingByRef(g, node, graph.EdgeFieldType); len(edges) != 1 || len(edges[0].Evidence) != 1 {
		t.Errorf("recursive field relationship = %#v, want one self edge with one evidence location", edges)
	}

	for _, ref := range []graph.SymbolRef{
		"example.com/shop/fielddeps/consumer::string",
		"example.com/shop/fielddeps/consumer::error",
	} {
		if _, exists := g.NodeByRef(ref); exists {
			t.Errorf("builtin acquired graph node %q", ref)
		}
	}
}
