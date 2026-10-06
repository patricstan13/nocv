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
	"nocv/internal/testutil"
)

func TestAnalyzeSignatureChangeResultInferenceAndAttribution(t *testing.T) {
	analysis := loadSignatureFixture(t, "results", "example.com/shop/hypothetical/results/repo", "example.com/shop/hypothetical/results/failure")
	impact := analyzeSignature(t, analysis, "example.com/shop/hypothetical/results/repo::Find", nil, []string{"*model.User", "error"}, false)
	if len(impact.CallSites) != 0 {
		t.Fatalf("result-only call sites = %d, want no noisy parameter analysis", len(impact.CallSites))
	}
	if impact.Compiler.BaselineStatus != goanalyzer.BaselineClean {
		t.Fatalf("baseline = %s, want clean", impact.Compiler.BaselineStatus)
	}
	var attributed bool
	for _, consequence := range impact.Compiler.Consequences {
		if consequence.Package == "example.com/shop/hypothetical/results/failure" && consequence.Symbol != nil && strings.HasSuffix(string(consequence.Symbol.Ref), "::Inferred") {
			attributed = true
		}
	}
	if !attributed {
		t.Fatalf("compiler consequences lack failure::Inferred attribution: %#v", impact.Compiler.Consequences)
	}
}

func TestAnalyzeSignatureChangeCompatibleInferenceHasNoCompilerConsequence(t *testing.T) {
	analysis := loadSignatureFixture(t, "results", "example.com/shop/hypothetical/results/repo", "example.com/shop/hypothetical/results/compatibleforward")
	impact := analyzeSignature(t, analysis, "example.com/shop/hypothetical/results/repo::Compatible", nil, []string{"any", "error"}, false)
	if len(impact.Compiler.Consequences) != 0 {
		t.Fatalf("compatible result change consequences = %#v", impact.Compiler.Consequences)
	}
}

