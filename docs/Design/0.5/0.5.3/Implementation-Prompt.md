# GoTradie v0.5.3 — Implementation Prompt

## Objective

Implement the accepted v0.5.3 hierarchical configuration design.

Primary design contract:

```text
docs/Design/0.5/0.5.3/Hierarchical-Configuration.md
```

Cross-audit contract:

```text
docs/Design/0.5/Cross-Audit.md
```

Series context:

```text
docs/Design/0.5/README.md
```

Cross-release CLI contract:

```text
docs/Design/Command-Line-Spec.md
```

Where the older CLI/config documentation conflicts with the accepted v0.5.3 configuration-precedence decision, v0.5.3 intentionally changes that behaviour and the affected documentation must be updated consistently.

## Mandatory preflight

Before making any code changes:

1. Fetch the latest `origin/main`.
2. Verify that all earlier slice branches and pull requests are merged into `main`.
3. If any earlier slice branch or PR is not merged, STOP and report it.
4. Inspect the current configuration implementation and tests.
5. State the intended change, proposed branch name, files/packages likely affected, and any design ambiguity discovered.
6. STOP and wait for approval before creating the branch or modifying code.

Suggested branch:

```text
v0.5.3-hierarchical-config
```

After approval, branch from latest `origin/main` only.

## Required implementation

Implement hierarchical YAML configuration while preserving current behaviour except where the accepted design explicitly changes it.

The implementation must:

- add YAML configuration support;
- represent configuration hierarchically;
- make environment variables override file values;
- preserve existing flat config support for this transition release;
- keep precedence deterministic and documented;
- preserve current Invoice Ninja, Bunnings, tax and ERPNext behaviour except where explicitly changed;
- provide structural homes for `invoice_ninja`, `tax`, `bas`, `eofy`, `product_sync`, and `providers`;
- support Provider canonical `name` plus `aliases`;
- keep Provider configuration declarative;
- avoid adding provider-fetch logic except where minimally required to validate parsing.

## Provider configuration contract

Provider configuration must support one canonical name and multiple accepted aliases.

Do not implement fuzzy supplier matching or automatic alias learning in this slice.

Store/location must not be encoded into the canonical Provider name.

## Compatibility rules

For v0.5.3:

- YAML is preferred.
- Legacy flat config remains readable.
- Environment variables override file-based values.
- Existing valid flat configuration must not silently change meaning except for the accepted precedence rule.
- Multiple config-source precedence must be deterministic, tested and documented.
- Legacy support removal is explicitly out of scope.

Do not build a generic migration framework.

## Scope exclusions

Do not implement BAS, Accounting Dataset, EOFY, Financial, redesigned Product Sync, NST fetching, generic CSV fetching, provider lifecycle logic, or speculative config features.

## Tests

Add or update tests covering at least:

- valid YAML parsing;
- hierarchical field mapping;
- environment-over-file precedence;
- legacy flat config compatibility;
- deterministic precedence where multiple sources are present;
- Provider canonical name and aliases;
- `eofy` configuration parsing;
- validation failures;
- preservation of existing defaults.

## Documentation

Update affected documentation together with the implementation.

At minimum inspect:

```text
docs/Design/0.5/0.5.3/Hierarchical-Configuration.md
docs/Design/0.5/Cross-Audit.md
docs/Design/0.5/README.md
docs/Design/Command-Line-Spec.md
apps/GoTradie/README.md
apps/GoTradie/internal/app/extended_help.go
apps/GoTradie/internal/app/app.go
CHANGES.md
```

Do not finalise BAS/EOFY/Financial local-file overwrite semantics in this slice. That remains explicitly unresolved.

## Verification

Before declaring the slice complete:

```text
gofmt
go vet
go test
go build
```

Maximum review/fix loops: **3**.

## Completion report

Report branch, files changed, config precedence, legacy compatibility, Provider config behaviour, tests, verification results, deferred issues, and PR readiness.
