package main

import (
	"context"
	"fmt"
	"os"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/query"
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

	fmt.Println("\nCalls:")
	printed := false
	for _, node := range g.Nodes() {
		if node.Kind != graph.NodeFunction {
			continue
		}
		edges := g.Outgoing(node.ID, graph.EdgeCalls)
		if len(edges) == 0 {
			continue
		}
		printed = true
		fmt.Println(node.ID)
		for _, edge := range edges {
			fmt.Printf("  -> %s (%d call site(s))\n", edge.To, len(edge.Evidence))
		}
	}
	if !printed {
		fmt.Println("  (none)")
	}

	fmt.Println("\nPackage call dependencies:")
	dependencies := query.Dependencies(g, graph.NodePackage)
	if len(dependencies) == 0 {
		fmt.Println("  (none)")
	}
	var previous graph.SymbolID
	for _, dependency := range dependencies {
		if dependency.From != previous {
			if previous != "" {
				fmt.Println()
			}
			fmt.Println(dependency.From)
			previous = dependency.From
		}
		fmt.Printf("  -> %s\n", dependency.To)
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
