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
go run ./cmd/nocv tree ./...
go run ./cmd/nocv calls ./...
go run ./cmd/nocv implementations ./...
go run ./cmd/nocv embeddings ./...
go run ./cmd/nocv signatures ./...
go run ./cmd/nocv imports ./...
go run ./cmd/nocv package-imports ./... nocv/query
go run ./cmd/nocv package-importers ./... nocv/graph
go run ./cmd/nocv direct-deps ./... nocv/query::Impact
go run ./cmd/nocv direct-dependents ./... nocv/query::DirectDependents
go run ./cmd/nocv impact ./... nocv/graph::Graph
go run ./cmd/nocv paths ./... nocv/query::Impact nocv/graph::Graph
go run ./cmd/nocv package-paths ./... nocv/cmd/nocv nocv/query
go run ./cmd/nocv package-deps ./...
go run ./cmd/nocv why-package-dep ./... nocv/cmd/nocv nocv/query
```

The graph model is language-independent. Go syntax trees are used only inside
`goanalyzer` and are converted into graph nodes and semantic edges before being
returned. The CLI prints both the hierarchy and a simple `Calls` section.
It also projects calls into direct package dependencies without storing another
semantic edge kind, and can explain each direct dependency using the underlying
call relationships and source locations. Direct semantic navigation combines
Calls, Implements, Embeds, Accepts, and Returns while preserving each stored
relationship kind and its evidence. Transitive impact analysis walks those
relationships in reverse and reports every distinct simple dependency path to
each potentially affected node. Exact symbol-to-symbol dependency explanation
uses the same semantic policy and reports every distinct simple path in stored
dependency direction without type or package projection. Package dependency
paths project those exact explanations onto package ownership, grouping equal
architectural routes while retaining every concrete semantic path as evidence.

Imports are stored direct package-level Go dependencies when both packages are
represented in the loaded graph. They are distinct from NOCV semantic
dependencies (`Calls`, `Implements`, `Embeds`, `Accepts`, and `Returns`) and
from derived package dependency paths. External packages are not represented
unless they are part of the analyzed package set.

Query and workflow features that benefit from manual validation should expose
a focused CLI command instead of adding unconditional demonstration output.
Internal helpers do not need commands of their own.
