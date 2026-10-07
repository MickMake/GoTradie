# GoTradie v0.5 Cross-Audit

## Status

Active design reconciliation before v0.5.3 implementation.

## Locked CLI vocabulary

GoTradie uses two distinct persistence/safety flags:

```text
--commit
    Permit persistent changes to Invoice Ninja / remote application state.

--force
    Permit overwriting existing local output files.
```

These meanings must not overlap.

### `--commit`

Use `--commit` only for commands that would otherwise preview or refuse a persistent Invoice Ninja change.

Examples:

```text
GoTradie sync refresh --commit
GoTradie sync import 0123456 --commit
GoTradie ninja import products products.csv --commit
GoTradie ninja import clients clients.csv --commit
GoTradie ninja import expenses purchases.csv --commit
```

Without `--commit`, write-capable Invoice Ninja operations preview only.

### `--force`

Use `--force` only for local output replacement.

Examples:

```text
GoTradie ninja export products products.csv --force
GoTradie ninja export erpnext ./erpnext-export --force
```

If the requested output does not already exist, export commands may create it normally without `--force`.

If the requested output already exists, the exporter refuses unless `--force` is supplied.

`--force` must not:

- imply Invoice Ninja writes;
- bypass accounting validation;
- bypass Product Sync freshness;
- mean "apply";
- act as an alternative to `--commit`.

## Configuration precedence

From v0.5.3 onward:

```text
defaults
  ↓
~/.GoTradie/config.yaml
  ↓
explicitly supported secret environment variables
```

`~/.GoTradie/config.yaml` is mandatory.

Environment variables may override explicitly supported secret values only. There is no generic environment-variable mapping for arbitrary configuration fields.

Legacy flat configuration is not supported by v0.5.3.

## Required BAS/GST configuration

The configuration must explicitly provide:

```yaml
bas:
  reporting_period: quarterly
  gst_basis: cash
```

Supported `reporting_period` values:

```text
monthly
quarterly
yearly
```

Supported `gst_basis` values:

```text
cash
accrual
```

Missing or unsupported BAS reporting period or GST basis is a configuration error.

## BAS period-selection contract

BAS period selection is governed by `bas.reporting_period`.

The command accepts:

```text
GoTradie ninja export bas
GoTradie ninja export bas --fy 2027
GoTradie ninja export bas --period 2
GoTradie ninja export bas --fy 2027 --period 2
```

Defaults:

```text
no --fy     -> current Australian financial year
no --period -> current reporting period
```

`--period` interpretation:

```text
monthly   -> 1-12
quarterly -> 1-4
yearly    -> invalid / not applicable
```

The configured reporting period determines the valid period-number range.

BAS does not accept arbitrary `--from/--to` date ranges.

Arbitrary date-range output belongs to the Financial/dump export.

## Required configuration homes

v0.5.3 must provide hierarchical homes for at least:

```text
invoice_ninja
tax
bas
eofy
product_sync
providers
```

## Provider identity

A configured Provider has:

```text
canonical name
type
aliases
provider-specific configuration
```

The canonical name is the official supplier name used as Vendor identity.

Aliases are recognition inputs only. They may resolve incoming supplier names to a Provider, but they must not become alternate Vendor identities.

## Vendor versus Provider

A Vendor is an accounting supplier identity stored in Invoice Ninja.

A Provider is an external product/catalogue source GoTradie knows how to query.

A Vendor does not need to have a Provider.

Unknown suppliers remain valid Vendors, but Product Sync only runs where a Vendor resolves to a configured Provider.

## Canonical Vendor identity and Store metadata

Vendor identity must use the canonical official supplier name only.

Required representation:

```text
Vendor: Bunnings
Store: Castle Hill
```

Store/location must be preserved as separate Expense metadata.

Store/location must not:

- be appended to or encoded into Vendor identity;
- create a separate Vendor identity;
- create a separate Provider identity;
- affect Provider alias resolution after the supplier has resolved.

Reporting and analytics may group by Vendor, Store, or Vendor + Store.

## Product identity and Product custom fields

Invoice Ninja Product identity is:

```text
Supplier + Product
```

where:

```text
Product        = supplier SKU
Supplier       = canonical supplier name
Store          = optional metadata only
Last Sync Date = date of last successful provider sync
Not Available  = availability toggle
```

Only `Supplier` and `Product` participate in product matching and deduplication.

`Store`, `Last Sync Date`, and `Not Available` must not participate in matching or deduplication.

`Last Sync Date` changes only after a successful provider sync for that product.

`Not Available` changes only from positive provider evidence:

```text
confirmed unavailable/discontinued -> true
confirmed available                 -> false
provider/source error               -> leave unchanged
```

A product that disappears and later reappears remains the same Product identity.

## Product Sync CLI

The v0.5.7 Product Sync command is:

```text
GoTradie sync refresh [--commit]
```

The command processes all configured Providers.

Without `--commit`, it previews proposed Invoice Ninja changes.

With `--commit`, it may persist the proposed Invoice Ninja Product changes.

No provider-selection flag is required in the first implementation.

## Provider freshness

Freshness depends on provider type.

### API/product-oriented providers

For providers such as Bunnings, freshness is evaluated per Product.

### File-backed providers

For providers whose catalogue is downloaded as a file, freshness is evaluated primarily at the source-file level.

The implementation may persist a small source-fetch cache containing metadata such as:

```text
provider identity
source URL
ETag
Last-Modified
content hash
last successful source check
```

A content hash is the authoritative change detector when available.

HTTP metadata such as ETag or Last-Modified may be used as optimisation hints.

This cache exists only to determine whether the source file changed. It must not become a second persistent product catalogue or accounting database.

## Product Sync freshness override

Do not use `--force` as a Product Sync freshness override.

The first v0.5.7 implementation does not require an "ignore freshness" flag.

## EOFY instant asset write-off threshold

For GST-registered businesses entitled to claim the relevant GST credit, threshold comparison uses asset cost excluding claimable GST.

The workbook should expose gross amount, GST, GST-exclusive cost, business-use information and source identity.

GoTradie does not calculate depreciation.

## Cross-audit rule

The implementation agent should never need to choose between contradictory authoritative documents.
