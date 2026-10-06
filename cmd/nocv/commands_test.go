package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nocv/graph"
	"nocv/internal/testutil"
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
		{name: "missing node path endpoint", args: []string{"node", "dependency-paths", "./...", "from"}, want: "usage: nocv node dependency-paths <pattern> <from-node-ref> <to-node-ref>"},
		{name: "missing import cycle endpoint", args: []string{"go", "package", "check-import-cycle", "./...", "from"}, want: "usage: nocv go package check-import-cycle <pattern> <from-package-ref> <to-package-ref>"},
		{name: "missing forbidden import endpoint", args: []string{"go", "package", "check-forbidden-import", "./...", "from"}, want: "usage: nocv go package check-forbidden-import <pattern> <from-package-ref> <to-package-ref>"},
		{name: "missing type dependency endpoint", args: []string{"go", "type", "dependencies", "./..."}, want: "usage: nocv go type dependencies <pattern> <type-ref>"},
		{name: "missing node inspection ref", args: []string{"node", "inspect", "./..."}, want: "usage: nocv node inspect <pattern> <node-ref>"},
		{name: "missing transitive dependent ref", args: []string{"node", "transitive-dependents", "./..."}, want: "usage: nocv node transitive-dependents <pattern> <node-ref>"},
		{name: "unknown node command", args: []string{"node", "unknown"}, want: `unknown node command "unknown"`},
		{name: "unknown Go scope", args: []string{"go", "unknown"}, want: `unknown Go scope "unknown"`},
		{name: "removed impact command", args: []string{"impact", "./...", "symbol"}, want: `unknown command "impact"`},
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

func TestParseScopedInvocations(t *testing.T) {
	tests := []struct {
		args       []string
		wantName   string
		wantValues int
	}{
		{args: []string{"serve", "./..."}, wantName: "serve"},
		{args: []string{"tree", "./..."}, wantName: "tree"},
		{args: []string{"node", "dependencies", "./...", "node"}, wantName: "node.dependencies", wantValues: 1},
		{args: []string{"node", "dependents", "./...", "node"}, wantName: "node.dependents", wantValues: 1},
		{args: []string{"node", "transitive-dependents", "./...", "node"}, wantName: "node.transitive-dependents", wantValues: 1},
		{args: []string{"node", "dependency-paths", "./...", "from", "to"}, wantName: "node.dependency-paths", wantValues: 2},
		{args: []string{"node", "inspect", "./...", "node"}, wantName: "node.inspect", wantValues: 1},
		{args: []string{"go", "package", "imports", "./...", "package"}, wantName: "go.package.imports", wantValues: 1},
		{args: []string{"go", "package", "importers", "./...", "package"}, wantName: "go.package.importers", wantValues: 1},
		{args: []string{"go", "package", "dependency-paths", "./...", "from", "to"}, wantName: "go.package.dependency-paths", wantValues: 2},
		{args: []string{"go", "package", "inspect-dependency", "./...", "from", "to"}, wantName: "go.package.inspect-dependency", wantValues: 2},
		{args: []string{"go", "package", "check-forbidden-import", "./...", "from", "to"}, wantName: "go.package.check-forbidden-import", wantValues: 2},
		{args: []string{"go", "package", "check-forbidden-dependency", "./...", "from", "to"}, wantName: "go.package.check-forbidden-dependency", wantValues: 2},
		{args: []string{"go", "package", "check-import-cycle", "./...", "from", "to"}, wantName: "go.package.check-import-cycle", wantValues: 2},
		{args: []string{"go", "type", "dependencies", "./...", "type"}, wantName: "go.type.dependencies", wantValues: 1},
		{args: []string{"go", "type", "dependency-paths", "./...", "from", "to"}, wantName: "go.type.dependency-paths", wantValues: 2},
	}
	for _, test := range tests {
		got, err := parseInvocation(test.args)
		if err != nil {
			t.Fatalf("parseInvocation(%v): %v", test.args, err)
		}
		if got.name != test.wantName || got.pattern != "./..." || len(got.values) != test.wantValues {
			t.Errorf("parseInvocation(%v) = %#v, want name %q, pattern ./..., %d values", test.args, got, test.wantName, test.wantValues)
		}
	}
}

