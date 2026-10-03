# Beta CLI namespace design

This document records the Task 53 design for NOCV's beta command namespace. It
supersedes the flat-name recommendations from Task 52 where a scope now carries
the same meaning more clearly. It does not change any command, parser, query,
graph semantic, or output.

## Decision summary

The recommended command model is:

```text
precise graph operations
    nocv node ...

precise Go-defined operations
    nocv go package ...
    nocv go type ...

runtime entry point
    nocv serve

future question-oriented composition
    nocv why ...
    nocv impact ...
```

`node` is a NOCV product concept. It means one represented entity in the
semantic graph and is addressed today by a textual `SymbolRef`. It does not
mean, expose, or require knowledge of the graph-local `NodeID` storage key.

`go` is a literal scope for operations whose public meaning requires current Go
concepts or rules. It is not a language plugin, provider, registration point,
or promise about another language.

## Namespace boundaries

### `node`: exact graph entities and relationships

The `node` boundary is:

> Any operation over exact represented graph nodes and stored relationships,
> regardless of which analyzer originally established those facts.

This is option A from the Task 53 question. It is preferable to limiting
`node` to an artificial set of “language-neutral relationship kinds.” Once a
`Calls`, `Implements`, `Embeds`, `Accepts`, or `Returns` edge is stored, direct
navigation and path traversal need only the graph. A caller does not need to
know that Go analysis produced the edge.

This boundary also aligns the CLI with visualization: the visual graph contains
nodes and relationships, and `nocv node` addresses those same conceptual
entities by stable textual reference.

The recommended node operations are:

```text
nocv node dependencies <pattern> <node-ref>
nocv node dependents <pattern> <node-ref>
nocv node transitive-dependents <pattern> <node-ref>
nocv node dependency-paths <pattern> <from-node-ref> <to-node-ref>
nocv node inspect <pattern> <node-ref>
```

They map directly to `query.DirectDependencies`, `DirectDependents`,
`TransitiveDependents`, `DependencyPaths`, and `InspectNode`. No compiler state
or Go-specific projection is needed to execute these operations on an existing
graph.

### Immediate relationships are the node default

Within `node`, unqualified `dependencies` and `dependents` mean one-hop stored
semantic relationships. The recommendation intentionally drops `direct-`:

```text
node dependencies
node dependents
node transitive-dependents
```

This reads naturally because the scope establishes the level and a plural
relationship query normally means immediate adjacency. `transitive-` remains
explicit because it changes the operation into recursive reachability. Help at
the `node` level must state this default rather than relying on intuition.

Keeping `direct-` would be more locally explicit but creates noisy commands and
does not remove the need to explain the five semantic relationship kinds. If a
future forward transitive closure proves useful, `node transitive-dependencies`
would be discoverable without renaming the immediate operation. Task 53 does
not add that capability merely for symmetry.

### `go`: semantics that require Go concepts or rules

The `go` boundary is:

> Operations whose user-visible meaning depends on Go-specific package,
> import, ownership, struct/interface, or method semantics.

This is option A from the Task 53 question. It is not “everything produced by
`goanalyzer`.” Exact node traversal remains under `node` even when its stored
facts came from Go analysis. Conversely, import relationships and the current
package/type projections are Go-scoped because the scopes and ownership rules
are defined by the loaded Go program.

The hierarchy is literal concrete CLI structure. It does not require matching
Go source packages, a `LanguageBackend`, registration hooks, or a generic nested
command framework.

## Recommended beta hierarchy

```text
nocv
├── serve
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

future top-level wrappers
├── why
└── impact
```

Only `serve`, `node`, and `go` belong to the implemented beta hierarchy. `why`
and `impact` show reserved placement; Task 53 does not add them.

## Go package operations

The `go package` scope supplies both language and projection context, so its
operation names should not repeat `go` or `package`:

