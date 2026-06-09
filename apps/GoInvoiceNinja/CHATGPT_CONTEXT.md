# CHATGPT_CONTEXT.md

## Purpose

`GoInvoiceNinja` is the reusable Go SDK for Invoice Ninja v5 API access.

It should stay focused on Invoice Ninja authentication, typed clients/services, request/response models, pagination helpers, upload helpers, and API error handling.

## Role in the wider workspace

`GoInvoiceNinja` is consumed by `GoTradie`.

```text
GoBunnings      -> reusable Bunnings API SDK
GoInvoiceNinja  -> reusable Invoice Ninja API SDK
GoTradie -> application/CLI that imports both SDKs
```

## Dependency rules

- `GoInvoiceNinja` must not import `GoTradie`.
- `GoInvoiceNinja` must not import `GoBunnings`.
- Keep Bunnings-specific logic out of this package.
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
- clear validation/API errors
- all-page helpers for paginated endpoints
- `Raw` escape hatches for unsupported endpoints

Avoid:

- Bunnings concepts
- CLI-specific behaviour
- job-costing or estimating rules
- hidden global state
- local filesystem assumptions

## Development workflow

When developing this repo together with `GoBunnings` and `GoTradie`, use a parent folder with a local `go.work` file. Do not commit local absolute-path `replace` directives to this repo.
