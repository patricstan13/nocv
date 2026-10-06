package goanalyzer

import (
	"context"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"nocv/graph"
	"nocv/query"
)

const implementationCertaintyFixture = `package certainty

type Repository interface {
	RepoHost() string
	RepoName() string
	RepoOwner() string
}

type DirectGood struct{}

func (DirectGood) RepoHost() string  { return "" }
func (DirectGood) RepoName() string  { return "" }
func (DirectGood) RepoOwner() string { return "" }

type Methods struct{}

func (Methods) RepoHost() string  { return "" }
func (Methods) RepoName() string  { return "" }
func (Methods) RepoOwner() string { return "" }

type PromotedGood struct {
	Methods
}

type PointerGood struct{}

func (*PointerGood) RepoHost() string  { return "" }
func (*PointerGood) RepoName() string  { return "" }
func (*PointerGood) RepoOwner() string { return "" }

type ResolvedWithoutMethods struct{}

type UnrelatedResolved struct {
	ResolvedWithoutMethods
}

type MissingAlias = MissingDependency

type ThroughUnresolvedEmbedding struct {
	MissingAlias
}

type ThroughUnresolvedPointerAlias struct {
	*MissingAlias
}

type PartialDirect struct {
	MissingAlias
}

func (PartialDirect) RepoHost() string { return "" }

type InvalidCarrier struct {
	MissingAlias
}

type ThroughRecursiveEmbedding struct {
	InvalidCarrier
}
`

func TestImplementationCertaintyInPartialAnalysis(t *testing.T) {
	analysis, err := LoadAnalysis(context.Background(), testModule(t, map[string]string{
		"certainty.go": implementationCertaintyFixture,
	}), ".")
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Status() != AnalysisPartial {
		t.Fatalf("status = %s, want partial", analysis.Status())
	}

	g := analysis.Graph()
	repository := graph.SymbolRef("example.com/statusfixture::Repository")
	assertImplementationCertainty(t, g, "example.com/statusfixture::DirectGood", repository, graph.RelationshipConfirmed)
	assertImplementationCertainty(t, g, "example.com/statusfixture::PromotedGood", repository, graph.RelationshipConfirmed)
	assertImplementationCertainty(t, g, "example.com/statusfixture::PointerGood", repository, graph.RelationshipConfirmed)
	assertImplementationCertainty(t, g, "example.com/statusfixture::ThroughUnresolvedEmbedding", repository, graph.RelationshipUncertain)
	assertImplementationCertainty(t, g, "example.com/statusfixture::ThroughUnresolvedPointerAlias", repository, graph.RelationshipUncertain)
	assertImplementationCertainty(t, g, "example.com/statusfixture::PartialDirect", repository, graph.RelationshipUncertain)
	assertImplementationCertainty(t, g, "example.com/statusfixture::ThroughRecursiveEmbedding", repository, graph.RelationshipUncertain)
	assertNoImplementation(t, g, "example.com/statusfixture::UnrelatedResolved", repository)

	for _, method := range []string{"RepoHost", "RepoName", "RepoOwner"} {
		assertImplementationCertainty(
			t,
			g,
			graph.SymbolRef("example.com/statusfixture::DirectGood::"+method),
			graph.SymbolRef("example.com/statusfixture::Repository::"+method),
			graph.RelationshipConfirmed,
		)
		assertImplementationCertainty(
			t,
			g,
			graph.SymbolRef("example.com/statusfixture::PointerGood::"+method),
			graph.SymbolRef("example.com/statusfixture::Repository::"+method),
			graph.RelationshipConfirmed,
		)
	}
	if _, exists := g.NodeByRef("example.com/statusfixture::PromotedGood::RepoHost"); exists {
		t.Fatal("promoted method unexpectedly has a synthetic graph node")
	}
	assertImplementationCertainty(
		t,
		g,
		"example.com/statusfixture::PartialDirect::RepoHost",
		"example.com/statusfixture::Repository::RepoHost",
		graph.RelationshipConfirmed,
	)

	relationships := query.DirectDependencies(g, "example.com/statusfixture::ThroughUnresolvedEmbedding")
	if len(relationships) != 1 || relationships[0].Kind != graph.EdgeImplements ||
		relationships[0].Certainty != graph.RelationshipUncertain {
		t.Fatalf("uncertain implementation query result = %#v", relationships)
	}

	for _, node := range g.Nodes() {
		for _, edge := range g.Outgoing(node.ID) {
			if edge.Certainty == graph.RelationshipUncertain && edge.Kind != graph.EdgeImplements {
				t.Errorf("unexpected uncertain %s edge: %#v", edge.Kind, edge)
			}
		}
	}
}

