# NOCV architecture contract

This document records the durable architecture and terminology contract for
the current Go beta. It describes the system as it exists; task history and
discarded alternatives belong in Git history, not in the product docs.

## Product boundary

NOCV turns compiler-established facts into inspectable structural answers:

```text
load Go packages
  -> build semantic graph
  -> derive exact, package, and type views
  -> answer navigation and architecture questions
  -> optionally simulate a callable signature change
```

The graph and query layers describe current structure. Hypothetical impact
analysis is a separate operation over retained Go compiler state. Neither layer
rewrites source.

## Package responsibilities

### `graph`

`graph` owns language-independent semantic storage:

- `Node`, opaque graph-local `NodeID`, and deterministic textual `SymbolRef`;
- source `Location`, hierarchy, edges, evidence, indexes, and validation;
- defensive read operations and explicit graph construction operations.

It must not depend on Go AST/type objects, query projections, presentation
models, frontend state, or hypothetical-change analysis.

### `query`

`query` interprets an existing graph. It owns:

- exact dependency and dependent navigation;
- semantic, package, type, and import paths;
- package/type projections and exact evidence aggregation;
- node and dependency inspection read models;
- forbidden dependency/import and import-cycle checks.

Queries are deterministic, return detached read models, and do not mutate the
graph. `query.SymbolSummary` is the shared language-independent detached symbol
description used by query and analyzer results.

### `goanalyzer`

`goanalyzer` owns all Go-specific analysis and retained compiler state:

- `go/packages`, AST, `go/types`, build/load context, and symbol indexing;
- graph extraction for Go declarations and relationships;
- analysis complete/partial status;
- callable signatures and Go type-expression resolution;
- call compatibility, interface contracts, method sets, and promotion;
- source-overlay construction, compiler rechecks, and diagnostic comparison.

Compiler objects remain private to `goanalyzer.Analysis`. They do not enter
`graph`, `query`, CLI, or HTTP models.

### Composition and presentation

`cmd/nocv` is the CLI composition root. `internal/webui` owns the local HTTP and
browser presentation. Both may combine `Analysis`, graph queries, and detached
read models, but must not recreate compiler semantics.

The loaded graph is treated as immutable by CLI and web consumers after
analysis construction. The graph API itself remains explicitly constructible
for analyzers and tests; concurrent mutation is not a supported contract.

## Identity and hierarchy

`NodeID` and `SymbolRef` have different purposes:

- `NodeID` is an opaque storage/traversal identity, unique only within one
  graph. Zero is invalid.
- `SymbolRef` is deterministic and human-readable. It is the identity used by
  the CLI and web API.

Package refs are import paths. Child refs append declaration names with `::`.
Repeated source names such as `init` or `_` receive deterministic source-order
suffixes. Parent/child hierarchy is structural storage, not a semantic
dependency edge.

Packages represent a whole package and therefore have no declaration location.
Represented types and callables retain declaration locations and source-authored
documentation.

## Relationship semantics

The stored semantic relationships are:

- `Calls`
- `Implements`
- `Embeds`
- `Accepts`
- `Returns`
- `FieldType`

`Imports` is stored separately because a direct Go import and a semantic
dependency are different facts.

`FieldType` connects a represented struct to represented structs or interfaces
that occur in a named, non-embedded field's declared type. Exact node
navigation uses the six semantic kinds. A semantic package
dependency exists when one of those relationships crosses a represented
package boundary. A type dependency exists when both endpoints have represented
struct/interface owners; methods map to their owner, package functions are
excluded, and same-type facts are suppressed. Projections retain every exact
fact as evidence instead of introducing new stored edge kinds.

Dependency means current reliance. Dependent means an entity that currently
relies on the selected entity. Impact is reserved for consequences of a
hypothetical change. These terms must not be used interchangeably.

## Determinism and evidence

User-visible nodes, relationships, status reasons, paths, impacts, and
diagnostics are sorted deterministically. Traversals use path-local cycle
guards and return distinct simple paths. Package and type path identities use
their projected symbol sequences; exact paths use their relationship steps.

Relationship evidence records source locations that established the fact.
Package/type projections aggregate exact evidence. Evidence need not provide
IDE-grade ranges, but it must remain precise enough to verify the relationship
in source.

## Analysis lifecycle

An analysis attempt has three product outcomes:

```text
failed                         nil Analysis plus error
succeeded, complete            usable supported model
succeeded, partial             usable model with known possible omissions
```

Complete is scoped to supported semantics, selected non-test packages, and the
active Go build environment. It does not mean all configurations or Go
constructs were analyzed.

Partial currently means incomplete type information may have prevented facts
that NOCV intends to model. Reasons are coarse, package-scoped, deduplicated,
and deterministic. Positive facts remain useful; missing facts are not
conclusive. Status stays on `Analysis` and is communicated at CLI/web
boundaries, not copied into graph/query results.

List, parse, load, zero-package, and graph-invariant failures return no usable
analysis. Type-check errors may produce Partial when sufficient compiler state
survives.

## Hypothetical signature changes

`Analysis.AnalyzeSignatureChange` is the single supported signature-change
path. A proposal contains ordered parameter types, ordered result types, and
variadic state. Parameter/result names are descriptive and do not participate
in semantic identity.

One resolved proposed signature feeds four distinct lenses:

1. direct call-site compatibility for parameter changes;
2. compiler diagnostic consequences from an in-memory overlay recheck;
3. loss of existing interface contracts;
4. concrete promoted-method surface changes through embedding.

The compiler recheck covers the changed package and represented reverse-import
closure. Exact baseline diagnostics are suppressed; shifted matching dirty-
baseline diagnostics are conservative `uncertain`; unmatched diagnostics are
`new`. Diagnostic absence does not establish behavioral correctness.

Semantic no-ops skip call-site, compiler, contract, and structural work.
Receiver changes, generic/type-parameter changes, gained contracts, source
rewriting, and test execution are outside the current contract.

## CLI ownership

The supported namespace is:

```text
serve
tree
node {dependencies, dependents, transitive-dependents, dependency-paths, inspect}
go package {imports, importers, dependency-paths, inspect-dependency,
            check-forbidden-import, check-forbidden-dependency,
            check-import-cycle}
go type {dependencies, dependency-paths}
```

`node` is the exact graph boundary. Immediate relationships are the default;
recursive or path behavior is explicit in the operation name. `go package` and
`go type` contain semantics that depend on Go package/import and type ownership
rules. `tree` remains a structural-orientation command. `serve` composes the
maintained local web workflow.

A supported command must answer a maintained product question. Global debug
dumps and compatibility aliases are not a permanent command class. Future
question-oriented `why` or `impact` commands, if added, must compose existing
precise capabilities instead of introducing a second semantic engine.

## Multi-language boundary

Multi-language readiness comes from keeping compiler-specific state out of the
graph/query layers, not from speculative abstraction. `goanalyzer` remains
intentionally Go-specific. Do not add generic analyzer/provider interfaces or
`common`/`shared` packages before a second implementation demonstrates a real
common contract.

## Readability and change policy

- Prefer domain names and explicit control flow over generic frameworks.
- Keep traversal helpers query-specific unless a proven shared abstraction
  makes every caller clearer.
- Keep Go loading, types, overlays, contracts, promotion, and diagnostics in
  one compiler-owned package until a concrete split improves cohesion.
- Preserve imports, exact semantics, package projection, type projection, and
  hypothetical impact as distinct concepts even when their DTOs look similar.
- Each behavioral change requires focused tests and a full test pass.
- Production changes must not depend on test fixtures.
