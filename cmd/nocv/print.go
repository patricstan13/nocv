package main

import (
	"fmt"
	"io"
	"strings"

	"nocv/graph"
	"nocv/query"
)

func printTree(out io.Writer, g *graph.Graph) {
	for _, node := range g.Nodes() {
		if node.Kind != graph.NodePackage {
			continue
		}
		fmt.Fprintf(out, "%s [package] (%s)\n", node.Name, node.Ref)
		printChildren(out, g, node.ID, "")
	}
}

func printChildren(out io.Writer, g *graph.Graph, parent graph.NodeID, indent string) {
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

func printDirectDependencies(out io.Writer, g *graph.Graph, id graph.SymbolRef) {
	fmt.Fprintln(out, id)
	relationships := query.DirectDependencies(g, id)
	if len(relationships) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, relationship := range relationships {
		fmt.Fprintf(out, "  %s%s -> %s\n", relationship.Kind, certaintyMarker(relationship.Certainty), relationship.To)
	}
}

func printDirectDependents(out io.Writer, g *graph.Graph, id graph.SymbolRef) {
	fmt.Fprintln(out, id)
	relationships := query.DirectDependents(g, id)
	if len(relationships) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, relationship := range relationships {
		fmt.Fprintf(out, "  <- %s%s %s\n", relationship.Kind, certaintyMarker(relationship.Certainty), relationship.From)
	}
}

func printDirectImports(out io.Writer, g *graph.Graph, id graph.SymbolRef) {
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

func printDirectImporters(out io.Writer, g *graph.Graph, id graph.SymbolRef) {
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

func printImportCycleCheck(out io.Writer, g *graph.Graph, from, to graph.SymbolRef) {
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

func printForbiddenImportCheck(out io.Writer, g *graph.Graph, from, to graph.SymbolRef) {
	fmt.Fprintf(out, "Forbidden package import:\n  %s -> %s\n\n", from, to)
	violation, exists := query.CheckForbiddenPackageImport(g, from, to)
	if !exists {
		fmt.Fprintln(out, "No violation.")
		return
	}

	fmt.Fprintln(out, "VIOLATION")
	fmt.Fprintln(out, "\nevidence:")
	for _, location := range violation.Evidence {
		fmt.Fprintf(out, "  %s\n", formatLocation(location))
	}
}

func formatLocation(location graph.Location) string {
	if location.File == "" {
		return "(unknown)"
	}
	if location.Line == 0 || location.Column == 0 {
		return location.File
	}
	return fmt.Sprintf("%s:%d:%d", location.File, location.Line, location.Column)
}

func printForbiddenDependencyCheck(out io.Writer, g *graph.Graph, from, to graph.SymbolRef) query.ForbiddenDependencyOutcome {
	fmt.Fprintf(out, "Forbidden package dependency:\n  %s -> %s\n\n", from, to)
	violation, exists := query.CheckForbiddenPackageDependency(g, from, to)
	if !exists {
		fmt.Fprintln(out, "No violation.")
		return query.ForbiddenDependencyNoObservedPath
	}

	if violation.Outcome == query.ForbiddenDependencyPotentialViolation {
		fmt.Fprintln(out, "POTENTIAL VIOLATION (inconclusive)")
		fmt.Fprintln(out, "Only uncertain semantic paths were observed.")
	} else {
		fmt.Fprintln(out, "VIOLATION")
	}
	fmt.Fprintln(out)
	printPackagePathResults(out, violation.Paths)
	return violation.Outcome
}

func printTransitiveDependents(out io.Writer, g *graph.Graph, id graph.SymbolRef) {
	fmt.Fprintf(out, "Transitive dependents of %s:\n", id)
	results := query.TransitiveDependents(g, id)
	if len(results) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, result := range results {
		fmt.Fprintln(out, result.ID)
		printSemanticPaths(out, result.Paths, "  ")
	}
}

func printDependencyPaths(out io.Writer, g *graph.Graph, from, to graph.SymbolRef) {
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
			fmt.Fprintf(out, "%s  %s %s%s -> %s\n", indent, step.From, step.Kind, certaintyMarker(step.Certainty), step.To)
		}
	}
}

func printPackageDependencyPaths(out io.Writer, g *graph.Graph, from, to graph.SymbolRef) {
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
			fmt.Fprintf(out, "  ->%s %s\n", certaintyMarker(step.Certainty), step.To)
			fmt.Fprintln(out, "     evidence:")
			for _, evidence := range step.Evidence {
				fmt.Fprintf(out, "       %s %s%s -> %s\n", evidence.From, evidence.Kind, certaintyMarker(evidence.Certainty), evidence.To)
			}
		}
	}
}

func printDirectTypeDependencies(out io.Writer, g *graph.Graph, id graph.SymbolRef) {
	fmt.Fprintf(out, "Type dependencies:\n  %s\n", id)
	dependencies := query.DirectTypeDependencies(g, id)
	if len(dependencies) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, dependency := range dependencies {
		fmt.Fprintf(out, "  ->%s %s\n", certaintyMarker(dependency.Certainty), dependency.To)
		fmt.Fprintln(out, "     evidence:")
		for _, evidence := range dependency.Evidence {
			fmt.Fprintf(out, "       %s %s%s -> %s\n", evidence.From, evidence.Kind, certaintyMarker(evidence.Certainty), evidence.To)
		}
	}
}

