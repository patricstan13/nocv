package goanalyzer

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"sort"

	"golang.org/x/tools/go/packages"

	"nocv/graph"
	"nocv/query"
)

type resolvedCallable struct {
	function  *types.Func
	signature *types.Signature
	pkg       *packages.Package
	node      *graph.Node
}

type resolvedSignature struct {
	model            CallableSignature
	parameterTypes   []types.Type // a variadic final parameter is stored as its element type
	resultTypes      []types.Type
	parameterSources []string
	resultSources    []string
}

type compilerCallSite struct {
	pkg      *packages.Package
	callerID graph.NodeID
	call     *ast.CallExpr
}

type actualArgument struct {
	typeAndValue types.TypeAndValue
}

// CallableSignature returns the compiler-extracted callable signature for a
// modeled function or method without exposing compiler objects. Its type
// displays use declaration-context package qualifiers so they can be submitted
// again as proposed Go type expressions.
func (a *Analysis) CallableSignature(callable graph.SymbolRef) (CallableSignature, error) {
	resolved, err := a.resolveCallable(callable)
	if err != nil {
		return CallableSignature{}, err
	}
	return a.editableSignature(resolved), nil
}

func (a *Analysis) editableSignature(callable resolvedCallable) CallableSignature {
	signature := callable.signature
	result := CallableSignature{Variadic: signature.Variadic()}
	for index := 0; index < signature.Params().Len(); index++ {
		variable := signature.Params().At(index)
		typ := variable.Type()
		if signature.Variadic() && index == signature.Params().Len()-1 {
			if slice, ok := types.Unalias(typ).(*types.Slice); ok {
				typ = slice.Elem()
			}
		}
		typeRef := a.goTypeRef(typ)
		typeRef.Display = types.TypeString(typ, func(pkg *types.Package) string {
			if pkg == callable.pkg.Types {
				return ""
			}
			return pkg.Name()
		})
		result.Parameters = append(result.Parameters, Parameter{Name: variable.Name(), Type: typeRef})
	}
	for index := 0; index < signature.Results().Len(); index++ {
		variable := signature.Results().At(index)
		typ := variable.Type()
		typeRef := a.goTypeRef(typ)
		typeRef.Display = types.TypeString(typ, func(pkg *types.Package) string {
			if pkg == callable.pkg.Types {
				return ""
			}
			return pkg.Name()
		})
		result.Results = append(result.Results, Result{Name: variable.Name(), Type: typeRef})
	}
	return result
}

// AnalyzeSignatureChange evaluates a hypothetical parameter/result signature.
// Precise local consequences are combined with a compiler overlay recheck of
// the changed package and its reverse import closure.
func (a *Analysis) AnalyzeSignatureChange(callable graph.SymbolRef, proposed ProposedSignature) (SignatureChangeImpact, error) {
	resolved, err := a.resolveCallable(callable)
	if err != nil {
		return SignatureChangeImpact{}, err
	}
	if hasSignatureTypeParameters(resolved.signature) {
		return SignatureChangeImpact{}, fmt.Errorf("generic callable signature changes are not supported: %s", callable)
	}

	before := a.extractSignature(resolved.signature)
	after, err := a.resolveProposedSignature(resolved, proposed, before.model)
	if err != nil {
		return SignatureChangeImpact{}, err
	}

	result := SignatureChangeImpact{
		Callable: callable,
		Before:   before.model,
		After:    after.model,
	}
	if !parameterPortionIdentical(resolved.signature, after) {
		for _, site := range a.collectCallSites(resolved.function) {
			result.CallSites = append(result.CallSites, a.checkCallSite(site, after))
		}
	}
	if signatureChanged(resolved.signature, after) {
		result.Compiler, err = a.compilerImpact(resolved, after)
		if err != nil {
			return SignatureChangeImpact{}, err
		}
		result.Contracts = a.contractImpacts(resolved, after)
		result.Structural = a.structuralImpacts(resolved, after)
	}
	sort.Slice(result.CallSites, func(i, j int) bool {
		left, right := result.CallSites[i], result.CallSites[j]
		if left.Caller.Ref != right.Caller.Ref {
			return left.Caller.Ref < right.Caller.Ref
		}
		if left.Location.File != right.Location.File {
			return left.Location.File < right.Location.File
		}
		return left.Location.Offset < right.Location.Offset
	})
	return result, nil
}

