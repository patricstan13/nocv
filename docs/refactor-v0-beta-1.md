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
| `query` | Direct navigation, import and semantic paths, package/type projections, inspections, explanations, rules, transitive impact, shared signature/impact DTOs | `graph` | `goanalyzer`, `internal/webui`, `cmd/nocv` | Graph-derived projections and read models are cohesive | `GoTypeRef`, callable/proposed signatures, and call compatibility are Go-specific and live here mainly to provide detached cross-package DTOs; `ParameterChangeImpact` is compatibility history |
| `goanalyzer` | Go loading, graph extraction, symbol index, compiler/type state, call-site analysis, signature resolution, contracts, promotion, source overlay, compiler recheck, diagnostic comparison and attribution | `graph`, `query`, `go/packages`, compiler packages | `internal/webui`, `cmd/nocv` | These operations share one loaded Go type universe and symbol index | The package has distinct internal areas; the legacy parameter-only entry point duplicates unified orchestration. No package split is yet justified solely by size |
| `internal/webui` | HTTP server, JSON presentation DTOs, embedded assets, package graph, inspector/navigation state, signature-change editor and results | `goanalyzer`, `graph`, `query` | `cmd/nocv` | It is a coherent internal adapter from analysis/read models to one UI | `app.js` carries several workflows in one mutable module; `parameter_impact.go` is a stale filename after unified signature support |
| `cmd/nocv` | Command parsing, project loading orchestration, query invocation, text formatting, web serving, diagnostic/debug workflows | `goanalyzer`, `graph`, `query`, `internal/webui` | Executable only | It is the composition root, so high fan-out is expected | `impact-params` and its formatter preserve the old parameter-only model; command names expose overlapping meanings of dependency and impact |
| `examples/impactdemo` | Small, separate Go module demonstrating call, result, contract, embedding, compiler, and dirty-baseline scenarios | Its own standard-library/local packages; no NOCV production dependency | Manual validation only | Purpose-built behavior fixture | Its README is manually maintained and can drift from analyzer behavior; that is a test-documentation risk, not a package ownership problem |

`graph` remains the stable lower layer. `query` is the main architectural
decision point: most of it is language-independent graph interpretation, while
one file now defines Go-specific signature data consumed and populated by
`goanalyzer`.

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
projects calls only. `inspect-dependency nocv/goanalyzer nocv/query` explained
the edge with exact type evidence:

- `Analysis` returns `CallSiteImpact` from `checkCallSite`.
- `Analysis` returns `CallableSignature` from `CallableSignature` and
  `editableSignature`, and accepts it in signature resolution.
- `Analysis` returns `GoTypeRef` from `goTypeRef`.
- `Analysis` returns `ParameterChangeImpact` from `AnalyzeParameterChange`.
- `Analysis` accepts `ProposedSignature` in both public change analyzers.
- `Analysis` returns `SymbolSummary` from diagnostic attribution and summary
  helpers.
- `argumentCountProblem` returns `SignatureProblem`.

This is an ownership smell visible semantically but not as an import cycle: a
Go implementation package depends upward on a package otherwise described as
graph querying because shared detached Go DTOs live there.

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
| `CallableSignature` | `query` | `goanalyzer`, webui, CLI | Go-specific detached domain/presentation model | Questionable: useful across packages, but not a graph query concept |
| `ProposedSignature` | `query` | `goanalyzer`, webui request conversion, CLI | Go-specific command/input model (`TypeExpr`) | Questionable for the same reason; explicitly Go syntax-aware |
| `ParameterChangeImpact` | `query` | legacy analyzer API and `impact-params` | Go-specific historical compatibility result | Incorrect as a long-term model; remove with the legacy surface after approval |
| `SignatureChangeImpact` | `goanalyzer` | webui presentation | Go-specific aggregate analysis result | Correct today because it includes compiler recheck results |
| `CallSiteImpact` | `query` | both analyzer entry points, CLI/web | Go-specific semantic result | Boundary decision required; it is compiler-backed but not a compiler diagnostic |
| `ContractImpact` | `query` | analyzer, CLI/web | Mostly generic wording, but implemented with Go method sets and Go symbol semantics | Shared use is legitimate; ownership is still coupled to the Go signature model |
| `StructuralImpact` | `query` | analyzer, CLI/web | Domain consequence whose current only meaning is Go method promotion | Useful detached result, but currently Go-specific in practice |
| `CompilerImpact` | `goanalyzer` | `SignatureChangeImpact`, webui | Go compiler consequence aggregate | Correct |
| `CompilerConsequence` | `goanalyzer` | webui | Go diagnostic delta with optional symbol attribution | Correct |
| `GoTypeRef` | `query` | signature, problems, web/CLI | Explicitly Go-specific detached type presentation | Misplaced conceptually in a general query package, though it prevents compiler-object leakage |

