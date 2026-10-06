# Relationship certainty final validation

## Scope

This report validates the complete relationship-certainty change for issue #8
without changing implementation behavior. It covers the canonical NOCV fixture,
Caddy, Wails, GitHub CLI, focused baseline comparisons, deterministic contract
analysis, architecture-rule outcomes, shell exits, and CLI/HTTP/browser
presentation.

The real-project directories were treated as read-only source snapshots. No
formatting, dependency update, generation, or module-maintenance command was run
inside them.

## Feature branch

Validation ran on `feature/relationship-certainty` at
`1e2b99ad4d28feb9ee412d0e20fd4270d0ac0777`.

The branch contained the six intended certainty commits from design through
consumer presentation. It was not switched, reset, rebased, amended, or merged.
The five pre-existing untracked evaluation documents were left untouched.

## Validation baseline

The semantic baseline was detached commit
`dbbbee1081398e8c4bf337ed43c85f1382aa9bf7`. Separate binaries were built from
the baseline and feature worktrees under `/tmp` and applied to the same local
source snapshots and active Go environment.

Comparisons were semantic and focused rather than byte-for-byte whole-project
dumps. They covered representative implementation sets, exact dependencies,
dependency paths, and type/package projections. Differences were classified as
certainty refinements, presentation-only differences, environment differences,
or potential regressions.

## Repository summary

Counts are stored graph relationships, including Confirmed `Imports`.
Every Uncertain count below consists exclusively of type-level `Implements`.

| Repository | Analysis status | Confirmed | Uncertain | Invariant triggered? | Result |
|---|---|---:|---:|---|---|
| Caddy | Partial, 44 package reasons | 5,536 | 562 | No | Pass with environment drift documented |
| Wails | Partial, 194 package reasons | 8,879 | 0 | No | Pass |
| GitHub CLI | Partial, 289 package reasons | 15,364 | 1,596 | No | Pass |

The canonical fixture analyzed as Complete with 249 Confirmed and zero
Uncertain relationships.

## Caddy

### Analysis status

The current source snapshot and active environment produced Partial analysis
with 44 package-scoped incomplete-type-information reasons. The same warning and
package list appeared with the pre-certainty binary, so this is environment
state rather than a feature-branch regression. The earlier evaluation's
Complete status does not describe the current environment.

### Interface implementations

`github.com/caddyserver/caddy/v2::Module` retained the baseline membership of
127 represented types. Of these, 122 established implementations are Confirmed
and five incomplete-embedding candidates are Uncertain. The 122 Confirmed count
matches the prior evaluation's useful implementation set.

`github.com/caddyserver/caddy/v2/modules/caddyhttp::MiddlewareHandler` retained
the baseline membership of 32 represented types. The same 23 established
implementations remain Confirmed and nine incomplete-embedding candidates are
Uncertain. The 23 Confirmed count matches the prior evaluation.

Representative real implementations such as reverse proxy `Handler`,
`FileServer`, `Authentication`, `Encode`, `Rewrite`, `Templates`, and `Subroute`
remain Confirmed. No established implementation was downgraded.

The Uncertain candidates have concrete unresolved-embedding explanations in
the current compiler state: examples embed external types such as
`*pflag.FlagSet`, `acme.Client`, `*quic.EarlyListener`, and `zapcore.Core`.
Baseline treated those compiler positives unconditionally; the feature retains
the relationships but qualifies them.

### Dependency/path regression check

The cross-package relationship from `caddyconfig/httpcaddyfile` to
`modules/caddyhttp` remains present with exact evidence. The representative
exact path

```text
httpcaddyfile::Helper::NewRoute
  accepts -> caddyhttp::MiddlewareHandler
```

was identical between baseline and feature. Evidence and endpoints were
preserved. The direct package hop was also present in the baseline package
inspection; exhaustive package inspection remains very large, as recorded in
the earlier evaluation.

### Certainty findings

Caddy produced 5,536 Confirmed and 562 Uncertain stored relationships. All 562
Uncertain relationships are type-level `Implements`; Calls, Embeds, Accepts,
Returns, FieldType, method-level Implements, and Imports remain Confirmed.
Partial status did not downgrade the thousands of independently established
relationships.

### Stage 2 invariant

Not triggered. Analysis returned a usable Partial result rather than the
unhandled-positive analyzer error.

## Wails

### Analysis status

Wails analyzed as Partial with 194 package-scoped incomplete-type-information
reasons. This is consistent with the previous evaluation's Partial result.

### Interface implementations

`Transport` has the same two baseline implementations, both Confirmed:

- `examples/websocket-transport::WebSocketTransport`
- `pkg/application::HTTPTransport`

