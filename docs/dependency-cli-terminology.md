# Dependency CLI semantics and terminology review

This document records the Task 52 review of the current dependency-oriented
CLI and query APIs. It is an inventory and a recommendation for human review,
not an implemented rename plan. No command, output, query, or traversal behavior
changed as part of this review.

## Vocabulary used by the review

NOCV currently has several deliberately different forms of reliance:

- An **import dependency** is one stored `Imports` edge between represented Go
  packages. Imports are direct source imports, not semantic projections.
- A **declaration-level semantic relationship** is one stored `Calls`,
  `Implements`, `Embeds`, `Accepts`, or `Returns` edge between exact symbols.
- A **semantic package dependency** projects those five relationship kinds to
  enclosing packages, suppresses same-package facts, and aggregates the exact
  cross-package facts as evidence.
- A **type dependency** projects those five relationship kinds to enclosing
  structs or interfaces, suppresses same-type facts, and aggregates exact facts
  as evidence. Package functions have no type owner and are excluded.
- A **call-only projected dependency** is the older view produced by
  `query.Dependencies`. It projects only `Calls` edges. It is not the maintained
  semantic package view, despite the current `package-deps` name.
- A **dependent** is an entity that already relies on the selected entity.
  Direct dependents are one reverse step; transitive dependents are the
  recursive reverse closure.
- **Impact** remains reserved for consequences of a hypothetical change. It is
  not a synonym for dependencies, dependents, or reachability.

All current CLI identities are `graph.SymbolRef` strings. Package commands
require a package `SymbolRef`; type commands require a struct or interface
`SymbolRef`; declaration-level commands accept any represented `SymbolRef`.
No reviewed command accepts a graph-local `NodeID` or resolves an unqualified
human name.

## Current-state concept map

```text
imports
    global diagnostic dump of direct represented Go package imports

package-imports / package-importers
    direct forward / reverse Go import relationships for one package

package-deps
    direct cross-package Calls projection only (legacy), all package pairs

why-package-dep
    exact call evidence for one direct call-only package projection

direct-deps / direct-dependents
    one-hop forward / reverse declaration-level semantic relationships

transitive-dependents
    recursive reverse declaration-level semantic closure, with all simple paths

paths
    all simple forward declaration-level semantic paths between two symbols

package-paths
    all simple paths through the derived semantic package view

type-deps
    direct dependencies in the derived semantic type view

type-paths
    all simple paths through the derived semantic type view

inspect-dependency
    package semantic paths, with each hop divided into type and exact-only facts
```

There is no command literally named `importers`. The reverse import command is
`package-importers`. There is also no forward transitive-dependency closure
command. `paths` is a bounded path query between two supplied symbols, not that
missing symmetric operation.

## Command semantics matrix

“Semantic five” below means `Calls`, `Implements`, `Embeds`, `Accepts`, and
`Returns`. “Evidence retained” means the query result retains exact relationship
or location data even when the current text renderer omits some of it.