No compiler objects escape `goanalyzer`. That invariant matters more than
achieving abstract package purity. Moving these DTOs is optional unless the
intended public meaning of `query` excludes language-specific analysis models.

## The `query` / `goanalyzer` boundary

The concepts crossing `goanalyzer → query` fall into five groups:

| Category | Current examples | Assessment |
|---|---|---|
| Graph-derived read models | `SymbolSummary` | Genuinely belongs near inspection/query code and is reusable |
| Generic semantic concepts | `ContractImpact`, `StructuralImpact`, `MethodExposure` | Detached domain results are useful to multiple presenters, but their only current semantics are Go contracts and promotion |
| Go signature/type concepts | `GoTypeRef`, `CallableSignature`, `ProposedSignature`, `Parameter`, `Result`, `Compatibility`, `SignatureProblem`, `CallSiteImpact` | Clearly Go-specific; they are in `query` mainly as a neutral DTO location |
| Go compiler consequences | `CompilerImpact`, `CompilerConsequence`, baseline and diagnostic classification | Correctly remain in `goanalyzer` |
| Historical compatibility | `ParameterChangeImpact` | Exists for `impact-params`; should not shape the beta API |

The current split—shared DTOs in `query`, compiler deltas in `goanalyzer`—is
pragmatic and avoids an import cycle. It is not conceptually crisp. Realistic
future choices are to accept `query` as the home of all public read/command
models, move the complete signature-change API into `goanalyzer`, or introduce
a deliberately named model package. The third option adds a package and should
not be chosen merely to reverse one arrow.

## Signature-change ownership and flow

`goanalyzer.Analysis.AnalyzeSignatureChange` is the orchestration boundary. It
resolves one callable and one proposal once, then uses the same resolved full
signature for all four lenses:

| Lens/model | Owner | Responsibility |
|---|---|---|
| `CallableSignature`, `ProposedSignature` | `query` model; populated/resolved by `goanalyzer` | Detached before/after input and display |
| `CallSiteImpact` | `query` model; computed by `goanalyzer` | Direct source-call compatibility for parameter changes |
| `CompilerImpact` | `goanalyzer` | Overlay compiler consequences across reverse import closure |
| `ContractImpact` | `query` model; computed by `goanalyzer` | Existing interface contracts lost under the full signature |
| `StructuralImpact` | `query` model; computed by `goanalyzer` | Existing promoted method surfaces changed under the full signature |
| `SignatureChangeImpact` | `goanalyzer` | Aggregate result preserving the four distinct lenses |

The aggregate correctly belongs beside compiler state today. The questionable
part is not behavior but why some of its children are in `query`. Do not move
individual children before agreeing whether `query` means “all detached read
models” or “language-independent graph queries.”

## Transitional compatibility inventory

