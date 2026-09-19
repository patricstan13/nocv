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
	symbols := make(map[types.Object]graph.SymbolID)
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

func addPackage(g *graph.Graph, pkg *packages.Package, symbols map[types.Object]graph.SymbolID) error {
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
			registerSymbol(symbols, pkg.TypesInfo.Defs[fn.Name], functionID)
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
	symbols map[types.Object]graph.SymbolID,
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
				registerSymbol(symbols, pkg.TypesInfo.Defs[name], functionID)
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

func addCalls(g *graph.Graph, pkg *packages.Package, symbols map[types.Object]graph.SymbolID) error {
	for _, source := range orderedFiles(pkg) {
		for _, decl := range source.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			callerID, ok := symbols[pkg.TypesInfo.Defs[fn.Name]]
			if !ok {
				continue
			}

			var visitErr error
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if visitErr != nil {
					return false
				}

				if _, ok := node.(*ast.FuncLit); ok {
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
				calleeID, represented := symbols[target]
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
			if visitErr != nil {
				return visitErr
			}
		}
	}
	return nil
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
