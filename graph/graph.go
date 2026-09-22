// Package graph contains NOCV's language-independent structural model.
package graph

import (
	"errors"
	"fmt"
	"slices"
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

// EdgeKind identifies a semantic relationship between two nodes.
type EdgeKind uint8

const (
	EdgeCalls EdgeKind = iota
	EdgeImplements
)

// String returns the human-readable name of an edge kind.
func (k EdgeKind) String() string {
	switch k {
	case EdgeCalls:
		return "calls"
	default:
		return fmt.Sprintf("EdgeKind(%d)", k)
	}
}

// Edge is one language-independent semantic relationship. Evidence records
// every source location that established the relationship.
type Edge struct {
	From     SymbolID
	To       SymbolID
	Kind     EdgeKind
	Evidence []Location
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
	outgoing map[SymbolID][]*Edge
	incoming map[SymbolID][]*Edge
	edges    map[edgeKey]*Edge
}

type edgeKey struct {
	from SymbolID
	to   SymbolID
	kind EdgeKind
}

// New creates an empty graph.
func New() *Graph {
	return &Graph{
		nodes:    make(map[SymbolID]*Node),
		children: make(map[SymbolID][]SymbolID),
		outgoing: make(map[SymbolID][]*Edge),
		incoming: make(map[SymbolID][]*Edge),
		edges:    make(map[edgeKey]*Edge),
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

// AncestorOfKind returns the nearest node at or above id with one of the
// requested kinds. A node is considered its own ancestor for this operation.
func (g *Graph) AncestorOfKind(id SymbolID, kinds ...NodeKind) (SymbolID, bool) {
	requested := make(map[NodeKind]bool, len(kinds))
	for _, kind := range kinds {
		requested[kind] = true
	}
	current, ok := g.nodes[id]
	for ok {
		if requested[current.Kind] {
			return current.ID, true
		}
		if current.Parent == "" {
			break
		}
		current, ok = g.nodes[current.Parent]
	}
	return "", false
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

// AddEdge adds a semantic relationship. Repeated relationships are merged and
// contribute additional unique evidence locations to the existing edge.
func (g *Graph) AddEdge(edge Edge) error {
	from, fromExists := g.nodes[edge.From]
	if !fromExists {
		return fmt.Errorf("edge source %q does not exist", edge.From)
	}
	to, toExists := g.nodes[edge.To]
	if !toExists {
		return fmt.Errorf("edge target %q does not exist", edge.To)
	}
	switch edge.Kind {
	case EdgeCalls:
		if from.Kind != NodeFunction || to.Kind != NodeFunction {
			return fmt.Errorf(
				"calls edge %q -> %q must connect functions",
				edge.From,
				edge.To,
			)
		}

	case EdgeImplements:
		validTypeImplementation :=
			from.Kind == NodeStruct &&
				to.Kind == NodeInterface

		validMethodImplementation :=
			from.Kind == NodeFunction &&
				to.Kind == NodeFunction

		if !validTypeImplementation && !validMethodImplementation {
			return fmt.Errorf(
				"implements edge %q -> %q must connect struct -> interface or function -> function",
				edge.From,
				edge.To,
			)
		}

	default:
		return fmt.Errorf("unsupported edge kind %d", edge.Kind)
	}

	if len(edge.Evidence) == 0 {
		return fmt.Errorf(
			"%s edge %q -> %q has no evidence",
			edge.Kind,
			edge.From,
			edge.To,
		)
	}

	key := edgeKey{from: edge.From, to: edge.To, kind: edge.Kind}
	if existing, ok := g.edges[key]; ok {
		appendUniqueEvidence(existing, edge.Evidence)
		return nil
	}

	copy := edge
	copy.Evidence = nil
	appendUniqueEvidence(&copy, edge.Evidence)
	g.edges[key] = &copy
	g.outgoing[edge.From] = append(g.outgoing[edge.From], &copy)
	g.incoming[edge.To] = append(g.incoming[edge.To], &copy)
	return nil
}

// Outgoing returns semantic edges originating at id. If kinds are supplied,
// only matching edge kinds are returned.
func (g *Graph) Outgoing(id SymbolID, kinds ...EdgeKind) []*Edge {
	return copyEdges(g.outgoing[id], kinds)
}

// Incoming returns semantic edges targeting id. If kinds are supplied, only
// matching edge kinds are returned.
func (g *Graph) Incoming(id SymbolID, kinds ...EdgeKind) []*Edge {
	return copyEdges(g.incoming[id], kinds)
}

func appendUniqueEvidence(edge *Edge, evidence []Location) {
	for _, candidate := range evidence {
		duplicate := slices.Contains(edge.Evidence, candidate)
		if !duplicate {
			edge.Evidence = append(edge.Evidence, candidate)
		}
	}
}

func copyEdges(edges []*Edge, kinds []EdgeKind) []*Edge {
	requested := make(map[EdgeKind]bool, len(kinds))
	for _, kind := range kinds {
		requested[kind] = true
	}

	result := make([]*Edge, 0, len(edges))
	for _, edge := range edges {
		if len(requested) > 0 && !requested[edge.Kind] {
			continue
		}
		copy := *edge
		copy.Evidence = append([]Location(nil), edge.Evidence...)
		result = append(result, &copy)
	}
	return result
}
