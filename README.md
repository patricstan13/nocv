# NOCV

NOCV discovers the structural hierarchy of Go projects—packages, structs,
interfaces, functions, struct methods, and interface methods—and resolved
function-to-function call relationships within the analyzed project. Calls
inside nested Go function literals are lexically attributed to their nearest
enclosing declared function or method. Direct function signature relationships
record represented project structs and
interfaces accepted or returned by functions and methods. Pointer and alias
uses normalize to their declarations; containers and variadic parameters are
intentionally not traversed.

It also records struct-to-interface and concrete-method-to-interface-method
implementation relationships established by Go's type system, plus explicit
struct and interface embedding relationships.

```sh
go run ./cmd/nocv serve ./...
go run ./cmd/nocv tree ./...
go run ./cmd/nocv node dependencies ./... nocv/query::TransitiveDependents
go run ./cmd/nocv node dependents ./... nocv/query::DirectDependents
go run ./cmd/nocv node transitive-dependents ./... nocv/graph::Graph
go run ./cmd/nocv node dependency-paths ./... nocv/query::TransitiveDependents nocv/graph::Graph
go run ./cmd/nocv node inspect ./testdata/go/project/... example.com/shop/documentation/service::Service
go run ./cmd/nocv go package imports ./... nocv/query
go run ./cmd/nocv go package importers ./... nocv/graph
go run ./cmd/nocv go package dependency-paths ./... nocv/cmd/nocv nocv/query
go run ./cmd/nocv go package inspect-dependency ./testdata/go/project/... example.com/shop/typeview/app example.com/shop/typeview/repository
go run ./cmd/nocv go package check-forbidden-import ./... nocv/query nocv/graph
go run ./cmd/nocv go package check-forbidden-dependency ./... nocv/cmd/nocv nocv/graph
go run ./cmd/nocv go package check-import-cycle ./... nocv/graph nocv/cmd/nocv
go run ./cmd/nocv go type dependencies ./testdata/go/project/... example.com/shop/typeview/service::Service
go run ./cmd/nocv go type dependency-paths ./testdata/go/project/... example.com/shop/typeview/service::Service example.com/shop/typeview/store::Store
```

`node` commands operate on exact represented graph entities and stored semantic
relationships. `node dependencies` and `node dependents` are immediate;
`node transitive-dependents` follows reverse relationships recursively and
includes simple dependency paths. `go package` and `go type` contain operations
whose meaning depends on Go packages, imports, structs, interfaces, and method
ownership. Run `nocv node --help`, `nocv go package --help`, or
`nocv go type --help` for the scoped command lists. “Impact” remains reserved
for consequences of a hypothetical change; no generic `impact` command is
currently exposed.

## Graphical package explorer

Run `nocv serve <pattern>` to load a graph once and start a read-only package
dependency explorer on an automatically selected local port. The command prints
the `http://127.0.0.1:<port>` URL and keeps serving until it is stopped; it does
not open a browser. For example:

```sh
go run ./cmd/nocv serve ./...
```

The canvas shows packages and direct semantic package dependencies. For
architectural reading, arrows point from a dependency to the package that uses
it: if `format` depends on `ast`, the canvas shows `ast → format`. NOCV's
underlying model still records this as `format` depending on `ast`. The graph
uses physics for its initial layout and then freezes for stable manual
exploration. Package search locates and focuses an existing package node
without filtering or changing the graph. Select a package to inspect
its documentation, compact contents, and semantic neighbors. Separately
modeled Go imports remain available under advanced package details. Its
Dependencies and Dependents jump directly to the corresponding
package relationship in the graph; a dependency opened this way provides a
Back action to its originating package, and both dependency endpoint packages
can be opened directly. Selecting a package also exposes its modeled types and
package-level functions; those symbols lead into type, method, function, and
semantic-relationship inspection. Select an edge to inspect the
type-level and exact-only facts that establish that direct dependency. A
contextual type drilldown shows only the type relationships explaining that
selected package dependency. Package-level facts with no type owner remain
available as exact-only evidence and are intentionally absent from the type
graph. The current interface intentionally has no reload, persistence, global
type graph, type search, or function graph.

