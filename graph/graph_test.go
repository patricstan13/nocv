package graph

import "testing"

func TestIDsAndHierarchy(t *testing.T) {
	g := New()
	pkgID := PackageID("example.com/shop/orders")
	serviceID := ChildID(pkgID, "Service")
	createID := ChildID(serviceID, "Create")

	for _, node := range []Node{
		{ID: pkgID, Kind: NodePackage, Name: "orders"},
		{ID: serviceID, Kind: NodeStruct, Name: "Service", Parent: pkgID, Documentation: "Service coordinates orders."},
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
	service, ok := g.Node(serviceID)
	if !ok {
		t.Fatalf("missing service node %q", serviceID)
	}
	if service.Documentation != "Service coordinates orders." {
		t.Fatalf("service documentation = %q", service.Documentation)
	}
	service.Documentation = "mutated"
	service, _ = g.Node(serviceID)
	if service.Documentation != "Service coordinates orders." {
		t.Fatalf("mutating Node result changed documentation to %q", service.Documentation)
	}
	for _, candidate := range g.Nodes() {
		if candidate.ID == serviceID && candidate.Documentation != "Service coordinates orders." {
			t.Fatalf("Nodes result lost documentation: %#v", candidate)
		}
	}
}

func TestAddNodeRejectsMissingParent(t *testing.T) {
	g := New()
	err := g.AddNode(Node{ID: "missing::Child", Kind: NodeStruct, Name: "Child", Parent: "missing"})
	if err == nil {
		t.Fatal("AddNode() accepted a node whose parent does not exist")
	}
}

func TestAncestorOfKind(t *testing.T) {
	g := New()
	pkgID := PackageID("example.com/project")
	structID := ChildID(pkgID, "Service")
	methodID := ChildID(structID, "Create")
	for _, node := range []Node{
		{ID: pkgID, Kind: NodePackage, Name: "project"},
		{ID: structID, Kind: NodeStruct, Name: "Service", Parent: pkgID},
		{ID: methodID, Kind: NodeFunction, Name: "Create", Parent: structID},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	for _, test := range []struct {
		name string
		id   SymbolID
		kind NodeKind
		want SymbolID
		ok   bool
	}{
		{name: "nearest struct", id: methodID, kind: NodeStruct, want: structID, ok: true},
		{name: "package", id: methodID, kind: NodePackage, want: pkgID, ok: true},
		{name: "self", id: structID, kind: NodeStruct, want: structID, ok: true},
		{name: "missing kind", id: methodID, kind: NodeInterface},
		{name: "missing node", id: "missing", kind: NodePackage},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := g.AncestorOfKind(test.id, test.kind)
			if got != test.want || ok != test.ok {
				t.Fatalf("AncestorOfKind(%q, %s) = (%q, %v), want (%q, %v)", test.id, test.kind, got, ok, test.want, test.ok)
			}
		})
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

func TestImplementsEdgesAcceptTypesAndMethods(t *testing.T) {
	g := New()
	pkgID := PackageID("example.com/project")
	structID := ChildID(pkgID, "Repository")
	interfaceID := ChildID(pkgID, "Store")
	concreteMethodID := ChildID(structID, "Save")
	interfaceMethodID := ChildID(interfaceID, "Save")
	for _, node := range []Node{
		{ID: pkgID, Kind: NodePackage, Name: "project"},
		{ID: structID, Kind: NodeStruct, Name: "Repository", Parent: pkgID},
		{ID: interfaceID, Kind: NodeInterface, Name: "Store", Parent: pkgID},
		{ID: concreteMethodID, Kind: NodeFunction, Name: "Save", Parent: structID},
		{ID: interfaceMethodID, Kind: NodeFunction, Name: "Save", Parent: interfaceID},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	evidence := []Location{{File: "project.go", Offset: 10}}
	for _, edge := range []Edge{
		{From: structID, To: interfaceID, Kind: EdgeImplements, Evidence: evidence},
		{From: concreteMethodID, To: interfaceMethodID, Kind: EdgeImplements, Evidence: evidence},
	} {
		if err := g.AddEdge(edge); err != nil {
			t.Fatalf("AddEdge(%q -> %q): %v", edge.From, edge.To, err)
		}
	}

	if got := g.Outgoing(structID, EdgeImplements); len(got) != 1 || got[0].To != interfaceID {
		t.Fatalf("type implementation edges = %#v", got)
	}
	if got := g.Incoming(interfaceMethodID, EdgeImplements); len(got) != 1 || got[0].From != concreteMethodID {
		t.Fatalf("method implementation edges = %#v", got)
	}
}

func TestImplementsEdgeRejectsInvalidEndpointKinds(t *testing.T) {
	g := New()
	pkgID := PackageID("example.com/project")
	structID := ChildID(pkgID, "Repository")
	interfaceID := ChildID(pkgID, "Store")
	functionID := ChildID(pkgID, "Save")
	for _, node := range []Node{
		{ID: pkgID, Kind: NodePackage, Name: "project"},
		{ID: structID, Kind: NodeStruct, Name: "Repository", Parent: pkgID},
		{ID: interfaceID, Kind: NodeInterface, Name: "Store", Parent: pkgID},
		{ID: functionID, Kind: NodeFunction, Name: "Save", Parent: pkgID},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	evidence := []Location{{File: "project.go"}}
	for _, edge := range []Edge{
		{From: structID, To: structID, Kind: EdgeImplements, Evidence: evidence},
		{From: interfaceID, To: structID, Kind: EdgeImplements, Evidence: evidence},
		{From: structID, To: functionID, Kind: EdgeImplements, Evidence: evidence},
		{From: functionID, To: interfaceID, Kind: EdgeImplements, Evidence: evidence},
	} {
		if err := g.AddEdge(edge); err == nil {
			t.Errorf("AddEdge(%#v) succeeded, want an error", edge)
		}
	}
}

func TestEmbedsEdgesAcceptMatchingTypeKinds(t *testing.T) {
	g := New()
	pkgID := PackageID("example.com/project")
	baseID := ChildID(pkgID, "Base")
	childID := ChildID(pkgID, "Child")
	readerID := ChildID(pkgID, "Reader")
	readWriterID := ChildID(pkgID, "ReadWriter")
	for _, node := range []Node{
		{ID: pkgID, Kind: NodePackage, Name: "project"},
		{ID: baseID, Kind: NodeStruct, Name: "Base", Parent: pkgID},
		{ID: childID, Kind: NodeStruct, Name: "Child", Parent: pkgID},
		{ID: readerID, Kind: NodeInterface, Name: "Reader", Parent: pkgID},
		{ID: readWriterID, Kind: NodeInterface, Name: "ReadWriter", Parent: pkgID},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	evidence := []Location{{File: "project.go", Offset: 10}}
	for _, edge := range []Edge{
		{From: childID, To: baseID, Kind: EdgeEmbeds, Evidence: evidence},
		{From: readWriterID, To: readerID, Kind: EdgeEmbeds, Evidence: evidence},
	} {
		if err := g.AddEdge(edge); err != nil {
			t.Fatalf("AddEdge(%q -> %q): %v", edge.From, edge.To, err)
		}
	}

	if got := g.Outgoing(childID, EdgeEmbeds); len(got) != 1 || got[0].To != baseID {
		t.Fatalf("struct embedding edges = %#v", got)
	}
	if got := g.Incoming(readerID, EdgeEmbeds); len(got) != 1 || got[0].From != readWriterID {
		t.Fatalf("interface embedding edges = %#v", got)
	}
}

func TestEmbedsEdgeRejectsMismatchedEndpointKinds(t *testing.T) {
	g := New()
	pkgID := PackageID("example.com/project")
	structID := ChildID(pkgID, "Struct")
	interfaceID := ChildID(pkgID, "Interface")
	functionID := ChildID(pkgID, "Function")
	for _, node := range []Node{
		{ID: pkgID, Kind: NodePackage, Name: "project"},
		{ID: structID, Kind: NodeStruct, Name: "Struct", Parent: pkgID},
		{ID: interfaceID, Kind: NodeInterface, Name: "Interface", Parent: pkgID},
		{ID: functionID, Kind: NodeFunction, Name: "Function", Parent: pkgID},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	evidence := []Location{{File: "project.go"}}
	for _, edge := range []Edge{
		{From: structID, To: interfaceID, Kind: EdgeEmbeds, Evidence: evidence},
		{From: interfaceID, To: structID, Kind: EdgeEmbeds, Evidence: evidence},
		{From: functionID, To: functionID, Kind: EdgeEmbeds, Evidence: evidence},
		{From: pkgID, To: pkgID, Kind: EdgeEmbeds, Evidence: evidence},
	} {
		if err := g.AddEdge(edge); err == nil {
			t.Errorf("AddEdge(%#v) succeeded, want an error", edge)
		}
	}
}

func TestSignatureEdgesAcceptStructsAndInterfaces(t *testing.T) {
	g := New()
	pkgID := PackageID("example.com/project")
	functionID := ChildID(pkgID, "Handle")
	structID := ChildID(pkgID, "Order")
	interfaceID := ChildID(pkgID, "Repository")
	for _, node := range []Node{
		{ID: pkgID, Kind: NodePackage, Name: "project"},
		{ID: functionID, Kind: NodeFunction, Name: "Handle", Parent: pkgID},
		{ID: structID, Kind: NodeStruct, Name: "Order", Parent: pkgID},
		{ID: interfaceID, Kind: NodeInterface, Name: "Repository", Parent: pkgID},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	evidence := []Location{{File: "project.go", Offset: 10}}
	for _, edge := range []Edge{
		{From: functionID, To: structID, Kind: EdgeAccepts, Evidence: evidence},
		{From: functionID, To: interfaceID, Kind: EdgeAccepts, Evidence: evidence},
		{From: functionID, To: structID, Kind: EdgeReturns, Evidence: evidence},
		{From: functionID, To: interfaceID, Kind: EdgeReturns, Evidence: evidence},
	} {
		if err := g.AddEdge(edge); err != nil {
			t.Fatalf("AddEdge(%s, %q -> %q): %v", edge.Kind, edge.From, edge.To, err)
		}
	}

	if got := g.Outgoing(functionID, EdgeAccepts); len(got) != 2 {
		t.Fatalf("accepts edges = %#v, want two", got)
	}
	if got := g.Incoming(interfaceID, EdgeReturns); len(got) != 1 || got[0].From != functionID {
		t.Fatalf("interface return edges = %#v, want one from %q", got, functionID)
	}
}

func TestSignatureEdgesRejectInvalidEndpointKinds(t *testing.T) {
	g := New()
	pkgID := PackageID("example.com/project")
	functionID := ChildID(pkgID, "Handle")
	structID := ChildID(pkgID, "Order")
	interfaceID := ChildID(pkgID, "Repository")
	for _, node := range []Node{
		{ID: pkgID, Kind: NodePackage, Name: "project"},
		{ID: functionID, Kind: NodeFunction, Name: "Handle", Parent: pkgID},
		{ID: structID, Kind: NodeStruct, Name: "Order", Parent: pkgID},
		{ID: interfaceID, Kind: NodeInterface, Name: "Repository", Parent: pkgID},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	evidence := []Location{{File: "project.go"}}
	for _, kind := range []EdgeKind{EdgeAccepts, EdgeReturns} {
		for _, edge := range []Edge{
			{From: pkgID, To: structID, Kind: kind, Evidence: evidence},
			{From: structID, To: interfaceID, Kind: kind, Evidence: evidence},
			{From: functionID, To: functionID, Kind: kind, Evidence: evidence},
			{From: functionID, To: pkgID, Kind: kind, Evidence: evidence},
		} {
			if err := g.AddEdge(edge); err == nil {
				t.Errorf("AddEdge(%#v) succeeded, want an error", edge)
			}
		}
	}
}

func TestImportsEdgesRequirePackagesAndAggregateEvidence(t *testing.T) {
	g := New()
	from := PackageID("example.com/project/app")
	to := PackageID("example.com/project/service")
	function := ChildID(from, "Run")
	structure := ChildID(to, "Service")
	for _, node := range []Node{
		{ID: from, Kind: NodePackage, Name: "app"},
		{ID: to, Kind: NodePackage, Name: "service"},
		{ID: function, Kind: NodeFunction, Name: "Run", Parent: from},
		{ID: structure, Kind: NodeStruct, Name: "Service", Parent: to},
	} {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}

	first := Location{File: "a.go", Offset: 20}
	second := Location{File: "b.go", Offset: 40}
	if err := g.AddEdge(Edge{From: from, To: to, Kind: EdgeImports, Evidence: []Location{first}}); err != nil {
		t.Fatalf("first AddEdge(): %v", err)
	}
	if err := g.AddEdge(Edge{From: from, To: to, Kind: EdgeImports, Evidence: []Location{first, second}}); err != nil {
		t.Fatalf("second AddEdge(): %v", err)
	}
	if got := g.Outgoing(from, EdgeImports); len(got) != 1 || len(got[0].Evidence) != 2 || got[0].Evidence[0] != first || got[0].Evidence[1] != second {
		t.Fatalf("import edges = %#v, want one edge with two evidence locations", got)
	}

	evidence := []Location{{File: "imports.go"}}
	for _, edge := range []Edge{
		{From: function, To: to, Kind: EdgeImports, Evidence: evidence},
		{From: from, To: function, Kind: EdgeImports, Evidence: evidence},
		{From: structure, To: to, Kind: EdgeImports, Evidence: evidence},
		{From: from, To: structure, Kind: EdgeImports, Evidence: evidence},
	} {
		if err := g.AddEdge(edge); err == nil {
			t.Errorf("AddEdge(%#v) succeeded, want an error", edge)
		}
	}
}
