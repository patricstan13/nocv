// Package goanalyzer discovers Go declarations and maps them into the NOCV graph.
package goanalyzer

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"nocv/graph"
)

// Load analyzes the packages matching patterns from dir. Patterns use the same
// syntax as go list; when omitted, the current package is loaded.
func Load(ctx context.Context, dir string, patterns ...string) (*graph.Graph, error) {
	if len(patterns) == 0 {
		patterns = []string{"."}
	}

	pkgs, err := packages.Load(&packages.Config{
		Context: ctx,
		Dir:     dir,
		Mode:    packages.LoadSyntax,
		Tests:   false,
	}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load Go packages: %w", err)
	}
	if err := packageErrors(pkgs); err != nil {
		return nil, err
	}

	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].PkgPath < pkgs[j].PkgPath })
	g := graph.New()
	symbols := newSymbolIndex()
	for _, pkg := range pkgs {
		if err := addPackage(g, pkg, symbols); err != nil {
			return nil, fmt.Errorf("analyze package %s: %w", pkg.PkgPath, err)
		}
	}
	for _, pkg := range pkgs {
		if err := addCalls(g, pkg, symbols); err != nil {
			return nil, fmt.Errorf("analyze calls in package %s: %w", pkg.PkgPath, err)
		}
	}
	for _, pkg := range pkgs {
		if err := addEmbeddings(g, pkg, symbols); err != nil {
			return nil, fmt.Errorf("analyze embeddings in package %s: %w", pkg.PkgPath, err)
		}
	}
	for _, pkg := range pkgs {
		if err := addSignatureRelationships(g, pkg, symbols); err != nil {
			return nil, fmt.Errorf("analyze signatures in package %s: %w", pkg.PkgPath, err)
		}
	}
	if err := addImplementations(g, symbols); err != nil {
		return nil, fmt.Errorf("analyze interface implementations: %w", err)
	}
	return g, nil
}

func packageErrors(pkgs []*packages.Package) error {
	var messages []string
	for _, pkg := range pkgs {
		for _, err := range pkg.Errors {
			messages = append(messages, err.Error())
		}
	}
	if len(messages) == 0 {
		return nil
	}
	sort.Strings(messages)
	return fmt.Errorf("load Go packages:\n%s", strings.Join(messages, "\n"))
}

type sourceFile struct {
	name string
	file *ast.File
}

type symbolIndex struct {
	objects    map[types.Object]graph.SymbolID
	namedTypes map[graph.SymbolID]*types.Named
}

func newSymbolIndex() *symbolIndex {
	return &symbolIndex{
		objects:    make(map[types.Object]graph.SymbolID),
		namedTypes: make(map[graph.SymbolID]*types.Named),
	}
}

func addPackage(g *graph.Graph, pkg *packages.Package, symbols *symbolIndex) error {
	files := orderedFiles(pkg)
	pkgID := graph.PackageID(pkg.PkgPath)
	location := graph.Location{}
	if len(files) > 0 {
		location = sourceLocation(pkg.Fset, files[0].file.Package)
	}
	if err := g.AddNode(graph.Node{
		ID:       pkgID,
		Kind:     graph.NodePackage,
		Name:     pkg.Name,
		Location: location,
	}); err != nil {
		return err
	}

	// Add named types before receiver methods so every method parent exists.
	for _, source := range files {
		for _, decl := range source.file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				if typeSpec.Assign.IsValid() { // aliases are not new structural declarations
					continue
				}
				if err := addType(g, pkg, pkgID, typeSpec, symbols); err != nil {
					return err
				}
			}
		}
	}

	for _, source := range files {
		for _, decl := range source.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			parent := pkgID
			if fn.Recv != nil {
				receiver := receiverName(fn.Recv)
				candidate := graph.ChildID(pkgID, receiver)
				parentNode, exists := g.Node(candidate)
				if receiver == "" || !exists || parentNode.Kind != graph.NodeStruct {
					// The current model has no parent kind for methods on named
					// scalar, slice, map, or other non-struct types.
					continue
				}
				parent = candidate
			}
			functionID, err := addFunction(g, pkg.Fset, parent, fn.Name.Name, fn.Name.Pos())
			if err != nil {
				return err
			}
			registerSymbol(symbols.objects, pkg.TypesInfo.Defs[fn.Name], functionID)
		}
	}
	return nil
}

