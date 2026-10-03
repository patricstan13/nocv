package main

import (
	"fmt"
	"io"

	"nocv/goanalyzer"
	"nocv/graph"
)

const topLevelHelp = `usage: nocv <command> [arguments...]

commands:
  serve <pattern>  start the graphical explorer
  tree <pattern>   show the structural hierarchy
  node ...         inspect graph nodes and semantic relationships
  go ...           run Go-specific package and type operations

use "nocv node --help" or "nocv go --help" for scoped commands`

const nodeHelp = `usage: nocv node <command> [arguments...]

a node is a represented graph entity addressed by its textual reference.
dependencies and dependents are immediate semantic relationships.
semantic navigation includes Calls, Implements, Embeds, Accepts, and Returns.

commands:
  dependencies <pattern> <node-ref>
  dependents <pattern> <node-ref>
  transitive-dependents <pattern> <node-ref>
  dependency-paths <pattern> <from-node-ref> <to-node-ref>
  inspect <pattern> <node-ref>`

const goHelp = `usage: nocv go <scope> [arguments...]

Go-specific semantic operations.

scopes:
  package  Go package imports, semantic dependencies, and architecture checks
  type     dependencies projected through structs, interfaces, and methods`

const goPackageHelp = `usage: nocv go package <command> [arguments...]

imports and importers are direct Go import relationships.
dependency operations use semantic Calls, Implements, Embeds, Accepts, and Returns;
imports are not included in semantic package dependencies.

commands:
  imports <pattern> <package-ref>
  importers <pattern> <package-ref>
  dependency-paths <pattern> <from-package-ref> <to-package-ref>
  inspect-dependency <pattern> <from-package-ref> <to-package-ref>
  check-forbidden-import <pattern> <from-package-ref> <to-package-ref>
  check-forbidden-dependency <pattern> <from-package-ref> <to-package-ref>
  check-import-cycle <pattern> <from-package-ref> <to-package-ref>`

const goTypeHelp = `usage: nocv go type <command> [arguments...]

type dependencies are projected through represented Go structs, interfaces,
and their methods. Package functions do not participate.

commands:
  dependencies <pattern> <type-ref>
  dependency-paths <pattern> <from-type-ref> <to-type-ref>`

type invocation struct {
	name    string
	pattern string
	values  []string
	help    string
}

func parseInvocation(args []string) (invocation, error) {
	if len(args) == 0 {
		return invocation{}, fmt.Errorf("%s", topLevelHelp)
	}
	if isHelp(args) {
		return invocation{help: topLevelHelp}, nil
	}
	switch args[0] {
	case "serve":
		return parseCommand("serve", "nocv serve <pattern>", args[1:], 0)
	case "tree":
		return parseCommand("tree", "nocv tree <pattern>", args[1:], 0)
	case "node":
		return parseNodeInvocation(args[1:])
	case "go":
		return parseGoInvocation(args[1:])
	default:
		return invocation{}, fmt.Errorf("unknown command %q", args[0])
	}
}

func parseNodeInvocation(args []string) (invocation, error) {
	if len(args) == 0 {
		return invocation{}, fmt.Errorf("%s", nodeHelp)
	}
	if isHelp(args) {
		return invocation{help: nodeHelp}, nil
	}
	switch args[0] {
	case "dependencies":
		return parseCommand("node.dependencies", "nocv node dependencies <pattern> <node-ref>", args[1:], 1)
	case "dependents":
		return parseCommand("node.dependents", "nocv node dependents <pattern> <node-ref>", args[1:], 1)
	case "transitive-dependents":
		return parseCommand("node.transitive-dependents", "nocv node transitive-dependents <pattern> <node-ref>", args[1:], 1)
	case "dependency-paths":
		return parseCommand("node.dependency-paths", "nocv node dependency-paths <pattern> <from-node-ref> <to-node-ref>", args[1:], 2)
	case "inspect":
		return parseCommand("node.inspect", "nocv node inspect <pattern> <node-ref>", args[1:], 1)
	default:
		return invocation{}, fmt.Errorf("unknown node command %q", args[0])
	}
}