| Command | Input scope | Actual meaning | Relationships | Depth / aggregation | Evidence or paths | Backing API | Name accurate? | Recommendation |
|---|---|---|---|---|---|---|---|---|
| `imports` | Load pattern only | Dump every represented direct Go package import in the graph | `Imports` only | Direct; graph edges grouped by source for display | Targets only; stored locations are not printed | No query API; `printEdges` reads `Graph.Outgoing` | Under-specified: it is global, represented-only, and diagnostic | **remove** before beta; `package-imports` is the selected-package product operation |
| `package-imports` | One package ref | Packages directly imported by that package | `Imports` only | Direct, no transitive closure | `Relationship` retains import locations; CLI prints targets only | `query.DirectImports` | Yes | **keep** |
| `package-importers` | One package ref | Packages that directly import that package | `Imports` only, reversed for lookup | Direct, no transitive closure | `Relationship` retains import locations; CLI prints sources only | `query.DirectImporters` | Yes | **keep** |
| `import-cycle` | Proposed from/to package refs | Whether adding one import would close an existing import path from target back to source | `Imports` only | Transitive path search; every distinct simple package path | Package sequences, no edge/location evidence | `query.WouldCreateImportCycle` | Yes | **keep** |
| `check-forbidden-import` | From/to package refs | Whether the exact direct import already exists | `Imports` only | Direct, not transitive | Prints every stored import location | `query.CheckForbiddenPackageImport` | Yes | **keep** |
| `package-deps` | Load pattern only | Every direct cross-package **call-only** projection | `Calls` only | Direct package projection; aggregates calls by package pair | `query.Dependency` retains caller/callee and locations; CLI discards them | `query.Dependencies(g, NodePackage)` | No: sounds like the broader semantic package view and can be mistaken for imports | **remove** before beta as a historical compatibility surface |
| `why-package-dep` | From/to package refs | Why one direct **call-only** projected package dependency exists | `Calls` only | Direct package projection; no transitive search | Prints exact caller/callee and call locations; no path | `query.WhyDependsOn` | No: it does not explain all semantic package dependency facts or routes | **needs human decision**; prefer subsuming its useful locations into package dependency inspection/future `why`, otherwise rename it explicitly as call-only |
| `direct-deps` | Any represented symbol ref | Exact outgoing semantic relationships attached to the selected graph node | Semantic five | One stored edge; no package/type projection or target aggregation | `Relationship` retains locations; CLI prints kind and target only | `query.DirectDependencies` | Semantics are sound, but `deps` hides declaration level and relation policy | **rename** to `direct-dependencies` and explain “declaration-level semantic” in help |
| `direct-dependents` | Any represented symbol ref | Exact incoming semantic relationships attached to the selected graph node | Semantic five | One stored reverse edge; no projection | `Relationship` retains locations; CLI prints kind and source only | `query.DirectDependents` | Accurate after Task 51 when help supplies the declaration-level policy | **keep** |
| `transitive-dependents` | Any represented symbol ref | Every symbol with a reverse semantic path to the selected symbol | Semantic five | Recursive reverse closure; all distinct simple paths | `SemanticPath`/`SemanticStep`; no location evidence | `query.TransitiveDependents` | Yes; result includes direct and indirect dependents | **keep** |
| `paths` | From/to represented symbol refs | Every declaration-level semantic path from source to target | Semantic five | Transitive forward search; all distinct simple paths | `SemanticPath`/`SemanticStep`; no location evidence | `query.DependencyPaths` | Too generic; neither dependency nor declaration level is visible | **rename** to `dependency-paths` |
| `package-paths` | From/to package refs | Every route through the derived semantic package dependency view | Semantic five; imports excluded | Transitive package projection; each hop aggregates exact cross-package facts | Package paths and hop evidence; locations retained in relationships but not printed | `query.PackageDependencyPaths` | Understandable but omits the important word “dependency” | **rename** to `package-dependency-paths` |
| `type-deps` | One struct/interface ref | Direct dependencies in the derived semantic type view | Semantic five; imports excluded | Direct type projection; aggregates method/type facts by type pair | Exact relationship evidence retained and printed without locations | `query.DirectTypeDependencies` | Concept is accurate; abbreviation weakens consistency/discoverability | **rename** to `type-dependencies` |
| `type-paths` | From/to struct/interface refs | Every route through the derived semantic type dependency view | Semantic five; imports excluded | Transitive type projection; each hop aggregates exact facts | Type paths and hop evidence; locations retained but not printed | `query.TypeDependencyPaths` | Understandable but omits “dependency” | **rename** to `type-dependency-paths` |
| `inspect-dependency` | From/to package refs | Inspect every semantic package route and classify each hop's evidence as type-backed or exact-only | Semantic five; imports excluded | Transitive package paths, then per-hop evidence classification | Package paths, type dependencies, and exact-only relationships; locations retained but not printed | `query.PackageDependencyPaths`, `query.InspectPackageDependency` | Misleadingly generic because it only accepts packages | **rename** to `inspect-package-dependency` |
| `check-forbidden-dependency` | From/to package refs | Whether any semantic package dependency route exists | Semantic five; imports excluded | Transitive package projection; every simple package route | Package paths and aggregated hop evidence; locations retained but not printed | `query.CheckForbiddenPackageDependency` | Too generic for a package-only semantic rule | **rename** to `check-forbidden-package-dependency` |

