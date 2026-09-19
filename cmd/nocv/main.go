package main

import (
	"context"
	"fmt"
	"os"

	"nocv/goanalyzer"
	"nocv/graph"
)

func main() {
	patterns := os.Args[1:]
	if len(patterns) == 0 {
		patterns = []string{"."}
	}

	g, err := goanalyzer.Load(context.Background(), "", patterns...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nocv:", err)
		os.Exit(1)
	}

	for _, node := range g.Nodes() {
		if node.Kind != graph.NodePackage {
			continue
		}
		fmt.Printf("%s [package] (%s)\n", node.Name, node.ID)
		printChildren(g, node.ID, "")
	}
}

func printChildren(g *graph.Graph, parent graph.SymbolID, indent string) {
	children := g.Children(parent)
	for i, id := range children {
		node, _ := g.Node(id)
		branch := "├── "
		nextIndent := indent + "│   "
		if i == len(children)-1 {
			branch = "└── "
			nextIndent = indent + "    "
		}
		fmt.Printf("%s%s%s [%s]\n", indent, branch, node.Name, node.Kind)
		printChildren(g, node.ID, nextIndent)
	}
}
