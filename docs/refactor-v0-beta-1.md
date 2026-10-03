# NOCV pre-beta refactor inventory

This document records the repository as inspected on the `refactor/v0-beta-1`
branch before broad cleanup. It is an inventory and decision aid, not a target
architecture. The semantic MVP is frozen while these decisions are reviewed.

## Scope and method

The review covered the production packages, their tests, `examples/impactdemo`,
the current CLI and browser UI, and the retained Go compiler state. Evidence was
collected from both Go imports and NOCV's own package, node, signature, and
dependency queries.

Two dependency views are intentionally kept separate throughout this report:

- **Go import DAG** means dependencies enforced by Go package imports.
- **Semantic dependency projection** means cross-package `Calls`, `Implements`,
  `Embeds`, `Accepts`, and `Returns` facts projected from graph symbols to their
  packages. `Imports` is a separate graph edge and is not part of that semantic
  projection.

The current `package-deps` CLI command is narrower than the latter definition:
it uses `query.Dependencies` and reports cross-package calls only. The web
`/api/packages` view uses the newer semantic package view. This distinction is
important evidence for the vocabulary review below.

## Current package map

| Package | Owns today | Depends on | Depended on by | Cohesive responsibilities | Misplaced or historical responsibilities |
|---|---|---|---|---|---|
| `graph` | Node identity, hierarchy, locations, semantic/import edges, indexes, validation, detached access | Standard library only | `query`, `goanalyzer`, `internal/webui`, `cmd/nocv` | The graph kernel is focused and language-neutral | None found |
| `query` | Direct navigation, import and semantic paths, package/type projections, inspections, explanations, rules, transitive dependents, symbol summaries | `graph` | `goanalyzer`, `internal/webui`, `cmd/nocv` | Graph-derived projections and read models are cohesive and language-agnostic | Task 47 removed its former Go-specific signature/change DTOs |
| `goanalyzer` | Go loading, graph extraction, symbol index, compiler/type state, call-site analysis, signature resolution, contracts, promotion, source overlay, compiler recheck, diagnostic comparison and attribution | `graph`, `query`, `go/packages`, compiler packages | `internal/webui`, `cmd/nocv` | These operations share one loaded Go type universe and symbol index | The package has distinct internal areas. No package split is yet justified solely by size |
| `internal/webui` | HTTP server, JSON presentation DTOs, embedded assets, package graph, inspector/navigation state, signature-change editor and results | `goanalyzer`, `graph`, `query` | `cmd/nocv` | It is a coherent internal adapter from analysis/read models to one UI | `app.js` carries several workflows in one mutable module; Task 47 renamed the stale impact file |
| `cmd/nocv` | Command parsing, project loading orchestration, query invocation, text formatting, web serving, diagnostic/debug workflows | `goanalyzer`, `graph`, `query`, `internal/webui` | Executable only | It is the composition root, so high fan-out is expected | Some dependency commands still need separate terminology review; Task 51 removed the dependents/impact collision |
| `examples/impactdemo` | Small, separate Go module demonstrating call, result, contract, embedding, compiler, and dirty-baseline scenarios | Its own standard-library/local packages; no NOCV production dependency | Manual validation only | Purpose-built behavior fixture | Its README is manually maintained and can drift from analyzer behavior; that is a test-documentation risk, not a package ownership problem |

`graph` remains the stable lower layer. After Task 47, `query` contains only
language-independent graph interpretation and detached symbol/read models;
Go-specific signature and change data is owned by `goanalyzer`.

## Actual dependency map and self-analysis

### Go imports

```text
cmd/nocv ───────→ internal/webui ───────→ goanalyzer
   │                    │                    │
   │                    ├──────────────────→ query
   │                    └──────────────────→ graph
   ├───────────────────────────────────────→ goanalyzer
   ├───────────────────────────────────────→ query
   └───────────────────────────────────────→ graph

goanalyzer ─────→ query ──────────────────→ graph
     └────────────────────────────────────→ graph
```

The import graph is acyclic. `graph` has fan-in four and fan-out zero.
`query` has fan-in three and fan-out one. `cmd/nocv` has fan-out four as the
composition root; `internal/webui` has fan-out three; `goanalyzer` has fan-out
two. These counts are not smells by themselves.

### NOCV semantic package projection

`/api/packages` returned the same ten directed package pairs as the import DAG:

| From | To |
|---|---|
| `nocv/cmd/nocv` | `nocv/goanalyzer`, `nocv/graph`, `nocv/internal/webui`, `nocv/query` |
| `nocv/internal/webui` | `nocv/goanalyzer`, `nocv/graph`, `nocv/query` |
| `nocv/goanalyzer` | `nocv/graph`, `nocv/query` |
| `nocv/query` | `nocv/graph` |

No semantic package cycle exists in the current NOCV source. Exact symbol
paths may contain cycles in arbitrary analyzed programs, and query tests protect
cycle-safe traversal, but self-analysis does not justify inventing a package
cycle concern here.

The old `package-deps` command omitted `goanalyzer → query` because that command
projects calls only. In the Task 44 snapshot, `inspect-dependency` explained the
edge through both Go-specific change DTOs and `SymbolSummary`. Task 47 moved all
Go-specific DTOs. The remaining import is intentional: Go-owned impact and
diagnostic models reuse the language-agnostic `query.SymbolSummary`, and analyzer
summary helpers return it. This is no longer a Go-model ownership leak.

Answers from self-analysis:

- Everything except `examples/impactdemo` depends directly or transitively on
  `graph`; `graph` has no production dependency and no outgoing semantic edge.
- `goanalyzer` depends on `query` for shared detached signature, call-site,
  contract, structural, and symbol-summary models—not for executing graph
  queries.
- `query` depends only on `graph`, both by imports and projected semantics.
- `graph` has the highest fan-in. `cmd/nocv` has the highest fan-out by design.
- There are no current package cycles to explain. The notable issue is boundary
  direction and terminology, not cyclic compilation.

## Model ownership inventory

“Presentation” below means shaped for a consumer rather than a compiler object;
it does not necessarily mean HTTP-only.

