package goanalyzer

import (
	"context"
	"path/filepath"
	"testing"

	"nocv/graph"
)

func TestLoadDiscoversStructuralHierarchy(t *testing.T) {
	g, err := Load(context.Background(), filepath.Join("testdata", "project"), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	want := map[graph.SymbolID]struct {
		kind   graph.NodeKind
		name   string
		parent graph.SymbolID
	}{
		"example.com/shop/inventory":                {graph.NodePackage, "inventory", ""},
		"example.com/shop/inventory::Item":          {graph.NodeStruct, "Item", "example.com/shop/inventory"},
		"example.com/shop/orders":                   {graph.NodePackage, "orders", ""},
		"example.com/shop/orders::Order":            {graph.NodeStruct, "Order", "example.com/shop/orders"},
		"example.com/shop/orders::Repository":       {graph.NodeInterface, "Repository", "example.com/shop/orders"},
		"example.com/shop/orders::Repository::Save": {graph.NodeFunction, "Save", "example.com/shop/orders::Repository"},
		"example.com/shop/orders::Service":          {graph.NodeStruct, "Service", "example.com/shop/orders"},
		"example.com/shop/orders::Service::Create":  {graph.NodeFunction, "Create", "example.com/shop/orders::Service"},
		"example.com/shop/orders::Service::Health":  {graph.NodeFunction, "Health", "example.com/shop/orders::Service"},
		"example.com/shop/orders::NewService":       {graph.NodeFunction, "NewService", "example.com/shop/orders"},
		"example.com/shop/orders::Box":              {graph.NodeStruct, "Box", "example.com/shop/orders"},
		"example.com/shop/orders::Box::Get":         {graph.NodeFunction, "Get", "example.com/shop/orders::Box"},
	}

	if got := len(g.Nodes()); got != len(want) {
		t.Fatalf("node count = %d, want %d; nodes: %#v", got, len(want), g.Nodes())
	}
	for id, expected := range want {
		node, ok := g.Node(id)
		if !ok {
			t.Errorf("missing node %q", id)
			continue
		}
		if node.Kind != expected.kind || node.Name != expected.name || node.Parent != expected.parent {
			t.Errorf("node %q = (%s, %q, parent %q), want (%s, %q, parent %q)",
				id, node.Kind, node.Name, node.Parent, expected.kind, expected.name, expected.parent)
		}
		if node.Location.File == "" || node.Location.Offset < 0 {
			t.Errorf("node %q has invalid location %#v", id, node.Location)
		}
	}

	assertChildren(t, g, "example.com/shop/orders", []graph.SymbolID{
		"example.com/shop/orders::Order",
		"example.com/shop/orders::Repository",
		"example.com/shop/orders::Service",
		"example.com/shop/orders::Box",
		"example.com/shop/orders::NewService",
	})
	assertChildren(t, g, "example.com/shop/orders::Repository", []graph.SymbolID{
		"example.com/shop/orders::Repository::Save",
	})
	assertChildren(t, g, "example.com/shop/orders::Service", []graph.SymbolID{
		"example.com/shop/orders::Service::Health",
		"example.com/shop/orders::Service::Create",
	})

	assertChildren(t, g, "example.com/shop/orders::Box", []graph.SymbolID{
		"example.com/shop/orders::Box::Get",
	})
}

func TestLoadProducesDeterministicIDs(t *testing.T) {
	dir := filepath.Join("testdata", "project")
	first, err := Load(context.Background(), dir, "./...")
	if err != nil {
		t.Fatalf("first Load(): %v", err)
	}
	second, err := Load(context.Background(), dir, "./...")
	if err != nil {
		t.Fatalf("second Load(): %v", err)
	}

	firstNodes, secondNodes := first.Nodes(), second.Nodes()
	if len(firstNodes) != len(secondNodes) {
		t.Fatalf("node counts differ: %d and %d", len(firstNodes), len(secondNodes))
	}
	for i := range firstNodes {
		if firstNodes[i].ID != secondNodes[i].ID {
			t.Fatalf("ID %d differs: %q and %q", i, firstNodes[i].ID, secondNodes[i].ID)
		}
	}
}

func assertChildren(t *testing.T, g *graph.Graph, parent graph.SymbolID, want []graph.SymbolID) {
	t.Helper()
	got := g.Children(parent)
	if len(got) != len(want) {
		t.Fatalf("Children(%q) = %v, want %v", parent, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Children(%q)[%d] = %q, want %q", parent, i, got[i], want[i])
		}
	}
}
