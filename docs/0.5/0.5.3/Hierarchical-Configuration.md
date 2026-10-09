# GoTradie v0.5.3 — Hierarchical Configuration

## Status

**CLOSED: Implemented on `v0.5.3-hierarchical-config` and merged to main**

## Purpose

Replace the flat `key=value` configuration with one strict hierarchical YAML file while preserving current Invoice Ninja and Bunnings behaviour. This is a configuration migration, not the v0.5.7 Product identity migration.

## Configuration source and precedence

Operational commands read:

```text
~/.GoTradie/config.yaml
```

The file is mandatory. Precedence is:

```text
documented defaults
  ↓
~/.GoTradie/config.yaml
  ↓
explicitly supported secret environment variables
```

The only supported environment overrides are:

```text
INVOICE_NINJA_TOKEN
BUNNINGS_CLIENT_SECRET
```

Legacy flat configuration, `--config`, `GOTRADIE_CONFIG`, automatic `./gotradie.conf` discovery and generic environment overrides are removed. There is no compatibility framework.

YAML is decoded with `go.yaml.in/yaml/v3` and strict unknown-field validation. Malformed YAML, duplicate fields, unknown fields, empty files and multiple YAML documents fail explicitly.

## Authoritative schema

```yaml
invoice_ninja:
  url: https://your.invoice-ninja.example
  token: ""

tax:
  name: GST
  rate: 10

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

eofy:
  accounting_basis: cash
  instant_asset_writeoff_threshold: 20000

exports:
  directory: ~/Documents/GoTradie

product_sync:
  custom_fields:
    bunnings_in: 1
    image_url: 2

providers:
  bunnings:
    name: Bunnings
    type: api
    aliases:
      - Bunnings
      - Bunnings Warehouse
      - Bunnings Trade
    environment: live
    client_id: ""
    client_secret: ""
    scopes: []
    country: AU
    location: ""

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
      price: RetailPrice
      quantity: PackQuantity
      image_url: ImageURL
```

`exports.directory` is optional. Generated reports use an explicit command path first, then `exports.directory`, then the current working directory.

`invoice_ninja.url` is optional and retains the Invoice Ninja client default when absent. Tokens and Bunnings credentials are command-specific requirements, so unrelated commands do not require every integration credential.

Defaults retained from existing behaviour are:

```text
tax.name                                      GST
tax.rate                                      10
product_sync.custom_fields.bunnings_in        1
product_sync.custom_fields.image_url          2
providers.bunnings.environment                live
providers.bunnings.country                    AU
```

## Old-to-new field mapping

| Legacy field/environment variable | v0.5.3 YAML field | Secret override |
|---|---|---|
| `INVOICE_NINJA_URL` | `invoice_ninja.url` | none |
| `INVOICE_NINJA_TOKEN` | `invoice_ninja.token` | `INVOICE_NINJA_TOKEN` |
| `BUNNINGS_ENV` | `providers.bunnings.environment` | none |
| `BUNNINGS_CLIENT_ID` | `providers.bunnings.client_id` | none |
| `BUNNINGS_CLIENT_SECRET` | `providers.bunnings.client_secret` | `BUNNINGS_CLIENT_SECRET` |
| `BUNNINGS_SCOPES` | `providers.bunnings.scopes` | none |
| `BUNNINGS_COUNTRY` | `providers.bunnings.country` | none |
| `BUNNINGS_LOCATION` | `providers.bunnings.location` | none |
| `BUNNINGS_IN_CUSTOM_FIELD` | `product_sync.custom_fields.bunnings_in` | none |
| `BUNNINGS_IMAGE_CUSTOM_FIELD` | `product_sync.custom_fields.image_url` | none |
| `TAX_NAME` | `tax.name` | none |
| `TAX_RATE` | `tax.rate` | none |
| `PRODUCT_PREFIX` | no configuration replacement | none |
| `ERPNEXT_*` | removed with the retired ERPNext exporter | none |

`providers.bunnings.scopes` changes from comma/space-delimited text to a YAML sequence.

The old configurable Product prefix is deliberately not retained. v0.5.3 uses a temporary internal `BUNNINGS-` constant so existing Product keys, lookup, refresh, import and Image URL behaviour remain unchanged. The constant is a transition constraint for v0.5.7, not a new public setting.

## BAS requirements

`bas.reporting_period`, `bas.gst_basis` and `bas.periods` are mandatory. Supported values are:

```text
reporting_period: monthly | quarterly | yearly
gst_basis: cash | accrual
```

The selected cadence must define exactly its complete financial year. Period keys, reporting boundaries and submission-window consistency are validated. Reporting periods must be contiguous from `07-01` through `06-30`, and each submission window must start the day after its reporting period ends.