func orderedFiles(pkg *packages.Package) []sourceFile {
	files := make([]sourceFile, 0, len(pkg.Syntax))
	for i, file := range pkg.Syntax {
		name := pkg.Fset.PositionFor(file.Pos(), false).Filename
		if i < len(pkg.CompiledGoFiles) {
			name = pkg.CompiledGoFiles[i]
		}
		files = append(files, sourceFile{name: name, file: file})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files
}

func addType(
	g *graph.Graph,
	pkg *packages.Package,
	pkgID graph.SymbolID,
	spec *ast.TypeSpec,
	symbols *symbolIndex,
) error {
	var kind graph.NodeKind
	switch spec.Type.(type) {
	case *ast.StructType:
		kind = graph.NodeStruct
	case *ast.InterfaceType:
		kind = graph.NodeInterface
	default:
		return nil
	}

	typeID := graph.ChildID(pkgID, spec.Name.Name)
	if err := g.AddNode(graph.Node{
		ID:       typeID,
		Kind:     kind,
		Name:     spec.Name.Name,
		Parent:   pkgID,
		Location: sourceLocation(pkg.Fset, spec.Name.Pos()),
	}); err != nil {
		return err
	}
	if typeName, ok := pkg.TypesInfo.Defs[spec.Name].(*types.TypeName); ok {
		registerSymbol(symbols.objects, typeName, typeID)
		if named, ok := typeName.Type().(*types.Named); ok {
			symbols.namedTypes[typeID] = named
		}
	}

	if iface, ok := spec.Type.(*ast.InterfaceType); ok {
		for _, field := range iface.Methods.List {
			if _, ok := field.Type.(*ast.FuncType); !ok {
				continue // embedded interface/type terms are not structural nodes
			}
			for _, name := range field.Names {
				functionID, err := addFunction(g, pkg.Fset, typeID, name.Name, name.Pos())
				if err != nil {
					return err
				}
				registerSymbol(symbols.objects, pkg.TypesInfo.Defs[name], functionID)
			}
		}
	}
	return nil
}

func addFunction(g *graph.Graph, fset *token.FileSet, parent graph.SymbolID, name string, pos token.Pos) (graph.SymbolID, error) {
	id := graph.ChildID(parent, name)
	err := g.AddNode(graph.Node{
		ID:       id,
		Kind:     graph.NodeFunction,
		Name:     name,
		Parent:   parent,
		Location: sourceLocation(fset, pos),
	})
	return id, err
}

func registerSymbol(symbols map[types.Object]graph.SymbolID, object types.Object, id graph.SymbolID) {
	if object != nil {
		symbols[object] = id
	}
}

func addCalls(g *graph.Graph, pkg *packages.Package, symbols *symbolIndex) error {
	for _, source := range orderedFiles(pkg) {
		for _, decl := range source.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			callerID, ok := symbols.objects[pkg.TypesInfo.Defs[fn.Name]]
			if !ok {
				continue
			}
			if err := addLexicalCalls(g, pkg, symbols, callerID, fn.Body); err != nil {
				return err
			}
		}
	}
	return nil
}

