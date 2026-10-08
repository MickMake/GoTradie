# GoTradie v0.5.5 — Implementation Prompt

**Objective:** Implement [EOFY export](./EOFY-Export.md) as the authoritative feature design.

**Suggested branch:** `v0.5.5-eofy-export`

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

- Inspect the v0.5.4 Accounting Dataset, Invoice Ninja expense categories and available asset evidence.
- Reuse shared accounting facts rather than building a second recognition or allocation engine.
- Generate the accountant-review workbook as specified in the design. Do not implement depreciation or pooling.

## Focused acceptance evidence

Test independent BAS/EOFY accounting bases; selected financial year; source/category traceability; no/full/partial GST credit entitlement; business-use treatment; first-used date and threshold boundary; exception severity; deterministic filenames and overwrite safety.

**Open question:** The design does not fully specify how to classify an asset when first-used date, GST entitlement or business-use evidence is unavailable. Flag it in preflight rather than inventing values or eligibility.
