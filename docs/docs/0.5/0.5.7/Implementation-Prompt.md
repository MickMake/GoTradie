# GoTradie v0.5.7 — Implementation Prompt

**Objective:** Implement [Product synchronisation](./Product-Sync.md) as the authoritative feature design.

**Suggested branch:** `v0.5.7-product-sync`

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

- Inspect current Product Sync and Bunnings error/availability handling before redesigning it.
- Verify the actual Invoice Ninja Product custom-field API representation and safe read/write behaviour.
- Inspect the actual NST CSV and source metadata before assuming field names or parsing rules.
- Implement Provider adapters, existing-product refresh, evidence-based discovery and narrow source-fingerprint caching to the design; do not create a durable parallel product catalogue.
- Preserve preview-by-default behaviour and avoid unrelated historical Vendor identity migration.

## Focused acceptance evidence

Test preview versus `--commit`; all configured Providers; canonical Supplier/Product matching, aliases and Store separation; ordering and per-product successful sync dates; known availability versus unknown/error; file fetch/parse once; content-hash change/unchanged handling; source errors and retry; and duplicate prevention.

**Open question:** The design does not clearly define whether a changed file's successful fingerprint is committed only after all affected Products are processed. Flag partial-failure/retry semantics before implementing the cache to prevent skipped Products.
