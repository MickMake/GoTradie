# CHATGPT_CONTEXT.md

## Purpose

`GoTradie` is the application/CLI that composes `GoXero`, `GoBunnings` and `GoInvoiceNinja`.

It owns the workflow that turns Bunnings product data into safe Invoice Ninja product updates/imports/exports.

## Repository modules

```text
apps/GoTradie        -> application/CLI that imports the three SDKs
apps/GoBunnings      -> reusable Bunnings API SDK
apps/GoXero          -> reusable Xero API SDK
apps/GoInvoiceNinja  -> reusable Invoice Ninja API SDK
```

## Dependency rules

- `GoTradie` may import `GoBunnings`.
- `GoTradie` may import `GoInvoiceNinja`.
- `GoTradie` may import `GoXero`.
- `GoBunnings` must not import `GoInvoiceNinja` or `GoTradie`.
- `GoInvoiceNinja` must not import `GoBunnings` or `GoTradie`.
- `GoXero` must not import `GoBunnings`, `GoInvoiceNinja` or `GoTradie`.
- Mapping logic belongs in `GoTradie`, preferably under `internal/mapper` or the relevant workflow package.

## Go version

Go `1.25`.

## Common commands

```bash
go test ./...
go vet ./...
go mod tidy
go build ./cmd/GoTradie
```

## Monorepo development

This is one repository with four independent modules under `apps/`. Use the existing committed root `go.work` workspace:

```text
GoTradie/
├── go.work
└── apps/
    ├── GoBunnings/
    ├── GoInvoiceNinja/
    ├── GoTradie/
    └── GoXero/
```

Do not create separate sibling repositories or run `go work init`; the repository workspace already connects the modules.

## Application boundaries

Keep here:

- CLI commands
- configuration loading
- sync/import/export workflows
- Bunnings-to-Invoice Ninja mapping
- preview-by-default behaviour and `--commit` safety guards for persistent changes

Avoid putting here:

- low-level Bunnings API transport logic
- low-level Invoice Ninja API transport logic
- low-level Xero API transport logic
- reusable SDK code that belongs upstream in `GoBunnings`, `GoInvoiceNinja` or `GoXero`
