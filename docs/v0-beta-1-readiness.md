# v0.beta.1 readiness audit

## Executive summary

NOCV is ready to tag `v0.beta.1`. The audit found no correctness blocker, no
misleading core semantic that must be fixed before the tag, and no maintained
test failure. The formatter, full suite, focused suites, canonical fixture,
race-enabled suite, documented CLI examples, generated Partial/failed cases,
web explorer, signature-change workflow, and self-analysis all passed.

The beta thesis is coherent: NOCV loads a Go project with compiler state,
builds a deterministic semantic graph, answers exact and projected structural
questions with evidence, and simulates parameter/result signature changes
without modifying source. Its answers are materially richer than grep or an
import dump because they preserve declaration identity and combine calls,
interface satisfaction, embedding, signature relationships, imports, compiler
diagnostic deltas, and source evidence.

Five non-blocking improvements would make the beta easier to maintain or
explain. Twelve explicitly deferred items remain outside the beta boundary.

## Beta definition

`v0.beta.1` is a Go-only, compiler-backed semantic analysis tool with:

- a graph of represented packages, structs, interfaces, functions, concrete
  methods, and interface methods;
- exact `Calls`, `Implements`, `Embeds`, `Accepts`, and `Returns`
  relationships, plus separate represented-package `Imports` relationships;
- deterministic node, package, and type navigation, paths, evidence, and
  architecture checks;
- complete/partial/failed analysis semantics for the active loaded Go build
  configuration;
- in-memory hypothetical callable parameter/result signature analysis with
  structured call-site, compiler, contract, and promoted-method consequences;
- a scoped CLI and a basic read-only web explorer with a signature-change
  workflow.

The beta does not promise refactoring, source mutation, behavioral correctness,
all Go declarations, all build configurations, test variants, or another
language.

## Readiness matrix

| Area | Status | Evidence | Issues | Classification |
|---|---|---|---|---|
| Project loading | ready | Complete, Partial, syntax-failure, invalid-path, and zero-package checks passed | None | ready |
| Graph identity and hierarchy | ready | Deterministic IDs, stable refs, repeated `init`, blank declarations, parent/child, and detached-copy tests passed | Unsupported declaration kinds remain intentional | ready |
| Relationship extraction | ready | Calls, implementations, embeddings, signatures, imports, lexical closures, and documentation tests passed | Dynamic function values and modeled-out declarations remain unsupported | ready |
| Query layer | ready | Exact navigation, all simple paths, reverse closure, package/type views, inspection, rules, and cycles passed focused and race tests | All-simple-path output can become large | ready with minor debt |
| Package/type projections | ready | Same-owner suppression, exact evidence aggregation, import exclusion, package-function handling, and ownership tests passed | None | ready |
| Analysis lifecycle | ready | Status reasons are deduplicated/sorted; Partial stays usable; zero packages fail | Overlay recheck does not propagate caller cancellation | ready with minor debt |
| Signature-change analysis | ready | Parameter, result, combined, no-op, variadic, contracts, promotion, dirty baseline, reverse-import closure, attribution, and source isolation passed | Explicit generic-rejection regression coverage is thin | ready with minor debt |
| Compiler consequence wording | ready | README and UI distinguish compiler diagnostics from behavioral correctness; uncertain diagnostics are explicit | No-op UI wording can more directly say no recheck was needed | ready with minor debt |
| CLI namespace | ready | Scoped parsing/help, removed aliases, deterministic rendering, and all README examples passed | One README example produces very large output on NOCV itself | ready with minor debt |
| Web explorer/API | ready | Status, graph, package/symbol/dependency navigation, signature editor/result, stale guards, and API errors passed; manual smoke passed | `app.js` remains a large shared-state file | ready with minor debt |
| Fixtures/tests | ready | One checked-in Go project; malformed/Partial cases generated; two overlay snapshots are referenced; full and focused suites passed | Small negative-path gaps are listed below | ready with minor debt |
| Documentation | ready | Current README commands and semantics matched runtime behavior; historical design records are clearly historical | Prerequisite and first-run ergonomics can improve | ready with minor debt |
| Repository hygiene | ready | Clean tracked tree, no tracked revision patches or generated binaries, tidy diff empty, vet clean | Preserved revision patches are intentionally untracked | ready |
| Concurrency | ready | `go test -race -count=1 ./...` passed | Graph mutability is an application-level convention, not concurrent mutation support | ready |

