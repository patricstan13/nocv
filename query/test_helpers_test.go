package query_test

import (
	"fmt"

	"nocv/graph"
)

type testNode struct {
	ID            graph.SymbolRef
	Kind          graph.NodeKind
	Name          string
	Parent        graph.SymbolRef
	Location      graph.Location
	Documentation string
}

type testEdge struct {
	From     graph.SymbolRef
	To       graph.SymbolRef
	Kind     graph.EdgeKind
	Evidence []graph.Location
}

func addTestNode(g *graph.Graph, node testNode) error {
	var parent graph.NodeID
	if node.Parent != "" {
		var exists bool
		parent, exists = g.Resolve(node.Parent)
		if !exists {
			return fmt.Errorf("parent %q does not exist", node.Parent)
		}
	}
	_, err := g.AddNode(graph.Node{
		Ref:           node.ID,
		Kind:          node.Kind,
		Name:          node.Name,
		Parent:        parent,
		Location:      node.Location,
		Documentation: node.Documentation,
	})
	return err
}

func addTestEdge(g *graph.Graph, edge testEdge) error {
	from, fromExists := g.Resolve(edge.From)
	to, toExists := g.Resolve(edge.To)
	if !fromExists || !toExists {
		return fmt.Errorf("edge endpoint missing: %q -> %q", edge.From, edge.To)
	}
	return g.AddEdge(graph.Edge{
		From: from, To: to, Kind: edge.Kind,
		Evidence: append([]graph.Location(nil), edge.Evidence...),
	})
}

func outgoingSnapshot(g *graph.Graph) map[graph.SymbolRef][]*graph.Edge {
	snapshot := make(map[graph.SymbolRef][]*graph.Edge)
	for _, node := range g.Nodes() {
		if edges := g.Outgoing(node.ID); len(edges) > 0 {
			snapshot[node.Ref] = edges
		}
	}
	return snapshot
}
