package goanalyzer_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/tools/go/packages"
)

const hypotheticalLoadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedDeps |
	packages.NeedSyntax |
	packages.NeedTypes |
	packages.NeedTypesInfo

type spikeDiagnostic struct {
	Package string
	Pos     string
	Message string
}

func (d spikeDiagnostic) String() string {
	return fmt.Sprintf("%s: %s: %s", d.Package, d.Pos, d.Message)
}

type packageLoad struct {
	Packages    []*packages.Package
	Diagnostics []spikeDiagnostic
	Duration    time.Duration
}

func TestOverlayResultChangeAllowsValidInferredPointer(t *testing.T) {
	dir := resultFixtureDir(t)
	patterns := []string{"example.com/hypothetical/repo", "example.com/hypothetical/valid"}
	baseline := loadForSpike(t, dir, patterns, nil)
	requireNoDiagnostics(t, "baseline", baseline.Diagnostics)

	overlay := resultOverlay(t, dir, "find_pointer.go.txt")
	changed := loadForSpike(t, dir, patterns, overlay)
	if delta := diagnosticDelta(baseline.Diagnostics, changed.Diagnostics); len(delta) != 0 {
		t.Fatalf("valid inferred pointer contexts produced new diagnostics:\n%s", formatDiagnostics(delta))
	}
}

func TestOverlayResultChangeFindsDownstreamInferredTypeFailures(t *testing.T) {
	dir := resultFixtureDir(t)
	patterns := []string{"example.com/hypothetical/repo", "example.com/hypothetical/failure"}
	baseline := loadForSpike(t, dir, patterns, nil)
	requireNoDiagnostics(t, "baseline", baseline.Diagnostics)

	changed := loadForSpike(t, dir, patterns, resultOverlay(t, dir, "find_pointer.go.txt"))
	delta := diagnosticDelta(baseline.Diagnostics, changed.Diagnostics)
	if len(delta) < 4 {
		t.Fatalf("new diagnostics = %d, want downstream inference, assignment, forwarding, and tuple failures:\n%s", len(delta), formatDiagnostics(delta))
	}
	assertDiagnosticPackages(t, delta, "example.com/hypothetical/failure")
	assertDiagnosticContains(t, delta, "cannot use")
}

func TestOverlayResultChangeAllowsCompatibleForwarding(t *testing.T) {
	dir := resultFixtureDir(t)
	patterns := []string{"example.com/hypothetical/repo", "example.com/hypothetical/compatibleforward"}
	baseline := loadForSpike(t, dir, patterns, nil)
	requireNoDiagnostics(t, "baseline", baseline.Diagnostics)

	changed := loadForSpike(t, dir, patterns, resultOverlay(t, dir, "compatible_pointer.go.txt"))
	if delta := diagnosticDelta(baseline.Diagnostics, changed.Diagnostics); len(delta) != 0 {
		t.Fatalf("compatible forwarded result produced new diagnostics:\n%s", formatDiagnostics(delta))
	}
}

func TestOverlayResultChangeFindsResultShapeFailures(t *testing.T) {
	dir := resultFixtureDir(t)
	tests := []struct {
		name     string
		consumer string
		mutation string
		want     int
	}{
		{name: "added result with discarded-call control", consumer: "count", mutation: "added_result.go.txt", want: 1},
		{name: "removed result", consumer: "count", mutation: "removed_result.go.txt", want: 1},
		{name: "changed second result", consumer: "second", mutation: "second_bool.go.txt", want: 1},
		{name: "reordered results", consumer: "reorder", mutation: "reordered.go.txt", want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			patterns := []string{"example.com/hypothetical/repo", "example.com/hypothetical/" + test.consumer}
			baseline := loadForSpike(t, dir, patterns, nil)
			requireNoDiagnostics(t, "baseline", baseline.Diagnostics)
			changed := loadForSpike(t, dir, patterns, resultOverlay(t, dir, test.mutation))
			delta := diagnosticDelta(baseline.Diagnostics, changed.Diagnostics)
			if len(delta) != test.want {
				t.Fatalf("overlay produced %d new diagnostics, want %d:\n%s", len(delta), test.want, formatDiagnostics(delta))
			}
			assertDiagnosticPackages(t, delta, "example.com/hypothetical/"+test.consumer)
		})
	}
}