func TestScopedHelpIsDiscoverableWithoutLoading(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{args: []string{"--help"}, want: []string{"serve <pattern>", "tree <pattern>", "node ...", "go ..."}},
		{args: []string{"node", "--help"}, want: []string{"represented graph entity", "dependencies and dependents are immediate", "FieldType", "transitive-dependents", "dependency-paths", "inspect"}},
		{args: []string{"go", "--help"}, want: []string{"Go-specific semantic operations", "package", "type"}},
		{args: []string{"go", "package", "--help"}, want: []string{"direct Go import relationships", "FieldType", "imports are not included", "inspect-dependency", "check-import-cycle"}},
		{args: []string{"go", "type", "--help"}, want: []string{"structs, interfaces", "Package functions do not participate", "dependency-paths"}},
	}
	for _, test := range tests {
		var stdout, stderr bytes.Buffer
		if status := run(test.args, &stdout, &stderr); status != 0 {
			t.Fatalf("run(%v) status = %d, stderr = %q", test.args, status, stderr.String())
		}
		if stderr.Len() != 0 {
			t.Errorf("run(%v) stderr = %q", test.args, stderr.String())
		}
		for _, want := range test.want {
			if !strings.Contains(stdout.String(), want) {
				t.Errorf("run(%v) help lacks %q:\n%s", test.args, want, stdout.String())
			}
		}
	}
}

