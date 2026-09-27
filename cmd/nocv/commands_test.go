package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nocv/graph"
)

func TestParseInvocationRejectsMissingUnknownAndWrongArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing command", want: "usage: nocv <command>"},
		{name: "unknown command", args: []string{"unknown", "./..."}, want: `unknown command "unknown"`},
		{name: "extra serve argument", args: []string{"serve", "./...", "extra"}, want: "usage: nocv serve <pattern>"},
		{name: "missing path endpoint", args: []string{"paths", "./...", "from"}, want: "usage: nocv paths <pattern> <from-symbol> <to-symbol>"},
		{name: "missing import cycle endpoint", args: []string{"import-cycle", "./...", "from"}, want: "usage: nocv import-cycle <pattern> <from-package> <to-package>"},
		{name: "missing forbidden import endpoint", args: []string{"check-forbidden-import", "./...", "from"}, want: "usage: nocv check-forbidden-import <pattern> <from-package> <to-package>"},
		{name: "missing forbidden dependency endpoint", args: []string{"check-forbidden-dependency", "./...", "from"}, want: "usage: nocv check-forbidden-dependency <pattern> <from-package> <to-package>"},
		{name: "missing type dependency endpoint", args: []string{"type-deps", "./..."}, want: "usage: nocv type-deps <pattern> <type-ref>"},
		{name: "missing type path endpoint", args: []string{"type-paths", "./...", "from"}, want: "usage: nocv type-paths <pattern> <from-type> <to-type>"},
		{name: "missing inspection endpoint", args: []string{"inspect-dependency", "./...", "from"}, want: "usage: nocv inspect-dependency <pattern> <from-package> <to-package>"},
		{name: "missing node inspection symbol", args: []string{"inspect-node", "./..."}, want: "usage: nocv inspect-node <pattern> <symbol-ref>"},
		{name: "extra tree argument", args: []string{"tree", "./...", "extra"}, want: "usage: nocv tree <pattern>"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseInvocation(test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("parseInvocation(%v) error = %v, want containing %q", test.args, err, test.want)
			}
		})
	}
}

