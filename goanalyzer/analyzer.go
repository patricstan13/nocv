// Package goanalyzer discovers Go declarations and maps them into the NOCV graph.
package goanalyzer

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
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
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax,
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
	for _, pkg := range pkgs {
		if err := addPackage(g, pkg); err != nil {
			return nil, fmt.Errorf("analyze package %s: %w", pkg.PkgPath, err)
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

func addPackage(g *graph.Graph, pkg *packages.Package) error {
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
				if err := addType(g, pkg.Fset, pkgID, typeSpec); err != nil {
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
			if err := addFunction(g, pkg.Fset, parent, fn.Name.Name, fn.Name.Pos()); err != nil {
				return err
			}
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

func addType(g *graph.Graph, fset *token.FileSet, pkgID graph.SymbolID, spec *ast.TypeSpec) error {
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
		Location: sourceLocation(fset, spec.Name.Pos()),
	}); err != nil {
		return err
	}

	if iface, ok := spec.Type.(*ast.InterfaceType); ok {
		for _, field := range iface.Methods.List {
			if _, ok := field.Type.(*ast.FuncType); !ok {
				continue // embedded interface/type terms are not structural nodes
			}
			for _, name := range field.Names {
				if err := addFunction(g, fset, typeID, name.Name, name.Pos()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func addFunction(g *graph.Graph, fset *token.FileSet, parent graph.SymbolID, name string, pos token.Pos) error {
	return g.AddNode(graph.Node{
		ID:       graph.ChildID(parent, name),
		Kind:     graph.NodeFunction,
		Name:     name,
		Parent:   parent,
		Location: sourceLocation(fset, pos),
	})
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