func TestRemovedFlatAndDebugCommandsAreRejected(t *testing.T) {
	for _, name := range []string{
		"direct-deps", "direct-dependents", "transitive-dependents", "paths",
		"package-imports", "package-importers", "import-cycle", "package-paths",
		"type-deps", "type-paths", "inspect-node", "inspect-dependency",
		"imports", "package-deps", "why-package-dep", "calls", "implementations",
		"embeddings", "signatures",
	} {
		if _, err := parseInvocation([]string{name, "./..."}); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Errorf("removed command %q error = %v", name, err)
		}
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

func TestRunWarnsOnceForPartialAnalysisAndKeepsUsefulOutput(t *testing.T) {
	pattern := filepath.Join(testutil.PartialGoProject(t), "...")
	var stdout, stderr bytes.Buffer
	if status := run([]string{"node", "inspect", pattern, "example.com/shop/status/brokenone::Service"}, &stdout, &stderr); status != 0 {
		t.Fatalf("run partial node inspect status = %d, stderr = %q", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Node inspection:") || !strings.Contains(stdout.String(), "example.com/shop/status/brokenone::Service") {
		t.Fatalf("partial node output is not useful: %q", stdout.String())
	}
	if count := strings.Count(stderr.String(), "warning: partial analysis"); count != 1 {
		t.Fatalf("partial warning count = %d, stderr = %q", count, stderr.String())
	}
	for _, packagePath := range []string{"example.com/shop/status/brokenone", "example.com/shop/status/brokentwo"} {
		if !strings.Contains(stderr.String(), packagePath) {
			t.Errorf("partial warning lacks package %q: %q", packagePath, stderr.String())
		}
	}
}

func TestRunDoesNotWarnForCompleteAnalysis(t *testing.T) {
	pattern := filepath.Join(testutil.GoProjectDir(t), "...")
	var stdout, stderr bytes.Buffer
	if status := run([]string{"tree", pattern}, &stdout, &stderr); status != 0 {
		t.Fatalf("run complete tree status = %d, stderr = %q", status, stderr.String())
	}
	if strings.Contains(stderr.String(), "partial analysis") {
		t.Fatalf("complete analysis warning = %q", stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("complete tree output is empty")
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
		{name: "go.package.imports", values: []string{string(ids.packageA)}, want: []string{"example.com/a", "imports -> example.com/b"}},
		{name: "go.package.importers", values: []string{string(ids.packageB)}, want: []string{"example.com/b", "<- imports example.com/a"}},
		{name: "go.package.check-import-cycle", values: []string{string(ids.packageB), string(ids.packageA)}, want: []string{"Proposed import:", "example.com/b -> example.com/a", "Would create import cycle.", "existing path 1:", "-> example.com/b"}},
		{name: "go.package.check-import-cycle", values: []string{string(ids.packageA), string(ids.packageB)}, want: []string{"Proposed import:", "No import cycle would be created."}, unwanted: []string{"existing path"}},
		{name: "go.package.check-forbidden-import", values: []string{string(ids.packageA), string(ids.packageB)}, want: []string{"Forbidden package import:", "example.com/a -> example.com/b", "VIOLATION", "evidence:", "fixture.go:1:1"}},
		{name: "go.package.check-forbidden-import", values: []string{string(ids.packageB), string(ids.packageA)}, want: []string{"Forbidden package import:", "No violation."}, unwanted: []string{"VIOLATION", "evidence:"}},
		{name: "go.package.check-forbidden-dependency", values: []string{string(ids.packageA), string(ids.packageB)}, want: []string{"Forbidden package dependency:", "example.com/a -> example.com/b", "VIOLATION", "path 1:", "evidence:", "calls ->"}},
		{name: "go.package.check-forbidden-dependency", values: []string{string(ids.packageB), string(ids.packageA)}, want: []string{"Forbidden package dependency:", "No violation."}, unwanted: []string{"VIOLATION", "path 1:"}},
		{name: "go.type.dependencies", values: []string{string(ids.service)}, want: []string{"Type dependencies:", string(ids.service), "-> " + string(ids.repository), "evidence:", "calls ->"}},
		{name: "go.type.dependency-paths", values: []string{string(ids.service), string(ids.repository)}, want: []string{"Type dependency paths:", "path 1:", string(ids.service), "-> " + string(ids.repository), "evidence:"}},
		{name: "go.package.inspect-dependency", values: []string{string(ids.packageA), string(ids.packageB)}, want: []string{"Dependency inspection:", "PACKAGE PATH 1", "HOP 1", "TYPE", string(ids.service) + " -> " + string(ids.repository), "EXACT ONLY"}},
		{name: "node.inspect", values: []string{string(ids.service)}, want: []string{"Node inspection:", string(ids.service), "Kind:", "struct", "Parent:", string(ids.packageA), "Documentation:", "(none)", "Methods:", string(ids.caller), "Type dependencies:", string(ids.repository), "Direct semantic dependencies:", "implements ->", "embeds ->"}},
		{name: "node.inspect", values: []string{string(ids.caller)}, want: []string{"Node inspection:", string(ids.caller), "Calls:", "calls -> " + string(ids.callee), "Called by:", "Accepts:", "accepts -> " + string(ids.repository), "Returns:", "returns -> " + string(ids.base), "Implements:", "implements -> " + string(ids.callee), "Implemented by:"}, unwanted: []string{"\nDependencies:", "\nDependents:"}},
		{name: "node.dependencies", values: []string{string(ids.caller)}, want: []string{string(ids.caller), "calls ->", "accepts ->"}},
		{name: "node.dependents", values: []string{string(ids.repository)}, want: []string{string(ids.repository), "<- implements", "<- accepts"}},
		{name: "node.transitive-dependents", values: []string{string(ids.callee)}, want: []string{"Transitive dependents of", string(ids.caller), "path 1:"}},
		{name: "node.dependency-paths", values: []string{string(ids.caller), string(ids.callee)}, want: []string{"Dependency paths:", "path 1:", "calls ->"}},
		{name: "go.package.dependency-paths", values: []string{string(ids.packageA), string(ids.packageB)}, want: []string{"Package dependency paths:", "evidence:", "example.com/a", "example.com/b"}},
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

func TestCLIExplicitlyMarksUncertainRelationshipsAndPaths(t *testing.T) {
	confirmedGraph, confirmedIDs := cliFixture(t)
	var confirmed bytes.Buffer
	if err := executeCommand(&confirmed, confirmedGraph, invocation{
		name: "node.dependencies", pattern: "./...", values: []string{string(confirmedIDs.caller)},
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(confirmed.String(), "[confirmed]") || strings.Contains(confirmed.String(), "[uncertain]") ||
		!strings.Contains(confirmed.String(), "calls -> "+string(confirmedIDs.callee)) {
		t.Fatalf("confirmed relationship output changed:\n%s", confirmed.String())
	}

	g := graph.New()
	source := graph.SymbolRef("example.com/source")
	target := graph.SymbolRef("example.com/target")
	concrete := graph.SymbolRef("example.com/source::Concrete")
	contract := graph.SymbolRef("example.com/target::Contract")
	for _, node := range []fixtureNode{
		{ID: source, Kind: graph.NodePackage, Name: "source"},
		{ID: target, Kind: graph.NodePackage, Name: "target"},
		{ID: concrete, Kind: graph.NodeStruct, Name: "Concrete", Parent: source},
		{ID: contract, Kind: graph.NodeInterface, Name: "Contract", Parent: target},
	} {
		if err := addFixtureNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	if err := addFixtureEdge(g, fixtureEdge{
		From: concrete, To: contract, Kind: graph.EdgeImplements,
		Certainty: graph.RelationshipUncertain,
		Evidence:  []graph.Location{{File: "uncertain.go", Line: 3, Column: 1}},
	}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{name: "node.dependencies", values: []string{string(concrete)}, want: "implements [uncertain] -> " + string(contract)},
		{name: "node.dependents", values: []string{string(contract)}, want: "<- implements [uncertain] " + string(concrete)},
		{name: "node.dependency-paths", values: []string{string(concrete), string(contract)}, want: "implements [uncertain] ->"},
		{name: "go.package.dependency-paths", values: []string{string(source), string(target)}, want: "implements [uncertain] ->"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := executeCommand(&output, g, invocation{name: test.name, pattern: "./...", values: test.values}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), test.want) {
				t.Fatalf("output lacks %q:\n%s", test.want, output.String())
			}
		})
	}

	var forbidden bytes.Buffer
	err := executeCommand(&forbidden, g, invocation{
		name: "go.package.check-forbidden-dependency", pattern: "./...",
		values: []string{string(source), string(target)},
	})
	if !errors.Is(err, errInconclusiveForbiddenDependency) ||
		!strings.Contains(forbidden.String(), "POTENTIAL VIOLATION (inconclusive)") ||
		strings.Contains(forbidden.String(), "\nVIOLATION\n") {
		t.Fatalf("uncertain forbidden dependency = error %v, output:\n%s", err, forbidden.String())
	}
}

func TestRunReturnsNonSuccessForUncertainForbiddenDependency(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":           "module example.com/cliuncertain\n\ngo 1.22\n",
		"source/source.go": "package source\n\ntype MissingAlias = MissingDependency\ntype Candidate struct { MissingAlias }\n",
		"target/target.go": "package target\n\ntype Contract interface { Required() }\n",
	}
	for name, contents := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	status := run([]string{
		"go", "package", "check-forbidden-dependency", filepath.Join(dir, "..."),
		"example.com/cliuncertain/source", "example.com/cliuncertain/target",
	}, &stdout, &stderr)
	if status != 1 || !strings.Contains(stdout.String(), "POTENTIAL VIOLATION (inconclusive)") ||
		!strings.Contains(stdout.String(), "implements [uncertain]") {
		t.Fatalf("uncertain run status = %d, stdout = %q, stderr = %q", status, stdout.String(), stderr.String())
	}
}

func TestRunPreservesSuccessForConfirmedForbiddenDependency(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":           "module example.com/cliconfirmed\n\ngo 1.22\n",
		"source/source.go": "package source\n\nimport \"example.com/cliconfirmed/target\"\n\nfunc Run() { target.Save() }\n",
		"target/target.go": "package target\n\nfunc Save() {}\n",
	}
	for name, contents := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	status := run([]string{
		"go", "package", "check-forbidden-dependency", filepath.Join(dir, "..."),
		"example.com/cliconfirmed/source", "example.com/cliconfirmed/target",
	}, &stdout, &stderr)
	if status != 0 || !strings.Contains(stdout.String(), "\nVIOLATION\n") || strings.Contains(stdout.String(), "[uncertain]") {
		t.Fatalf("confirmed run status = %d, stdout = %q, stderr = %q", status, stdout.String(), stderr.String())
	}
}

func TestNodeDependentsDistinguishImmediateFromTransitive(t *testing.T) {
	g := graph.New()
	pkg := graph.SymbolRef("example.com/depth")
	a := graph.SymbolRef("example.com/depth::A")
	b := graph.SymbolRef("example.com/depth::B")
	c := graph.SymbolRef("example.com/depth::C")
	for _, node := range []fixtureNode{
		{ID: pkg, Kind: graph.NodePackage, Name: "depth"},
		{ID: a, Kind: graph.NodeFunction, Name: "A", Parent: pkg},
		{ID: b, Kind: graph.NodeFunction, Name: "B", Parent: pkg},
		{ID: c, Kind: graph.NodeFunction, Name: "C", Parent: pkg},
	} {
		if err := addFixtureNode(g, node); err != nil {
			t.Fatal(err)
		}
	}
	for _, edge := range []fixtureEdge{
		{From: b, To: a, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "depth.go", Offset: 1}}},
		{From: c, To: b, Kind: graph.EdgeCalls, Evidence: []graph.Location{{File: "depth.go", Offset: 2}}},
	} {
		if err := addFixtureEdge(g, edge); err != nil {
			t.Fatal(err)
		}
	}

	var direct bytes.Buffer
	if err := executeCommand(&direct, g, invocation{name: "node.dependents", values: []string{string(a)}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(direct.String(), string(b)) || strings.Contains(direct.String(), string(c)) {
		t.Fatalf("node dependents output = %q, want only B", direct.String())
	}

	var transitive bytes.Buffer
	if err := executeCommand(&transitive, g, invocation{name: "node.transitive-dependents", values: []string{string(a)}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(transitive.String(), string(b)) || !strings.Contains(transitive.String(), string(c)) {
		t.Fatalf("node transitive-dependents output = %q, want B and C", transitive.String())
	}
}

func TestExecuteCommandReportsMissingAndInvalidSymbols(t *testing.T) {
	g, ids := cliFixture(t)
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{name: "node.dependencies", values: []string{"missing"}, want: "unknown symbol: missing"},
		{name: "node.dependency-paths", values: []string{string(ids.caller), "missing"}, want: "unknown symbol: missing"},
		{name: "go.package.dependency-paths", values: []string{"missing", string(ids.packageB)}, want: "unknown package: missing"},
		{name: "go.package.dependency-paths", values: []string{string(ids.caller), string(ids.packageB)}, want: "symbol is not a package: " + string(ids.caller)},
		{name: "go.package.imports", values: []string{"missing"}, want: "unknown package: missing"},
		{name: "go.package.importers", values: []string{string(ids.repository)}, want: "symbol is not a package: " + string(ids.repository)},
		{name: "go.package.check-import-cycle", values: []string{"missing", string(ids.packageB)}, want: "unknown package: missing"},
		{name: "go.package.check-import-cycle", values: []string{string(ids.packageA), string(ids.repository)}, want: "symbol is not a package: " + string(ids.repository)},
		{name: "go.package.check-forbidden-import", values: []string{"missing", string(ids.packageB)}, want: "unknown package: missing"},
		{name: "go.package.check-forbidden-import", values: []string{string(ids.packageA), string(ids.repository)}, want: "symbol is not a package: " + string(ids.repository)},
		{name: "go.package.check-forbidden-dependency", values: []string{"missing", string(ids.packageB)}, want: "unknown package: missing"},
		{name: "go.package.check-forbidden-dependency", values: []string{string(ids.packageA), string(ids.repository)}, want: "symbol is not a package: " + string(ids.repository)},
		{name: "go.type.dependencies", values: []string{"missing"}, want: "unknown symbol: missing"},
		{name: "go.type.dependencies", values: []string{string(ids.packageA)}, want: "symbol is not a type: " + string(ids.packageA)},
		{name: "go.type.dependency-paths", values: []string{string(ids.service), string(ids.caller)}, want: "symbol is not a type: " + string(ids.caller)},
		{name: "go.package.inspect-dependency", values: []string{"missing", string(ids.packageB)}, want: "unknown package: missing"},
		{name: "go.package.inspect-dependency", values: []string{string(ids.packageA), string(ids.repository)}, want: "symbol is not a package: " + string(ids.repository)},
		{name: "node.inspect", values: []string{"missing"}, want: "unknown symbol: missing"},
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
		{From: ids.packageA, To: ids.packageB, Kind: graph.EdgeImports, Evidence: []graph.Location{{File: "fixture.go", Line: 1, Column: 1, Offset: 1}}},
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
