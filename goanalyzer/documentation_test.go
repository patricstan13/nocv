package goanalyzer

import (
	"context"
	"reflect"
	"testing"

	"nocv/graph"
	"nocv/internal/testutil"
)

func TestLoadAttachesDeclarationDocumentation(t *testing.T) {
	dir := testutil.GoProjectDir(t)
	g, err := Load(context.Background(), dir, "./documentation/...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	const pkg = graph.SymbolRef("example.com/shop/documentation/service")
	want := map[graph.SymbolRef]struct {
		kind          graph.NodeKind
		parent        graph.SymbolRef
		documentation string
	}{
		pkg: {
			kind: graph.NodePackage,
			// a_doc.go and b_doc.go contain identical docs. The duplicate is
			// removed, then service.go's distinct block follows in file order.
			documentation: "Package service provides documented business operations.\n\n" +
				"Package service coordinates application workflows.",
		},
		pkg + "::Service": {
			kind:   graph.NodeStruct,
			parent: pkg,
			documentation: "Service coordinates order creation.\n\n" +
				"It validates requests before persistence.",
		},
		pkg + "::Repository": {
			kind:          graph.NodeInterface,
			parent:        pkg,
			documentation: "Repository defines persistence behavior.",
		},
		pkg + "::Repository::Save": {
			kind:          graph.NodeFunction,
			parent:        pkg + "::Repository",
			documentation: "Save persists an entity.",
		},
		pkg + "::Worker": {
			kind:          graph.NodeStruct,
			parent:        pkg,
			documentation: "Worker performs background work.",
		},
		pkg + "::Store": {
			kind:          graph.NodeInterface,
			parent:        pkg,
			documentation: "Store defines grouped persistence behavior.",
		},
		pkg + "::Store::Put": {
			kind:          graph.NodeFunction,
			parent:        pkg + "::Store",
			documentation: "Put stores a value.",
		},
		pkg + "::UndocumentedGrouped": {
			kind:   graph.NodeStruct,
			parent: pkg,
		},
		pkg + "::Run": {
			kind:          graph.NodeFunction,
			parent:        pkg,
			documentation: "Run starts the application.",
		},
		pkg + "::Service::Create": {
			kind:          graph.NodeFunction,
			parent:        pkg + "::Service",
			documentation: "Create persists a new order.",
		},
		pkg + "::helper": {
			kind:   graph.NodeFunction,
			parent: pkg,
		},
		pkg + "::PostgresRepository": {
			kind:          graph.NodeStruct,
			parent:        pkg,
			documentation: "PostgresRepository persists entities.",
		},
		pkg + "::PostgresRepository::Save": {
			kind:   graph.NodeFunction,
			parent: pkg + "::PostgresRepository",
		},
	}

	if got := len(g.Nodes()); got != len(want) {
		t.Fatalf("node count = %d, want %d: %#v", got, len(want), g.Nodes())
	}
	for id, expected := range want {
		node, exists := nodeByRef(g, id)
		if !exists {
			t.Errorf("missing node %q", id)
			continue
		}
		actualParent := parentRef(g, node)
		if node.Kind != expected.kind || actualParent != expected.parent || node.Documentation != expected.documentation {
			t.Errorf("node %q = kind %s, parent %q, documentation %q; want kind %s, parent %q, documentation %q",
				id, node.Kind, actualParent, node.Documentation,
				expected.kind, expected.parent, expected.documentation)
		}
		if node.Kind == graph.NodePackage {
			if node.Location != (graph.Location{}) {
				t.Errorf("documented package %q has declaration location %#v", id, node.Location)
			}
		} else if node.Location.File == "" || node.Location.Line < 1 || node.Location.Column < 1 {
			t.Errorf("documented declaration %q has invalid location %#v", id, node.Location)
		}
	}

	assertCall(t, g, pkg+"::Run", pkg+"::Service::Create", 1)
	assertDocumentationEdge(t, g, pkg+"::PostgresRepository", pkg+"::Repository", graph.EdgeImplements)
	assertDocumentationEdge(t, g, pkg+"::PostgresRepository::Save", pkg+"::Repository::Save", graph.EdgeImplements)

	again, err := Load(context.Background(), dir, "./documentation/...")
	if err != nil {
		t.Fatalf("second Load(): %v", err)
	}
	if !reflect.DeepEqual(g.Nodes(), again.Nodes()) {
		t.Fatalf("documented nodes are not deterministic:\nfirst:  %#v\nsecond: %#v", g.Nodes(), again.Nodes())
	}
	if !reflect.DeepEqual(documentationEdgeSnapshot(g), documentationEdgeSnapshot(again)) {
		t.Fatalf("semantic edges are not deterministic after adding documentation")
	}
}

func assertDocumentationEdge(t *testing.T, g *graph.Graph, from, to graph.SymbolRef, kind graph.EdgeKind) {
	t.Helper()
	for _, edge := range outgoingByRef(g, from, kind) {
		if edge.To == to {
			return
		}
	}
	t.Errorf("missing %s edge %s -> %s", kind, from, to)
}

func documentationEdgeSnapshot(g *graph.Graph) []graph.Edge {
	var edges []graph.Edge
	for _, node := range g.Nodes() {
		for _, edge := range g.Outgoing(node.ID) {
			edges = append(edges, *edge)
		}
	}
	return edges
}
