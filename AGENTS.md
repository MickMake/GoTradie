# Repository architecture

This repository contains three independent Go modules.

Modules:
- apps/GoTradie
- apps/GoBunnings
- apps/GoInvoiceNinja

Allowed dependencies:
- GoTradie -> GoBunnings
- GoTradie -> GoInvoiceNinja

Forbidden dependencies:
- GoBunnings -> GoTradie
- GoInvoiceNinja -> GoTradie
- GoBunnings <-> GoInvoiceNinja

Rules:
- Preserve module boundaries.
- Do not refactor across module boundaries unless explicitly instructed.
- Do not move code between modules unless explicitly instructed.
- Do not introduce new cross-module imports unless explicitly instructed.
- Each module has its own AGENTS.md containing local operational rules.
- Run tests separately for each changed module.
- Summarise changes module-by-module.
- Use Go 1.22.

Repository intent:
- GoTradie is the primary application/orchestration module.
- GoBunnings and GoInvoiceNinja are standalone reusable modules.