func (a *Analysis) resolveCallable(ref graph.SymbolRef) (resolvedCallable, error) {
	if a == nil || a.graph == nil || a.symbols == nil {
		return resolvedCallable{}, fmt.Errorf("Go analysis is unavailable")
	}
	node, exists := a.graph.NodeByRef(ref)
	if !exists {
		return resolvedCallable{}, fmt.Errorf("unknown symbol: %s", ref)
	}
	if node.Kind != graph.NodeFunction {
		return resolvedCallable{}, fmt.Errorf("symbol is not a function or method: %s", ref)
	}

	for object, id := range a.symbols.objects {
		if id != node.ID {
			continue
		}
		function, ok := object.(*types.Func)
		if !ok {
			continue
		}
		signature, ok := function.Type().(*types.Signature)
		if !ok {
			continue
		}
		for _, pkg := range a.packages {
			if function.Pkg() == pkg.Types {
				return resolvedCallable{function: function, signature: signature, pkg: pkg, node: node}, nil
			}
		}
	}
	return resolvedCallable{}, fmt.Errorf("compiler signature is unavailable for %s", ref)
}

func hasSignatureTypeParameters(signature *types.Signature) bool {
	return signature.TypeParams().Len() != 0 || signature.RecvTypeParams().Len() != 0
}

func (a *Analysis) extractSignature(signature *types.Signature) resolvedSignature {
	result := resolvedSignature{model: CallableSignature{Variadic: signature.Variadic()}}
	parameters := signature.Params()
	for index := 0; index < parameters.Len(); index++ {
		variable := parameters.At(index)
		typ := variable.Type()
		if signature.Variadic() && index == parameters.Len()-1 {
			if slice, ok := types.Unalias(typ).(*types.Slice); ok {
				typ = slice.Elem()
			}
		}
		result.parameterTypes = append(result.parameterTypes, typ)
		result.model.Parameters = append(result.model.Parameters, Parameter{
			Name: variable.Name(),
			Type: a.goTypeRef(typ),
		})
	}
	results := signature.Results()
	for index := 0; index < results.Len(); index++ {
		variable := results.At(index)
		result.resultTypes = append(result.resultTypes, variable.Type())
		result.model.Results = append(result.model.Results, Result{
			Name: variable.Name(),
			Type: a.goTypeRef(variable.Type()),
		})
	}
	return result
}

func signatureWithProposal(
	original *types.Signature,
	receiver *types.Var,
	proposed resolvedSignature,
) *types.Signature {
	parameters := make([]*types.Var, 0, len(proposed.parameterTypes))
	for index, typ := range proposed.parameterTypes {
		if proposed.model.Variadic && index == len(proposed.parameterTypes)-1 {
			typ = types.NewSlice(typ)
		}
		name := ""
		var pkg *types.Package
		if index < original.Params().Len() {
			parameter := original.Params().At(index)
			name = parameter.Name()
			pkg = parameter.Pkg()
		}
		parameters = append(parameters, types.NewVar(token.NoPos, pkg, name, typ))
	}
	results := make([]*types.Var, 0, len(proposed.resultTypes))
	for index, typ := range proposed.resultTypes {
		name := ""
		var pkg *types.Package
		if index < original.Results().Len() {
			result := original.Results().At(index)
			name = result.Name()
			pkg = result.Pkg()
		}
		results = append(results, types.NewVar(token.NoPos, pkg, name, typ))
	}
	return types.NewSignatureType(
		receiver,
		nil,
		nil,
		types.NewTuple(parameters...),
		types.NewTuple(results...),
		proposed.model.Variadic,
	)
}

func (a *Analysis) resolveProposedSignature(callable resolvedCallable, proposed ProposedSignature, before CallableSignature) (resolvedSignature, error) {
	if proposed.Variadic && len(proposed.Parameters) == 0 {
		return resolvedSignature{}, fmt.Errorf("a variadic signature requires at least one parameter")
	}
	result := resolvedSignature{model: CallableSignature{Variadic: proposed.Variadic}}
	for index, parameter := range proposed.Parameters {
		if parameter.TypeExpr == "" {
			return resolvedSignature{}, fmt.Errorf("proposed parameter %d has an empty type", index+1)
		}
		value, err := types.Eval(callable.pkg.Fset, callable.pkg.Types, callable.function.Pos(), parameter.TypeExpr)
		if err != nil {
			return resolvedSignature{}, fmt.Errorf("resolve proposed parameter %d type %q: %w", index+1, parameter.TypeExpr, err)
		}
		if !value.IsType() || value.Type == nil {
			return resolvedSignature{}, fmt.Errorf("proposed parameter %d expression %q is not a type", index+1, parameter.TypeExpr)
		}
		name := parameter.Name
		if name == "" && index < len(before.Parameters) {
			name = before.Parameters[index].Name
		}
		result.parameterTypes = append(result.parameterTypes, value.Type)
		result.parameterSources = append(result.parameterSources, parameter.TypeExpr)
		result.model.Parameters = append(result.model.Parameters, Parameter{Name: name, Type: a.goTypeRef(value.Type)})
	}
	for index, proposedResult := range proposed.Results {
		if proposedResult.TypeExpr == "" {
			return resolvedSignature{}, fmt.Errorf("proposed result %d has an empty type", index+1)
		}
		value, err := types.Eval(callable.pkg.Fset, callable.pkg.Types, callable.function.Pos(), proposedResult.TypeExpr)
		if err != nil {
			return resolvedSignature{}, fmt.Errorf("resolve proposed result %d type %q: %w", index+1, proposedResult.TypeExpr, err)
		}
		if !value.IsType() || value.Type == nil {
			return resolvedSignature{}, fmt.Errorf("proposed result %d expression %q is not a type", index+1, proposedResult.TypeExpr)
		}
		name := proposedResult.Name
		if name == "" && index < len(before.Results) {
			name = before.Results[index].Name
		}
		result.resultTypes = append(result.resultTypes, value.Type)
		result.resultSources = append(result.resultSources, proposedResult.TypeExpr)
		result.model.Results = append(result.model.Results, Result{Name: name, Type: a.goTypeRef(value.Type)})
	}
	return result, nil
}

