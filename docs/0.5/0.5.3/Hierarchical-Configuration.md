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

Environment variables must not override structural configuration such as BAS reporting period, GST basis, provider mappings, URLs, filenames, or field mappings.

## Required BAS/GST configuration

BAS reporting period and GST basis materially affect accounting output and must always be explicit in the configuration file.

Required shape:

```yaml
bas:
  reporting_period: quarterly
  gst_basis: cash

  periods:
    Q1:
      bas_begin: "07-01"
      bas_end: "09-30"
      submit_begin: "10-01"
      submit_end: "10-28"

    Q2:
      bas_begin: "10-01"
      bas_end: "12-31"
      submit_begin: "01-01"
      submit_end: "02-28"

    Q3:
      bas_begin: "01-01"
      bas_end: "03-31"
      submit_begin: "04-01"
      submit_end: "04-28"

    Q4:
      bas_begin: "04-01"
      bas_end: "06-30"
      submit_begin: "07-01"
      submit_end: "07-28"
```

Supported BAS reporting period values:

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

`ato_due_dates.verify_every_days` controls how often GoTradie should re-check the ATO BAS due-date rules before considering its locally cached verification stale.

The verification interval is operational configuration, not accounting state.

## Required EOFY accounting basis

EOFY income/expense recognition is configured independently of BAS GST timing.

Required shape:

```yaml
eofy:
  accounting_basis: cash
  instant_asset_writeoff_threshold: 20000
```

Supported `accounting_basis` values:

```text
cash
accrual
```

`eofy.accounting_basis` is the sole EOFY recognition-basis setting.

It must not be inferred from, copied from, or otherwise coupled to:

```text
bas.gst_basis
```

The two settings answer different questions:

```text
bas.gst_basis
    -> GST timing for BAS

eofy.accounting_basis
    -> income/expense recognition for EOFY
```

EOFY recognition must be deterministic from `eofy.accounting_basis`.

`eofy.instant_asset_writeoff_threshold` is the instant asset write-off threshold to apply for the selected EOFY reporting year.

The threshold is year-dependent tax data. Do not treat the configured number as a timeless universal threshold or hard-code one into reporting logic.

For threshold testing, use the asset's relevant cost reduced only by GST input tax credits the business is entitled to claim. Do not reduce the threshold-test cost by private/non-business use.

Business-use percentage affects the deductible/review amount after the threshold test; it does not reduce the asset cost used to determine whether the asset is below the threshold.

Missing or unsupported accounting-significant values are configuration errors. GoTradie must not silently fall back to another BAS reporting period, GST basis, EOFY accounting basis, or asset threshold.

## Optional export directory

Generated report exports may use an optional default directory:

```yaml
exports:
  directory: ~/Documents/GoTradie
```

`exports.directory` is optional.

If it is not configured, generated BAS, EOFY and Financial exports are written to the current working directory.

The global output-resolution order is:

```text
explicit output path, if supported by the command
    ↓
exports.directory, if configured
    ↓
current working directory
```

Do not make an export directory mandatory.

## Proposed structure

```yaml
invoice_ninja:
  url: ...
  token: ...

tax:
  name: GST
  rate: 10

bas:
  reporting_period: quarterly
  gst_basis: cash
  ato_due_dates:
    verify_every_days: 30

eofy:
  accounting_basis: cash
  instant_asset_writeoff_threshold: 20000

exports:
  directory: ~/Documents/GoTradie

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
      product: PartNo
      description: Description
      cost: TradePrice
      price: Price
      quantity: PackQuantity
      image_url: ImageURL
```

## BAS ATO due-date verification

The BAS default-selection workflow is date-driven and depends on ATO BAS period and lodgement due-date rules.

GoTradie may keep a small local operational cache such as:

```text
~/.GoTradie/cache/ato_due_dates.json
```

The cache may contain only due-date verification metadata such as:

```text
last successful verification date
ATO source/rule version or identifier where available
cached BAS due-date rules
```

It must not contain BAS lodgement state, accounting records, Invoice Ninja-derived financial data, or a side ledger.

When the cached ATO due-date verification is older than `bas.ato_due_dates.verify_every_days`, GoTradie should attempt to verify the rules again.

If verification cannot be completed, GoTradie must continue using the existing configured/cached rules and print a warning to stdout indicating that the ATO due-date rules have not been refreshed recently.

A stale or failed ATO verification is not, by itself, a reason to fail BAS generation.

## Secrets

Secrets may exist in YAML, but explicitly supported environment variables may override them.

Examples:

```text
INVOICE_NINJA_TOKEN
BUNNINGS_CLIENT_SECRET
```

Environment overrides are for secret/security-sensitive values only.

Do not add general environment overrides for ordinary configuration values such as URLs, BAS reporting period, GST basis, EOFY accounting basis, provider mappings, filenames, or field mappings.

## Validation

Configuration should fail clearly for:

- missing `~/.GoTradie/config.yaml`;
- malformed YAML;
- unknown configuration fields;
- missing required BAS reporting period;
- missing required GST basis;
- missing required EOFY accounting basis;
- missing required EOFY instant asset write-off threshold;
- unsupported BAS reporting period;
- unsupported GST basis;
- unsupported EOFY accounting basis;
- invalid EOFY instant asset write-off threshold;
- invalid `ato_due_dates` configuration;
- invalid `exports.directory` value when present;
- invalid values that cannot be interpreted safely.

Silent fallback is not acceptable for accounting-significant configuration.

## Provider configuration

Each Provider has one canonical `name`.

That canonical name is the official supplier name and is the Vendor identity used when an incoming supplier resolves to the Provider.

`aliases` are recognition inputs only. They may resolve incoming supplier names to the Provider, but they do not create alternate Vendor identities.

Provider alias matching should initially be deterministic:

- trim surrounding whitespace;
- compare case-insensitively;
- no fuzzy matching;
- no automatic alias learning.

Store/location does not belong in the Provider or Vendor name.

Store/location is separate Expense metadata used for business analytics.

## Generic CSV provider

For configurable file/web Providers, field mappings should mirror Invoice Ninja Product concepts rather than supplier-specific terminology.

Example:

```yaml
fields:
  product: PartNo
  description: Description
  cost: TradePrice
  price: RetailPrice
  quantity: PackQuantity
  image_url: ImageURL
```

`product` is mandatory for a configurable syncing Provider and means the supplier's own product identifier, regardless of whether that supplier calls it SKU, I/N, PartNo, Item Code, Stock Code, or something else.

Built-in Providers such as Bunnings may define their source-to-Product mapping in code instead of YAML.

## Compatibility and migration

v0.5.3 is an intentional configuration break.

1. YAML is the supported configuration format from v0.5.3 onward.
2. The configuration file lives at `~/.GoTradie/config.yaml`.
3. Legacy flat configuration is not supported.
4. Environment variables override only explicitly supported secret fields.
5. No generic migration or compatibility framework is required.

## Scope guardrail

Do not implement BAS, EOFY, Financial, Product Sync, provider fetching, or catalogue logic in this slice.

## Design rule

> Hierarchical where the data is hierarchical; boring everywhere else.
