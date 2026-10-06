# Relationship certainty design

## Issue

#8 — Interface implementation results can be misleading under Partial analysis

## Problem statement

The investigation in `partial-interface-implementations.md` established a
relationship-level trust problem. In a Partial analysis, `go/types.Implements`
can return true because a concrete type contains an invalid embedded field,
even when the available method set does not establish the required interface
methods. NOCV currently stores that result as the same unconditional
`EdgeImplements` used for a fully established implementation.

Package-level Partial status cannot solve this distinction. A single ill-typed
package can contain a directly declared implementation whose method set is
fully known and another type whose apparent implementation exists only because
`go/types` suppresses follow-on errors. Marking the package or all its nodes as
uncertain would erase useful precision.

The model needs a deterministic relationship state:

- **Confirmed** means the available supported semantic information establishes
  the relationship.
- **Uncertain** means incomplete semantic information permits the relationship,
  but does not establish it conclusively.

This is analysis state, not probability, likelihood, or a confidence score.

## Current graph representation

`graph.Edge` currently contains `From`, `To`, `Kind`, and a list of source
locations. Its semantic identity is the private `edgeKey` tuple:

```text
(From, To, Kind)
```

`Graph.AddEdge` validates endpoint kinds and evidence, then merges repeated
insertions into that identity. Evidence locations are deduplicated. The same
edge pointer is stored in the edge map and both adjacency indexes; `Outgoing`
and `Incoming` return detached copies.

No edge or evidence location carries analysis state. `Analysis.Status` is kept
outside the graph and reports only package-scoped reasons for the whole load.
The graph is intentionally language-independent and contains no `go/types`
objects.

This representation already has the right relationship identity and merge
point for certainty. It lacks only a relationship-wide state and a merge rule.

## Current relationship/query flow

There is no single query-level `Relationship` through which all semantic use is
routed:

1. Direct dependency and import navigation copies graph edges into
   `query.Relationship`.
2. Exact dependency paths and transitive dependents traverse `Graph.Outgoing`
   or `Graph.Incoming` directly and construct `SemanticStep` values.
3. Package and type projections traverse raw edges directly, then create
   `Relationship` values as exact evidence beneath derived dependencies.
4. Package dependency inspection matches projected evidence by
   `(From, To, Kind)`.
5. Node inspection wraps query relationships in `SymbolRelationship`.
6. Contract impact analysis reads type-level `EdgeImplements` directly from the
   graph. Structural impact similarly reads `EdgeEmbeds` directly.
7. Architecture dependency rules consume projected paths. Import rules and
   cycle checks consume `EdgeImports` directly.
8. CLI rendering consumes query DTOs. The web server maps query DTOs into a
   separate set of explicit JSON presentation types, and JavaScript renders
   those JSON types.

Consequently, query DTOs are presentation and projection boundaries, not the
semantic source of truth. Certainty added only after an edge reaches one of
those boundaries would arrive too late for several current consumers.

## Option 1 — certainty on graph edges

### Required changes

Conceptually, add a language-independent enum and field:

```go
type RelationshipCertainty uint8

const (
    RelationshipCertaintyUnknown RelationshipCertainty = iota
    RelationshipConfirmed
    RelationshipUncertain
)

type Edge struct {
    From      NodeID
    To        NodeID
    Kind      EdgeKind
    Certainty RelationshipCertainty
    Evidence  []Location
}
```

`Graph.AddEdge` should reject `Unknown`, retain certainty outside `edgeKey`, and
merge repeated insertions with this rule:

```text
Confirmed + anything => Confirmed
Uncertain + Uncertain => Uncertain
```

All existing extractors would explicitly emit Confirmed edges. Only the
type-level `Implements` extractor would initially be able to emit Uncertain.
The graph should not restrict Uncertain to `EdgeImplements`; storage is generic,
while production remains relationship-specific.

Query models that copy or summarize edges would carry certainty forward.
Human-facing traversals would preserve it. Strict decision consumers would
filter or classify using it. Web presentation DTOs and renderers would expose
it deliberately rather than serializing graph structs directly.

### Advantages

- The stored relationship is the single semantic source of truth.
- Every direct graph consumer sees the state before making a traversal or
  decision.
- The existing edge merge point naturally supports an uncertainty-to-confirmed
  upgrade.
- Graph identity, path identity, and location evidence remain stable.
- The graph stays language-independent: it stores a deterministic relationship
  property, while `goanalyzer` owns the Go-specific reason for producing it.