| Model | Package | Main consumers | Character | Ownership assessment |
|---|---|---|---|---|
| `Node` | `graph` | all production packages | Language-independent domain graph fact | Correct |
| `Relationship` | `query` | query results, CLI, web DTO conversion | Language-independent graph-derived read model | Correct; distinct from stored `graph.Edge` |
| `PackageDependency` | `query` | package paths/inspection, CLI, web | Language-independent projected domain read model with exact evidence | Correct |
| `TypeDependency` | `query` | type paths/inspection, CLI | Language-independent projected domain read model with exact evidence | Correct |
| `NodeInspection` | `query` | CLI and web adapters | Language-independent composite inspection read model | Correct, although its optional callable augmentation occurs in webui |
| `PackageInspection` | `query` | `NodeInspection`, CLI/web | Language-independent presentation/read model | Correct |
| `TypeInspection` | `query` | `NodeInspection`, CLI/web | Language-independent presentation/read model | Correct |
| `FunctionInspection` | `query` | `NodeInspection`, CLI/web | Language-independent relationship read model | Correct; callable signature is intentionally not embedded here |
| `SymbolSummary` | `query` | inspections, analyzer impacts, CLI/web | Language-independent detached semantic/presentation summary | Legitimately shared, but analyzer-side construction duplicates query summary logic |
| `CallableSignature` | `goanalyzer` | analyzer, webui | Go-specific detached domain/presentation model | Moved in Task 47; correct |
| `ProposedSignature` | `goanalyzer` | analyzer, webui request conversion, analyzer tests | Go-specific command/input model (`TypeExpr`) | Moved in Task 47; correct |
| `ParameterChangeImpact` | Removed in Task 46 | None | Historical Go-specific compatibility result | Removed with the legacy parameter-only surface |
| `SignatureChangeImpact` | `goanalyzer` | webui presentation | Go-specific aggregate analysis result | Correct today because it includes compiler recheck results |
| `CallSiteImpact` | `goanalyzer` | analyzer and web presentation | Go-specific semantic result | Moved in Task 47; correct |
| `ContractImpact` | `goanalyzer` | analyzer and web presentation | Go method-set contract result | Moved in Task 47; correct |
| `StructuralImpact` | `goanalyzer` | analyzer and web presentation | Go embedding/promotion consequence | Moved in Task 47; correct |
| `CompilerImpact` | `goanalyzer` | `SignatureChangeImpact`, webui | Go compiler consequence aggregate | Correct |
| `CompilerConsequence` | `goanalyzer` | webui | Go diagnostic delta with optional symbol attribution | Correct |
| `GoTypeRef` | `goanalyzer` | signatures, problems, web presentation | Explicitly Go-specific detached type presentation | Moved in Task 47; correct |

No compiler objects escape `goanalyzer`. Task 47 completed the approved move
without aliases, duplicate models, or a shared package.

## The `query` / `goanalyzer` boundary

The concepts crossing `goanalyzer → query` fall into five groups:

| Category | Current examples | Assessment |
|---|---|---|
| Graph-derived read models | `SymbolSummary` | Genuinely belongs near inspection/query code and is reusable |
| Go contract/promotion concepts | `ContractImpact`, `StructuralImpact`, `MethodExposure` | Moved to `goanalyzer` in Task 47; their current semantics are Go method sets and promotion |
| Go signature/type concepts | `GoTypeRef`, `CallableSignature`, `ProposedSignature`, `Parameter`, `Result`, `Compatibility`, `SignatureProblem`, `CallSiteImpact` | Moved to `goanalyzer` in Task 47 |
| Go compiler consequences | `CompilerImpact`, `CompilerConsequence`, baseline and diagnostic classification | Correctly remain in `goanalyzer` |
| Historical compatibility | `ParameterChangeImpact` | Removed in Task 46 with `impact-params` and `AnalyzeParameterChange` |

Task 47 completed the approved boundary: Go-specific signature/change models
live in `goanalyzer`; `SymbolSummary` remains in `query`. The dependency remains
for that legitimate language-agnostic model, not for misplaced Go DTOs.

## Signature-change ownership and flow

`goanalyzer.Analysis.AnalyzeSignatureChange` is the orchestration boundary. It
resolves one callable and one proposal once, then uses the same resolved full
signature for all four lenses:

| Lens/model | Owner | Responsibility |
|---|---|---|
| `CallableSignature`, `ProposedSignature` | `goanalyzer` | Detached before/after input and display |
| `CallSiteImpact` | `goanalyzer` | Direct source-call compatibility for parameter changes |
| `CompilerImpact` | `goanalyzer` | Overlay compiler consequences across reverse import closure |
| `ContractImpact` | `goanalyzer` | Existing interface contracts lost under the full signature |
| `StructuralImpact` | `goanalyzer` | Existing promoted method surfaces changed under the full signature |
| `SignatureChangeImpact` | `goanalyzer` | Aggregate result preserving the four distinct lenses |

The aggregate and its Go-specific children now live together beside compiler
state in `goanalyzer`. Task 47 completed that ownership move without adding a
shared package or changing the language-agnostic `query.SymbolSummary` model.

## Transitional compatibility inventory

| Item | Classification | Evidence and reason |
|---|---|---|
| `Analysis.AnalyzeParameterChange` | Removed in Task 46 | Unified `AnalyzeSignatureChange` is now the only maintained analysis entry point |
| `query.ParameterChangeImpact` | Removed in Task 46 | Maintained consumers use `SignatureChangeImpact` |
| `impact-params` CLI | Removed in Task 46 | The browser/API unified signature workflow is the maintained product surface |
| `query/parameter_change.go` | Removed in Task 47 | Its Go-specific contents moved to `goanalyzer`; no language-neutral content remained |
| `goanalyzer/signature_analysis.go` | Renamed in Task 47 | It owns unified signature resolution and call-site analysis; the legacy parameter-only wrapper is gone |
| `internal/webui/signature_impact.go` | Renamed in Task 47 | Filename now matches the existing unified endpoint and content |
| `cmd/nocv/parameter_impact.go` | Removed in Task 46 | Its parser and formatter existed only for `impact-params` |
| Parameter-change analyzer tests | Migrated in Task 46 | They exercise the unified API and retain call-site, contract, promotion, constant, variadic, and no-op behavior |
| `hypothetical_recheck_test.go` spike helpers | Consolidate before beta | The test retains independent loading, closure, and diagnostic-delta implementations from the spike; behavior coverage is valuable but duplicate production algorithms are not |
| `/api/signature-impact` | Keep intentionally | It is already the unified endpoint; no `/api/parameter-impact` route remains |

## Vocabulary glossary

| Term | Preferred meaning | Does not mean | Current inconsistency / recommendation |
|---|---|---|---|
| Node | One stored declaration/package entity in the graph | A UI DOM node or projected dependency | Keep; qualify UI nodes when needed |
| NodeID | Internal graph identity key | Public stable symbol spelling | Keep distinct from `SymbolRef` |
| SymbolRef | Stable external reference to a modeled declaration/package | Compiler `types.Object` identity | Keep; use `Ref` only in tightly scoped fields |
| Symbol | Human shorthand for a modeled declaration | Every AST/compiler object | Prefer `symbol` only when a `SymbolRef` or graph node is actually available |
| Relationship | One exact directed semantic/import fact, usually a detached edge view | Transitive path or package projection | Keep for `query.Relationship`; say `edge` for storage |
| Dependency | A direct or projected semantic reliance with evidence | Go import unless explicitly “import dependency” | Overloaded by call-only `Dependencies`; qualify level and projection |
| Dependent | The source that relies on the selected target | An importer in every context | Qualify `semantic dependent` or `importer` |
| Direct | Exactly one stored edge, or one edge in a named projected view | Necessarily one AST reference | Always state the view: exact, package, type, or import |
| Semantic | Derived from modeled program meaning (`Calls`, `Implements`, `Embeds`, `Accepts`, `Returns`) | Imports, hierarchy, or arbitrary text matching | Keep and document the included edge kinds centrally |
| Exact | Declaration-level graph relationship/path | “More correct,” compiler identity, or package projection | Prefer `declaration-level` in user output where clearer |
| Projection | A derived package/type view backed by exact evidence | A newly stored graph edge | Keep |
| Inspection | Read model for one selected node/dependency | Mutation, full project analysis, or compiler validation | Keep |
| Impact | Modeled consequence of a hypothetical change | Existing reverse reliance or an arbitrary dependency | Enforced in Task 51; current reverse reliance uses direct/transitive dependents terminology |
| Consequence | Compiler diagnostic attributable to the hypothetical overlay | Every structured call/contract/structural impact | Keep “compiler consequence” as a qualified term |
| Evidence | Exact relationship/location supporting a derived statement | An inference without a graph fact | Keep |
| Owner | Enclosing package/type used for projection | Go package ownership, receiver identity, or architectural code owner | Prefer `enclosing type/package` unless projection ownership is meant |
| Contract | Existing concrete-type/interface implementation relation | API promise in the broad product sense | Use `interface contract` in prose |
| Structural | Effective concrete method-surface change through embedding/promotion | Generic dependency blast radius | Keep qualified as `promoted-method structural impact` where space permits |
| Compiler | Result produced by Go loading/type checking | Every `go/types`-backed structured check | Reserve “compiler consequence” for overlay diagnostic delta |
| Baseline | Diagnostics from the unmodified loaded source in the affected recheck scope | Proof the entire repository is compiler-clean | Always include scope |
| Overlay | In-memory replacement source supplied to `go/packages` | Disk mutation or general virtual workspace | Keep |
| Signature | Callable parameters, results, and variadic state in this feature | Receiver or type parameters | State this scope in public docs |
| Callable | Modeled package function or method, including interface method where supported | A type, function literal node, or arbitrary function value | Keep |
| Analysis | Retained loaded Go packages, graph, indexes, and operations | Immutable result value or compile-clean guarantee | Current name is broad; decide before renaming |

