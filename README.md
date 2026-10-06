# NOCV

NOCV is an experimental explorer for understanding the structure of a Go
codebase.

Large projects are difficult to reason about from imports or text search alone.
An import tells you that two packages are connected, but not which declarations
create the dependency, whether an interface contract is involved, or how a
method reached a type through embedding. NOCV loads the project with Go's own
compiler tooling and answers questions such as:

- What does this function, type, or package depend on?
- What depends on it, directly or transitively?
- Which exact calls, signatures, implementations, or embeddings establish a
  package dependency?
- Would a proposed import create a cycle?
- What compile-time, interface-contract, or promoted-method consequences could
  follow from changing a callable's parameters or results?

NOCV keeps those answers tied to stable symbol references and source evidence.
It is an analysis tool: it does not rewrite source code or claim that a change
is behaviorally correct.

## Development status

NOCV is early beta software and is under active development. It is already
useful for exploring Go projects, but:

- the CLI, APIs, and web interface may change;
- only Go is supported;
- analysis covers the active build configuration, not every possible platform
  or build-tag combination;
- the web interface is a focused local explorer, not a full IDE.

Please evaluate findings against the source before using them for architectural
or refactoring decisions.

## Requirements

- Go 1.27 or newer
- a Go module or workspace that the active Go toolchain can load

NOCV currently analyzes non-test packages (`Tests: false`) selected by the
provided Go package pattern.

## Quick start

Start the local graphical explorer from the root of a Go project:

```sh
go run ./cmd/nocv serve ./...
```

NOCV prints a loopback URL such as `http://127.0.0.1:43210`. Open it in a
browser, search for a package, and select packages, dependencies, types, and
callables to inspect the evidence behind the graph.

For a bounded CLI introduction using NOCV's own source:

```sh
# Inspect one symbol and its immediate relationships.
go run ./cmd/nocv node inspect ./... nocv/query::DirectDependencies

# Explain exact symbol-to-symbol dependency paths.
go run ./cmd/nocv node dependency-paths ./... \
  nocv/query::TransitiveDependents nocv/graph::Graph

# Show the Go packages imported directly by query.
go run ./cmd/nocv go package imports ./... nocv/query

# Explain why the command package semantically depends on query.
go run ./cmd/nocv go package inspect-dependency ./... \
  nocv/cmd/nocv nocv/query
```

The canonical test project provides stable examples independent of NOCV's own
implementation:

```sh
go run ./cmd/nocv node inspect ./testdata/go/project/... \
  example.com/shop/typeview/service::Service

go run ./cmd/nocv go type dependency-paths ./testdata/go/project/... \
  example.com/shop/typeview/service::Service \
  example.com/shop/typeview/store::Store
```

Run `go run ./cmd/nocv --help` for top-level help. Add `node --help`,
`go package --help`, or `go type --help` after `./cmd/nocv` to discover each
scoped command set.

## What NOCV models

NOCV represents packages, structs, interfaces, functions, concrete methods,
and interface methods. Each node has:

- an opaque graph-local ID used internally; and
- a deterministic textual reference used by the CLI and web API, such as
  `nocv/query::DirectDependencies`.

It records seven relationship kinds:

| Relationship | Meaning |
|---|---|
| `Calls` | one represented callable invokes another |
| `Implements` | a concrete type or method satisfies an interface contract |
| `Embeds` | a represented struct or interface embeds another represented type |
| `Accepts` | a callable directly accepts a represented type |
| `Returns` | a callable directly returns a represented type |
| `FieldType` | a named, non-embedded struct field contains a represented type |
| `Imports` | one represented Go package directly imports another |

The first six relationships form NOCV's semantic dependency view. `Imports`
is deliberately separate: source imports and semantic reliance answer different
questions.

Package and type dependencies are projections of exact semantic facts. NOCV
retains those facts as evidence, so a package edge can be inspected down to the
calls, contracts, embeddings, and signature relationships that establish it.

Relationships carry an explicit certainty. **Confirmed** means the available
supported semantic information establishes the relationship. **Uncertain**
means incomplete compiler information permits the relationship but does not
establish it conclusively. Exploratory queries retain both states and mark
uncertain relationships explicitly; deterministic conclusions use only
confirmed relationships and preserve relevant uncertainty as inconclusive
context. Currently, only type-level `Implements` relationships have a proven
uncertainty-producing condition.