### Projection and participation details

| Command family | Same-owner handling | Package-level functions | Imports | Determinism / duplicate policy |
|---|---|---|---|---|
| `direct-deps`, `direct-dependents`, `paths`, `transitive-dependents` | No package/type suppression; these are exact graph-node operations | Participate like any represented function | Excluded by the semantic-five policy | Direct results sort by kind and ref. Paths are simple and sort shortest then lexicographically; duplicate step sequences are suppressed |
| `package-deps`, `why-package-dep` | Same projected package is suppressed | Participate because functions project to their enclosing package | Excluded | Package pairs sort by refs; all contributing call evidence is retained by the query |
| Modern package semantic view (`package-paths`, `inspect-dependency`, forbidden dependency, package inspection) | Same package is suppressed | Participate; facts with no type owner remain exact-only during inspection | Excluded | Exact facts aggregate by package pair. Paths are simple, deduplicated by package sequence, and sorted shortest then lexicographically |
| Type semantic view (`type-deps`, `type-paths`, type inspection) | Same enclosing type is suppressed | Excluded because they have no type owner | Excluded | Exact facts aggregate by type pair. Paths are simple, deduplicated by type sequence, and sorted shortest then lexicographically |
| Import commands | Import edges already connect distinct package nodes | Not applicable | The only included relation | Direct results sort by package ref. Import-cycle paths are simple, deduplicated by package sequence, and sorted shortest then lexicographically |

External and standard-library imports appear only when the imported package is
represented in the loaded graph. The commands do not add module/stdlib/external
classification.

## `package-deps` and `why-package-dep`

These two commands use the oldest dependency model in the repository:
`query.Dependencies` iterates function nodes, reads only outgoing `Calls`, and
projects caller and callee to the nearest requested owner. With `NodePackage`,
the result is a direct call-coupling relation between packages. Same-package
calls are suppressed. Package functions and methods both participate.

`why-package-dep A B` finds one such direct projected package pair and returns
all exact calls that establish it, including their source locations. It does
not traverse a route, consider non-call semantic facts, or inspect imports.
The output heading, “Package call dependency explanation,” is more accurate
than the command name.

The maintained package model is different: `DirectPackageDependencies` and
`PackageDependencyPaths` include all semantic-five relationships, aggregate
exact facts by package boundary, and preserve evidence. Package inspection and
the browser use this model. NOCV self-analysis has already demonstrated that
the call-only and semantic projections can return different package edges.

Consequently `package-deps` should not ship as the apparent canonical package
dependency command. The preferred beta direction is to retire it rather than
rename it: `calls` already exposes raw call facts, and package inspection/path
operations expose the maintained architectural view. `why-package-dep` needs a
human product choice because its printed call locations remain useful. The
preferred choice is to preserve that useful evidence in broader package
dependency inspection or a future `why` wrapper, then retire the public
call-only command. If it remains independently supported, its name must say
that it explains direct package **call** dependency only.

## Exact, package, and type semantic views

`direct-deps` and `direct-dependents` are precise low-level operations. They do
not aggregate by target beyond the graph's stored edge representation and do
not project to packages or types. Their query models retain location evidence,
although the CLI currently hides it. A package ref is technically accepted,
but package nodes normally have no semantic-five edge because imports are
deliberately separate.

`transitive-dependents` follows the same five edge kinds in reverse. It reports
each reachable dependent once and attaches every distinct simple route from
that dependent to the selected symbol. Path-local visited state prevents
cycles; the selected symbol is not reintroduced through a cycle. Results sort
by dependent ref, and each dependent's paths sort shortest then
lexicographically. This is the reference meaning of recursive current reverse
reliance. No symmetric `transitive-dependencies` command currently exists, and
this review does not propose one merely for symmetry.

The type view maps a type node to itself and a method to its directly enclosing
struct or interface. It therefore aggregates method/type semantic facts while
excluding package functions. The package view maps every non-package semantic
source and target to enclosing packages, so package functions participate.
This is why a package dependency inspection can legitimately contain
“exact-only” evidence that has no type dependency counterpart.

