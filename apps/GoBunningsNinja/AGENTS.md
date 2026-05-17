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
