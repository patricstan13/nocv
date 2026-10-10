package characterization

import (
	"cmp"
	"context"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/internal/testutil"
)

func TestFromLegacyCanonicalAnalysis(t *testing.T) {
	root := testutil.GoProjectDir(t)
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), root, "./...")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := FromLegacy(root, analysis)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("symbols", func(t *testing.T) {
		want := []NormalizedSymbol{
			{SymbolRef: "example.com/shop/orders", Kind: SymbolPackage},
			{SymbolRef: "example.com/shop/orders::Service", Kind: SymbolStruct, ParentRef: "example.com/shop/orders"},
			{SymbolRef: "example.com/shop/orders::Repository", Kind: SymbolInterface, ParentRef: "example.com/shop/orders"},
			{SymbolRef: "example.com/shop/orders::NewService", Kind: SymbolFunction, ParentRef: "example.com/shop/orders"},
			{SymbolRef: "example.com/shop/orders::Service::Create", Kind: SymbolFunction, ParentRef: "example.com/shop/orders::Service"},
			{SymbolRef: "example.com/shop/orders::Repository::Save", Kind: SymbolFunction, ParentRef: "example.com/shop/orders::Repository"},
		}
		for _, symbol := range want {
			if !slices.Contains(snapshot.Symbols, symbol) {
				t.Errorf("missing normalized symbol %#v", symbol)
			}
		}
	})

	t.Run("relationship kinds", func(t *testing.T) {
		observed := make(map[FactKind]bool)
		for _, fact := range snapshot.Facts {
			observed[fact.Kind] = true
		}
		for _, kind := range []FactKind{
			FactCalls,
			FactImplements,
			FactEmbeds,
			FactAccepts,
			FactReturns,
			FactFieldType,
			FactImports,
		} {
			if !observed[kind] {
				t.Errorf("canonical facts do not contain kind %q", kind)
			}
		}
	})

	t.Run("repeated source evidence", func(t *testing.T) {
		fact, ok := findFact(
			snapshot,
			"example.com/shop/orders::Process",
			FactCalls,
			"example.com/shop/orders::Validate",
		)
		if !ok {
			t.Fatal("missing repeated Process -> Validate calls fact")
		}
		if len(fact.Evidence) != 2 {
			t.Fatalf("Process -> Validate evidence count = %d, want 2: %#v", len(fact.Evidence), fact.Evidence)
		}
		first := fact.Evidence[0].(SourceEvidence)
		second := fact.Evidence[1].(SourceEvidence)
		if first.FileRef != "orders/orders.go" || second.FileRef != "orders/orders.go" {
			t.Fatalf("source file refs = %q, %q, want orders/orders.go", first.FileRef, second.FileRef)
		}
		if first.StartOffset >= second.StartOffset {
			t.Fatalf("source offsets are not distinct and sorted: %d, %d", first.StartOffset, second.StartOffset)
		}
	})

	t.Run("implements evidence omitted", func(t *testing.T) {
		fact, ok := findFact(
			snapshot,
			"example.com/shop/orders::PostgresRepository",
			FactImplements,
			"example.com/shop/orders::Repository",
		)
		if !ok {
			t.Fatal("missing PostgresRepository -> Repository implementation fact")
		}
		if len(fact.Evidence) != 0 {
			t.Fatalf("implementation evidence = %#v, want empty", fact.Evidence)
		}
	})

	t.Run("complete analysis", func(t *testing.T) {
		if snapshot.Analysis.Status != AnalysisComplete {
			t.Fatalf("status = %q, want complete", snapshot.Analysis.Status)
		}
		if len(snapshot.Analysis.Reasons) != 0 {
			t.Fatalf("reasons = %#v, want empty", snapshot.Analysis.Reasons)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		second, err := FromLegacy(root, analysis)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(snapshot, second) {
			t.Fatal("normalizing the same analysis twice produced different snapshots")
		}
		assertSorted(t, snapshot)
	})
}

