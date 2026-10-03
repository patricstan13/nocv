# Analysis status and failure semantics

Task 49 defines the semantic contract for analysis outcomes. It is design-only:
no status API or behavior described here is implemented yet.

## Decision summary

An analysis attempt has one of three product outcomes:

```text
analysis attempt
├── failed                     LoadAnalysis returns nil, error
└── succeeded                  LoadAnalysis returns Analysis, nil
    ├── complete
    └── partial
```

- **Failed** means NOCV could not produce a usable semantic model for the
  requested load. No returned `Analysis` should be used.
- **Complete** means NOCV completed its supported analysis for the packages and
  active build configuration it actually loaded, with no known tolerated
  condition that may have removed supported semantic facts.
- **Partial** means NOCV produced a usable semantic model, but observed a
  tolerated condition that may have removed supported semantic facts.

`Complete` does not mean every Go configuration was checked, or even that NOCV
models every Go construct. `Partial` does not mean NOCV supports or reports all
compiler errors. The distinction only qualifies the completeness of NOCV's
supported semantic model for this load.

For beta, two successful states are sufficient. No severity, confidence score,
per-edge status, or additional successful state is justified.

## Current load path

`LoadAnalysis` currently performs these stages in order:

1. Default an empty pattern list to `.`.
2. Resolve the analysis directory using `os.Getwd` when needed and
   `filepath.Abs`.
3. Call `packages.Load` with `packages.LoadSyntax`, `Tests: false`, the active
   process/build environment, and the requested patterns.
4. Run `packageErrors`. It rejects all package errors except
   `packages.TypeError` and package-summary messages beginning with `# `.
5. Sort returned root packages by package path.
6. Create an empty graph and compiler-object symbol index.
7. Add package, supported type, function, and method nodes.
8. Add represented package imports.
9. Add resolvable call relationships.
10. Add represented struct/interface embedding relationships.
11. Add represented accepted/returned type relationships.
12. Derive interface implementations from retained `go/types` state.
13. Return the graph, packages, symbol index, load directory, and load mode in
    `Analysis`.

Every error after package loading is wrapped with its extraction stage and
returns `nil, error`. The current implementation never returns a non-nil
`Analysis` together with an error.

## Current failure paths

| Source | Current behavior | State available at failure | Proposed outcome |
|---|---|---|---|
| Current directory or absolute-path resolution fails | Returns a wrapped error | No packages or graph | Failed |
| `packages.Load` returns a top-level error | Returns `load Go packages` error | No reliable root package set | Failed |
| Package list, parse, unknown, or other non-type error | `packageErrors` returns sorted messages | `go/packages` may have partial packages/AST, but NOCV discards them | Failed |
| Pattern matches no packages | Currently returns an empty `Analysis` and empty graph | No semantic model | **Future failure**; this is not usable analysis |
| Package has no usable syntax | Usually represented by a list/parse error and currently fails | Package metadata may exist; supported extraction cannot run truthfully | Failed |
| Node construction violates graph identity/hierarchy invariants | Wrapped `analyze package` error | A partially built graph exists only locally and is discarded | Failed |
| Import/call/embedding/signature edge insertion fails | Wrapped stage-specific error | A partially built graph exists only locally and is discarded | Failed |
| Implementation edge insertion fails | Wrapped implementation error | A partially built graph exists only locally and is discarded | Failed |

For fixed source, patterns, toolchain, and environment, package-error sorting and
graph construction are deterministic. Directory access, module resolution, the
active Go environment, and external filesystem/module state can naturally alter
load failures.

Plain wrapped errors with the existing stage context are sufficient for beta.
A typed load-error hierarchy would add API surface without a current consumer.
The later implementation should add one concise wrapped error for a zero-package
match.

## Currently tolerated conditions

### Conditions observed by the loader

- `packages.TypeError` is deliberately tolerated.
- A package-summary message beginning with `# ` is ignored because it commonly
  duplicates the positioned type-check diagnostic that follows.
- Ill-typed packages can still contain usable `Syntax`, `Types`, and
  `TypesInfo`, so graph construction continues.

Controlled experiments showed that ordinary argument type errors, missing
imports reported as type errors, and unresolved declaration types all return
usable compiler state. They also showed why that model is potentially
incomplete: declarations remain available, but call, signature,
implementation, import, or embedding facts can be absent when compiler objects
or types cannot be resolved.

### Information silently skipped during extraction

Current extraction represents missing semantic information as `nil`, `false`,
or an empty result rather than an error in many places:

- a declaration with no `types.Object` is not entered in the symbol index;
- an unresolved call target or unrepresented caller/callee creates no call
  edge;
- an unresolved embedded type creates no embedding edge;
- an unresolved parameter/result type creates no signature edge;
- an unavailable named type or method selection creates no implementation
  edge;
