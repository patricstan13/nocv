package goanalyzer

import (
	"go/types"
	"sort"

	"nocv/graph"
	"nocv/query"
)

// structuralImpacts reports concrete named types whose value method set
// contains the selected declaration through promotion. Pointer-only modeled
// types are not synthesized; embedding *T naturally contributes *T's methods
// to the embedding named type's method set according to go/types.
func (a *Analysis) structuralImpacts(callable resolvedCallable, proposed resolvedSignature) []query.StructuralImpact {
	parent, exists := a.graph.Node(callable.node.Parent)
	if !exists || parent.Kind != graph.NodeStruct {
		return nil
	}
	if !parameterSignatureChanged(callable.signature, proposed) {
		return nil
	}

	var result []query.StructuralImpact
	for _, candidateID := range a.embeddingCandidates(parent.ID) {
		named := a.symbols.namedTypes[candidateID]
		if named == nil || hasTypeParameters(named) {
			continue
		}
		if !methodSetPromotes(types.NewMethodSet(named), callable.function) {
			continue
		}
		result = append(result, query.StructuralImpact{
			Kind:         query.StructuralPromotedMethodChanged,
			Type:         a.symbolSummary(candidateID),
			OriginMethod: a.symbolSummary(callable.node.ID),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Type.Ref != result[j].Type.Ref {
			return result[i].Type.Ref < result[j].Type.Ref
		}
		return result[i].OriginMethod.Ref < result[j].OriginMethod.Ref
	})
	return result
}

// embeddingCandidates follows reverse concrete Embeds edges from the origin
// type. The visited set both deduplicates multiple paths and prevents cycles.
func (a *Analysis) embeddingCandidates(origin graph.NodeID) []graph.NodeID {
	visited := map[graph.NodeID]bool{origin: true}
	queue := []graph.NodeID{origin}
	var result []graph.NodeID
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range a.graph.Incoming(current, graph.EdgeEmbeds) {
			candidate, exists := a.graph.Node(edge.From)
			if !exists || candidate.Kind != graph.NodeStruct || visited[candidate.ID] {
				continue
			}
			visited[candidate.ID] = true
			result = append(result, candidate.ID)
			queue = append(queue, candidate.ID)
		}
	}
	return result
}

func methodSetPromotes(methodSet *types.MethodSet, origin *types.Func) bool {
	selection := methodSet.Lookup(origin.Pkg(), origin.Name())
	return selection != nil && selection.Obj() == origin && len(selection.Index()) > 1
}

func parameterSignatureChanged(current *types.Signature, proposed resolvedSignature) bool {
	hypothetical := signatureWithParameters(current, current.Recv(), proposed)
	return !types.Identical(current, hypothetical)
}