## Core workflow validation

The product spine is present and regression-protected:

| Step | Entry point | Evidence | Limitation | Blocker? |
|---|---|---|---|---|
| Load | `goanalyzer.LoadAnalysis` | Full/focused tests and complete/partial/failed manual cases | Active build environment, `Tests: false` | No |
| Semantic model | `graph.Graph` populated by `goanalyzer` | Analyzer, graph, identity, documentation, import, and relationship tests | Selected node and relationship kinds only | No |
| Query | `query` exact/package/type read models | Focused query suite, CLI fixture smoke, self-analysis | All simple paths may be voluminous | No |
| Simulate change | `Analysis.AnalyzeSignatureChange` | Focused signature tests and `/api/signature-impact` manual result | Parameters/results/variadic only | No |
| Explain consequences | CLI evidence and web inspector | Manual dependency and signature-impact inspection | Basic presentation; no automated rewrite | No |

The three MVP goals are met:

1. The semantic model has stable public identity and relationship contracts.
2. Users can verify current dependencies and hypothetical consequences against
   exact symbols and source locations.
3. Complete/Partial/failed status and conservative compiler-diagnostic
   classification prevent absence or clean recheck output from being
   overstated.

## Repository and API review

Package direction remains understandable:

```text
graph
  <- query
  <- goanalyzer
       -> query only for the approved detached SymbolSummary read model
  <- cmd/nocv and internal/webui as composition/presentation layers
```

- `graph` owns opaque graph-local `NodeID`, stable textual `SymbolRef`, nodes,
  edges, evidence, hierarchy, and mutation invariants. It contains no AST or
  `go/types` object.
- `query` owns language-independent deterministic navigation and detached read
  models. Query operations did not mutate the graph in tests or inspection.
- `goanalyzer` owns package loading, compiler objects, Go-specific signature
  and consequence models, overlays, rechecks, contracts, and promotion.
- `cmd/nocv` and `internal/webui` compose analysis and query operations and do
  not reproduce compiler semantics.

The public APIs are proportionate to the product. `graph.Graph` exposes its
intentional construction and read operations; `query` exposes named
capabilities rather than a generic traversal framework; `goanalyzer.Analysis`
is the single retained compiler-state boundary. `Load` remains a useful graph
convenience while `LoadAnalysis` is authoritative for lifecycle and change
analysis. No compatibility alias from the removed parameter-only workflow was
found.

`Analysis.Graph()` returns the retained mutable graph pointer. Current CLI and
web consumers treat it as immutable after loading, and the race suite passed.
The beta does not claim that callers may concurrently mutate it.

## Graph and relationship review

Node identity, hierarchy, and source semantics are consistent:

- `NodeID` is graph-local and never crosses CLI/web boundaries.
- `SymbolRef` is deterministic; duplicate source names receive source-order
  suffixes.
- packages have zero declaration location, while concrete declarations retain
  locations.
- aliases are normalized where supported and are not separate structural
  declarations.
- methods on modeled-out non-struct named types and other unsupported named
  declarations are deliberately omitted rather than assigned a false parent.

The relationship contract remains:

```text
semantic: Calls, Implements, Embeds, Accepts, Returns
Go import graph: Imports
```

Exact navigation traverses only the five semantic kinds. Package and type views
derive from those same facts, suppress same-owner facts, and retain aggregated
exact evidence. Imports remain separate and power import navigation, forbidden
import checks, and hypothetical cycle checks. Tests and manual output showed no
direction or terminology contradiction.