func TestFromLegacyPartialAnalysis(t *testing.T) {
	root := testutil.PartialGoProject(t)
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), root, "./status/...")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := FromLegacy(root, analysis)
	if err != nil {
		t.Fatal(err)
	}

	want := NormalizedAnalysis{
		Status: AnalysisPartial,
		Reasons: []NormalizedAnalysisReason{
			{Kind: ReasonIncompleteTypeInformation, PackageRef: "example.com/shop/status/brokenone"},
			{Kind: ReasonIncompleteTypeInformation, PackageRef: "example.com/shop/status/brokentwo"},
		},
	}
	if !reflect.DeepEqual(snapshot.Analysis, want) {
		t.Fatalf("analysis = %#v, want %#v", snapshot.Analysis, want)
	}
}

func TestNormalizeEdgeExcludesUncertainAndRejectsInvalidCertainty(t *testing.T) {
	legacyGraph := graph.New()
	from, err := legacyGraph.AddNode(graph.Node{Ref: "example.com/test", Kind: graph.NodePackage, Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	to, err := legacyGraph.AddNode(graph.Node{Ref: "example.com/other", Kind: graph.NodePackage, Name: "other"})
	if err != nil {
		t.Fatal(err)
	}

	uncertain := &graph.Edge{From: from, To: to, Kind: graph.EdgeImports, Certainty: graph.RelationshipUncertain}
	if _, include, err := normalizeEdge(t.TempDir(), legacyGraph, uncertain); err != nil || include {
		t.Fatalf("normalize uncertain edge = (include %t, err %v), want excluded without error", include, err)
	}

	invalid := &graph.Edge{From: from, To: to, Kind: graph.EdgeImports, Certainty: graph.RelationshipCertaintyUnknown}
	if _, _, err := normalizeEdge(t.TempDir(), legacyGraph, invalid); err == nil {
		t.Fatal("normalize invalid-certainty edge succeeded, want error")
	}
}

func TestNormalizeSourceEvidenceRejectsFileOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.go")
	if _, err := normalizeSourceEvidence(root, graph.Location{File: outside}); err == nil {
		t.Fatal("normalize source evidence outside root succeeded, want error")
	}
}

func findFact(snapshot NormalizedSnapshot, from graph.SymbolRef, kind FactKind, to graph.SymbolRef) (NormalizedFact, bool) {
	for _, fact := range snapshot.Facts {
		if fact.FromRef == from && fact.Kind == kind && fact.ToRef == to {
			return fact, true
		}
	}
	return NormalizedFact{}, false
}

func assertSorted(t *testing.T, snapshot NormalizedSnapshot) {
	t.Helper()
	if !slices.IsSortedFunc(snapshot.Symbols, func(left, right NormalizedSymbol) int {
		return cmp.Compare(left.SymbolRef, right.SymbolRef)
	}) {
		t.Error("symbols are not sorted by symbol reference")
	}
	if !slices.IsSortedFunc(snapshot.Facts, func(left, right NormalizedFact) int {
		if result := cmp.Compare(left.FromRef, right.FromRef); result != 0 {
			return result
		}
		if result := cmp.Compare(left.Kind, right.Kind); result != 0 {
			return result
		}
		return cmp.Compare(left.ToRef, right.ToRef)
	}) {
		t.Error("facts are not sorted by source, kind, and target")
	}
	for _, fact := range snapshot.Facts {
		if !slices.IsSortedFunc(fact.Evidence, func(left, right Evidence) int {
			leftSource := left.(SourceEvidence)
			rightSource := right.(SourceEvidence)
			if result := cmp.Compare(leftSource.FileRef, rightSource.FileRef); result != 0 {
				return result
			}
			return leftSource.StartOffset - rightSource.StartOffset
		}) {
			t.Errorf("evidence is not sorted for fact %#v", fact)
		}
	}
}