| Recommended command | Meaning | Current capability |
|---|---|---|
| `go package imports` | Direct represented packages imported by one package | `query.DirectImports` |
| `go package importers` | Direct represented packages importing one package | `query.DirectImporters` |
| `go package dependency-paths` | Every simple route through the five-edge semantic package projection | `query.PackageDependencyPaths` |
| `go package inspect-dependency` | Package routes with each hop divided into type-backed and exact-only facts | `query.PackageDependencyPaths` plus `query.InspectPackageDependency` |
| `go package check-forbidden-import` | Whether one exact direct Go import exists | `query.CheckForbiddenPackageImport` |
| `go package check-forbidden-dependency` | Whether any semantic package dependency route exists | `query.CheckForbiddenPackageDependency` |
| `go package check-import-cycle` | Whether adding a proposed import would close an existing import path | `query.WouldCreateImportCycle` |

`check-import-cycle` is preferable to `import-cycle`: the operation evaluates a
hypothetical proposed edge rather than listing cycles in the current graph.
The verb also aligns it with the two architecture-rule checks without creating
a generic top-level `check` namespace.

Imports remain distinct from semantic dependencies inside this scope. Package
dependency paths use `Calls`, `Implements`, `Embeds`, `Accepts`, and `Returns`;
they do not silently include imports.

Dedicated `go package dependencies` and `go package dependents` commands do not
exist today and are not added for symmetry. `node inspect` on a package already
shows direct semantic dependencies, dependents, imports, and importers. The
path, explanation, and rule operations above remain valuable distinct package
capabilities.

## Go type operations

The current type projection uses represented Go structs and interfaces, maps
their methods to enclosing types, excludes package functions, suppresses
same-type facts, and aggregates exact semantic evidence. `go type` is therefore
more truthful than a universal top-level `type` scope.

```text
nocv go type dependencies <pattern> <type-ref>
nocv go type dependency-paths <pattern> <from-type-ref> <to-type-ref>
```

These map to `query.DirectTypeDependencies` and `TypeDependencyPaths`.

No dedicated `go type dependents` command exists. Type inspection already
exposes direct type dependents, and Task 53 does not add a command solely for
symmetry. Likewise, `go type inspect` would duplicate `node inspect` without a
different read model.

## Inspection placement

`nocv node inspect` remains the single-node inspection operation. The existing
`query.NodeInspection` dispatches by node kind:

- package nodes show contents, semantic dependencies/dependents, and
  imports/importers;
- struct/interface nodes show methods, projected type relationships, and exact
  relationships;
- function/method nodes show their categorized exact relationships.

That kind-aware rendering does not require duplicate `go package inspect` or
`go type inspect` commands. The stable input is still a textual node ref.

`nocv go package inspect-dependency` is not a duplicate. It accepts two package
refs and explains one or more package dependency routes and their hop evidence,
instead of inspecting a single package node.

## Dependency-path symmetry under scopes

Using `dependency-paths` consistently is beneficial because each namespace
supplies the missing projection level:

```text
nocv node dependency-paths
    exact declaration-level semantic paths

nocv go package dependency-paths
    paths through aggregated package semantic dependencies

nocv go type dependency-paths
    paths through aggregated type semantic dependencies
```

The shared operation phrase aids discovery without pretending the three result
models are interchangeable. Their existing query APIs remain explicit and
separate.

## Existing top-level and diagnostic commands

The complete current command inventory contains no `analyze` command. Its
remaining non-dependency commands are classified as follows:

| Current command | Classification | Recommended beta placement |
|---|---|---|
| `serve` | Runtime/composition entry point for the maintained UI | Keep top-level as `nocv serve` |
| `inspect-node` | Core exact-node inspection | Move to `nocv node inspect` |
| `tree` | Global hierarchy diagnostic dump | Remove before beta, subject to human approval |
| `calls` | Global dump of one stored relationship kind | Remove before beta; node dependencies/inspection expose precise call facts |
| `implementations` | Global dump of stored `Implements` relationships | Remove before beta; node navigation/inspection retains the capability |
| `embeddings` | Global dump of stored `Embeds` relationships | Remove before beta; node navigation/inspection retains the capability |
| `signatures` | Global dump of represented `Accepts`/`Returns` edges, not complete Go signatures | Remove before beta; its name is especially misleading |