func TestOverlayResultChangeUsesCompilerForArgumentsAndInterfaces(t *testing.T) {
	dir := resultFixtureDir(t)

	argumentPatterns := []string{"example.com/hypothetical/repo", "example.com/hypothetical/argument"}
	argumentBaseline := loadForSpike(t, dir, argumentPatterns, nil)
	requireNoDiagnostics(t, "argument baseline", argumentBaseline.Diagnostics)
	argumentChanged := loadForSpike(t, dir, argumentPatterns, resultOverlay(t, dir, "single_pointer.go.txt"))
	argumentDelta := diagnosticDelta(argumentBaseline.Diagnostics, argumentChanged.Diagnostics)
	if len(argumentDelta) != 1 {
		t.Fatalf("argument delta = %d, want only the concrete User consumer to fail:\n%s", len(argumentDelta), formatDiagnostics(argumentDelta))
	}

	interfacePatterns := []string{"example.com/hypothetical/repo", "example.com/hypothetical/interfaces"}
	interfaceBaseline := loadForSpike(t, dir, interfacePatterns, nil)
	requireNoDiagnostics(t, "interface baseline", interfaceBaseline.Diagnostics)
	stillAssignable := loadForSpike(t, dir, interfacePatterns, resultOverlay(t, dir, "single_pointer.go.txt"))
	if delta := diagnosticDelta(interfaceBaseline.Diagnostics, stillAssignable.Diagnostics); len(delta) != 0 {
		t.Fatalf("User to *User unexpectedly lost the value-receiver interface:\n%s", formatDiagnostics(delta))
	}
	noLongerAssignable := loadForSpike(t, dir, interfacePatterns, resultOverlay(t, dir, "pointer_value.go.txt"))
	delta := diagnosticDelta(interfaceBaseline.Diagnostics, noLongerAssignable.Diagnostics)
	if len(delta) != 1 {
		t.Fatalf("pointer-only interface delta = %d, want 1:\n%s", len(delta), formatDiagnostics(delta))
	}
}

func TestOverlayResultChangeStopsAtFirstCascadingBoundary(t *testing.T) {
	dir := resultFixtureDir(t)
	patterns := []string{
		"example.com/hypothetical/repo",
		"example.com/hypothetical/cascade/service",
		"example.com/hypothetical/cascade/api",
	}
	baseline := loadForSpike(t, dir, patterns, nil)
	requireNoDiagnostics(t, "baseline", baseline.Diagnostics)
	changed := loadForSpike(t, dir, patterns, resultOverlay(t, dir, "find_pointer.go.txt"))
	delta := diagnosticDelta(baseline.Diagnostics, changed.Diagnostics)
	if len(delta) == 0 {
		t.Fatal("overlay produced no forwarding diagnostic")
	}
	assertDiagnosticPackages(t, delta, "example.com/hypothetical/cascade/service")
}

func TestDiagnosticDeltaIgnoresBaselineErrors(t *testing.T) {
	dir := resultFixtureDir(t)
	patterns := []string{"example.com/hypothetical/repo", "example.com/hypothetical/broken"}
	baseline := loadForSpike(t, dir, patterns, nil)
	if len(baseline.Diagnostics) != 1 {
		t.Fatalf("baseline diagnostics = %d, want the deliberate existing error:\n%s", len(baseline.Diagnostics), formatDiagnostics(baseline.Diagnostics))
	}

	changed := loadForSpike(t, dir, patterns, resultOverlay(t, dir, "find_pointer.go.txt"))
	delta := diagnosticDelta(baseline.Diagnostics, changed.Diagnostics)
	if len(delta) != 1 {
		t.Fatalf("diagnostic delta = %d, want only the downstream mutation error:\n%s", len(delta), formatDiagnostics(delta))
	}
	assertDiagnosticContains(t, delta, "cannot use u")
}

func TestDiagnosticIdentityMovesWithSourcePositions(t *testing.T) {
	dir := resultFixtureDir(t)
	patterns := []string{"example.com/hypothetical/broken"}
	baseline := loadForSpike(t, dir, patterns, nil)
	if len(baseline.Diagnostics) != 1 {
		t.Fatalf("baseline diagnostics = %d, want 1", len(baseline.Diagnostics))
	}

	mutation := readFixture(t, filepath.Join(dir, "mutations", "broken_shift.go.txt"))
	target := absolutePath(t, filepath.Join(dir, "broken", "broken.go"))
	changed := loadForSpike(t, dir, patterns, map[string][]byte{target: mutation})
	exact := diagnosticDelta(baseline.Diagnostics, changed.Diagnostics)
	messageOnly := diagnosticMessageDelta(baseline.Diagnostics, changed.Diagnostics)
	if len(exact) != 2 || len(messageOnly) != 1 {
		t.Fatalf("position shift experiment: exact delta %d, message delta %d; want 2 and 1\nexact:\n%s\nmessage:\n%s",
			len(exact), len(messageOnly), formatDiagnostics(exact), formatDiagnostics(messageOnly))
	}
	t.Log("exact package+position+message identity treats a moved baseline error as new; package+message filters this fixture but is not a final identity strategy")
}