func printTypeDependencyPaths(out io.Writer, g *graph.Graph, from, to graph.SymbolRef) {
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
			fmt.Fprintf(out, "  ->%s %s\n", certaintyMarker(step.Certainty), step.To)
			fmt.Fprintln(out, "     evidence:")
			for _, evidence := range step.Evidence {
				fmt.Fprintf(out, "       %s %s%s -> %s\n", evidence.From, evidence.Kind, certaintyMarker(evidence.Certainty), evidence.To)
			}
		}
	}
}

func printDependencyInspection(out io.Writer, g *graph.Graph, from, to graph.SymbolRef) {
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
					fmt.Fprintf(out, "    %s ->%s %s\n", dependency.From, certaintyMarker(dependency.Certainty), dependency.To)
					fmt.Fprintln(out, "      evidence:")
					for _, evidence := range dependency.Evidence {
						fmt.Fprintf(out, "        %s %s%s -> %s\n", evidence.From, evidence.Kind, certaintyMarker(evidence.Certainty), evidence.To)
					}
				}
			}

			fmt.Fprintln(out, "\n  EXACT ONLY")
			if len(inspection.ExactOnly) == 0 {
				fmt.Fprintln(out, "    (none)")
			}
			for _, evidence := range inspection.ExactOnly {
				fmt.Fprintf(out, "    %s %s%s -> %s\n", evidence.From, evidence.Kind, certaintyMarker(evidence.Certainty), evidence.To)
			}
		}
	}
}

func printNodeInspection(out io.Writer, g *graph.Graph, id graph.SymbolRef) {
	inspection, exists := query.InspectNode(g, id)
	if !exists {
		return
	}
	node := inspection.Node
	fmt.Fprintf(out, "Node inspection:\n  %s\n\nKind:\n  %s\n", node.Ref, node.Kind)
	if parent, exists := g.Node(node.Parent); exists {
		fmt.Fprintf(out, "\nParent:\n  %s\n", parent.Ref)
	}
	if node.Kind != graph.NodePackage {
		fmt.Fprintln(out, "\nLocation:")
		if node.Location.File == "" {
			fmt.Fprintln(out, "  (unknown)")
		} else {
			fmt.Fprintf(out, "  %s\n", formatLocation(node.Location))
		}
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
		printSymbolSummaryList(out, "Methods", detail.Methods)
		printTypeDependenciesForNode(out, "Type dependencies", detail.Dependencies, false)
		printTypeDependenciesForNode(out, "Type dependents", detail.Dependents, true)
		printSymbolRelationshipsForNode(out, "Direct semantic dependencies", detail.DirectDependencies, false)
		printSymbolRelationshipsForNode(out, "Direct semantic dependents", detail.DirectDependents, true)

	case inspection.Function != nil:
		detail := inspection.Function
		printSymbolRelationshipsForNode(out, "Calls", detail.Calls, false)
		printSymbolRelationshipsForNode(out, "Called by", detail.CalledBy, true)
		printSymbolRelationshipsForNode(out, "Accepts", detail.Accepts, false)
		printSymbolRelationshipsForNode(out, "Returns", detail.Returns, false)
		printSymbolRelationshipsForNode(out, "Implements", detail.Implements, false)
		printSymbolRelationshipsForNode(out, "Implemented by", detail.ImplementedBy, true)
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
		fmt.Fprintf(out, "  %s (%s)\n", node.Name, node.Ref)
	}
}

func printSymbolSummaryList(out io.Writer, heading string, summaries []query.SymbolSummary) {
	fmt.Fprintf(out, "\n%s:\n", heading)
	if len(summaries) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, summary := range summaries {
		fmt.Fprintf(out, "  %s (%s)\n", summary.Name, summary.Ref)
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
			fmt.Fprintf(out, "  <-%s %s\n", certaintyMarker(dependency.Certainty), dependency.From)
		} else {
			fmt.Fprintf(out, "  ->%s %s\n", certaintyMarker(dependency.Certainty), dependency.To)
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
			fmt.Fprintf(out, "  <-%s %s\n", certaintyMarker(dependency.Certainty), dependency.From)
		} else {
			fmt.Fprintf(out, "  ->%s %s\n", certaintyMarker(dependency.Certainty), dependency.To)
		}
		printNodeEvidence(out, dependency.Evidence)
	}
}

func printNodeEvidence(out io.Writer, evidence []query.Relationship) {
	fmt.Fprintln(out, "     evidence:")
	for _, relationship := range evidence {
		fmt.Fprintf(out, "       %s %s%s -> %s\n", relationship.From, relationship.Kind, certaintyMarker(relationship.Certainty), relationship.To)
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
			fmt.Fprintf(out, "  <- %s%s %s\n", relationship.Kind, certaintyMarker(relationship.Certainty), relationship.From)
		} else {
			fmt.Fprintf(out, "  %s%s -> %s\n", relationship.Kind, certaintyMarker(relationship.Certainty), relationship.To)
		}
	}
}

func printSymbolRelationshipsForNode(out io.Writer, heading string, relationships []query.SymbolRelationship, incoming bool) {
	fmt.Fprintf(out, "\n%s:\n", heading)
	if len(relationships) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, relationship := range relationships {
		if incoming {
			fmt.Fprintf(out, "  <- %s%s %s\n", relationship.Kind, certaintyMarker(relationship.Certainty), relationship.From.Ref)
		} else {
			fmt.Fprintf(out, "  %s%s -> %s\n", relationship.Kind, certaintyMarker(relationship.Certainty), relationship.To.Ref)
		}
	}
}

func certaintyMarker(certainty graph.RelationshipCertainty) string {
	if certainty == graph.RelationshipUncertain {
		return " [uncertain]"
	}
	return ""
}