`calls`, `implementations`, and `embeddings` are graph facts after extraction,
but their current commands are unscoped all-project debug dumps. Moving them to
`node` would suggest maintained per-node operations that they are not. The
general `node dependencies` and kind-aware `node inspect` preserve direct access
without permanently supporting global diagnostic commands.

Go interface satisfaction and embedding determine how current edges are
created, but generic navigation over existing `Implements` and `Embeds` edges
still belongs under `node`. A future Go-specific operation that recomputes or
hypothesizes contract/promotion behavior would belong under `go` or top-level
`impact` according to its user question.

## Future question-oriented wrappers

`nocv why` remains top-level. It should answer explanation-oriented questions
by selecting and composing precise node, package, or type relationship/path
capabilities from endpoint context. Its argument grammar is deferred. It must
use the maintained five-edge package view rather than inherit the legacy
call-only `WhyDependsOn` semantics.

`nocv impact` also remains top-level. It asks what would happen under a
hypothetical change and composes concrete change analyses. It must not become a
synonym for current dependencies, dependents, or transitive reachability. Its
mutation grammar is deferred.

These wrappers are additive. Major deterministic capabilities remain directly
callable through `node` and `go`.

## Current-to-beta migration map

All examples preserve the current required load pattern after the operation.

| Current command | Recommended beta command or disposition | Reason |
|---|---|---|
| `serve` | `serve` | Runtime entry point remains top-level |
| `inspect-node` | `node inspect` | Node scope makes the old suffix redundant |
| `direct-deps` | `node dependencies` | Exact immediate outgoing semantic relationships |
| `direct-dependents` | `node dependents` | Exact immediate incoming semantic relationships |
| `transitive-dependents` | `node transitive-dependents` | Recursive reverse reachability remains explicit |
| `paths` | `node dependency-paths` | Scope identifies exact-node projection; operation identifies paths |
| `package-imports` | `go package imports` | Direct Go package imports |
| `package-importers` | `go package importers` | Direct reverse Go package imports |
| `import-cycle` | `go package check-import-cycle` | Makes the hypothetical check explicit |
| `check-forbidden-import` | `go package check-forbidden-import` | Go package scope supplies omitted context |
| `package-paths` | `go package dependency-paths` | Modern semantic package projection |
| `inspect-dependency` | `go package inspect-dependency` | Two-package path/evidence inspection |
| `check-forbidden-dependency` | `go package check-forbidden-dependency` | Package-scoped semantic architecture rule |
| `type-deps` | `go type dependencies` | Direct Go type projection; immediate is default |
| `type-paths` | `go type dependency-paths` | Go type projection paths |
| `imports` | Remove | Approved Task 52 removal of global import dump |
| `package-deps` | Remove | Approved removal of legacy call-only package projection |
| `why-package-dep` | Retire into future `why`; preserve useful call locations in maintained package evidence | Approved removal of incomplete public wrapper |
| `tree` | Remove, pending human approval | Global diagnostic surface |
| `calls` | Remove, pending human approval | Global relationship diagnostic superseded by node operations |
| `implementations` | Remove, pending human approval | Global relationship diagnostic superseded by node operations |
| `embeddings` | Remove, pending human approval | Global relationship diagnostic superseded by node operations |
| `signatures` | Remove, pending human approval | Incomplete global relationship dump with misleading name |

Task 52's flat recommendations—such as `direct-dependencies`,
`package-dependency-paths`, `type-dependency-paths`, and
`inspect-package-dependency`—remain part of the historical decision record.
This design supersedes them with scoped forms because `node`, `go package`, and
`go type` carry the level/domain context without repeated prefixes.