`ServiceStartup` has the same nine baseline implementations, all Confirmed,
including the Dock, key-value store, notifications, and SQLite services plus
the represented example and fixture services. Baseline and feature inspector
outputs were identical.

### Dependency/path regression check

The application-to-asset-server relationship remains present. The exact path

```text
pkg/application::New
  calls -> internal/assetserver::NewAssetServer
```

was identical between baseline and feature.

### Certainty findings

Wails produced 8,879 Confirmed and zero Uncertain relationships. Its global
Partial state did not make its established Transport or ServiceStartup
contracts uncertain and did not change existing path behavior.

### Stage 2 invariant

Not triggered.

## GitHub CLI

### Analysis status

GitHub CLI analyzed as Partial with 289 package-scoped
incomplete-type-information reasons. The analysis completed with a usable
graph.

### ghrepo.Interface

The feature retained all 15 baseline type-level candidates and divided them
into three Confirmed and twelve Uncertain relationships.

Confirmed:

- `api::Repository`
- `context::Remote`
- `internal/ghrepo::ghRepo`

Representative Uncertain candidates:

- `api::GraphQLError`
- `api::HTTPError`
- `internal/codespaces/connection::TunnelClient`
- `internal/codespaces/connection::socketConn`
- `internal/prompter::MockPrompter`
- `internal/tableprinter::TablePrinter`
- `pkg/cmd/attestation/inspect::CertificateInspection`

The remaining uncertain candidates were also retained rather than omitted.
Sampled candidates embed external compiler types such as
`*ghAPI.GraphQLError`, `*ghAPI.HTTPError`, `*tunnels.Client`,
`*websocket.Conn`, generated/mock types, and `certificate.Summary`. These are
the unresolved-embedding condition established by the Stage 2 investigation;
they are not classified merely by name.

### Path regression check

The baseline and feature produced identical command-registration path:

```text
ghcmd.Main
  -> root.NewCmdRoot
  -> issue.NewCmdIssue
  -> list.NewCmdList
  -> listRun
```

They also produced the same two GraphQL paths:

```text
listRun -> issueList -> listIssues -> api.Client.GraphQL
listRun -> issueList -> searchIssues -> api.Client.GraphQL
```

No path step or evidence was lost.

### Certainty findings

GitHub CLI produced 15,364 Confirmed and 1,596 Uncertain stored relationships.
Every Uncertain relationship is type-level `Implements`; every other
relationship kind remains Confirmed. The `ghrepo.Interface` diff consists only
of explicit uncertainty markers on the twelve suspicious baseline candidates.

### Stage 2 invariant

Not triggered.

## Baseline comparison

### Expected certainty refinements

Baseline interface membership was retained. Caddy's incomplete embedded-type
candidates and GitHub CLI's issue-reproducing candidates changed from
unqualified presentation to Uncertain. Established Caddy, Wails, and GitHub CLI
implementations remained Confirmed.

### Presentation-only differences

Feature CLI output adds `[uncertain]` to individual uncertain relationships,
projected rows, and path evidence. Confirmed lines retain baseline formatting.
HTTP adds explicit certainty fields, and the browser adds textual/dashed
uncertainty presentation.

### Environment differences

Caddy was Complete in the historical evaluation but is Partial in the current
active environment. Both newly built baseline and feature binaries reported
the same 44 package reasons, so this does not originate in relationship
certainty. Wails and GitHub CLI remain Partial as expected; reason counts are
reported as current observations rather than stable repository properties.

### Potential regressions

None found. Representative memberships, exact dependencies, paths, projection
evidence, and import semantics were preserved. No new Confirmed relationship,
lost established relationship, unexplained downgrade, or invariant failure was
observed.

## Complete-analysis invariant

The canonical healthy fixture analyzed as Complete and contained 249 Confirmed
and zero Uncertain relationships. The existing canonical invariant test also
passed. None of the three current real-project environments analyzed as
Complete, so no real-project Complete result contradicted the contract.

## Partial-analysis behavior

The observations match the documented policy. Confirmed and Uncertain coexist
in Partial Caddy and GitHub CLI graphs; Wails is Partial while every represented
relationship remains Confirmed. Certainty is relationship-specific rather than
derived from global status. Uncertain relationships identify particular
compiler-positive unresolved embeddings, while absence under Partial analysis
remains inconclusive.

## Change/contract analysis validation

The full suite and focused contract tests passed. Confirmed Implements can
produce deterministic lost-contract consequences. Uncertain Implements do not
enter `Contracts`; relevant candidates remain available in the separate
`UncertainContracts` collection with explicit certainty. Concrete-method and
interface-method deterministic cases continue to pass.

