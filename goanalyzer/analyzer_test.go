package goanalyzer

import (
	"context"
	"strings"
	"testing"

	"nocv/graph"
	"nocv/internal/testutil"
)

func TestCanonicalAnalysisProducesOnlyConfirmedRelationships(t *testing.T) {
	g, err := Load(context.Background(), testutil.GoProjectDir(t), "./...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	edgeCount := 0
	for _, node := range g.Nodes() {
		for _, edge := range g.Outgoing(node.ID) {
			edgeCount++
			if edge.Certainty != graph.RelationshipConfirmed {
				t.Errorf("edge %d -> %d (%s) certainty = %s, want confirmed", edge.From, edge.To, edge.Kind, edge.Certainty)
			}
		}
	}
	if edgeCount == 0 {
		t.Fatal("canonical analysis produced no relationships")
	}
}

func TestLoadDiscoversStructuralHierarchy(t *testing.T) {
	g, err := Load(context.Background(), testutil.GoProjectDir(t), "./app", "./inventory", "./logging", "./orders", "./repository", "./service")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	want := map[graph.SymbolRef]struct {
		kind   graph.NodeKind
		name   string
		parent graph.SymbolRef
	}{
		"example.com/shop/inventory":                        {graph.NodePackage, "inventory", ""},
		"example.com/shop/inventory::Item":                  {graph.NodeStruct, "Item", "example.com/shop/inventory"},
		"example.com/shop/app":                              {graph.NodePackage, "app", ""},
		"example.com/shop/app::Run":                         {graph.NodeFunction, "Run", "example.com/shop/app"},
		"example.com/shop/logging":                          {graph.NodePackage, "logging", ""},
		"example.com/shop/logging::Info":                    {graph.NodeFunction, "Info", "example.com/shop/logging"},
		"example.com/shop/repository":                       {graph.NodePackage, "repository", ""},
		"example.com/shop/repository::Save":                 {graph.NodeFunction, "Save", "example.com/shop/repository"},
		"example.com/shop/service":                          {graph.NodePackage, "service", ""},
		"example.com/shop/service::Create":                  {graph.NodeFunction, "Create", "example.com/shop/service"},
		"example.com/shop/service::Update":                  {graph.NodeFunction, "Update", "example.com/shop/service"},
		"example.com/shop/orders":                           {graph.NodePackage, "orders", ""},
		"example.com/shop/orders::ConvertOnly":              {graph.NodeFunction, "ConvertOnly", "example.com/shop/orders"},
		"example.com/shop/orders::ExternalOnly":             {graph.NodeFunction, "ExternalOnly", "example.com/shop/orders"},
		"example.com/shop/orders::Process":                  {graph.NodeFunction, "Process", "example.com/shop/orders"},
		"example.com/shop/orders::Validate":                 {graph.NodeFunction, "Validate", "example.com/shop/orders"},
		"example.com/shop/orders::Order":                    {graph.NodeStruct, "Order", "example.com/shop/orders"},
		"example.com/shop/orders::Repository":               {graph.NodeInterface, "Repository", "example.com/shop/orders"},
		"example.com/shop/orders::Repository::Save":         {graph.NodeFunction, "Save", "example.com/shop/orders::Repository"},
		"example.com/shop/orders::Processor":                {graph.NodeInterface, "Processor", "example.com/shop/orders"},
		"example.com/shop/orders::Processor::Process":       {graph.NodeFunction, "Process", "example.com/shop/orders::Processor"},
		"example.com/shop/orders::Service":                  {graph.NodeStruct, "Service", "example.com/shop/orders"},
		"example.com/shop/orders::Service::Create":          {graph.NodeFunction, "Create", "example.com/shop/orders::Service"},
		"example.com/shop/orders::Service::ClosureCalls":    {graph.NodeFunction, "ClosureCalls", "example.com/shop/orders::Service"},
		"example.com/shop/orders::Service::Health":          {graph.NodeFunction, "Health", "example.com/shop/orders::Service"},
		"example.com/shop/orders::Service::Transform":       {graph.NodeFunction, "Transform", "example.com/shop/orders::Service"},
		"example.com/shop/orders::Service::Validate":        {graph.NodeFunction, "Validate", "example.com/shop/orders::Service"},
		"example.com/shop/orders::NewService":               {graph.NodeFunction, "NewService", "example.com/shop/orders"},
		"example.com/shop/orders::Box":                      {graph.NodeStruct, "Box", "example.com/shop/orders"},
		"example.com/shop/orders::Box::Get":                 {graph.NodeFunction, "Get", "example.com/shop/orders::Box"},
		"example.com/shop/orders::Outer":                    {graph.NodeFunction, "Outer", "example.com/shop/orders"},
		"example.com/shop/orders::NestedClosures":           {graph.NodeFunction, "NestedClosures", "example.com/shop/orders"},
		"example.com/shop/orders::PostgresRepository":       {graph.NodeStruct, "PostgresRepository", "example.com/shop/orders"},
		"example.com/shop/orders::PostgresRepository::Save": {graph.NodeFunction, "Save", "example.com/shop/orders::PostgresRepository"},
		"example.com/shop/orders::MemoryRepository":         {graph.NodeStruct, "MemoryRepository", "example.com/shop/orders"},
		"example.com/shop/orders::MemoryRepository::Save":   {graph.NodeFunction, "Save", "example.com/shop/orders::MemoryRepository"},
		"example.com/shop/orders::BrokenRepository":         {graph.NodeStruct, "BrokenRepository", "example.com/shop/orders"},
		"example.com/shop/orders::BaseRepository":           {graph.NodeStruct, "BaseRepository", "example.com/shop/orders"},
		"example.com/shop/orders::BaseRepository::Save":     {graph.NodeFunction, "Save", "example.com/shop/orders::BaseRepository"},
		"example.com/shop/orders::PromotedRepository":       {graph.NodeStruct, "PromotedRepository", "example.com/shop/orders"},
		"example.com/shop/orders::Empty":                    {graph.NodeInterface, "Empty", "example.com/shop/orders"},
		"example.com/shop/orders::ExternalStringer":         {graph.NodeStruct, "ExternalStringer", "example.com/shop/orders"},
		"example.com/shop/orders::ExternalStringer::String": {graph.NodeFunction, "String", "example.com/shop/orders::ExternalStringer"},
		"example.com/shop/orders::EmbeddedBase":             {graph.NodeStruct, "EmbeddedBase", "example.com/shop/orders"},
		"example.com/shop/orders::EmbeddedChild":            {graph.NodeStruct, "EmbeddedChild", "example.com/shop/orders"},
		"example.com/shop/orders::PointerEmbeddedChild":     {graph.NodeStruct, "PointerEmbeddedChild", "example.com/shop/orders"},
		"example.com/shop/orders::NamedFieldChild":          {graph.NodeStruct, "NamedFieldChild", "example.com/shop/orders"},
		"example.com/shop/orders::Holder":                   {graph.NodeStruct, "Holder", "example.com/shop/orders"},
		"example.com/shop/orders::Reader":                   {graph.NodeInterface, "Reader", "example.com/shop/orders"},
		"example.com/shop/orders::Reader::Read":             {graph.NodeFunction, "Read", "example.com/shop/orders::Reader"},
		"example.com/shop/orders::Writer":                   {graph.NodeInterface, "Writer", "example.com/shop/orders"},
		"example.com/shop/orders::Writer::Write":            {graph.NodeFunction, "Write", "example.com/shop/orders::Writer"},
		"example.com/shop/orders::ReadWriter":               {graph.NodeInterface, "ReadWriter", "example.com/shop/orders"},
		"example.com/shop/orders::Number":                   {graph.NodeInterface, "Number", "example.com/shop/orders"},
		"example.com/shop/orders::HTTPWrapper":              {graph.NodeStruct, "HTTPWrapper", "example.com/shop/orders"},
		"example.com/shop/orders::ExternalReader":           {graph.NodeInterface, "ExternalReader", "example.com/shop/orders"},
		"example.com/shop/orders::HandleOrder":              {graph.NodeFunction, "HandleOrder", "example.com/shop/orders"},
		"example.com/shop/orders::LoadPair":                 {graph.NodeFunction, "LoadPair", "example.com/shop/orders"},
		"example.com/shop/orders::CompareOrders":            {graph.NodeFunction, "CompareOrders", "example.com/shop/orders"},
		"example.com/shop/orders::PointerOrder":             {graph.NodeFunction, "PointerOrder", "example.com/shop/orders"},
		"example.com/shop/orders::ExternalSignature":        {graph.NodeFunction, "ExternalSignature", "example.com/shop/orders"},
		"example.com/shop/orders::ContainerSignature":       {graph.NodeFunction, "ContainerSignature", "example.com/shop/orders"},
		"example.com/shop/orders::VariadicOrders":           {graph.NodeFunction, "VariadicOrders", "example.com/shop/orders"},
		"example.com/shop/orders::FindUser":                 {graph.NodeFunction, "FindUser", "example.com/shop/orders"},
		"example.com/shop/orders::AliasOrder":               {graph.NodeFunction, "AliasOrder", "example.com/shop/orders"},
		"example.com/shop/orders::GenericBox":               {graph.NodeFunction, "GenericBox", "example.com/shop/orders"},
		"example.com/shop/orders::RunCreate":                {graph.NodeFunction, "RunCreate", "example.com/shop/orders"},
	}

	if got := len(g.Nodes()); got != len(want) {
		t.Fatalf("node count = %d, want %d; nodes: %#v", got, len(want), g.Nodes())
	}
	for id, expected := range want {
		node, ok := nodeByRef(g, id)
		if !ok {
			t.Errorf("missing node %q", id)
			continue
		}
		actualParent := parentRef(g, node)
		if node.Kind != expected.kind || node.Name != expected.name || actualParent != expected.parent {
			t.Errorf("node %q = (%s, %q, parent %q), want (%s, %q, parent %q)",
				id, node.Kind, node.Name, actualParent, expected.kind, expected.name, expected.parent)
		}
		if node.Kind == graph.NodePackage {
			if node.Location != (graph.Location{}) {
				t.Errorf("package node %q has declaration location %#v", id, node.Location)
			}
		} else if node.Location.File == "" || node.Location.Line < 1 || node.Location.Column < 1 {
			t.Errorf("declaration node %q has invalid location %#v", id, node.Location)
		}
	}

	assertChildren(t, g, "example.com/shop/orders", []graph.SymbolRef{
		"example.com/shop/orders::Order",
		"example.com/shop/orders::Repository",
		"example.com/shop/orders::Processor",
		"example.com/shop/orders::Service",
		"example.com/shop/orders::Box",
		"example.com/shop/orders::PostgresRepository",
		"example.com/shop/orders::MemoryRepository",
		"example.com/shop/orders::BrokenRepository",
		"example.com/shop/orders::BaseRepository",
		"example.com/shop/orders::PromotedRepository",
		"example.com/shop/orders::Empty",
		"example.com/shop/orders::ExternalStringer",
		"example.com/shop/orders::EmbeddedBase",
		"example.com/shop/orders::EmbeddedChild",
		"example.com/shop/orders::PointerEmbeddedChild",
		"example.com/shop/orders::NamedFieldChild",
		"example.com/shop/orders::Holder",
		"example.com/shop/orders::Reader",
		"example.com/shop/orders::Writer",
		"example.com/shop/orders::ReadWriter",
		"example.com/shop/orders::Number",
		"example.com/shop/orders::HTTPWrapper",
		"example.com/shop/orders::ExternalReader",
		"example.com/shop/orders::NewService",
		"example.com/shop/orders::RunCreate",
		"example.com/shop/orders::Process",
		"example.com/shop/orders::Validate",
		"example.com/shop/orders::ExternalOnly",
		"example.com/shop/orders::ConvertOnly",
		"example.com/shop/orders::Outer",
		"example.com/shop/orders::NestedClosures",
		"example.com/shop/orders::HandleOrder",
		"example.com/shop/orders::LoadPair",
		"example.com/shop/orders::CompareOrders",
		"example.com/shop/orders::PointerOrder",
		"example.com/shop/orders::ExternalSignature",
		"example.com/shop/orders::ContainerSignature",
		"example.com/shop/orders::VariadicOrders",
		"example.com/shop/orders::FindUser",
		"example.com/shop/orders::AliasOrder",
		"example.com/shop/orders::GenericBox",
	})
	assertChildren(t, g, "example.com/shop/orders::Repository", []graph.SymbolRef{
		"example.com/shop/orders::Repository::Save",
	})
	assertChildren(t, g, "example.com/shop/orders::Service", []graph.SymbolRef{
		"example.com/shop/orders::Service::Health",
		"example.com/shop/orders::Service::Validate",
		"example.com/shop/orders::Service::Create",
		"example.com/shop/orders::Service::Transform",
		"example.com/shop/orders::Service::ClosureCalls",
	})
	assertChildren(t, g, "example.com/shop/orders::Processor", []graph.SymbolRef{
		"example.com/shop/orders::Processor::Process",
	})

	assertChildren(t, g, "example.com/shop/orders::Box", []graph.SymbolRef{
		"example.com/shop/orders::Box::Get",
	})
	assertChildren(t, g, "example.com/shop/orders::PostgresRepository", []graph.SymbolRef{
		"example.com/shop/orders::PostgresRepository::Save",
	})
	assertChildren(t, g, "example.com/shop/orders::MemoryRepository", []graph.SymbolRef{
		"example.com/shop/orders::MemoryRepository::Save",
	})
	assertChildren(t, g, "example.com/shop/orders::BaseRepository", []graph.SymbolRef{
		"example.com/shop/orders::BaseRepository::Save",
	})
	assertChildren(t, g, "example.com/shop/orders::Reader", []graph.SymbolRef{
		"example.com/shop/orders::Reader::Read",
	})
	assertChildren(t, g, "example.com/shop/orders::Writer", []graph.SymbolRef{
		"example.com/shop/orders::Writer::Write",
	})
}

func TestLoadReportsSourceLineAndColumn(t *testing.T) {
	g, err := Load(context.Background(), testutil.GoProjectDir(t), "./typeview/...")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	service, exists := nodeByRef(g, "example.com/shop/typeview/service::Service")
	if !exists {
		t.Fatal("missing Service node")
	}
	if service.Location.Line != 5 || service.Location.Column != 6 || !strings.HasSuffix(service.Location.File, "/typeview/service/service.go") {
		t.Errorf("Service location = %#v, want service.go:5:6", service.Location)
	}

	create, exists := nodeByRef(g, "example.com/shop/typeview/service::Service::Create")
	if !exists {
		t.Fatal("missing Service.Create node")
	}
	if create.Location.Line != 7 || create.Location.Column != 16 || !strings.HasSuffix(create.Location.File, "/typeview/service/service.go") {
		t.Errorf("Service.Create location = %#v, want service.go:7:16", create.Location)
	}

	calls := outgoingByRef(g, "example.com/shop/typeview/service::Service::Create", graph.EdgeCalls)
	if len(calls) != 1 || len(calls[0].Evidence) != 1 {
		t.Fatalf("Service.Create calls = %#v, want one call with one evidence location", calls)
	}
	evidence := calls[0].Evidence[0]
	if evidence.Line != 8 || evidence.Column != 2 || !strings.HasSuffix(evidence.File, "/typeview/service/service.go") {
		t.Errorf("Service.Create call evidence = %#v, want service.go:8:2", evidence)
	}
}

func TestLoadDiscoversCalls(t *testing.T) {
	g, err := Load(context.Background(), testutil.GoProjectDir(t), "./app", "./inventory", "./logging", "./orders", "./repository", "./service")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	assertCall(t, g,
		"example.com/shop/orders::Process",
		"example.com/shop/orders::Validate",
		2,
	)
	assertCall(t, g,
		"example.com/shop/orders::Service::Create",
		"example.com/shop/orders::Service::Health",
		1,
	)
	assertCall(t, g,
		"example.com/shop/orders::Service::Create",
		"example.com/shop/orders::Service::Validate",
		1,
	)
	assertCall(t, g,
		"example.com/shop/orders::Service::Create",
		"example.com/shop/orders::Repository::Save",
		1,
	)
	assertCall(t, g,
		"example.com/shop/orders::Process",
		"example.com/shop/logging::Info",
		1,
	)

	incoming := incomingByRef(g, "example.com/shop/orders::Repository::Save", graph.EdgeCalls)
	wantRepositoryCallers := map[graph.SymbolRef]bool{
		"example.com/shop/orders::Service::ClosureCalls": true,
		"example.com/shop/orders::Service::Create":       true,
	}
	if len(incoming) != len(wantRepositoryCallers) {
		t.Fatalf("incoming calls to Repository.Save = %#v, want two fixture callers", incoming)
	}
	for _, edge := range incoming {
		if !wantRepositoryCallers[edge.From] {
			t.Errorf("unexpected Repository.Save caller %q", edge.From)
		}
	}

	assertCall(t, g,
		"example.com/shop/orders::Outer",
		"example.com/shop/orders::Validate",
		1,
	)
	outerNode, _ := nodeByRef(g, "example.com/shop/orders::Outer")
	outerCall := outgoingByRef(g, "example.com/shop/orders::Outer", graph.EdgeCalls)[0]
	if outerCall.Evidence[0] == outerNode.Location || outerCall.Evidence[0].Line <= outerNode.Location.Line {
		t.Errorf("Outer call evidence = %#v, want the nested Validate() call site after declaration %#v", outerCall.Evidence, outerNode.Location)
	}
	assertCall(t, g,
		"example.com/shop/orders::NestedClosures",
		"example.com/shop/orders::Validate",
		3,
	)
	assertCall(t, g,
		"example.com/shop/orders::Service::ClosureCalls",
		"example.com/shop/orders::Service::Health",
		1,
	)
	assertCall(t, g,
		"example.com/shop/orders::Service::ClosureCalls",
		"example.com/shop/orders::Repository::Save",
		1,
	)
	if edges := outgoingByRef(g, "example.com/shop/orders::Service::ClosureCalls", graph.EdgeCalls); len(edges) != 2 {
		t.Fatalf("ClosureCalls edges = %#v, want only resolved project method and interface calls", edges)
	}

	for _, id := range []graph.SymbolRef{
		"example.com/shop/orders::ExternalOnly",
		"example.com/shop/orders::ConvertOnly",
	} {
		if edges := outgoingByRef(g, id, graph.EdgeCalls); len(edges) != 0 {
			t.Errorf("Outgoing(%q) = %#v, want no calls for builtins, external functions, or conversions", id, edges)
		}
	}
	for _, id := range []graph.SymbolRef{"fmt", "fmt::Println", "builtin::len", "builtin::append", "builtin::string"} {
		if node, exists := nodeByRef(g, id); exists {
			t.Errorf("external or builtin node %q unexpectedly exists: %#v", id, node)
		}
	}
	for _, node := range g.Nodes() {
		if strings.Contains(string(node.Ref), "::<closure") || strings.Contains(string(node.Ref), "::<func@") {
			t.Errorf("synthetic anonymous-function node unexpectedly exists: %#v", node)
		}
	}
}

func TestLoadDiscoversInterfaceImplementations(t *testing.T) {
	g, err := Load(context.Background(), testutil.GoProjectDir(t), "./app", "./inventory", "./logging", "./orders", "./repository", "./service")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	repositoryID := graph.SymbolRef("example.com/shop/orders::Repository")
	repositorySaveID := graph.SymbolRef("example.com/shop/orders::Repository::Save")
	wantImplementations := map[graph.SymbolRef]bool{
		"example.com/shop/orders::BaseRepository":     true,
		"example.com/shop/orders::MemoryRepository":   true,
		"example.com/shop/orders::PostgresRepository": true,
		"example.com/shop/orders::PromotedRepository": true,
	}
	typeEdges := incomingByRef(g, repositoryID, graph.EdgeImplements)
	if len(typeEdges) != len(wantImplementations) {
		t.Fatalf("Repository implementations = %#v, want %d", typeEdges, len(wantImplementations))
	}
	for _, edge := range typeEdges {
		if !wantImplementations[edge.From] {
			t.Errorf("unexpected Repository implementation %q", edge.From)
		}
		assertValidEvidence(t, edge)
	}

	wantMethods := map[graph.SymbolRef]bool{
		"example.com/shop/orders::BaseRepository::Save":     true,
		"example.com/shop/orders::MemoryRepository::Save":   true,
		"example.com/shop/orders::PostgresRepository::Save": true,
	}
	methodEdges := incomingByRef(g, repositorySaveID, graph.EdgeImplements)
	if len(methodEdges) != len(wantMethods) {
		t.Fatalf("Repository.Save implementations = %#v, want %d", methodEdges, len(wantMethods))
	}
	for _, edge := range methodEdges {
		if !wantMethods[edge.From] {
			t.Errorf("unexpected Repository.Save implementation %q", edge.From)
		}
		assertValidEvidence(t, edge)
	}

	for _, id := range []graph.SymbolRef{
		"example.com/shop/orders::BrokenRepository",
		"example.com/shop/orders::ExternalStringer",
	} {
		if edges := outgoingByRef(g, id, graph.EdgeImplements); len(edges) != 0 {
			t.Errorf("Outgoing(%q, implements) = %#v, want none", id, edges)
		}
	}
	if edges := incomingByRef(g, "example.com/shop/orders::Empty", graph.EdgeImplements); len(edges) != 0 {
		t.Errorf("empty-interface implementations = %#v, want none", edges)
	}
	if _, exists := nodeByRef(g, "fmt::Stringer"); exists {
		t.Error("external fmt.Stringer unexpectedly has a graph node")
	}
	if _, exists := nodeByRef(g, "example.com/shop/orders::PromotedRepository::Save"); exists {
		t.Error("promoted Save unexpectedly has a synthetic method node")
	}
	if edges := outgoingByRef(g, "example.com/shop/orders::PromotedRepository", graph.EdgeImplements); len(edges) != 1 || edges[0].To != repositoryID {
		t.Fatalf("promoted-method type implementation = %#v, want Repository", edges)
	}
}

func TestLoadDiscoversEmbeddings(t *testing.T) {
	g, err := Load(context.Background(), testutil.GoProjectDir(t), "./app", "./inventory", "./logging", "./orders", "./repository", "./service")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	wantOutgoing := map[graph.SymbolRef][]graph.SymbolRef{
		"example.com/shop/orders::PromotedRepository":   {"example.com/shop/orders::BaseRepository"},
		"example.com/shop/orders::EmbeddedChild":        {"example.com/shop/orders::EmbeddedBase"},
		"example.com/shop/orders::PointerEmbeddedChild": {"example.com/shop/orders::EmbeddedBase"},
		"example.com/shop/orders::Holder":               {"example.com/shop/orders::Box"},
		"example.com/shop/orders::ReadWriter": {
			"example.com/shop/orders::Reader",
			"example.com/shop/orders::Writer",
		},
	}
	for from, wantTargets := range wantOutgoing {
		edges := outgoingByRef(g, from, graph.EdgeEmbeds)
		if len(edges) != len(wantTargets) {
			t.Fatalf("embeddings from %q = %#v, want %v", from, edges, wantTargets)
		}
		for index, edge := range edges {
			if edge.To != wantTargets[index] {
				t.Errorf("embedding %d from %q targets %q, want %q", index, from, edge.To, wantTargets[index])
			}
			assertValidEvidence(t, edge)
		}
	}

	incoming := incomingByRef(g, "example.com/shop/orders::EmbeddedBase", graph.EdgeEmbeds)
	if len(incoming) != 2 {
		t.Fatalf("incoming EmbeddedBase embeddings = %#v, want two", incoming)
	}
	wantSources := map[graph.SymbolRef]bool{
		"example.com/shop/orders::EmbeddedChild":        true,
		"example.com/shop/orders::PointerEmbeddedChild": true,
	}
	for _, edge := range incoming {
		if !wantSources[edge.From] {
			t.Errorf("unexpected EmbeddedBase embedding source %q", edge.From)
		}
	}

	for _, id := range []graph.SymbolRef{
		"example.com/shop/orders::NamedFieldChild",
		"example.com/shop/orders::Reader",
		"example.com/shop/orders::Writer",
		"example.com/shop/orders::Number",
		"example.com/shop/orders::HTTPWrapper",
		"example.com/shop/orders::ExternalReader",
	} {
		if edges := outgoingByRef(g, id, graph.EdgeEmbeds); len(edges) != 0 {
			t.Errorf("Outgoing(%q, embeds) = %#v, want none", id, edges)
		}
	}
	for _, id := range []graph.SymbolRef{
		"net/http::Client",
		"io::Reader",
		"example.com/shop/orders::Box[int]",
	} {
		if node, exists := nodeByRef(g, id); exists {
			t.Errorf("synthetic or external node %q unexpectedly exists: %#v", id, node)
		}
	}

	// Existing semantic passes remain intact when embedding edges are added.
	assertCall(t, g,
		"example.com/shop/orders::Service::Create",
		"example.com/shop/orders::Repository::Save",
		1,
	)
	if edges := outgoingByRef(g, "example.com/shop/orders::PromotedRepository", graph.EdgeImplements); len(edges) != 1 {
		t.Fatalf("PromotedRepository implementation edges = %#v, want one", edges)
	}
}

func TestLoadDiscoversSignatureRelationships(t *testing.T) {
	g, err := Load(context.Background(), testutil.GoProjectDir(t), "./app", "./inventory", "./logging", "./orders", "./repository", "./service")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	orderID := graph.SymbolRef("example.com/shop/orders::Order")
	repositoryID := graph.SymbolRef("example.com/shop/orders::Repository")
	boxID := graph.SymbolRef("example.com/shop/orders::Box")

	// Package functions cover multiple parameters/results and pointer normalization.
	assertRelationship(t, g, "example.com/shop/orders::HandleOrder", orderID, graph.EdgeAccepts, 1)
	assertRelationship(t, g, "example.com/shop/orders::HandleOrder", repositoryID, graph.EdgeAccepts, 1)
	assertRelationship(t, g, "example.com/shop/orders::HandleOrder", orderID, graph.EdgeReturns, 1)
	assertRelationship(t, g, "example.com/shop/orders::LoadPair", orderID, graph.EdgeReturns, 1)
	assertRelationship(t, g, "example.com/shop/orders::LoadPair", repositoryID, graph.EdgeReturns, 1)
	assertRelationship(t, g, "example.com/shop/orders::PointerOrder", orderID, graph.EdgeAccepts, 1)
	assertRelationship(t, g, "example.com/shop/orders::PointerOrder", orderID, graph.EdgeReturns, 1)

	// Repeated direct occurrences aggregate into one edge with both locations.
	assertRelationship(t, g, "example.com/shop/orders::CompareOrders", orderID, graph.EdgeAccepts, 2)

	// Concrete and interface methods use the same signature pass; receivers do not.
	assertRelationship(t, g, "example.com/shop/orders::Service::Transform", orderID, graph.EdgeAccepts, 1)
	assertRelationship(t, g, "example.com/shop/orders::Service::Transform", repositoryID, graph.EdgeAccepts, 1)
	assertRelationship(t, g, "example.com/shop/orders::Service::Transform", repositoryID, graph.EdgeReturns, 1)
	assertRelationship(t, g, "example.com/shop/orders::Processor::Process", orderID, graph.EdgeAccepts, 1)
	assertRelationship(t, g, "example.com/shop/orders::Processor::Process", repositoryID, graph.EdgeAccepts, 1)
	assertRelationship(t, g, "example.com/shop/orders::Processor::Process", repositoryID, graph.EdgeReturns, 1)
	if edges := outgoingByRef(g, "example.com/shop/orders::Service::Health", graph.EdgeAccepts); len(edges) != 0 {
		t.Fatalf("receiver unexpectedly produced accepts edges: %#v", edges)
	}

	// Aliases and direct generic instances normalize to represented declarations.
	assertRelationship(t, g, "example.com/shop/orders::AliasOrder", orderID, graph.EdgeAccepts, 1)
	assertRelationship(t, g, "example.com/shop/orders::AliasOrder", orderID, graph.EdgeReturns, 1)
	assertRelationship(t, g, "example.com/shop/orders::GenericBox", boxID, graph.EdgeAccepts, 1)
	assertRelationship(t, g, "example.com/shop/orders::GenericBox", boxID, graph.EdgeReturns, 1)

	// Builtins, external declarations, containers, variadics, and unrepresented
	// named scalar types are deliberately outside direct signature semantics.
	for _, id := range []graph.SymbolRef{
		"example.com/shop/orders::ConvertOnly",
		"example.com/shop/orders::ExternalSignature",
		"example.com/shop/orders::ContainerSignature",
		"example.com/shop/orders::VariadicOrders",
		"example.com/shop/orders::FindUser",
	} {
		if edges := outgoingByRef(g, id, graph.EdgeAccepts, graph.EdgeReturns); len(edges) != 0 {
			t.Errorf("signature relationships from %q = %#v, want none", id, edges)
		}
	}
	for _, id := range []graph.SymbolRef{
		"context::Context",
		"net/http::Request",
		"example.com/shop/orders::UserID",
		"example.com/shop/orders::Purchase",
		"example.com/shop/orders::Box[int]",
	} {
		if node, exists := nodeByRef(g, id); exists {
			t.Errorf("external or synthetic signature node %q unexpectedly exists: %#v", id, node)
		}
	}

	// Existing relationship passes remain intact.
	assertCall(t, g, "example.com/shop/orders::Process", "example.com/shop/orders::Validate", 2)
	if edges := outgoingByRef(g, "example.com/shop/orders::PromotedRepository", graph.EdgeImplements); len(edges) != 1 {
		t.Fatalf("PromotedRepository implementation edges = %#v, want one", edges)
	}
	if edges := outgoingByRef(g, "example.com/shop/orders::EmbeddedChild", graph.EdgeEmbeds); len(edges) != 1 {
		t.Fatalf("EmbeddedChild embedding edges = %#v, want one", edges)
	}
}

func TestLoadProducesDeterministicIDs(t *testing.T) {
	dir := testutil.GoProjectDir(t)
	first, err := Load(context.Background(), dir, "./...")
	if err != nil {
		t.Fatalf("first Load(): %v", err)
	}
	second, err := Load(context.Background(), dir, "./...")
	if err != nil {
		t.Fatalf("second Load(): %v", err)
	}

	firstNodes, secondNodes := first.Nodes(), second.Nodes()
	if len(firstNodes) != len(secondNodes) {
		t.Fatalf("node counts differ: %d and %d", len(firstNodes), len(secondNodes))
	}
	for i := range firstNodes {
		if firstNodes[i].ID != secondNodes[i].ID {
			t.Fatalf("ID %d differs: %d and %d", i, firstNodes[i].ID, secondNodes[i].ID)
		}
	}
}

func assertChildren(t *testing.T, g *graph.Graph, parent graph.SymbolRef, want []graph.SymbolRef) {
	t.Helper()
	parentID, exists := g.Resolve(parent)
	if !exists {
		t.Fatalf("missing parent %q", parent)
	}
	childIDs := g.Children(parentID)
	got := make([]graph.SymbolRef, 0, len(childIDs))
	for _, childID := range childIDs {
		child, _ := g.Node(childID)
		got = append(got, child.Ref)
	}
	if len(got) != len(want) {
		t.Fatalf("Children(%q) = %v, want %v", parent, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Children(%q)[%d] = %q, want %q", parent, i, got[i], want[i])
		}
	}
}

func assertCall(t *testing.T, g *graph.Graph, from, to graph.SymbolRef, evidenceCount int) {
	t.Helper()
	for _, edge := range outgoingByRef(g, from, graph.EdgeCalls) {
		if edge.To != to {
			continue
		}
		if len(edge.Evidence) != evidenceCount {
			t.Fatalf("call %q -> %q has %d evidence locations, want %d", from, to, len(edge.Evidence), evidenceCount)
		}
		for _, evidence := range edge.Evidence {
			if evidence.File == "" || evidence.Line < 1 || evidence.Column < 1 {
				t.Errorf("call %q -> %q has invalid evidence %#v", from, to, evidence)
			}
		}
		return
	}
	t.Fatalf("missing call %q -> %q; outgoing: %#v", from, to, outgoingByRef(g, from, graph.EdgeCalls))
}

func assertRelationship(
	t *testing.T,
	g *graph.Graph,
	from, to graph.SymbolRef,
	kind graph.EdgeKind,
	evidenceCount int,
) {
	t.Helper()
	for _, edge := range outgoingByRef(g, from, kind) {
		if edge.To != to {
			continue
		}
		if len(edge.Evidence) != evidenceCount {
			t.Fatalf("%s relationship %q -> %q has %d evidence locations, want %d", kind, from, to, len(edge.Evidence), evidenceCount)
		}
		for _, evidence := range edge.Evidence {
			if evidence.File == "" || evidence.Line < 1 || evidence.Column < 1 {
				t.Errorf("%s relationship %q -> %q has invalid evidence %#v", kind, from, to, evidence)
			}
		}
		return
	}
	t.Fatalf("missing %s relationship %q -> %q; outgoing: %#v", kind, from, to, outgoingByRef(g, from, kind))
}

func assertValidEvidence(t *testing.T, edge *symbolEdge) {
	t.Helper()
	if len(edge.Evidence) != 1 || edge.Evidence[0].File == "" || edge.Evidence[0].Line < 1 || edge.Evidence[0].Column < 1 {
		t.Errorf("edge %q -> %q has invalid evidence %#v", edge.From, edge.To, edge.Evidence)
	}
}

type symbolEdge struct {
	From     graph.SymbolRef
	To       graph.SymbolRef
	Kind     graph.EdgeKind
	Evidence []graph.Location
}

func nodeByRef(g *graph.Graph, ref graph.SymbolRef) (*graph.Node, bool) {
	return g.NodeByRef(ref)
}

func outgoingByRef(g *graph.Graph, ref graph.SymbolRef, kinds ...graph.EdgeKind) []*symbolEdge {
	id, exists := g.Resolve(ref)
	if !exists {
		return nil
	}
	return presentEdges(g, g.Outgoing(id, kinds...))
}

func incomingByRef(g *graph.Graph, ref graph.SymbolRef, kinds ...graph.EdgeKind) []*symbolEdge {
	id, exists := g.Resolve(ref)
	if !exists {
		return nil
	}
	return presentEdges(g, g.Incoming(id, kinds...))
}

func presentEdges(g *graph.Graph, edges []*graph.Edge) []*symbolEdge {
	result := make([]*symbolEdge, 0, len(edges))
	for _, edge := range edges {
		from, _ := g.Node(edge.From)
		to, _ := g.Node(edge.To)
		result = append(result, &symbolEdge{From: from.Ref, To: to.Ref, Kind: edge.Kind, Evidence: edge.Evidence})
	}
	return result
}

func parentRef(g *graph.Graph, node *graph.Node) graph.SymbolRef {
	if node == nil || node.Parent == 0 {
		return ""
	}
	parent, _ := g.Node(node.Parent)
	return parent.Ref
}