`type-deps` duplicates the dependency portion of `inspect-node` for a type, but
it remains a useful focused low-level query because it shows exact evidence.
Package `inspect-node` similarly exposes modern direct semantic dependencies,
dependents, imports, and importers. The older `package-deps` command does not
duplicate that inspection model; it returns a narrower call-only view.

## Path behavior

All current recursive dependency traversals enumerate simple paths rather than
choosing one representative route:

- `DependencyPaths` uses forward declaration-level semantic steps.
- `TransitiveDependents` uses the same steps in reverse search direction and
  reports paths in natural dependent-to-selected orientation.
- `PackageDependencyPaths` traverses derived package steps; each step carries
  every exact fact at that package boundary. Different exact evidence does not
  create duplicate paths with the same package sequence.
- `TypeDependencyPaths` behaves analogously for type sequences.
- import-cycle checking searches existing import paths from the proposed target
  back to the proposed source.

Each uses path-local visited state, so cycles cannot recurse forever while
alternative acyclic routes remain discoverable. Results are deterministic:
shorter paths precede longer paths and equal-length paths compare
lexicographically. These are path/explanation operations, not closure sets,
except that `TransitiveDependents` groups its paths by every reachable node.

## Query API naming findings

| Query API | Actual semantics | Assessment |
|---|---|---|
| `Dependencies`, `Dependency`, `WhyDependsOn` | Direct projected **call-only** dependencies, at package or type level | Generic names materially overstate the relation set; historical API debt tied to the old commands |
| `DirectDependencies`, `DirectDependents` | One-hop declaration-level semantic-five relationships | Internally coherent, provided the shared edge policy remains documented |
| `TransitiveDependents` | Recursive reverse declaration-level semantic-five closure with all simple paths | Accurate after Task 51 |
| `DependencyPaths` | All simple forward declaration-level semantic-five paths | Accurate in query context; CLI `paths` is less clear |
| `DirectPackageDependencies`, `DirectPackageDependents`, `PackageDependencyPaths` | Modern derived semantic package view with aggregated exact evidence | Accurate and explicit |
| `DirectTypeDependencies`, `DirectTypeDependents`, `TypeDependencyPaths` | Derived semantic type view with aggregated exact evidence | Accurate and explicit |
| `DirectImports`, `DirectImporters` | One-hop represented Go import relationships | Accurate and distinct from semantic navigation |
| `WouldCreateImportCycle`, `CheckForbiddenPackageImport` | Proposed import-cycle paths / existing exact direct import | Accurate |
| `CheckForbiddenPackageDependency` | Any route in the semantic package view | API is more precise than its CLI name |
| `InspectPackageDependency` | Classifies one direct package step into type-backed and exact-only facts | Accurate; the CLI name omits package scope |

This review does not recommend a parameterized generic dependency API. The
distinct explicit operations carry meaningful endpoint, projection,
aggregation, and evidence rules.

## Help, README, and output gaps

The current top-level CLI error lists command usages but describes only
`direct-dependents` and `transitive-dependents`. It does not tell a user that:

- `package-deps` and `why-package-dep` are call-only legacy projections;
- `direct-deps` is declaration-level semantic navigation;
- imports are separate from semantic dependencies;
- `paths`, `package-paths`, and `type-paths` traverse different views;
- `inspect-dependency` and `check-forbidden-dependency` are package-only; or
- the bare `imports` command dumps all represented imports rather than taking a
  package ref.

README prose accurately defines direct imports and the modern semantic package
and type views, but its command list presents the legacy and modern commands as
peers without warning that their relationship sets differ. It also says NOCV
projects calls into package dependencies and explains them, then later explains
the broader semantic package view. Both statements are mechanically true, but
their proximity under the same “dependency” term obscures which commands use
which model. `inspect-dependency` is documented as package-specific in prose,
while its name remains generic.

Current output has a second distinction: several query DTOs retain exact source
locations while their CLI renderers print only refs and edge kinds. That is not
a semantic defect, but help must not promise location evidence unless the
renderer exposes it. Conversely, `why-package-dep` and
`check-forbidden-import` do print locations.

After human naming decisions, help and README should be updated together. This
review intentionally does not edit either public surface in advance.

## Product classification and beta recommendation

