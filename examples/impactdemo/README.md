# Impact demo

`impactdemo` is a controlled manual-validation target for NOCV. It is larger
and more realistic than a focused analyzer fixture, but small enough that its
relationships and change impacts can be predicted before running the tool.

The example is a separate Go module so it stays out of NOCV's own `./...`
package graph and remains independently compilable. It has no external
dependencies and does not import NOCV.

## Architecture

The five packages have this source-level dependency shape:

```text
api -> service -> repository -> domain
 |        |                       ^
 |        +-----------------------+
 +------> repository
api/service -> util
```

The direct semantic view is similarly predictable:

- `api` depends on `service`, `domain`, and `util`.
- `service` depends on `repository`, `domain`, and `util`.
- `repository` depends on `domain`.
- `domain` and `util` have no semantic dependencies.
- `util` has two dependents: `api` and `service`.

The project deliberately includes:

- the named scalar `domain.ID` and the `domain.User` struct;
- the `repository.Repository` interface;
- value-receiver `MemoryRepository` and pointer-receiver
  `*PostgresRepository` implementations;
- concrete embedding, recursive promotion, pointer-only promotion, and
  explicit method shadowing;
- package-level functions with multiple callers;
- ordinary and ellipsis calls to a variadic function; and
- concrete values passed through an interface parameter.

The ambiguity edge case remains in the focused analyzer fixtures because it
would make this small application less immediately readable.

## Expected relationships

These facts are useful orientation points:

- `MemoryRepository` and `*PostgresRepository` implement `Repository`.
- Their `Save` and `Find` methods implement the corresponding interface
  methods.
- `Service` embeds `BaseService` and `AuditBase`.
- `ExtendedService` embeds `Service`, recursively promoting its methods.
- `AuditBase.Audit` is exposed only through `*Service` and
  `*ExtendedService` because `Service` embeds an `AuditBase` value.
- `CustomService` embeds `BaseService` but declares its own `Validate`, which
  shadows the promoted method.
- `Handler.CreateUser` calls `Service.CreateUser`.
- `Service.CreateUser` calls `Repository.Save` and `util.NormalizeName`.
- `api` has no dependents inside the example, while `util` is used by both
  `api` and `service`.

## Running NOCV

Run these commands from the NOCV repository root. The CLI detects the nested
module and loads `./...` with `examples/impactdemo` as the working directory.

```bash
go run ./cmd/nocv serve ./examples/impactdemo/...
```

The same pattern works with focused commands:

```bash
go run ./cmd/nocv tree ./examples/impactdemo/...
go run ./cmd/nocv implementations ./examples/impactdemo/...
go run ./cmd/nocv embeddings ./examples/impactdemo/...
```

Compile the example independently with:

```bash
cd examples/impactdemo
go test ./...
```

## Parameter-change scenarios

### A. Named scalar to primitive and interface contract loss

```bash
go run ./cmd/nocv impact-params ./examples/impactdemo/... \
  example.com/impactdemo/repository::Repository::Save \
  --param string
```

Expected: the calls from `Service.CreateUser` and `SaveWith` are incompatible,
and both current implementations lose the `Repository` contract.

### B. Add a service parameter

```bash
go run ./cmd/nocv impact-params ./examples/impactdemo/... \
  example.com/impactdemo/service::Service::CreateUser \
  --param domain.ID --param string --param bool
```

Expected: `Handler.CreateUser` is incompatible by argument count. Because
`ExtendedService` embeds `Service`, its pointer method surface also changes.

### C. Promoted method change and shadowing

```bash
go run ./cmd/nocv impact-params ./examples/impactdemo/... \
  example.com/impactdemo/service::BaseService::Validate \
  --param string
```

Expected: the `domain.ID` call in `Service.CreateUser` is incompatible, the
untyped string call in `Handler.ValidateGuest` remains compatible, and
`Service` plus `ExtendedService` have structural impacts. `CustomService` is
excluded because it declares its own `Validate`.

For the pointer-only promotion case:

```bash
go run ./cmd/nocv impact-params ./examples/impactdemo/... \
  example.com/impactdemo/service::AuditBase::Audit \
  --param string
```

Expected: structural results identify `*Service` and `*ExtendedService`.

### D. Interface method change

Scenario A selects the interface method itself. Its contract section should
report both current implementers as lost; structural impact remains empty.

### E. Unchanged proposal

```bash
go run ./cmd/nocv impact-params ./examples/impactdemo/... \
  example.com/impactdemo/service::BaseService::Validate \
  --param domain.ID
```

Expected: both existing calls are compatible, with no contract or structural
impact.

### F. Variadic to slice

```bash
go run ./cmd/nocv impact-params ./examples/impactdemo/... \
  example.com/impactdemo/util::JoinTags \
  --param '[]string'
```

Expected: the ordinary two-argument call has the wrong argument count, while
the ellipsis call is incompatible with the proposed non-variadic signature.

## Manual checklist

- [ ] package topology matches expectation
- [ ] interface implementations are visible
- [ ] promoted methods are visible
- [ ] parameter type change finds expected callers
- [ ] contract loss matches expectation
- [ ] pointer/value promotion matches expectation
- [ ] unchanged proposal has no false positives
- [ ] variadic calls behave correctly

This sandbox does not cover return/result changes, receiver changes, generic
callable parameter changes, transitive architectural impact, or source
rewriting.
