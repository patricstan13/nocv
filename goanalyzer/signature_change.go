package goanalyzer

import (
	"nocv/graph"
	"nocv/query"
)

// SignatureChangeImpact combines precise local semantic checks with the Go
// compiler's diagnostics for one hypothetical callable signature.
type SignatureChangeImpact struct {
	Callable   graph.SymbolRef
	Before     CallableSignature
	After      CallableSignature
	CallSites  []CallSiteImpact
	Compiler   CompilerImpact
	Contracts  []ContractImpact
	Structural []StructuralImpact
}

// CompilerImpact is the compiler-observed portion of a signature change. Its
// affected scope is the changed package's real reverse import closure.
type CompilerImpact struct {
	AffectedPackages []graph.SymbolRef
	Consequences     []CompilerConsequence
	BaselineStatus   BaselineStatus
}

type CompilerConsequence struct {
	Package        graph.SymbolRef
	Symbol         *query.SymbolSummary
	Location       graph.Location
	Message        string
	Classification DiagnosticClassification
}

type BaselineStatus uint8

const (
	BaselineUnknown BaselineStatus = iota
	BaselineClean
	BaselineHasDiagnostics
)

func (s BaselineStatus) String() string {
	switch s {
	case BaselineClean:
		return "clean"
	case BaselineHasDiagnostics:
		return "has diagnostics"
	default:
		return "unknown"
	}
}

type DiagnosticClassification uint8

const (
	DiagnosticUnknown DiagnosticClassification = iota
	DiagnosticNew
	DiagnosticUncertain
)

func (c DiagnosticClassification) String() string {
	switch c {
	case DiagnosticNew:
		return "new"
	case DiagnosticUncertain:
		return "uncertain"
	default:
		return "unknown"
	}
}