## CLI structure

```text
nocv
├── serve
├── tree
├── node
│   ├── dependencies
│   ├── dependents
│   ├── transitive-dependents
│   ├── dependency-paths
│   └── inspect
└── go
    ├── package
    │   ├── imports
    │   ├── importers
    │   ├── dependency-paths
    │   ├── inspect-dependency
    │   ├── check-forbidden-import
    │   ├── check-forbidden-dependency
    │   └── check-import-cycle
    └── type
        ├── dependencies
        └── dependency-paths
```

`node` operates on exact graph entities and the six semantic relationships.
Unqualified dependencies and dependents are immediate; transitive traversal is
named explicitly. `go package` and `go type` contain operations whose meaning
depends on Go imports, package boundaries, and struct/interface method
ownership.

## Hypothetical signature changes

Function and method inspectors in the web interface can propose a new ordered
parameter/result list and variadic state. NOCV reports four separate lenses:

- **Call-site impact:** compatibility of known direct calls when parameters
  change.
- **Compiler consequences:** new or uncertain diagnostics after rechecking the
  changed package and its reverse-import closure through an in-memory overlay.
- **Contract impact:** existing interface implementations that the proposed
  signature would lose.
- **Structural impact:** concrete types whose promoted method surface would
  change through embedding.

The source tree is never modified. “No new compiler diagnostics” only means no
new compile/type-check diagnostic was observed in the rechecked scope. It does
not prove behavioral correctness, run tests, or guarantee that another build
configuration is unaffected.

## Analysis status

A load has one of three outcomes:

- **Complete:** NOCV found no known condition that prevented its supported
  analysis for the loaded packages and active build configuration.
- **Partial:** the graph is useful, but incomplete type information may have
  caused supported facts to be omitted or particular relationships to remain
  uncertain. The CLI warns once and the web interface displays a persistent
  status indicator.
- **Failed:** no usable analysis is returned, for example after a syntax/load
  failure or when no packages match.

Partial does not make every relationship uncertain. Confirmed relationships
remain established by the available supported semantic information. Uncertain
relationships are retained and labeled as architectural context. Absence under
Partial analysis remains inconclusive.

## Current limitations

- Go is the only supported language.
- Only the active toolchain environment and build configuration are analyzed.
- Test variants are not loaded.
- Fields, aliases as separate declarations, and methods on named non-struct
  receiver types are not represented as structural nodes.
- Dynamic function-value targets and unresolved calls may be absent.
- External packages appear only when they are included in the analyzed package
  set.
- Signature-change analysis does not support receiver changes or generic/type-
  parameter changes.
- Contract analysis reports lost existing implementations, not newly gained
  contracts.
- Compiler-diagnostic attribution is conservative on dirty baselines; some
  diagnostics cannot be mapped to a represented symbol.
- Dependency-path queries return all distinct simple paths and can produce
  substantial output on densely connected projects.
- The web explorer has no reload, persistence, global type/function graph, or
  general navigation history.
- NOCV performs analysis only. It does not apply refactorings or rewrite code.

## Development

The repository keeps one representative Go fixture in
`testdata/go/project`. Narrow malformed or partial scenarios are generated by
tests instead of being checked in as broken Go projects.

Format, test, and vet the project with the standard Go toolchain:

```sh
gofmt -w .
go test -count=1 ./...
go vet ./...
```

## Credits and development disclosure

NOCV is built on the Go standard library and
[`golang.org/x/tools/go/packages`](https://pkg.go.dev/golang.org/x/tools/go/packages)
for compiler-backed project loading and type information. The graphical
explorer embeds [vis-network](https://visjs.github.io/vis-network/); its license
notices are recorded in
[`internal/webui/THIRD_PARTY.md`](internal/webui/THIRD_PARTY.md). Formatting,
testing, and static analysis use the standard Go toolchain.

Substantial portions of NOCV's design, implementation, tests, and documentation
have been developed with AI assistance using OpenAI Codex, under human
direction and review. AI-assisted changes should be evaluated with the same
care as any other contribution.