func TestSignatureChangeSeparatesUncertainContractsFromDeterministicImpacts(t *testing.T) {
	analysis, err := LoadAnalysis(context.Background(), testModule(t, map[string]string{
		"certainty.go": implementationCertaintyFixture,
	}), ".")
	if err != nil {
		t.Fatal(err)
	}

	interfaceImpact, err := analysis.AnalyzeSignatureChange(
		"example.com/statusfixture::Repository::RepoHost",
		ProposedSignature{Results: []ProposedResult{{TypeExpr: "int"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, impact := range interfaceImpact.Contracts {
		if impact.Concrete.Ref == "example.com/statusfixture::PartialDirect" ||
			impact.Concrete.Ref == "example.com/statusfixture::ThroughUnresolvedEmbedding" ||
			impact.Concrete.Ref == "example.com/statusfixture::ThroughUnresolvedPointerAlias" ||
			impact.Concrete.Ref == "example.com/statusfixture::ThroughRecursiveEmbedding" {
			t.Fatalf("uncertain implementation reported as deterministic impact: %#v", impact)
		}
	}
	wantUncertain := map[graph.SymbolRef]bool{
		"example.com/statusfixture::InvalidCarrier":                true,
		"example.com/statusfixture::PartialDirect":                 true,
		"example.com/statusfixture::ThroughRecursiveEmbedding":     true,
		"example.com/statusfixture::ThroughUnresolvedEmbedding":    true,
		"example.com/statusfixture::ThroughUnresolvedPointerAlias": true,
	}
	if len(interfaceImpact.UncertainContracts) != len(wantUncertain) {
		t.Fatalf("uncertain interface contracts = %#v, want %d", interfaceImpact.UncertainContracts, len(wantUncertain))
	}
	for _, contract := range interfaceImpact.UncertainContracts {
		if !wantUncertain[contract.Concrete.Ref] || contract.Certainty != graph.RelationshipUncertain {
			t.Errorf("unexpected uncertain contract context: %#v", contract)
		}
	}

	concreteImpact, err := analysis.AnalyzeSignatureChange(
		"example.com/statusfixture::PartialDirect::RepoHost",
		ProposedSignature{Results: []ProposedResult{{TypeExpr: "int"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(concreteImpact.Contracts) != 0 || len(concreteImpact.UncertainContracts) != 1 ||
		concreteImpact.UncertainContracts[0].Concrete.Ref != "example.com/statusfixture::PartialDirect" {
		t.Fatalf("partial concrete method contract result = %#v", concreteImpact)
	}
}

func TestResolvedEmbeddingDoesNotImplement(t *testing.T) {
	resolved := strings.Replace(
		implementationCertaintyFixture,
		"package certainty\n",
		"package certainty\n\ntype MissingDependency struct{}\n",
		1,
	)
	analysis, err := LoadAnalysis(context.Background(), testModule(t, map[string]string{
		"certainty.go": resolved,
	}), ".")
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Status() != AnalysisComplete {
		t.Fatalf("status = %s, want complete", analysis.Status())
	}

	repository := graph.SymbolRef("example.com/statusfixture::Repository")
	assertNoImplementation(t, analysis.Graph(), "example.com/statusfixture::ThroughUnresolvedEmbedding", repository)
	assertNoImplementation(t, analysis.Graph(), "example.com/statusfixture::ThroughUnresolvedPointerAlias", repository)
	assertNoImplementation(t, analysis.Graph(), "example.com/statusfixture::PartialDirect", repository)
	assertNoImplementation(t, analysis.Graph(), "example.com/statusfixture::ThroughRecursiveEmbedding", repository)
}

func TestInvalidEmbeddedTypeTraversalTerminatesOnCycles(t *testing.T) {
	pkg := types.NewPackage("example.com/cycle", "cycle")
	a := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "A", nil), nil, nil)
	b := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "B", nil), nil, nil)
	a.SetUnderlying(types.NewStruct([]*types.Var{
		types.NewField(token.NoPos, pkg, "B", types.NewPointer(b), true),
	}, nil))
	b.SetUnderlying(types.NewStruct([]*types.Var{
		types.NewField(token.NoPos, pkg, "A", types.NewPointer(a), true),
	}, nil))

	if hasInvalidEmbeddedType(a) {
		t.Fatal("valid recursive embedding reported invalid")
	}
}

func assertImplementationCertainty(
	t *testing.T,
	g *graph.Graph,
	fromRef, toRef graph.SymbolRef,
	want graph.RelationshipCertainty,
) {
	t.Helper()
	from, fromExists := g.Resolve(fromRef)
	to, toExists := g.Resolve(toRef)
	if !fromExists || !toExists {
		t.Fatalf("missing implementation endpoint %q -> %q", fromRef, toRef)
	}
	edges := g.Outgoing(from, graph.EdgeImplements)
	for _, edge := range edges {
		if edge.To == to {
			if edge.Certainty != want {
				t.Fatalf("%s -> %s certainty = %s, want %s", fromRef, toRef, edge.Certainty, want)
			}
			return
		}
	}
	t.Fatalf("missing implementation %s -> %s", fromRef, toRef)
}

func assertNoImplementation(t *testing.T, g *graph.Graph, fromRef, toRef graph.SymbolRef) {
	t.Helper()
	from, fromExists := g.Resolve(fromRef)
	to, toExists := g.Resolve(toRef)
	if !fromExists || !toExists {
		t.Fatalf("missing implementation endpoint %q -> %q", fromRef, toRef)
	}
	for _, edge := range g.Outgoing(from, graph.EdgeImplements) {
		if edge.To == to {
			t.Fatalf("unexpected implementation %s -> %s: %#v", fromRef, toRef, edge)
		}
	}
}
