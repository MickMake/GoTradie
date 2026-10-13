# Repository architecture

This repository contains three independent Go modules.

Modules:
- apps/GoTradie
- apps/GoBunnings
- apps/GoInvoiceNinja
- apps/GoXero

Allowed dependencies:
- GoTradie -> GoBunnings
- GoTradie -> GoInvoiceNinja
- GoTradie -> GoXero

Forbidden dependencies:
- GoInvoiceNinja -> GoTradie
- GoBunnings -> GoTradie
- GoXero -> GoTradie
- GoBunnings <-> GoInvoiceNinja
- GoInvoiceNinja <-> GoXero
- GoXero <-> GoBunnings

Rules:
- Preserve module boundaries.
- Do not refactor across module boundaries unless explicitly instructed.
- Do not move code between modules unless explicitly instructed.
- Do not introduce new cross-module imports unless explicitly instructed.
- Each module has its own AGENTS.md containing local operational rules.
- Run tests separately for each changed module.
- Summarise changes module-by-module.
- Use Go 1.25.
- Implementation work must follow docs/Implementation-Workflow.md.
- For v0.5.x releases, read the relevant docs/0.5/0.5.x/Implementation-Prompt.md and its linked authoritative design before making changes.

Repository intent:
- GoTradie is the primary application/orchestration module.
- GoXero, GoBunnings and GoInvoiceNinja are standalone reusable modules.
