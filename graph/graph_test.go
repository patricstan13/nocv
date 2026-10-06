package graph

import (
	"reflect"
	"testing"
)

func TestNodeIdentityLookupAndHierarchy(t *testing.T) {
	g := New()
	pkgRef := PackageRef("example.com/shop/orders")
	pkg := addTestNode(t, g, Node{Ref: pkgRef, Kind: NodePackage, Name: "orders"})
	serviceRef := ChildRef(pkgRef, "Service")
	service := addTestNode(t, g, Node{Ref: serviceRef, Kind: NodeStruct, Name: "Service", Parent: pkg, Documentation: "Service coordinates orders."})
	createRef := ChildRef(serviceRef, "Create")
	create := addTestNode(t, g, Node{Ref: createRef, Kind: NodeFunction, Name: "Create", Parent: service})

	if pkg != 1 || service != 2 || create != 3 {
		t.Fatalf("allocated IDs = %d, %d, %d; want monotonic IDs 1, 2, 3", pkg, service, create)
	}
	if pkg == 0 || service == 0 || create == 0 || pkg == service || service == create || pkg == create {
		t.Fatalf("allocated IDs = %d, %d, %d; want distinct non-zero IDs", pkg, service, create)
	}
	if got, want := g.Children(pkg), []NodeID{service}; !reflect.DeepEqual(got, want) {
		t.Fatalf("package children = %v, want %v", got, want)
	}
	if got, want := g.Children(service), []NodeID{create}; !reflect.DeepEqual(got, want) {
		t.Fatalf("struct children = %v, want %v", got, want)
	}
	byID, ok := g.Node(create)
	if !ok || byID.Ref != createRef || byID.Parent != service {
		t.Fatalf("Node(%d) = %#v, %v", create, byID, ok)
	}
	byRef, ok := g.NodeByRef(serviceRef)
	if !ok || byRef.ID != service {
		t.Fatalf("NodeByRef(%q) = %#v, %v", serviceRef, byRef, ok)
	}
	if resolved, ok := g.Resolve(createRef); !ok || resolved != create {
		t.Fatalf("Resolve(%q) = %d, %v", createRef, resolved, ok)
	}

	byRef.Documentation = "mutated"
	byRef, _ = g.Node(service)
	if byRef.Documentation != "Service coordinates orders." {
		t.Fatalf("mutating lookup result changed graph node: %#v", byRef)
	}
	nodes := g.Nodes()
	if got := []SymbolRef{nodes[0].Ref, nodes[1].Ref, nodes[2].Ref}; !reflect.DeepEqual(got, []SymbolRef{pkgRef, serviceRef, createRef}) {
		t.Fatalf("Nodes refs = %v", got)
	}
	nodes[1].Name = "mutated"
	unchanged, _ := g.Node(nodes[1].ID)
	if unchanged.Name == "mutated" {
		t.Fatal("mutating Nodes result changed graph node")
	}
}

func TestAddNodeRejectsInvalidIdentityAndParent(t *testing.T) {
	g := New()
	pkg := addTestNode(t, g, Node{Ref: "example.com/project", Kind: NodePackage, Name: "project"})
	tests := []Node{
		{Ref: "", Kind: NodePackage, Name: "empty-ref"},
		{ID: 99, Ref: "preassigned", Kind: NodePackage, Name: "preassigned"},
		{Ref: "empty-name", Kind: NodePackage},
		{Ref: "example.com/project", Kind: NodePackage, Name: "duplicate"},
		{Ref: "bad-package", Kind: NodePackage, Name: "bad", Parent: pkg},
		{Ref: "missing-parent", Kind: NodeStruct, Name: "child"},
		{Ref: "unknown-parent", Kind: NodeStruct, Name: "child", Parent: 999},
	}
	for _, node := range tests {
		if id, err := g.AddNode(node); err == nil || id != 0 {
			t.Errorf("AddNode(%#v) = %d, %v; want zero and error", node, id, err)
		}
	}
}

func TestAncestorOfKindUsesNodeIDs(t *testing.T) {
	g := New()
	pkg := addTestNode(t, g, Node{Ref: "example.com/project", Kind: NodePackage, Name: "project"})
	structure := addTestNode(t, g, Node{Ref: "example.com/project::Service", Kind: NodeStruct, Name: "Service", Parent: pkg})
	method := addTestNode(t, g, Node{Ref: "example.com/project::Service::Create", Kind: NodeFunction, Name: "Create", Parent: structure})

	tests := []struct {
		id   NodeID
		kind NodeKind
		want NodeID
		ok   bool
	}{
		{id: method, kind: NodeStruct, want: structure, ok: true},
		{id: method, kind: NodePackage, want: pkg, ok: true},
		{id: structure, kind: NodeStruct, want: structure, ok: true},
		{id: method, kind: NodeInterface},
		{id: 999, kind: NodePackage},
	}
	for _, test := range tests {
		got, ok := g.AncestorOfKind(test.id, test.kind)
		if got != test.want || ok != test.ok {
			t.Errorf("AncestorOfKind(%d, %s) = %d, %v; want %d, %v", test.id, test.kind, got, ok, test.want, test.ok)
		}
	}
}

