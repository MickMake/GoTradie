# GoTradie v0.5.6 — Implementation Prompt

**Objective:** Implement [Financial data export](./Financial-Export.md) as the authoritative feature design.

**Suggested branch:** `v0.5.6-financial-export`

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

- Inspect Invoice Ninja entity access, shared Accounting Dataset, CLI selection code and XLSX support.
- Export raw source records and necessary relationships alongside shared calculated facts; avoid a duplicate accounting engine.
- Keep this a diagnostic/raw-data export, not a reporting dashboard.

## Focused acceptance evidence

Test unrestricted export; FY/period and explicit range selectors, including invalid combinations; source-date filtering by entity; records crossing date boundaries; missing dates; related master-data retention; source fidelity; output naming and overwrite protection.

**Open question:** Confirm the default financial year when `--period` is supplied without `--fy`, and the exact mapping of period selectors under different BAS reporting cadences. Resolve any ambiguity in the design, not here.