Evidence locations were present for calls, implementations, embeddings,
signature facts, imports, package/type inspection, call-site impact, and
compiler consequences. Byte offsets are sufficient for current verification;
the beta does not promise IDE-grade ranges.

## CLI review

The supported top level is intentionally:

```text
serve
tree
node
go package
go type
```

`tree` was explicitly retained after the Task 53 review. All removed flat and
global diagnostic commands are rejected by tests. Help at every scope explains
immediate versus transitive traversal, the five semantic kinds, the Go package
import/semantic distinction, type ownership, and textual refs without exposing
`NodeID`.

Every current README CLI example completed successfully. The canonical fixture
smoke also exercised all requested operations. One deliberately precise node
path returned no path because exact traversal does not invent structural
parent/child edges; a direct callable-to-callable example returned the expected
call path. Package projection independently found both semantic package routes,
which confirms rather than contradicts that distinction.

The self-analysis `node transitive-dependents ./... nocv/graph::Graph` example
is valid but produced more than 7,000 lines. Replacing it with a smaller
first-run example would improve onboarding without changing semantics.

## Analysis status review

The implemented lifecycle matches the design:

```text
failed   -> nil Analysis plus error
complete -> usable Analysis with no known tolerated loss of supported facts
partial  -> usable Analysis plus deterministic package-scoped reasons
```

Zero matched packages, invalid paths, and syntax errors fail rather than
returning an apparently useful model. Type-check errors produce Partial when
usable syntax/compiler state remains. The CLI prints one concise warning to
stderr, produces normal output, and exits successfully. The web API exposes
status separately, and the UI keeps a persistent non-blocking indicator.
Status does not leak into graph/query DTOs.

Manual Partial validation added one temporary type error to a copied canonical
fixture. NOCV warned once for `example.com/shop/inventory`, rendered its package
inspection, and exited zero. Manual zero-package and invalid-path checks
returned concise non-zero failures.

## Signature impact review

`Analysis.AnalyzeSignatureChange` is the sole production path. It resolves the
whole proposed signature once and uses it for:

- parameter call-site compatibility;
- an in-memory source overlay and compiler recheck of the real reverse-import
  closure;
- current interface contract loss;
- concrete promoted-method surface changes.

Parameter/result counts, types, order, and variadic status are modeled. Names
are descriptive only. Semantic identity uses `go/types`, so an identical or
name-only proposal skips call-site, compiler, contract, and structural work.
Receiver and generic/type-parameter changes remain explicitly unsupported.

The overlay builder and recheck pipeline remain separated after Task 48. It
reads the declaration file, replaces only the callable signature in an
in-memory overlay, reloads the changed package and represented reverse-import
closure, and compares sorted diagnostic multisets. Exact baseline matches are
suppressed; shifted same-package/same-message diagnostics are conservative
`uncertain`; unmatched diagnostics are `new`. Tests verify repeated analyses do
not accumulate edits and disk source remains unchanged.

Manual analysis of `BaseService.Validate(domain.ID) error` to
`BaseService.Validate(string) error` produced:

- call sites: one incompatible, one compatible, zero unknown;
- compiler: one new diagnostic attributed to `Service.CreateUser` at its
  source location;
- contracts: none for that selected method/change;
- structural: `Service` and recursive `ExtendedService` expose the changed
  promoted method.

The structured call result and compiler diagnostic agreed at the incompatible
direct call. Compiler method-set results agreed with structural promotion. No
contract/compiler contradiction appeared. Focused tests separately cover
concrete/interface contract loss, pointer receivers, result-only and combined
changes, shadowing, ambiguity, pointer-only exposure, forwarding, inference,
dirty baselines, and source isolation.

Compiler consequences mean only new or uncertain compile/type-check diagnostics
in the affected scope. README and UI explicitly state that no new diagnostic
does not prove behavioral correctness. Current contract and promoted-method
facts are structural lenses, not compiler-failure claims.

