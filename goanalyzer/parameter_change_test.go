package goanalyzer_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/internal/testutil"
)

const impactPackage = "example.com/shop/parameterimpact"

func TestAnalyzeSignatureChangeParameterNamedScalarAndUntypedLiteral(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	result := analyzeParameterChange(t, analysis, impactPackage+"::UseID", signature("string"))

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
		previous, current := result.CallSites[index-1].Location, result.CallSites[index].Location
		if previous.Line > current.Line || previous.Line == current.Line && previous.Column >= current.Column {
			t.Fatalf("call sites are not ordered by location: %+v", result.CallSites)
		}
	}

	reverse := analyzeParameterChange(t, analysis, impactPackage+"::UseString", signature("ID"))
	assertCompatibilityCounts(t, reverse, 1, 2, 0)
	for _, site := range reverse.CallSites {
		if site.Compatibility == goanalyzer.CompatibilityIncompatible {
			if len(site.Problems) != 1 || site.Problems[0].Kind != goanalyzer.ProblemArgumentType ||
				site.Problems[0].Argument != 1 ||
				site.Problems[0].Expected.Display != impactPackage+".ID" {
				t.Fatalf("unexpected reverse scalar problem: %+v", site.Problems)
			}
		}
	}

	alias := analyzeParameterChange(t, analysis, impactPackage+"::UseString", signature("Alias"))
	assertNoSignatureConsequences(t, alias)
}

func TestAnalyzeSignatureChangeParameterUsesCompilerConstantRepresentability(t *testing.T) {
	result := analyzeParameterChange(t, loadImpactAnalysis(t), impactPackage+"::UseNumber", signature("uint8"))
	assertCompatibilityCounts(t, result, 1, 1, 0)
	if result.CallSites[0].Compatibility != goanalyzer.CompatibilityCompatible ||
		result.CallSites[1].Compatibility != goanalyzer.CompatibilityIncompatible {
		t.Fatalf("constant compatibility = %v, %v; want compatible, incompatible",
			result.CallSites[0].Compatibility, result.CallSites[1].Compatibility)
	}
}

func TestAnalyzeSignatureChangeParameterStructInterfaceAndPointerAssignability(t *testing.T) {
	result := analyzeParameterChange(t, loadImpactAnalysis(t), impactPackage+"::UseAny", signature("Marker"))
	assertCompatibilityCounts(t, result, 1, 1, 0)
	if result.After.Parameters[0].Type.Symbol != impactPackage+"::Marker" {
		t.Fatalf("modeled interface symbol = %q", result.After.Parameters[0].Type.Symbol)
	}
	if result.CallSites[0].Compatibility != goanalyzer.CompatibilityIncompatible ||
		result.CallSites[1].Compatibility != goanalyzer.CompatibilityCompatible {
		t.Fatalf("value/pointer compatibility = %v, %v", result.CallSites[0].Compatibility, result.CallSites[1].Compatibility)
	}

	imported := analyzeParameterChange(t, loadImpactAnalysis(t), impactPackage+"::UseAny", signature("time.Time"))
	if got := imported.After.Parameters[0].Type.Display; got != "time.Time" {
		t.Fatalf("imported type display = %q, want time.Time", got)
	}
}

func TestAnalyzeSignatureChangeParameterCountsVariadicsAndEllipsis(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	added := analyzeParameterChange(t, analysis, impactPackage+"::UseID", signature("ID", "string"))
	assertCompatibilityCounts(t, added, 0, 3, 0)
	for _, site := range added.CallSites {
		if site.Problems[0].Kind != goanalyzer.ProblemArgumentCount ||
			site.Problems[0].ExpectedCount != 2 || site.Problems[0].ActualCount != 1 {
			t.Fatalf("count problem = %+v", site.Problems)
		}
	}
	removed := analyzeParameterChange(t, analysis, impactPackage+"::UseID", signature())
	assertCompatibilityCounts(t, removed, 0, 3, 0)
	for _, site := range removed.CallSites {
		if site.Problems[0].Kind != goanalyzer.ProblemArgumentCount ||
			site.Problems[0].ExpectedCount != 0 || site.Problems[0].ActualCount != 1 {
			t.Fatalf("removed-parameter count problem = %+v", site.Problems)
		}
	}

	variadic := goanalyzer.ProposedSignature{
		Parameters: []goanalyzer.ProposedParameter{{TypeExpr: "string"}},
		Variadic:   true,
	}
	unchanged := analyzeParameterChange(t, analysis, impactPackage+"::Variadic", variadic)
	assertCompatibilityCounts(t, unchanged, 0, 0, 0)
	if !unchanged.Before.Variadic || unchanged.Before.Parameters[0].Type.Display != "string" {
		t.Fatalf("before variadic signature = %+v", unchanged.Before)
	}
	assertNoSignatureConsequences(t, unchanged)

	nonVariadic := analyzeParameterChange(t, analysis, impactPackage+"::Variadic", signature("[]string"))
	assertCompatibilityCounts(t, nonVariadic, 0, 2, 0)
	kinds := []goanalyzer.SignatureProblemKind{
		nonVariadic.CallSites[0].Problems[0].Kind,
		nonVariadic.CallSites[1].Problems[0].Kind,
	}
	if !slices.Contains(kinds, goanalyzer.ProblemArgumentCount) || !slices.Contains(kinds, goanalyzer.ProblemVariadic) {
		t.Fatalf("non-variadic problems = %v", kinds)
	}
}