- imports that cannot be resolved to represented loaded packages create no
  import edge.

These skips are not recorded individually. Today NOCV cannot say exactly which
facts a type-check error prevented, so it must qualify the whole returned
analysis conservatively.

### Unsupported by design is not partial

Intentional model boundaries do not make an analysis partial. Current examples
include fields, aliases as structural declarations, methods on modeled-out
non-struct named receiver types, function literals as separate callable nodes,
dynamic function-value targets, generic implementation candidates skipped by
the current implementation, and symbols outside the requested represented
package scope.

`Complete` means complete for the supported model, not complete for all Go
semantics. `Partial` is reserved for a known condition that may have prevented
NOCV from producing a fact it otherwise intends to model.

## Observed `go/packages` scenarios

The following were observed with the current `packages.LoadSyntax`,
`Tests: false` configuration using temporary Go modules. `go/packages` error
kinds below are the actual package errors returned; its top-level error was nil
unless stated otherwise.

| Scenario | `go/packages` result | Current `LoadAnalysis` | Graph usefulness | Recommended outcome |
|---|---|---|---|---|
| Valid small module | One package; syntax/types/type info present; no errors | Returns graph | Package, type, function, and signature facts present | Complete |
| Ordinary argument type error | One ill-typed package; summary plus positioned `TypeError`; syntax/types/type info present | Returns graph | Declarations, call, and signature facts were still present | Partial: incomplete type information |
| Missing imported package | Root package with two `TypeError`s; syntax/types/type info present | Returns graph | Local package/function nodes present; import/type facts absent | Partial: incomplete type information |
| Undefined interface method parameter type | Ill-typed package with positioned `TypeError` | Returns graph | Interface/concrete method nodes present; signature/implementation facts absent | Partial: incomplete type information |
| One type-broken file plus one valid file | One ill-typed package with both syntax files and type info | Returns graph | Valid declarations and several relationships remain useful | Partial: incomplete type information |
| Syntax error | Recovered syntax plus `ParseError`s | Returns nil and sorted error | Recovered AST is deliberately not treated as a trustworthy model | Failed |
| One syntax-broken file plus one valid file | Both files returned, package has multiple `ParseError`s | Returns nil and sorted error | Potential partial graph is discarded | Failed |
| Empty module loaded as `.` | Placeholder package with `ListError`: no Go files | Returns nil and error | No useful graph | Failed |
| Empty module loaded as `./...` | Zero packages, no top-level error | Currently returns empty analysis | No useful graph | Future failure |
| Pattern matching no packages | Zero packages, no top-level error | Currently returns empty analysis | No useful graph | Future failure |
| Invalid package path | Placeholder package with `ListError` | Returns nil and error | No useful graph | Failed |
| Invalid analysis directory | Zero packages plus top-level `packages.Load` error | Returns nil and wrapped error | No useful graph | Failed |

No valid-source fixture was found that triggers an internal graph construction
error. Those paths enforce NOCV identity/edge invariants; if one occurs, the
partially built graph is untrustworthy and failure remains correct.

## Usable-analysis boundary

A returned `Analysis` should require all of the following:

1. At least one root package was discovered for the requested patterns.
2. Each returned root package has the syntax required by supported extraction;
   list/parse/unknown load failures remain fatal.
3. Stable package/declaration references and graph hierarchy were constructed.
4. Every graph extraction stage completed without an invariant error.
5. The retained compiler state is sufficient for at least the modeled facts
   that survived extraction.

Ordinary type-check errors do not automatically violate this boundary. The
result can still answer useful positive questions such as “which declarations
and resolved calls were found?” It cannot make equally strong negative claims,
so it is partial.

## Complete and partial semantics

### Complete

NOCV completed its supported analysis for the non-test packages selected by the
requested patterns and active Go build context, without a known tolerated
condition that may have removed supported semantic facts.

This is scoped to the files and packages actually selected by the current
GOOS/GOARCH, build tags, module/workspace configuration, toolchain, and
`Tests: false`. It says nothing about other configurations or test variants.

### Partial

NOCV produced a usable graph and retained compiler state, but one or more loaded
packages were ill-typed or reported type-check diagnostics. Because extraction
silently skips unresolved compiler objects/types, supported semantic facts may
be missing.

For beta, this is the only demonstrated partial reason. Package summary wrappers
are not a separate reason because they duplicate the underlying diagnostics.
Future explicit tolerated extraction failures may add reasons, but only when
the analyzer can detect them reliably.

### Relationship absence

For a complete analysis, “not found” means no fact was found within NOCV's
supported model, represented package scope, and active build configuration. It
is not a claim about unsupported Go constructs or other configurations.

