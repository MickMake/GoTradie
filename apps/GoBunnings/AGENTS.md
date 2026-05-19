# GoBunnings module

This module is an independent reusable library/application.

This module MUST remain standalone.

Forbidden imports:
- apps/GoBunningsNinja
- apps/GoInvoiceNinja

Rules:
- Bunnings website lookup/search helpers belong in this module when they are reusable.
- Website-backed retrieval must stay Bunnings-specific and must not depend on GoBunningsNinja.
- Prefer normal HTTP plus structured-data/HTML parsing before browser automation.
- If browser automation is unavoidable, isolate it behind a small API so callers do not care how web data is retrieved.
- Website-backed lookup/search must fail clearly when required product data cannot be retrieved.
- Do not refactor across module boundaries unless explicitly instructed.
- Preserve module independence.
- Keep reusable Bunnings logic inside this module.
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
