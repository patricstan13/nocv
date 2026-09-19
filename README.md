# NOCV

NOCV discovers the structural hierarchy of Go projects—packages, structs,
interfaces, functions, struct methods, and interface methods—and resolved
function-to-function call relationships within the analyzed project.

```sh
go run ./cmd/nocv .
go run ./cmd/nocv ./...
```

The graph model is language-independent. Go syntax trees are used only inside
`goanalyzer` and are converted into graph nodes and semantic edges before being
returned. The CLI prints both the hierarchy and a simple `Calls` section.