func TestRunReturnsUsageStatusWithoutLoading(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run(nil, &stdout, &stderr); status != 2 {
		t.Fatalf("run(nil) status = %d, want 2", status)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage: nocv") {
		t.Fatalf("run(nil) stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestLocalModulePattern(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if gotDir, gotPattern := localModulePattern(filepath.Join(dir, "...")); gotDir != dir || gotPattern != "./..." {
		t.Fatalf("localModulePattern(module/...) = (%q, %q), want (%q, %q)", gotDir, gotPattern, dir, "./...")
	}
	if gotDir, gotPattern := localModulePattern("example.com/remote/..."); gotDir != "" || gotPattern != "example.com/remote/..." {
		t.Fatalf("localModulePattern(remote) = (%q, %q), want unchanged remote pattern", gotDir, gotPattern)
	}
}

func TestExecuteCommandsRenderFocusedDeterministicOutput(t *testing.T) {
	g, ids := cliFixture(t)
	tests := []struct {
		name     string
		values   []string
		want     []string
		unwanted []string
	}{
		{name: "tree", want: []string{"a [package]", "Service [struct]"}, unwanted: []string{"Calls:"}},
		{name: "calls", want: []string{"Calls:", "calls -> example.com/b::Repository::Save", "2 call site(s)"}, unwanted: []string{"Implementations:"}},
		{name: "implementations", want: []string{"Implementations:", "implements -> example.com/b::Repository"}},
		{name: "embeddings", want: []string{"Embeddings:", "embeds -> example.com/a::Base"}},
		{name: "signatures", want: []string{"Signatures:", "accepts -> example.com/b::Repository", "returns -> example.com/a::Base"}},
		{name: "imports", want: []string{"Imports:", "example.com/a", "imports -> example.com/b"}, unwanted: []string{"Package call dependencies:"}},
		{name: "package-imports", values: []string{string(ids.packageA)}, want: []string{"example.com/a", "imports -> example.com/b"}},
		{name: "package-importers", values: []string{string(ids.packageB)}, want: []string{"example.com/b", "<- imports example.com/a"}},
		{name: "import-cycle", values: []string{string(ids.packageB), string(ids.packageA)}, want: []string{"Proposed import:", "example.com/b -> example.com/a", "Would create import cycle.", "existing path 1:", "-> example.com/b"}},
		{name: "import-cycle", values: []string{string(ids.packageA), string(ids.packageB)}, want: []string{"Proposed import:", "No import cycle would be created."}, unwanted: []string{"existing path"}},
		{name: "check-forbidden-import", values: []string{string(ids.packageA), string(ids.packageB)}, want: []string{"Forbidden package import:", "example.com/a -> example.com/b", "VIOLATION", "evidence:", "fixture.go:1"}},
		{name: "check-forbidden-import", values: []string{string(ids.packageB), string(ids.packageA)}, want: []string{"Forbidden package import:", "No violation."}, unwanted: []string{"VIOLATION", "evidence:"}},
		{name: "check-forbidden-dependency", values: []string{string(ids.packageA), string(ids.packageB)}, want: []string{"Forbidden package dependency:", "example.com/a -> example.com/b", "VIOLATION", "path 1:", "evidence:", "calls ->"}},
		{name: "check-forbidden-dependency", values: []string{string(ids.packageB), string(ids.packageA)}, want: []string{"Forbidden package dependency:", "No violation."}, unwanted: []string{"VIOLATION", "path 1:"}},
		{name: "type-deps", values: []string{string(ids.service)}, want: []string{"Type dependencies:", string(ids.service), "-> " + string(ids.repository), "evidence:", "calls ->"}},
		{name: "type-paths", values: []string{string(ids.service), string(ids.repository)}, want: []string{"Type dependency paths:", "path 1:", string(ids.service), "-> " + string(ids.repository), "evidence:"}},
		{name: "inspect-dependency", values: []string{string(ids.packageA), string(ids.packageB)}, want: []string{"Dependency inspection:", "PACKAGE PATH 1", "HOP 1", "TYPE", string(ids.service) + " -> " + string(ids.repository), "EXACT ONLY"}},
		{name: "inspect-node", values: []string{string(ids.service)}, want: []string{"Node inspection:", string(ids.service), "Kind:", "struct", "Parent:", string(ids.packageA), "Documentation:", "(none)", "Methods:", string(ids.caller), "Type dependencies:", string(ids.repository), "Direct semantic dependencies:", "implements ->", "embeds ->"}},
		{name: "direct-deps", values: []string{string(ids.caller)}, want: []string{string(ids.caller), "calls ->", "accepts ->"}},
		{name: "direct-dependents", values: []string{string(ids.repository)}, want: []string{string(ids.repository), "<- implements", "<- accepts"}},
		{name: "impact", values: []string{string(ids.callee)}, want: []string{"Impact of", string(ids.caller), "path 1:"}},
		{name: "paths", values: []string{string(ids.caller), string(ids.callee)}, want: []string{"Dependency paths:", "path 1:", "calls ->"}},
		{name: "package-paths", values: []string{string(ids.packageA), string(ids.packageB)}, want: []string{"Package dependency paths:", "evidence:", "example.com/a", "example.com/b"}},
		{name: "package-deps", want: []string{"Package call dependencies:", "example.com/a -> example.com/b"}},
		{name: "why-package-dep", values: []string{string(ids.packageA), string(ids.packageB)}, want: []string{"Package call dependency explanation:", "calls ->", "at fixture.go:"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invocation := invocation{name: test.name, pattern: "./...", values: test.values}
			var first bytes.Buffer
			if err := executeCommand(&first, g, invocation); err != nil {
				t.Fatalf("executeCommand(%s): %v", test.name, err)
			}
			for _, want := range test.want {
				if !strings.Contains(first.String(), want) {
					t.Errorf("%s output lacks %q:\n%s", test.name, want, first.String())
				}
			}
			for _, unwanted := range test.unwanted {
				if strings.Contains(first.String(), unwanted) {
					t.Errorf("%s output unexpectedly contains %q:\n%s", test.name, unwanted, first.String())
				}
			}

			var second bytes.Buffer
			if err := executeCommand(&second, g, invocation); err != nil {
				t.Fatalf("second executeCommand(%s): %v", test.name, err)
			}
			if second.String() != first.String() {
				t.Errorf("%s output is not deterministic:\nfirst:  %q\nsecond: %q", test.name, first.String(), second.String())
			}
		})
	}
}

func TestExecuteCommandReportsMissingAndInvalidSymbols(t *testing.T) {
	g, ids := cliFixture(t)
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{name: "direct-deps", values: []string{"missing"}, want: "unknown symbol: missing"},
		{name: "paths", values: []string{string(ids.caller), "missing"}, want: "unknown symbol: missing"},
		{name: "package-paths", values: []string{"missing", string(ids.packageB)}, want: "unknown package: missing"},
		{name: "package-paths", values: []string{string(ids.caller), string(ids.packageB)}, want: "symbol is not a package: " + string(ids.caller)},
		{name: "why-package-dep", values: []string{string(ids.packageA), string(ids.repository)}, want: "symbol is not a package: " + string(ids.repository)},
		{name: "package-imports", values: []string{"missing"}, want: "unknown package: missing"},
		{name: "package-importers", values: []string{string(ids.repository)}, want: "symbol is not a package: " + string(ids.repository)},
		{name: "import-cycle", values: []string{"missing", string(ids.packageB)}, want: "unknown package: missing"},
		{name: "import-cycle", values: []string{string(ids.packageA), string(ids.repository)}, want: "symbol is not a package: " + string(ids.repository)},
		{name: "check-forbidden-import", values: []string{"missing", string(ids.packageB)}, want: "unknown package: missing"},
		{name: "check-forbidden-import", values: []string{string(ids.packageA), string(ids.repository)}, want: "symbol is not a package: " + string(ids.repository)},
		{name: "check-forbidden-dependency", values: []string{"missing", string(ids.packageB)}, want: "unknown package: missing"},
		{name: "check-forbidden-dependency", values: []string{string(ids.packageA), string(ids.repository)}, want: "symbol is not a package: " + string(ids.repository)},
		{name: "type-deps", values: []string{"missing"}, want: "unknown symbol: missing"},
		{name: "type-deps", values: []string{string(ids.packageA)}, want: "symbol is not a type: " + string(ids.packageA)},
		{name: "type-paths", values: []string{string(ids.service), string(ids.caller)}, want: "symbol is not a type: " + string(ids.caller)},
		{name: "inspect-dependency", values: []string{"missing", string(ids.packageB)}, want: "unknown package: missing"},
		{name: "inspect-dependency", values: []string{string(ids.packageA), string(ids.repository)}, want: "symbol is not a package: " + string(ids.repository)},
		{name: "inspect-node", values: []string{"missing"}, want: "unknown symbol: missing"},
	}
	for _, test := range tests {
		t.Run(test.name+test.want, func(t *testing.T) {
			err := executeCommand(&bytes.Buffer{}, g, invocation{name: test.name, pattern: "./...", values: test.values})
			if err == nil || err.Error() != test.want {
				t.Fatalf("executeCommand() error = %v, want %q", err, test.want)
			}
		})
	}
}

