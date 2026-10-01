package goanalyzer_test

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/query"
)

const impactPackage = "example.com/parameterimpact"

func TestAnalyzeParameterChangeNamedScalarAndUntypedLiteral(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	result := analyze(t, analysis, impactPackage+"::UseID", signature("string"))

	if got, want := result.Before.Parameters[0].Type.Display, impactPackage+".ID"; got != want {
		t.Fatalf("Before parameter = %q, want %q", got, want)
	}
	if got := result.After.Parameters[0].Type.Display; got != "string" {
		t.Fatalf("After parameter = %q, want string", got)
	}
	if got := result.After.Parameters[0].Name; got != "id" {
		t.Fatalf("preserved parameter name = %q, want id", got)
	}
	if result.Before.Parameters[0].Type.Symbol != "" {
		t.Fatalf("named scalar unexpectedly has graph symbol %q", result.Before.Parameters[0].Type.Symbol)
	}
	assertCompatibilityCounts(t, result, 1, 2, 0)
	if result.CallSites[0].Caller.Ref != impactPackage+"::CallsID" {
		t.Fatalf("first caller = %q", result.CallSites[0].Caller.Ref)
	}
	for index := 1; index < len(result.CallSites); index++ {
		if result.CallSites[index-1].Location.Offset >= result.CallSites[index].Location.Offset {
			t.Fatalf("call sites are not ordered by location: %+v", result.CallSites)
		}
	}

	reverse := analyze(t, analysis, impactPackage+"::UseString", signature("ID"))
	assertCompatibilityCounts(t, reverse, 1, 2, 0)
	for _, site := range reverse.CallSites {
		if site.Compatibility == query.CompatibilityIncompatible {
			if len(site.Problems) != 1 || site.Problems[0].Kind != query.ProblemArgumentType ||
				site.Problems[0].Argument != 1 ||
				site.Problems[0].Expected.Display != impactPackage+".ID" {
				t.Fatalf("unexpected reverse scalar problem: %+v", site.Problems)
			}
		}
	}

	alias := analyze(t, analysis, impactPackage+"::UseString", signature("Alias"))
	assertCompatibilityCounts(t, alias, 3, 0, 0)
}

func TestAnalyzeParameterChangeUsesCompilerConstantRepresentability(t *testing.T) {
	result := analyze(t, loadImpactAnalysis(t), impactPackage+"::UseNumber", signature("uint8"))
	assertCompatibilityCounts(t, result, 1, 1, 0)
	if result.CallSites[0].Compatibility != query.CompatibilityCompatible ||
		result.CallSites[1].Compatibility != query.CompatibilityIncompatible {
		t.Fatalf("constant compatibility = %v, %v; want compatible, incompatible",
			result.CallSites[0].Compatibility, result.CallSites[1].Compatibility)
	}
}

func TestAnalyzeParameterChangeStructInterfaceAndPointerAssignability(t *testing.T) {
	result := analyze(t, loadImpactAnalysis(t), impactPackage+"::UseAny", signature("Marker"))
	assertCompatibilityCounts(t, result, 1, 1, 0)
	if result.After.Parameters[0].Type.Symbol != impactPackage+"::Marker" {
		t.Fatalf("modeled interface symbol = %q", result.After.Parameters[0].Type.Symbol)
	}
	if result.CallSites[0].Compatibility != query.CompatibilityIncompatible ||
		result.CallSites[1].Compatibility != query.CompatibilityCompatible {
		t.Fatalf("value/pointer compatibility = %v, %v", result.CallSites[0].Compatibility, result.CallSites[1].Compatibility)
	}

	imported := analyze(t, loadImpactAnalysis(t), impactPackage+"::UseAny", signature("time.Time"))
	if got := imported.After.Parameters[0].Type.Display; got != "time.Time" {
		t.Fatalf("imported type display = %q, want time.Time", got)
	}
}

