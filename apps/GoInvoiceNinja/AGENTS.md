# GoInvoiceNinja module

This module is an independent reusable library/application.

This module MUST remain standalone.

Forbidden imports:
- apps/GoTradie
- apps/GoBunnings

Rules:
- Do not refactor across module boundaries unless explicitly instructed.
- Preserve module independence.
- Keep reusable Invoice Ninja logic inside this module.
- Do not move application orchestration logic here.
- Do not add dependencies on the main application module.
- Preserve public APIs unless explicitly instructed otherwise.
- Avoid introducing unnecessary dependencies.
- Prefer small focused changes.
- Do not add app-specific logic.

Forbidden:
- Do not merge modules together.
- Do not consolidate packages for "consistency."
- Do not introduce circular dependencies.
- Do not copy large blocks of code between modules.
- Do not move shared logic into unrelated modules.

Commands:
- go test ./...
- go build ./...
