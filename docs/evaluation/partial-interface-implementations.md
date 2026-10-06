# Partial interface implementation investigation

## Issue

#8 — Interface implementation results can be misleading under Partial analysis

This investigation used the current NOCV checkout, the local GitHub CLI checkout
at `/home/owen/Projects/cli`, and temporary complete and partial Go modules. The
temporary modules and diagnostic code were removed after recording the results.

## Current contract

`AnalysisComplete` means a successful analysis has no known condition that
prevented NOCV from producing all supported facts for the loaded packages and
active build configuration. It is not a claim about other build tags, platforms,
test packages, or unsupported semantics.

`AnalysisPartial` means incomplete type information may have prevented supported
facts from being modeled. The implementation marks a package partial when
`packages.Package.IllTyped` is true or the package has a `packages.TypeError`.
Reasons are package-scoped, deduplicated, and sorted. Load, parse, and graph
invariant failures do not produce a Partial analysis; they fail the analysis.

Both `README.md` and `docs/architecture.md` say that, under Partial analysis,
"positive facts remain useful; missing facts are not conclusive." The status
code and architecture text describe incomplete information as something that
may remove or omit facts. They do not explicitly say that a positive fact may
itself be uncertain.

The most natural reading is therefore asymmetric: positive facts are usable as
facts, while absence is inconclusive. The behavior below is stronger than that
contract allows. Some positive `Implements` relationships are not confirmed
facts at all; they are conservative results returned by `go/types` when it
cannot prove a method is missing. Documentation and behavior are consequently
misaligned.

## Current implementation mechanism

`goanalyzer.addImplementations` compares every represented non-generic struct
with every represented non-generic, non-empty method-set interface. It completes
the interface and then calls:

1. `types.Implements(named, interfaceType)` for the value type;
2. `types.Implements(types.NewPointer(named), interfaceType)` for the pointer
   type if the value check is false.

If either call returns true, NOCV emits a type-level `EdgeImplements`. Its
evidence is the concrete struct declaration location. The edge has no certainty
or provenance marker beyond that location.

NOCV then uses `types.NewMethodSet(concreteType).Lookup` to add method-level
implementation edges. It only adds such an edge when the selected concrete
method is a represented direct child of the concrete type. Promoted methods can
therefore prove the type-level relationship without creating synthetic method
nodes or direct method-level edges.

There is no additional check for invalid embedded fields. Analysis status is
not consulted while implementation edges are created. A positive result from
`types.Implements` is represented as an unconditional graph relationship in
Complete and Partial analyses alike.

## GitHub CLI reproduction

The symptom was reproduced with the current NOCV code against the local GitHub
CLI checkout. Analysis was `Partial`, including `github.com/cli/cli/v2/api` and
`github.com/cli/cli/v2/internal/ghrepo` among the packages reported with
incomplete type information.

The expected implementations of
`github.com/cli/cli/v2/internal/ghrepo::Interface` remained present:

- `github.com/cli/cli/v2/api::Repository`;
- `github.com/cli/cli/v2/context::Remote`;
- `github.com/cli/cli/v2/internal/ghrepo::ghRepo`.

Those types expose the required `RepoHost`, `RepoName`, and `RepoOwner` methods
in their loaded method sets.

Suspicious type-level implementation edges also remained present, including:

- `api::GraphQLError` and `api::HTTPError`;
- `internal/codespaces/connection::TunnelClient` and `socketConn`;
- `internal/prompter::MockPrompter`;
- `internal/tableprinter::TablePrinter`;
- `pkg/cmd/attestation/inspect::CertificateInspection`;
- `pkg/surveyext::GhEditor`;
- several test/mock or data structs with unresolved embeddings.

The local dependency state explains representative entries. GitHub CLI's
`api.GraphQLError` embeds `*github.com/cli/go-gh/v2/pkg/api.GraphQLError`, but
that imported package was unavailable to the loader in this environment. The
package diagnostic was `could not import ... (invalid package name: "")`, the
embedded field became `invalid type`, and the package was ill typed.

For `api.GraphQLError`, both value and pointer method sets had length zero and
lookups for all three interface methods returned nil. Nevertheless,
`types.Implements` returned true for both forms and NOCV emitted the edge.
`api.HTTPError` behaved the same way apart from its one locally declared,
unrelated `ScopesSuggestion` method.

The missing dependencies were not downloaded merely to force a second GitHub
CLI environment. The controlled resolved/invalid pair below establishes that
dependency availability changes this class of result.

## Controlled reproduction

The controlled target was:

```go
type Repository interface {
    RepoHost() string
    RepoName() string
    RepoOwner() string
}
```

### Complete case

The Complete module contained three controls:

- a type embedding a valid external type with all three methods;
- a type embedding a resolved alias to a type with no methods;
- an unrelated type embedding a valid type with no methods.

The analysis status was `Complete` with no reasons. The real implementation had
three promoted methods in both relevant method sets, `types.Implements` returned
true, and NOCV emitted an implementation edge. The resolved non-implementation
and unrelated control had empty method sets, `types.Implements` returned false
for both value and pointer forms, and NOCV emitted no edges for them.

