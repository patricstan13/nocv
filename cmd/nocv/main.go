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

	fmt.Println("\nImplementations:")
	printed = false
	for _, node := range g.Nodes() {
		if node.Kind != graph.NodeInterface {
			continue
		}
		edges := g.Incoming(node.ID, graph.EdgeImplements)
		if len(edges) == 0 {
			continue
		}
		printed = true
		fmt.Println(node.ID)
		for _, edge := range edges {
			fmt.Printf("  <- %s\n", edge.From)
		}
	}
	if !printed {
		fmt.Println("  (none)")
	}

	fmt.Println("\nEmbeddings:")
	printed = false
	for _, node := range g.Nodes() {
		if node.Kind != graph.NodeStruct && node.Kind != graph.NodeInterface {
			continue
		}
		edges := g.Outgoing(node.ID, graph.EdgeEmbeds)
		if len(edges) == 0 {
			continue
		}
		printed = true
		fmt.Println(node.ID)
		for _, edge := range edges {
			fmt.Printf("  -> %s\n", edge.To)
		}
	}
	if !printed {
		fmt.Println("  (none)")
	}

	printSignatureRelationships(g, "Accepted types", graph.EdgeAccepts)
	printSignatureRelationships(g, "Returned types", graph.EdgeReturns)
	printDirectNavigation(g)

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

	printDependencyExplanations(g, dependencies)
}

func printDirectNavigation(g *graph.Graph) {
	fmt.Println("\nDirect semantic dependencies:")
	printed := false
	for _, node := range g.Nodes() {
		relationships := query.DirectDependencies(g, node.ID)
		if len(relationships) == 0 {
			continue
		}
		printed = true
		fmt.Println(node.ID)
		for _, relationship := range relationships {
			fmt.Printf("  %s -> %s\n", relationship.Kind, relationship.To)
		}
	}
	if !printed {
		fmt.Println("  (none)")
	}

	fmt.Println("\nDirect semantic dependents:")
	printed = false
	for _, node := range g.Nodes() {
		relationships := query.DirectDependents(g, node.ID)
		if len(relationships) == 0 {
			continue
		}
		printed = true
		fmt.Println(node.ID)
		for _, relationship := range relationships {
			fmt.Printf("  <- %s %s\n", relationship.Kind, relationship.From)
		}
	}
	if !printed {
		fmt.Println("  (none)")
	}
}

func printSignatureRelationships(g *graph.Graph, heading string, kind graph.EdgeKind) {
	fmt.Printf("\n%s:\n", heading)
	printed := false
	for _, node := range g.Nodes() {
		if node.Kind != graph.NodeFunction {
			continue
		}
		edges := g.Outgoing(node.ID, kind)
		if len(edges) == 0 {
			continue
		}
		printed = true
		fmt.Println(node.ID)
		for _, edge := range edges {
			fmt.Printf("  -> %s\n", edge.To)
		}
	}
	if !printed {
		fmt.Println("  (none)")
	}
}

func printDependencyExplanations(g *graph.Graph, dependencies []query.Dependency) {
	fmt.Println("\nDependency explanations:")
	if len(dependencies) == 0 {
		fmt.Println("  (none)")
		return
	}
	for index, dependency := range dependencies {
		explanation, ok := query.WhyDependsOn(g, dependency.From, dependency.To)
		if !ok {
			continue
		}
		if index > 0 {
			fmt.Println()
		}
		fmt.Printf("%s -> %s\n", explanation.From, explanation.To)
		for _, evidence := range explanation.Evidence {
			fmt.Printf("  because %s\n", evidence.From)
			fmt.Printf("    calls %s\n", evidence.To)
			for _, location := range evidence.Locations {
				fmt.Printf("    at %s:%d\n", location.File, location.Offset)
			}
		}
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