func (a *Analysis) goTypeRef(typ types.Type) GoTypeRef {
	if typ == nil {
		return GoTypeRef{}
	}
	result := GoTypeRef{Display: types.TypeString(typ, func(pkg *types.Package) string {
		return pkg.Path()
	})}
	if typeName := directTypeName(typ); typeName != nil {
		if id, represented := a.symbols.objects[typeName]; represented {
			if node, exists := a.graph.Node(id); exists {
				result.Symbol = node.Ref
			}
		}
	}
	return result
}

func (a *Analysis) collectCallSites(target *types.Func) []compilerCallSite {
	var result []compilerCallSite
	for _, pkg := range a.packages {
		for _, source := range orderedFiles(pkg) {
			for _, declaration := range source.file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Body == nil {
					continue
				}
				callerID, represented := a.symbols.objects[pkg.TypesInfo.Defs[function.Name]]
				if !represented {
					continue
				}
				ast.Inspect(function.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					callee, resolved := calledFunction(pkg.TypesInfo, call.Fun)
					if resolved && callee == target {
						result = append(result, compilerCallSite{pkg: pkg, callerID: callerID, call: call})
					}
					return true
				})
			}
		}
	}
	return result
}

func (a *Analysis) checkCallSite(site compilerCallSite, proposed resolvedSignature) CallSiteImpact {
	result := CallSiteImpact{
		Caller:   a.symbolSummary(site.callerID),
		Location: sourceLocation(site.pkg.Fset, site.call.Pos()),
	}
	arguments, known := callArguments(site.pkg, site.call)
	if !known {
		result.Compatibility = CompatibilityUnknown
		result.Problems = []SignatureProblem{{Kind: ProblemUnknown}}
		return result
	}

	if site.call.Ellipsis.IsValid() && !proposed.model.Variadic {
		result.Compatibility = CompatibilityIncompatible
		result.Problems = []SignatureProblem{{Kind: ProblemVariadic}}
		return result
	}
	if problem := argumentCountProblem(len(arguments), site.call.Ellipsis.IsValid(), proposed); problem != nil {
		result.Compatibility = CompatibilityIncompatible
		result.Problems = []SignatureProblem{*problem}
		return result
	}

	result.Compatibility = CompatibilityCompatible
	for index, argument := range arguments {
		expected := expectedArgumentType(index, site.call.Ellipsis.IsValid(), proposed)
		if argument.typeAndValue.Type == nil || expected == nil {
			result.Compatibility = CompatibilityUnknown
			result.Problems = append(result.Problems, SignatureProblem{
				Kind: ProblemUnknown, Argument: index + 1,
			})
			continue
		}
		if !compilerAssignable(argument.typeAndValue, expected) {
			result.Problems = append(result.Problems, SignatureProblem{
				Kind:     ProblemArgumentType,
				Argument: index + 1,
				Expected: a.goTypeRef(expected),
				Actual:   a.goTypeRef(argument.typeAndValue.Type),
			})
		}
	}
	if result.Compatibility == CompatibilityUnknown {
		return result
	}
	if len(result.Problems) != 0 {
		result.Compatibility = CompatibilityIncompatible
	} else {
		result.Compatibility = CompatibilityCompatible
	}
	return result
}

