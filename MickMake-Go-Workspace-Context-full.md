# MickMake Go Workspace Context

## Overview

This Project is for a three-repo Go 1.22 software ecosystem under the `MickMake` GitHub account.

The three repositories are:

- `MickMake/GoBunnings`
- `MickMake/GoInvoiceNinja`
- `MickMake/GoBunningsNinja`

Repository references are listed in `GitHub-references.txt`.

## Purpose of Each Repository

### GoBunnings

`GoBunnings` is a reusable Go SDK for Bunnings API access.

It owns:

- Bunnings API authentication
- Bunnings API transport/client logic
- Bunnings request and response types
- Bunnings product, pricing, inventory, location, and order API models
- Bunnings-specific parsing/helpers
- Bunnings API error handling
- reusable Bunnings SDK behaviour

It must not import:

- `GoInvoiceNinja`
- `GoBunningsNinja`

It should stay reusable and boring. Boring software is good software; exciting software is usually holding a mop and looking guilty.

---

### GoInvoiceNinja

`GoInvoiceNinja` is a reusable Go SDK for Invoice Ninja v5 API access.

It owns:

- Invoice Ninja API authentication
- Invoice Ninja API transport/client logic
- Invoice Ninja request and response types
- services for clients, products, quotes, invoices, and payments
- pagination helpers
- upload helpers
- Invoice Ninja API error handling
- reusable Invoice Ninja SDK behaviour

It must not import:

- `GoBunnings`
- `GoBunningsNinja`

It should not contain Bunnings-specific logic.

---

### GoBunningsNinja

`GoBunningsNinja` is the CLI/application that imports both SDKs.

It owns:

- CLI commands
- application configuration
- sync workflows
- import/export workflows
- dry-run/safety logic
- Bunnings-to-Invoice Ninja mapping
- orchestration between `GoBunnings` and `GoInvoiceNinja`
- user-facing workflow behaviour

It may import:

- `GoBunnings`
- `GoInvoiceNinja`

It should not contain low-level Bunnings API client logic or low-level Invoice Ninja API client logic unless that code is purely application-specific glue.

## Dependency Direction

The dependency direction is:

```text
GoBunnings      ->
                  GoBunningsNinja
GoInvoiceNinja  ->
```

Rules:

- `GoBunningsNinja` may import both SDKs.
- `GoBunnings` must not import `GoInvoiceNinja`.
- `GoInvoiceNinja` must not import `GoBunnings`.
- Neither SDK should import `GoBunningsNinja`.
- Do not introduce circular dependencies.
- Cross-system mapping belongs in `GoBunningsNinja`.

## Local Development Setup

The user has all three repos cloned locally side by side in a Go workspace.

Typical local structure:

```text
GoNinjaWorkspace/
├── go.work
├── GoBunnings/
├── GoInvoiceNinja/
└── GoBunningsNinja/
```

The workspace uses Go `1.22`.

Typical `go.work` shape:

```go
go 1.22

use (
    ./GoBunnings
    ./GoInvoiceNinja
    ./GoBunningsNinja
)
```

`GoBunningsNinja` may use relative local `replace` directives for simple local development, for example:

```go
replace github.com/MickMake/GoBunnings => ../GoBunnings
replace github.com/MickMake/GoInvoiceNinja => ../GoInvoiceNinja
```

## User Workflow Preference

The user is the only developer.

The user prefers:

- direct commits to `main`
- no branches unless absolutely necessary
- no pull requests unless explicitly requested
- small, clear changes
- practical explanations
- exact local commands after GitHub edits
- honesty about what was and was not tested

When making meaningful changes, update `CHANGES.md`.

## ChatGPT Workflow

When the user asks for a change:

1. Inspect the relevant GitHub repositories.
2. Check repo context files where available:
   - `CHATGPT_CONTEXT.md`
   - `ECOSYSTEM.md`
   - `CHANGES.md`
   - `README.md`
3. Decide which repository owns the change.
4. Keep SDK logic in the SDK repositories.
5. Keep workflow/mapping/CLI logic in `GoBunningsNinja`.
6. Make the smallest safe change.
7. Commit directly to `main` if safe and requested.
8. Update `CHANGES.md` for meaningful changes.
9. Tell the user exactly what to run locally.

## Local Commands to Give the User After Changes

For changes in all three repos:

```bash
cd /path/to/GoNinjaWorkspace

cd GoBunnings
git pull
go test ./...
go vet ./...

cd ../GoInvoiceNinja
git pull
go test ./...
go vet ./...

cd ../GoBunningsNinja
git pull
go test ./...
go vet ./...
go build ./cmd/bunnings-ninja
```

For changes only in `GoBunningsNinja`:

```bash
cd /path/to/GoNinjaWorkspace/GoBunningsNinja
git pull
go test ./...
go vet ./...
go build ./cmd/bunnings-ninja
```

If the user reports failures, inspect the error output and fix the relevant repo directly.

## Design Rules

Use these ownership rules:

| Type of change | Repository |
|---|---|
| Bunnings API authentication/client logic | `GoBunnings` |
| Bunnings API request/response models | `GoBunnings` |
| Bunnings product/search/pricing/inventory helpers | `GoBunnings` |
| Invoice Ninja API authentication/client logic | `GoInvoiceNinja` |
| Invoice Ninja clients/products/quotes/invoices/payments models | `GoInvoiceNinja` |
| Invoice Ninja pagination/upload/error helpers | `GoInvoiceNinja` |
| CLI commands | `GoBunningsNinja` |
| Config loading | `GoBunningsNinja` |
| Dry-run behaviour | `GoBunningsNinja` |
| Import/export workflows | `GoBunningsNinja` |
| Bunnings-to-Invoice Ninja mapping | `GoBunningsNinja` |
| Cross-system sync orchestration | `GoBunningsNinja` |

## Versioning and Change Logs

Preserve and update existing `CHANGES.md` files.

The user has previously used version-style entries such as:

- `v0.1`
- `v0.2`
- `v0.3`
- `v0.4`

When a change is meaningful, add a new entry or update the current unreleased/latest entry as appropriate.

Do not invent test results. Only say tests passed if they were actually run and passed.

## Communication Style

Be concise and practical.

The user prefers:

- clear summaries
- exact commands
- direct explanations
- no excessive theory
- enough humour to keep the goblins quiet

Avoid making the workflow sound more complicated than it needs to be.

The simple model is:

```text
User asks for one clear change
↓
Assistant inspects GitHub
↓
Assistant edits the right repo
↓
Assistant commits to main
↓
User pulls locally and runs tests
↓
Assistant fixes any errors
```

## Standard Starting Prompt for Future Chats

If context is lost, assume this:

The user is working on a three-repo Go 1.22 project under `MickMake`:

- `GoBunnings`: reusable Bunnings API SDK
- `GoInvoiceNinja`: reusable Invoice Ninja v5 API SDK
- `GoBunningsNinja`: CLI/app that imports both

Keep SDK code in the SDK repos. Keep mapping, sync, import/export, config, dry-run safety, and CLI workflows in `GoBunningsNinja`.

The user prefers direct commits to `main`, with `CHANGES.md` updated for meaningful changes.

After editing GitHub, provide local pull/test/build commands.
