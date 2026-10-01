package goanalyzer

import (
	"go/token"
	"go/types"
	"sort"

	"nocv/graph"
	"nocv/query"
)

func (a *Analysis) contractImpacts(callable resolvedCallable, proposed resolvedSignature) []query.ContractImpact {
	parent, exists := a.graph.Node(callable.node.Parent)
	if !exists {
		return nil
	}

	var impacts []query.ContractImpact
	switch parent.Kind {
	case graph.NodeStruct:
		impacts = a.contractsForConcreteMethod(callable, proposed)
	case graph.NodeInterface:
		impacts = a.contractsForInterfaceMethod(callable, proposed)
	default:
		return nil
	}

	impacts = uniqueContractImpacts(impacts)
	sort.Slice(impacts, func(i, j int) bool {
		left, right := impacts[i], impacts[j]
		if left.Concrete.Ref != right.Concrete.Ref {
			return left.Concrete.Ref < right.Concrete.Ref
		}
		if left.Interface.Ref != right.Interface.Ref {
			return left.Interface.Ref < right.Interface.Ref
		}
		if left.ConcreteMethod.Ref != right.ConcreteMethod.Ref {
			return left.ConcreteMethod.Ref < right.ConcreteMethod.Ref
		}
		return left.InterfaceMethod.Ref < right.InterfaceMethod.Ref
	})
	return impacts
}

// contractsForConcreteMethod starts with current type-level implementation
// edges from the method's owning struct. Compiler method-set lookup then proves
// that the selected declaration supplies a method required by that contract.
func (a *Analysis) contractsForConcreteMethod(callable resolvedCallable, proposed resolvedSignature) []query.ContractImpact {
	var result []query.ContractImpact
	concreteID := callable.node.Parent
	concreteNamed := a.symbols.namedTypes[concreteID]
	if concreteNamed == nil {
		return nil
	}

	for _, edge := range a.graph.Outgoing(concreteID, graph.EdgeImplements) {
		interfaceType := a.interfaceType(edge.To)
		if interfaceType == nil {
			continue
		}
		implementation, currentlyImplements := implementationType(concreteNamed, interfaceType)
		if !currentlyImplements {
			continue
		}
		methodSet := types.NewMethodSet(implementation)
		for interfaceMethod := range interfaceType.Methods() {
			selection := methodSet.Lookup(interfaceMethod.Pkg(), interfaceMethod.Name())
			if selection == nil || selection.Obj() != callable.function {
				continue
			}
			if hypotheticalMethodImplements(callable, proposed, interfaceMethod, implementation) {
				continue
			}
			result = append(result, query.ContractImpact{
				Kind:            query.ContractImplementationLost,
				Concrete:        a.symbolSummary(concreteID),
				Interface:       a.symbolSummary(edge.To),
				ConcreteMethod:  a.symbolSummary(callable.node.ID),
				InterfaceMethod: a.symbolSummaryForObject(interfaceMethod),
			})
		}
	}
	return result
}

// contractsForInterfaceMethod considers current type-level implementation
// edges whose completed interface method set contains the selected declaration.
// A temporary flattened interface substitutes only the proposed method
// signature, and go/types rechecks satisfaction against the unchanged concrete
// method set.
func (a *Analysis) contractsForInterfaceMethod(callable resolvedCallable, proposed resolvedSignature) []query.ContractImpact {
	var result []query.ContractImpact
	for _, concrete := range a.graph.Nodes() {
		if concrete.Kind != graph.NodeStruct {
			continue
		}
		concreteNamed := a.symbols.namedTypes[concrete.ID]
		if concreteNamed == nil {
			continue
		}
		for _, edge := range a.graph.Outgoing(concrete.ID, graph.EdgeImplements) {
			interfaceType := a.interfaceType(edge.To)
			if interfaceType == nil || !interfaceContainsMethod(interfaceType, callable.function) {
				continue
			}
			implementation, currentlyImplements := implementationType(concreteNamed, interfaceType)
			if !currentlyImplements {
				continue
			}
			hypothetical := interfaceWithSubstitutedMethod(interfaceType, callable.function, proposed)
			if types.Implements(implementation, hypothetical) {
				continue
			}

			concreteMethod := query.SymbolSummary{}
			if selection := types.NewMethodSet(implementation).Lookup(callable.function.Pkg(), callable.function.Name()); selection != nil {
				concreteMethod = a.symbolSummaryForObject(selection.Obj())
			}
			result = append(result, query.ContractImpact{
				Kind:            query.ContractImplementationLost,
				Concrete:        a.symbolSummary(concrete.ID),
				Interface:       a.symbolSummary(edge.To),
				ConcreteMethod:  concreteMethod,
				InterfaceMethod: a.symbolSummary(callable.node.ID),
			})
		}
	}
	return result
}