| Item | Classification | Evidence and reason |
|---|---|---|
| `Analysis.AnalyzeParameterChange` | Remove before beta, with human confirmation | It repeats resolution, analysis, and sorting instead of delegating to the unified entry point, and intentionally omits compiler consequences |
| `query.ParameterChangeImpact` | Remove before beta | Its only production purpose is the legacy CLI result shape; new surfaces use `SignatureChangeImpact` |
| `impact-params` CLI | Remove or replace before beta—human decision required | It exposes an older product concept and prevents removal of the wrapper/model; it may still be useful as a debug command |
| `query/parameter_change.go` | Rename before beta after ownership decision | It now contains full parameter/result signature and three impact categories; a mechanical rename first would hide the ownership question |
| `goanalyzer/parameter_change.go` | Rename/split within package after ownership decision | It now owns unified signature resolution and call-site analysis as well as the legacy wrapper |
| `internal/webui/parameter_impact.go` | Rename before beta | Endpoint and content are already unified signature impact; filename is stale |
| `cmd/nocv/parameter_impact.go` | Keep temporarily | Correctly matches `impact-params`; remove or rename together with that command |
| Parameter-only analyzer tests | Keep intentionally, then rename where needed | They still document valid call-site, contract, promotion, constant, and variadic behavior even after the wrapper disappears |
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
| Impact | Modeled consequence of a hypothetical change, or currently a transitive dependent query | Any dependency | Overloaded: rename the transitive `impact` command before beta or qualify both uses |
| Consequence | Compiler diagnostic attributable to the hypothetical overlay | Every structured call/contract/structural impact | Keep “compiler consequence” as a qualified term |
| Evidence | Exact relationship/location supporting a derived statement | An inference without a graph fact | Keep |
| Owner | Enclosing package/type used for projection | Go package ownership, receiver identity, or architectural code owner | Prefer `enclosing type/package` unless projection ownership is meant |
| Contract | Existing concrete-type/interface implementation relation | API promise in the broad product sense | Use `interface contract` in prose |
| Structural | Effective concrete method-surface change through embedding/promotion | Generic dependency blast radius | Keep qualified as `promoted-method structural impact` where space permits |
| Compiler | Result produced by Go loading/type checking | Every `go/types`-backed structured check | Reserve “compiler consequence” for overlay diagnostic delta |
| Baseline | Diagnostics from the unmodified loaded source in the affected recheck scope | Proof the entire repository is healthy | Always include scope |
| Overlay | In-memory replacement source supplied to `go/packages` | Disk mutation or general virtual workspace | Keep |
| Signature | Callable parameters, results, and variadic state in this feature | Receiver or type parameters | State this scope in public docs |
| Callable | Modeled package function or method, including interface method where supported | A type, function literal node, or arbitrary function value | Keep |
| Analysis | Retained loaded Go packages, graph, indexes, and operations | Immutable result value or compile-clean guarantee | Current name is broad; decide before renaming |

## Concrete vocabulary ambiguities

| Term | Current examples | Why confusing | Possible replacement | Before beta? |
|---|---|---|---|---|
| Relationship | `graph.Edge` versus `query.Relationship` | Both represent the same fact at storage/read boundaries | Keep both, document `edge` as storage and `relationship` as detached view | Documentation before beta; no rename required |
| Dependency | `Dependencies` is call-only; package/type dependency views include five semantic kinds; imports have separate “dependency” rules | The same label selects different edge sets | `CallDependencies` for the old function/command, or retire it in favor of semantic package dependency | Yes, public CLI decision |
| Impact | `query.Impact` is transitive semantic dependents; signature impact is hypothetical change analysis | One word names two unrelated workflows | `semantic dependents`/`dependents-paths` for the former; `signature impact` for the latter | Yes for user-facing command names |
| Direct | Direct exact edges and direct projected package/type edges | A projected direct step can summarize many exact facts | Qualify the level in names and headings | Documentation now; code rename can wait |
| Exact | Exact semantic paths | Sounds like a confidence claim | `declaration-level` in prose; retain internal names if tests are clear | Can defer |
| Analysis | `Analysis` stores a live loaded project; methods also return individual analyses | Value and service/session meanings collide | `LoadedProject` or `GoProject` are candidates, not decisions | Human decision; can defer if documented |
| Inspection | Node and package-dependency inspections plus UI inspector state | Domain read model and screen state share the noun | Qualify as `NodeInspection`, `PackageDependencyInspection`, `inspector view` | No urgent rename |
| Signature | Parameter-only legacy files beside full parameter/result implementation | Old names imply narrower scope | Rename files/functions after compatibility decision | Yes for stale public names |
| Owner | `typeOwner`/package projection and ordinary declaration containment | “Owner” hides which hierarchy is being projected | `enclosingType`, `packageOf` where touched | Can defer |

## File naming review

