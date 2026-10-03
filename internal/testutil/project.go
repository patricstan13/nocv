// Package testutil provides concrete helpers for NOCV's shared Go test project.
package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// GoProjectDir returns the canonical checked-in Go test project directory.
func GoProjectDir(t testing.TB) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate testutil source file")
	}
	return filepath.Join(filepath.Dir(sourceFile), "..", "..", "testdata", "go", "project")
}

// CopyGoProject copies the canonical Go test project into a mutable temporary
// directory and returns the copied project root.
func CopyGoProject(t testing.TB) string {
	t.Helper()
	destination := filepath.Join(t.TempDir(), "project")
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(destination, os.DirFS(GoProjectDir(t))); err != nil {
		t.Fatal(err)
	}
	return destination
}

// PartialGoProject returns a mutable project copy with two deliberately
// type-broken packages for analysis-status integration tests.
func PartialGoProject(t testing.TB) string {
	t.Helper()
	projectDir := CopyGoProject(t)
	writeProjectFile(t, projectDir, "status/brokenone/broken.go", `package brokenone

type ID string

type Service struct{}

func (Service) Save(ID) {}

var first int = "one"
var second string = 2
`)
	writeProjectFile(t, projectDir, "status/brokentwo/broken.go", `package brokentwo

func Run() {}

var existing bool = "not bool"
`)
	return projectDir
}

func writeProjectFile(t testing.TB, projectDir, name, contents string) {
	t.Helper()
	path := filepath.Join(projectDir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