## Test and fixture review

There is exactly one checked-in representative Go project:
`testdata/go/project`. It is valid, understandable by scenario directory, and
rich enough to cover current integration behavior. Narrow syntax/Partial/dirty
states are generated or created from copies. Production code has no fixture
dependency.

The two files under `goanalyzer/testdata/overlays/nocv` are source snapshots,
not projects. `TestOverlayRecheckNOCVTimings` references both to compare
controlled self-analysis overlays; their `.txt` extension keeps invalid
hypothetical variants out of normal Go tooling.

Validation results:

- `make fmt-check`: pass.
- `make test`: pass (`cmd/nocv`, `goanalyzer`, `graph`, `internal/webui`, and
  `query`; `internal/testutil` has no direct tests).
- `go test -race -count=1 ./...`: pass.
- canonical fixture `go test -count=1 ./...`: pass for every package.
- focused analyzer suite: pass.
- focused query suite: pass.
- focused CLI suite: pass.
- full focused web suite: pass.
- `go vet ./...`: pass.
- `go mod tidy -diff`: no diff.

The initial attempt to run several Go suites concurrently against the same
build cache produced cache-entry errors, and the restricted audit sandbox
blocked a loopback listener. Sequential reruns with isolated temporary caches
and normal loopback permission passed; these were audit-environment artifacts,
not repository failures.

## Web and API review

The manual explorer smoke used the canonical project copy and verified:

- the page and package graph loaded;
- the Partial status indicator remained visible and named the affected package;
- package search selected `example.com/shop/impact/service`;
- package, type, and method inspection worked;
- dependency navigation opened exact/type evidence and source locations;
- the signature editor prefilled parameters and results;
- analysis rendered call-site, compiler, contract, and structural sections;
- compatible sites remained behind a disclosure;
- impacted callers, types, and origin methods were real symbol-navigation
  controls;
- edit/back navigation returned to the retained callable context.

The inspected `/api/signature-impact` JSON contained backend-computed
`callSites`, `compiler`, `contracts`, `structural`, and `exposure` fields in
deterministic order. The frontend only presented those facts. Endpoint tests
cover method constraints, concise JSON errors, invalid symbols/types, stale
request guards, and deterministic graph/status/inspection responses.

The server binds loopback on an ephemeral port, reports the URL, keeps Partial
analysis useful, and returns startup/listener errors. It relies on process
interruption for shutdown; graceful lifecycle controls are not required for
this beta.

## Documentation review

The README's scoped command names, graph identity, relationship meanings,
package/type projections, import separation, analysis-only signature workflow,
and limitations matched code and manual behavior. No deleted example path or
current use of the removed flat CLI/API names was found. Old terms remain only
in clearly historical design/refactor records and negative compatibility tests.

The README can better state the required Go version and can replace the
high-volume `Graph` transitive-dependent example with a smaller first-run
example. Its opening paragraph emphasizes call discovery before introducing
the broader semantic model, but the following text is accurate; this is
onboarding debt, not a false capability claim.

The repository-wide debt-marker search found only explanatory uses of
“temporary” and a local compiler-construction comment. No actionable
`TODO`/`FIXME`/`HACK`/`XXX` remained in production. The only production panic is
an impossible embedded-filesystem invariant while constructing the web
handler; normal invalid input follows errors or HTTP responses.

## Must fix before beta

None.

There are no observed correctness, semantic identity, determinism,
documentation-command, lifecycle-status, signature-impact, repository-hygiene,
or maintained-test blockers.

## Nice to fix before beta

These do not need to delay the tag.

1. **Clarify semantic no-op compiler wording** — When no signature change
   exists, the web result currently says no new diagnostics were observed even
   though the intentional optimization skipped the recheck. Saying “No
   compiler recheck was needed” would describe the path more exactly. Scope:
   tiny.