type cliIDs struct {
	packageA   graph.SymbolRef
	packageB   graph.SymbolRef
	service    graph.SymbolRef
	base       graph.SymbolRef
	repository graph.SymbolRef
	caller     graph.SymbolRef
	callee     graph.SymbolRef
}

func cliFixture(t *testing.T) (*graph.Graph, cliIDs) {
	t.Helper()
	ids := cliIDs{
		packageA:   "example.com/a",
		packageB:   "example.com/b",
		service:    "example.com/a::Service",
		base:       "example.com/a::Base",
		repository: "example.com/b::Repository",
		caller:     "example.com/a::Service::Run",
		callee:     "example.com/b::Repository::Save",
	}
	g := graph.New()
	for _, node := range []fixtureNode{
		{ID: ids.packageA, Kind: graph.NodePackage, Name: "a"},
		{ID: ids.packageB, Kind: graph.NodePackage, Name: "b"},
		{ID: ids.service, Kind: graph.NodeStruct, Name: "Service", Parent: ids.packageA},
		{ID: ids.base, Kind: graph.NodeStruct, Name: "Base", Parent: ids.packageA},
		{ID: ids.repository, Kind: graph.NodeInterface, Name: "Repository", Parent: ids.packageB},
		{ID: ids.caller, Kind: graph.NodeFunction, Name: "Run", Parent: ids.service},
		{ID: ids.callee, Kind: graph.NodeFunction, Name: "Save", Parent: ids.repository},
	} {
		if err := addFixtureNode(g, node); err != nil {
			t.Fatalf("AddNode(%q): %v", node.ID, err)
		}
	}
	edges := []fixtureEdge{
		{From: ids.packageA, To: ids.packageB, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "fixture.go", Offset: 1}}},
		{From: ids.caller, To: ids.callee, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "fixture.go", Offset: 20}, {File: "fixture.go", Offset: 40}}},
		{From: ids.caller, To: ids.callee, Kind: graph.EdgeImplements, Evidence: []graph.Location{{File: "fixture.go", Offset: 10}}},
		{From: ids.service, To: ids.repository, Kind: graph.EdgeImplements, Evidence: []graph.Location{{File: "fixture.go", Offset: 5}}},
		{From: ids.service, To: ids.base, Kind: graph.EdgeEmbeds, Evidence: []graph.Location{{File: "fixture.go", Offset: 6}}},
		{From: ids.caller, To: ids.repository, Kind: graph.EdgeAccepts, Evidence: []graph.Location{{File: "fixture.go", Offset: 7}}},
		{From: ids.caller, To: ids.base, Kind: graph.EdgeReturns, Evidence: []graph.Location{{File: "fixture.go", Offset: 8}}},
	}
	for _, edge := range edges {
		if err := addFixtureEdge(g, edge); err != nil {
			t.Fatalf("AddEdge(%s, %q -> %q): %v", edge.Kind, edge.From, edge.To, err)
		}
	}
	return g, ids
}