Type and function inspectors expose related symbols directly. Methods, call
targets, accepted and returned types, callers, owners, and implementation
relationships can be followed without leaving the inspector. Evidence
locations remain available as collapsed supporting detail.
Function and method inspectors can also evaluate a hypothetical callable
signature change covering ordered parameters, ordered results, and variadic
status. The result keeps direct argument compatibility, compiler-observed
consequences, lost interface contracts, and promoted-method structural changes
separate. The compiler section rechecks the changed Go package and its real
reverse import closure through an in-memory source overlay. A clean result means
only that no new compile/type-check diagnostics were observed in that scope; it
does not establish behavioral correctness. The workflow is analysis-only.

The graph model is language-independent. Go syntax trees are used only inside
`goanalyzer` and are converted into graph nodes and semantic edges before being
returned. The CLI prints both the hierarchy and a simple `Calls` section.
Every node has two deliberately separate identities: an opaque, graph-local
`NodeID` used for storage and traversal, and a deterministic, human-readable
`SymbolRef` used by commands and APIs (for example
`nocv/query::TransitiveDependents`). Node
IDs start at 1 for each loaded graph and are never part of CLI or web output.
When Go permits repeated declarations such as `init` or the blank identifier,
their references receive deterministic source-order suffixes such as `#1` and
`#2`; the declaration name itself remains unchanged.
Nodes retain source-authored declaration documentation where available. For Go,
this includes package, type, function, method, and interface-method doc
comments; inline implementation comments are not modeled.
It also projects calls into direct package dependencies without storing another
semantic edge kind, and can explain each direct dependency using the underlying
call relationships and source locations. Direct semantic navigation combines
Calls, Implements, Embeds, Accepts, and Returns while preserving each stored
relationship kind and its evidence. Transitive-dependent analysis walks those
relationships in reverse and reports every distinct simple dependency path to
each recursively dependent node. Exact symbol-to-symbol dependency explanation
uses the same semantic policy and reports every distinct simple path in stored
dependency direction without type or package projection. Package dependency
paths instead traverse a derived semantic package view. A direct package
dependency exists when a stored semantic relationship crosses a package
boundary, and multiple lower-level facts are aggregated as evidence for that
package edge. Package routes do not require one continuous symbol-level path.
Go imports remain a separate package relation.

Exact symbol queries operate on functions, methods, and types directly. Type
dependency queries derive struct/interface relationships only when both ends of
a semantic fact belong to represented types; package-level functions do not
participate. Package dependency queries independently derive relationships from
semantic facts crossing package boundaries.

`go package inspect-dependency` shows each package dependency hop, the type-level
relationships that explain that hop, and any remaining exact semantic facts
that have no type-level representation.

`node inspect` shows declaration metadata and documentation, structural
children, direct semantic relationships, and relevant package/type projections
for one graph node.

Unified signature analysis evaluates every known direct Go call site when a
proposal changes parameters and reports compatible, incompatible, and genuinely
unknown sites separately. Type expressions are resolved with Go's type checker
in the callable declaration file's context, so builtins, local named types,
imported qualified types, aliases, and composite types retain compiler
semantics. Primitive and named non-struct types participate without becoming
graph nodes. Parameter changes also run the same compiler overlay recheck as
result changes and can report lost interface contracts and concrete types whose
promoted method surface changes. Structural results distinguish methods
promoted onto `T` itself from methods visible only on `*T`.

The maintained workflow covers parameter and result types, counts, order, and
variadic status. It does not support receiver changes, generic signature
changes, source rewriting, test execution, or behavioral correctness. Contract
analysis is limited to existing implementations and does not report newly
gained contracts.

Imports are stored direct package-level Go dependencies when both packages are
represented in the loaded graph. They are distinct from NOCV semantic
dependencies (`Calls`, `Implements`, `Embeds`, `Accepts`, and `Returns`) and
from derived package dependency paths. External packages are not represented
unless they are part of the analyzed package set.

`go package check-import-cycle` checks whether a proposed direct package import would close an
existing import path and therefore create a cycle.

`go package check-forbidden-import` checks whether one exact package directly imports
another forbidden package. It does not evaluate transitive imports or semantic
dependency paths.

`go package check-forbidden-dependency` checks whether one exact package has any projected
semantic dependency path to another exact package. It is distinct from
`go package check-forbidden-import`, which checks only a direct Go import.

Query and workflow features that benefit from manual validation should expose
a focused CLI command instead of adding unconditional demonstration output.
Internal helpers do not need commands of their own.