- Future consumers can choose exploratory or strict semantics without
  re-running compiler logic.

### Risks

- Every production edge constructor and many graph-focused tests must state
  Confirmed explicitly because an omitted zero value should be rejected.
- Certainty must be copied through several query DTOs; missing one copy site
  could silently lose presentation state.
- Existing algorithms currently treat every returned edge as unconditional.
  Each consumer needs an explicit policy before uncertain edges are introduced.
- A relationship-wide merge loses proof-specific certainty if future extractors
  produce independently classified evidence for one edge. Current extraction
  does not require that distinction.

### Blast radius

The implementation would touch, at minimum:

- `graph.Edge`, `Graph.AddEdge`, edge-copy tests, and deduplication tests;
- all analyzer edge creation sites, with a focused classifier for type-level
  `Implements`;
- `Relationship`, `SymbolRelationship`, and `SemanticStep` copies;
- package and type dependency summaries and their exact evidence;
- exact, package, type, and transitive path rendering;
- contract-impact consumption of existing implementation edges;
- architecture dependency checks;
- CLI labels and web JSON/presentation rendering;
- focused Complete/Partial, projection, rule, API, and UI tests.

This is broad propagation but shallow in most locations: copy an existing edge
property rather than derive new semantics.

## Option 2 — certainty on query relationships

### Required changes

`query.Relationship` and related presentation models would gain certainty while
`graph.Edge` remained unchanged. Some component would still have to remember
which stored `Implements` edge was uncertain and attach that state whenever a
relationship was created.

Because the graph currently stores no such distinction, this option would need
one of the following additional mechanisms:

- re-run Go-specific invalid-embedding/method-set reasoning in every query
  construction path;
- pass `goanalyzer.Analysis` into query APIs that currently accept only
  `*graph.Graph`;
- maintain an analysis-owned side map keyed by graph edge identity; or
- classify only selected human-facing DTOs and knowingly leave raw graph
  consumers unconditional.

Exact paths, transitive traversal, projections, and contract impact would each
need special integration because they read raw graph edges before constructing
or without constructing `query.Relationship`.

### Advantages

- `graph.Edge` and existing graph insertion tests would initially remain
  unchanged.
- Human-facing node/package responses could be labeled with a small local DTO
  change if they were the only consumers that mattered.
- It could avoid exposing a generic graph capability before another edge kind
  needs it.

### Risks

- It creates two semantic sources of truth: a graph that says the relationship
  exists unconditionally and a query layer that sometimes qualifies it.
- The query package cannot derive the known uncertainty correctly because it
  deliberately has no compiler state.
- Graph-first algorithms can traverse an uncertain relationship before any
  query state is attached.
- Contract impact and future refactoring could treat an uncertain contract as
  confirmed even while inspectors label it uncertain.
- Adding a side map would make graph copies, tests, and consumers easy to
  separate from the state they need.
- Passing `Analysis` into language-independent query functions would reverse
  the current compiler boundary.

### Blast radius

The apparent DTO-only change is misleading. A presentation-only implementation
would be smaller but semantically incomplete. A correct implementation would
touch every raw graph consumer plus introduce either a side channel or a new
analysis/query coupling. It would duplicate more logic than Option 1 and make
future consumers less safe by default.

## Comparison

| Criterion | Option 1: graph edge | Option 2: query relationship |
|---|---|---|
| One semantic source of truth | Yes | No, unless paired with a side channel |
| Exact path traversal sees certainty | Directly | Requires separate lookup/derivation |
| Package/type projection sees certainty | Directly | Must classify while projecting |
| Contract impact sees certainty | Directly | Bypasses query relationship |
| Preserves compiler boundary | Yes | Not without a side map |
| Initial graph churn | Moderate | Low only for an incomplete solution |
| Ongoing consumer safety | Stronger | Easy to bypass |
| Supports future strict workflows | Yes | Requires coordination with parallel state |

Option 1 is preferred. The current architecture does in fact suffer from the
risks described for Option 2; semantic consumption is not centralized behind
`query.Relationship`.

## Edge identity and deduplication

Confirmed and Uncertain variants of the same `(From, To, Kind)` are the same
relationship with different aggregate state, not two graph facts. Certainty
must not be added to `edgeKey`.

The merge rule is deliberately small:

```text
any confirmed proof => Confirmed
otherwise, uncertain proof => Uncertain
```

