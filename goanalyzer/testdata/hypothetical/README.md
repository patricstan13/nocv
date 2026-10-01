# Hypothetical overlay recheck spike

This test corpus evaluates a compiler-backed mechanism for hypothetical Go
source changes. It is deliberately not a product API or a result-impact model.

The tests load a baseline with `go/packages`, replace known source files through
`packages.Config.Overlay`, reload explicitly selected packages, and compare the
baseline and overlay diagnostics. The load mode is:

```text
NeedName
NeedFiles
NeedCompiledGoFiles
NeedImports
NeedDeps
NeedSyntax
NeedTypes
NeedTypesInfo
```

The `results` module isolates result-shape and consumption contexts. Mutated
source variants use `.go.txt` so they cannot become packages themselves. The
`closure` module isolates reverse-import traversal:

```text
repo <- service <- api
  ^
  +--- worker

unrelated
```

The comparison identity used by the main experiment is package path, exact
position, and exact diagnostic message, with duplicate counts preserved. This
works when a mutation leaves an existing error's location stable. The source
position experiment demonstrates that inserting a line makes the moved error
look new under exact identity. Package plus message filters that small case,
but is also not sufficient as a final strategy: diagnostics can repeat, wording
can change, and one error may suppress or replace another.

Overlay rechecks inherit the active Go environment, including GOOS, GOARCH,
build tags, module settings, and cgo availability. A clean diagnostic delta
therefore means only that no new compiler/type-check errors were observed for
that loaded configuration and requested scope. It does not establish behavioral
correctness, test success, runtime safety, or correctness under other build
configurations.
