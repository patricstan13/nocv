package graph

import "testing"

func TestIDsAndHierarchy(t *testing.T) {
	g := New()
	pkgID := PackageID("example.com/shop/orders")
	serviceID := ChildID(pkgID, "Service")
	createID := ChildID(serviceID, "Create")

	for _, node := range []Node{
		{ID: pkgID, Kind: NodePackage, Name: "orders"},
		{ID: serviceID, Kind: NodeStruct, Name: "Service", Parent: pkgID},
		{ID: createID, Kind: NodeFunction, Name: "Create", Parent: serviceID},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	if got, want := PackageID("example.com/shop/orders"), pkgID; got != want {
		t.Fatalf("PackageID() = %q, want %q", got, want)
	}
	if got, want := ChildID(ChildID(pkgID, "Service"), "Create"), createID; got != want {
		t.Fatalf("nested ChildID() = %q, want %q", got, want)
	}
	if got := g.Children(pkgID); len(got) != 1 || got[0] != serviceID {
		t.Fatalf("package children = %v, want [%s]", got, serviceID)
	}
	if got := g.Children(serviceID); len(got) != 1 || got[0] != createID {
		t.Fatalf("struct children = %v, want [%s]", got, createID)
	}
	node, ok := g.Node(createID)
	if !ok || node.Parent != serviceID {
		t.Fatalf("method node = %#v, %v; want parent %q", node, ok, serviceID)
	}
}

func TestAddNodeRejectsMissingParent(t *testing.T) {
	g := New()
	err := g.AddNode(Node{ID: "missing::Child", Kind: NodeStruct, Name: "Child", Parent: "missing"})
	if err == nil {
		t.Fatal("AddNode() accepted a node whose parent does not exist")
	}
}

func TestCallsEdgesAggregateEvidenceAndSupportBothDirections(t *testing.T) {
	g := New()
	pkgID := PackageID("example.com/project")
	callerID := ChildID(pkgID, "Caller")
	calleeID := ChildID(pkgID, "Callee")
	for _, node := range []Node{
		{ID: pkgID, Kind: NodePackage, Name: "project"},
		{ID: callerID, Kind: NodeFunction, Name: "Caller", Parent: pkgID},
		{ID: calleeID, Kind: NodeFunction, Name: "Callee", Parent: pkgID},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	first := Location{File: "project.go", Offset: 20}
	second := Location{File: "project.go", Offset: 40}
	if err := g.AddEdge(Edge{From: callerID, To: calleeID, Kind: EdgeCalls, Evidence: []Location{first}}); err != nil {
		t.Fatalf("first AddEdge(): %v", err)
	}
	if err := g.AddEdge(Edge{From: callerID, To: calleeID, Kind: EdgeCalls, Evidence: []Location{first, second}}); err != nil {
		t.Fatalf("second AddEdge(): %v", err)
	}

	outgoing := g.Outgoing(callerID, EdgeCalls)
	if len(outgoing) != 1 {
		t.Fatalf("Outgoing() returned %d edges, want 1", len(outgoing))
	}
	if got := outgoing[0].Evidence; len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("outgoing evidence = %#v, want %#v", got, []Location{first, second})
	}
	incoming := g.Incoming(calleeID, EdgeCalls)
	if len(incoming) != 1 || incoming[0].From != callerID {
		t.Fatalf("Incoming() = %#v, want one edge from %q", incoming, callerID)
	}

	// Results are defensive copies of both the edge and its evidence.
	outgoing[0].Evidence[0].Offset = 999
	if got := g.Outgoing(callerID)[0].Evidence[0]; got != first {
		t.Fatalf("mutating an Outgoing result changed graph evidence to %#v", got)
	}
}

func TestCallsEdgeRequiresExistingFunctionNodesAndEvidence(t *testing.T) {
	g := New()
	pkgID := PackageID("example.com/project")
	fnID := ChildID(pkgID, "Function")
	if err := g.AddNode(Node{ID: pkgID, Kind: NodePackage, Name: "project"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddNode(Node{ID: fnID, Kind: NodeFunction, Name: "Function", Parent: pkgID}); err != nil {
		t.Fatal(err)
	}

	for _, edge := range []Edge{
		{From: "missing", To: fnID, Kind: EdgeCalls, Evidence: []Location{{File: "project.go"}}},
		{From: fnID, To: "missing", Kind: EdgeCalls, Evidence: []Location{{File: "project.go"}}},
		{From: pkgID, To: fnID, Kind: EdgeCalls, Evidence: []Location{{File: "project.go"}}},
		{From: fnID, To: fnID, Kind: EdgeCalls},
	} {
		if err := g.AddEdge(edge); err == nil {
			t.Errorf("AddEdge(%#v) succeeded, want an error", edge)
		}
	}
}
