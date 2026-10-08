# GoTradie v0.5.3 — Implementation Prompt

**Objective:** Implement [Hierarchical configuration](./Hierarchical-Configuration.md) as the authoritative feature design.

**Suggested branch:** `v0.5.3-hierarchical-config`

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

- Inspect existing flat configuration consumers and their tests before replacing the loader.
- Update configuration parsing, validation, secret override handling, and consumers as specified by the design.
- Check that existing Invoice Ninja, Bunnings, tax and ERPNext paths still work under the replacement configuration.
- Update CLI help and affected application documentation; do not introduce backwards compatibility unless the design is formally amended.

## Focused acceptance evidence

Test mandatory-file handling; unknown/invalid fields; required accounting settings; valid hierarchy and provider mapping; defaults/YAML/secret precedence; rejection of generic environment overrides; and migration failures that should be explicit rather than silent.

**Dependency:** The accepted configuration design currently specifies ATO due-date verification. Our proposed change to configured BAS/submission dates must be made in the **design** before this slice is implemented.
