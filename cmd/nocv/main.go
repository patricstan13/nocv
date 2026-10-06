package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nocv/goanalyzer"
	"nocv/graph"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	invocation, err := parseInvocation(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if invocation.help != "" {
		fmt.Fprintln(stdout, invocation.help)
		return 0
	}

	var analysisStart time.Time
	if invocation.name == "serve" {
		fmt.Fprintln(stdout, "Analyzing project...")
		analysisStart = time.Now()
	}
	analysis, err := loadAnalysis(invocation.pattern)
	if invocation.name == "serve" {
		fmt.Fprintf(stdout, "Analysis total: %s\n", time.Since(analysisStart).Round(time.Millisecond))
	}
	if err != nil {
		fmt.Fprintln(stderr, "nocv:", err)
		return 1
	}
	writeAnalysisStatusWarning(stderr, analysis)
	if err := executeCommandWithAnalysis(stdout, analysis.Graph(), analysis, invocation); err != nil {
		if errors.Is(err, errInconclusiveForbiddenDependency) {
			return 1
		}
		fmt.Fprintln(stderr, "nocv:", err)
		return 1
	}
	return 0
}

func writeAnalysisStatusWarning(stderr io.Writer, analysis *goanalyzer.Analysis) {
	if analysis.Status() != goanalyzer.AnalysisPartial {
		return
	}
	packages := make([]string, 0, len(analysis.StatusReasons()))
	for _, reason := range analysis.StatusReasons() {
		packages = append(packages, string(reason.Package))
	}
	fmt.Fprintf(stderr, "warning: partial analysis; incomplete type information in packages: %s\n", strings.Join(packages, ", "))
}

func load(pattern string) (*graph.Graph, error) {
	analysis, err := loadAnalysis(pattern)
	if err != nil {
		return nil, err
	}
	return analysis.Graph(), nil
}

func loadAnalysis(pattern string) (*goanalyzer.Analysis, error) {
	dir, loadPattern := localModulePattern(pattern)
	return goanalyzer.LoadAnalysis(context.Background(), dir, loadPattern)
}

func localModulePattern(pattern string) (string, string) {
	dir := pattern
	loadPattern := "."
	if strings.HasSuffix(pattern, "/...") {
		dir = strings.TrimSuffix(pattern, "/...")
		loadPattern = "./..."
	}
	if dir == "" {
		dir = "."
	}
	if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
		return dir, loadPattern
	}
	return "", pattern
}
