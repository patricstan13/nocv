package goanalyzer

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"nocv/graph"
	"nocv/query"
)

// SignatureChangeImpact combines precise local semantic checks with the Go
// compiler's diagnostics for one hypothetical callable signature.
type SignatureChangeImpact struct {
	Callable   graph.SymbolRef
	Before     CallableSignature
	After      CallableSignature
	CallSites  []CallSiteImpact
	Compiler   CompilerImpact
	Contracts  []ContractImpact
	Structural []StructuralImpact
}

// CompilerImpact is the compiler-observed portion of a signature change. Its
// affected scope is the changed package's real reverse import closure.
type CompilerImpact struct {
	AffectedPackages []graph.SymbolRef
	Consequences     []CompilerConsequence
	BaselineStatus   BaselineStatus
}

type CompilerConsequence struct {
	Package        graph.SymbolRef
	Symbol         *query.SymbolSummary
	Location       graph.Location
	Message        string
	Classification DiagnosticClassification
}

type BaselineStatus uint8

const (
	BaselineUnknown BaselineStatus = iota
	BaselineClean
	BaselineHasDiagnostics
)

func (s BaselineStatus) String() string {
	switch s {
	case BaselineClean:
		return "clean"
	case BaselineHasDiagnostics:
		return "has diagnostics"
	default:
		return "unknown"
	}
}

type DiagnosticClassification uint8

const (
	DiagnosticUnknown DiagnosticClassification = iota
	DiagnosticNew
	DiagnosticUncertain
)

func (c DiagnosticClassification) String() string {
	switch c {
	case DiagnosticNew:
		return "new"
	case DiagnosticUncertain:
		return "uncertain"
	default:
		return "unknown"
	}
}

type compilerDiagnostic struct {
	Package string
	Pos     string
	Message string
	pkg     *packages.Package
}

