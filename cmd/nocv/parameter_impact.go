package main

import (
	"fmt"
	"io"
	"strings"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/query"
)

func printParameterChangeImpact(out io.Writer, analysis *goanalyzer.Analysis, values []string) error {
	callable, proposed, err := parseProposedSignature(values)
	if err != nil {
		return err
	}
	impact, err := analysis.AnalyzeParameterChange(callable, proposed)
	if err != nil {
		return err
	}

	name := callableName(string(impact.Callable))
	fmt.Fprintln(out, "SIGNATURE CHANGE")
	fmt.Fprintf(out, "\n%s\n", impact.Callable)
	fmt.Fprintf(out, "\nBefore\n  %s%s\n", name, formatSignature(impact.Before))
	fmt.Fprintf(out, "\nAfter\n  %s%s\n", name, formatSignature(impact.After))
	fmt.Fprintln(out, "\nCALL-SITE IMPACT")
	if len(impact.CallSites) == 0 {
		fmt.Fprintln(out, "  (none)")
	} else {
		for _, compatibility := range []query.Compatibility{
			query.CompatibilityIncompatible,
			query.CompatibilityUnknown,
			query.CompatibilityCompatible,
		} {
			printed := false
			for _, site := range impact.CallSites {
				if site.Compatibility != compatibility {
					continue
				}
				if !printed {
					fmt.Fprintf(out, "\n%s\n", strings.ToUpper(compatibility.String()))
					printed = true
				}
				fmt.Fprintf(out, "  %s\n", site.Caller.Ref)
				fmt.Fprintf(out, "    %s @ %d\n", site.Location.File, site.Location.Offset)
				for _, problem := range site.Problems {
					printSignatureProblem(out, problem)
				}
			}
		}
	}

	fmt.Fprintln(out, "\nCONTRACT IMPACT")
	if len(impact.Contracts) == 0 {
		fmt.Fprintln(out, "  (none)")
	} else {
		fmt.Fprintln(out, "\nLOST IMPLEMENTATION")
		for _, contract := range impact.Contracts {
			fmt.Fprintf(out, "  %s no longer implements %s\n", contract.Concrete.Name, contract.Interface.Name)
			if contract.ConcreteMethod.Ref != "" {
				fmt.Fprintf(out, "    %s\n", methodDisplay(contract.ConcreteMethod))
			}
			if contract.InterfaceMethod.Ref != "" {
				fmt.Fprintf(out, "    %s\n", methodDisplay(contract.InterfaceMethod))
			}
		}
	}

	fmt.Fprintln(out, "\nSTRUCTURAL IMPACT")
	if len(impact.Structural) == 0 {
		fmt.Fprintln(out, "  (none)")
		return nil
	}
	fmt.Fprintln(out, "\nPROMOTED METHOD CHANGED")
	for _, structural := range impact.Structural {
		typeName := structural.Type.Name
		if structural.Exposure == query.MethodExposurePointerOnly {
			typeName = "*" + typeName
		}
		fmt.Fprintf(
			out,
			"  %s exposes %s from %s\n",
			typeName,
			structural.OriginMethod.Name,
			methodDisplay(structural.OriginMethod),
		)
	}
	return nil
}

func methodDisplay(symbol query.SymbolSummary) string {
	if symbol.ParentName != "" {
		return symbol.ParentName + "." + symbol.Name
	}
	return symbol.Name
}

func parseProposedSignature(values []string) (graph.SymbolRef, query.ProposedSignature, error) {
	if len(values) == 0 {
		return "", query.ProposedSignature{}, fmt.Errorf("impact-params requires a callable symbol")
	}
	callable := graph.SymbolRef(values[0])
	var proposed query.ProposedSignature
	for index := 1; index < len(values); index++ {
		switch values[index] {
		case "--param":
			index++
			if index >= len(values) {
				return "", query.ProposedSignature{}, fmt.Errorf("--param requires a Go type expression")
			}
			proposed.Parameters = append(proposed.Parameters, query.ProposedParameter{TypeExpr: values[index]})
		case "--variadic":
			if proposed.Variadic {
				return "", query.ProposedSignature{}, fmt.Errorf("--variadic may be specified only once")
			}
			proposed.Variadic = true
		default:
			return "", query.ProposedSignature{}, fmt.Errorf("unknown impact-params option: %s", values[index])
		}
	}
	return callable, proposed, nil
}

func callableName(ref string) string {
	if index := strings.LastIndex(ref, "::"); index >= 0 {
		return ref[index+2:]
	}
	return ref
}

func formatSignature(signature query.CallableSignature) string {
	parameters := make([]string, 0, len(signature.Parameters))
	for index, parameter := range signature.Parameters {
		display := parameter.Type.Display
		if signature.Variadic && index == len(signature.Parameters)-1 {
			display = "..." + display
		}
		parameters = append(parameters, display)
	}
	display := "(" + strings.Join(parameters, ", ") + ")"
	results := make([]string, 0, len(signature.Results))
	for _, result := range signature.Results {
		results = append(results, result.Type.Display)
	}
	if len(results) == 1 {
		return display + " " + results[0]
	}
	if len(results) > 1 {
		return display + " (" + strings.Join(results, ", ") + ")"
	}
	return display
}

func printSignatureProblem(out io.Writer, problem query.SignatureProblem) {
	switch problem.Kind {
	case query.ProblemArgumentCount:
		fmt.Fprintf(out, "    argument count: %d -> %d\n", problem.ActualCount, problem.ExpectedCount)
	case query.ProblemArgumentType:
		fmt.Fprintf(out, "    argument %d: %s -> %s\n", problem.Argument, problem.Actual.Display, problem.Expected.Display)
	case query.ProblemVariadic:
		fmt.Fprintln(out, "    variadic call syntax is incompatible with the proposed signature")
	case query.ProblemUnknown:
		if problem.Argument != 0 {
			fmt.Fprintf(out, "    argument %d: compatibility unknown\n", problem.Argument)
		} else {
			fmt.Fprintln(out, "    compatibility unknown")
		}
	}
}
