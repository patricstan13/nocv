// Package characterization defines the small semantic comparison surface used
// while replacing the legacy analyzer.
package characterization

import "nocv/graph"

// NormalizedSnapshot is the deterministic semantic output of one analysis.
type NormalizedSnapshot struct {
	Symbols  []NormalizedSymbol
	Facts    []NormalizedFact
	Analysis NormalizedAnalysis
}

// NormalizedSymbol describes a represented declaration without graph-local
// identity or presentation metadata.
type NormalizedSymbol struct {
	SymbolRef graph.SymbolRef
	Kind      SymbolKind
	ParentRef graph.SymbolRef
}

// SymbolKind identifies a semantic declaration category.
type SymbolKind string

const (
	SymbolPackage   SymbolKind = "package"
	SymbolStruct    SymbolKind = "struct"
	SymbolInterface SymbolKind = "interface"
	SymbolFunction  SymbolKind = "function"
)

// NormalizedFact describes one established semantic relationship. Fact
// identity is the tuple (FromRef, Kind, ToRef).
type NormalizedFact struct {
	FromRef  graph.SymbolRef
	Kind     FactKind
	ToRef    graph.SymbolRef
	Evidence []Evidence
}

// FactKind identifies a semantic relationship category.
type FactKind string

const (
	FactCalls      FactKind = "calls"
	FactImplements FactKind = "implements"
	FactEmbeds     FactKind = "embeds"
	FactAccepts    FactKind = "accepts"
	FactReturns    FactKind = "returns"
	FactFieldType  FactKind = "field_type"
	FactImports    FactKind = "imports"
)

// Evidence is a closed set of normalized evidence values. Concrete evidence
// remains heterogeneous so unrelated evidence forms do not share empty fields.
type Evidence interface {
	isEvidence()
}

// SourceEvidence identifies one source occurrence retained by the legacy
// analyzer. FileRef is slash-separated and relative to the analysis root.
type SourceEvidence struct {
	FileRef     string
	StartOffset int
}

func (SourceEvidence) isEvidence() {}

// NormalizedAnalysis describes the completeness of a usable snapshot.
type NormalizedAnalysis struct {
	Status  AnalysisStatus
	Reasons []NormalizedAnalysisReason
}

// AnalysisStatus describes whether all supported facts were available.
type AnalysisStatus string

const (
	AnalysisComplete AnalysisStatus = "complete"
	AnalysisPartial  AnalysisStatus = "partial"
)

// NormalizedAnalysisReason identifies a package-scoped completeness reason.
type NormalizedAnalysisReason struct {
	Kind       AnalysisReasonKind
	PackageRef graph.SymbolRef
}

// AnalysisReasonKind identifies why an analysis is partial.
type AnalysisReasonKind string

const (
	ReasonIncompleteTypeInformation AnalysisReasonKind = "incomplete_type_information"
)
