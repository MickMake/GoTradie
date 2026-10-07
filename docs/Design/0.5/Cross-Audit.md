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
  frequency: quarterly
  gst_basis: cash
```

Supported `frequency` values:

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

Missing or unsupported BAS/GST configuration is a configuration error.

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

## Product Sync freshness override

Do not use `--force` as a Product Sync freshness override.

The first v0.5.7 implementation does not require an "ignore freshness" flag.

## EOFY instant asset write-off threshold

For GST-registered businesses entitled to claim the relevant GST credit, threshold comparison uses asset cost excluding claimable GST.

The workbook should expose gross amount, GST, GST-exclusive cost, business-use information and source identity.

GoTradie does not calculate depreciation.

## Cross-audit rule

The implementation agent should never need to choose between contradictory authoritative documents.
