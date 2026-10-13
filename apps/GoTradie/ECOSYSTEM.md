# GoBunnings Ninja Ecosystem

This ecosystem is made of four separate Go modules that are developed together locally.

## Repositories

| Repository       | Role                                    | Imports                                             |
|------------------|-----------------------------------------|-----------------------------------------------------|
| `GoBunnings`     | Reusable Bunnings API SDK               | Standard library and external SDK dependencies only |
| `GoInvoiceNinja` | Reusable Invoice Ninja v5 API SDK       | Standard library and external SDK dependencies only |
| `GoXero`         | Reusable Xero API SDK                   | Standard library and external SDK dependencies only |
| `GoTradie`       | CLI/application that composes both SDKs | `GoBunnings`, `GoInvoiceNinja`, `GoXero`            |

## Dependency direction

```text
GoBunnings      ┐
GoXero         -├── GoTradie
GoInvoiceNinja  ┘
```

Rules:

- `GoBunnings`, `GoInvoiceNinja` & `GoXero` must NOT import each other.
- `GoBunnings`, `GoInvoiceNinja` & `GoXero` should NOT import `GoTradie`.
- `GoTradie` may import `GoBunnings`, `GoInvoiceNinja` & `GoXero` APIs.
- Bunnings-to-Invoice Ninja mapping belongs in `GoTradie`.

## Local development layout

Use one parent folder and a local Go workspace:

```text
GoNinjaWorkspace/
├── go.work
├── GoBunnings/
├── GoInvoiceNinja/
├── GoXero/
└── GoTradie/
```

From the parent folder:

```bash
git clone git@github.com:MickMake/GoTradie.git

go work init ./GoBunnings ./GoInvoiceNinja ./GoTradie ./GoXero
go work sync
```

Expected `go.work` shape:

```go
go 1.25

use (
    ./GoBunnings
    ./GoInvoiceNinja
	./GoXero
    ./GoTradie
)
```

`go.work` is intentionally local to the workspace. It should normally not be committed to any individual repo.

## Release order

When SDK changes are needed, release in dependency order:

1. `GoBunnings`, when Bunnings SDK changes are required.
2. `GoInvoiceNinja`, when Invoice Ninja SDK changes are required.
3. `GoXero`, when Xero SDK changes are required.
4. `GoTradie`, after updating its `go.mod` to the tagged SDK versions.

During active development, the local `go.work` file lets `GoTradie` use local SDK checkouts without committed absolute-path `replace` directives.

## Command checklist

Run from each repo as relevant:

```bash
go test ./...
go vet ./...
go mod tidy
```

For the app repo:

```bash
go build ./cmd/GoTradie
```

## Boundary checklist

Before adding new code, ask:

- Is this about Bunnings API behaviour? Put it in `GoBunnings`.
- Is this about Invoice Ninja API behaviour? Put it in `GoInvoiceNinja`.
- Is this about Xero API behaviour? Put it in `GoXero`.
- Is this about converting, syncing, importing, exporting, CLI behaviour, config, or safety guards? Put it in `GoTradie`.

If a package starts knowing too much about another package's private business, split it before it develops a hat and starts calling meetings.
