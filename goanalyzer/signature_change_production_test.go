package goanalyzer_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/query"
)

func TestAnalyzeSignatureChangeResultInferenceAndAttribution(t *testing.T) {
	analysis := loadSignatureFixture(t, "results", "example.com/hypothetical/repo", "example.com/hypothetical/failure")
	impact := analyzeSignature(t, analysis, "example.com/hypothetical/repo::Find", nil, []string{"*model.User", "error"}, false)
	if len(impact.CallSites) != 0 {
		t.Fatalf("result-only call sites = %d, want no noisy parameter analysis", len(impact.CallSites))
	}
	if impact.Compiler.BaselineStatus != goanalyzer.BaselineClean {
		t.Fatalf("baseline = %s, want clean", impact.Compiler.BaselineStatus)
	}
	var attributed bool
	for _, consequence := range impact.Compiler.Consequences {
		if consequence.Package == "example.com/hypothetical/failure" && consequence.Symbol != nil && strings.HasSuffix(string(consequence.Symbol.Ref), "::Inferred") {
			attributed = true
		}
	}
	if !attributed {
		t.Fatalf("compiler consequences lack failure::Inferred attribution: %#v", impact.Compiler.Consequences)
	}
}

func TestAnalyzeSignatureChangeCompatibleInferenceHasNoCompilerConsequence(t *testing.T) {
	analysis := loadSignatureFixture(t, "results", "example.com/hypothetical/repo", "example.com/hypothetical/compatibleforward")
	impact := analyzeSignature(t, analysis, "example.com/hypothetical/repo::Compatible", nil, []string{"any", "error"}, false)
	if len(impact.Compiler.Consequences) != 0 {
		t.Fatalf("compatible result change consequences = %#v", impact.Compiler.Consequences)
	}
}

func TestAnalyzeSignatureChangeDirtyBaselineKeepsOnlyDelta(t *testing.T) {
	analysis := loadSignatureFixture(t, "results", "example.com/hypothetical/repo", "example.com/hypothetical/broken")
	impact := analyzeSignature(t, analysis, "example.com/hypothetical/repo::Find", nil, []string{"*model.User", "error"}, false)
	if impact.Compiler.BaselineStatus != goanalyzer.BaselineHasDiagnostics {
		t.Fatalf("baseline = %s, want dirty", impact.Compiler.BaselineStatus)
	}
	if len(impact.Compiler.Consequences) == 0 {
		t.Fatal("dirty baseline hid the new result consequence")
	}
	for _, consequence := range impact.Compiler.Consequences {
		if strings.Contains(consequence.Message, "existing") {
			t.Fatalf("pre-existing diagnostic was returned as a consequence: %#v", consequence)
		}
	}
}

func TestAnalyzeSignatureChangeCombinedContractAndStructural(t *testing.T) {
	analysis := loadSignatureFixture(t, "parameterimpact", ".")
	combined := analyzeSignature(t, analysis, "example.com/parameterimpact::Service::Save", []string{"string"}, []string{"error"}, false)
	if len(combined.CallSites) == 0 || len(combined.Compiler.Consequences) == 0 || len(combined.Contracts) == 0 {
		t.Fatalf("combined impact lacks categories: calls=%d compiler=%d contracts=%d", len(combined.CallSites), len(combined.Compiler.Consequences), len(combined.Contracts))
	}

	promoted := analyzeSignature(t, analysis, "example.com/parameterimpact::PromotionBase::Change", []string{"ID"}, []string{"error"}, false)
	if len(promoted.Structural) < 2 {
		t.Fatalf("result-only promoted impacts = %#v", promoted.Structural)
	}
	pointer := analyzeSignature(t, analysis, "example.com/parameterimpact::PointerBase::Touch", []string{"ID"}, []string{"error"}, false)
	hasPointerOnly := false
	for _, impact := range pointer.Structural {
		hasPointerOnly = hasPointerOnly || impact.Exposure == query.MethodExposurePointerOnly
	}
	if !hasPointerOnly {
		t.Fatalf("pointer-only result impact = %#v", pointer.Structural)
	}
}

func TestAnalyzeSignatureChangeResultContracts(t *testing.T) {
	analysis := loadSignatureFixture(t, "parameterimpact", ".")
	concrete := analyzeSignature(t, analysis, "example.com/parameterimpact::ResultBase::Fetch", []string{"ID"}, []string{"*Item"}, false)
	if len(concrete.Contracts) != 1 || len(concrete.Structural) != 1 {
		t.Fatalf("concrete result impact: contracts=%#v structural=%#v", concrete.Contracts, concrete.Structural)
	}
	interfaceMethod := analyzeSignature(t, analysis, "example.com/parameterimpact::ResultStore::Fetch", []string{"ID"}, []string{"*Item"}, false)
	if len(interfaceMethod.Contracts) < 1 || len(interfaceMethod.Structural) != 0 {
		t.Fatalf("interface result impact: contracts=%#v structural=%#v", interfaceMethod.Contracts, interfaceMethod.Structural)
	}
}