func (a *Analysis) interfaceType(id graph.NodeID) *types.Interface {
	named := a.symbols.namedTypes[id]
	if named == nil {
		return nil
	}
	iface, ok := named.Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	return iface.Complete()
}

func interfaceContainsMethod(iface *types.Interface, selected *types.Func) bool {
	for method := range iface.Methods() {
		if method == selected {
			return true
		}
	}
	return false
}

func hypotheticalMethodImplements(
	callable resolvedCallable,
	proposed resolvedSignature,
	required *types.Func,
	currentImplementation types.Type,
) bool {
	typeName := types.NewTypeName(token.NoPos, callable.function.Pkg(), "__nocvHypothetical", nil)
	hypotheticalNamed := types.NewNamed(typeName, types.NewStruct(nil, nil), nil)

	receiverType := types.Type(hypotheticalNamed)
	if _, pointerReceiver := types.Unalias(callable.signature.Recv().Type()).(*types.Pointer); pointerReceiver {
		receiverType = types.NewPointer(hypotheticalNamed)
	}
	receiver := types.NewVar(token.NoPos, callable.function.Pkg(), "", receiverType)
	signature := signatureWithParameters(callable.signature, receiver, proposed)
	method := types.NewFunc(token.NoPos, callable.function.Pkg(), callable.function.Name(), signature)
	hypotheticalNamed.AddMethod(method)

	hypotheticalImplementation := types.Type(hypotheticalNamed)
	if _, pointerImplementation := currentImplementation.(*types.Pointer); pointerImplementation {
		hypotheticalImplementation = types.NewPointer(hypotheticalNamed)
	}
	requirement := types.NewInterfaceType([]*types.Func{receiverlessMethod(required)}, nil).Complete()
	return types.Implements(hypotheticalImplementation, requirement)
}

func interfaceWithSubstitutedMethod(
	current *types.Interface,
	selected *types.Func,
	proposed resolvedSignature,
) *types.Interface {
	methods := make([]*types.Func, 0, current.NumMethods())
	for method := range current.Methods() {
		if method == selected {
			original := method.Type().(*types.Signature)
			signature := signatureWithParameters(original, nil, proposed)
			methods = append(methods, types.NewFunc(method.Pos(), method.Pkg(), method.Name(), signature))
			continue
		}
		methods = append(methods, receiverlessMethod(method))
	}
	return types.NewInterfaceType(methods, nil).Complete()
}

func receiverlessMethod(method *types.Func) *types.Func {
	original := method.Type().(*types.Signature)
	signature := types.NewSignatureType(
		nil,
		nil,
		nil,
		original.Params(),
		original.Results(),
		original.Variadic(),
	)
	return types.NewFunc(method.Pos(), method.Pkg(), method.Name(), signature)
}

func (a *Analysis) symbolSummaryForObject(object types.Object) query.SymbolSummary {
	id, represented := a.symbols.objects[object]
	if !represented {
		return query.SymbolSummary{}
	}
	return a.symbolSummary(id)
}

func uniqueContractImpacts(impacts []query.ContractImpact) []query.ContractImpact {
	type key struct {
		concrete graph.SymbolRef
		iface    graph.SymbolRef
	}
	seen := make(map[key]bool, len(impacts))
	result := make([]query.ContractImpact, 0, len(impacts))
	for _, impact := range impacts {
		key := key{
			concrete: impact.Concrete.Ref,
			iface:    impact.Interface.Ref,
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, impact)
	}
	return result
}
