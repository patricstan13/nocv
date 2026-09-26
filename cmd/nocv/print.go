package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

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

func printImportCycleCheck(out io.Writer, g *graph.Graph, from, to graph.SymbolID) {
	fmt.Fprintf(out, "Proposed import:\n  %s -> %s\n\n", from, to)
	result := query.WouldCreateImportCycle(g, from, to)
	if !result.WouldCycle {
		fmt.Fprintln(out, "No import cycle would be created.")
		return
	}

	fmt.Fprintln(out, "Would create import cycle.")
	for index, path := range result.Paths {
		fmt.Fprintf(out, "\nexisting path %d:\n", index+1)
		for packageIndex, packageID := range path.Packages {
			if packageIndex == 0 {
				fmt.Fprintf(out, "  %s\n", packageID)
			} else {
				fmt.Fprintf(out, "  -> %s\n", packageID)
			}
		}
	}
}

func printForbiddenImportCheck(out io.Writer, g *graph.Graph, from, to graph.SymbolID) {
	fmt.Fprintf(out, "Forbidden package import:\n  %s -> %s\n\n", from, to)
	violation, exists := query.CheckForbiddenPackageImport(g, from, to)
	if !exists {
		fmt.Fprintln(out, "No violation.")
		return
	}

	fmt.Fprintln(out, "VIOLATION")
	fmt.Fprintln(out, "\nevidence:")
	for _, location := range violation.Evidence {
		fmt.Fprintf(out, "  %s:%d\n", location.File, location.Offset)
	}
}

func printForbiddenDependencyCheck(out io.Writer, g *graph.Graph, from, to graph.SymbolID) {
	fmt.Fprintf(out, "Forbidden package dependency:\n  %s -> %s\n\n", from, to)
	violation, exists := query.CheckForbiddenPackageDependency(g, from, to)
	if !exists {
		fmt.Fprintln(out, "No violation.")
		return
	}

	fmt.Fprintln(out, "VIOLATION")
	fmt.Fprintln(out)
	printPackagePathResults(out, violation.Paths)
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
	printPackagePathResults(out, paths)
}

func printPackagePathResults(out io.Writer, paths []query.PackageDependencyPath) {
	for index, path := range paths {
		fmt.Fprintf(out, "path %d:\n", index+1)
		fmt.Fprintf(out, "  %s\n", path.Packages[0])
		for _, step := range path.Steps {
			fmt.Fprintf(out, "  -> %s\n", step.To)
			fmt.Fprintln(out, "     evidence:")
			for _, evidence := range step.Evidence {
				fmt.Fprintf(out, "       %s %s -> %s\n", evidence.From, evidence.Kind, evidence.To)
			}
		}
	}
}

func printDirectTypeDependencies(out io.Writer, g *graph.Graph, id graph.SymbolID) {
	fmt.Fprintf(out, "Type dependencies:\n  %s\n", id)
	dependencies := query.DirectTypeDependencies(g, id)
	if len(dependencies) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, dependency := range dependencies {
		fmt.Fprintf(out, "  -> %s\n", dependency.To)
		fmt.Fprintln(out, "     evidence:")
		for _, evidence := range dependency.Evidence {
			fmt.Fprintf(out, "       %s %s -> %s\n", evidence.From, evidence.Kind, evidence.To)
		}
	}
}

func printTypeDependencyPaths(out io.Writer, g *graph.Graph, from, to graph.SymbolID) {
	fmt.Fprintf(out, "Type dependency paths: %s -> %s\n", from, to)
	paths := query.TypeDependencyPaths(g, from, to)
	if len(paths) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for index, path := range paths {
		fmt.Fprintf(out, "path %d:\n", index+1)
		fmt.Fprintf(out, "  %s\n", path.Types[0])
		for _, step := range path.Steps {
			fmt.Fprintf(out, "  -> %s\n", step.To)
			fmt.Fprintln(out, "     evidence:")
			for _, evidence := range step.Evidence {
				fmt.Fprintf(out, "       %s %s -> %s\n", evidence.From, evidence.Kind, evidence.To)
			}
		}
	}
}

func printDependencyInspection(out io.Writer, g *graph.Graph, from, to graph.SymbolID) {
	fmt.Fprintf(out, "Dependency inspection:\n  %s -> %s\n", from, to)
	paths := query.PackageDependencyPaths(g, from, to)
	if len(paths) == 0 {
		fmt.Fprintln(out, "\nPACKAGE\n  no dependency")
		return
	}

	for pathIndex, path := range paths {
		fmt.Fprintf(out, "\nPACKAGE PATH %d\n", pathIndex+1)
		fmt.Fprintf(out, "  %s\n", path.Packages[0])
		for _, packageID := range path.Packages[1:] {
			fmt.Fprintf(out, "  -> %s\n", packageID)
		}

		for stepIndex, step := range path.Steps {
			fmt.Fprintf(out, "\nHOP %d\n  %s -> %s\n", stepIndex+1, step.From, step.To)
			inspection := query.InspectPackageDependency(g, step)
			fmt.Fprintln(out, "\n  TYPE")
			if len(inspection.TypeDependencies) == 0 {
				fmt.Fprintln(out, "    (none)")
			} else {
				for _, dependency := range inspection.TypeDependencies {
					fmt.Fprintf(out, "    %s -> %s\n", dependency.From, dependency.To)
					fmt.Fprintln(out, "      evidence:")
					for _, evidence := range dependency.Evidence {
						fmt.Fprintf(out, "        %s %s -> %s\n", evidence.From, evidence.Kind, evidence.To)
					}
				}
			}

			fmt.Fprintln(out, "\n  EXACT ONLY")
			if len(inspection.ExactOnly) == 0 {
				fmt.Fprintln(out, "    (none)")
			}
			for _, evidence := range inspection.ExactOnly {
				fmt.Fprintf(out, "    %s %s -> %s\n", evidence.From, evidence.Kind, evidence.To)
			}
		}
	}
}