// addLexicalCalls attributes every resolvable call in body to the fixed
// declared caller, including calls nested inside any depth of function literal.
func addLexicalCalls(
	g *graph.Graph,
	pkg *packages.Package,
	symbols *symbolIndex,
	callerID graph.SymbolID,
	body *ast.BlockStmt,
) error {
	var visitErr error
	ast.Inspect(body, func(node ast.Node) bool {
		if visitErr != nil {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		target, ok := calledFunction(pkg.TypesInfo, call.Fun)
		if !ok {
			return true
		}
		calleeID, represented := symbols.objects[target]
		if !represented {
			return true
		}
		visitErr = g.AddEdge(graph.Edge{
			From:     callerID,
			To:       calleeID,
			Kind:     graph.EdgeCalls,
			Evidence: []graph.Location{sourceLocation(pkg.Fset, call.Pos())},
		})
		return visitErr == nil
	})
	return visitErr
}

func addEmbeddings(g *graph.Graph, pkg *packages.Package, symbols *symbolIndex) error {
	for _, source := range orderedFiles(pkg) {
		for _, declaration := range source.file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, specification := range general.Specs {
				typeSpec := specification.(*ast.TypeSpec)
				if typeSpec.Assign.IsValid() {
					continue
				}
				sourceID, represented := symbols.objects[pkg.TypesInfo.Defs[typeSpec.Name]]
				if !represented {
					continue
				}

				var fields *ast.FieldList
				switch declaration := typeSpec.Type.(type) {
				case *ast.StructType:
					fields = declaration.Fields
				case *ast.InterfaceType:
					fields = declaration.Methods
				default:
					continue
				}
				if err := addEmbeddedFields(g, pkg, symbols, sourceID, fields); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func addEmbeddedFields(
	g *graph.Graph,
	pkg *packages.Package,
	symbols *symbolIndex,
	sourceID graph.SymbolID,
	fields *ast.FieldList,
) error {
	if fields == nil {
		return nil
	}
	sourceNode, exists := g.Node(sourceID)
	if !exists {
		return nil
	}
	for _, field := range fields.List {
		if len(field.Names) != 0 {
			continue
		}
		typeName := directTypeName(pkg.TypesInfo.TypeOf(field.Type))
		if typeName == nil {
			continue
		}
		targetID, represented := symbols.objects[typeName]
		if !represented {
			continue
		}
		targetNode, targetExists := g.Node(targetID)
		if !targetExists || sourceNode.Kind != targetNode.Kind ||
			(sourceNode.Kind != graph.NodeStruct && sourceNode.Kind != graph.NodeInterface) {
			continue
		}
		if err := g.AddEdge(graph.Edge{
			From:     sourceID,
			To:       targetID,
			Kind:     graph.EdgeEmbeds,
			Evidence: []graph.Location{sourceLocation(pkg.Fset, field.Type.Pos())},
		}); err != nil {
			return err
		}
	}
	return nil
}

func addSignatureRelationships(g *graph.Graph, pkg *packages.Package, symbols *symbolIndex) error {
	for _, source := range orderedFiles(pkg) {
		for _, declaration := range source.file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				function, ok := pkg.TypesInfo.Defs[declaration.Name].(*types.Func)
				if !ok {
					continue
				}
				functionID, represented := symbols.objects[function]
				if !represented {
					continue
				}
				if _, ok := function.Type().(*types.Signature); !ok {
					continue
				}
				if err := addSignatureFieldRelationships(g, pkg, symbols, functionID, graph.EdgeAccepts, declaration.Type.Params); err != nil {
					return err
				}
				if err := addSignatureFieldRelationships(g, pkg, symbols, functionID, graph.EdgeReturns, declaration.Type.Results); err != nil {
					return err
				}

			case *ast.GenDecl:
				if declaration.Tok != token.TYPE {
					continue
				}
				for _, specification := range declaration.Specs {
					typeSpec := specification.(*ast.TypeSpec)
					interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
					if !ok {
						continue
					}
					for _, field := range interfaceType.Methods.List {
						functionType, ok := field.Type.(*ast.FuncType)
						if !ok {
							continue
						}
						for _, name := range field.Names {
							function, ok := pkg.TypesInfo.Defs[name].(*types.Func)
							if !ok {
								continue
							}
							functionID, represented := symbols.objects[function]
							if !represented {
								continue
							}
							if _, ok := function.Type().(*types.Signature); !ok {
								continue
							}
							if err := addSignatureFieldRelationships(g, pkg, symbols, functionID, graph.EdgeAccepts, functionType.Params); err != nil {
								return err
							}
							if err := addSignatureFieldRelationships(g, pkg, symbols, functionID, graph.EdgeReturns, functionType.Results); err != nil {
								return err
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func addSignatureFieldRelationships(
	g *graph.Graph,
	pkg *packages.Package,
	symbols *symbolIndex,
	functionID graph.SymbolID,
	kind graph.EdgeKind,
	fields *ast.FieldList,
) error {
	if fields == nil {
		return nil
	}
	for _, field := range fields.List {
		typeName := directTypeName(pkg.TypesInfo.TypeOf(field.Type))
		if typeName == nil {
			continue
		}
		targetID, represented := symbols.objects[typeName]
		if !represented {
			continue
		}
		targetNode, exists := g.Node(targetID)
		if !exists || (targetNode.Kind != graph.NodeStruct && targetNode.Kind != graph.NodeInterface) {
			continue
		}
		if err := g.AddEdge(graph.Edge{
			From:     functionID,
			To:       targetID,
			Kind:     kind,
			Evidence: []graph.Location{sourceLocation(pkg.Fset, field.Type.Pos())},
		}); err != nil {
			return err
		}
	}
	return nil
}

func directTypeName(typ types.Type) *types.TypeName {
	if typ == nil {
		return nil
	}
	typ = types.Unalias(typ)
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = types.Unalias(pointer.Elem())
	}
	named, ok := typ.(*types.Named)
	if !ok {
		return nil
	}
	return named.Obj()
}

func addImplementations(g *graph.Graph, symbols *symbolIndex) error {
	var structs, interfaces []*graph.Node
	for _, node := range g.Nodes() {
		switch node.Kind {
		case graph.NodeStruct:
			structs = append(structs, node)
		case graph.NodeInterface:
			interfaces = append(interfaces, node)
		}
	}

	for _, structNode := range structs {
		structType := symbols.namedTypes[structNode.ID]
		if structType == nil || hasTypeParameters(structType) {
			continue
		}
		for _, interfaceNode := range interfaces {
			interfaceNamed := symbols.namedTypes[interfaceNode.ID]
			if interfaceNamed == nil || hasTypeParameters(interfaceNamed) {
				continue
			}
			interfaceType, ok := interfaceNamed.Underlying().(*types.Interface)
			if !ok {
				continue
			}
			interfaceType = interfaceType.Complete()
			if interfaceType.NumMethods() == 0 || !interfaceType.IsMethodSet() {
				continue
			}

			concreteType, implements := implementationType(structType, interfaceType)
			if !implements {
				continue
			}
			if err := g.AddEdge(graph.Edge{
				From:     structNode.ID,
				To:       interfaceNode.ID,
				Kind:     graph.EdgeImplements,
				Evidence: []graph.Location{structNode.Location},
			}); err != nil {
				return err
			}
			if err := addMethodImplementations(g, symbols, structNode.ID, interfaceNode.ID, concreteType, interfaceType); err != nil {
				return err
			}
		}
	}
	return nil
}

func implementationType(named *types.Named, iface *types.Interface) (types.Type, bool) {
	if types.Implements(named, iface) {
		return named, true
	}
	pointer := types.NewPointer(named)
	if types.Implements(pointer, iface) {
		return pointer, true
	}
	return nil, false
}

func addMethodImplementations(
	g *graph.Graph,
	symbols *symbolIndex,
	structID graph.SymbolID,
	interfaceID graph.SymbolID,
	concreteType types.Type,
	interfaceType *types.Interface,
) error {
	methodSet := types.NewMethodSet(concreteType)
	for interfaceMethod := range interfaceType.Methods() {
		interfaceMethod := interfaceMethod
		interfaceMethodID, represented := symbols.objects[interfaceMethod]
		if !represented || !isDirectChild(g, interfaceMethodID, interfaceID) {
			continue
		}

		selection := methodSet.Lookup(interfaceMethod.Pkg(), interfaceMethod.Name())
		if selection == nil {
			continue
		}
		concreteMethodID, represented := symbols.objects[selection.Obj()]
		if !represented || !isDirectChild(g, concreteMethodID, structID) {
			// Promoted methods prove the type-level relationship, but their
			// declaration belongs to the embedded type rather than this struct.
			continue
		}
		concreteMethod, _ := g.Node(concreteMethodID)
		if err := g.AddEdge(graph.Edge{
			From:     concreteMethodID,
			To:       interfaceMethodID,
			Kind:     graph.EdgeImplements,
			Evidence: []graph.Location{concreteMethod.Location},
		}); err != nil {
			return err
		}
	}
	return nil
}

func hasTypeParameters(named *types.Named) bool {
	parameters := named.TypeParams()
	return parameters != nil && parameters.Len() > 0
}

func isDirectChild(g *graph.Graph, childID, parentID graph.SymbolID) bool {
	child, ok := g.Node(childID)
	return ok && child.Parent == parentID
}

func calledFunction(info *types.Info, expr ast.Expr) (*types.Func, bool) {
	var object types.Object
	switch expr := expr.(type) {
	case *ast.Ident:
		object = info.Uses[expr]
	case *ast.SelectorExpr:
		if selection := info.Selections[expr]; selection != nil {
			object = selection.Obj()
		} else {
			object = info.Uses[expr.Sel]
		}
	case *ast.ParenExpr:
		return calledFunction(info, expr.X)
	case *ast.IndexExpr:
		return calledFunction(info, expr.X)
	case *ast.IndexListExpr:
		return calledFunction(info, expr.X)
	default:
		return nil, false
	}
	function, ok := object.(*types.Func)
	return function, ok
}

func receiverName(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	return baseTypeName(fields.List[0].Type)
}

func baseTypeName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		return baseTypeName(expr.X)
	case *ast.ParenExpr:
		return baseTypeName(expr.X)
	case *ast.IndexExpr:
		return baseTypeName(expr.X)
	case *ast.IndexListExpr:
		return baseTypeName(expr.X)
	default:
		return ""
	}
}

func sourceLocation(fset *token.FileSet, pos token.Pos) graph.Location {
	position := fset.PositionFor(pos, false)
	return graph.Location{File: position.Filename, Offset: position.Offset}
}