| File | Action | Reason |
|---|---|---|
| `query/parameter_change.go` | Rename or relocate only after model-ownership decision | Responsibility is now full signature DTOs and impacts, not parameter-only analysis |
| `goanalyzer/parameter_change.go` | Split/rename within package after legacy removal | Contains signature extraction/resolution, direct call checking, and legacy/unified orchestration |
| `goanalyzer/signature_change.go` | Keep name; consider internal file split | It accurately names the unified feature, but overlay and diagnostics are separate cohesive concepts |
| `goanalyzer/contract_impact.go` | Keep | Name matches focused responsibility |
| `goanalyzer/structural_impact.go` | Keep | Name matches focused responsibility |
| `internal/webui/parameter_impact.go` | Rename mechanically to `signature_impact.go` | Content and endpoint are already signature-wide |
| `cmd/nocv/parameter_impact.go` | Keep temporarily, then remove/rename with command | It accurately implements the legacy `impact-params` surface |

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
need lifecycle, health, reload, or multiple build configurations. Possible
future names such as `LoadedProject` may describe it better, but renaming now
would imply a lifecycle design that has not been chosen.

## Loading and partial-project semantics

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
valid but incomplete graph without an explicit marker.

There is no project-wide analysis-health model. `CompilerImpact.BaselineStatus`
only describes diagnostics in the affected reverse-import recheck scope for one
hypothetical signature analysis. Other consumers cannot distinguish a clean
load from a load with ignored type diagnostics, nor identify which graph facts
may be incomplete.

Consequences:

- absence of an edge can be mistaken for proof of no relationship;
- inspection and projection results do not carry a completeness caveat;
- baseline health becomes visible only when signature analysis runs;
- reloading uses the current active environment rather than an explicitly
  persisted build configuration object.

For beta, this needs an explicit product decision and documentation. A new
health API is not automatically required if beta clearly labels dirty/partial
analysis limitations, but silently implying completeness is unsafe.

## Signature overlay and compiler architecture

| Step | Current function/file | Responsibility and dependencies | Cohesion |
|---|---|---|---|
| Resolve declaration | `signatureOverlay`, `goanalyzer/signature_change.go` | Match `types.Func`, find `ast.FuncDecl`; fallback AST search for interface method `FuncType` | Correct operation but buried inside a larger source-mutation function |
| Select source range | `signatureOverlay` | Choose parameter opening through result end using token positions | Coupled appropriately to mutation, worth a named helper for readability |
| Render signature | `renderSourceSignature` and helpers | Preserve/reconstruct names, tuple form, variadic syntax, parameter/result type source | Cohesive source-rendering operation |
| Create overlay | `signatureOverlay` | Read the source file, splice bytes in memory, return filename/content | Cohesive outcome; does not mutate disk |
| Reverse-import closure | `affectedPackagePaths` | Build reverse imports from loaded packages and BFS from changed package | Cohesive and deterministic |
| Baseline diagnostics | `collectCompilerDiagnostics` | Gather diagnostics for allowed affected packages | Cohesive |
| Overlay load | `compilerImpact` | Call `packages.Load` with retained directory/mode and one overlay | Reasonable orchestration |
| Overlay diagnostics | `collectCompilerDiagnostics` | Gather diagnostics from rechecked packages | Cohesive |
| Compare diagnostics | `compareCompilerDiagnostics` and helpers | Count-sensitive exact match; same package + normalized message becomes uncertain; unmatched becomes new | Cohesive, policy-bearing operation |
| Attribute symbols | `Analysis.attributeDiagnostic` | Parse location, find enclosing function/method AST, map/build `SymbolSummary`; leave package-level diagnostics unattributed | Cohesive but dependent on loaded AST and symbol conventions |

These are five distinct concepts: hypothetical source mutation, compiler
recheck, diagnostic collection, diagnostic comparison, and attribution. They
can remain private operations in `goanalyzer`; no new framework or package is
warranted. Separating overlay construction from diagnostic policy into
same-package files would make invariants easier to review. `compilerImpact`
should remain the small coordinator.

## Implementation hotspots

Line count was used only to find candidates; the reasons below are conceptual.

