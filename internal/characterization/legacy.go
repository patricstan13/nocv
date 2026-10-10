package characterization

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"nocv/goanalyzer"
	"nocv/graph"
)

// FromLegacy converts a legacy Go analysis into the semantic comparison
// surface. Root must be the project root used to run the analysis.
func FromLegacy(root string, analysis *goanalyzer.Analysis) (NormalizedSnapshot, error) {
	if analysis == nil {
		return NormalizedSnapshot{}, fmt.Errorf("normalize legacy analysis: analysis is nil")
	}
	if root == "" {
		return NormalizedSnapshot{}, fmt.Errorf("normalize legacy analysis: root is empty")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return NormalizedSnapshot{}, fmt.Errorf("normalize legacy analysis root: %w", err)
	}

	legacyGraph := analysis.Graph()
	if legacyGraph == nil {
		return NormalizedSnapshot{}, fmt.Errorf("normalize legacy analysis: graph is nil")
	}

	symbols, err := normalizeSymbols(legacyGraph)
	if err != nil {
		return NormalizedSnapshot{}, err
	}
	facts, err := normalizeFacts(root, legacyGraph)
	if err != nil {
		return NormalizedSnapshot{}, err
	}
	normalizedAnalysis, err := normalizeAnalysis(analysis)
	if err != nil {
		return NormalizedSnapshot{}, err
	}

	return NormalizedSnapshot{
		Symbols:  symbols,
		Facts:    facts,
		Analysis: normalizedAnalysis,
	}, nil
}

func normalizeSymbols(legacyGraph *graph.Graph) ([]NormalizedSymbol, error) {
	nodes := legacyGraph.Nodes()
	symbols := make([]NormalizedSymbol, 0, len(nodes))
	for _, node := range nodes {
		kind, err := normalizeSymbolKind(node.Kind)
		if err != nil {
			return nil, fmt.Errorf("normalize symbol %q: %w", node.Ref, err)
		}

		var parentRef graph.SymbolRef
		if node.Parent != 0 {
			parent, ok := legacyGraph.Node(node.Parent)
			if !ok {
				return nil, fmt.Errorf("normalize symbol %q: parent %d does not exist", node.Ref, node.Parent)
			}
			parentRef = parent.Ref
		}
		symbols = append(symbols, NormalizedSymbol{
			SymbolRef: node.Ref,
			Kind:      kind,
			ParentRef: parentRef,
		})
	}
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].SymbolRef < symbols[j].SymbolRef
	})
	return symbols, nil
}

func normalizeFacts(root string, legacyGraph *graph.Graph) ([]NormalizedFact, error) {
	var facts []NormalizedFact
	for _, from := range legacyGraph.Nodes() {
		for _, edge := range legacyGraph.Outgoing(from.ID) {
			fact, include, err := normalizeEdge(root, legacyGraph, edge)
			if err != nil {
				return nil, err
			}
			if include {
				facts = append(facts, fact)
			}
		}
	}
	sort.Slice(facts, func(i, j int) bool {
		if facts[i].FromRef != facts[j].FromRef {
			return facts[i].FromRef < facts[j].FromRef
		}
		if facts[i].Kind != facts[j].Kind {
			return facts[i].Kind < facts[j].Kind
		}
		return facts[i].ToRef < facts[j].ToRef
	})
	return facts, nil
}

func normalizeEdge(root string, legacyGraph *graph.Graph, edge *graph.Edge) (NormalizedFact, bool, error) {
	if edge == nil {
		return NormalizedFact{}, false, fmt.Errorf("normalize legacy fact: edge is nil")
	}
	switch edge.Certainty {
	case graph.RelationshipConfirmed:
	case graph.RelationshipUncertain:
		return NormalizedFact{}, false, nil
	default:
		return NormalizedFact{}, false, fmt.Errorf("normalize legacy fact %d -> %d: invalid certainty %d", edge.From, edge.To, edge.Certainty)
	}

	from, ok := legacyGraph.Node(edge.From)
	if !ok {
		return NormalizedFact{}, false, fmt.Errorf("normalize legacy fact: source %d does not exist", edge.From)
	}
	to, ok := legacyGraph.Node(edge.To)
	if !ok {
		return NormalizedFact{}, false, fmt.Errorf("normalize legacy fact: target %d does not exist", edge.To)
	}
	kind, err := normalizeFactKind(edge.Kind)
	if err != nil {
		return NormalizedFact{}, false, fmt.Errorf("normalize legacy fact %q -> %q: %w", from.Ref, to.Ref, err)
	}

	fact := NormalizedFact{FromRef: from.Ref, Kind: kind, ToRef: to.Ref}
	// Legacy implementation locations identify declarations, not the semantic
	// proof used by Go's type system, so they are deliberately omitted.
	if edge.Kind == graph.EdgeImplements {
		fact.Evidence = []Evidence{}
		return fact, true, nil
	}

	fact.Evidence = make([]Evidence, 0, len(edge.Evidence))
	for _, location := range edge.Evidence {
		evidence, err := normalizeSourceEvidence(root, location)
		if err != nil {
			return NormalizedFact{}, false, fmt.Errorf("normalize legacy fact %q -> %q: %w", from.Ref, to.Ref, err)
		}
		fact.Evidence = append(fact.Evidence, evidence)
	}
	sort.Slice(fact.Evidence, func(i, j int) bool {
		left := fact.Evidence[i].(SourceEvidence)
		right := fact.Evidence[j].(SourceEvidence)
		if left.FileRef != right.FileRef {
			return left.FileRef < right.FileRef
		}
		return left.StartOffset < right.StartOffset
	})
	return fact, true, nil
}