func TestAnalyzeSignatureChangeParameterMethodsInterfacesClosuresAndNoCallers(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	method := analyzeParameterChange(t, analysis, impactPackage+"::Service::Save", signature("string"))
	assertCompatibilityCounts(t, method, 0, 1, 0)
	if method.CallSites[0].Caller.Ref != impactPackage+"::CallsMethod" {
		t.Fatalf("method caller = %q", method.CallSites[0].Caller.Ref)
	}

	iface := analyzeParameterChange(t, analysis, impactPackage+"::Store::Save", signature("string"))
	assertCompatibilityCounts(t, iface, 0, 1, 0)
	if iface.CallSites[0].Caller.Ref != impactPackage+"::CallsInterface" {
		t.Fatalf("interface caller = %q", iface.CallSites[0].Caller.Ref)
	}

	closure := analyzeParameterChange(t, analysis, impactPackage+"::UseID", signature("string"))
	closureCalls := 0
	for _, site := range closure.CallSites {
		if site.Caller.Ref == impactPackage+"::CallsID" {
			closureCalls++
		}
	}
	if closureCalls != 3 {
		t.Fatalf("lexically attributed CallsID sites = %d, want 3", closureCalls)
	}

	unused := analyzeParameterChange(t, analysis, impactPackage+"::Unused", signature("ID"))
	if len(unused.CallSites) != 0 {
		t.Fatalf("unused call sites = %+v, want empty", unused.CallSites)
	}
}

func TestAnalyzeSignatureChangeParameterConcreteMethodContractImpact(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	result := analyzeParameterChange(t, analysis, impactPackage+"::Service::Save", signature("string"))

	wantInterfaces := []graph.SymbolRef{
		impactPackage + "::ExtendedStore",
		impactPackage + "::Saver",
		impactPackage + "::Store",
	}
	if got := contractInterfaces(result.Contracts); !slices.Equal(got, wantInterfaces) {
		t.Fatalf("lost interfaces = %v, want %v", got, wantInterfaces)
	}
	for _, impact := range result.Contracts {
		if impact.Kind != goanalyzer.ContractImplementationLost {
			t.Errorf("contract kind = %v", impact.Kind)
		}
		if impact.Concrete.Ref != impactPackage+"::Service" ||
			impact.ConcreteMethod.Ref != impactPackage+"::Service::Save" {
			t.Errorf("concrete contract evidence = %+v", impact)
		}
		if impact.InterfaceMethod.Name != "Save" {
			t.Errorf("interface method = %+v, want Save", impact.InterfaceMethod)
		}
	}
	actualInterfaces := contractInterfaces(result.Contracts)
	if slices.Contains(actualInterfaces, graph.SymbolRef(impactPackage+"::Healthy")) ||
		slices.Contains(actualInterfaces, graph.SymbolRef(impactPackage+"::StringSaver")) {
		t.Fatalf("unrelated contract reported: %+v", result.Contracts)
	}

	unchanged := analyzeParameterChange(t, analysis, impactPackage+"::Service::Save", signature("ID"))
	if len(unchanged.Contracts) != 0 {
		t.Fatalf("semantically unchanged contract impacts = %+v", unchanged.Contracts)
	}
}

