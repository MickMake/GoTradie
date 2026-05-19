# MickMake Go Workspace Context

## Overview

This Project is for a monorepo Go 1.22 software ecosystem under the `MickMake` GitHub account.

The current repository is:

- `MickMake/GoBunningsNinja`

This repository currently contains three separate Go modules:

- `apps/GoBunnings`
- `apps/GoInvoiceNinja`
- `apps/GoBunningsNinja`

These modules are kept separate inside one repository so Codex only needs to clone and work with a single GitHub repository.

Later, these modules may be split into separate repositories:

- `MickMake/GoBunnings`
- `MickMake/GoInvoiceNinja`
- `MickMake/GoBunningsNinja`

For now, **do not clone `GoBunnings` or `GoInvoiceNinja` as separate GitHub repositories**. They do not exist as separate repositories in the current working model.

The current source of truth is the monorepo:

```text
MickMake/GoBunningsNinja