func parseGoInvocation(args []string) (invocation, error) {
	if len(args) == 0 {
		return invocation{}, fmt.Errorf("%s", goHelp)
	}
	if isHelp(args) {
		return invocation{help: goHelp}, nil
	}
	switch args[0] {
	case "package":
		return parseGoPackageInvocation(args[1:])
	case "type":
		return parseGoTypeInvocation(args[1:])
	default:
		return invocation{}, fmt.Errorf("unknown Go scope %q", args[0])
	}
}

func parseGoPackageInvocation(args []string) (invocation, error) {
	if len(args) == 0 {
		return invocation{}, fmt.Errorf("%s", goPackageHelp)
	}
	if isHelp(args) {
		return invocation{help: goPackageHelp}, nil
	}
	switch args[0] {
	case "imports":
		return parseCommand("go.package.imports", "nocv go package imports <pattern> <package-ref>", args[1:], 1)
	case "importers":
		return parseCommand("go.package.importers", "nocv go package importers <pattern> <package-ref>", args[1:], 1)
	case "dependency-paths":
		return parseCommand("go.package.dependency-paths", "nocv go package dependency-paths <pattern> <from-package-ref> <to-package-ref>", args[1:], 2)
	case "inspect-dependency":
		return parseCommand("go.package.inspect-dependency", "nocv go package inspect-dependency <pattern> <from-package-ref> <to-package-ref>", args[1:], 2)
	case "check-forbidden-import":
		return parseCommand("go.package.check-forbidden-import", "nocv go package check-forbidden-import <pattern> <from-package-ref> <to-package-ref>", args[1:], 2)
	case "check-forbidden-dependency":
		return parseCommand("go.package.check-forbidden-dependency", "nocv go package check-forbidden-dependency <pattern> <from-package-ref> <to-package-ref>", args[1:], 2)
	case "check-import-cycle":
		return parseCommand("go.package.check-import-cycle", "nocv go package check-import-cycle <pattern> <from-package-ref> <to-package-ref>", args[1:], 2)
	default:
		return invocation{}, fmt.Errorf("unknown Go package command %q", args[0])
	}
}

func parseGoTypeInvocation(args []string) (invocation, error) {
	if len(args) == 0 {
		return invocation{}, fmt.Errorf("%s", goTypeHelp)
	}
	if isHelp(args) {
		return invocation{help: goTypeHelp}, nil
	}
	switch args[0] {
	case "dependencies":
		return parseCommand("go.type.dependencies", "nocv go type dependencies <pattern> <type-ref>", args[1:], 1)
	case "dependency-paths":
		return parseCommand("go.type.dependency-paths", "nocv go type dependency-paths <pattern> <from-type-ref> <to-type-ref>", args[1:], 2)
	default:
		return invocation{}, fmt.Errorf("unknown Go type command %q", args[0])
	}
}

func parseCommand(name, usage string, args []string, valueCount int) (invocation, error) {
	if len(args) != 1+valueCount {
		return invocation{}, fmt.Errorf("usage: %s", usage)
	}
	return invocation{name: name, pattern: args[0], values: args[1:]}, nil
}

func isHelp(args []string) bool {
	return len(args) == 1 && (args[0] == "--help" || args[0] == "-h")
}

func executeCommand(out io.Writer, g *graph.Graph, invocation invocation) error {
	return executeCommandWithAnalysis(out, g, nil, invocation)
}

func executeCommandWithAnalysis(out io.Writer, g *graph.Graph, analysis *goanalyzer.Analysis, invocation invocation) error {
	switch invocation.name {
	case "serve":
		if analysis == nil {
			return fmt.Errorf("Go analysis is unavailable for serve")
		}
		return serve(out, analysis)
	case "tree":
		printTree(out, g)
	case "node.dependencies", "node.dependents", "node.transitive-dependents", "node.dependency-paths", "node.inspect":
		return executeNodeCommand(out, g, invocation)
	case "go.package.imports", "go.package.importers", "go.package.dependency-paths", "go.package.inspect-dependency",
		"go.package.check-forbidden-import", "go.package.check-forbidden-dependency", "go.package.check-import-cycle":
		return executeGoPackageCommand(out, g, invocation)
	case "go.type.dependencies", "go.type.dependency-paths":
		return executeGoTypeCommand(out, g, invocation)
	default:
		return fmt.Errorf("unknown command %q", invocation.name)
	}
	return nil
}

