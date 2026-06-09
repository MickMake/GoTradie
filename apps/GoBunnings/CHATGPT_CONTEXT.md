# CHATGPT_CONTEXT.md

## Purpose

`GoBunnings` is the reusable Go SDK for Bunnings API access.

It should stay focused on Bunnings API transport, authentication, typed requests/responses, error handling, pagination/query helpers, and related domain types.

## Role in the wider workspace

`GoBunnings` is consumed by `GoTradie`.

```text
GoBunnings      -> reusable Bunnings API SDK
GoInvoiceNinja  -> reusable Invoice Ninja API SDK
GoTradie -> application/CLI that imports both SDKs
```

## Dependency rules

- `GoBunnings` must not import `GoTradie`.
- `GoBunnings` must not import `GoInvoiceNinja`.
- Keep business workflow and mapping logic out of this package.
- Bunnings-to-Invoice Ninja conversion belongs in `GoTradie`.

## Go version

Go `1.22`.

## Common commands

```bash
go test ./...
go vet ./...
go mod tidy
```

## Public API guidance

Prefer:

- context-aware functions
- typed request and response structs
- stable exported types
- clear API errors
- small composable service wrappers

Avoid:

- CLI-specific logic
- Invoice Ninja concepts
- job-costing or estimating rules
- hidden global state
- local filesystem assumptions

## Development workflow

When developing this repo together with `GoInvoiceNinja` and `GoTradie`, use a parent folder with a local `go.work` file. Do not commit local absolute-path `replace` directives to this repo.
