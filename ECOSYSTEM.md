# GoBunnings Ninja Ecosystem

This ecosystem is made of three separate Go modules that are developed together locally.

## Repositories

| Repository | Role | Imports |
|---|---|---|
| `GoBunnings` | Reusable Bunnings API SDK | Standard library and external SDK dependencies only |
| `GoInvoiceNinja` | Reusable Invoice Ninja v5 API SDK | Standard library and external SDK dependencies only |
| `GoBunningsNinja` | CLI/application that composes both SDKs | `GoBunnings`, `GoInvoiceNinja` |

## Dependency direction

```text
GoBunnings      ┐
                ├── GoBunningsNinja
GoInvoiceNinja  ┘
```

Rules:

- `GoBunningsNinja` may import both SDKs.
- `GoBunnings` must not import `GoInvoiceNinja`.
- `GoInvoiceNinja` must not import `GoBunnings`.
- Neither SDK should import `GoBunningsNinja`.
- Bunnings-to-Invoice Ninja mapping belongs in `GoBunningsNinja`.

## Local development layout

Use one parent folder and a local Go workspace:

```text
GoNinjaWorkspace/
├── go.work
├── GoBunnings/
├── GoInvoiceNinja/
└── GoBunningsNinja/
```

From the parent folder:

```bash
git clone git@github.com:MickMake/GoBunnings.git
git clone git@github.com:MickMake/GoInvoiceNinja.git
git clone git@github.com:MickMake/GoBunningsNinja.git

go work init ./GoBunnings ./GoInvoiceNinja ./GoBunningsNinja
go work sync
```

Expected `go.work` shape:

```go
go 1.22

use (
    ./GoBunnings
    ./GoInvoiceNinja
    ./GoBunningsNinja
)
```

`go.work` is intentionally local to the workspace. It should normally not be committed to any individual repo.

## Release order

When SDK changes are needed, release in dependency order:

1. `GoBunnings`, when Bunnings SDK changes are required.
2. `GoInvoiceNinja`, when Invoice Ninja SDK changes are required.
3. `GoBunningsNinja`, after updating its `go.mod` to the tagged SDK versions.

During active development, the local `go.work` file lets `GoBunningsNinja` use local SDK checkouts without committed absolute-path `replace` directives.

## Command checklist

Run from each repo as relevant:

```bash
go test ./...
go vet ./...
go mod tidy
```

For the app repo:

```bash
go build ./cmd/bunnings-ninja
```

## Boundary checklist

Before adding new code, ask:

- Is this about Bunnings API behaviour? Put it in `GoBunnings`.
- Is this about Invoice Ninja API behaviour? Put it in `GoInvoiceNinja`.
- Is this about converting, syncing, importing, exporting, CLI behaviour, config, or safety guards? Put it in `GoBunningsNinja`.

If a package starts knowing too much about another package's private business, split it before it develops a hat and starts calling meetings.