## Concrete vocabulary ambiguities

| Term | Current examples | Why confusing | Possible replacement | Before beta? |
|---|---|---|---|---|
| Relationship | `graph.Edge` versus `query.Relationship` | Both represent the same fact at storage/read boundaries | Keep both, document `edge` as storage and `relationship` as detached view | Documentation before beta; no rename required |
| Dependency | `Dependencies` is call-only; package/type dependency views include five semantic kinds; imports have separate “dependency” rules | The same label selects different edge sets | `CallDependencies` for the old function/command, or retire it in favor of semantic package dependency | Yes, public CLI decision |
| Impact/dependents | Before Task 51, `query.Impact` meant transitive semantic dependents while signature impact meant hypothetical change analysis | One word named two unrelated workflows | Resolved as `TransitiveDependents`/`transitive-dependents`; impact remains hypothetical | Completed in Task 51 |
| Direct | Direct exact edges and direct projected package/type edges | A projected direct step can summarize many exact facts | Qualify the level in names and headings | Documentation now; code rename can wait |
| Exact | Exact semantic paths | Sounds like a confidence claim | `declaration-level` in prose; retain internal names if tests are clear | Can defer |
| Analysis | `Analysis` stores a live loaded project; methods also return individual analyses | Value and service/session meanings collide | `LoadedProject` or `GoProject` are candidates, not decisions | Human decision; can defer if documented |
| Inspection | Node and package-dependency inspections plus UI inspector state | Domain read model and screen state share the noun | Qualify as `NodeInspection`, `PackageDependencyInspection`, `inspector view` | No urgent rename |
| Signature | Parameter-only legacy files beside full parameter/result implementation | Old names imply narrower scope | Rename files/functions after compatibility decision | Yes for stale public names |
| Owner | `typeOwner`/package projection and ordinary declaration containment | “Owner” hides which hierarchy is being projected | `enclosingType`, `packageOf` where touched | Can defer |

## File naming review

| File | Action | Reason |
|---|---|---|
| `query/parameter_change.go` | Removed in Task 47 | All contents were Go-specific and moved to `goanalyzer/signature_models.go` |
| `goanalyzer/signature_analysis.go` | Renamed in Task 47; keep cohesive | Contains unified signature extraction/resolution and direct call checking |
| `goanalyzer/signature_change.go` | Keep | It now contains the unified impact and diagnostic-status models only |
| `goanalyzer/signature_overlay.go` | Added in Task 48 | Owns callable declaration lookup, source-range selection, signature rendering, and in-memory overlay assembly |
| `goanalyzer/compiler_recheck.go` | Added in Task 48 | Owns the readable compiler-impact orchestration, retained-config reload, and reverse-import closure |
| `goanalyzer/compiler_diagnostics.go` | Added in Task 48 | Owns diagnostic collection, comparison, attribution, and deterministic consequence construction |
| `goanalyzer/contract_impact.go` | Keep | Name matches focused responsibility |
| `goanalyzer/structural_impact.go` | Keep | Name matches focused responsibility |
| `internal/webui/signature_impact.go` | Renamed in Task 47 | Content, filename, and endpoint are now consistently signature-wide |
| `cmd/nocv/parameter_impact.go` | Removed in Task 46 | It implemented only the removed legacy CLI surface |

## What `Analysis` represents

Current retained state is:

```go
graph    *graph.Graph
packages []*packages.Package
symbols  *symbolIndex
loadDir  string
loadMode packages.LoadMode
```

`Analysis` is therefore not simply an analysis result. It is a loaded Go
project snapshot plus its semantic graph, compiler/type information, symbol
index, and enough load configuration to perform hypothetical rechecks.

Natural operations on it are graph access, callable resolution, signature/type
resolution, direct call checking, contract/method-set analysis, promotion
analysis, and overlay compiler rechecking. Graph-only path/projection queries
remain free functions in `query`, which is a useful boundary.

The type does not yet feel like an arbitrary grab bag because every method uses
the same retained compiler universe. The name becomes misleading only if users
need lifecycle, status, reload, or multiple build configurations. Possible
future names such as `LoadedProject` may describe it better, but renaming now
would imply a lifecycle design that has not been chosen.

## Loading and analysis-status semantics

`LoadAnalysis` calls `packages.Load` with `packages.LoadSyntax`, `Tests: false`,
the requested directory/patterns, and the active process Go environment. It
rejects list/parse/non-type package errors, but deliberately ignores
`packages.TypeError` and package summary messages beginning with `# `. It then
constructs nodes and relationships from available syntax and type information.

Today, “load succeeds” means:

1. package discovery/loading did not fail in a rejected category;
2. enough syntax and type state existed to execute graph construction; and
3. an `Analysis` and graph were returned.

It does **not** mean the project is compile-clean. It also does not guarantee
that every intended semantic fact was recoverable. Extraction code generally
skips missing `TypesInfo` objects/types safely, so a dirty project can yield a
valid but incomplete graph.

Task 50 added a project-wide complete/partial status to `Analysis`.
`CompilerImpact.BaselineStatus` remains separate and only describes diagnostics
in the affected reverse-import recheck scope for one hypothetical signature
analysis.

Consequences:

- absence of an edge can be mistaken for proof of no relationship;
- inspection and projection results do not carry a completeness caveat;
- baseline compiler state becomes visible only when signature analysis runs;
- reloading uses the current active environment rather than an explicitly
  persisted build configuration object.

Task 49 rejected `health` as misleading terminology and defined a coarse
first-class **analysis status** contract. A load attempt fails when it cannot
produce a usable semantic model; a returned analysis is either complete or
partial. The design, scenario evidence, and implemented API are recorded in
[`analysis-status-design.md`](analysis-status-design.md). Status stays outside
the graph and query DTOs.

## Signature overlay and compiler architecture

