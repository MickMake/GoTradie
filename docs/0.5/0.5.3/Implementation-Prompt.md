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

## Mandatory preflight

Before making any code changes:

1. Fetch the latest `origin/main`.
2. Verify that all earlier slice branches and pull requests are merged into `main` or explicitly retired.
3. If any earlier slice branch or PR is still active and unmerged, STOP and report it.
4. Inspect the current configuration implementation and tests.
5. State the intended change, proposed branch name, files/packages likely affected, and any design ambiguity discovered.
6. STOP and wait for approval before creating the branch or modifying code.

Suggested branch:

```text
v0.5.3-hierarchical-config
```

After approval, branch from latest `origin/main` only.

## Required implementation

Implement hierarchical YAML configuration.

The implementation must:

- load the mandatory configuration file from `~/.GoTradie/config.yaml`;
- fail clearly if the configuration file is absent;
- represent configuration hierarchically;
- apply precedence as `defaults -> YAML -> explicitly supported secret environment overrides`;
- support environment overrides only for explicitly defined secret/security-sensitive fields;
- not implement generic environment-variable mapping;
- not preserve legacy flat config support;
- require BAS reporting period and GST basis in YAML;
- accept BAS reporting period only as `monthly`, `quarterly`, or `yearly`;
- accept GST basis only as `cash` or `accrual`;
- fail on missing or unsupported accounting-significant configuration;
- fail on unknown YAML fields rather than silently ignoring misspellings;
- preserve current Invoice Ninja, Bunnings, tax and ERPNext behaviour except where explicitly changed;
- provide structural homes for `invoice_ninja`, `tax`, `bas`, `eofy`, `product_sync`, and `providers`;
- support Provider canonical `name` plus `aliases`;
- keep Provider configuration declarative;
- not preserve or introduce a Product-key prefix setting such as the former `PRODUCT_PREFIX` / `BUNNINGS-` scheme;
- support configurable Provider field mappings using Invoice Ninja Product concepts such as `product`, `description`, `cost`, `price`, `quantity`, and `image_url`;
- avoid adding provider-fetch logic except where minimally required to validate parsing.

## Secret override contract

Environment-variable overrides are permitted only for explicitly supported secrets, for example:

```text
INVOICE_NINJA_TOKEN
BUNNINGS_CLIENT_SECRET
```

Do not add environment overrides for ordinary structural configuration such as URLs, BAS reporting period, GST basis, provider mappings, filenames or field mappings.

## Provider configuration contract

Provider configuration must support one canonical official supplier name and multiple accepted aliases.

When an incoming supplier matches a configured Provider alias, the Provider canonical name is the Vendor identity.

Aliases are recognition inputs only and must not become alternate Vendor identities.

Store/location must remain separate Expense metadata and must not be encoded into Provider or Vendor identity.

Do not implement fuzzy supplier matching or automatic alias learning in this slice.

## Provider Product-field mapping contract

For configurable file/web Providers:

- mapping keys describe the target Invoice Ninja Product concept;
- mapping values identify the Provider source field;
- `product` is required and is the supplier's own product identifier;
- supplier terminology such as SKU, I/N, PartNo, Item Code, or Stock Code must not leak into GoTradie's canonical field names;
- built-in Providers may define the same mapping in code;
- do not configure or implement stock-level synchronisation in v0.5.3.

Do not add a Product prefix setting. Product identity in v0.5.7 is based on `(Supplier, Product)`, with Product holding the supplier's exact product identifier.

## Scope exclusions

Do not implement BAS, Accounting Dataset, EOFY, Financial, redesigned Product Sync, NST fetching, generic CSV fetching, provider lifecycle logic, speculative config features, legacy config migration, or a general environment-variable configuration system.

## Tests

Add or update tests covering at least:

- valid YAML parsing;
- mandatory `~/.GoTradie/config.yaml` behaviour;
- hierarchical field mapping;
- default -> YAML -> secret-environment precedence;
- explicitly supported secret overrides;
- absence of generic environment overrides;
- required BAS reporting period;
- required GST basis;
- valid BAS reporting-period values;
- valid GST basis values;
- unknown-field rejection;
- malformed YAML;
- Provider canonical name and aliases;
- canonical Vendor identity after alias resolution;
- Store/location remaining separate from Vendor identity;
- configurable Provider Product-field mapping;
- absence of a Product prefix setting;
- `eofy` configuration parsing;
- validation failures;
- preservation of existing defaults where defaults are appropriate.

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

BAS/EOFY/Financial local-file overwrite semantics are already defined by `docs/Design/Command-Line-Spec.md` and must not be redefined differently here.

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

Report branch, files changed, config precedence, mandatory config behaviour, secret override behaviour, BAS/GST validation, Provider config behaviour, tests, verification results, deferred issues, and PR readiness.
