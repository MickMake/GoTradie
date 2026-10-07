# GoTradie v0.5.3 — Implementation Prompt

## Objective

Implement the accepted v0.5.3 hierarchical configuration design.

Primary design contract:

```text
docs/Design/0.5/0.5.3/Hierarchical-Configuration.md
```

Series context:

```text
docs/Design/0.5/README.md
```

Cross-release CLI contract:

```text
docs/Design/Command-Line-Spec.md
```

Do not reinterpret or extend those contracts without explicit approval.

## Mandatory preflight

Before making any code changes:

1. Fetch the latest `origin/main`.
2. Verify that all earlier slice branches and pull requests are merged into `main`.
3. If any earlier slice branch or PR is not merged, STOP and report it.
4. Inspect the current configuration implementation and tests.
5. State the intended change, proposed branch name, files/packages likely affected, and any design ambiguity discovered.
6. STOP and wait for approval before creating the branch or modifying code.

Suggested branch name:

```text
v0.5.3-hierarchical-config
```

After approval, branch from the latest `origin/main` only.

## Required implementation

Implement hierarchical YAML configuration while preserving current behaviour.

The implementation must:

- add YAML configuration support;
- represent configuration hierarchically;
- preserve current environment-variable overrides;
- preserve existing flat config support for this transition release;
- keep precedence deterministic and documented;
- preserve current Invoice Ninja, Bunnings, tax and ERPNext configuration behaviour unless the design explicitly changes it;
- provide the structural homes required for `bas`, `product_sync`, and `providers`;
- keep provider configuration declarative;
- avoid adding provider-fetch logic except where minimally required to validate configuration parsing.

The new configuration must support the structure defined in the design contract.

## Compatibility rules

For v0.5.3:

- YAML is the preferred format.
- Legacy flat config remains readable.
- Environment variables continue to override file-based values where already supported.
- Existing valid flat configuration must not silently change meaning.
- If both legacy and YAML inputs can be provided together, precedence must be deterministic, tested and documented.
- Legacy support removal is explicitly out of scope.

Do not build a generic migration framework.

## Scope exclusions

Do not implement:

- BAS export;
- Accounting Dataset;
- EOFY export;
- Financial export;
- redesigned product sync;
- North Shore Timber fetching;
- generic CSV catalogue fetching;
- provider lifecycle logic;
- speculative configuration features not required by the design.

If implementation pressure suggests adding any of the above, STOP and report why.

## Tests

Add or update tests covering at least:

- valid YAML parsing;
- hierarchical field mapping;
- environment override precedence;
- legacy flat config compatibility;
- deterministic precedence where multiple sources are present;
- validation failures;
- unknown/unsupported configuration where applicable;
- preservation of existing default behaviour.

Prefer focused tests over broad fixtures.

## Documentation

Update affected documentation together with the implementation.

At minimum inspect:

```text
docs/Design/0.5/0.5.3/Hierarchical-Configuration.md
docs/Design/0.5/README.md
apps/GoTradie/README.md
CHANGES.md
```

Only change the accepted design if implementation reveals a real contradiction or missing decision. If so, STOP and request approval before changing the design contract.

## Verification

Before declaring the slice complete:

```text
gofmt
go vet
go test
go build
```

Run them across the relevant module(s).

Then perform an evidence-backed review of the change.

Maximum review/fix loops: **3**.

If significant issues remain after three loops, STOP and report them rather than continuing indefinitely.

## Completion report

Report:

- branch used;
- files changed;
- config precedence implemented;
- legacy compatibility behaviour;
- tests added/changed;
- verification commands and results;
- any deferred issues;
- whether the slice is ready for PR/review.

Keep the implementation deliberately small.

> Hierarchical where the data is hierarchical; boring everywhere else.