| Step | Current function/file | Responsibility and dependencies | Cohesion |
|---|---|---|---|
| Resolve declaration | `findCallableDeclaration`, `signature_overlay.go` | Match `types.Func`, find `ast.FuncDecl`; fallback AST search for interface method `FuncType` | Focused and compiler-identity-backed |
| Select source range | `signatureSourceRange`, `signature_overlay.go` | Choose parameter opening through result end using token positions and validate byte bounds | Focused source-mutation boundary |
| Render signature | `renderSourceSignature` and helpers, `signature_overlay.go` | Preserve/reconstruct names, tuple form, variadic syntax, parameter/result type source | Cohesive source-rendering operation |
| Create overlay | `buildSignatureOverlay`, `signature_overlay.go` | Read the source file, splice bytes in memory, return filename/content | Cohesive outcome; does not mutate disk |
| Reverse-import closure | `affectedPackagePaths`, `compiler_recheck.go` | Build reverse imports from loaded packages and BFS from changed package | Cohesive and deterministic |
| Baseline diagnostics | `collectCompilerDiagnostics`, `compiler_diagnostics.go` | Gather diagnostics for allowed affected packages | Cohesive |
| Overlay load | `recheckPackagesWithOverlay`, `compiler_recheck.go` | Call `packages.Load` with retained directory/mode and one overlay | Distinct from source construction |
| Overlay diagnostics | `collectCompilerDiagnostics`, `compiler_diagnostics.go` | Gather diagnostics from rechecked packages | Cohesive |
| Compare diagnostics | `compareCompilerDiagnostics` and helpers, `compiler_diagnostics.go` | Count-sensitive exact match; same package + normalized message becomes uncertain; unmatched becomes new | Cohesive, policy-bearing operation |
| Attribute symbols | `Analysis.attributeDiagnostic`, `compiler_diagnostics.go` | Parse location, find enclosing function/method AST, map/build `SymbolSummary`; leave package-level diagnostics unattributed | Cohesive but dependent on loaded AST and symbol conventions |

These are five distinct concepts: hypothetical source mutation, compiler
recheck, diagnostic collection, diagnostic comparison, and attribution. They
remain private operations in `goanalyzer`; no new framework or package is
warranted. Task 48 separated them into focused same-package files while keeping
`compilerImpact` as the short coordinator.

## Implementation hotspots

Line count was used only to find candidates; the reasons below are conceptual.

| Area/function | Responsibilities mixed | Why reading is difficult | Plausible extraction | Value now |
|---|---|---|---|---|
| `addPackage`, `goanalyzer/analyzer.go` | Ordered files/docs, package/type/function nodes, identities, compiler-object index | Central extraction path must preserve many identity and documentation invariants | Keep coordinator; named helpers by declaration kind only when touched | Probable debt, not urgent |
| `addCalls`, `goanalyzer/analyzer.go` | AST traversal, lexical function-literal ownership, object lookup, call edge/evidence creation | Closure attribution rules are embedded in traversal state | Named call-attribution traversal helper | Useful only with focused tests; defer |
| `AnalyzeSignatureChange`, `goanalyzer/signature_analysis.go` | Resolve, validate, compare, collect four impact lenses, sort | Orchestration is broad but linear and expresses product semantics | Keep as coordinator; extract shared result sorting | Mostly intentional complexity |
| `resolveProposedSignature`, same file | Parse type expressions, declaration-context resolution, validate variadic shape, construct compiler/model signatures | Compiler and detached DTO construction are interleaved | Separate type-expression resolution from model assembly only if readability requires it | Probable debt |
| `checkCallSite`, same file | Argument extraction, count/ellipsis rules, compiler assignability/constants, problems DTO | Dense because Go call semantics are genuinely detailed | Keep focused helpers already present; no package split | Intentional complexity |
| Signature overlay path | Declaration lookup, range selection, file IO, render, byte splicing | Task 48 gave each meaningful operation a focused helper in `signature_overlay.go` | Keep concrete and signature-specific | Resolved in Task 48 |
| `compilerImpact`, `compiler_recheck.go` | Scope, baseline, overlay, reload, compare, attribute, sort | Reads as a short sequence of focused operations after Task 48 | Keep as coordinator | Resolved in Task 48 |
| Contract helpers, `contract_impact.go` | Candidate selection, method-set substitution, summaries, deduplication | Domain is complex but helpers are already named | No extraction unless ownership moves | Intentional complexity |
| Structural helpers, `structural_impact.go` | Reverse embedding traversal and `go/types` method-set proof | Small and cohesive | Keep | No debt found |
| `packageDependencyView` / `typeDependencyView`, `query` | Projection, aggregation, evidence sorting/deduplication | Parallel structures look duplicative but encode different ownership rules | Consolidate only if behavior starts diverging | Harmless duplication |
| `InspectNode`, `query/node_inspection.go` | Node-kind dispatch and assembly of several projections | Broad DTO construction is expected for an inspection facade | Per-kind builders if new fields increase branching | Can defer |
| `executeCommandWithAnalysis`, `cmd/nocv/commands.go` | Dispatch, validation, query selection | Switch is long but explicit and testable | Do not add command framework; move only coherent command implementations | Intentional at current scale |
| `print.go` | Text rendering for unrelated query domains | A change requires navigating one 493-line formatter file | Same-package files grouped by inspection/path/rule output | Safe later mechanical work |
| `app.js` | Graph/layout/search, API calls, inspector state, dependency navigation, signature editor, impact presentation, stale guards | Independent workflows share mutable DOM/state and generation invariants | Same-runtime modules or focused regions/helpers after behavior tests improve | Probable debt; framework choice not implied |

### Nested and local functions

- Path DFS closures in `query` express path-local visited state next to each
  traversal. They are understandable and should stay unless reused.
- AST callbacks in `addCalls`, `collectCallSites`, and `findCallableDeclaration`
  encode meaningful attribution rules. Task 48 named the declaration operation;
  mechanically extracting every `ast.Inspect` callback would not help.
- Browser event callbacks are expected. The problem is workflow state shared
  across graph, inspection, and signature analysis, not the mere presence of
  closures.

## Duplication inventory

| Duplication | Classification | Reason |
|---|---|---|
| Unified and legacy analyzer orchestration/sorting | Resolved in Task 46 | The legacy path was removed; `AnalyzeSignatureChange` is authoritative |
| Production overlay algorithms and Task 41 spike helpers in tests | Worth consolidating | Two implementations can make tests pass while production changes independently |
| Package/type projected dependency builders | Harmless for now | Similar mechanics but different ownership and endpoint rules; a generic abstraction may obscure semantics |
| `query.symbolSummary` and analyzer summary construction | Review during the approved model move | Same presentation concept is built from different source domains; `SymbolSummary` remains in `query`, so forced consolidation may create the wrong dependency |
| CLI, web, and analyzer signature formatting | Partly intentional | CLI text, editable Go expressions, and browser display have different constraints; semantic signature construction must remain single-source in analyzer |
| Web presentation DTO copies | Intentional | They make the HTTP boundary explicit and avoid exporting compiler internals or binding JSON to internal structs |
| Repeated deterministic sorts | Harmless | Explicit ordering is a public invariant and local comparators are easy to audit |

## Tests as behavior documentation

