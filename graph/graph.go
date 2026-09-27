// Package graph contains NOCV's language-independent structural model.
package graph

import (
	"fmt"
	"slices"
	"sort"
)

// NodeID is an opaque identity unique within one graph. Zero means no node.
type NodeID uint64

// SymbolRef is a deterministic, human-readable address for a declaration.
type SymbolRef string

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
	ID            NodeID
	Ref           SymbolRef
	Kind          NodeKind
	Name          string
	Parent        NodeID
	Location      Location
	Documentation string
}

// EdgeKind identifies a relationship between two nodes.
type EdgeKind uint8

const (
	EdgeCalls EdgeKind = iota
	EdgeImplements
	EdgeEmbeds
	EdgeAccepts
	EdgeReturns
	EdgeImports
)

// String returns the human-readable name of an edge kind.
func (k EdgeKind) String() string {
	switch k {
	case EdgeCalls:
		return "calls"
	case EdgeImplements:
		return "implements"
	case EdgeEmbeds:
		return "embeds"
	case EdgeAccepts:
		return "accepts"
	case EdgeReturns:
		return "returns"
	case EdgeImports:
		return "imports"
	default:
		return fmt.Sprintf("EdgeKind(%d)", k)
	}
}

// Edge is one language-independent relationship. Evidence records every
// source location that established the relationship.
type Edge struct {
	From     NodeID
	To       NodeID
	Kind     EdgeKind
	Evidence []Location
}

// PackageRef constructs a package reference.
func PackageRef(importPath string) SymbolRef {
	return SymbolRef(importPath)
}

// ChildRef constructs the reference of a declaration nested under parent.
func ChildRef(parent SymbolRef, name string) SymbolRef {
	return SymbolRef(string(parent) + "::" + name)
}

// Graph stores structural nodes and their parent/child hierarchy.
type Graph struct {
	nextID   NodeID
	nodes    map[NodeID]*Node
	byRef    map[SymbolRef]NodeID
	children map[NodeID][]NodeID
	outgoing map[NodeID][]*Edge
	incoming map[NodeID][]*Edge
	edges    map[edgeKey]*Edge
}

type edgeKey struct {
	from NodeID
	to   NodeID
	kind EdgeKind
}

// New creates an empty graph.
func New() *Graph {
	return &Graph{
		nextID:   1,
		nodes:    make(map[NodeID]*Node),
		byRef:    make(map[SymbolRef]NodeID),
		children: make(map[NodeID][]NodeID),
		outgoing: make(map[NodeID][]*Edge),
		incoming: make(map[NodeID][]*Edge),
		edges:    make(map[edgeKey]*Edge),
	}
}

// AddNode validates and adds node, assigning a new graph-local NodeID.
func (g *Graph) AddNode(node Node) (NodeID, error) {
	if node.ID != 0 {
		return 0, fmt.Errorf("node %q already has graph ID %d", node.Ref, node.ID)
	}
	if node.Ref == "" {
		return 0, fmt.Errorf("node reference is empty")
	}
	if node.Name == "" {
		return 0, fmt.Errorf("node %q has an empty name", node.Ref)
	}
	if _, exists := g.byRef[node.Ref]; exists {
		return 0, fmt.Errorf("node reference %q already exists", node.Ref)
	}
	if node.Kind == NodePackage {
		if node.Parent != 0 {
			return 0, fmt.Errorf("package %q cannot have a parent", node.Ref)
		}
	} else {
		if node.Parent == 0 {
			return 0, fmt.Errorf("node %q has no parent", node.Ref)
		}
		if _, exists := g.nodes[node.Parent]; !exists {
			return 0, fmt.Errorf("parent %d of node %q does not exist", node.Parent, node.Ref)
		}
	}

	id := g.nextID
	if id == 0 {
		return 0, fmt.Errorf("node ID space exhausted")
	}
	g.nextID++
	copy := node
	copy.ID = id
	g.nodes[id] = &copy
	g.byRef[node.Ref] = id
	if node.Parent != 0 {
		g.children[node.Parent] = append(g.children[node.Parent], id)
	}
	return id, nil
}

// Node returns a copy of the node with id.
func (g *Graph) Node(id NodeID) (*Node, bool) {
	node, ok := g.nodes[id]
	if !ok {
		return nil, false
	}
	copy := *node
	return &copy, true
}