func TestAnalyzeSignatureChangeParameterInterfaceMethodContractImpact(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	result := analyzeParameterChange(t, analysis, impactPackage+"::Store::Save", signature("string"))

	got := make([]string, 0, len(result.Contracts))
	for _, impact := range result.Contracts {
		got = append(got, string(impact.Concrete.Ref)+" -> "+string(impact.Interface.Ref))
		if impact.InterfaceMethod.Ref != impactPackage+"::Store::Save" {
			t.Errorf("interface method = %+v", impact.InterfaceMethod)
		}
	}
	want := []string{
		impactPackage + "::PointerStore -> " + impactPackage + "::Store",
		impactPackage + "::Service -> " + impactPackage + "::ExtendedStore",
		impactPackage + "::Service -> " + impactPackage + "::Store",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lost contracts = %v, want %v", got, want)
	}
	unchanged := analyzeParameterChange(t, analysis, impactPackage+"::Store::Save", signature("ID"))
	if len(unchanged.Contracts) != 0 {
		t.Fatalf("unchanged interface contract impacts = %+v", unchanged.Contracts)
	}
}

func TestAnalyzeSignatureChangeParameterPointerReceiverAndNoContractCandidates(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	pointer := analyzeParameterChange(t, analysis, impactPackage+"::PointerStore::Save", signature("string"))
	want := []graph.SymbolRef{impactPackage + "::Saver", impactPackage + "::Store"}
	if got := contractInterfaces(pointer.Contracts); !slices.Equal(got, want) {
		t.Fatalf("pointer receiver lost interfaces = %v, want %v", got, want)
	}

	ordinary := analyzeParameterChange(t, analysis, impactPackage+"::UseID", signature("string"))
	if len(ordinary.Contracts) != 0 {
		t.Fatalf("package function contract impacts = %+v, want empty", ordinary.Contracts)
	}
}

func TestAnalyzeSignatureChangeParameterDirectRecursiveAndShadowedPromotion(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	result := analyzeParameterChange(t, analysis, impactPackage+"::PromotionBase::Change", signature("string"))

	want := []graph.SymbolRef{
		impactPackage + "::PromotionOuter",
		impactPackage + "::PromotionWrapper",
	}
	if got := structuralTypes(result.Structural); !slices.Equal(got, want) {
		t.Fatalf("structurally impacted types = %v, want %v", got, want)
	}
	if _, exists := analysis.Graph().NodeByRef(impactPackage + "::PromotionWrapper::Change"); exists {
		t.Fatal("promoted method was incorrectly materialized as a graph node")
	}
	for _, impact := range result.Structural {
		if impact.Kind != goanalyzer.StructuralPromotedMethodChanged {
			t.Errorf("structural kind = %v", impact.Kind)
		}
		if impact.Exposure != goanalyzer.MethodExposureValue {
			t.Errorf("%s exposure = %v, want value", impact.Type.Ref, impact.Exposure)
		}
		if impact.OriginMethod.Ref != impactPackage+"::PromotionBase::Change" {
			t.Errorf("origin method = %+v", impact.OriginMethod)
		}
	}
	for _, excluded := range []graph.SymbolRef{
		impactPackage + "::PromotionShadow",
		impactPackage + "::PromotionAmbiguous",
	} {
		if slices.Contains(structuralTypes(result.Structural), excluded) {
			t.Errorf("shadowed or ambiguous type %s was reported", excluded)
		}
	}

	unchanged := analyzeParameterChange(t, analysis, impactPackage+"::PromotionBase::Change", signature("ID"))
	if len(unchanged.Structural) != 0 {
		t.Fatalf("unchanged signature structural impacts = %+v", unchanged.Structural)
	}
}

func TestAnalyzeSignatureChangeParameterPointerAndVariadicPromotion(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	pointer := analyzeParameterChange(t, analysis, impactPackage+"::PointerBase::Touch", signature("string"))
	wantPointer := []struct {
		ref      graph.SymbolRef
		exposure goanalyzer.MethodExposure
	}{
		{ref: impactPackage + "::PointerEmbed", exposure: goanalyzer.MethodExposureValue},
		{ref: impactPackage + "::RecursivePointerMid", exposure: goanalyzer.MethodExposurePointerOnly},
		{ref: impactPackage + "::RecursivePointerOuter", exposure: goanalyzer.MethodExposurePointerOnly},
		{ref: impactPackage + "::ValueEmbed", exposure: goanalyzer.MethodExposurePointerOnly},
	}
	if len(pointer.Structural) != len(wantPointer) {
		t.Fatalf("pointer promotion impacts = %+v, want %d", pointer.Structural, len(wantPointer))
	}
	for index, want := range wantPointer {
		got := pointer.Structural[index]
		if got.Type.Ref != want.ref || got.Exposure != want.exposure {
			t.Errorf("pointer impact %d = (%s, %s), want (%s, %s)",
				index, got.Type.Ref, got.Exposure, want.ref, want.exposure)
		}
	}

	variadic := analyzeParameterChange(t, analysis, impactPackage+"::VariadicBase::Collect", signature("[]string"))
	wantVariadic := []graph.SymbolRef{impactPackage + "::VariadicEmbed"}
	if got := structuralTypes(variadic.Structural); !slices.Equal(got, wantVariadic) {
		t.Fatalf("variadic promotion types = %v, want %v", got, wantVariadic)
	}
	if variadic.Structural[0].Exposure != goanalyzer.MethodExposureValue {
		t.Fatalf("variadic exposure = %v, want value", variadic.Structural[0].Exposure)
	}
}