2. **Propagate cancellation into overlay rechecks** — Initial loading accepts a
   caller context, but `recheckPackagesWithOverlay` creates
   `context.Background()`. Cancellation is not currently exposed by the CLI or
   web workflow, so this is lifecycle/readability debt rather than a broken
   operation. Scope: small.
3. **Add two explicit negative-path regressions** — Directly cover generic
   callable rejection and an HTTP variadic proposal with zero parameters. The
   implementation rejects both through existing validation, and analyzer-level
   empty-variadic coverage exists, but endpoint/unsupported coverage could be
   more explicit. Scope: tiny.
4. **Improve README first-run ergonomics** — State the Go 1.27 requirement and
   replace the 7,000-line self-analysis transitive example with a bounded
   example. Scope: tiny.
5. **Reduce frontend shared-state concentration** — `internal/webui/static/app.js`
   still combines graph, navigation, inspector, status, and signature workflow
   state. Existing stale-response tests protect behavior, so a framework
   migration is unwarranted; focused same-runtime modules/helpers would make
   later changes safer. Scope: medium.

## Deferred after beta

Twelve items are deliberately deferred:

### Product

1. Compose a question-oriented `why` wrapper over existing precise queries.
2. Add a top-level `impact` CLI wrapper for maintained hypothetical analyses.
3. Add forward transitive dependencies only when a concrete product workflow
   requires them.
4. Add deterministic source rewriting/refactoring only after analysis semantics
   remain stable.
5. Consider additional node/relationship kinds only from demonstrated user
   questions.

### Architecture

6. Reconsider a shared traversal primitive only if the named query-specific
   walkers reveal a genuinely clearer common abstraction.
7. Rename or split `Analysis` only if lifecycle/build-configuration consumers
   outgrow its current cohesive retained-state role.

### UX

8. Revisit tree presentation, global history, deeper back navigation, and
   general UI polish as separate workflows.
9. Add result limiting/pagination or alternative path explanation after
   measuring large-repository path-volume behavior.

### Languages

10. Add another language only with concrete semantics; do not introduce a
    speculative backend interface first.

### Performance

11. Optimize loading, projection, or traversal only from measured bottlenecks;
    current functional and race checks are satisfactory.

### Agent integration

12. Design MCP/agent-facing composition only after the human CLI and read
    models have settled.

## Known limitations

- Analysis is Go-only and uses the active GOOS/GOARCH, tags, module/workspace,
  toolchain, and environment. It does not claim other build configurations.
- Tests are not loaded (`packages.Config.Tests` is false).
- The graph models packages, structs, interfaces, functions, concrete methods,
  and interface methods. Fields, aliases as declarations, and methods on
  non-struct named receiver types are outside the current structural model.
- Dynamic function-value targets and other unresolved calls may be absent;
  positive resolved facts remain authoritative within scope.
- External packages are represented only when included in the analyzed package
  set.
- Partial analysis means supported facts may be missing; absence is not
  conclusive.
- Signature changes cover parameter/result types, counts, order, and variadic
  status. Receiver and generic/type-parameter changes are unsupported.
- Contract impact reports loss among existing implementations, not newly gained
  contracts.
- Compiler consequence attribution is conservative for shifted or duplicate
  dirty-baseline diagnostics; some diagnostics have no symbol attribution.
- No-new-diagnostics means no new compiler/type-check diagnostic in the
  rechecked reverse-import scope, not behavioral correctness or passing tests.
- Hypothetical analysis does not execute tests, mutate source, or apply fixes.
- Path queries return all distinct simple paths and may produce large output on
  densely connected projects.
- The web explorer is a basic local read/analysis interface without reload,
  persistence, global type/function graphs, or general navigation history.

## Release recommendation

**Ready to tag `v0.beta.1`.**

No specific production fix is required before the tag. The five nice-to-fix
items can be selected independently without changing the beta definition. The
human release gate should confirm this beta definition and known-limitations
language, then either tag the current audited commit or choose a separate,
focused documentation polish task first.
