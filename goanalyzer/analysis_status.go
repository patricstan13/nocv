package goanalyzer

import (
	"sort"

	"golang.org/x/tools/go/packages"

	"nocv/graph"
)

// AnalysisStatus describes whether a successful analysis is known to contain
// all facts supported by the analyzer for the loaded build configuration.
type AnalysisStatus uint8

const (
	AnalysisComplete AnalysisStatus = iota
	AnalysisPartial
)

func (status AnalysisStatus) String() string {
	switch status {
	case AnalysisComplete:
		return "complete"
	case AnalysisPartial:
		return "partial"
	default:
		return "unknown"
	}
}

// AnalysisStatusReasonKind identifies a known condition that may have removed
// supported semantic facts from an otherwise usable analysis.
type AnalysisStatusReasonKind uint8

const (
	AnalysisIncompleteTypeInformation AnalysisStatusReasonKind = iota
)

func (kind AnalysisStatusReasonKind) String() string {
	switch kind {
	case AnalysisIncompleteTypeInformation:
		return "incomplete type information"
	default:
		return "unknown"
	}
}

// AnalysisStatusReason gives the package scope of a coarse analysis-status
// reason without exposing raw compiler diagnostics.
type AnalysisStatusReason struct {
	Kind    AnalysisStatusReasonKind
	Package graph.SymbolRef
}

// Status reports the completeness of this successful analysis.
func (analysis *Analysis) Status() AnalysisStatus {
	return analysis.status
}

// StatusReasons returns a copy of the deterministic package-scoped reasons
// that make this analysis partial.
func (analysis *Analysis) StatusReasons() []AnalysisStatusReason {
	return append([]AnalysisStatusReason(nil), analysis.statusReasons...)
}

func analysisStatusReasons(loadedPackages []*packages.Package) []AnalysisStatusReason {
	partialPackages := make(map[graph.SymbolRef]struct{})
	for _, loadedPackage := range loadedPackages {
		if loadedPackage == nil {
			continue
		}
		incompleteTypeInformation := loadedPackage.IllTyped
		for _, packageError := range loadedPackage.Errors {
			if packageError.Kind == packages.TypeError {
				incompleteTypeInformation = true
				break
			}
		}
		// IllTyped is a conservative backstop because go/packages may not
		// enumerate every condition that left Types or TypesInfo incomplete.
		if incompleteTypeInformation {
			partialPackages[graph.PackageRef(loadedPackage.PkgPath)] = struct{}{}
		}
	}

	reasons := make([]AnalysisStatusReason, 0, len(partialPackages))
	for packageRef := range partialPackages {
		reasons = append(reasons, AnalysisStatusReason{
			Kind:    AnalysisIncompleteTypeInformation,
			Package: packageRef,
		})
	}
	sort.Slice(reasons, func(i, j int) bool {
		return reasons[i].Package < reasons[j].Package
	})
	return reasons
}
