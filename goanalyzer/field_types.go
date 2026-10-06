package goanalyzer

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"

	"nocv/graph"
)

// addFieldRelationships records represented named types that occur in the
// declared types of named struct fields. Embedded fields remain exclusively
// represented by EdgeEmbeds.
func addFieldRelationships(g *graph.Graph, pkg *packages.Package, symbols *symbolIndex) error {
	for _, source := range orderedFiles(pkg) {
		for _, declaration := range source.file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, specification := range general.Specs {
				typeSpec := specification.(*ast.TypeSpec)
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok || typeSpec.Assign.IsValid() {
					continue
				}
				sourceID, represented := symbols.objects[pkg.TypesInfo.Defs[typeSpec.Name]]
				if !represented {
					continue
				}
				for _, field := range structType.Fields.List {
					if len(field.Names) == 0 {
						continue
					}
					for _, targetID := range representedTypesInField(pkg.TypesInfo.TypeOf(field.Type), symbols) {
						if err := g.AddEdge(graph.Edge{
							From:     sourceID,
							To:       targetID,
							Kind:     graph.EdgeFieldType,
							Evidence: []graph.Location{sourceLocation(pkg.Fset, field.Type.Pos())},
						}); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func representedTypesInField(fieldType types.Type, symbols *symbolIndex) []graph.NodeID {
	walker := fieldTypeWalker{
		symbols:     symbols,
		visited:     make(map[types.Type]bool),
		seenTargets: make(map[graph.NodeID]bool),
	}
	walker.walk(fieldType)
	return walker.targets
}

type fieldTypeWalker struct {
	symbols     *symbolIndex
	visited     map[types.Type]bool
	seenTargets map[graph.NodeID]bool
	targets     []graph.NodeID
}

func (w *fieldTypeWalker) walk(typ types.Type) {
	if typ == nil {
		return
	}
	typ = types.Unalias(typ)
	if w.visited[typ] {
		return
	}
	w.visited[typ] = true

	switch typ := typ.(type) {
	case *types.Named:
		if target, represented := w.symbols.objects[typ.Obj()]; represented && !w.seenTargets[target] {
			w.seenTargets[target] = true
			w.targets = append(w.targets, target)
		}
		// Type arguments are part of the field's declared type. The named
		// type's underlying structure is not: expanding it would attribute
		// the target declaration's own fields to the containing struct.
		for index := 0; index < typ.TypeArgs().Len(); index++ {
			w.walk(typ.TypeArgs().At(index))
		}

	case *types.Pointer:
		w.walk(typ.Elem())
	case *types.Slice:
		w.walk(typ.Elem())
	case *types.Array:
		w.walk(typ.Elem())
	case *types.Map:
		w.walk(typ.Key())
		w.walk(typ.Elem())
	case *types.Chan:
		w.walk(typ.Elem())
	case *types.Tuple:
		w.walkTuple(typ)
	case *types.Signature:
		w.walkTuple(typ.Params())
		w.walkTuple(typ.Results())
		w.walkTypeParameters(typ.TypeParams())
	case *types.Struct:
		for index := 0; index < typ.NumFields(); index++ {
			w.walk(typ.Field(index).Type())
		}
	case *types.Interface:
		complete := typ.Complete()
		for index := 0; index < complete.NumEmbeddeds(); index++ {
			w.walk(complete.EmbeddedType(index))
		}
		for index := 0; index < complete.NumExplicitMethods(); index++ {
			w.walk(complete.ExplicitMethod(index).Type())
		}
	case *types.TypeParam:
		w.walk(typ.Constraint())
	case *types.Union:
		for index := 0; index < typ.Len(); index++ {
			w.walk(typ.Term(index).Type())
		}
	}
}

func (w *fieldTypeWalker) walkTuple(tuple *types.Tuple) {
	if tuple == nil {
		return
	}
	for index := 0; index < tuple.Len(); index++ {
		w.walk(tuple.At(index).Type())
	}
}

func (w *fieldTypeWalker) walkTypeParameters(parameters *types.TypeParamList) {
	if parameters == nil {
		return
	}
	for index := 0; index < parameters.Len(); index++ {
		w.walk(parameters.At(index))
	}
}
