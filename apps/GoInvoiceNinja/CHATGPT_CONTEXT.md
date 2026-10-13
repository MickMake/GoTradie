# CHATGPT_CONTEXT.md

## Purpose

`GoInvoiceNinja` is the reusable Go SDK for Invoice Ninja v5 API access.

It should stay focused on Invoice Ninja authentication, typed clients/services, request/response models, pagination helpers, upload helpers, and API error handling.

## Role in the wider workspace

`GoInvoiceNinja` is consumed by `GoTradie`.

```text
GoTradie        -> application/CLI that imports both SDKs
GoBunnings      -> reusable Bunnings API SDK
GoXero          -> reusable Xero API SDK
GoInvoiceNinja  -> reusable Invoice Ninja API SDK
```

## Dependency rules

- `GoInvoiceNinja` must not import `GoTradie`.
- `GoInvoiceNinja` must not import `GoBunnings`.
- `GoInvoiceNinja` must not import `GoXero`.
- Keep Bunnings-specific logic out of this package.
- Bunnings-to-Invoice Ninja conversion belongs in `GoTradie`.

## Go version

Go `1.25`.

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

When developing this repo together with `GoTradie`, `GoXero` and `GoBunnings`, use a parent folder with a local `go.work` file. Do not commit local absolute-path `replace` directives to this repo.
