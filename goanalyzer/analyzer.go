// Package goanalyzer discovers Go declarations and maps them into the NOCV graph.
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
)

// Analysis owns one graph together with the Go compiler state needed by
// language-specific queries. Compiler objects never enter the graph or query
// read models.
type Analysis struct {
	graph         *graph.Graph
	packages      []*packages.Package
	symbols       *symbolIndex
	loadDir       string
	loadMode      packages.LoadMode
	status        AnalysisStatus
	statusReasons []AnalysisStatusReason
}

// Graph returns the language-independent graph produced by this analysis.
func (a *Analysis) Graph() *graph.Graph {
	if a == nil {
		return nil
	}
	return a.graph
}

// Load analyzes the packages matching patterns from dir. Patterns use the same
// syntax as go list; when omitted, the current package is loaded.
func Load(ctx context.Context, dir string, patterns ...string) (*graph.Graph, error) {
	analysis, err := LoadAnalysis(ctx, dir, patterns...)
	if err != nil {
		return nil, err
	}
	return analysis.Graph(), nil
}

// LoadAnalysis loads the graph and retains the compiler state required for
// Go-specific analyses such as hypothetical parameter changes.
func LoadAnalysis(ctx context.Context, dir string, patterns ...string) (*Analysis, error) {
	if len(patterns) == 0 {
		patterns = []string{"."}
	}

	loadDir := dir
	var err error
	if loadDir == "" {
		loadDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve analysis directory: %w", err)
		}
	}
	loadDir, err = filepath.Abs(loadDir)
	if err != nil {
		return nil, fmt.Errorf("resolve analysis directory: %w", err)
	}
	const loadMode = packages.LoadSyntax
	pkgs, err := packages.Load(&packages.Config{
		Context: ctx,
		Dir:     loadDir,
		Mode:    loadMode,
		Tests:   false,
	}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load Go packages: %w", err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("load Go packages: no packages matched %q", strings.Join(patterns, ", "))
	}
	if err := packageErrors(pkgs); err != nil {
		return nil, err
	}
	statusReasons := analysisStatusReasons(pkgs)
	status := AnalysisComplete
	if len(statusReasons) > 0 {
		status = AnalysisPartial
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
		if err := addImports(g, pkg); err != nil {
			return nil, fmt.Errorf("analyze imports in package %s: %w", pkg.PkgPath, err)
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
	return &Analysis{
		graph:         g,
		packages:      pkgs,
		symbols:       symbols,
		loadDir:       loadDir,
		loadMode:      loadMode,
		status:        status,
		statusReasons: statusReasons,
	}, nil
}

func addImports(g *graph.Graph, pkg *packages.Package) error {
	from, represented := g.Resolve(graph.PackageRef(pkg.PkgPath))
	if !represented {
		return nil
	}
	for _, source := range orderedFiles(pkg) {
		for _, spec := range source.file.Imports {
			imported := importedPackage(pkg, spec)
			if imported == nil {
				continue
			}
			to, represented := g.Resolve(graph.PackageRef(imported.PkgPath))
			if !represented {
				continue
			}
			if from == to {
				continue
			}
			target, targetExists := g.Node(to)
			if !targetExists || target.Kind != graph.NodePackage {
				continue
			}
			if err := g.AddEdge(graph.Edge{
				From:     from,
				To:       to,
				Kind:     graph.EdgeImports,
				Evidence: []graph.Location{sourceLocation(pkg.Fset, spec.Pos())},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func importedPackage(pkg *packages.Package, spec *ast.ImportSpec) *packages.Package {
	if pkg == nil || spec == nil || spec.Path == nil {
		return nil
	}
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return nil
	}
	return pkg.Imports[path]
}

func packageErrors(pkgs []*packages.Package) error {
	var messages []string
	for _, pkg := range pkgs {
		for _, err := range pkg.Errors {
			// Retain type-check diagnostics as baseline state for hypothetical
			// compiler rechecks. packages.Load still supplies the partial typed
			// syntax needed by the graph for ordinary declaration-level errors.
			if err.Kind == packages.TypeError || strings.HasPrefix(err.Msg, "# ") {
				continue
			}
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
	objects    map[types.Object]graph.NodeID
	namedTypes map[graph.NodeID]*types.Named
}

func newSymbolIndex() *symbolIndex {
	return &symbolIndex{
		objects:    make(map[types.Object]graph.NodeID),
		namedTypes: make(map[graph.NodeID]*types.Named),
	}
}

func addPackage(g *graph.Graph, pkg *packages.Package, symbols *symbolIndex) error {
	files := orderedFiles(pkg)
	pkgRef := graph.PackageRef(pkg.PkgPath)
	pkgID, err := g.AddNode(graph.Node{
		Ref:           pkgRef,
		Kind:          graph.NodePackage,
		Name:          pkg.Name,
		Documentation: packageDocumentation(files),
	})
	if err != nil {
		return err
	}
	refs := newReferenceAllocator(files, pkgRef)

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
				if err := addType(g, pkg, pkgID, pkgRef, typeSpec, typeDocumentation(gen, typeSpec), symbols, refs); err != nil {
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
			parentRef := pkgRef
			if fn.Recv != nil {
				receiver := receiverName(fn.Recv)
				candidate := graph.ChildRef(pkgRef, receiver)
				parentNode, exists := g.NodeByRef(candidate)
				if receiver == "" || !exists || parentNode.Kind != graph.NodeStruct {
					// The current model has no parent kind for methods on named
					// scalar, slice, map, or other non-struct types.
					continue
				}
				parent = parentNode.ID
				parentRef = parentNode.Ref
			}
			functionID, err := addFunction(g, pkg.Fset, parent, refs.ref(parentRef, fn.Name.Name, fn.Name.Pos()), fn.Name.Name, fn.Name.Pos(), documentation(fn.Doc))
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

// referenceAllocator adds a source-order discriminator only when declarations
// with the same modeled parent and source name would otherwise collide.
type referenceAllocator struct {
	totals   map[string]int
	ordinals map[token.Pos]int
}

func newReferenceAllocator(files []sourceFile, pkgRef graph.SymbolRef) *referenceAllocator {
	allocator := &referenceAllocator{totals: make(map[string]int), ordinals: make(map[token.Pos]int)}
	for _, source := range files {
		for _, declaration := range source.file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				parent := pkgRef
				if declaration.Recv != nil {
					receiver := receiverName(declaration.Recv)
					if receiver == "" {
						continue
					}
					parent = graph.ChildRef(pkgRef, receiver)
				}
				allocator.record(parent, declaration.Name.Name, declaration.Name.Pos())

			case *ast.GenDecl:
				if declaration.Tok != token.TYPE {
					continue
				}
				for _, specification := range declaration.Specs {
					typeSpec := specification.(*ast.TypeSpec)
					if typeSpec.Assign.IsValid() {
						continue
					}
					switch typeSpec.Type.(type) {
					case *ast.StructType, *ast.InterfaceType:
						allocator.record(pkgRef, typeSpec.Name.Name, typeSpec.Name.Pos())
					}
				}
			}
		}
	}
	for _, source := range files {
		for _, declaration := range source.file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, specification := range general.Specs {
				typeSpec := specification.(*ast.TypeSpec)
				iface, ok := typeSpec.Type.(*ast.InterfaceType)
				if !ok || typeSpec.Assign.IsValid() {
					continue
				}
				parent := allocator.ref(pkgRef, typeSpec.Name.Name, typeSpec.Name.Pos())
				for _, field := range iface.Methods.List {
					if _, ok := field.Type.(*ast.FuncType); !ok {
						continue
					}
					for _, name := range field.Names {
						allocator.record(parent, name.Name, name.Pos())
					}
				}
			}
		}
	}
	return allocator
}

func (a *referenceAllocator) record(parent graph.SymbolRef, name string, pos token.Pos) {
	key := referenceKey(parent, name)
	a.totals[key]++
	a.ordinals[pos] = a.totals[key]
}

func (a *referenceAllocator) ref(parent graph.SymbolRef, name string, pos token.Pos) graph.SymbolRef {
	key := referenceKey(parent, name)
	component := name
	if a.totals[key] > 1 {
		component += "#" + strconv.Itoa(a.ordinals[pos])
	}
	return graph.ChildRef(parent, component)
}

func referenceKey(parent graph.SymbolRef, name string) string {
	return string(parent) + "\x00" + name
}

func addType(
	g *graph.Graph,
	pkg *packages.Package,
	pkgID graph.NodeID,
	pkgRef graph.SymbolRef,
	spec *ast.TypeSpec,
	docText string,
	symbols *symbolIndex,
	refs *referenceAllocator,
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

	typeRef := refs.ref(pkgRef, spec.Name.Name, spec.Name.Pos())
	typeID, err := g.AddNode(graph.Node{
		Ref:           typeRef,
		Kind:          kind,
		Name:          spec.Name.Name,
		Parent:        pkgID,
		Location:      sourceLocation(pkg.Fset, spec.Name.Pos()),
		Documentation: docText,
	})
	if err != nil {
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
				functionID, err := addFunction(g, pkg.Fset, typeID, refs.ref(typeRef, name.Name, name.Pos()), name.Name, name.Pos(), documentation(field.Doc))
				if err != nil {
					return err
				}
				registerSymbol(symbols.objects, pkg.TypesInfo.Defs[name], functionID)
			}
		}
	}
	return nil
}

func addFunction(
	g *graph.Graph,
	fset *token.FileSet,
	parent graph.NodeID,
	ref graph.SymbolRef,
	name string,
	pos token.Pos,
	docText string,
) (graph.NodeID, error) {
	id, err := g.AddNode(graph.Node{
		Ref:           ref,
		Kind:          graph.NodeFunction,
		Name:          name,
		Parent:        parent,
		Location:      sourceLocation(fset, pos),
		Documentation: docText,
	})
	return id, err
}

func packageDocumentation(files []sourceFile) string {
	seen := make(map[string]bool)
	var blocks []string
	for _, source := range files {
		text := documentation(source.file.Doc)
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		blocks = append(blocks, text)
	}
	return strings.Join(blocks, "\n\n")
}

func typeDocumentation(declaration *ast.GenDecl, specification *ast.TypeSpec) string {
	if text := documentation(specification.Doc); text != "" {
		return text
	}
	if declaration != nil && !declaration.Lparen.IsValid() {
		return documentation(declaration.Doc)
	}
	return ""
}

func documentation(comments *ast.CommentGroup) string {
	if comments == nil {
		return ""
	}
	return strings.TrimSpace(comments.Text())
}

func registerSymbol(symbols map[types.Object]graph.NodeID, object types.Object, id graph.NodeID) {
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
	callerID graph.NodeID,
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
	sourceID graph.NodeID,
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
	functionID graph.NodeID,
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
	structID graph.NodeID,
	interfaceID graph.NodeID,
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

func isDirectChild(g *graph.Graph, childID, parentID graph.NodeID) bool {
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