func TestAnalyzeParameterChangeCountsVariadicsAndEllipsis(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	added := analyze(t, analysis, impactPackage+"::UseID", signature("ID", "string"))
	assertCompatibilityCounts(t, added, 0, 3, 0)
	for _, site := range added.CallSites {
		if site.Problems[0].Kind != query.ProblemArgumentCount ||
			site.Problems[0].ExpectedCount != 2 || site.Problems[0].ActualCount != 1 {
			t.Fatalf("count problem = %+v", site.Problems)
		}
	}
	removed := analyze(t, analysis, impactPackage+"::UseID", signature())
	assertCompatibilityCounts(t, removed, 0, 3, 0)
	for _, site := range removed.CallSites {
		if site.Problems[0].Kind != query.ProblemArgumentCount ||
			site.Problems[0].ExpectedCount != 0 || site.Problems[0].ActualCount != 1 {
			t.Fatalf("removed-parameter count problem = %+v", site.Problems)
		}
	}

	variadic := query.ProposedSignature{
		Parameters: []query.ProposedParameter{{TypeExpr: "string"}},
		Variadic:   true,
	}
	unchanged := analyze(t, analysis, impactPackage+"::Variadic", variadic)
	assertCompatibilityCounts(t, unchanged, 2, 0, 0)
	if !unchanged.Before.Variadic || unchanged.Before.Parameters[0].Type.Display != "string" {
		t.Fatalf("before variadic signature = %+v", unchanged.Before)
	}

	nonVariadic := analyze(t, analysis, impactPackage+"::Variadic", signature("[]string"))
	assertCompatibilityCounts(t, nonVariadic, 0, 2, 0)
	kinds := []query.SignatureProblemKind{
		nonVariadic.CallSites[0].Problems[0].Kind,
		nonVariadic.CallSites[1].Problems[0].Kind,
	}
	if !slices.Contains(kinds, query.ProblemArgumentCount) || !slices.Contains(kinds, query.ProblemVariadic) {
		t.Fatalf("non-variadic problems = %v", kinds)
	}
}

func TestAnalyzeParameterChangeMethodsInterfacesClosuresAndNoCallers(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	method := analyze(t, analysis, impactPackage+"::Service::Save", signature("string"))
	assertCompatibilityCounts(t, method, 0, 1, 0)
	if method.CallSites[0].Caller.Ref != impactPackage+"::CallsMethod" {
		t.Fatalf("method caller = %q", method.CallSites[0].Caller.Ref)
	}

	iface := analyze(t, analysis, impactPackage+"::Store::Save", signature("string"))
	assertCompatibilityCounts(t, iface, 0, 1, 0)
	if iface.CallSites[0].Caller.Ref != impactPackage+"::CallsInterface" {
		t.Fatalf("interface caller = %q", iface.CallSites[0].Caller.Ref)
	}

	closure := analyze(t, analysis, impactPackage+"::UseID", signature("string"))
	closureCalls := 0
	for _, site := range closure.CallSites {
		if site.Caller.Ref == impactPackage+"::CallsID" {
			closureCalls++
		}
	}
	if closureCalls != 3 {
		t.Fatalf("lexically attributed CallsID sites = %d, want 3", closureCalls)
	}

	unused := analyze(t, analysis, impactPackage+"::Unused", signature("ID"))
	if len(unused.CallSites) != 0 {
		t.Fatalf("unused call sites = %+v, want empty", unused.CallSites)
	}
}

func TestAnalyzeParameterChangeValidationErrors(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	tests := []struct {
		name string
		ref  graph.SymbolRef
		sig  query.ProposedSignature
		want string
	}{
		{name: "missing", ref: "missing", sig: signature("string"), want: "unknown symbol: missing"},
		{name: "non callable", ref: impactPackage + "::Item", sig: signature("string"), want: "symbol is not a function or method"},
		{name: "invalid type", ref: impactPackage + "::UseID", sig: signature("DoesNotExist"), want: "resolve proposed parameter 1 type"},
		{name: "empty variadic", ref: impactPackage + "::UseID", sig: query.ProposedSignature{Variadic: true}, want: "requires at least one parameter"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := analysis.AnalyzeParameterChange(test.ref, test.sig)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func loadImpactAnalysis(t *testing.T) *goanalyzer.Analysis {
	t.Helper()
	dir, err := filepath.Abs("testdata/parameterimpact")
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), dir, "./...")
	if err != nil {
		t.Fatal(err)
	}
	return analysis
}

func signature(typeExpressions ...string) query.ProposedSignature {
	result := query.ProposedSignature{}
	for _, expression := range typeExpressions {
		result.Parameters = append(result.Parameters, query.ProposedParameter{TypeExpr: expression})
	}
	return result
}

func analyze(t *testing.T, analysis *goanalyzer.Analysis, ref graph.SymbolRef, proposed query.ProposedSignature) query.ParameterChangeImpact {
	t.Helper()
	result, err := analysis.AnalyzeParameterChange(ref, proposed)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertCompatibilityCounts(t *testing.T, result query.ParameterChangeImpact, compatible, incompatible, unknown int) {
	t.Helper()
	var gotCompatible, gotIncompatible, gotUnknown int
	for _, site := range result.CallSites {
		switch site.Compatibility {
		case query.CompatibilityCompatible:
			gotCompatible++
		case query.CompatibilityIncompatible:
			gotIncompatible++
		case query.CompatibilityUnknown:
			gotUnknown++
		}
	}
	if gotCompatible != compatible || gotIncompatible != incompatible || gotUnknown != unknown {
		t.Fatalf("compatibility counts = (%d, %d, %d), want (%d, %d, %d): %+v",
			gotCompatible, gotIncompatible, gotUnknown, compatible, incompatible, unknown, result.CallSites)
	}
}
