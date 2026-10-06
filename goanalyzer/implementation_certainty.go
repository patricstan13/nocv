package goanalyzer

import (
	"fmt"
	"go/types"

	"nocv/graph"
)

func implementationCertainty(candidate types.Type, iface *types.Interface) (graph.RelationshipCertainty, error) {
	if methodSetEstablishesInterface(candidate, iface) {
		return graph.RelationshipConfirmed, nil
	}

	// In ill-typed programs, types.Implements deliberately returns true when
	// an invalid embedded field might supply a missing method. Preserve that
	// useful candidate relationship, but do not present it as established.
	if hasInvalidEmbeddedType(candidate) {
		return graph.RelationshipUncertain, nil
	}

	return graph.RelationshipCertaintyUnknown, fmt.Errorf(
		"types.Implements accepted %s for %s without a complete method-set proof or invalid embedded type",
		candidate,
		iface,
	)
}

func methodSetEstablishesInterface(candidate types.Type, iface *types.Interface) bool {
	methodSet := types.NewMethodSet(candidate)
	for requiredMethod := range iface.Methods() {
		selection := methodSet.Lookup(requiredMethod.Pkg(), requiredMethod.Name())
		if selection == nil || !types.Identical(selection.Obj().Type(), requiredMethod.Type()) {
			return false
		}
	}
	return true
}

func hasInvalidEmbeddedType(candidate types.Type) bool {
	return hasInvalidEmbeddedTypeSeen(candidate, make(map[types.Type]bool))
}

func hasInvalidEmbeddedTypeSeen(candidate types.Type, visited map[types.Type]bool) bool {
	if candidate == nil || visited[candidate] {
		return false
	}
	visited[candidate] = true

	unalias := types.Unalias(candidate)
	if unalias != candidate {
		if visited[unalias] {
			return false
		}
		visited[unalias] = true
	}
	candidate = unalias

	switch candidate := candidate.(type) {
	case *types.Basic:
		return candidate.Kind() == types.Invalid
	case *types.Pointer:
		return hasInvalidEmbeddedTypeSeen(candidate.Elem(), visited)
	case *types.Named:
		return hasInvalidEmbeddedTypeSeen(candidate.Underlying(), visited)
	case *types.Struct:
		for index := range candidate.NumFields() {
			field := candidate.Field(index)
			if field.Embedded() && hasInvalidEmbeddedTypeSeen(field.Type(), visited) {
				return true
			}
		}
	}

	return false
}