This permits `Uncertain -> Confirmed` during insertion and never permits
`Confirmed -> Uncertain`. Evidence locations continue to merge uniquely.
Outgoing and incoming indexes already share the stored edge pointer, so an
upgrade at insertion would be visible from both directions.

Current extraction order does not require an upgrade: a struct/interface pair
is considered once by implementation extraction, and other repeated edges have
uniformly confirmed evidence. The upgrade rule is nevertheless appropriate at
the existing deduplication boundary and prevents order-dependent behavior if a
future extractor supplies a second proof.

Path identities should continue to use endpoint/kind sequences or projected
symbol sequences. Because one edge identity has one aggregate certainty, adding
certainty to path keys would create no valid additional path and would make an
upgrade look like a different route.

## Evidence interaction

Certainty belongs on the relationship, not on each `Location`.

For the proven `Implements` case, NOCV emits one type-level relationship for one
struct/interface comparison and attaches the struct declaration as its
location evidence. It does not have multiple independently classified proofs
for that edge. Method-level implementation edges are separate relationships
and, when emitted from an established method selection, remain Confirmed.

If repeated insertions eventually provide both uncertain and confirmed
evidence, the relationship is Confirmed and all unique locations remain useful
verification context. The model does not claim that every location independently
proves the aggregate state. A per-evidence certainty lattice would add types,
merge behavior, projection behavior, and UI complexity without a current
semantic need. It should be introduced only if a future extractor demonstrates
that users must distinguish proofs within one relationship.

## Traversal semantics

`Graph.Outgoing` and `Graph.Incoming` should continue to return every matching
edge. They are storage accessors, not policy APIs. Each returned edge carries
its certainty.

The default for exploratory architecture workflows should be **include
Confirmed and Uncertain, while preserving state**. This applies to direct
dependencies/dependents, implementation navigation, exact dependency paths,
transitive dependents, node inspection, and explanatory package/type paths.
Dropping uncertain relationships would contradict the agreed goal of retaining
useful context in unfamiliar codebases.

The default for deterministic decisions should be **act on Confirmed only and
report relevant Uncertain relationships separately as inconclusive context**.
An uncertain edge must never be silently treated as either a confirmed fact or
as proof that no relationship exists.

Cycle checks currently use only direct `Imports`, for which no uncertainty case
is known. They remain confirmed-only by production, with no behavior change.

Exact `SemanticStep` values need certainty so a path cannot erase the state of
one of its edges. A separate path-level enum is unnecessary initially: a path
is fully confirmed only when every step is Confirmed; otherwise its steps show
exactly where it is uncertain.

## Projection semantics

Package and type projection should include both Confirmed and Uncertain exact
relationships. Each exact `Relationship` in `Evidence` retains its own
certainty.

A derived `PackageDependency` or `TypeDependency` should also expose one
summary certainty using the same merge rule as a stored edge:

```text
any confirmed exact relationship => projected dependency is Confirmed
only uncertain exact relationships => projected dependency is Uncertain
```

There should be one projected dependency per source/target pair, not parallel
confirmed and uncertain projections. If a confirmed call and an uncertain
implementation both cross the same package boundary, the package dependency is
Confirmed because its existence is established by the call. The uncertain
implementation remains visible and labeled in the dependency's evidence.

Projected path steps inherit the projected dependency summary. Inspection must
retain the exact evidence states so users can distinguish the established
reason from additional unresolved context. Existing evidence matching can keep
its `(From, To, Kind)` identity because certainty is state, not identity.

The package and type projections are currently rebuilt from exact graph edges,
so this rule does not require a second stored projection graph.

## Consumer matrix

| Consumer | Confirmed | Uncertain | Notes |
|---|---|---|---|
| implementation query | Include | Include and label | An uncertain implementation is useful context but not a conclusive contract. |
| dependencies | Include | Include and preserve | Direct exploratory navigation should not discard context. |
| dependency paths | Include | Include and preserve per step | A path is fully confirmed only when all steps are Confirmed. |
| transitive dependents | Include | Include and preserve per path step | Do not present an uncertain-only route as a certain dependent route. |
| package projection | Include | Include and aggregate | Any confirmed exact fact confirms the projected pair; retain all labeled evidence. |
| type projection | Include | Include and aggregate | Same rule as package projection. |
| architecture rule checks | Enforce | Report separately as potential/inconclusive | A confirmed path is a violation; an uncertain-only path is neither a clean pass nor a confirmed violation. |
| signature/change analysis | Use for deterministic contract consequences | Exclude from deterministic contract impacts; optionally report unresolved context | Call-site and compiler lenses are independent; current `Embeds` structural facts remain Confirmed. |
| CLI inspector | Show normally | Show with an explicit `uncertain` marker | Do not require users to infer state from a global Partial warning. |
| web UI | Show normally | Show with label/style and explanatory text | Backend supplies state; JavaScript does not infer it. |
| future deterministic refactor | Actionable | Never actionable; block or request complete analysis | Source rewriting must not rely on a relationship the compiler information cannot establish. |
| future agent context | Include as fact | Include explicitly as unresolved context | Agents need architectural clues without being told they are confirmed. |

