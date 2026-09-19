// Package graph contains NOCV's language-independent structural model.
package graph

import (
	"errors"
	"fmt"
	"sort"
)

// SymbolID is a deterministic identity derived from a declaration's hierarchy.
type SymbolID string

// NodeKind identifies the structural kind of a node.
type NodeKind uint8

const (
	NodePackage NodeKind = iota
	NodeStruct
	NodeInterface
	NodeFunction
)

// String returns the human-readable name of a node kind.
func (k NodeKind) String() string {
	switch k {
	case NodePackage:
		return "package"
	case NodeStruct:
		return "struct"
	case NodeInterface:
		return "interface"
	case NodeFunction:
		return "function"
	default:
		return fmt.Sprintf("NodeKind(%d)", k)
	}
}

// Location identifies the start of a declaration in source.
type Location struct {
	File   string
	Offset int
}

// Node is one declaration in a project's structural hierarchy.
// It deliberately contains no Go compiler or syntax-tree objects.
type Node struct {
	ID       SymbolID
	Kind     NodeKind
	Name     string
	Parent   SymbolID
	Location Location
}

// PackageID constructs a package identity.
func PackageID(importPath string) SymbolID {
	return SymbolID(importPath)
}

// ChildID constructs the identity of a declaration nested under parent.
// ID encoding is centralized here so it can change without affecting analyzers.
func ChildID(parent SymbolID, name string) SymbolID {
	return SymbolID(string(parent) + "::" + name)
}

// Graph stores structural nodes and their parent/child hierarchy.
type Graph struct {
	nodes    map[SymbolID]*Node
	children map[SymbolID][]SymbolID
}

// New creates an empty graph.
func New() *Graph {
	return &Graph{
		nodes:    make(map[SymbolID]*Node),
		children: make(map[SymbolID][]SymbolID),
	}
}

// AddNode adds node after validating its structural parent.
func (g *Graph) AddNode(node Node) error {
	if node.ID == "" {
		return errors.New("node ID is empty")
	}
	if node.Name == "" {
		return fmt.Errorf("node %q has an empty name", node.ID)
	}
	if _, exists := g.nodes[node.ID]; exists {
		return fmt.Errorf("node %q already exists", node.ID)
	}
	if node.Kind == NodePackage {
		if node.Parent != "" {
			return fmt.Errorf("package %q cannot have a parent", node.ID)
		}
	} else {
		if node.Parent == "" {
			return fmt.Errorf("node %q has no parent", node.ID)
		}
		if _, exists := g.nodes[node.Parent]; !exists {
			return fmt.Errorf("parent %q of node %q does not exist", node.Parent, node.ID)
		}
	}

	copy := node
	g.nodes[node.ID] = &copy
	if node.Parent != "" {
		g.children[node.Parent] = append(g.children[node.Parent], node.ID)
	}
	return nil
}

// Node returns a copy of the node with id.
func (g *Graph) Node(id SymbolID) (*Node, bool) {
	node, ok := g.nodes[id]
	if !ok {
		return nil, false
	}
	copy := *node
	return &copy, true
}

// Children returns the child IDs of parent in insertion order.
func (g *Graph) Children(parent SymbolID) []SymbolID {
	return append([]SymbolID(nil), g.children[parent]...)
}

// Nodes returns copies of all nodes, sorted by ID.
func (g *Graph) Nodes() []*Node {
	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	nodes := make([]*Node, 0, len(ids))
	for _, id := range ids {
		node, _ := g.Node(SymbolID(id))
		nodes = append(nodes, node)
	}
	return nodes
}
