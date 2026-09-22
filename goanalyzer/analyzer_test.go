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
		"example.com/shop/inventory":                        {graph.NodePackage, "inventory", ""},
		"example.com/shop/inventory::Item":                  {graph.NodeStruct, "Item", "example.com/shop/inventory"},
		"example.com/shop/logging":                          {graph.NodePackage, "logging", ""},
		"example.com/shop/logging::Info":                    {graph.NodeFunction, "Info", "example.com/shop/logging"},
		"example.com/shop/orders":                           {graph.NodePackage, "orders", ""},
		"example.com/shop/orders::ConvertOnly":              {graph.NodeFunction, "ConvertOnly", "example.com/shop/orders"},
		"example.com/shop/orders::ExternalOnly":             {graph.NodeFunction, "ExternalOnly", "example.com/shop/orders"},
		"example.com/shop/orders::Process":                  {graph.NodeFunction, "Process", "example.com/shop/orders"},
		"example.com/shop/orders::Validate":                 {graph.NodeFunction, "Validate", "example.com/shop/orders"},
		"example.com/shop/orders::Order":                    {graph.NodeStruct, "Order", "example.com/shop/orders"},
		"example.com/shop/orders::Repository":               {graph.NodeInterface, "Repository", "example.com/shop/orders"},
		"example.com/shop/orders::Repository::Save":         {graph.NodeFunction, "Save", "example.com/shop/orders::Repository"},
		"example.com/shop/orders::Service":                  {graph.NodeStruct, "Service", "example.com/shop/orders"},
		"example.com/shop/orders::Service::Create":          {graph.NodeFunction, "Create", "example.com/shop/orders::Service"},
		"example.com/shop/orders::Service::Health":          {graph.NodeFunction, "Health", "example.com/shop/orders::Service"},
		"example.com/shop/orders::Service::Validate":        {graph.NodeFunction, "Validate", "example.com/shop/orders::Service"},
		"example.com/shop/orders::NewService":               {graph.NodeFunction, "NewService", "example.com/shop/orders"},
		"example.com/shop/orders::Box":                      {graph.NodeStruct, "Box", "example.com/shop/orders"},
		"example.com/shop/orders::Box::Get":                 {graph.NodeFunction, "Get", "example.com/shop/orders::Box"},
		"example.com/shop/orders::Outer":                    {graph.NodeFunction, "Outer", "example.com/shop/orders"},
		"example.com/shop/orders::PostgresRepository":       {graph.NodeStruct, "PostgresRepository", "example.com/shop/orders"},
		"example.com/shop/orders::PostgresRepository::Save": {graph.NodeFunction, "Save", "example.com/shop/orders::PostgresRepository"},
		"example.com/shop/orders::MemoryRepository":         {graph.NodeStruct, "MemoryRepository", "example.com/shop/orders"},
		"example.com/shop/orders::MemoryRepository::Save":   {graph.NodeFunction, "Save", "example.com/shop/orders::MemoryRepository"},
		"example.com/shop/orders::BrokenRepository":         {graph.NodeStruct, "BrokenRepository", "example.com/shop/orders"},
		"example.com/shop/orders::BaseRepository":           {graph.NodeStruct, "BaseRepository", "example.com/shop/orders"},
		"example.com/shop/orders::BaseRepository::Save":     {graph.NodeFunction, "Save", "example.com/shop/orders::BaseRepository"},
		"example.com/shop/orders::PromotedRepository":       {graph.NodeStruct, "PromotedRepository", "example.com/shop/orders"},
		"example.com/shop/orders::Empty":                    {graph.NodeInterface, "Empty", "example.com/shop/orders"},
		"example.com/shop/orders::ExternalStringer":         {graph.NodeStruct, "ExternalStringer", "example.com/shop/orders"},
		"example.com/shop/orders::ExternalStringer::String": {graph.NodeFunction, "String", "example.com/shop/orders::ExternalStringer"},
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
		"example.com/shop/orders::PostgresRepository",
		"example.com/shop/orders::MemoryRepository",
		"example.com/shop/orders::BrokenRepository",
		"example.com/shop/orders::BaseRepository",
		"example.com/shop/orders::PromotedRepository",
		"example.com/shop/orders::Empty",
		"example.com/shop/orders::ExternalStringer",
		"example.com/shop/orders::NewService",
		"example.com/shop/orders::Process",
		"example.com/shop/orders::Validate",
		"example.com/shop/orders::ExternalOnly",
		"example.com/shop/orders::ConvertOnly",
		"example.com/shop/orders::Outer",
	})
	assertChildren(t, g, "example.com/shop/orders::Repository", []graph.SymbolID{
		"example.com/shop/orders::Repository::Save",
	})
	assertChildren(t, g, "example.com/shop/orders::Service", []graph.SymbolID{
		"example.com/shop/orders::Service::Health",
		"example.com/shop/orders::Service::Validate",
		"example.com/shop/orders::Service::Create",
	})

	assertChildren(t, g, "example.com/shop/orders::Box", []graph.SymbolID{
		"example.com/shop/orders::Box::Get",
	})
	assertChildren(t, g, "example.com/shop/orders::PostgresRepository", []graph.SymbolID{
		"example.com/shop/orders::PostgresRepository::Save",
	})
	assertChildren(t, g, "example.com/shop/orders::MemoryRepository", []graph.SymbolID{
		"example.com/shop/orders::MemoryRepository::Save",
	})
	assertChildren(t, g, "example.com/shop/orders::BaseRepository", []graph.SymbolID{
		"example.com/shop/orders::BaseRepository::Save",
	})
}