| Area/function | Responsibilities mixed | Why reading is difficult | Plausible extraction | Value now |
|---|---|---|---|---|
| `addPackage`, `goanalyzer/analyzer.go` | Ordered files/docs, package/type/function nodes, identities, compiler-object index | Central extraction path must preserve many identity and documentation invariants | Keep coordinator; named helpers by declaration kind only when touched | Probable debt, not urgent |
| `addCalls`, `goanalyzer/analyzer.go` | AST traversal, lexical function-literal ownership, object lookup, call edge/evidence creation | Closure attribution rules are embedded in traversal state | Named call-attribution traversal helper | Useful only with focused tests; defer |
| `AnalyzeSignatureChange`, `goanalyzer/parameter_change.go` | Resolve, validate, compare, collect four impact lenses, sort | Orchestration is broad but linear and expresses product semantics | Keep as coordinator; extract shared result sorting | Mostly intentional complexity |
| `AnalyzeParameterChange`, same file | Repeats the above without compiler impact | Duplicate workflow can diverge | Remove or delegate after CLI decision | Clear debt |
| `resolveProposedSignature`, same file | Parse type expressions, declaration-context resolution, validate variadic shape, construct compiler/model signatures | Compiler and detached DTO construction are interleaved | Separate type-expression resolution from model assembly if ownership changes | Probable debt |
| `checkCallSite`, same file | Argument extraction, count/ellipsis rules, compiler assignability/constants, problems DTO | Dense because Go call semantics are genuinely detailed | Keep focused helpers already present; no package split | Intentional complexity |
| `signatureOverlay`, `signature_change.go` | Declaration lookup, range selection, file IO, render, byte splicing | Several failure modes and invariants live in one function | `findCallableDeclaration`, `signatureSourceRange`, overlay assembly | Worth extracting mechanically |
| `compilerImpact`, same file | Scope, baseline, overlay, reload, compare, attribute, sort | Coordinator spans separate concepts but is currently readable | Move operations to focused same-package files; keep coordinator | Probable debt |
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
- AST callbacks in `addCalls`, `collectCallSites`, and declaration fallback in
  `signatureOverlay` encode meaningful attribution rules. Naming the latter
  declaration lookup would improve reviewability; mechanically extracting every
  `ast.Inspect` callback would not.
- Browser event callbacks are expected. The problem is workflow state shared
  across graph, inspection, and signature analysis, not the mere presence of
  closures.

## Duplication inventory

| Duplication | Classification | Reason |
|---|---|---|
| Unified and legacy analyzer orchestration/sorting | Dangerous divergence | The legacy path already omits compiler impact and does not share the unified no-op path |
| Production overlay algorithms and Task 41 spike helpers in tests | Worth consolidating | Two implementations can make tests pass while production changes independently |
| Package/type projected dependency builders | Harmless for now | Similar mechanics but different ownership and endpoint rules; a generic abstraction may obscure semantics |
| `query.symbolSummary` and analyzer summary construction | Worth consolidating after ownership decision | Same presentation concept is built from different source domains; premature consolidation could create the wrong dependency |
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
| `POST /api/signature-impact` | Go-specific JSON proposal converted to `query.ProposedSignature`; response presents `goanalyzer.SignatureChangeImpact` | Correctly delegates semantics; shape is a beta API decision, not a general graph endpoint |

The HTTP DTO duplication is intentional boundary code. Temporary artifacts are
the stale `parameter_impact.go` filename and source-string UI tests. JavaScript
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
| `impact` | Rename before stable user documentation | Means transitive semantic dependents, not hypothetical change impact |
| `impact-params` | Remove or retain explicitly as debug-only—human decision | Sole user of legacy parameter-only analyzer result |

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

The package is mostly a coherent library of graph-derived navigation,
projections, paths, rules, explanations, and inspections. It also owns generic
semantic read models. `parameter_change.go` adds a second role: detached
Go-specific command/result models for analyzer operations. That mix is the main
ownership decision; there is no evidence for splitting all query files.

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
| Go-specific signature/type DTOs are owned by otherwise graph-oriented `query`, creating `goanalyzer → query` for models rather than queries | Probable debt | The boundary is conceptually unclear, but detached shared DTOs are genuinely needed and no cycle results |
| `AnalyzeParameterChange` duplicates unified orchestration and preserves a result without compiler consequences | Clear debt | It can diverge semantically and already represents a superseded product surface |
| Call-only and full-semantic package dependencies share similar names | Clear debt | NOCV self-analysis returns materially different package edges depending on command/API |
| `impact` names both transitive semantic dependents and hypothetical signature consequences | Clear debt | User-facing workflows are unrelated |
| Successful load can silently contain compiler diagnostics and incomplete semantic facts | Probable correctness/product debt | Missing facts have no health/completeness marker; a graph result can look authoritative |
| Source mutation, recheck, comparison, and attribution share one file | Probable debt | Concepts have separate invariants, though current functions are mostly isolated |
| `signatureOverlay` combines lookup, range selection, IO, rendering, and splicing | Clear readability debt | Focused extraction would make source-mutation safety easier to audit without changing architecture |
| `app.js` holds graph, navigation, inspection, and signature-change workflows with shared state | Probable debt | A change in one workflow can disturb stale-request/navigation invariants; no framework migration is implied |
| HTTP DTOs duplicate query/analyzer fields | Intentional complexity | This protects the transport boundary and compiler privacy |
| Contract and structural analysis use `go/types` method sets and several helpers | Intentional complexity | This is the domain complexity required for correct Go behavior |
| `Analysis` retains graph plus compiler state | Uncertain | It is cohesive today; the name becomes debt only when lifecycle/health/build configuration becomes public |

