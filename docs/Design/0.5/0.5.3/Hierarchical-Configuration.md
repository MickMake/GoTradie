# GoTradie v0.5.3 — Hierarchical Configuration

## Status

**Planned — implementation-ready design**

## Purpose

Replace the current flat `key=value` configuration with structured hierarchical YAML.

The implementation should remain deliberately small.

## Goals

- Move configuration to YAML.
- Use a single mandatory configuration file at `~/.GoTradie/config.yaml`.
- Allow environment-variable overrides only for explicitly supported secrets.
- Represent related settings hierarchically.
- Provide clean homes for BAS, EOFY, Product Sync and provider configuration.
- Keep configuration declarative.
- Avoid creating a programmable configuration language.
- Do not preserve legacy flat configuration compatibility.

## Configuration precedence

From v0.5.3 onward:

```text
defaults
  ↓
~/.GoTradie/config.yaml
  ↓
explicitly supported secret environment variables
```

The configuration file is mandatory.

If `~/.GoTradie/config.yaml` is missing, configuration loading fails.

Environment-variable support is opt-in per secret field. There is no generic hierarchical environment-variable mapping.

Environment variables must not override structural configuration such as BAS frequency, GST basis, provider mappings, URLs, filenames, or field mappings.

## Required BAS/GST configuration

BAS frequency and GST basis materially affect accounting output and must always be explicit in the configuration file.

Required shape:

```yaml
bas:
  frequency: quarterly
  gst_basis: cash
```

Supported BAS frequency values:

```text
monthly
quarterly
yearly
```

Supported GST basis values:

```text
cash
accrual
```

Missing or unsupported values are configuration errors. GoTradie must not silently fall back to another BAS frequency or GST basis.

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

Secrets may exist in YAML, but explicitly supported environment variables may override them.

Examples:

```text
INVOICE_NINJA_TOKEN
BUNNINGS_CLIENT_SECRET
```

Environment overrides are for secret/security-sensitive values only.

Do not add general environment overrides for ordinary configuration values such as URLs, BAS frequency, GST basis, provider mappings, filenames, or field mappings.

## Validation

Configuration should fail clearly for:

- missing `~/.GoTradie/config.yaml`;
- malformed YAML;
- unknown configuration fields;
- missing required BAS frequency;
- missing required GST basis;
- unsupported BAS frequency;
- unsupported GST basis;
- invalid values that cannot be interpreted safely.

Silent fallback is not acceptable for accounting-significant configuration.

## Provider configuration

Each Provider has one canonical `name`.

That canonical name is the official supplier name and is the Vendor identity used when an incoming supplier resolves to the Provider.

`aliases` are recognition inputs only. They may resolve incoming supplier names to the Provider, but they do not create alternate Vendor identities.

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

Store/location does not belong in the Provider or Vendor name.

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

v0.5.3 is an intentional configuration break.

1. YAML is the supported configuration format from v0.5.3 onward.
2. The configuration file lives at `~/.GoTradie/config.yaml`.
3. Legacy flat configuration is not supported.
4. Environment variables override only explicitly supported secret fields.
5. No generic migration or compatibility framework is required.

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