| Suite | Current value | Coupling / action during refactor |
|---|---|---|
| `graph` tests | Strong identity, validation, hierarchy, defensive-copy behavior | Preserve as foundational invariants |
| `query` tests | Strong fixtures for edge kinds, projections, evidence, paths, cycles, determinism | Behavior-oriented; keep even if internals move |
| `goanalyzer` tests | Strong compiler-backed coverage of calls, results, contracts, promotion, diagnostics and loading | Parameter-named tests remain useful, but migrate callers off the legacy API; remove duplicate spike algorithms while retaining scenarios |
| `internal/webui` HTTP tests | Exercise real endpoint validation/presentation | Behavior-oriented and valuable |
| `internal/webui` asset tests | Assert exact source strings and helper names in `app.js` | Brittle implementation tests used in lieu of a JS harness; replace selected critical flows with DOM/behavior tests before splitting the file, not all at once |
| CLI tests | Assert exit behavior and deterministic human output | Presentation coupling is appropriate; update deliberately when vocabulary changes |
| `impactdemo` | Human-readable end-to-end scenarios across packages | Keep as manual semantic check; prevent README from becoming the only assertion for a scenario |

## Frontend and HTTP inventory

`internal/webui/static/app.js` is a single 1,091-line script. Its pressure comes
from several independent workflows sharing the selected package, selected
symbol, inspector mode, graph network, DOM helpers, and request generation—not
from line count alone. The explicit inspector states (`symbol`, signature edit,
signature result) and shared stale-request generation token are good invariants.

Current endpoints:

| Endpoint | Input/output ownership | Notes |
|---|---|---|
| `GET /api/packages` | Web presentation DTO built from query package projection | Language-independent package graph; exact semantic edge detail is not returned |
| `GET /api/node` | Web DTO wrapping `query.NodeInspection`, augmented with `Analysis.CallableSignature` | Hybrid endpoint: general inspection plus Go-specific callable signature |
| `GET /api/package-dependency` | Web DTO built from `query.PackageDependencyInspection` | Language-independent presentation of exact semantic evidence |
| `POST /api/signature-impact` | Go-specific JSON proposal converted to `goanalyzer.ProposedSignature`; response presents `goanalyzer.SignatureChangeImpact` | Correctly delegates semantics; shape is a beta API decision, not a general graph endpoint |

The HTTP DTO duplication is intentional boundary code. The former stale
`parameter_impact.go` filename was resolved in Task 47; source-string UI tests
remain a temporary artifact. JavaScript
does not reimplement call compatibility, contract, promotion, or diagnostic
classification; it presents backend results. Navigation correctly reuses symbol
inspection, and late requests are guarded by the common generation token.

No frontend framework or runtime choice follows from this inventory.

## CLI inventory

| Commands | Classification | Notes |
|---|---|---|
| `serve` | Keep | Current primary UI entry point |
| `inspect-node`, `inspect-dependency` | Keep | Current inspection/explanation concepts |
| `package-imports`, `package-importers`, `import-cycle`, `check-forbidden-import` | Keep | Exact import workflows are distinct and clearly named |
| `check-forbidden-dependency` | Keep, clarify semantic edge set | Useful architecture check; “dependency” must be documented |
| `package-paths`, `type-paths`, `paths` | Keep or rename coherently later | Useful layered path concepts; output levels must remain explicit |
| `tree`, `calls`, `implementations`, `embeddings`, `signatures` | Debug-only candidates | Valuable analyzer diagnostics, not necessarily beta product surface |
| `direct-deps`, `direct-dependents` | Debug-only / rename | Exact semantic navigation, but “direct” and level are under-specified |
| `type-deps` | Keep as diagnostic/product aid | Uses the full type projection and evidence |
| `package-deps`, `why-package-dep` | Replace later or rename | Older call-only projection overlaps newer semantic package paths/inspection |
| `transitive-dependents` | Keep | Recursive reverse declaration-level semantic closure; renamed from the pre-beta `impact` command in Task 51 |

Formatting is centralized in `print.go` plus parameter-specific formatting.
Explicit functions are preferable to a generic rendering framework, but the
file can later be split mechanically by output domain.

## Graph, query, analyzer, and command boundaries

### `graph`

The package remains focused on nodes, stable references, hierarchy, semantic
and import edges, indexes, validation, deterministic access, and defensive
copies. No query projection, presentation DTO, Go compiler object, callable
signature, or impact model leaks into it.

### `query`

The package is a coherent library of graph-derived navigation, projections,
paths, rules, explanations, inspections, and language-agnostic semantic read
models. Task 47 removed its former second role as owner of detached Go-specific
signature and impact models. There is no evidence for splitting the remaining
query files.

### `goanalyzer`

Loading, extraction, indexes, type resolution, call semantics, contracts,
promotion, overlay generation, rechecking, and attribution all require the same
`go/packages`/`go/types` universe. They remain cohesive as internal areas of one
Go analyzer package. Concrete same-package boundaries are emerging between:

- initial project load and graph extraction;
- signature resolution/direct call analysis;
- contract and structural semantics;
- source overlay construction;
- compiler recheck and diagnostic policy.

That supports file-level separation, not yet a new exported package boundary.

### `cmd/nocv`

The package contains parsing, application orchestration, query execution, web
serving, debug commands, and text presentation. As a small executable adapter,
that is acceptable. Obvious low-risk separation is grouping text formatters by
domain. A command framework would add indirection without solving a current
problem.

## Architecture smells, with classification

| Evidence-backed smell | Classification | Why |
|---|---|---|
| Go-specific signature/type DTOs were owned by otherwise graph-oriented `query` | Resolved in Task 47 | The DTOs now live in `goanalyzer`; its remaining production dependency on `query` is the approved language-agnostic `SymbolSummary` |
| Legacy parameter-only orchestration | Resolved in Task 46 | The wrapper, result model, CLI, and legacy-only presentation were removed |
| Call-only and full-semantic package dependencies share similar names | Clear debt | NOCV self-analysis returns materially different package edges depending on command/API |
| Transitive dependents and hypothetical signature consequences formerly shared `impact` | Resolved in Task 51 | Query, CLI, output, docs, and tests now use dependents terminology for current graph state |
| Successful load can silently contain compiler diagnostics and incomplete semantic facts | Probable correctness/product debt | Missing facts have no analysis-status marker; a graph result can look authoritative |
| Source mutation, recheck, comparison, and attribution shared one file | Resolved in Task 48 | Focused same-package files now expose the distinct invariants without new abstractions |
| Signature overlay combined lookup, range selection, IO, rendering, and splicing | Resolved in Task 48 | Concrete helpers now make source mutation auditable while preserving one-file in-memory overlays |
| `app.js` holds graph, navigation, inspection, and signature-change workflows with shared state | Probable debt | A change in one workflow can disturb stale-request/navigation invariants; no framework migration is implied |
| HTTP DTOs duplicate query/analyzer fields | Intentional complexity | This protects the transport boundary and compiler privacy |
| Contract and structural analysis use `go/types` method sets and several helpers | Intentional complexity | This is the domain complexity required for correct Go behavior |
| `Analysis` retains graph plus compiler state | Uncertain | It is cohesive today; the name becomes debt only when lifecycle/status/build configuration becomes public |

## Beta boundary and priorities

No concrete semantic correctness defect was found during this inventory, and
the Go import/semantic package graphs are acyclic. The implementation can reach
`v0.beta.1` through internal refactor and contract clarification; it does not
need a new feature or architecture rewrite.