func TestDistinctSameNamedNodesHaveIndependentEdges(t *testing.T) {
	g := New()
	pkg := addTestNode(t, g, Node{Ref: "example.com/repeated", Kind: NodePackage, Name: "repeated"})
	init1 := addTestNode(t, g, Node{Ref: "example.com/repeated::init#1", Kind: NodeFunction, Name: "init", Parent: pkg})
	init2 := addTestNode(t, g, Node{Ref: "example.com/repeated::init#2", Kind: NodeFunction, Name: "init", Parent: pkg})
	one := addTestNode(t, g, Node{Ref: "example.com/repeated::one", Kind: NodeFunction, Name: "one", Parent: pkg})
	two := addTestNode(t, g, Node{Ref: "example.com/repeated::two", Kind: NodeFunction, Name: "two", Parent: pkg})

	addTestEdge(t, g, Edge{From: init1, To: one, Kind: EdgeCalls, Certainty: RelationshipConfirmed, Evidence: []Location{{File: "a.go", Offset: 10}}})
	addTestEdge(t, g, Edge{From: init2, To: two, Kind: EdgeCalls, Certainty: RelationshipConfirmed, Evidence: []Location{{File: "b.go", Offset: 20}}})
	if got := g.Outgoing(init1, EdgeCalls); len(got) != 1 || got[0].To != one {
		t.Fatalf("init#1 edges = %#v", got)
	}
	if got := g.Outgoing(init2, EdgeCalls); len(got) != 1 || got[0].To != two {
		t.Fatalf("init#2 edges = %#v", got)
	}
}

func TestEdgesValidateKindsAggregateEvidenceAndCopyResults(t *testing.T) {
	g := New()
	pkg := addTestNode(t, g, Node{Ref: "pkg", Kind: NodePackage, Name: "pkg"})
	otherPkg := addTestNode(t, g, Node{Ref: "other", Kind: NodePackage, Name: "other"})
	structure := addTestNode(t, g, Node{Ref: "pkg::Struct", Kind: NodeStruct, Name: "Struct", Parent: pkg})
	base := addTestNode(t, g, Node{Ref: "pkg::Base", Kind: NodeStruct, Name: "Base", Parent: pkg})
	iface := addTestNode(t, g, Node{Ref: "pkg::Interface", Kind: NodeInterface, Name: "Interface", Parent: pkg})
	childIface := addTestNode(t, g, Node{Ref: "pkg::ChildInterface", Kind: NodeInterface, Name: "ChildInterface", Parent: pkg})
	function := addTestNode(t, g, Node{Ref: "pkg::Function", Kind: NodeFunction, Name: "Function", Parent: pkg})
	callee := addTestNode(t, g, Node{Ref: "pkg::Callee", Kind: NodeFunction, Name: "Callee", Parent: pkg})
	method := addTestNode(t, g, Node{Ref: "pkg::Struct::Run", Kind: NodeFunction, Name: "Run", Parent: structure})
	interfaceMethod := addTestNode(t, g, Node{Ref: "pkg::Interface::Run", Kind: NodeFunction, Name: "Run", Parent: iface})
	evidence := []Location{{File: "one.go", Offset: 10}}
	valid := []Edge{
		{From: function, To: callee, Kind: EdgeCalls, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: structure, To: iface, Kind: EdgeImplements, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: method, To: interfaceMethod, Kind: EdgeImplements, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: structure, To: base, Kind: EdgeEmbeds, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: childIface, To: iface, Kind: EdgeEmbeds, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: function, To: structure, Kind: EdgeAccepts, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: function, To: iface, Kind: EdgeReturns, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: structure, To: base, Kind: EdgeFieldType, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: structure, To: iface, Kind: EdgeFieldType, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: pkg, To: otherPkg, Kind: EdgeImports, Certainty: RelationshipConfirmed, Evidence: evidence},
	}
	for _, edge := range valid {
		addTestEdge(t, g, edge)
	}

	second := Location{File: "two.go", Offset: 20}
	addTestEdge(t, g, Edge{From: function, To: callee, Kind: EdgeCalls, Certainty: RelationshipConfirmed, Evidence: []Location{evidence[0], second}})
	outgoing := g.Outgoing(function, EdgeCalls)
	if len(outgoing) != 1 || !reflect.DeepEqual(outgoing[0].Evidence, []Location{evidence[0], second}) {
		t.Fatalf("aggregated calls = %#v", outgoing)
	}
	outgoing[0].Evidence[0].Offset = 999
	if got := g.Outgoing(function, EdgeCalls)[0].Evidence[0]; got != evidence[0] {
		t.Fatalf("edge result aliases graph evidence: %#v", got)
	}
	if incoming := g.Incoming(callee, EdgeCalls); len(incoming) != 1 || incoming[0].From != function {
		t.Fatalf("incoming calls = %#v", incoming)
	}

	invalid := []Edge{
		{From: 999, To: callee, Kind: EdgeCalls, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: function, To: 999, Kind: EdgeCalls, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: pkg, To: function, Kind: EdgeCalls, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: function, To: callee, Kind: EdgeCalls, Certainty: RelationshipConfirmed},
		{From: iface, To: structure, Kind: EdgeImplements, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: structure, To: iface, Kind: EdgeEmbeds, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: structure, To: iface, Kind: EdgeAccepts, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: function, To: callee, Kind: EdgeReturns, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: function, To: structure, Kind: EdgeFieldType, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: iface, To: structure, Kind: EdgeFieldType, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: pkg, To: structure, Kind: EdgeFieldType, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: structure, To: function, Kind: EdgeFieldType, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: function, To: otherPkg, Kind: EdgeImports, Certainty: RelationshipConfirmed, Evidence: evidence},
		{From: pkg, To: otherPkg, Kind: EdgeKind(99), Certainty: RelationshipConfirmed, Evidence: evidence},
	}
	for _, edge := range invalid {
		if err := g.AddEdge(edge); err == nil {
			t.Errorf("AddEdge(%#v) succeeded", edge)
		}
	}
}