`MM-last` is accepted only for `bas_end`, allowing monthly configuration to remain correct for month length and leap years.

### Quarterly

Quarterly reporting retains the existing `Q1`–`Q4` structure shown in the authoritative schema.

### Monthly

The minimum monthly schema uses `Jul` through `Jun`:

```yaml
bas:
  reporting_period: monthly
  gst_basis: cash
  periods:
    Jul: {bas_begin: "07-01", bas_end: "07-last", submit_begin: "08-01", submit_end: "08-21"}
    Aug: {bas_begin: "08-01", bas_end: "08-last", submit_begin: "09-01", submit_end: "09-21"}
    Sep: {bas_begin: "09-01", bas_end: "09-last", submit_begin: "10-01", submit_end: "10-21"}
    Oct: {bas_begin: "10-01", bas_end: "10-last", submit_begin: "11-01", submit_end: "11-21"}
    Nov: {bas_begin: "11-01", bas_end: "11-last", submit_begin: "12-01", submit_end: "12-21"}
    Dec: {bas_begin: "12-01", bas_end: "12-last", submit_begin: "01-01", submit_end: "01-21"}
    Jan: {bas_begin: "01-01", bas_end: "01-last", submit_begin: "02-01", submit_end: "02-21"}
    Feb: {bas_begin: "02-01", bas_end: "02-last", submit_begin: "03-01", submit_end: "03-21"}
    Mar: {bas_begin: "03-01", bas_end: "03-last", submit_begin: "04-01", submit_end: "04-21"}
    Apr: {bas_begin: "04-01", bas_end: "04-last", submit_begin: "05-01", submit_end: "05-21"}
    May: {bas_begin: "05-01", bas_end: "05-last", submit_begin: "06-01", submit_end: "06-21"}
    Jun: {bas_begin: "06-01", bas_end: "06-last", submit_begin: "07-01", submit_end: "07-21"}
```

### Yearly

The minimum yearly schema uses one financial-year period. The submission window remains explicit because annual lodgement dates can depend on circumstances.

```yaml
bas:
  reporting_period: yearly
  gst_basis: cash
  periods:
    FY:
      bas_begin: "07-01"
      bas_end: "06-30"
      submit_begin: "07-01"
      submit_end: "10-31"
```

The BAS default-selection workflow is date-driven and depends on ATO reporting-period and lodgement rules. A later BAS implementation must verify applicable dates rather than treating one example submission window as universal tax advice.

## EOFY accounting basis

`eofy.accounting_basis` and `eofy.instant_asset_writeoff_threshold` are mandatory. The EOFY basis is independent of `bas.gst_basis`; neither is inferred from the other. The threshold must be a finite number greater than zero and remains year-dependent configured tax data.

For threshold testing, use the asset's relevant cost reduced only by GST input tax credits the business is entitled to claim. Private/non-business use affects the deductible or review amount after the threshold test; it does not reduce the asset cost used for that test.

Missing or unsupported accounting-significant values are configuration errors. GoTradie must not silently select another BAS cadence, GST basis, EOFY accounting basis or asset threshold.

## Provider identity and mappings

Each Provider has one canonical `name`. Aliases are recognition inputs only: trim surrounding whitespace, compare case-insensitively, and do not fuzzy-match or learn aliases. Alias/name collisions across Providers are invalid.

When an incoming supplier resolves to a Provider, its canonical name is the Vendor identity. Store/location remains separate Expense metadata and is not appended to Vendor identity.

For configurable CSV Providers, `fields.product` is mandatory and represents the supplier's product identifier. Built-in Bunnings source mappings remain in code.

## Product compatibility boundary

v0.5.3 preserves the existing Product contract:

```text
Product key              BUNNINGS-<item number>
bunnings_in mapping      configured existing custom-field index
image_url mapping        configured existing custom-field index
```

It does not implement the v0.5.7 `(Supplier, Product)` identity, reject legacy Products, reassign Product custom fields, or add Supplier/Store/Last Sync Date/Not Available metadata.

Invoice Ninja exposes four Product custom fields. The current `bunnings_in` and `image_url` fields already consume two, while the v0.5.7 design proposes four more concepts. That allocation cannot fit literally and must be resolved as part of v0.5.7 before its Product identity/custom-field transition is implemented.

## ERPNext retirement

The ERPNext migration exporter is retired in v0.5.3. Its command dispatch, implementation, tests, configuration, help and README material are removed. No ERPNext fields exist in the YAML schema.

## Scope guardrail

This slice changes configuration loading and the minimum consumers needed to use it. It does not implement BAS, EOFY, Financial or v0.5.7 Product Sync features; add persistence or caches; or introduce a compatibility framework.

> Hierarchical where the data is hierarchical; boring everywhere else.