func printNodeInspection(out io.Writer, g *graph.Graph, id graph.SymbolID) {
	inspection, exists := query.InspectNode(g, id)
	if !exists {
		return
	}
	node := inspection.Node
	fmt.Fprintf(out, "Node inspection:\n  %s\n\nKind:\n  %s\n", node.ID, node.Kind)
	if node.Parent != "" {
		fmt.Fprintf(out, "\nParent:\n  %s\n", node.Parent)
	}
	fmt.Fprintln(out, "\nLocation:")
	if node.Location.File == "" {
		fmt.Fprintln(out, "  (unknown)")
	} else {
		fmt.Fprintf(out, "  %s:%d\n", node.Location.File, node.Location.Offset)
	}
	fmt.Fprintln(out, "\nDocumentation:")
	printDocumentation(out, node.Documentation, "  ")

	switch {
	case inspection.Package != nil:
		detail := inspection.Package
		printNodeList(out, "Types", detail.Types)
		printNodeList(out, "Functions", detail.Functions)
		printPackageDependenciesForNode(out, "Semantic dependencies", detail.Dependencies, false)
		printPackageDependenciesForNode(out, "Semantic dependents", detail.Dependents, true)
		printRelationshipsForNode(out, "Imports", detail.Imports, false)
		printRelationshipsForNode(out, "Importers", detail.Importers, true)

	case inspection.Type != nil:
		detail := inspection.Type
		printNodeList(out, "Methods", detail.Methods)
		printTypeDependenciesForNode(out, "Type dependencies", detail.Dependencies, false)
		printTypeDependenciesForNode(out, "Type dependents", detail.Dependents, true)
		printRelationshipsForNode(out, "Direct semantic dependencies", detail.DirectDependencies, false)
		printRelationshipsForNode(out, "Direct semantic dependents", detail.DirectDependents, true)

	case inspection.Function != nil:
		printRelationshipsForNode(out, "Dependencies", inspection.Function.Dependencies, false)
		printRelationshipsForNode(out, "Dependents", inspection.Function.Dependents, true)
	}
}

func printDocumentation(out io.Writer, documentation, indent string) {
	if documentation == "" {
		fmt.Fprintf(out, "%s(none)\n", indent)
		return
	}
	for _, line := range strings.Split(documentation, "\n") {
		fmt.Fprintf(out, "%s%s\n", indent, line)
	}
}

func printNodeList(out io.Writer, heading string, nodes []graph.Node) {
	fmt.Fprintf(out, "\n%s:\n", heading)
	if len(nodes) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, node := range nodes {
		fmt.Fprintf(out, "  %s (%s)\n", node.Name, node.ID)
	}
}

func printPackageDependenciesForNode(out io.Writer, heading string, dependencies []query.PackageDependency, incoming bool) {
	fmt.Fprintf(out, "\n%s:\n", heading)
	if len(dependencies) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, dependency := range dependencies {
		if incoming {
			fmt.Fprintf(out, "  <- %s\n", dependency.From)
		} else {
			fmt.Fprintf(out, "  -> %s\n", dependency.To)
		}
		printNodeEvidence(out, dependency.Evidence)
	}
}

func printTypeDependenciesForNode(out io.Writer, heading string, dependencies []query.TypeDependency, incoming bool) {
	fmt.Fprintf(out, "\n%s:\n", heading)
	if len(dependencies) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, dependency := range dependencies {
		if incoming {
			fmt.Fprintf(out, "  <- %s\n", dependency.From)
		} else {
			fmt.Fprintf(out, "  -> %s\n", dependency.To)
		}
		printNodeEvidence(out, dependency.Evidence)
	}
}

func printNodeEvidence(out io.Writer, evidence []query.Relationship) {
	fmt.Fprintln(out, "     evidence:")
	for _, relationship := range evidence {
		fmt.Fprintf(out, "       %s %s -> %s\n", relationship.From, relationship.Kind, relationship.To)
	}
}

func printRelationshipsForNode(out io.Writer, heading string, relationships []query.Relationship, incoming bool) {
	fmt.Fprintf(out, "\n%s:\n", heading)
	if len(relationships) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, relationship := range relationships {
		if incoming {
			fmt.Fprintf(out, "  <- %s %s\n", relationship.Kind, relationship.From)
		} else {
			fmt.Fprintf(out, "  %s -> %s\n", relationship.Kind, relationship.To)
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