func TestAnalyzeSignatureChangeDirtyBaselineKeepsOnlyDelta(t *testing.T) {
	analysis := loadSignatureFixture(t, "dirty-results", "example.com/shop/hypothetical/results/repo", "example.com/shop/hypothetical/results/broken")
	impact := analyzeSignature(t, analysis, "example.com/shop/hypothetical/results/repo::Find", nil, []string{"*model.User", "error"}, false)
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

func TestAnalyzeSignatureChangeShiftedDirtyDiagnosticIsUncertain(t *testing.T) {
	analysis := loadSignatureFixture(t, "dirty-results", "example.com/shop/hypothetical/results/broken")
	impact := analyzeSignature(t, analysis, "example.com/shop/hypothetical/results/broken::Shift", []string{"struct {\n\tValue int\n}"}, nil, false)
	if impact.Compiler.BaselineStatus != goanalyzer.BaselineHasDiagnostics || len(impact.Compiler.Consequences) != 1 {
		t.Fatalf("shifted dirty impact = %#v", impact.Compiler)
	}
	consequence := impact.Compiler.Consequences[0]
	if consequence.Classification != goanalyzer.DiagnosticUncertain || consequence.Symbol != nil || !strings.Contains(consequence.Message, "existing") {
		t.Fatalf("shifted diagnostic = %#v, want uncertain package-level baseline diagnostic", consequence)
	}
}

func TestAnalyzeSignatureChangeCombinedContractAndStructural(t *testing.T) {
	analysis := loadSignatureFixture(t, "parameterimpact", "./parameterimpact")
	combined := analyzeSignature(t, analysis, "example.com/shop/parameterimpact::Service::Save", []string{"string"}, []string{"error"}, false)
	if len(combined.CallSites) == 0 || len(combined.Compiler.Consequences) == 0 || len(combined.Contracts) == 0 {
		t.Fatalf("combined impact lacks categories: calls=%d compiler=%d contracts=%d", len(combined.CallSites), len(combined.Compiler.Consequences), len(combined.Contracts))
	}

	promoted := analyzeSignature(t, analysis, "example.com/shop/parameterimpact::PromotionBase::Change", []string{"ID"}, []string{"error"}, false)
	if len(promoted.Structural) < 2 {
		t.Fatalf("result-only promoted impacts = %#v", promoted.Structural)
	}
	pointer := analyzeSignature(t, analysis, "example.com/shop/parameterimpact::PointerBase::Touch", []string{"ID"}, []string{"error"}, false)
	hasPointerOnly := false
	for _, impact := range pointer.Structural {
		hasPointerOnly = hasPointerOnly || impact.Exposure == goanalyzer.MethodExposurePointerOnly
	}
	if !hasPointerOnly {
		t.Fatalf("pointer-only result impact = %#v", pointer.Structural)
	}
}

func TestAnalyzeSignatureChangeResultContracts(t *testing.T) {
	analysis := loadSignatureFixture(t, "parameterimpact", "./parameterimpact")
	concrete := analyzeSignature(t, analysis, "example.com/shop/parameterimpact::ResultBase::Fetch", []string{"ID"}, []string{"*Item"}, false)
	if len(concrete.Contracts) != 1 || len(concrete.Structural) != 1 {
		t.Fatalf("concrete result impact: contracts=%#v structural=%#v", concrete.Contracts, concrete.Structural)
	}
	interfaceMethod := analyzeSignature(t, analysis, "example.com/shop/parameterimpact::ResultStore::Fetch", []string{"ID"}, []string{"*Item"}, false)
	if len(interfaceMethod.Contracts) < 1 || len(interfaceMethod.Structural) != 0 {
		t.Fatalf("interface result impact: contracts=%#v structural=%#v", interfaceMethod.Contracts, interfaceMethod.Structural)
	}
}

func TestAnalyzeSignatureChangeResultShapes(t *testing.T) {
	analysis := loadSignatureFixture(t, "results", "example.com/shop/hypothetical/results/repo", "example.com/shop/hypothetical/results/count", "example.com/shop/hypothetical/results/reorder")
	added := analyzeSignature(t, analysis, "example.com/shop/hypothetical/results/repo::Single", nil, []string{"model.User", "error"}, false)
	if len(added.Compiler.Consequences) == 0 {
		t.Fatal("added result produced no compiler consequence")
	}
	removed := analyzeSignature(t, analysis, "example.com/shop/hypothetical/results/repo::Pair", nil, []string{"model.User"}, false)
	if len(removed.Compiler.Consequences) == 0 {
		t.Fatal("removed result produced no compiler consequence")
	}
	reordered := analyzeSignature(t, analysis, "example.com/shop/hypothetical/results/repo::Reordered", nil, []string{"error", "model.User"}, false)
	if len(reordered.Compiler.Consequences) == 0 {
		t.Fatal("reordered results produced no compiler consequence")
	}
}

func TestAnalyzeSignatureChangeNoopAndVariadicOnly(t *testing.T) {
	analysis := loadSignatureFixture(t, "parameterimpact", "./parameterimpact")
	parameterOnly := analyzeSignature(t, analysis, "example.com/shop/parameterimpact::UseID", []string{"string"}, nil, false)
	if len(parameterOnly.CallSites) == 0 || len(parameterOnly.Compiler.AffectedPackages) == 0 {
		t.Fatalf("parameter-only impact = %#v", parameterOnly)
	}
	noop := analyzeSignature(t, analysis, "example.com/shop/parameterimpact::UseID", []string{"ID"}, nil, false)
	if len(noop.CallSites) != 0 || len(noop.Compiler.Consequences) != 0 || len(noop.Contracts) != 0 || len(noop.UncertainContracts) != 0 || len(noop.Structural) != 0 {
		t.Fatalf("semantic no-op = %#v", noop)
	}
	nameOnly, err := analysis.AnalyzeSignatureChange("example.com/shop/parameterimpact::ResultBase::Fetch", goanalyzer.ProposedSignature{
		Parameters: []goanalyzer.ProposedParameter{{Name: "renamedParameter", TypeExpr: "ID"}},
		Results:    []goanalyzer.ProposedResult{{Name: "renamedResult", TypeExpr: "Item"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(nameOnly.CallSites) != 0 || len(nameOnly.Compiler.Consequences) != 0 || len(nameOnly.Contracts) != 0 || len(nameOnly.UncertainContracts) != 0 || len(nameOnly.Structural) != 0 {
		t.Fatalf("name-only proposal = %#v", nameOnly)
	}
	variadic := analyzeSignature(t, analysis, "example.com/shop/parameterimpact::Variadic", []string{"string"}, nil, false)
	if len(variadic.CallSites) == 0 || len(variadic.Compiler.AffectedPackages) == 0 {
		t.Fatalf("variadic-only impact = %#v", variadic)
	}
}

func TestAnalyzeSignatureChangeAddedSlotsKeepNamedDeclarationValid(t *testing.T) {
	analysis := loadSignatureFixture(t, "parameterimpact", "./parameterimpact")
	addedParameter := analyzeSignature(t, analysis, "example.com/shop/parameterimpact::UseID", []string{"ID", "bool"}, nil, false)
	assertNoCompilerMessage(t, addedParameter, "missing parameter type", "mixed named and unnamed parameters")

	addedResult := analyzeSignature(t, analysis, "example.com/shop/parameterimpact::NamedResult", []string{"ID"}, []string{"Item", "error"}, false)
	assertNoCompilerMessage(t, addedResult, "missing parameter type", "mixed named and unnamed parameters")
}

func assertNoCompilerMessage(t *testing.T, impact goanalyzer.SignatureChangeImpact, fragments ...string) {
	t.Helper()
	for _, consequence := range impact.Compiler.Consequences {
		for _, fragment := range fragments {
			if strings.Contains(consequence.Message, fragment) {
				t.Fatalf("compiler consequence %q contains overlay syntax error %q", consequence.Message, fragment)
			}
		}
	}
}

func TestAnalyzeSignatureChangeUsesReverseImportClosureAndLeavesSourceUntouched(t *testing.T) {
	dir := signatureFixtureDir(t, "closure")
	target := filepath.Join(dir, "hypothetical", "closure", "repo", "repo.go")
	before := readTestFile(t, target)
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), dir, "./hypothetical/closure/...")
	if err != nil {
		t.Fatal(err)
	}
	impact := analyzeSignature(t, analysis, "example.com/shop/hypothetical/closure/repo::Find", nil, []string{"string"}, false)
	want := []graph.SymbolRef{
		"example.com/shop/hypothetical/closure/api", "example.com/shop/hypothetical/closure/repo", "example.com/shop/hypothetical/closure/service", "example.com/shop/hypothetical/closure/worker",
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
	if fixture != "dirty-results" {
		return testutil.GoProjectDir(t)
	}
	dir := testutil.CopyGoProject(t)
	brokenDir := filepath.Join(dir, "hypothetical", "results", "broken")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := `package broken

import (
	"example.com/shop/hypothetical/results/model"
	"example.com/shop/hypothetical/results/repo"
)

func Shift(value int) {}

var existing int = "existing"

func consumeUser(model.User) {}

func Example() {
	u, _ := repo.Find()
	consumeUser(u)
}
`
	if err := os.WriteFile(filepath.Join(brokenDir, "broken.go"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func analyzeSignature(t *testing.T, analysis *goanalyzer.Analysis, ref graph.SymbolRef, parameters, results []string, variadic bool) goanalyzer.SignatureChangeImpact {
	t.Helper()
	proposal := goanalyzer.ProposedSignature{Variadic: variadic}
	for _, typ := range parameters {
		proposal.Parameters = append(proposal.Parameters, goanalyzer.ProposedParameter{TypeExpr: typ})
	}
	for _, typ := range results {
		proposal.Results = append(proposal.Results, goanalyzer.ProposedResult{TypeExpr: typ})
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
