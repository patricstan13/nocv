package main

import (
	"fmt"
	"io"
	"sort"

	"nocv/graph"
	"nocv/query"
)

func printTree(out io.Writer, g *graph.Graph) {
	for _, node := range g.Nodes() {
		if node.Kind != graph.NodePackage {
			continue
		}
		fmt.Fprintf(out, "%s [package] (%s)\n", node.Name, node.ID)
		printChildren(out, g, node.ID, "")
	}
}

func printChildren(out io.Writer, g *graph.Graph, parent graph.SymbolID, indent string) {
	children := g.Children(parent)
	for index, id := range children {
		node, _ := g.Node(id)
		branch := "|- "
		nextIndent := indent + "|  "
		if index == len(children)-1 {
			branch = "`- "
			nextIndent = indent + "   "
		}
		fmt.Fprintf(out, "%s%s%s [%s]\n", indent, branch, node.Name, node.Kind)
		printChildren(out, g, node.ID, nextIndent)
	}
}

func printEdges(out io.Writer, g *graph.Graph, heading string, kind graph.EdgeKind) {
	fmt.Fprintf(out, "%s:\n", heading)
	printed := false
	for _, node := range g.Nodes() {
		edges := sortedOutgoing(g, node.ID, kind)
		if len(edges) == 0 {
			continue
		}
		printed = true
		fmt.Fprintln(out, node.ID)
		for _, edge := range edges {
			if kind == graph.EdgeCalls {
				fmt.Fprintf(out, "  calls -> %s (%d call site(s))\n", edge.To, len(edge.Evidence))
			} else {
				fmt.Fprintf(out, "  %s -> %s\n", edge.Kind, edge.To)
			}
		}
	}
	if !printed {
		fmt.Fprintln(out, "  (none)")
	}
}

func printSignatures(out io.Writer, g *graph.Graph) {
	fmt.Fprintln(out, "Signatures:")
	printed := false
	for _, node := range g.Nodes() {
		if node.Kind != graph.NodeFunction {
			continue
		}
		edges := sortedOutgoing(g, node.ID, graph.EdgeAccepts, graph.EdgeReturns)
		if len(edges) == 0 {
			continue
		}
		printed = true
		fmt.Fprintln(out, node.ID)
		for _, edge := range edges {
			fmt.Fprintf(out, "  %s -> %s\n", edge.Kind, edge.To)
		}
	}
	if !printed {
		fmt.Fprintln(out, "  (none)")
	}
}

func printDirectDependencies(out io.Writer, g *graph.Graph, id graph.SymbolID) {
	fmt.Fprintln(out, id)
	relationships := query.DirectDependencies(g, id)
	if len(relationships) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, relationship := range relationships {
		fmt.Fprintf(out, "  %s -> %s\n", relationship.Kind, relationship.To)
	}
}

func printDirectDependents(out io.Writer, g *graph.Graph, id graph.SymbolID) {
	fmt.Fprintln(out, id)
	relationships := query.DirectDependents(g, id)
	if len(relationships) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, relationship := range relationships {
		fmt.Fprintf(out, "  <- %s %s\n", relationship.Kind, relationship.From)
	}
}

func printDirectImports(out io.Writer, g *graph.Graph, id graph.SymbolID) {
	fmt.Fprintln(out, id)
	relationships := query.DirectImports(g, id)
	if len(relationships) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, relationship := range relationships {
		fmt.Fprintf(out, "  imports -> %s\n", relationship.To)
	}
}

func printDirectImporters(out io.Writer, g *graph.Graph, id graph.SymbolID) {
	fmt.Fprintln(out, id)
	relationships := query.DirectImporters(g, id)
	if len(relationships) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, relationship := range relationships {
		fmt.Fprintf(out, "  <- imports %s\n", relationship.From)
	}
}

func printImpact(out io.Writer, g *graph.Graph, id graph.SymbolID) {
	fmt.Fprintf(out, "Impact of %s:\n", id)
	results := query.Impact(g, id)
	if len(results) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, result := range results {
		fmt.Fprintln(out, result.ID)
		printSemanticPaths(out, result.Paths, "  ")
	}
}

func printDependencyPaths(out io.Writer, g *graph.Graph, from, to graph.SymbolID) {
	fmt.Fprintf(out, "Dependency paths: %s -> %s\n", from, to)
	paths := query.DependencyPaths(g, from, to)
	if len(paths) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	printSemanticPaths(out, paths, "")
}

func printSemanticPaths(out io.Writer, paths []query.SemanticPath, indent string) {
	for index, path := range paths {
		fmt.Fprintf(out, "%spath %d:\n", indent, index+1)
		for _, step := range path.Steps {
			fmt.Fprintf(out, "%s  %s %s -> %s\n", indent, step.From, step.Kind, step.To)
		}
	}
}

func printPackageDependencyPaths(out io.Writer, g *graph.Graph, from, to graph.SymbolID) {
	fmt.Fprintf(out, "Package dependency paths: %s -> %s\n", from, to)
	paths := query.PackageDependencyPaths(g, from, to)
	if len(paths) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for index, path := range paths {
		fmt.Fprintf(out, "path %d:\n", index+1)
		for packageIndex, packageID := range path.Packages {
			if packageIndex == 0 {
				fmt.Fprintf(out, "  %s\n", packageID)
			} else {
				fmt.Fprintf(out, "  -> %s\n", packageID)
			}
		}
		for evidenceIndex, evidence := range path.Evidence {
			fmt.Fprintf(out, "  evidence %d:\n", evidenceIndex+1)
			for _, step := range evidence.Steps {
				fmt.Fprintf(out, "    %s %s -> %s\n", step.From, step.Kind, step.To)
			}
		}
	}
}

func printPackageDependencyExplanation(out io.Writer, g *graph.Graph, from, to graph.SymbolID) {
	fmt.Fprintf(out, "Package call dependency explanation: %s -> %s\n", from, to)
	explanation, exists := query.WhyDependsOn(g, from, to)
	if !exists {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, evidence := range explanation.Evidence {
		fmt.Fprintf(out, "  %s calls -> %s\n", evidence.From, evidence.To)
		for _, location := range evidence.Locations {
			fmt.Fprintf(out, "    at %s:%d\n", location.File, location.Offset)
		}
	}
}

func sortedOutgoing(g *graph.Graph, id graph.SymbolID, kinds ...graph.EdgeKind) []*graph.Edge {
	edges := g.Outgoing(id, kinds...)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Kind != edges[j].Kind {
			return edges[i].Kind < edges[j].Kind
		}
		return edges[i].To < edges[j].To
	})
	return edges
}
