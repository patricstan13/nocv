package main

import (
	"fmt"

	"nocv/graph"
)

type fixtureNode struct {
	ID            graph.SymbolRef
	Kind          graph.NodeKind
	Name          string
	Parent        graph.SymbolRef
	Location      graph.Location
	Documentation string
}

type fixtureEdge struct {
	From     graph.SymbolRef
	To       graph.SymbolRef
	Kind     graph.EdgeKind
	Evidence []graph.Location
}

func addFixtureNode(g *graph.Graph, node fixtureNode) error {
	var parent graph.NodeID
	if node.Parent != "" {
		var exists bool
		parent, exists = g.Resolve(node.Parent)
		if !exists {
			return fmt.Errorf("parent %q does not exist", node.Parent)
		}
	}
	_, err := g.AddNode(graph.Node{
		Ref: node.ID, Kind: node.Kind, Name: node.Name, Parent: parent,
		Location: node.Location, Documentation: node.Documentation,
	})
	return err
}

func addFixtureEdge(g *graph.Graph, edge fixtureEdge) error {
	from, fromExists := g.Resolve(edge.From)
	to, toExists := g.Resolve(edge.To)
	if !fromExists || !toExists {
		return fmt.Errorf("edge endpoint missing: %q -> %q", edge.From, edge.To)
	}
	return g.AddEdge(graph.Edge{From: from, To: to, Kind: edge.Kind, Evidence: edge.Evidence})
}