func normalizeSourceEvidence(root string, location graph.Location) (SourceEvidence, error) {
	if location.File == "" {
		return SourceEvidence{}, fmt.Errorf("source evidence file is empty")
	}
	file := location.File
	if !filepath.IsAbs(file) {
		file = filepath.Join(root, file)
	}
	file, err := filepath.Abs(file)
	if err != nil {
		return SourceEvidence{}, fmt.Errorf("resolve source evidence file %q: %w", location.File, err)
	}
	relative, err := filepath.Rel(root, file)
	if err != nil {
		return SourceEvidence{}, fmt.Errorf("make source evidence file %q relative to root: %w", location.File, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return SourceEvidence{}, fmt.Errorf("source evidence file %q is outside analysis root %q", location.File, root)
	}
	return SourceEvidence{
		FileRef:     filepath.ToSlash(relative),
		StartOffset: location.Offset,
	}, nil
}

func normalizeSymbolKind(kind graph.NodeKind) (SymbolKind, error) {
	switch kind {
	case graph.NodePackage:
		return SymbolPackage, nil
	case graph.NodeStruct:
		return SymbolStruct, nil
	case graph.NodeInterface:
		return SymbolInterface, nil
	case graph.NodeFunction:
		return SymbolFunction, nil
	default:
		return "", fmt.Errorf("unsupported node kind %d", kind)
	}
}

func normalizeFactKind(kind graph.EdgeKind) (FactKind, error) {
	switch kind {
	case graph.EdgeCalls:
		return FactCalls, nil
	case graph.EdgeImplements:
		return FactImplements, nil
	case graph.EdgeEmbeds:
		return FactEmbeds, nil
	case graph.EdgeAccepts:
		return FactAccepts, nil
	case graph.EdgeReturns:
		return FactReturns, nil
	case graph.EdgeFieldType:
		return FactFieldType, nil
	case graph.EdgeImports:
		return FactImports, nil
	default:
		return "", fmt.Errorf("unsupported edge kind %d", kind)
	}
}

func normalizeAnalysis(analysis *goanalyzer.Analysis) (NormalizedAnalysis, error) {
	var status AnalysisStatus
	switch analysis.Status() {
	case goanalyzer.AnalysisComplete:
		status = AnalysisComplete
	case goanalyzer.AnalysisPartial:
		status = AnalysisPartial
	default:
		return NormalizedAnalysis{}, fmt.Errorf("normalize legacy analysis: unsupported status %d", analysis.Status())
	}

	seen := make(map[NormalizedAnalysisReason]struct{})
	for _, legacyReason := range analysis.StatusReasons() {
		var kind AnalysisReasonKind
		switch legacyReason.Kind {
		case goanalyzer.AnalysisIncompleteTypeInformation:
			kind = ReasonIncompleteTypeInformation
		default:
			return NormalizedAnalysis{}, fmt.Errorf("normalize legacy analysis: unsupported reason kind %d", legacyReason.Kind)
		}
		seen[NormalizedAnalysisReason{Kind: kind, PackageRef: legacyReason.Package}] = struct{}{}
	}

	reasons := make([]NormalizedAnalysisReason, 0, len(seen))
	for reason := range seen {
		reasons = append(reasons, reason)
	}
	sort.Slice(reasons, func(i, j int) bool {
		if reasons[i].Kind != reasons[j].Kind {
			return reasons[i].Kind < reasons[j].Kind
		}
		return reasons[i].PackageRef < reasons[j].PackageRef
	})
	if status == AnalysisComplete && len(reasons) != 0 {
		return NormalizedAnalysis{}, fmt.Errorf("normalize legacy analysis: complete status has reasons")
	}
	if status == AnalysisPartial && len(reasons) == 0 {
		return NormalizedAnalysis{}, fmt.Errorf("normalize legacy analysis: partial status has no reasons")
	}
	return NormalizedAnalysis{Status: status, Reasons: reasons}, nil
}