func TestAffectedPackagesUsesReverseImportClosure(t *testing.T) {
	dir := absolutePath(t, filepath.Join("testdata", "hypothetical", "closure"))
	loaded := loadForSpike(t, dir, []string{"./..."}, nil)
	requireNoDiagnostics(t, "closure fixture", loaded.Diagnostics)

	got := affectedPackagePaths(loaded.Packages, "example.com/closure/repo")
	want := []string{
		"example.com/closure/api",
		"example.com/closure/repo",
		"example.com/closure/service",
		"example.com/closure/worker",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("affected packages = %v, want %v", got, want)
	}
	if slices.Contains(got, "example.com/closure/unrelated") {
		t.Fatal("unrelated package entered reverse import closure")
	}
	t.Logf("overlay reload patterns: %s", strings.Join(got, ", "))
}

func TestOverlayRecheckImpactdemoTimings(t *testing.T) {
	root := absolutePath(t, "..")
	dir := filepath.Join(root, "examples", "impactdemo")
	targetFile := filepath.Join(dir, "repository", "repository.go")
	runRealProjectTiming(
		t,
		"impactdemo",
		dir,
		"example.com/impactdemo/repository",
		targetFile,
		readFixture(t, filepath.Join("testdata", "hypothetical", "real", "impactdemo_repository_pointer.go.txt")),
		readFixture(t, filepath.Join("testdata", "hypothetical", "real", "impactdemo_repository_bool.go.txt")),
	)
}

func TestOverlayRecheckNOCVTimings(t *testing.T) {
	dir := absolutePath(t, "..")
	targetFile := filepath.Join(dir, "query", "imports.go")
	runRealProjectTiming(
		t,
		"NOCV",
		dir,
		"nocv/query",
		targetFile,
		readFixture(t, filepath.Join("testdata", "hypothetical", "real", "nocv_imports_any.go.txt")),
		readFixture(t, filepath.Join("testdata", "hypothetical", "real", "nocv_imports_pointer_slice.go.txt")),
	)
}

func runRealProjectTiming(
	t *testing.T,
	name string,
	dir string,
	targetPackage string,
	targetFile string,
	firstMutation []byte,
	secondMutation []byte,
) {
	t.Helper()
	original := readFixture(t, targetFile)
	baseline := loadForSpike(t, dir, []string{"./..."}, nil)
	requireNoDiagnostics(t, name+" baseline", baseline.Diagnostics)
	affected := affectedPackagePaths(baseline.Packages, targetPackage)
	if len(affected) == 0 {
		t.Fatalf("no affected packages for %s", targetPackage)
	}

	first := loadForSpike(t, dir, affected, map[string][]byte{targetFile: firstMutation})
	repeat := loadForSpike(t, dir, affected, map[string][]byte{targetFile: firstMutation})
	different := loadForSpike(t, dir, affected, map[string][]byte{targetFile: secondMutation})
	if len(diagnosticDelta(baseline.Diagnostics, first.Diagnostics)) == 0 {
		t.Fatal("first real-project overlay produced no new diagnostic")
	}
	if len(diagnosticDelta(baseline.Diagnostics, different.Diagnostics)) == 0 {
		t.Fatal("different real-project overlay produced no new diagnostic")
	}
	after := readFixture(t, targetFile)
	if !bytes.Equal(original, after) {
		t.Fatalf("overlay changed %s on disk", targetFile)
	}
	t.Logf(
		"%s: baseline=%s first-overlay=%s repeat-identical=%s different-same-package=%s affected=%s first-new-diagnostics=%d",
		name,
		baseline.Duration.Round(time.Millisecond),
		first.Duration.Round(time.Millisecond),
		repeat.Duration.Round(time.Millisecond),
		different.Duration.Round(time.Millisecond),
		strings.Join(affected, ","),
		len(diagnosticDelta(baseline.Diagnostics, first.Diagnostics)),
	)
}

func loadForSpike(t *testing.T, dir string, patterns []string, overlay map[string][]byte) packageLoad {
	t.Helper()
	start := time.Now()
	loaded, err := packages.Load(&packages.Config{
		Dir:     dir,
		Mode:    hypotheticalLoadMode,
		Overlay: overlay,
		Tests:   false,
	}, patterns...)
	duration := time.Since(start)
	if err != nil {
		t.Fatalf("packages.Load(%v): %v", patterns, err)
	}
	return packageLoad{Packages: loaded, Diagnostics: collectDiagnostics(loaded), Duration: duration}
}

