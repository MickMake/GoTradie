# GoTradie v0.5.4 — Implementation Prompt

**Objective:** Implement [BAS export and shared Accounting Dataset](./BAS-Export.md) as the authoritative feature design.

**Suggested branch:** `v0.5.4-bas-export`

## Mandatory preflight

Before modifying code:

1. Fetch the latest `origin/main` and inspect the current implementation, tests, and relevant module layout.
2. Check earlier v0.5 slice branches and PRs. Confirm they are merged or explicitly retired. Report any unresolved active predecessor and STOP.
3. Compare existing behaviour with the authoritative design. Identify missing source data, incompatible APIs, or design ambiguities; do not invent answers.
4. Present the proposed branch, affected packages/files, intended approach, and blockers. **STOP for approval before creating a branch or changing code.**

After approval, branch from the latest `origin/main`.

## Common delivery rules

- The linked design is authoritative for feature behaviour. Do not reproduce or reinterpret its configuration schema, CLI rules, financial calculations, or data model in this prompt.
- Follow the global [CLI contract](../../Command-Line-Spec.md) for `--commit`, `--force`, output naming, severity, and exit status.
- Preserve existing unrelated behaviour. Do not implement a later slice early.
- Add focused unit and integration tests for the design's acceptance conditions, including errors and boundary cases. Use deterministic fixtures and no live service dependency in automated tests.
- Run `gofmt`, `go vet`, `go test`, and `go build` **for every affected Go module**, using correct module-relative commands; report any untested hardware, API, or external-service paths.
- Maximum review/fix iterations: **3**. Report remaining issues instead of continuing indefinitely.
- Completion report: branch and commit, implementation summary, tests and commands/results, design deviations (if any), and known limitations.

## Slice-specific implementation work

- Inspect current Invoice Ninja invoices, customer payments, expenses, supplier transactions/settlements, configuration and XLSX libraries.
- Implement the minimum shared Accounting Dataset in this slice; keep source identity and accounting facts independent from workbook presentation.
- Build BAS export using that shared dataset and the authoritative selection, calculation, output and exception rules.
- Do not implement EOFY or Financial exports in this slice.

## Focused acceptance evidence

Use deterministic examples for period/FY selection, boundary dates, cash versus accrual recognition, partial customer receipts, partial supplier settlement and FIFO allocation, GST rounding, missing/ambiguous evidence, source traceability, workbook totals, incomplete-report status and overwrite safety.

**Design blocker:** The accepted BAS design still describes ATO due-date verification/cache and selection rules. Resolve that document against the agreed config-driven BAS/reporting/submission dates **before coding**. Do not implement both approaches or decide the selection rule inside this prompt.