## Beta boundary and priorities

No concrete semantic correctness defect was found during this inventory, and
the Go import/semantic package graphs are acyclic. The implementation can reach
`v0.beta.1` through internal refactor and contract clarification; it does not
need a new feature or architecture rewrite.

| Priority | Item | Rationale |
|---|---|---|
| Must decide/document before beta | Meaning of `query`, `dependency`, and `impact` in the supported surface | Beta should not freeze contradictory API/CLI vocabulary |
| Must decide/document before beta | What successful analysis says about dirty/partial projects | Users must not infer compile cleanliness or complete semantic extraction |
| Should fix before beta | Remove or explicitly quarantine the parameter-only wrapper/model/CLI | Avoid freezing duplicate semantics |
| Should fix before beta | Rename stale parameter-specific files after the compatibility decision | Make current unified responsibility visible |
| Should fix before beta | Separate overlay construction from compiler diagnostic operations within `goanalyzer` | Improves auditability of the riskiest implementation without changing behavior |
| Should fix before beta | Replace duplicate spike algorithms with tests of production behavior | Prevent false confidence during refactor |
| Can defer after beta | Rename/reconceptualize `Analysis` | Requires lifecycle/health design; current type remains cohesive |
| Can defer after beta | Move signature DTO ownership or add a model package | Current boundary works; change only after explicit architectural decision |
| Can defer after beta | Split `goanalyzer` into packages | No current cycle or independent state boundary justifies it |
| Can defer after beta | Modularize `app.js` or change frontend technology | UI works; improve behavior tests before structural change |
| Can defer after beta | Generalize package/type projection algorithms | Duplication is understandable and deterministic |

## Candidate slow refactor sequence

Every stage begins by inspecting the then-current repository and ends with a
human review point.

1. Approve glossary and supported CLI/API terminology. Documentation/naming
   decision only; preserve behavior.
2. Decide the fate of `impact-params`; then remove or quarantine the legacy
   wrapper and `ParameterChangeImpact` in one focused change.
3. Rename stale parameter-specific files and tests within their existing
   packages. No type moves.
4. Extract declaration/range/overlay assembly from compiler recheck operations
   within `goanalyzer`; preserve all signature tests and public APIs.
5. Remove duplicated Task 41 spike algorithms while retaining behavior cases.
6. Decide whether Go signature/impact DTOs remain in `query`, move together to
   `goanalyzer`, or justify a dedicated model boundary. Move nothing before the
   decision.
7. Decide and document partial-analysis health semantics; implementation, if
   any, is a separate task.
8. Review graph/query CLI commands against the approved glossary and remove or
   rename only one command family at a time.
9. Improve critical browser workflow tests, then consider internal JavaScript
   module/file separation without choosing a new framework.

## Decision gates and safe mechanical candidates

| Change | Gate |
|---|---|
| Define whether `query` includes Go-specific operation DTOs | **Human architecture decision required** |
| Rename `Analysis` or introduce project/session concepts | **Human architecture decision required** |
| Treat dirty/partial health as first-class public state | **Human product/architecture decision required** |
| Remove `impact-params` and its wrapper | **Human public-surface confirmation required** |
| Rename `impact`, `package-deps`, or dependency APIs | **Human terminology/public-surface decision required** |
| Split `goanalyzer` into packages | **Human architecture decision required**; current evidence does not recommend it |
| Rename `internal/webui/parameter_impact.go` after glossary approval | Mechanical; Codex can proceed |
| Move overlay/diagnostic functions between files in the same package | Mechanical if tests remain unchanged |
| Extract declaration lookup and source-range helpers | Mechanical with focused overlay tests |
| Split CLI formatting into same-package files | Mechanical; low architectural risk |
| Remove duplicate spike helpers after mapping scenarios to production tests | Mechanical after test review |
| Update comments/tests/docs after approved terms | Mechanical |

