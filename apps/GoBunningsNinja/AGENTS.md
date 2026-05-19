# GoBunningsNinja module

This is the primary application module.

This module MAY depend on:
- apps/GoBunnings
- apps/GoInvoiceNinja

This module coordinates the overall workflow between the other modules.

Rules:
- Do not move shared logic into this module unless explicitly instructed.
- Prefer keeping reusable logic inside the originating module.
- Do not refactor across module boundaries unless explicitly instructed.
- Preserve module boundaries.
- Keep orchestration/application logic here.
- Keep reusable library logic in the independent modules.
- Before changing interfaces used by other modules, inspect the affected module carefully.

Forbidden:
- Do not merge modules together.
- Do not consolidate unrelated packages for "consistency."
- Do not introduce circular dependencies.
- Do not copy large blocks of code between modules.

Commands:
- go test ./...
- go build ./...

## CLI safety rules

Commands preview or refuse risky writes by default.

Use --commit to make persistent changes.

Do not add:
- --dry-run
- --apply
- --force

--commit is the only flag that allows:
- Invoice Ninja writes
- local file overwrites

## Bunnings source selection

Default Bunnings data source is the Bunnings API.

--web means:
Use the public Bunnings website instead of the Bunnings API.

--web must not:
- imply --commit
- silently fall back to the API
- modify Invoice Ninja
- change output shape
- make Invoice Ninja credentials required for Bunnings-only commands

If --web is supplied and required website data cannot be retrieved, fail clearly.

Commands that should support --web:
- bunnings get <IN...>
- bunnings lookup <IN...>
- bunnings find <query>
- sync refresh
- sync import <IN>
- sync search <query>

Commands that should not support --web:
- ninja export ...
- ninja import ...
- version

## Output compatibility

When --web is used, output shape must remain the same as the API-backed command.

bunnings get and bunnings find must keep the CSV output shape.

bunnings lookup must keep the human-readable detail output shape.

sync commands must keep the same preview/commit result output shape.