func TestLoadDiscoversCalls(t *testing.T) {
	g, err := Load(context.Background(), filepath.Join("testdata", "project"), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	assertCall(t, g,
		"example.com/shop/orders::Process",
		"example.com/shop/orders::Validate",
		2,
	)
	assertCall(t, g,
		"example.com/shop/orders::Service::Create",
		"example.com/shop/orders::Service::Health",
		1,
	)
	assertCall(t, g,
		"example.com/shop/orders::Service::Create",
		"example.com/shop/orders::Service::Validate",
		1,
	)
	assertCall(t, g,
		"example.com/shop/orders::Service::Create",
		"example.com/shop/orders::Repository::Save",
		1,
	)
	assertCall(t, g,
		"example.com/shop/orders::Process",
		"example.com/shop/logging::Info",
		1,
	)

	incoming := g.Incoming("example.com/shop/orders::Repository::Save", graph.EdgeCalls)
	if len(incoming) != 1 || incoming[0].From != "example.com/shop/orders::Service::Create" {
		t.Fatalf("incoming calls to Repository.Save = %#v, want one from Service.Create", incoming)
	}

	if edges := g.Outgoing(
		"example.com/shop/orders::Outer",
		graph.EdgeCalls,
	); len(edges) != 0 {
		t.Fatalf("Outer unexpectedly has calls: %#v", edges)
	}

	for _, id := range []graph.SymbolID{
		"example.com/shop/orders::ExternalOnly",
		"example.com/shop/orders::ConvertOnly",
	} {
		if edges := g.Outgoing(id, graph.EdgeCalls); len(edges) != 0 {
			t.Errorf("Outgoing(%q) = %#v, want no calls for builtins, external functions, or conversions", id, edges)
		}
	}
	for _, id := range []graph.SymbolID{"fmt", "fmt::Println", "builtin::len", "builtin::append", "builtin::string"} {
		if node, exists := g.Node(id); exists {
			t.Errorf("external or builtin node %q unexpectedly exists: %#v", id, node)
		}
	}
}

func TestLoadDiscoversInterfaceImplementations(t *testing.T) {
	g, err := Load(context.Background(), filepath.Join("testdata", "project"), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	repositoryID := graph.SymbolID("example.com/shop/orders::Repository")
	repositorySaveID := graph.SymbolID("example.com/shop/orders::Repository::Save")
	wantImplementations := map[graph.SymbolID]bool{
		"example.com/shop/orders::BaseRepository":     true,
		"example.com/shop/orders::MemoryRepository":   true,
		"example.com/shop/orders::PostgresRepository": true,
		"example.com/shop/orders::PromotedRepository": true,
	}
	typeEdges := g.Incoming(repositoryID, graph.EdgeImplements)
	if len(typeEdges) != len(wantImplementations) {
		t.Fatalf("Repository implementations = %#v, want %d", typeEdges, len(wantImplementations))
	}
	for _, edge := range typeEdges {
		if !wantImplementations[edge.From] {
			t.Errorf("unexpected Repository implementation %q", edge.From)
		}
		assertValidEvidence(t, edge)
	}

	wantMethods := map[graph.SymbolID]bool{
		"example.com/shop/orders::BaseRepository::Save":     true,
		"example.com/shop/orders::MemoryRepository::Save":   true,
		"example.com/shop/orders::PostgresRepository::Save": true,
	}
	methodEdges := g.Incoming(repositorySaveID, graph.EdgeImplements)
	if len(methodEdges) != len(wantMethods) {
		t.Fatalf("Repository.Save implementations = %#v, want %d", methodEdges, len(wantMethods))
	}
	for _, edge := range methodEdges {
		if !wantMethods[edge.From] {
			t.Errorf("unexpected Repository.Save implementation %q", edge.From)
		}
		assertValidEvidence(t, edge)
	}

	for _, id := range []graph.SymbolID{
		"example.com/shop/orders::BrokenRepository",
		"example.com/shop/orders::ExternalStringer",
	} {
		if edges := g.Outgoing(id, graph.EdgeImplements); len(edges) != 0 {
			t.Errorf("Outgoing(%q, implements) = %#v, want none", id, edges)
		}
	}
	if edges := g.Incoming("example.com/shop/orders::Empty", graph.EdgeImplements); len(edges) != 0 {
		t.Errorf("empty-interface implementations = %#v, want none", edges)
	}
	if _, exists := g.Node("fmt::Stringer"); exists {
		t.Error("external fmt.Stringer unexpectedly has a graph node")
	}
	if _, exists := g.Node("example.com/shop/orders::PromotedRepository::Save"); exists {
		t.Error("promoted Save unexpectedly has a synthetic method node")
	}
	if edges := g.Outgoing("example.com/shop/orders::PromotedRepository", graph.EdgeImplements); len(edges) != 1 || edges[0].To != repositoryID {
		t.Fatalf("promoted-method type implementation = %#v, want Repository", edges)
	}
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

func assertCall(t *testing.T, g *graph.Graph, from, to graph.SymbolID, evidenceCount int) {
	t.Helper()
	for _, edge := range g.Outgoing(from, graph.EdgeCalls) {
		if edge.To != to {
			continue
		}
		if len(edge.Evidence) != evidenceCount {
			t.Fatalf("call %q -> %q has %d evidence locations, want %d", from, to, len(edge.Evidence), evidenceCount)
		}
		for _, evidence := range edge.Evidence {
			if evidence.File == "" || evidence.Offset < 0 {
				t.Errorf("call %q -> %q has invalid evidence %#v", from, to, evidence)
			}
		}
		return
	}
	t.Fatalf("missing call %q -> %q; outgoing: %#v", from, to, g.Outgoing(from, graph.EdgeCalls))
}

func assertValidEvidence(t *testing.T, edge *graph.Edge) {
	t.Helper()
	if len(edge.Evidence) != 1 || edge.Evidence[0].File == "" || edge.Evidence[0].Offset < 0 {
		t.Errorf("edge %q -> %q has invalid evidence %#v", edge.From, edge.To, edge.Evidence)
	}
}