| Priority | Item | Rationale |
|---|---|---|
| Must implement/document before beta | Approved meanings of `query`, `dependency`, `dependents`, and `impact` in supported surfaces | Beta should not freeze contradictory API/CLI vocabulary |
| Completed in Task 51 | Rename transitive `impact` to `transitive-dependents` | Current reverse reliance and hypothetical change consequences now have distinct names |
| Completed in Task 50 | Coarse complete/partial analysis status | Users can distinguish known incomplete type information from a complete supported analysis |
| Completed in Task 46 | Remove the parameter-only wrapper/model/CLI | Unified callable signature analysis is now authoritative |
| Completed in Task 47 | Rename stale parameter-specific production files | Current unified responsibility is visible |
| Completed in Task 48 | Separate overlay construction from compiler diagnostic operations within `goanalyzer` | The riskiest implementation is now auditable without changing behavior |
| Should fix before beta | Replace duplicate spike algorithms with tests of production behavior | Prevent false confidence during refactor |
| Can defer after beta | Rename/reconceptualize `Analysis` | Requires lifecycle/status design; current type remains cohesive |
| Completed in Task 47 | Move Go-specific signature/change models to `goanalyzer` | Current package ownership matches the approved contract |
| Can defer after beta | Add a shared model package | No concrete responsibility currently justifies one |
| Can defer after beta | Split `goanalyzer` into packages | No current cycle or independent state boundary justifies it |
| Can defer after beta | Modularize `app.js` or change frontend technology | UI works; improve behavior tests before structural change |
| Can defer after beta | Generalize package/type projection algorithms | Duplication is understandable and deterministic |

## Candidate slow refactor sequence

Every stage begins by inspecting the then-current repository and ends with a
human review point.

1. Remove duplicated Task 41 spike algorithms while retaining behavior cases.
2. Implement the approved coarse analysis-status contract without per-edge confidence.
3. Review graph/query CLI commands against the approved glossary and remove or
   rename only one command family at a time.
4. Improve critical browser workflow tests, then consider internal JavaScript
   module/file separation without choosing a new framework.

## Decision gates and safe mechanical candidates

| Change | Gate |
|---|---|
| Move Go-specific operation DTOs from `query` to `goanalyzer` | Completed in Task 47 without aliases or a shared package |
| Rename `Analysis` or introduce project/session concepts | **Human architecture decision required** |
| Design failed/complete/partial analysis status | Designed in Task 49 and implemented in Task 50 |
| Remove `impact-params` and its wrapper | Completed in Task 46 |
| Rename transitive `impact` | Completed in Task 51 as `transitive-dependents`, including the query API |
| Rename `package-deps` or other dependency APIs | Terminology direction approved; requires separate focused review |
| Split `goanalyzer` into packages | **Human architecture decision required**; current evidence does not recommend it |
| Rename `internal/webui/parameter_impact.go` | Completed in Task 47 as `signature_impact.go` |
| Move overlay/diagnostic functions between files in the same package | Mechanical if tests remain unchanged |
| Extract declaration lookup and source-range helpers | Mechanical with focused overlay tests |
| Split CLI formatting into same-package files | Mechanical; low architectural risk |
| Remove duplicate spike helpers after mapping scenarios to production tests | Mechanical after test review |
| Update comments/tests/docs after approved terms | Mechanical |

Human-owned decisions that remain open are the exact replacement names for
existing CLI commands, any future rename of
`Analysis`, and any new package boundary. The ownership direction, terminology
meanings, legacy removal direction, and coarse-status requirement are approved.

## Refactor invariants

All later refactors must preserve, unless a separately approved behavior change
says otherwise:

- `NodeID`/`SymbolRef` identity, duplicate handling, deterministic references,
  and package nodes with no declaration location;
- parent/child hierarchy for packages, types, functions, and methods;
- exact meanings and evidence of `Calls`, `Implements`, `Embeds`, `Accepts`,
  `Returns`, and `Imports`;
- deterministic package/type projections without storing synthetic projection
  edges;
- package/type/function inspection content and symbol navigation;
- dependency explanation and exact/projected path behavior, including cycle
  termination and evidence preservation;
- direct call-site compatibility, constants, ellipsis/variadic behavior, and
  unchanged-parameter no-op behavior;
- one full proposed signature shared by call, compiler, contract, and structural
  lenses;
- compiler overlay isolation, reverse-import scope, dirty-baseline comparison,
  conservative uncertain classification, attribution, and deterministic order;
- concrete/interface contract loss including pointer and embedded-interface
  behavior;
- promoted-method structural impact including recursion, pointer-only exposure,
  shadowing, ambiguity, and duplicate suppression;
- no source mutation and no accumulated state across repeated analyses;
- web stale-response protection and navigation reuse;
- the expected `examples/impactdemo` scenarios.

## Default validation policy for this branch

For each small refactor:

1. run targeted tests for the files/package changed;
2. run `go test -count=1 ./...` from the root module;
3. run an `examples/impactdemo` sanity scenario when signature semantics,
   overlays, contracts, promotion, or diagnostics are touched;
4. run NOCV self-analysis when imports, package boundaries, model ownership, or
   dependency terminology changes;
5. avoid external-project validation unless the change specifically requires
   it.

## Approved pre-beta architecture contract

The following decisions replace the open recommendations recorded by Task 44.
They define direction, not permission to combine implementation work. Every
implementation change still receives its own scoped task and review.

### Package responsibilities

1. **`graph` is language-agnostic semantic storage.** It owns `Node`, `NodeID`,
   `SymbolRef`, `Location`, hierarchy, semantic/import edges, indexes, and
   validation. It must not depend on Go compiler objects, Go signature concepts,
   query projections, presentation DTOs, frontend state, or hypothetical-change
   analysis.
2. **`query` is language-agnostic graph interpretation.** It owns graph-derived
   navigation, projections, paths, explanations, inspections, symbol summaries,
   and other language-independent read models. Its long-term contract excludes
   Go-specific analysis models.
3. **`goanalyzer` owns Go-specific semantic analysis and models.** This includes
   `go/packages`, AST and type state, Go type expressions, call compatibility,
   callable signatures, hypothetical signature changes, method sets, promotion,
   overlays, compiler rechecks, diagnostic policy, and Go-specific impacts.
   Task 47 moved the existing Go-specific models from `query` into
   `goanalyzer`.

`query.SymbolSummary` is an explicit exception to that migration direction. It
is a detached summary of a graph symbol, is not intrinsically Go-specific, and
may remain in `query`. Later work should not duplicate it merely to remove an
import.

### Multi-language preparation

NOCV prepares for multiple languages by preserving clean language boundaries,
not by speculating about common interfaces before a second implementation
exists. `goanalyzer` remains intentionally Go-specific. A future language should
first use its best native compiler/semantic tooling; only concepts demonstrated
to be common by both implementations should be extracted.

Do not introduce placeholder concepts such as `LanguageAnalyzer`,
`LanguageBackend`, `CompilerBackend`, `SemanticProvider`, or `LanguageService`.
Do not create a `model`, `domain`, `api`, `shared`, or `common` package solely to
reverse the current `goanalyzer → query` model dependency. If the approved type
move reveals a concrete unsolved dependency, stop for human review.

### Dependency, dependents, and impact

“Dependency” is qualified when context changes its meaning. These are distinct
domain concepts, not variants forced into one universal abstraction:

- An **import dependency** means package A imports package B. Preferred terms
  are `import dependency`, `importer`, `import path`, and `import cycle`.