## Meaningful alternative considered

The credible alternative is:

```text
nocv node ...
nocv package ...
nocv type ...
```

It is shorter, and package/type concepts are prominent in the current product.
It is not recommended. The current type projection specifically depends on Go
struct/interface/method ownership, and imports are Go package imports. Leaving
`package` and `type` top-level would reserve apparently universal vocabulary for
current Go rules and make a later language boundary less honest.

The explicit `go` scope costs one word but teaches the correct model without
designing any future language namespace. It also groups related import and
architecture-rule operations cleanly.

## Help and discoverability contract

The scoped design should support these conceptual help levels:

- `nocv --help`: introduce `serve`, graph-level `node`, Go-specific `go`, and
  reserve explanation/change terminology for future `why` and `impact` only
  when those commands exist.
- `nocv node --help`: define a node as a represented graph entity addressed by
  textual ref; state that dependencies/dependents are immediate and list the
  five traversed relationship kinds.
- `nocv go --help`: state that this is the literal current Go semantic scope,
  then expose `package` and `type`.
- `nocv go package --help`: distinguish direct imports from derived semantic
  package dependencies and distinguish navigation, evidence, and checks.
- `nocv go type --help`: describe the struct/interface/method-owned projection
  and explain that package functions do not participate.

Concrete usage lines must continue to show `<pattern>` and textual refs. Help
should call them node, package, or type refs rather than exposing the Go type
name `SymbolRef`, and must never ask users for `NodeID`.

An agent can discover precise operations incrementally from these levels
without knowing historical flat names. No agent-specific protocol is needed.

## Naming policy for beta additions

1. Scope names identify the domain or projection; operation names identify the
   action and do not repeat the scope.
2. Under `node`, `go package`, and `go type`, unqualified plural relationships
   are immediate. Recursive operations say `transitive` or `paths` explicitly.
3. Use `dependency` for current reliance and `dependent` for reverse current
   reliance. Imports retain import-specific terms.
4. Hypothetical boolean/rule operations use an action such as `check-` when
   that prevents them from looking like current-state listings.
5. `why` means explanation-oriented composition; `impact` means consequences
   of a hypothetical change.
6. Do not preserve historical abbreviations such as `deps` when a scoped,
   readable full word is available.
7. Do not add reverse or projected commands merely for visual symmetry; require
   a maintained product use.

## Mechanical findings and product decisions

Mechanical conclusions:

- Exact navigation APIs operate only on the stored graph and fit `node`.
- Imports, package projection, and the current type projection have concrete Go
  meanings and fit literal `go package` / `go type` scopes.
- `NodeInspection` already handles packages, types, and callables, so scoped
  inspection aliases would duplicate one read model.
- Current package/type inspection exposes reverse projections, but no dedicated
  package/type dependent CLI exists.
- The current parser is flat and concrete; implementing this hierarchy does not
  require reflection, plugins, command registration, or language interfaces.
- Inline recursive query `walk` closures remain separate readability debt and
  must not be refactored as part of namespace implementation.

Product decisions requiring human approval:

1. Approve `node` as the exact-graph boundary regardless of relation origin.
2. Approve literal `go` as the boundary for operations requiring current Go
   concepts/rules, rather than for everything produced by `goanalyzer`.
3. Approve immediate-by-default `node dependencies` / `node dependents` and the
   removal of their `direct-` prefix.
4. Approve the exact `go package` and `go type` operation names, especially
   `check-import-cycle`.
5. Approve omitting duplicate `go package inspect`, `go type inspect`, and
   package/type dependent commands.
6. Approve removal of `tree`, `calls`, `implementations`, `embeddings`, and
   `signatures` as debug surfaces in addition to the already-approved Task 52
   legacy removals.
7. Confirm future `why` and `impact` remain top-level wrappers with deferred
   grammars.

No hierarchy implementation should begin before this review gate is resolved.
