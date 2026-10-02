package goanalyzer

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"nocv/graph"
	"nocv/query"
)

type compilerDiagnostic struct {
	Package string
	Pos     string
	Message string
	pkg     *packages.Package
}

func collectCompilerDiagnostics(pkgs []*packages.Package, allowed map[string]bool) []compilerDiagnostic {
	result := make([]compilerDiagnostic, 0)
	for _, pkg := range pkgs {
		if !allowed[pkg.PkgPath] {
			continue
		}
		for _, packageError := range pkg.Errors {
			// The go command may add a package-summary error containing the
			// same detailed diagnostics that follow. Keep the positioned
			// compiler diagnostics and suppress that duplicate wrapper.
			if strings.HasPrefix(packageError.Msg, "# ") {
				continue
			}
			result = append(result, compilerDiagnostic{
				Package: pkg.PkgPath,
				Pos:     packageError.Pos,
				Message: packageError.Msg,
				pkg:     pkg,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Package != result[j].Package {
			return result[i].Package < result[j].Package
		}
		if result[i].Pos != result[j].Pos {
			return result[i].Pos < result[j].Pos
		}
		return result[i].Message < result[j].Message
	})
	return result
}

type classifiedDiagnostic struct {
	compilerDiagnostic
	classification DiagnosticClassification
}

// compareCompilerDiagnostics uses exact identity first. On a dirty baseline, a
// same-package/same-message diagnostic that moved is deliberately uncertain:
// it may be a shifted old diagnostic or an additional occurrence.
func compareCompilerDiagnostics(baseline, changed []compilerDiagnostic) []classifiedDiagnostic {
	matched := make([]bool, len(baseline))
	var result []classifiedDiagnostic
	for _, diagnostic := range changed {
		exact := -1
		for index, candidate := range baseline {
			if !matched[index] && candidate.Package == diagnostic.Package && candidate.Pos == diagnostic.Pos && candidate.Message == diagnostic.Message {
				exact = index
				break
			}
		}
		if exact >= 0 {
			matched[exact] = true
			continue
		}
		classification := DiagnosticNew
		for index, candidate := range baseline {
			if !matched[index] && candidate.Package == diagnostic.Package && normalizedDiagnostic(candidate.Message) == normalizedDiagnostic(diagnostic.Message) {
				matched[index] = true
				classification = DiagnosticUncertain
				break
			}
		}
		result = append(result, classifiedDiagnostic{compilerDiagnostic: diagnostic, classification: classification})
	}
	return result
}

func normalizedDiagnostic(message string) string {
	return strings.Join(strings.Fields(message), " ")
}

func (a *Analysis) compilerConsequences(diagnostics []classifiedDiagnostic) []CompilerConsequence {
	consequences := make([]CompilerConsequence, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		consequence := CompilerConsequence{
			Package:        graph.PackageRef(diagnostic.Package),
			Message:        diagnostic.Message,
			Classification: diagnostic.classification,
		}
		consequence.Location, consequence.Symbol = a.attributeDiagnostic(diagnostic.compilerDiagnostic)
		consequences = append(consequences, consequence)
	}
	sort.Slice(consequences, func(i, j int) bool {
		left, right := consequences[i], consequences[j]
		if left.Package != right.Package {
			return left.Package < right.Package
		}
		if left.Location.File != right.Location.File {
			return left.Location.File < right.Location.File
		}
		if left.Location.Offset != right.Location.Offset {
			return left.Location.Offset < right.Location.Offset
		}
		if left.Message != right.Message {
			return left.Message < right.Message
		}
		return left.Classification < right.Classification
	})
	return consequences
}

func (a *Analysis) attributeDiagnostic(diagnostic compilerDiagnostic) (graph.Location, *query.SymbolSummary) {
	filename, line, column, ok := parseDiagnosticPosition(diagnostic.Pos)
	if !ok || diagnostic.pkg == nil {
		return graph.Location{}, nil
	}
	filename, _ = filepath.Abs(filename)
	for _, source := range orderedFiles(diagnostic.pkg) {
		sourceName, _ := filepath.Abs(source.name)
		if filepath.Clean(sourceName) != filepath.Clean(filename) {
			continue
		}
		tokenFile := diagnostic.pkg.Fset.File(source.file.Pos())
		if tokenFile == nil || line < 1 || line > tokenFile.LineCount() {
			return graph.Location{File: filename}, nil
		}
		position := tokenFile.LineStart(line) + token.Pos(max(column-1, 0))
		location := sourceLocation(diagnostic.pkg.Fset, position)
		for _, declaration := range source.file.Decls {
			function, isFunction := declaration.(*ast.FuncDecl)
			if !isFunction || position < function.Pos() || position > function.End() {
				continue
			}
			ref := graph.ChildRef(graph.PackageRef(diagnostic.Package), function.Name.Name)
			if function.Recv != nil {
				receiver := receiverName(function.Recv)
				ref = graph.ChildRef(graph.ChildRef(graph.PackageRef(diagnostic.Package), receiver), function.Name.Name)
			}
			if node, exists := a.graph.NodeByRef(ref); exists {
				summary := a.symbolSummary(node.ID)
				return location, &summary
			}
			return location, nil
		}
		return location, nil
	}
	return graph.Location{File: filename}, nil
}

func parseDiagnosticPosition(position string) (string, int, int, bool) {
	last := strings.LastIndex(position, ":")
	if last < 0 {
		return "", 0, 0, false
	}
	column, err := strconv.Atoi(position[last+1:])
	if err != nil {
		return "", 0, 0, false
	}
	before := position[:last]
	second := strings.LastIndex(before, ":")
	if second < 0 {
		return "", 0, 0, false
	}
	line, err := strconv.Atoi(before[second+1:])
	if err != nil {
		return "", 0, 0, false
	}
	return before[:second], line, column, true
}
