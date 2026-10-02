package goanalyzer

import (
	"context"
	"fmt"
	"sort"

	"golang.org/x/tools/go/packages"

	"nocv/graph"
)

func (a *Analysis) compilerImpact(callable resolvedCallable, proposed resolvedSignature) (CompilerImpact, error) {
	affectedPaths := affectedPackagePaths(a.packages, callable.pkg.PkgPath)
	impact := CompilerImpact{AffectedPackages: make([]graph.SymbolRef, 0, len(affectedPaths))}
	affectedPathSet := make(map[string]bool, len(affectedPaths))
	for _, path := range affectedPaths {
		impact.AffectedPackages = append(impact.AffectedPackages, graph.PackageRef(path))
		affectedPathSet[path] = true
	}

	baselineDiagnostics := collectCompilerDiagnostics(a.packages, affectedPathSet)
	if len(baselineDiagnostics) == 0 {
		impact.BaselineStatus = BaselineClean
	} else {
		impact.BaselineStatus = BaselineHasDiagnostics
	}

	overlayFilename, overlaySource, err := buildSignatureOverlay(callable, proposed)
	if err != nil {
		return CompilerImpact{}, err
	}
	recheckedPackages, err := a.recheckPackagesWithOverlay(affectedPaths, overlayFilename, overlaySource)
	if err != nil {
		return CompilerImpact{}, err
	}
	overlayDiagnostics := collectCompilerDiagnostics(recheckedPackages, affectedPathSet)
	changedDiagnostics := compareCompilerDiagnostics(baselineDiagnostics, overlayDiagnostics)
	impact.Consequences = a.compilerConsequences(changedDiagnostics)
	return impact, nil
}

func (a *Analysis) recheckPackagesWithOverlay(packagePaths []string, filename string, source []byte) ([]*packages.Package, error) {
	loaded, err := packages.Load(&packages.Config{
		Context: context.Background(),
		Dir:     a.loadDir,
		Mode:    a.loadMode,
		Tests:   false,
		Overlay: map[string][]byte{filename: source},
	}, packagePaths...)
	if err != nil {
		return nil, fmt.Errorf("recheck hypothetical signature: %w", err)
	}
	return loaded, nil
}

func affectedPackagePaths(pkgs []*packages.Package, changed string) []string {
	known := make(map[string]*packages.Package, len(pkgs))
	for _, pkg := range pkgs {
		known[pkg.PkgPath] = pkg
	}
	reverse := make(map[string][]string)
	for _, pkg := range pkgs {
		for imported := range pkg.Imports {
			if known[imported] != nil {
				reverse[imported] = append(reverse[imported], pkg.PkgPath)
			}
		}
	}
	seen := map[string]bool{changed: true}
	queue := []string{changed}
	for len(queue) != 0 {
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