## Architecture-rule validation

Focused query and CLI tests plus direct shell runs established:

- a fully Confirmed forbidden semantic path returns `ConfirmedViolation` and
  prints `VIOLATION`;
- an Uncertain-only path returns `PotentialViolation`, prints
  `POTENTIAL VIOLATION (inconclusive)`, and is neither a clean pass nor a
  confirmed violation;
- no path returns `NoObservedPath` and prints `No violation.`;
- when both path classes exist, Confirmed evidence dominates the aggregate
  result while exact uncertain evidence remains qualified.

`check-forbidden-import` and `check-import-cycle` retain their baseline query,
rendering, and shell behavior.

## CLI exit-code contract

### Current behavior

Direct shell execution produced:

| Result | Exit status |
|---|---:|
| Confirmed forbidden semantic dependency | 0 |
| Potential/inconclusive forbidden semantic dependency | 1 |
| No observed semantic dependency | 0 |
| Confirmed forbidden import | 0 |
| Proposed import would create a cycle | 0 |

Parsing, package-resolution, analysis, and other execution errors also use
non-zero status. A PotentialViolation is implemented as a focused sentinel
error after the complete human-readable result has been printed.

### Sibling command behavior

`check-forbidden-import` returns exit 0 for both violation and no violation.
`check-import-cycle` returns exit 0 for both cycle and no cycle. Their command
handlers render semantic conclusions but return errors only when the command
cannot execute correctly. Tests assert content, not rule-pass exit status.

### Interpretation

Historically, exit 0 in this command family means “the command executed and
reported its answer,” not “the architecture rule passed.” Stage 3 intentionally
uses exit 1 for uncertainty because the accepted task required an uncertain-only
route not to look like clean success. That choice is understandable for an
inconclusive result but is asymmetric: a confirmed violation exits 0 while a
potential violation exits 1. The CLI therefore does not yet offer a coherent
lint-style contract suitable for CI automation.

This does not invalidate the semantic result model or issue #8 fix. Changing
all check-command exits would be a broader CLI contract decision, not a local
certainty-classification correction.

### Recommendation

**C. Preserve current behavior for #8 but follow up with a separate CLI
consistency issue before relying on these commands in CI.**

Reason: the outputs and query outcomes are correct, sibling commands establish
execution-success as the historical meaning of exit 0, and changing one or all
check commands requires an explicit compatibility decision. The asymmetry is
independent technical debt and does not block this branch.

## Presentation sanity check

A focused Partial fixture was served through the actual feature binary.

- CLI displayed `implements [uncertain]` and marked the projected path step.
- HTTP `/api/node` returned `"certainty":"uncertain"` on both the type
  dependency and exact Implements evidence.
- The browser graph rendered the uncertain-only package edge dashed with the
  explicit label `uncertain`.
- Package and type inspectors rendered the textual badge `UNCERTAIN`.
- After adding an independent Confirmed implementation in the temporary
  fixture, the projected graph edge became solid and unlabeled, the Confirmed
  dependent remained visually quiet, and the separate uncertain exact/type
  relationship retained its badge.

This confirms that the frontend presents backend certainty rather than
re-deriving it and does not rely on color alone.

## Merge blockers

None found.

The Stage 2 invariant did not fire, no Complete analysis produced Uncertain
relationships, no unexpected relationship or path regression appeared, and
all automated and manual gates passed. The check-command exit asymmetry is
documented as separate non-blocking CLI contract debt.

## Merge readiness

**READY**

Reason: formatting, full tests, vet, nested fixture, focused certainty tests,
three real-project analyses, baseline comparisons, deterministic consumer
checks, and presentation checks all passed without an unexplained semantic
regression or invariant failure. The original misleading unconditional
interface-implementation behavior is replaced by explicit, relationship-local
certainty while established facts remain Confirmed.

## Issue #8 recommendation

**READY TO CLOSE AFTER MERGE**

Reason: the reported GitHub CLI failure mode is reproduced and now represented
as Uncertain end to end. Deterministic consumers do not act on it as proof,
exploratory consumers retain it as context, and real established
implementations remain Confirmed. No validation blocker remains. The issue must
not be closed before merge.

## Follow-up work

- Define and document a uniform shell-exit contract for
  `check-forbidden-dependency`, `check-forbidden-import`, and
  `check-import-cycle` before advertising them as CI gates.
- If current-environment Complete Caddy analysis is needed again, investigate
  its 44 incomplete-type-information package reasons separately without
  weakening relationship certainty or downloading dependencies merely to
  force Complete status.

No classifier, graph, query, CLI, browser, documentation-contract, or test
change is recommended as part of this validation commit.
