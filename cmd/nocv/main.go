package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

	g, err := load(invocation.pattern)
	if err != nil {
		fmt.Fprintln(stderr, "nocv:", err)
		return 1
	}
	if err := executeCommand(stdout, g, invocation); err != nil {
		fmt.Fprintln(stderr, "nocv:", err)
		return 1
	}
	return 0
}

func load(pattern string) (*graph.Graph, error) {
	dir, loadPattern := localModulePattern(pattern)
	return goanalyzer.Load(context.Background(), dir, loadPattern)
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
