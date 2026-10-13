# GoTradie Ecosystem

This repository contains four independent Go modules that are developed together through the committed root `go.work` workspace.

## Modules

| Module                 | Role                                     | Imports                                             |
|------------------------|------------------------------------------|-----------------------------------------------------|
| `apps/GoBunnings`      | Reusable Bunnings API SDK                | Standard library and external SDK dependencies only |
| `apps/GoInvoiceNinja`  | Reusable Invoice Ninja v5 API SDK        | Standard library and external SDK dependencies only |
| `apps/GoXero`          | Reusable Xero API SDK                    | Standard library and external SDK dependencies only |
| `apps/GoTradie`        | CLI/application that composes three SDKs | `GoBunnings`, `GoInvoiceNinja`, `GoXero`            |

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

## Repository layout

Use the existing workspace at the repository root:

```text
GoTradie/
├── go.work
└── apps/
    ├── GoBunnings/
    ├── GoInvoiceNinja/
    ├── GoTradie/
    └── GoXero/
```

Expected `go.work` shape:

```go
go 1.25

use (
    ./apps/GoBunnings
    ./apps/GoInvoiceNinja
    ./apps/GoTradie
    ./apps/GoXero
)
```

The root `go.work` is part of this repository. Do not run `go work init` or create separate sibling checkouts for these modules.

## Release order

When SDK changes are needed, release in dependency order:

1. `GoBunnings`, when Bunnings SDK changes are required.
2. `GoInvoiceNinja`, when Invoice Ninja SDK changes are required.
3. `GoXero`, when Xero SDK changes are required.
4. `GoTradie`, after any required SDK module changes.

During active development, the root `go.work` lets `GoTradie` use the other modules directly from this checkout.

## Command checklist

Run from each module directory as relevant:

```bash
go test ./...
go vet ./...
go mod tidy
```

For the application module:

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
