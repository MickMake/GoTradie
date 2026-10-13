# MickMake Go Workspace Context

## Overview

This Project is for a monorepo Go 1.25 software ecosystem under the `MickMake` GitHub account.

The current repository is:

- `MickMake/GoTradie`

This repository currently contains three separate Go modules:

- `apps/GoBunnings`
- `apps/GoInvoiceNinja`
- `apps/GoXero`
- `apps/GoTradie`

These modules are kept separate inside one repository so Codex only needs to clone and work with a single GitHub repository.

Later, these modules may be split into separate repositories:

- `MickMake/GoBunnings`
- `MickMake/GoInvoiceNinja`
- `MickMake/GoXero`
- `MickMake/GoTradie`

For now, **do not clone `GoXero`, ```GoBunnings` or `GoInvoiceNinja` as separate GitHub repositories**. They do not exist as separate repositories in the current working model.

The current source of truth is the monorepo:

```text
MickMake/GoTradie