Import queries and import-cycle checks continue to use Confirmed imports only,
because no uncertain `Imports` production is proposed.

Architecture dependency checks need a future tri-state result rather than a
boolean interpretation:

```text
confirmed violation
potential/inconclusive violation
no observed violation
```

If any fully confirmed forbidden path exists, the result is a confirmed
violation. If no confirmed path exists but an uncertain path does, the result
is potential/inconclusive. Only the absence of both is “no observed violation,”
and under global Partial status even that absence retains the existing
package-level caveat.

## Implements-specific uncertainty detection

Uncertainty must not be derived from `AnalysisPartial` or
`packages.Package.IllTyped` alone. Confirmed and uncertain implementations can
coexist in the same package.

For the proven case, future extraction needs these facts for the value or
pointer type selected by the current implementation predicate:

1. `types.Implements(candidate, interface)` returns true;
2. the candidate's effective method set does not positively establish every
   required interface method with the required signature; and
3. the candidate contains direct or recursive embedded type information that
   resolves to invalid/unresolved compiler types.

When all required methods are established by `types.NewMethodSet` and compiler
object/signature identity, the edge is Confirmed even if the containing package
is Partial for another reason. When the predicate is true only through the
known invalid-embedding behavior, the type-level edge is Uncertain.

The future implementation will need a small Go-specific recursive invalid
embedded-type check, including pointer/alias handling and a visited set, because
the relevant `go/types` helper is not exported. That helper belongs in
`goanalyzer`, not `graph` or `query`.

This design does not classify every possible behavior of ill-typed Go programs.
If `types.Implements` is true, the method proof is incomplete, and no invalid
embedded type explains it, extraction should not silently invent another
uncertainty rule. That condition should be covered or surfaced as an analyzer
invariant during the implementation task.

Only type-level `EdgeImplements` needs to produce Uncertain initially.
Method-level implementation edges with a concrete compiler selection remain
Confirmed. No evidence currently justifies uncertain `Calls`, `Embeds`,
`Accepts`, `Returns`, `FieldType`, or `Imports` production.

## Naming recommendation

Use **Confirmed** and **Uncertain** as the public pair, represented by a
`RelationshipCertainty` enum. These names describe whether the relationship is
established without suggesting probability.

Use `Unknown` only as the zero-value/internal invalid state:

```text
Unknown = construction omitted or invalid
Confirmed = available semantic information establishes the relationship
Uncertain = incomplete information permits but does not establish it
```

`Resolved/Unresolved` is less precise because a relationship is not itself a
compiler name-resolution operation. `Established/Uncertain` is defensible, but
`Confirmed` is shorter and clearer in CLI/UI output.

The zero value must not mean Confirmed. `Graph.AddEdge` should reject Unknown,
forcing every producer to state its semantics. This creates a mechanical
migration for current extractors but prevents a new edge from silently becoming
trusted because its constructor omitted a field.

## Serialization/API impact

Graph edges are not serialized directly. The web API already uses detached,
explicit presentation DTOs for relationships, symbol relationships, package
dependencies, type dependencies, and package graph edges. The CLI renders query
models directly rather than serializing them.

The eventual API change can therefore be additive:

- add a required string `certainty` to exact relationship and symbol-
  relationship JSON;
- add a required summary `certainty` to package/type dependency JSON and
  package graph edges;
- preserve certainty on path DTOs if path endpoints are later exposed by HTTP;
- serialize both `"confirmed"` and `"uncertain"` explicitly, with no
  `omitempty`.

Explicit Confirmed output prevents old absence/default ambiguity and makes
captured responses self-describing. The maintained browser client can be
updated atomically. Generic JSON clients normally tolerate additive fields,
but strict external decoders could require adjustment; NOCV currently documents
no stable external HTTP API version.