func collectDiagnostics(roots []*packages.Package) []spikeDiagnostic {
	visited := make(map[string]bool)
	var diagnostics []spikeDiagnostic
	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if pkg == nil || visited[pkg.ID] {
			return
		}
		visited[pkg.ID] = true
		for _, pkgError := range pkg.Errors {
			diagnostics = append(diagnostics, spikeDiagnostic{Package: pkg.PkgPath, Pos: pkgError.Pos, Message: pkgError.Msg})
		}
		for _, imported := range pkg.Imports {
			visit(imported)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	sort.Slice(diagnostics, func(i, j int) bool {
		left, right := diagnostics[i], diagnostics[j]
		if left.Package != right.Package {
			return left.Package < right.Package
		}
		if left.Pos != right.Pos {
			return left.Pos < right.Pos
		}
		return left.Message < right.Message
	})
	return diagnostics
}

func diagnosticDelta(baseline, changed []spikeDiagnostic) []spikeDiagnostic {
	return subtractDiagnostics(baseline, changed, func(d spikeDiagnostic) string {
		return d.Package + "\x00" + d.Pos + "\x00" + d.Message
	})
}

func diagnosticMessageDelta(baseline, changed []spikeDiagnostic) []spikeDiagnostic {
	return subtractDiagnostics(baseline, changed, func(d spikeDiagnostic) string {
		return d.Package + "\x00" + d.Message
	})
}

func subtractDiagnostics(
	baseline []spikeDiagnostic,
	changed []spikeDiagnostic,
	identity func(spikeDiagnostic) string,
) []spikeDiagnostic {
	remaining := make(map[string]int)
	for _, diagnostic := range baseline {
		remaining[identity(diagnostic)]++
	}
	var delta []spikeDiagnostic
	for _, diagnostic := range changed {
		key := identity(diagnostic)
		if remaining[key] > 0 {
			remaining[key]--
			continue
		}
		delta = append(delta, diagnostic)
	}
	return delta
}

func affectedPackagePaths(pkgs []*packages.Package, target string) []string {
	known := make(map[string]*packages.Package, len(pkgs))
	for _, pkg := range pkgs {
		known[pkg.PkgPath] = pkg
	}
	reverse := make(map[string][]string)
	for _, pkg := range pkgs {
		for importedPath := range pkg.Imports {
			if known[importedPath] != nil {
				reverse[importedPath] = append(reverse[importedPath], pkg.PkgPath)
			}
		}
	}
	seen := map[string]bool{target: true}
	queue := []string{target}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, importer := range reverse[current] {
			if seen[importer] {
				continue
			}
			seen[importer] = true
			queue = append(queue, importer)
		}
	}
	result := make([]string, 0, len(seen))
	for path := range seen {
		if known[path] != nil {
			result = append(result, path)
		}
	}
	sort.Strings(result)
	return result
}

func resultFixtureDir(t *testing.T) string {
	t.Helper()
	return absolutePath(t, filepath.Join("testdata", "hypothetical", "results"))
}

func resultOverlay(t *testing.T, dir, mutation string) map[string][]byte {
	t.Helper()
	target := absolutePath(t, filepath.Join(dir, "repo", "find.go"))
	return map[string][]byte{target: readFixture(t, filepath.Join(dir, "mutations", mutation))}
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func absolutePath(t *testing.T, path string) string {
	t.Helper()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return absolute
}

func requireNoDiagnostics(t *testing.T, label string, diagnostics []spikeDiagnostic) {
	t.Helper()
	if len(diagnostics) != 0 {
		t.Fatalf("%s diagnostics:\n%s", label, formatDiagnostics(diagnostics))
	}
}

func assertDiagnosticPackages(t *testing.T, diagnostics []spikeDiagnostic, want string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Package != want {
			t.Fatalf("diagnostic package = %q, want %q:\n%s", diagnostic.Package, want, formatDiagnostics(diagnostics))
		}
	}
}

func assertDiagnosticContains(t *testing.T, diagnostics []spikeDiagnostic, want string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, want) {
			return
		}
	}
	t.Fatalf("diagnostics do not contain %q:\n%s", want, formatDiagnostics(diagnostics))
}

func formatDiagnostics(diagnostics []spikeDiagnostic) string {
	if len(diagnostics) == 0 {
		return "  (none)"
	}
	lines := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		lines = append(lines, "  "+diagnostic.String())
	}
	return strings.Join(lines, "\n")
}