func executeNodeCommand(out io.Writer, g *graph.Graph, invocation invocation) error {
	switch invocation.name {
	case "node.dependencies":
		id := graph.SymbolRef(invocation.values[0])
		if err := requireSymbol(g, id); err != nil {
			return err
		}
		printDirectDependencies(out, g, id)
	case "node.dependents":
		id := graph.SymbolRef(invocation.values[0])
		if err := requireSymbol(g, id); err != nil {
			return err
		}
		printDirectDependents(out, g, id)
	case "node.transitive-dependents":
		id := graph.SymbolRef(invocation.values[0])
		if err := requireSymbol(g, id); err != nil {
			return err
		}
		printTransitiveDependents(out, g, id)
	case "node.dependency-paths":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requireSymbol(g, from); err != nil {
			return err
		}
		if err := requireSymbol(g, to); err != nil {
			return err
		}
		printDependencyPaths(out, g, from, to)
	case "node.inspect":
		id := graph.SymbolRef(invocation.values[0])
		if err := requireSymbol(g, id); err != nil {
			return err
		}
		printNodeInspection(out, g, id)
	default:
		return fmt.Errorf("unknown node command %q", invocation.name)
	}
	return nil
}

func executeGoPackageCommand(out io.Writer, g *graph.Graph, invocation invocation) error {
	switch invocation.name {
	case "go.package.imports":
		id := graph.SymbolRef(invocation.values[0])
		if err := requirePackage(g, id); err != nil {
			return err
		}
		printDirectImports(out, g, id)
	case "go.package.importers":
		id := graph.SymbolRef(invocation.values[0])
		if err := requirePackage(g, id); err != nil {
			return err
		}
		printDirectImporters(out, g, id)
	case "go.package.check-import-cycle":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requirePackage(g, from); err != nil {
			return err
		}
		if err := requirePackage(g, to); err != nil {
			return err
		}
		printImportCycleCheck(out, g, from, to)
	case "go.package.check-forbidden-import":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requirePackage(g, from); err != nil {
			return err
		}
		if err := requirePackage(g, to); err != nil {
			return err
		}
		printForbiddenImportCheck(out, g, from, to)
	case "go.package.check-forbidden-dependency":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requirePackage(g, from); err != nil {
			return err
		}
		if err := requirePackage(g, to); err != nil {
			return err
		}
		printForbiddenDependencyCheck(out, g, from, to)
	case "go.package.dependency-paths":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requirePackage(g, from); err != nil {
			return err
		}
		if err := requirePackage(g, to); err != nil {
			return err
		}
		printPackageDependencyPaths(out, g, from, to)
	case "go.package.inspect-dependency":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requirePackage(g, from); err != nil {
			return err
		}
		if err := requirePackage(g, to); err != nil {
			return err
		}
		printDependencyInspection(out, g, from, to)
	default:
		return fmt.Errorf("unknown Go package command %q", invocation.name)
	}
	return nil
}

func executeGoTypeCommand(out io.Writer, g *graph.Graph, invocation invocation) error {
	switch invocation.name {
	case "go.type.dependencies":
		id := graph.SymbolRef(invocation.values[0])
		if err := requireType(g, id); err != nil {
			return err
		}
		printDirectTypeDependencies(out, g, id)
	case "go.type.dependency-paths":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requireType(g, from); err != nil {
			return err
		}
		if err := requireType(g, to); err != nil {
			return err
		}
		printTypeDependencyPaths(out, g, from, to)
	default:
		return fmt.Errorf("unknown Go type command %q", invocation.name)
	}
	return nil
}

func requireSymbol(g *graph.Graph, id graph.SymbolRef) error {
	if _, exists := g.NodeByRef(id); !exists {
		return fmt.Errorf("unknown symbol: %s", id)
	}
	return nil
}

func requirePackage(g *graph.Graph, id graph.SymbolRef) error {
	node, exists := g.NodeByRef(id)
	if !exists {
		return fmt.Errorf("unknown package: %s", id)
	}
	if node.Kind != graph.NodePackage {
		return fmt.Errorf("symbol is not a package: %s", id)
	}
	return nil
}

func requireType(g *graph.Graph, id graph.SymbolRef) error {
	node, exists := g.NodeByRef(id)
	if !exists {
		return fmt.Errorf("unknown symbol: %s", id)
	}
	if node.Kind != graph.NodeStruct && node.Kind != graph.NodeInterface {
		return fmt.Errorf("symbol is not a type: %s", id)
	}
	return nil
}