- A **semantic package dependency** means symbols in package A rely on symbols
  in package B through `Calls`, `Implements`, `Embeds`, `Accepts`, or `Returns`.
  It is projected from declaration-level relationships; imports remain separate.
- A **type dependency** is the corresponding semantic projection from one
  modeled type to another.
- At declaration level, use the exact relationship when known: `calls`/`called
  by`, `implements`/`implemented by`, `embeds`/`embedded by`, and `promotion` or
  `promoted method`.

Short UI headings such as “Dependencies” and “Dependents” are acceptable when
the surrounding view makes the semantic level unambiguous. Implementation and
API names should be explicit where ambiguity would otherwise remain.

Some duplicated read-model structure is acceptable when it preserves material
domain distinctions—for example, separate import, semantic package, and type
dependency models. Do not duplicate structures for cosmetic names, but prefer
explicit domain meaning over forced generic reuse. Shared mechanics may be
reused when the result remains readable.

**Impact** is reserved for consequences of a hypothetical change: call-site,
compiler, contract, structural, and aggregate signature-change impact. It asks,
“What happens if this changes?”

**Dependents** are entities that already rely on a selected package, type, or
symbol. **Direct dependents** are immediate reverse semantic relationships.
**Transitive dependents** are the recursively reachable reverse-reliance
closure, with declaration-level paths. These answer, “What depends on this?”
Task 51 enforces that distinction in the query API and CLI.

The low-level capabilities remain precise: direct and transitive dependents,
callers, implementations, dependency paths, and signature-change analysis.
Future task-oriented questions should compose those deterministic capabilities:
“Why?” composes current relationship/path/evidence queries, while “Impact?”
composes concrete hypothetical-change analyses. Neither wrapper requires a new
heuristic semantic engine or duplicate implementation path.

### Product CLI contract

A supported CLI command must answer a maintained NOCV product question.
Historical convenience alone is not a reason to preserve a command. Temporary
debug commands are acceptable only when explicitly marked, used for a focused
investigation, and removed afterwards; NOCV should not acquire a permanent
semi-supported command class.

Current dependency-oriented names map to concepts as follows:

| Command | Current concept | Contract status / later attention |
|---|---|---|
| `package-imports` | Direct import relationships from one package | Correct import terminology; keep |
| `package-importers` | Direct reverse import relationships | Correct importer terminology; keep |
| `package-deps` | Call-only package projection via `query.Dependencies` | Name is too broad; retire or explicitly rename as call-only |
| `why-package-dep` | Explanation of one direct call-only package projection | Name is too broad; align with the fate of `package-deps` |
| `direct-deps` | Direct declaration-level semantic relationships across five semantic edge kinds | Make “semantic” and the declaration level explicit if retained |
| `direct-dependents` | Direct reverse declaration-level semantic relationships | Meaning matches dependents; level/kinds remain implicit |
| `type-deps` | Direct projected type relationships across semantic edge kinds | Type dependency is approved terminology |
| `transitive-dependents` | Recursive reverse semantic closure and its declaration-level paths | Correct dependents terminology; implemented in Task 51 |
| `check-forbidden-dependency` | Semantic package dependency paths across the five semantic kinds | Qualify as semantic in API/help where ambiguity matters |

The removed `impact-params` command is not a supported hypothetical-impact
term. The unified callable signature-change workflow supersedes it.

### Legacy parameter-only workflow

Task 46 removed the parallel product workflow comprising `impact-params`,
`Analysis.AnalyzeParameterChange`, and `query.ParameterChangeImpact`. Parameter
changes now use `AnalyzeSignatureChange`, including its compiler-consequence
path. The removal covered these former references:

| Location | Current role |
|---|---|
| `cmd/nocv/commands.go` | Command registration, help, and dispatch removed |
| `cmd/nocv/parameter_impact.go` | Legacy parser and formatter file removed |
| `goanalyzer/signature_analysis.go` | Compatibility method removed in Task 46; unified machinery retained and renamed in Task 47 |
| `query/parameter_change.go` | Compatibility result removed in Task 46; the remaining Go-specific models moved and the file was removed in Task 47 |
| `README.md` | Legacy command example and description removed |
| `examples/impactdemo/README.md` | Scenarios rewritten for the unified inspector workflow |
| `cmd/nocv/parameter_impact_test.go` | Removed because it tested only the deleted CLI surface |
| `goanalyzer/parameter_change_test.go` | Valuable behavior migrated to `AnalyzeSignatureChange` |

`internal/webui/signature_impact.go` implements `/api/signature-impact` and
calls `AnalyzeSignatureChange`; Task 47 aligned its filename with that role.

### Go-specific type migration completed

Task 47 moved these types to `goanalyzer`, not to a new shared package. The
public HTTP JSON shape and analyzer behavior were preserved.

| Type(s) | Current consumers | Owner | Boundary note |
|---|---|---|---|
| `GoTypeRef` | Signature DTOs, call problems, analyzer construction, web presentation | `goanalyzer` | References `graph.SymbolRef`; no compiler object is exposed |
| `CallableSignature`, `Parameter`, `Result` | Analyzer public API/result and web node/signature presentation | `goanalyzer` | Detached presentation models remain compiler-object-free |
| `ProposedSignature`, `ProposedParameter`, `ProposedResult` | Analyzer entry point, web request conversion, analyzer tests | `goanalyzer` | Public input model uses Go type-expression strings |
| `Compatibility`, `CallSiteImpact` | Go call checker, aggregate result, web rendering and tests | `goanalyzer` | Compiler-backed Go assignability semantics; uses `query.SymbolSummary` |
| `SignatureProblem`, `SignatureProblemKind` | Go call checker and web rendering | `goanalyzer` | Moves with call-site impact and retains `GoTypeRef` dependency |
| `ContractImpact`, `ContractImpactKind` | Go method-set analysis, aggregate results, web presentation | `goanalyzer` | Uses language-agnostic `query.SymbolSummary`; that import remains intentional |
| `StructuralImpact`, `StructuralImpactKind`, `MethodExposure` | Go embedding/method-set analysis, aggregate results, web presentation | `goanalyzer` | Promotion and pointer exposure remain explicitly Go-specific |
| `ParameterChangeImpact` | Removed in Task 46 | Removed, not moved | Superseded compatibility type |

`SignatureChangeImpact`, `CompilerImpact`, `CompilerConsequence`, baseline status,
and diagnostic classification already live in `goanalyzer`, which is consistent
with this contract.

### Analysis status and failure semantics

Task 49 replaced the earlier `health` terminology and Task 50 implemented the
approved design. `Health` was rejected because it suggests a compiler/build-
quality product. Analysis status qualifies only the completeness of NOCV's
supported semantic model.

The approved conceptual outcomes are:

```text
failed                         nil Analysis plus error
succeeded, complete           usable model; no known tolerated incompleteness
succeeded, partial            usable model; known conditions may have omitted facts
```

Current type-check diagnostics remain tolerated because `go/packages` supplies
useful syntax and type state, but they can cause extraction to skip calls,
signatures, embeddings, imports, or implementations. They therefore justify a
coarse partial status. List/parse/load and graph-invariant errors remain failed
attempts. Zero-package patterns now fail instead of returning an empty analysis.

`CompilerImpact.BaselineStatus` remains distinct: it describes the affected
reverse-import scope of one hypothetical compiler recheck, not the completeness
of the initially loaded analysis.

