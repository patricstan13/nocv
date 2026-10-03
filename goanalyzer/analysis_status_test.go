package goanalyzer

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"nocv/graph"
)

func TestLoadAnalysisStatusComplete(t *testing.T) {
	analysis := loadStatusAnalysis(t, "status", "./clean")
	if analysis.Status() != AnalysisComplete {
		t.Fatalf("status = %s, want complete", analysis.Status())
	}
	if reasons := analysis.StatusReasons(); len(reasons) != 0 {
		t.Fatalf("status reasons = %#v, want none", reasons)
	}
}

func TestLoadAnalysisStatusPartialIsDeduplicatedAndDeterministic(t *testing.T) {
	analysis := loadStatusAnalysis(t, "status", "./...")
	if analysis.Status() != AnalysisPartial {
		t.Fatalf("status = %s, want partial", analysis.Status())
	}
	want := []AnalysisStatusReason{
		{Kind: AnalysisIncompleteTypeInformation, Package: "example.com/status/brokenone"},
		{Kind: AnalysisIncompleteTypeInformation, Package: "example.com/status/brokentwo"},
	}
	if got := analysis.StatusReasons(); !slices.Equal(got, want) {
		t.Fatalf("status reasons = %#v, want %#v", got, want)
	}

	copyOfReasons := analysis.StatusReasons()
	copyOfReasons[0].Package = "changed"
	if got := analysis.StatusReasons(); !slices.Equal(got, want) {
		t.Fatalf("mutating returned reasons changed analysis: %#v", got)
	}
}

func TestAnalysisStatusReasonsUsesIllTypedBackstop(t *testing.T) {
	reasons := analysisStatusReasons([]*packages.Package{
		{PkgPath: "example.com/z", IllTyped: true},
		{PkgPath: "example.com/a", Errors: []packages.Error{
			{Kind: packages.TypeError, Msg: "first"},
			{Kind: packages.TypeError, Msg: "second"},
		}},
	})
	want := []AnalysisStatusReason{
		{Kind: AnalysisIncompleteTypeInformation, Package: "example.com/a"},
		{Kind: AnalysisIncompleteTypeInformation, Package: "example.com/z"},
	}
	if !slices.Equal(reasons, want) {
		t.Fatalf("status reasons = %#v, want %#v", reasons, want)
	}
}

func TestLoadAnalysisStatusFailures(t *testing.T) {
	t.Run("syntax error", func(t *testing.T) {
		analysis, err := LoadAnalysis(context.Background(), statusFixtureDir(t, "statussyntax"), ".")
		if err == nil || analysis != nil {
			t.Fatalf("LoadAnalysis syntax error = (%#v, %v), want nil error result", analysis, err)
		}
	})
	t.Run("zero packages", func(t *testing.T) {
		analysis, err := LoadAnalysis(context.Background(), statusFixtureDir(t, "statusempty"), "./...")
		if err == nil || analysis != nil || !strings.Contains(err.Error(), "no packages matched") {
			t.Fatalf("LoadAnalysis zero packages = (%#v, %v), want nil matching error", analysis, err)
		}
	})
}

func TestAnalyzeSignatureChangeRunsOnPartialAnalysis(t *testing.T) {
	analysis := loadStatusAnalysis(t, "status", "./brokenone")
	if analysis.Status() != AnalysisPartial {
		t.Fatalf("status = %s, want partial", analysis.Status())
	}
	impact, err := analysis.AnalyzeSignatureChange(
		graph.SymbolRef("example.com/status/brokenone::Service::Save"),
		ProposedSignature{Parameters: []ProposedParameter{{TypeExpr: "string"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if impact.Callable != "example.com/status/brokenone::Service::Save" {
		t.Fatalf("impact callable = %q", impact.Callable)
	}
}

func TestAnalysisStatusStringsDefendUnexpectedValues(t *testing.T) {
	if got := AnalysisStatus(99).String(); got != "unknown" {
		t.Fatalf("unexpected status string = %q", got)
	}
	if got := AnalysisStatusReasonKind(99).String(); got != "unknown" {
		t.Fatalf("unexpected reason string = %q", got)
	}
}

func loadStatusAnalysis(t *testing.T, fixture string, patterns ...string) *Analysis {
	t.Helper()
	analysis, err := LoadAnalysis(context.Background(), statusFixtureDir(t, fixture), patterns...)
	if err != nil {
		t.Fatal(err)
	}
	return analysis
}

func statusFixtureDir(t *testing.T, fixture string) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