No compiler object or invalid-type detail should cross the API boundary. The
API communicates the relationship state, not the internal Go-specific reason.

CLI output can minimize noise by leaving Confirmed rows visually unchanged and
adding `[uncertain]` only to Uncertain rows. The semantic DTO should still carry
both states explicitly.

## Documentation impact

If implemented, `README.md` and `docs/architecture.md` should replace the
current incomplete-facts-only wording with semantics equivalent to:

> A Partial analysis remains useful but may omit supported facts. Relationships
> marked Confirmed are established by the available supported semantic
> information. Relationships marked Uncertain are retained as architectural
> context because incomplete compiler information permits them but does not
> establish them. Absence remains inconclusive.

The architecture contract should also state:

```text
Structure is for orientation.
Relationships are for reasoning.
Evidence is for verification.
Certainty states how conclusively a relationship is established.
```

It should clarify that Partial does not make every relationship uncertain and
that only `Implements` currently has an uncertainty-producing condition.
Command and web documentation should explain the visual/text marker and the
strict treatment used by architecture rules and deterministic changes.

No documentation contract changes belong in this design-only task.

## Recommended design

Choose **Option 1: store certainty on `graph.Edge`**.

Use a generic, language-independent `RelationshipCertainty` capability with
`Unknown`, `Confirmed`, and `Uncertain`. Reject Unknown at graph insertion.
Keep edge identity as `(From, To, Kind)`, merge evidence as today, and upgrade
the aggregate state to Confirmed when any confirmed proof is inserted.

Initially, every existing extractor emits Confirmed. Only type-level
`Implements` extraction may emit Uncertain, based on method-set proof plus the
specific invalid embedded-type condition established by the investigation.

Exploratory graph/query/projection/UI workflows include both states and preserve
them. Deterministic architecture decisions, impact claims, and future
refactoring act only on Confirmed relationships and surface relevant Uncertain
relationships as inconclusive context.

Package and type projections retain certainty on each exact evidence
relationship and summarize a projected pair as Confirmed if any exact evidence
is Confirmed, otherwise Uncertain.

## Why this is the smallest coherent design

The smallest code diff in isolation would be a label added to one query DTO,
but it would not solve the semantic problem. Current exact traversals,
projections, contract analysis, and rules consume graph edges directly. Any
correct query-only design would need a parallel certainty lookup or repeated
Go-specific inference.

One enum and one field at the existing relationship storage and deduplication
boundary eliminate those parallel mechanisms. The remaining changes are
mechanical propagation or consumer policy. The graph capability is generic
because certainty is a relationship property; production is narrow because
only `Implements` has demonstrated uncertain semantics.

This preserves the current boundaries:

- `goanalyzer` decides why a Go relationship is Uncertain;
- `graph` stores identity, aggregate certainty, and evidence;
- `query` preserves or projects that state;
- CLI and web presentation explain it;
- strict workflows decide whether the state is actionable.

No side channel, compiler leakage, speculative confidence model, stored
projection graph, or unrelated node redesign is required.

## Open questions

The implementation task should resolve these focused details with tests:

1. The exact recursive invalid-embedded-type helper across aliases, pointers,
   and nested embedding, matching the installed/supported `go/types` behavior.
2. Whether an incomplete interface declaration can create a distinct uncertain
   positive; this investigation proved only the concrete invalid-embedding
   case.
3. The precise CLI exit/status behavior for an uncertain-only forbidden
   dependency, while preserving the recommended tri-state semantics.
4. Whether the package graph should use a dashed edge, badge, or both for an
   uncertain-only projection; the API state should not depend on that visual
   choice.
5. Whether future path DTOs need a convenience aggregate state in addition to
   per-step certainty. Per-step state is sufficient for correctness now.
6. Whether a future mixed-proof extractor needs per-evidence certainty. No
   current extractor does.

These are implementation and presentation decisions within the recommended
model, not reasons to defer the model choice.

## What should not change yet

This design task should not yet change:

- `graph.Edge`, `Graph.AddEdge`, or edge identity;
- `query.Relationship`, projections, or traversal behavior;
- `Implements` extraction or compiler helpers;
- analysis status semantics;
- architecture-rule results or exit behavior;
- signature/change impact behavior;
- CLI or web output and JSON;
- fixtures or tests;
- README or architecture contracts;
- any relationship kind other than the proposed future `Implements` producer.

Issue #8 remains open. This report is related to the issue and does not close or
fix it.