func TestEdgeCertaintyValidationMergeAndCopies(t *testing.T) {
	if RelationshipConfirmed.String() != "confirmed" || RelationshipUncertain.String() != "uncertain" {
		t.Fatalf("certainty strings = %q, %q", RelationshipConfirmed, RelationshipUncertain)
	}

	newCallGraph := func(t *testing.T) (*Graph, NodeID, NodeID) {
		t.Helper()
		g := New()
		pkg := addTestNode(t, g, Node{Ref: "pkg", Kind: NodePackage, Name: "pkg"})
		from := addTestNode(t, g, Node{Ref: "pkg::From", Kind: NodeFunction, Name: "From", Parent: pkg})
		to := addTestNode(t, g, Node{Ref: "pkg::To", Kind: NodeFunction, Name: "To", Parent: pkg})
		return g, from, to
	}

	t.Run("unknown rejected", func(t *testing.T) {
		g, from, to := newCallGraph(t)
		err := g.AddEdge(Edge{
			From: from, To: to, Kind: EdgeCalls,
			Certainty: RelationshipCertaintyUnknown,
			Evidence:  []Location{{File: "unknown.go", Offset: 1}},
		})
		if err == nil {
			t.Fatal("AddEdge with unknown certainty succeeded")
		}
	})

	for _, test := range []struct {
		name  string
		first RelationshipCertainty
		last  RelationshipCertainty
	}{
		{name: "uncertain remains uncertain", first: RelationshipUncertain, last: RelationshipUncertain},
		{name: "uncertain upgrades to confirmed", first: RelationshipUncertain, last: RelationshipConfirmed},
		{name: "confirmed does not downgrade", first: RelationshipConfirmed, last: RelationshipUncertain},
		{name: "confirmed remains confirmed", first: RelationshipConfirmed, last: RelationshipConfirmed},
	} {
		t.Run(test.name, func(t *testing.T) {
			g, from, to := newCallGraph(t)
			firstEvidence := Location{File: "first.go", Offset: 1}
			secondEvidence := Location{File: "second.go", Offset: 2}
			addTestEdge(t, g, Edge{
				From: from, To: to, Kind: EdgeCalls, Certainty: test.first,
				Evidence: []Location{firstEvidence},
			})
			addTestEdge(t, g, Edge{
				From: from, To: to, Kind: EdgeCalls, Certainty: test.last,
				Evidence: []Location{firstEvidence, secondEvidence},
			})

			want := RelationshipUncertain
			if test.first == RelationshipConfirmed || test.last == RelationshipConfirmed {
				want = RelationshipConfirmed
			}
			outgoing := g.Outgoing(from, EdgeCalls)
			incoming := g.Incoming(to, EdgeCalls)
			if len(outgoing) != 1 || len(incoming) != 1 {
				t.Fatalf("edge counts = outgoing %d, incoming %d; want one each", len(outgoing), len(incoming))
			}
			if outgoing[0].Certainty != want || incoming[0].Certainty != want {
				t.Fatalf("certainties = %s, %s; want %s", outgoing[0].Certainty, incoming[0].Certainty, want)
			}
			if got := outgoing[0].Evidence; !reflect.DeepEqual(got, []Location{firstEvidence, secondEvidence}) {
				t.Fatalf("evidence = %#v", got)
			}

			outgoing[0].Certainty = RelationshipCertaintyUnknown
			outgoing[0].Evidence[0].Offset = 99
			stored := g.Outgoing(from, EdgeCalls)[0]
			if stored.Certainty != want || stored.Evidence[0] != firstEvidence {
				t.Fatalf("detached copy mutated stored edge: %#v", stored)
			}
		})
	}
}

func addTestNode(t *testing.T, g *Graph, node Node) NodeID {
	t.Helper()
	id, err := g.AddNode(node)
	if err != nil {
		t.Fatalf("AddNode(%q): %v", node.Ref, err)
	}
	return id
}

func addTestEdge(t *testing.T, g *Graph, edge Edge) {
	t.Helper()
	if err := g.AddEdge(edge); err != nil {
		t.Fatalf("AddEdge(%d -> %d, %s): %v", edge.From, edge.To, edge.Kind, err)
	}
}
