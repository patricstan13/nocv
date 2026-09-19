# NOCV

NOCV currently discovers the structural hierarchy of Go projects: packages,
structs, interfaces, functions, struct methods, and interface methods.

```sh
go run ./cmd/nocv .
go run ./cmd/nocv ./...
```

The graph model is language-independent. Go syntax trees are used only inside
`goanalyzer` and are converted into graph nodes before being returned.