// NodeByRef returns a copy of the node with ref.
func (g *Graph) NodeByRef(ref SymbolRef) (*Node, bool) {
	id, ok := g.byRef[ref]
	if !ok {
		return nil, false
	}
	return g.Node(id)
}

// Resolve returns the graph-local identity associated with ref.
func (g *Graph) Resolve(ref SymbolRef) (NodeID, bool) {
	id, ok := g.byRef[ref]
	return id, ok
}

// Children returns the child IDs of parent in insertion order.
func (g *Graph) Children(parent NodeID) []NodeID {
	return append([]NodeID(nil), g.children[parent]...)
}

// AncestorOfKind returns the nearest node at or above id with one of the
// requested kinds. A node is considered its own ancestor for this operation.
func (g *Graph) AncestorOfKind(id NodeID, kinds ...NodeKind) (NodeID, bool) {
	requested := make(map[NodeKind]bool, len(kinds))
	for _, kind := range kinds {
		requested[kind] = true
	}
	current, ok := g.nodes[id]
	for ok {
		if requested[current.Kind] {
			return current.ID, true
		}
		if current.Parent == 0 {
			break
		}
		current, ok = g.nodes[current.Parent]
	}
	return 0, false
}

// Nodes returns copies of all nodes, sorted by SymbolRef.
func (g *Graph) Nodes() []*Node {
	refs := make([]string, 0, len(g.byRef))
	for ref := range g.byRef {
		refs = append(refs, string(ref))
	}
	sort.Strings(refs)

	nodes := make([]*Node, 0, len(refs))
	for _, ref := range refs {
		node, _ := g.NodeByRef(SymbolRef(ref))
		nodes = append(nodes, node)
	}
	return nodes
}

// AddEdge adds a semantic relationship. Repeated relationships are merged and
// contribute additional unique evidence locations to the existing edge.
func (g *Graph) AddEdge(edge Edge) error {
	from, fromExists := g.nodes[edge.From]
	if !fromExists {
		return fmt.Errorf("edge source %d does not exist", edge.From)
	}
	to, toExists := g.nodes[edge.To]
	if !toExists {
		return fmt.Errorf("edge target %d does not exist", edge.To)
	}
	switch edge.Kind {
	case EdgeCalls:
		if from.Kind != NodeFunction || to.Kind != NodeFunction {
			return fmt.Errorf(
				"calls edge %d -> %d must connect functions",
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
				"implements edge %d -> %d must connect struct -> interface or function -> function",
				edge.From,
				edge.To,
			)
		}

	case EdgeEmbeds:
		validStructEmbedding := from.Kind == NodeStruct && to.Kind == NodeStruct
		validInterfaceEmbedding := from.Kind == NodeInterface && to.Kind == NodeInterface
		if !validStructEmbedding && !validInterfaceEmbedding {
			return fmt.Errorf(
				"embeds edge %d -> %d must connect struct -> struct or interface -> interface",
				edge.From,
				edge.To,
			)
		}

	case EdgeAccepts, EdgeReturns:
		validTarget := to.Kind == NodeStruct || to.Kind == NodeInterface
		if from.Kind != NodeFunction || !validTarget {
			return fmt.Errorf(
				"%s edge %d -> %d must connect function -> struct or function -> interface",
				edge.Kind,
				edge.From,
				edge.To,
			)
		}

	case EdgeImports:
		if from.Kind != NodePackage || to.Kind != NodePackage {
			return fmt.Errorf(
				"imports edge %d -> %d must connect packages",
				edge.From,
				edge.To,
			)
		}

	default:
		return fmt.Errorf("unsupported edge kind %d", edge.Kind)
	}

	if len(edge.Evidence) == 0 {
		return fmt.Errorf(
			"%s edge %d -> %d has no evidence",
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

// Outgoing returns edges originating at id. If kinds are supplied, only
// matching edge kinds are returned.
func (g *Graph) Outgoing(id NodeID, kinds ...EdgeKind) []*Edge {
	return copyEdges(g.outgoing[id], kinds)
}

// Incoming returns edges targeting id. If kinds are supplied, only matching
// edge kinds are returned.
func (g *Graph) Incoming(id NodeID, kinds ...EdgeKind) []*Edge {
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