The implemented beta shape is a stored whole-analysis complete/partial enum plus
one coarse, deterministic reason per ill-typed or type-error package.
`Package.IllTyped` is a conservative backstop, affected package paths are
included, signature-impact analysis remains available on Partial when the
callable is resolvable, and zero matched packages fail. It deliberately excludes
diagnostic messages, confidence scores, per-package status objects, and per-edge
metadata. See [`analysis-status-design.md`](analysis-status-design.md) for the
load-path inventory, experiments, scenario matrix, API, and product wording.

### `Analysis` and `goanalyzer` structure

`Analysis` keeps its current name for beta. It represents loaded Go package and
compiler state, the semantic graph, the symbol index, and operations requiring
that retained Go type universe. `GoProject`, `LoadedProject`, `Workspace`, and
`Session` remain deferred because they imply lifecycle/configuration behavior
that has not been designed.

`goanalyzer` remains one package through the first cleanup stages. Loading and
extraction, signature/call analysis, contracts, promotion, overlay construction,
compiler recheck, and diagnostic policy share one compiler/type universe.
Improve files, functions, and internal boundaries before proposing exported
packages, interfaces, or service objects. Any future package split requires a
separate human architecture decision.

### Readability contract

All pre-beta refactors optimize for code whose intent a junior developer can
reconstruct and explain six months later. This is not a mandate for verbosity.

- Prefer expressive role names such as `affectedPackages`,
  `proposedSignature`, `compilerDiagnostic`, `callable`, `packagePath`,
  `currentSignature`, `sourceContents`, and `declaration`. Short names remain
  appropriate for tiny obvious scopes and conventional indexes.
- Prefer a readable orchestrator that tells the operation's story, with focused
  helpers for lookup, validation, AST traversal, type resolution, source
  mutation, compiler loading, sorting, and presentation when separation reveals
  real intent.
- Do not fragment an understandable function into helpers that merely restate
  individual lines. A helper should represent a meaningful operation, hide
  mechanical detail, express domain intent, or isolate a non-obvious invariant.
- Name functions for intent: `resolveProposedSignature`,
  `collectCompilerDiagnostics`, `compareCompilerDiagnostics`,
  `findCallableDeclaration`, `affectedPackagePaths`, and
  `attributeDiagnostic` are the preferred style. Avoid vague `process`,
  `handleData`, `doWork`, `buildStuff`, or `helper` names without strong context.
- Prefer structure and names over comments. Comment non-obvious language rules,
  deliberate conservative behavior, surprising preserved cases, and the “why”
  behind an invariant; do not narrate straightforward loops.
- Prefer boring, explicit control flow over abstractions or generics that hide
  AST, type, signature, diagnostic, graph-projection, or impact semantics.
- Preserve real domain complexity. A focused dense helper for Go call rules,
  method sets, promotion, diagnostic matching, or AST ownership is better than
  scattering one rule across files.

Each later refactor task has one primary architecture/readability goal, limited
scope, explicit invariants, targeted tests, a full test pass, and a human review
point before the next major task. Human edits between tasks are authoritative:
inspect the current repository, adapt, and never restore an earlier generated
implementation automatically.

### Commit contract

New commits on `refactor/v0-beta-1` use:

```text
<type>: <imperative summary>
```

Allowed types are `refactor:`, `fix:`, `test:`, `docs:`, and `chore:`. Each
commit represents one coherent concern, uses a concise imperative subject,
avoids vague “cleanup”, “misc”, or “changes”, and omits package scopes unless a
later need proves them useful. Existing history is not rewritten.

### Approved decisions summary

1. `graph` is language-agnostic semantic storage.
2. `query` remains language-agnostic graph interpretation and read models.
3. Go-specific semantics and models belong in `goanalyzer`.
4. Multi-language readiness comes from clean boundaries, not speculative
   interfaces.
5. Dependency terminology is explicit by context and meaning.
6. Domain-specific read-model duplication is acceptable when it improves
   clarity.
7. Impact means consequences of a hypothetical change.
8. Dependents means entities that already rely on a selected entity.
9. Supported CLI commands represent maintained product workflows; temporary
   debug commands are disposable.
10. The parameter-only change workflow was removed in Task 46.
11. Failed/complete/partial analysis status becomes a coarse first-class beta concept.
12. `Analysis` keeps its current name for now.
13. `goanalyzer` remains one package initially; internal readability comes first.
14. Refactor code for explainability through expressive names, clear
    orchestration, focused helpers, intentional comments, and explicit control
    flow.

### Explicitly deferred decisions

- cross-language analyzer/provider interfaces;
- a universal shared model package;
- second-language frontend abstractions;
- renaming `Analysis`;
- splitting `goanalyzer` into exported packages;
- per-node or per-edge completeness, confidence, or percentages; and
- frontend framework or runtime migration.

### Future authorized refactor areas

The following areas are authorized for separate future tasks, one at a time:

- clarify dependency-oriented CLI/API names;
- review remaining dependency-oriented CLI/API names separately;
- remove duplicated Task 41 spike algorithms while preserving scenarios;
- implement the approved coarse analysis-status contract;
- improve focused implementation readability; and
- improve focused frontend tests before structural JavaScript changes.

Authorization records direction, not permission to bundle these areas. A new
shared model package, cross-language interface, `Analysis` rename,
`goanalyzer` package split, or frontend/runtime migration
still requires explicit human review.

## Human review gate after Task 50

The human should review the implemented failure boundary, complete/partial
definitions, reason representation, status ownership, CLI/web wording, and
relationship to `CompilerImpact.BaselineStatus`. Later tasks must start from the
current repository state and preserve intervening human edits. No next
implementation task is selected automatically.

## Task 52 dependency CLI review

Task 52 completed a design/inventory review without changing production code,
commands, query behavior, output, or traversal. The full command matrix and
beta vocabulary recommendation are recorded in
[`dependency-cli-terminology.md`](dependency-cli-terminology.md).

The review confirmed four materially distinct current models: direct Go import
edges, declaration-level semantic relationships, semantic package projection,
and semantic type projection. It also confirmed a legacy fifth surface:
`package-deps` and `why-package-dep` use a call-only projection rather than the
maintained semantic package view. `direct-dependents` remains one-hop reverse
semantic navigation, and Task 51's `transitive-dependents` remains the recursive
reverse closure with all distinct simple declaration-level paths. Impact stays
reserved for hypothetical-change consequences.

The recommendation for human review is to keep the precise import operations
and the direct/transitive dependent capabilities; rename terse or level-opaque
dependency/path commands; remove the global `imports` diagnostic dump and
legacy call-only `package-deps`; and decide whether the useful location evidence
in `why-package-dep` should be subsumed into package inspection/future `why` or
retained under an explicitly call-only name. No symmetric forward closure is
proposed merely for naming symmetry.

The review also records traversal readability debt separately. Five query
algorithms currently use inline recursive `walk` closures. A future task should
extract named query-specific traversal helpers first and consider reuse only
after their different direction, projection, evidence, and duplicate rules are
plain. Terminology changes must not be bundled with that refactor.

## Human review gate after Task 52

Before a follow-up implementation task is written, the human should review the
command semantics matrix, the proposed beta vocabulary, removal of the global
and call-only legacy surfaces, the fate of `why-package-dep`, and the continued
separation between future `why` composition and hypothetical `impact` analysis.