func TestAnalyzeSignatureChangeResultShapes(t *testing.T) {
	analysis := loadSignatureFixture(t, "results", "example.com/hypothetical/repo", "example.com/hypothetical/count", "example.com/hypothetical/reorder")
	added := analyzeSignature(t, analysis, "example.com/hypothetical/repo::Single", nil, []string{"model.User", "error"}, false)
	if len(added.Compiler.Consequences) == 0 {
		t.Fatal("added result produced no compiler consequence")
	}
	removed := analyzeSignature(t, analysis, "example.com/hypothetical/repo::Pair", nil, []string{"model.User"}, false)
	if len(removed.Compiler.Consequences) == 0 {
		t.Fatal("removed result produced no compiler consequence")
	}
	reordered := analyzeSignature(t, analysis, "example.com/hypothetical/repo::Reordered", nil, []string{"error", "model.User"}, false)
	if len(reordered.Compiler.Consequences) == 0 {
		t.Fatal("reordered results produced no compiler consequence")
	}
}

func TestAnalyzeSignatureChangeNoopAndVariadicOnly(t *testing.T) {
	analysis := loadSignatureFixture(t, "parameterimpact", ".")
	parameterOnly := analyzeSignature(t, analysis, "example.com/parameterimpact::UseID", []string{"string"}, nil, false)
	if len(parameterOnly.CallSites) == 0 || len(parameterOnly.Compiler.AffectedPackages) == 0 {
		t.Fatalf("parameter-only impact = %#v", parameterOnly)
	}
	noop := analyzeSignature(t, analysis, "example.com/parameterimpact::UseID", []string{"ID"}, nil, false)
	if len(noop.CallSites) != 0 || len(noop.Compiler.Consequences) != 0 || len(noop.Contracts) != 0 || len(noop.Structural) != 0 {
		t.Fatalf("semantic no-op = %#v", noop)
	}
	nameOnly, err := analysis.AnalyzeSignatureChange("example.com/parameterimpact::ResultBase::Fetch", query.ProposedSignature{
		Parameters: []query.ProposedParameter{{Name: "renamedParameter", TypeExpr: "ID"}},
		Results:    []query.ProposedResult{{Name: "renamedResult", TypeExpr: "Item"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(nameOnly.CallSites) != 0 || len(nameOnly.Compiler.Consequences) != 0 || len(nameOnly.Contracts) != 0 || len(nameOnly.Structural) != 0 {
		t.Fatalf("name-only proposal = %#v", nameOnly)
	}
	variadic := analyzeSignature(t, analysis, "example.com/parameterimpact::Variadic", []string{"string"}, nil, false)
	if len(variadic.CallSites) == 0 || len(variadic.Compiler.AffectedPackages) == 0 {
		t.Fatalf("variadic-only impact = %#v", variadic)
	}
}

func TestAnalyzeSignatureChangeUsesReverseImportClosureAndLeavesSourceUntouched(t *testing.T) {
	dir := signatureFixtureDir(t, "closure")
	target := filepath.Join(dir, "repo", "repo.go")
	before := readTestFile(t, target)
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), dir, "./...")
	if err != nil {
		t.Fatal(err)
	}
	impact := analyzeSignature(t, analysis, "example.com/closure/repo::Find", nil, []string{"string"}, false)
	want := []graph.SymbolRef{
		"example.com/closure/api", "example.com/closure/repo", "example.com/closure/service", "example.com/closure/worker",
	}
	if !slices.Equal(impact.Compiler.AffectedPackages, want) {
		t.Fatalf("affected packages = %v, want %v", impact.Compiler.AffectedPackages, want)
	}
	if after := readTestFile(t, target); string(after) != string(before) {
		t.Fatal("signature overlay modified source on disk")
	}
}

func loadSignatureFixture(t *testing.T, fixture string, patterns ...string) *goanalyzer.Analysis {
	t.Helper()
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), signatureFixtureDir(t, fixture), patterns...)
	if err != nil {
		t.Fatal(err)
	}
	return analysis
}

func signatureFixtureDir(t *testing.T, fixture string) string {
	t.Helper()
	path := filepath.Join("testdata", "hypothetical", fixture)
	if fixture == "parameterimpact" {
		path = filepath.Join("testdata", fixture)
	}
	dir, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func analyzeSignature(t *testing.T, analysis *goanalyzer.Analysis, ref graph.SymbolRef, parameters, results []string, variadic bool) goanalyzer.SignatureChangeImpact {
	t.Helper()
	proposal := query.ProposedSignature{Variadic: variadic}
	for _, typ := range parameters {
		proposal.Parameters = append(proposal.Parameters, query.ProposedParameter{TypeExpr: typ})
	}
	for _, typ := range results {
		proposal.Results = append(proposal.Results, query.ProposedResult{TypeExpr: typ})
	}
	impact, err := analysis.AnalyzeSignatureChange(ref, proposal)
	if err != nil {
		t.Fatal(err)
	}
	return impact
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}