| Command | Current role | Beta decision proposed |
|---|---|---|
| `package-imports`, `package-importers`, `import-cycle`, `check-forbidden-import` | Core import capabilities | Keep |
| `direct-deps`, `direct-dependents`, `transitive-dependents`, `paths` | Useful precise declaration-level queries | Keep the capability; rename only `direct-deps` and `paths` |
| `type-deps`, `type-paths` | Useful precise type-projection queries | Keep the capability with expanded names |
| `package-paths`, `inspect-dependency`, `check-forbidden-dependency` | Core semantic package path, explanation, and rule capabilities | Keep the capability with package-explicit names |
| `imports` | Temporary/global diagnostic surface | Remove before beta |
| `package-deps` | Historical compatibility surface over the call-only projection | Remove before beta |
| `why-package-dep` | Candidate higher-level wrapper with useful but incomplete call evidence | Human decision: preferably subsume into broader package inspection/future `why` |

### Recommended beta command vocabulary

Subject to human approval, the coherent beta set for the reviewed capabilities
is:

```text
package-imports
package-importers
import-cycle
check-forbidden-import

direct-dependencies
direct-dependents
transitive-dependents
dependency-paths

type-dependencies
type-dependency-paths

package-dependency-paths
inspect-package-dependency
check-forbidden-package-dependency
```

The recommendation removes the global `imports` dump and call-only
`package-deps`. It leaves the fate of `why-package-dep` at the human gate. The
names are not perfectly formal encodings: help still needs to state the five
semantic kinds and declaration/package/type levels. This is shorter and more
usable than names such as `direct-semantic-package-dependencies` while making
the material import/semantic and exact/projected distinctions visible.

Flat command names remain workable at this size. A grouped future style such
as `nocv package dependencies` might reduce prefixes, but current evidence does
not justify a CLI hierarchy redesign in this task.

## Future `why` and `impact`

A future `why` can compose current deterministic relationship, path, and
evidence operations based on explicit endpoint kinds. It should use the modern
semantic package view for package questions, not silently inherit the call-only
meaning of `WhyDependsOn`. The useful call locations from the legacy query may
be included as evidence without preserving its incomplete public contract.

`impact` remains separate: it asks what a hypothetical change would cause and
is backed by concrete change analyses. Neither `transitive-dependents` nor a
future forward dependency closure should be renamed or summarized as impact.

## Traversal readability debt

`DependencyPaths`, `PackageDependencyPaths`, `TypeDependencyPaths`,
`TransitiveDependents`, and the import-cycle `importPaths` helper each define an
inline recursive `walk` closure. Their algorithms share path-local cycle
avoidance and deterministic result sorting, but differ materially in direction,
result grouping, projection, step evidence, and duplicate identity.

This is separate from terminology. A later readability task should first
extract named, query-specific traversal helpers so each algorithm reads in
domain terms. Only after comparing those concrete helpers should NOCV consider
shared traversal machinery, and only if a common abstraction improves rather
than hides these distinctions.

## Mechanical conclusions and human decisions

Mechanical findings:

- Current commands use “dependency” for import edges, exact semantic edges,
  call-only projections, semantic package projections, and type projections.
- `package-deps` is call-only; it is neither imports nor the maintained semantic
  package view.
- `why-package-dep` is direct call evidence, not a path or complete package
  semantic explanation.
- Import edges are excluded from every semantic dependency operation reviewed.
- Package functions participate in package semantic projection and can become
  exact-only evidence; they are excluded from type projection.
- Direct, recursive, aggregated, and path-oriented behavior is deterministic
  but not consistently visible in current names/help.

Human decisions required before implementation:

1. Approve removing the global `imports` diagnostic command before beta.
2. Approve removing the legacy call-only `package-deps` command rather than
   preserving it under a longer call-specific name.
3. Decide whether `why-package-dep` is retired into package inspection/future
   `why`, or retained under an explicitly call-only name.
4. Approve the proposed mechanical command renames as one coherent CLI/API/help
   task, including whether corresponding historical query APIs are removed or
   renamed.
5. Confirm that no new forward transitive closure command is needed merely to
   mirror `transitive-dependents`.

No implementation should begin until this review gate is resolved.
