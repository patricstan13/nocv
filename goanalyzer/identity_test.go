package goanalyzer

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"nocv/graph"
	"nocv/query"
)

func TestLoadAssignsDistinctRefsAndCallsToRepeatedInitFunctions(t *testing.T) {
	g, err := Load(context.Background(), filepath.Join("testdata", "identity"), "./repeated")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	init1 := graph.SymbolRef("example.com/identity/repeated::init#1")
	init2 := graph.SymbolRef("example.com/identity/repeated::init#2")
	one := graph.SymbolRef("example.com/identity/repeated::one")
	two := graph.SymbolRef("example.com/identity/repeated::two")
	first, firstExists := g.NodeByRef(init1)
	second, secondExists := g.NodeByRef(init2)
	if !firstExists || !secondExists {
		t.Fatalf("repeated init nodes missing: first=%#v second=%#v", first, second)
	}
	if first.ID == second.ID || first.ID == 0 || second.ID == 0 || first.Name != "init" || second.Name != "init" {
		t.Fatalf("repeated init nodes = %#v, %#v", first, second)
	}
	if got := query.DirectDependencies(g, init1); !reflect.DeepEqual(relationshipEndpoints(got), [][2]graph.SymbolRef{{init1, one}}) {
		t.Fatalf("init#1 calls = %#v", got)
	}
	if got := query.DirectDependencies(g, init2); !reflect.DeepEqual(relationshipEndpoints(got), [][2]graph.SymbolRef{{init2, two}}) {
		t.Fatalf("init#2 calls = %#v", got)
	}
}

func TestLoadAssignsDistinctRefsToBlankDeclarations(t *testing.T) {
	g, err := Load(context.Background(), filepath.Join("testdata", "identity"), "./blank")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	want := []graph.SymbolRef{
		"example.com/identity/blank::_#1",
		"example.com/identity/blank::_#2",
		"example.com/identity/blank::_#3",
		"example.com/identity/blank::_#4",
	}
	ids := make(map[graph.NodeID]bool)
	for _, ref := range want {
		node, exists := g.NodeByRef(ref)
		if !exists {
			t.Errorf("missing blank declaration %q", ref)
			continue
		}
		if node.Name != "_" || node.ID == 0 || ids[node.ID] {
			t.Errorf("blank declaration %q = %#v", ref, node)
		}
		ids[node.ID] = true
	}
}

func TestLoadPreservesNormalSymbolRefs(t *testing.T) {
	g, err := Load(context.Background(), filepath.Join("testdata", "typeview"), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	for _, ref := range []graph.SymbolRef{
		"example.com/typeview/app",
		"example.com/typeview/service::Service",
		"example.com/typeview/service::Service::Create",
		"example.com/typeview/app::Run",
	} {
		if _, exists := g.NodeByRef(ref); !exists {
			t.Errorf("normal symbol ref changed: %q", ref)
		}
	}
}

func relationshipEndpoints(relationships []query.Relationship) [][2]graph.SymbolRef {
	result := make([][2]graph.SymbolRef, len(relationships))
	for index, relationship := range relationships {
		result[index] = [2]graph.SymbolRef{relationship.From, relationship.To}
	}
	return result
}