func callArguments(pkg *packages.Package, call *ast.CallExpr) ([]actualArgument, bool) {
	if len(call.Args) == 1 && !call.Ellipsis.IsValid() {
		value, exists := expressionTypeAndValue(pkg, call.Args[0])
		if !exists {
			return nil, false
		}
		if tuple, ok := value.Type.(*types.Tuple); ok {
			result := make([]actualArgument, 0, tuple.Len())
			for index := 0; index < tuple.Len(); index++ {
				result = append(result, actualArgument{typeAndValue: types.TypeAndValue{Type: tuple.At(index).Type()}})
			}
			return result, true
		}
	}
	result := make([]actualArgument, 0, len(call.Args))
	for _, expression := range call.Args {
		value, exists := expressionTypeAndValue(pkg, expression)
		if !exists {
			return nil, false
		}
		result = append(result, actualArgument{typeAndValue: value})
	}
	return result, true
}

func expressionTypeAndValue(pkg *packages.Package, expression ast.Expr) (types.TypeAndValue, bool) {
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	if err := types.CheckExpr(pkg.Fset, pkg.Types, expression.Pos(), expression, info); err != nil {
		return types.TypeAndValue{}, false
	}
	value, exists := info.Types[expression]
	return value, exists && value.Type != nil
}

func argumentCountProblem(actual int, ellipsis bool, proposed resolvedSignature) *SignatureProblem {
	expected := len(proposed.parameterTypes)
	valid := actual == expected
	if proposed.model.Variadic && !ellipsis {
		valid = actual >= expected-1
	}
	if valid {
		return nil
	}
	return &SignatureProblem{
		Kind:          ProblemArgumentCount,
		ExpectedCount: expected,
		ActualCount:   actual,
	}
}

func expectedArgumentType(index int, ellipsis bool, proposed resolvedSignature) types.Type {
	last := len(proposed.parameterTypes) - 1
	if !proposed.model.Variadic || index < last {
		return proposed.parameterTypes[index]
	}
	if ellipsis {
		return types.NewSlice(proposed.parameterTypes[last])
	}
	return proposed.parameterTypes[last]
}

func signatureChanged(current *types.Signature, proposed resolvedSignature) bool {
	return !types.Identical(current, signatureWithProposal(current, current.Recv(), proposed))
}

func parameterPortionIdentical(current *types.Signature, proposed resolvedSignature) bool {
	if current.Variadic() != proposed.model.Variadic || current.Params().Len() != len(proposed.parameterTypes) {
		return false
	}
	for index, typ := range proposed.parameterTypes {
		currentType := current.Params().At(index).Type()
		if current.Variadic() && index == current.Params().Len()-1 {
			if slice, ok := types.Unalias(currentType).(*types.Slice); ok {
				currentType = slice.Elem()
			}
		}
		if !types.Identical(currentType, typ) {
			return false
		}
	}
	return true
}

// compilerAssignable uses go/types' assignability rules and asks a small
// compiler check to enforce representability of untyped constant values.
func compilerAssignable(value types.TypeAndValue, target types.Type) bool {
	if value.Type == nil || target == nil || !types.AssignableTo(value.Type, target) {
		return false
	}
	if value.Value == nil {
		return true
	}
	basic := constantTargetBasic(target, value.Type)
	if basic == nil {
		return true
	}
	return compilerAcceptsConstant(value.Value, basic)
}

func constantTargetBasic(target, source types.Type) *types.Basic {
	typ := types.Unalias(target)
	if named, ok := typ.(*types.Named); ok {
		typ = named.Underlying()
	}
	if basic, ok := typ.(*types.Basic); ok {
		return basic
	}
	if _, ok := typ.Underlying().(*types.Interface); ok {
		defaultType := types.Default(source)
		if basic, ok := types.Unalias(defaultType).Underlying().(*types.Basic); ok {
			return basic
		}
	}
	return nil
}

func compilerAcceptsConstant(value constant.Value, target *types.Basic) bool {
	source := "package impactcheck\nvar _ " + target.Name() + " = " + value.ExactString() + "\n"
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, "impactcheck.go", source, 0)
	if err != nil {
		return false
	}
	_, err = new(types.Config).Check("impactcheck", files, []*ast.File{file}, nil)
	return err == nil
}

func (a *Analysis) symbolSummary(id graph.NodeID) query.SymbolSummary {
	node, exists := a.graph.Node(id)
	if !exists {
		return query.SymbolSummary{}
	}
	result := query.SymbolSummary{Ref: node.Ref, Kind: node.Kind, Name: node.Name}
	if parent, parentExists := a.graph.Node(node.Parent); parentExists {
		result.ParentRef = parent.Ref
		result.ParentName = parent.Name
	}
	return result
}