Human-owned decisions should remain limited to the actual open questions:

- the intended semantic scope of `query`;
- public meanings and names for dependency and impact workflows;
- whether the legacy parameter-only CLI is supported or debug-only/deleted;
- whether analysis health/partial completeness is first-class for beta;
- whether `Analysis` is a suitable long-term name;
- whether a future package boundary inside `goanalyzer` is valuable (current
  evidence supports internal file boundaries first).

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

## Decisions for human review

### 1. What does `query` own?

**Current state:** it owns both language-independent graph queries/read models
and detached Go signature/impact DTOs.

**Concrete problem:** `goanalyzer` imports `query` mainly to exchange models it
itself computes, making package intent unclear.

**Options:**

1. Define `query` as the home of all detached NOCV command/read models. Minimal
   churn, but the name no longer implies language independence.
2. Move the complete Go signature-change model into `goanalyzer`. Strong
   cohesion and no new package, but web/CLI depend more directly on analyzer and
   generic `SymbolSummary` needs careful treatment.
3. Introduce a deliberately named shared model package. Clear dependency
   direction, but adds a package whose value may be only taxonomic.

**Recommendation:** choose option 2 if signature change remains Go-specific for
beta; keep graph-derived `SymbolSummary` in `query` or replace it with a narrowly
agreed shared summary. Do not add a model package without another independent
consumer pressure.

### 2. Does the parameter-only compatibility surface survive beta?

**Current state:** `impact-params`, `AnalyzeParameterChange`, and
`ParameterChangeImpact` duplicate part of unified signature analysis.

**Concrete problem:** the older path can disagree with the product path and
prevents terminology/file cleanup.

**Options:** remove all three; keep the CLI as explicitly unsupported debug
surface delegating to unified analysis; or support both as public APIs.

**Recommendation:** remove them before beta unless the human actively uses the
CLI for debugging. If retained, delegate to unified analysis and label the
command debug-only rather than maintaining a second semantic path.

### 3. What are the supported meanings of dependency and impact?

**Current state:** call-only and five-edge semantic projections both use
“package dependency”; `impact` means both transitive dependents and hypothetical
signature consequences.

**Concrete problem:** commands with similar names return different graphs, and
beta documentation cannot explain them without implementation history.

**Options:** qualify every view (`call dependency`, `semantic dependency`,
`import`); make the newer semantic projection canonical and retire call-only
commands; or retain all commands under a documented debug namespace/convention.

**Recommendation:** make semantic package/type dependency the product meaning,
keep import relationships explicitly separate, and rename or retire call-only
and transitive-impact commands before advertising the beta CLI.

### 4. Is dirty/partial analysis health first-class in beta?

**Current state:** loading tolerates type diagnostics, graph extraction may skip
unavailable facts, and only signature recheck exposes affected-scope baseline
status.

**Concrete problem:** successful load can look complete and clean when it is
neither.

**Options:** documentation-only limitation for beta; expose a coarse analysis
health/baseline summary; or attach completeness metadata to individual facts.

**Recommendation:** choose the coarse health summary if UI users will analyze
dirty repositories during beta. Per-fact completeness is disproportionate now.
At minimum, explicitly document that successful load is not compile-clean proof.

### 5. Should `Analysis` be renamed now?

**Current state:** it is a retained loaded-project/compiler snapshot with query
operations.

**Concrete problem:** “analysis” sounds like an immutable result and says
nothing about lifecycle or partial health.

**Options:** keep and document it; rename to `LoadedProject`/`GoProject`; or wait
until health/reload/build-configuration semantics are designed.

**Recommendation:** keep it for beta and revisit after the health decision. A
rename now would promise a project/workspace abstraction that does not yet
exist.

### Recommended next task

Approve and document the glossary/public command terminology—especially the
canonical meanings of **dependency** and **impact**—without renaming code or
changing behavior. This is small, reviewable, and unblocks the compatibility,
file-naming, and ownership tasks without surrendering architectural control.
