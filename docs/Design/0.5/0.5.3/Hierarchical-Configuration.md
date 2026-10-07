# GoTradie v0.5.3 — Hierarchical Configuration

## Status

**Planned — implementation-ready design**

## Purpose

Replace the current flat `key=value` configuration with structured hierarchical YAML.

The implementation should remain deliberately small.

## Goals

- Move configuration to YAML.
- Preserve environment-variable overrides for secrets and deployment-specific values.
- Represent related settings hierarchically.
- Provide clean homes for BAS, EOFY, Product Sync and provider configuration.
- Keep configuration declarative.
- Avoid creating a programmable configuration language.

## Configuration precedence

From v0.5.3 onward:

```text
defaults
  ↓
config file
  ↓
environment variables
```

Environment variables override file values.

This is an intentional change from the older config/CLI documentation and must be reflected consistently in CLI help and user documentation when v0.5.3 is implemented.

## Proposed structure

```yaml
invoice_ninja:
  url: ...
  token: ...

tax:
  name: GST
  rate: 10

bas:
  frequency: quarterly
  gst_basis: cash

eofy:
  instant_asset_writeoff_threshold: 20000

product_sync:
  prefix: BUNNINGS-

providers:
  bunnings:
    name: Bunnings
    type: api
    aliases:
      - Bunnings
      - Bunnings Warehouse
      - Bunnings Trade

  nst:
    name: North Shore Timber
    type: csv
    aliases:
      - North Shore Timber
      - NST
    url: https://www.nst.net.au/nst/DownloadCSV
    fields:
      item: PartNo
      description: Description
      price: Price
```

The exact North Shore Timber CSV column names must be verified against the actual CSV before implementation.

## Secrets

Structural configuration belongs in YAML.

Secrets should remain overrideable through environment variables, for example:

```text
INVOICE_NINJA_TOKEN
BUNNINGS_CLIENT_SECRET
```

## Provider configuration

Each Provider has one canonical `name`.

`aliases` are accepted source/vendor names used only to resolve input data to that Provider.

Example:

```text
Incoming supplier: Bunnings Warehouse
Configured alias:  Bunnings Warehouse
Canonical Provider/Vendor identity: Bunnings
```

Provider alias matching should initially be deterministic:

- trim surrounding whitespace;
- compare case-insensitively;
- no fuzzy matching;
- no automatic alias learning.

Unknown supplier names do not prevent the supplier from existing as an Invoice Ninja Vendor. They simply do not resolve to a product-sync Provider until deliberately configured.

Store/location does not belong in the Provider name.

Store/location is separate Expense metadata used for business analytics.

## Generic CSV provider

Initial configuration should remain small:

```text
source URL
canonical provider name
vendor aliases
item field
description field
price field
```

Do not initially add:

```text
arbitrary regex transforms
HTML selector languages
embedded scripting
row-expression languages
generic workflow logic
```

Rule:

> Config describes the source. Code implements behaviour.

## Compatibility and migration

v0.5.3 is a transition release.

1. YAML is the preferred configuration format from v0.5.3 onward.
2. Existing flat config remains accepted during v0.5.3.
3. Environment variables override file-based values.
4. Existing behaviour must not silently change except for the explicitly accepted precedence change.
5. If both legacy and YAML configuration are supplied, precedence must be deterministic and documented.
6. Removal of legacy flat-config support is a later explicit decision.

Do not build a generic migration framework.

## Required structural homes

The v0.5.3 implementation must provide configuration homes for at least:

```text
invoice_ninja
tax
bas
eofy
product_sync
providers
```

## Scope guardrail

Do not implement BAS, EOFY, Financial, Product Sync, provider fetching, or catalogue logic in this slice.

## Design rule

> Hierarchical where the data is hierarchical; boring everywhere else.