func TestAnalyzeSignatureChangeParameterStructuralImpactExcludesFunctionsAndInterfaces(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	ordinary := analyzeParameterChange(t, analysis, impactPackage+"::UseID", signature("string"))
	if len(ordinary.Structural) != 0 {
		t.Fatalf("package function structural impacts = %+v", ordinary.Structural)
	}
	iface := analyzeParameterChange(t, analysis, impactPackage+"::Store::Save", signature("string"))
	if len(iface.Structural) != 0 {
		t.Fatalf("interface method structural impacts = %+v", iface.Structural)
	}
}

func TestAnalyzeSignatureChangeParameterValidationErrors(t *testing.T) {
	analysis := loadImpactAnalysis(t)
	tests := []struct {
		name string
		ref  graph.SymbolRef
		sig  goanalyzer.ProposedSignature
		want string
	}{
		{name: "missing", ref: "missing", sig: signature("string"), want: "unknown symbol: missing"},
		{name: "non callable", ref: impactPackage + "::Item", sig: signature("string"), want: "symbol is not a function or method"},
		{name: "invalid type", ref: impactPackage + "::UseID", sig: signature("DoesNotExist"), want: "resolve proposed parameter 1 type"},
		{name: "empty variadic", ref: impactPackage + "::UseID", sig: goanalyzer.ProposedSignature{Variadic: true}, want: "requires at least one parameter"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := analysis.AnalyzeSignatureChange(test.ref, test.sig)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func loadImpactAnalysis(t *testing.T) *goanalyzer.Analysis {
	t.Helper()
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), testutil.GoProjectDir(t), "./parameterimpact")
	if err != nil {
		t.Fatal(err)
	}
	return analysis
}

func signature(typeExpressions ...string) goanalyzer.ProposedSignature {
	result := goanalyzer.ProposedSignature{}
	for _, expression := range typeExpressions {
		result.Parameters = append(result.Parameters, goanalyzer.ProposedParameter{TypeExpr: expression})
	}
	return result
}

func analyzeParameterChange(t *testing.T, analysis *goanalyzer.Analysis, ref graph.SymbolRef, proposed goanalyzer.ProposedSignature) goanalyzer.SignatureChangeImpact {
	t.Helper()
	result, err := analysis.AnalyzeSignatureChange(ref, proposed)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertCompatibilityCounts(t *testing.T, result goanalyzer.SignatureChangeImpact, compatible, incompatible, unknown int) {
	t.Helper()
	var gotCompatible, gotIncompatible, gotUnknown int
	for _, site := range result.CallSites {
		switch site.Compatibility {
		case goanalyzer.CompatibilityCompatible:
			gotCompatible++
		case goanalyzer.CompatibilityIncompatible:
			gotIncompatible++
		case goanalyzer.CompatibilityUnknown:
			gotUnknown++
		}
	}
	if gotCompatible != compatible || gotIncompatible != incompatible || gotUnknown != unknown {
		t.Fatalf("compatibility counts = (%d, %d, %d), want (%d, %d, %d): %+v",
			gotCompatible, gotIncompatible, gotUnknown, compatible, incompatible, unknown, result.CallSites)
	}
}

func assertNoSignatureConsequences(t *testing.T, impact goanalyzer.SignatureChangeImpact) {
	t.Helper()
	if len(impact.CallSites) != 0 || len(impact.Compiler.Consequences) != 0 ||
		len(impact.Contracts) != 0 || len(impact.UncertainContracts) != 0 || len(impact.Structural) != 0 {
		t.Fatalf("semantic no-op impact = %+v", impact)
	}
}

func contractInterfaces(impacts []goanalyzer.ContractImpact) []graph.SymbolRef {
	result := make([]graph.SymbolRef, 0, len(impacts))
	for _, impact := range impacts {
		result = append(result, impact.Interface.Ref)
	}
	return result
}

func structuralTypes(impacts []goanalyzer.StructuralImpact) []graph.SymbolRef {
	result := make([]graph.SymbolRef, 0, len(impacts))
	for _, impact := range impacts {
		result = append(result, impact.Type.Ref)
	}
	return result
}