### Partial case

The Partial module changed the external alias target to an undefined
`MissingDependency` and embedded that alias in the candidate. The external
package reported the undefined-name type error. Both the external and candidate
packages had `IllTyped == true`; NOCV returned `Partial` with deterministic
`incomplete type information` reasons for those two packages.

The candidate's underlying type was a struct containing the embedded alias, but
the alias ultimately had invalid type information. Its value and pointer method
sets were empty, and lookups for `RepoHost`, `RepoName`, and `RepoOwner` all
returned nil. Despite this, `types.Implements` returned true for both value and
pointer forms. NOCV consequently emitted:

```text
ThroughUnresolvedEmbedding --Implements--> Repository
```

The same Partial module also contained a direct, valid implementation. Its
three declared methods were present, `types.Implements` returned true, and NOCV
correctly emitted its edge. A valid unrelated embedded type still returned
false. Partial results therefore mix confirmed positive relationships with
uncertain positive relationships; package-level Partial status alone does not
identify which is which.

## go/types behavior

The surprising predicate result originates in `go/types`, not in a custom NOCV
method-set algorithm.

The installed Go source documents `Checker.hasAllMethods` as deliberately
returning true when a struct contains embedded fields with invalid types. The
rationale is that the invalid embedded field might provide the requested method,
so the checker cannot say with certainty that a method is missing and avoids a
follow-on error. `types.Implements` reaches that logic after confirming that the
candidate's outer underlying type is a valid struct.

This explains the apparent contradiction:

```text
NewMethodSet(candidate): no target methods
Implements(candidate, target): true
```

In an ill-typed program, `types.Implements` is acting as a conservative
type-checking predicate, not issuing a proof that every required method is
known. When the embedded alias is valid and resolves to a type without the
methods, `types.Implements` returns false as expected.

## NOCV behavior

NOCV faithfully exposes the boolean returned by `types.Implements`; it does not
invent the positive result. The product-level problem is that it converts a
compiler's uncertainty-tolerant answer into an unconditional semantic edge.

The graph and its readers cannot distinguish a confirmed positive relationship
from one accepted only because an embedded field is invalid. Analysis status is
available on `Analysis`, but it is package-wide and is not copied onto graph
edges or query results. The absence of method-level implementation edges on the
GitHub CLI suspicious types is diagnostic evidence, not a supported certainty
signal: valid promoted implementations can also lack direct method-level edges.

Existing tests cover Complete implementation extraction, valid promotion, and
Complete/Partial/Failed status behavior. The Partial status fixture exercises
ordinary type errors, not invalid embedding combined with `Implements`, so this
case was not protected by a regression test.

## Are positive Implements facts trustworthy under Partial analysis?

No, not uniformly. A positive `Implements` edge in a Partial analysis can be:

- a confirmed direct or promoted structural implementation; or
- a partial-only positive caused by an invalid embedded field, where the loaded
  method set does not contain the required methods and complete resolution may
  make `types.Implements` false.

The latter is not safe to treat as a semantic fact. It is also not merely a
missing-fact problem. The controlled resolved counterpart proves that a positive
edge can disappear when the relevant type information becomes complete.

## Primary outcome

Outcome B — Positive facts can be uncertain in Partial analysis

Reason: incomplete embedded type information makes `go/types` deliberately
return a conservative positive from `types.Implements`. NOCV faithfully but too
strongly represents that answer as an unconditional edge. The issue is not an
independent NOCV implementation-predicate bug, and it is not specific to the
GitHub CLI environment.

## Product impact

Queries, inspection, dependency projections, and UI views that consume
`EdgeImplements` cannot currently tell whether a Partial-analysis relationship
was compiler-confirmed or uncertainty-tolerant. This can create misleading
architecture results even though the global Partial warning is accurate.

The current phrase "positive facts remain useful" remains true in the weak sense
that many positives are correct, but it is insufficient as a trust guarantee.
Users cannot safely interpret every positive `Implements` result as confirmed.
The finding is relationship-specific evidence that the present Partial contract
needs either narrower output or explicit uncertainty semantics.

## Recommended next action

Move issue #8 toward a focused design/fix task. First preserve this exact
invalid-embedding case as a durable regression fixture. Then choose explicitly
between:

1. suppressing an `Implements` edge when its positive result depends on invalid
   embedded type information; or
2. carrying and presenting relationship-level uncertainty so such an edge is
   not shown as confirmed.

Either approach should preserve confirmed direct and promoted implementations
that happen to coexist in a Partial analysis. A blanket removal of all
implementation edges from Partial packages would discard useful confirmed
facts. The chosen behavior should be reflected in the Partial trust contract,
CLI/web presentation, and implementation-query tests.

## What should not change yet

This investigation does not select the product representation, so it should not
yet change:

- `types.Implements` usage or implementation-edge filtering;
- analysis status semantics;
- graph or query DTOs;
- uncertainty fields;
- CLI warnings or web presentation;
- README or architecture wording.

Issue #8 should remain open until the product choice is made and implemented.