For a partial analysis, a positive fact remains useful, but absence is not
conclusive: an unresolved compiler object or type may have prevented that fact
from being created. This applies to dependencies, dependents, callers,
implementations, embedding, and accepted/returned type relationships.

## Recommended beta representation

An enum alone is too terse because users need to know why negative results are
qualified. A `Complete bool` plus reasons permits confusing combinations and
has poor zero-value semantics. The smallest useful design is an enum plus a
separate defensively copied reason list owned by `Analysis`:

```go
type AnalysisStatus uint8

const (
    analysisStatusUnknown AnalysisStatus = iota // defensive zero, not a product outcome
    AnalysisComplete
    AnalysisPartial
)

type AnalysisStatusReasonKind uint8

const (
    AnalysisReasonIncompleteTypeInformation AnalysisStatusReasonKind = iota + 1
)

type AnalysisStatusReason struct {
    Kind    AnalysisStatusReasonKind
    Package graph.SymbolRef
}

func (analysis *Analysis) Status() AnalysisStatus
func (analysis *Analysis) StatusReasons() []AnalysisStatusReason
```

Reasons should be deduplicated and deterministically ordered by package path.
One reason per affected package is enough. Do not expose diagnostic messages,
positions, severities, or counts in the beta status API. This communicates scope
without turning status into a compiler-diagnostics feed.

Status should be computed once during loading and stored on `Analysis`. The
inputs are currently derivable from retained packages, but storing the result
ties it to the load attempt and leaves a clear place for later explicit
tolerated extraction conditions. The implementation should use positioned
`TypeError`s and `Package.IllTyped` as a conservative backstop; it should not
count duplicate `# ` summaries as separate reasons.

This is a coarse whole-analysis status with package-scoped reasons. Per-package
status objects, per-node/edge metadata, confidence percentages, multiple
partial severities, and a diagnostic taxonomy are unnecessary for beta.

## Product integration recommendation

### CLI

- Failed: preserve the current non-zero exit and wrapped error on stderr.
- Partial: execute the requested command and print its useful result normally;
  print one concise warning to stderr before or after the result.
- Complete: print the normal result with no additional status noise.

Suggested partial wording:

```text
NOCV produced a partial analysis; some semantic relationships may be missing
because one or more loaded packages could not be fully type-checked.
```

### Web UI

Expose one persistent, non-alarming “Partial analysis” indicator with the same
short explanation. Do not add a diagnostics panel. The web server owns an
`Analysis`, so status can be exposed once in bootstrap/status data rather than
copied into every query response.

### Query models

Keep analysis status out of language-agnostic `query` DTOs. Status describes the
Go analysis session, not an individual graph projection. Callers that combine
an `Analysis` with query results should communicate the status alongside the
result at the application boundary.

### Signature-change analysis

A partial base analysis should still allow `AnalyzeSignatureChange` when the
selected callable has sufficient retained compiler state. Consumers should be
able to see the base analysis status separately from the impact result.

`CompilerImpact.BaselineStatus` remains independent. It reports whether
diagnostics exist in the affected reverse-import scope for one hypothetical
recheck; analysis status reports whether the initially returned project model
is known to be semantically incomplete. A later overlay load/recheck failure
continues to return an operation error and does not retroactively change the
base analysis status.

## Minimal implementation recommendation for the next task

1. Fail `LoadAnalysis` explicitly when `packages.Load` returns zero root
   packages.
2. After fatal package-error filtering, collect one
   `AnalysisReasonIncompleteTypeInformation` per ill-typed/type-error package.
3. Complete every existing extraction stage exactly as today; any stage error
   remains a failed attempt returning nil.
4. Store complete/partial status and sorted reasons only after construction
   succeeds.
5. Add read-only, defensive-copy accessors on `Analysis`.
6. Test valid, type-error, missing-import, parse-error, no-package, and mixed
   valid/type-broken scenarios.
7. Handle CLI/UI presentation in separately reviewed changes if desired; do not
   require status propagation through query DTOs.

## Deliberately deferred complexity

Beta does not need:

- multiple partial severities or successful states;
- comprehensive compiler-diagnostic support;
- structured diagnostic messages or locations;
- per-package status objects;
- per-node or per-edge confidence/completeness;
- percentages or probabilities;
- a typed load-error hierarchy;
- status fields on every query result;
- build-matrix analysis; or
- analysis of test variants.

## Human decisions before implementation

The recommended defaults are:

1. Treat `Package.IllTyped` as a conservative partial-status backstop in
   addition to explicit `TypeError`s.
2. Expose the affected package path in each coarse reason, but no diagnostic
   details or counts.
3. Let signature-impact analysis run against a partial base analysis when its
   selected callable remains resolvable.
4. Make zero matched packages a failed attempt.

These four points should be approved before the implementation task. No status
API or behavior change is part of Task 49.
