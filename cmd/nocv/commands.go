package main

import (
	"fmt"
	"io"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/query"
)

type commandSpec struct {
	usage      string
	extraCount int
}

var commandSpecs = map[string]commandSpec{
	"serve":                      {usage: "nocv serve <pattern>"},
	"tree":                       {usage: "nocv tree <pattern>"},
	"calls":                      {usage: "nocv calls <pattern>"},
	"implementations":            {usage: "nocv implementations <pattern>"},
	"embeddings":                 {usage: "nocv embeddings <pattern>"},
	"signatures":                 {usage: "nocv signatures <pattern>"},
	"imports":                    {usage: "nocv imports <pattern>"},
	"package-imports":            {usage: "nocv package-imports <pattern> <package-ref>", extraCount: 1},
	"package-importers":          {usage: "nocv package-importers <pattern> <package-ref>", extraCount: 1},
	"import-cycle":               {usage: "nocv import-cycle <pattern> <from-package> <to-package>", extraCount: 2},
	"check-forbidden-import":     {usage: "nocv check-forbidden-import <pattern> <from-package> <to-package>", extraCount: 2},
	"check-forbidden-dependency": {usage: "nocv check-forbidden-dependency <pattern> <from-package> <to-package>", extraCount: 2},
	"direct-deps":                {usage: "nocv direct-deps <pattern> <symbol-ref>", extraCount: 1},
	"direct-dependents":          {usage: "nocv direct-dependents <pattern> <symbol-ref>", extraCount: 1},
	"impact":                     {usage: "nocv impact <pattern> <symbol-ref>", extraCount: 1},
	"paths":                      {usage: "nocv paths <pattern> <from-symbol> <to-symbol>", extraCount: 2},
	"package-paths":              {usage: "nocv package-paths <pattern> <from-package> <to-package>", extraCount: 2},
	"package-deps":               {usage: "nocv package-deps <pattern>"},
	"why-package-dep":            {usage: "nocv why-package-dep <pattern> <from-package> <to-package>", extraCount: 2},
	"type-deps":                  {usage: "nocv type-deps <pattern> <type-ref>", extraCount: 1},
	"type-paths":                 {usage: "nocv type-paths <pattern> <from-type> <to-type>", extraCount: 2},
	"inspect-dependency":         {usage: "nocv inspect-dependency <pattern> <from-package> <to-package>", extraCount: 2},
	"inspect-node":               {usage: "nocv inspect-node <pattern> <symbol-ref>", extraCount: 1},
}

type invocation struct {
	name    string
	pattern string
	values  []string
}

func parseInvocation(args []string) (invocation, error) {
	if len(args) == 0 {
		return invocation{}, fmt.Errorf("usage: nocv <command> <pattern> [arguments...]\ncommands: serve, tree, calls, implementations, embeddings, signatures, imports, package-imports, package-importers, import-cycle, check-forbidden-import, check-forbidden-dependency, direct-deps, direct-dependents, impact, paths, package-paths, package-deps, why-package-dep, type-deps, type-paths, inspect-dependency, inspect-node")
	}
	spec, exists := commandSpecs[args[0]]
	if !exists {
		return invocation{}, fmt.Errorf("unknown command %q", args[0])
	}
	if len(args) != 2+spec.extraCount {
		return invocation{}, fmt.Errorf("usage: %s", spec.usage)
	}
	return invocation{name: args[0], pattern: args[1], values: args[2:]}, nil
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
	case "calls":
		printEdges(out, g, "Calls", graph.EdgeCalls)
	case "implementations":
		printEdges(out, g, "Implementations", graph.EdgeImplements)
	case "embeddings":
		printEdges(out, g, "Embeddings", graph.EdgeEmbeds)
	case "signatures":
		printSignatures(out, g)
	case "imports":
		printEdges(out, g, "Imports", graph.EdgeImports)
	case "package-imports":
		id := graph.SymbolRef(invocation.values[0])
		if err := requirePackage(g, id); err != nil {
			return err
		}
		printDirectImports(out, g, id)
	case "package-importers":
		id := graph.SymbolRef(invocation.values[0])
		if err := requirePackage(g, id); err != nil {
			return err
		}
		printDirectImporters(out, g, id)
	case "import-cycle":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requirePackage(g, from); err != nil {
			return err
		}
		if err := requirePackage(g, to); err != nil {
			return err
		}
		printImportCycleCheck(out, g, from, to)
	case "check-forbidden-import":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requirePackage(g, from); err != nil {
			return err
		}
		if err := requirePackage(g, to); err != nil {
			return err
		}
		printForbiddenImportCheck(out, g, from, to)
	case "check-forbidden-dependency":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requirePackage(g, from); err != nil {
			return err
		}
		if err := requirePackage(g, to); err != nil {
			return err
		}
		printForbiddenDependencyCheck(out, g, from, to)
	case "direct-deps":
		id := graph.SymbolRef(invocation.values[0])
		if err := requireSymbol(g, id); err != nil {
			return err
		}
		printDirectDependencies(out, g, id)
	case "direct-dependents":
		id := graph.SymbolRef(invocation.values[0])
		if err := requireSymbol(g, id); err != nil {
			return err
		}
		printDirectDependents(out, g, id)
	case "impact":
		id := graph.SymbolRef(invocation.values[0])
		if err := requireSymbol(g, id); err != nil {
			return err
		}
		printImpact(out, g, id)
	case "paths":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requireSymbol(g, from); err != nil {
			return err
		}
		if err := requireSymbol(g, to); err != nil {
			return err
		}
		printDependencyPaths(out, g, from, to)
	case "package-paths":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requirePackage(g, from); err != nil {
			return err
		}
		if err := requirePackage(g, to); err != nil {
			return err
		}
		printPackageDependencyPaths(out, g, from, to)
	case "package-deps":
		printPackageDependencies(out, g)
	case "why-package-dep":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requirePackage(g, from); err != nil {
			return err
		}
		if err := requirePackage(g, to); err != nil {
			return err
		}
		printPackageDependencyExplanation(out, g, from, to)
	case "type-deps":
		id := graph.SymbolRef(invocation.values[0])
		if err := requireType(g, id); err != nil {
			return err
		}
		printDirectTypeDependencies(out, g, id)
	case "type-paths":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requireType(g, from); err != nil {
			return err
		}
		if err := requireType(g, to); err != nil {
			return err
		}
		printTypeDependencyPaths(out, g, from, to)
	case "inspect-dependency":
		from := graph.SymbolRef(invocation.values[0])
		to := graph.SymbolRef(invocation.values[1])
		if err := requirePackage(g, from); err != nil {
			return err
		}
		if err := requirePackage(g, to); err != nil {
			return err
		}
		printDependencyInspection(out, g, from, to)
	case "inspect-node":
		id := graph.SymbolRef(invocation.values[0])
		if err := requireSymbol(g, id); err != nil {
			return err
		}
		printNodeInspection(out, g, id)
	default:
		return fmt.Errorf("unknown command %q", invocation.name)
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

func printPackageDependencies(out io.Writer, g *graph.Graph) {
	fmt.Fprintln(out, "Package call dependencies:")
	dependencies := query.Dependencies(g, graph.NodePackage)
	if len(dependencies) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, dependency := range dependencies {
		fmt.Fprintf(out, "%s -> %s\n", dependency.From, dependency.To)
	}
}
