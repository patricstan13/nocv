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