func (a *Analysis) compilerImpact(callable resolvedCallable, proposed resolvedSignature) (CompilerImpact, error) {
	paths := affectedPackagePaths(a.packages, callable.pkg.PkgPath)
	result := CompilerImpact{AffectedPackages: make([]graph.SymbolRef, 0, len(paths))}
	for _, path := range paths {
		result.AffectedPackages = append(result.AffectedPackages, graph.PackageRef(path))
	}

	allowed := make(map[string]bool, len(paths))
	for _, path := range paths {
		allowed[path] = true
	}
	baseline := collectCompilerDiagnostics(a.packages, allowed)
	if len(baseline) == 0 {
		result.BaselineStatus = BaselineClean
	} else {
		result.BaselineStatus = BaselineHasDiagnostics
	}

	filename, contents, err := signatureOverlay(callable, proposed)
	if err != nil {
		return CompilerImpact{}, err
	}
	changed, err := packages.Load(&packages.Config{
		Context: context.Background(),
		Dir:     a.loadDir,
		Mode:    a.loadMode,
		Tests:   false,
		Overlay: map[string][]byte{filename: contents},
	}, paths...)
	if err != nil {
		return CompilerImpact{}, fmt.Errorf("recheck hypothetical signature: %w", err)
	}
	for _, diagnostic := range compareCompilerDiagnostics(baseline, collectCompilerDiagnostics(changed, allowed)) {
		consequence := CompilerConsequence{
			Package:        graph.PackageRef(diagnostic.Package),
			Message:        diagnostic.Message,
			Classification: diagnostic.classification,
		}
		consequence.Location, consequence.Symbol = a.attributeDiagnostic(diagnostic.compilerDiagnostic)
		result.Consequences = append(result.Consequences, consequence)
	}
	sort.Slice(result.Consequences, func(i, j int) bool {
		left, right := result.Consequences[i], result.Consequences[j]
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
	return result, nil
}

func affectedPackagePaths(pkgs []*packages.Package, changed string) []string {
	known := make(map[string]*packages.Package, len(pkgs))
	for _, pkg := range pkgs {
		known[pkg.PkgPath] = pkg
	}
	reverse := make(map[string][]string)
	for _, pkg := range pkgs {
		for imported := range pkg.Imports {
			if known[imported] != nil {
				reverse[imported] = append(reverse[imported], pkg.PkgPath)
			}
		}
	}
	seen := map[string]bool{changed: true}
	queue := []string{changed}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		for _, importer := range reverse[current] {
			if seen[importer] {
				continue
			}
			seen[importer] = true
			queue = append(queue, importer)
		}
	}
	result := make([]string, 0, len(seen))
	for path := range seen {
		if known[path] != nil {
			result = append(result, path)
		}
	}
	sort.Strings(result)
	return result
}

func signatureOverlay(callable resolvedCallable, proposed resolvedSignature) (string, []byte, error) {
	var functionType *ast.FuncType
	var filename string
	for _, source := range orderedFiles(callable.pkg) {
		for _, candidate := range source.file.Decls {
			function, ok := candidate.(*ast.FuncDecl)
			if ok && callable.pkg.TypesInfo.Defs[function.Name] == callable.function {
				functionType = function.Type
				filename = source.name
				break
			}
		}
		if functionType == nil {
			ast.Inspect(source.file, func(node ast.Node) bool {
				field, ok := node.(*ast.Field)
				if !ok {
					return true
				}
				candidate, ok := field.Type.(*ast.FuncType)
				if !ok {
					return true
				}
				for _, name := range field.Names {
					if callable.pkg.TypesInfo.Defs[name] == callable.function {
						functionType = candidate
						filename = source.name
						return false
					}
				}
				return true
			})
		}
		if functionType != nil {
			break
		}
	}
	if functionType == nil || filename == "" {
		return "", nil, fmt.Errorf("source declaration is unavailable for %s", callable.node.Ref)
	}
	filename, err := filepath.Abs(filename)
	if err != nil {
		return "", nil, fmt.Errorf("resolve declaration file for %s: %w", callable.node.Ref, err)
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		return "", nil, fmt.Errorf("read declaration file for %s: %w", callable.node.Ref, err)
	}
	tokenFile := callable.pkg.Fset.File(functionType.Params.Opening)
	if tokenFile == nil {
		return "", nil, fmt.Errorf("source positions are unavailable for %s", callable.node.Ref)
	}
	start := tokenFile.Offset(functionType.Params.Opening)
	endPos := functionType.Params.End()
	if functionType.Results != nil {
		endPos = functionType.Results.End()
	}
	end := tokenFile.Offset(endPos)
	if start < 0 || end < start || end > len(contents) {
		return "", nil, fmt.Errorf("invalid declaration source range for %s", callable.node.Ref)
	}
	replacement := renderSourceSignature(callable.signature, proposed)
	overlay := make([]byte, 0, len(contents)-(end-start)+len(replacement))
	overlay = append(overlay, contents[:start]...)
	overlay = append(overlay, replacement...)
	overlay = append(overlay, contents[end:]...)
	return filename, overlay, nil
}

func renderSourceSignature(current *types.Signature, proposed resolvedSignature) []byte {
	namedParameters := tupleHasNames(current.Params())
	parameters := make([]string, 0, len(proposed.parameterSources))
	for index, source := range proposed.parameterSources {
		if proposed.model.Variadic && index == len(proposed.parameterSources)-1 {
			source = "..." + source
		}
		name := ""
		if index < current.Params().Len() {
			name = current.Params().At(index).Name()
		} else if namedParameters {
			name = "_"
		}
		if name != "" {
			source = name + " " + source
		}
		parameters = append(parameters, source)
	}
	text := "(" + strings.Join(parameters, ", ") + ")"
	namedResults := tupleHasNames(current.Results())
	results := make([]string, 0, len(proposed.resultSources))
	for index, source := range proposed.resultSources {
		name := ""
		if index < current.Results().Len() {
			name = current.Results().At(index).Name()
		} else if namedResults {
			name = "_"
		}
		if name != "" {
			source = name + " " + source
		}
		results = append(results, source)
	}
	if len(results) == 1 && currentResultName(current, 0) == "" {
		text += " " + results[0]
	} else if len(results) != 0 {
		text += " (" + strings.Join(results, ", ") + ")"
	}
	return []byte(text)
}

func tupleHasNames(tuple *types.Tuple) bool {
	for index := 0; index < tuple.Len(); index++ {
		if tuple.At(index).Name() != "" {
			return true
		}
	}
	return false
}

func currentResultName(current *types.Signature, index int) string {
	if index >= current.Results().Len() {
		return ""
	}
	return current.Results().At(index).Name()
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
