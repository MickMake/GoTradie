# CHATGPT_CONTEXT.md

## Purpose

`GoBunningsNinja` is the application/CLI that composes `GoBunnings` and `GoInvoiceNinja`.

It owns the workflow that turns Bunnings product data into safe Invoice Ninja product updates/imports/exports.

## Related repositories

```text
GoBunnings      -> reusable Bunnings API SDK
GoInvoiceNinja  -> reusable Invoice Ninja API SDK
GoBunningsNinja -> application/CLI that imports both SDKs
```

## Dependency rules

- `GoBunningsNinja` may import `GoBunnings`.
- `GoBunningsNinja` may import `GoInvoiceNinja`.
- `GoBunnings` must not import `GoInvoiceNinja` or `GoBunningsNinja`.
- `GoInvoiceNinja` must not import `GoBunnings` or `GoBunningsNinja`.
- Mapping logic belongs in `GoBunningsNinja`, preferably under `internal/mapper` or the relevant workflow package.

## Go version

Go `1.22`.

## Common commands

```bash
go test ./...
go vet ./...
go mod tidy
go build ./cmd/bunnings-ninja
```

## Local multi-repo development

Develop all three repos in one parent folder using a local `go.work` file:

```text
GoNinjaWorkspace/
├── go.work
├── GoBunnings/
├── GoInvoiceNinja/
└── GoBunningsNinja/
```

Create the workspace from the parent folder:

```bash
go work init ./GoBunnings ./GoInvoiceNinja ./GoBunningsNinja
go work sync
```

The `go.work` file is a local development convenience. The app module should not depend on absolute-path `replace` directives committed to `go.mod`.

## Application boundaries

Keep here:

- CLI commands
- configuration loading
- sync/import/export workflows
- Bunnings-to-Invoice Ninja mapping
- dry-run and safety guards

Avoid putting here:

- low-level Bunnings API transport logic
- low-level Invoice Ninja API transport logic
- reusable SDK code that belongs upstream in `GoBunnings` or `GoInvoiceNinja`
