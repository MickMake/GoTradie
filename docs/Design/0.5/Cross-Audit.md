# GoTradie v0.5 Cross-Audit

## Status

Active design reconciliation before v0.5.3 implementation.

## Purpose

Cross-audit the v0.5.3-v0.5.7 designs against:

- the existing v0.5.1/v0.5.2 accounting contracts;
- the current CLI contract;
- current GoTradie behaviour;
- later-slice dependencies.

The goal is to remove contradictory instructions before implementation begins.

## Locked decisions

### Configuration precedence

From v0.5.3 onward, configuration precedence is:

```text
defaults
  ↓
config file
  ↓
environment variables
```

Environment variables override file values.

This is an intentional change from the current CLI/config documentation and must be reflected consistently when v0.5.3 is implemented.

### Required configuration homes

v0.5.3 must provide hierarchical homes for at least:

```text
invoice_ninja
tax
bas
eofy
product_sync
providers
```

### Provider identity

A configured provider has:

```text
canonical name
type
aliases
provider-specific configuration
```

Example:

```yaml
providers:
  bunnings:
    name: Bunnings
    type: api
    aliases:
      - Bunnings
      - Bunnings Warehouse
      - Bunnings Trade
```

Aliases are recognition inputs.

The configured canonical name is the stable supplier identity used by GoTradie when a provider mapping is known.

### Vendor versus provider

These are different concepts.

```text
Vendor
    Accounting supplier identity stored in Invoice Ninja.

Provider
    Configured external product/catalogue source that GoTradie knows how to query.
```

A Vendor does not need to have a configured Provider.

Unknown suppliers must still be valid accounting Vendors.

If an Expense references a legitimate supplier that does not exist in Invoice Ninja, GoTradie may create that Vendor as required by the import/accounting workflow.

If that Vendor does not resolve to a configured Provider, product sync simply does not run for that supplier.

### Canonical Vendor identity and store metadata

Store/location information is important business-analytics data and must not be discarded.

However, store/location must not create uncontrolled Vendor-name variants.

Preferred representation:

```text
Vendor: Bunnings
Store: Castle Hill
```

not:

```text
Vendor: Bunnings - Castle Hill
Vendor: Bunnings Warehouse Castle Hill
Vendor: Bunnings Castle Hill NSW
```

When a supplier resolves to a configured Provider:

- the canonical provider name is the Invoice Ninja Vendor identity;
- store/location remains separate Expense metadata;
- analytics may group by Vendor, Store, or Vendor + Store.

This intentionally supersedes the older import behaviour that constructed Vendor names from `Supplier - Store`.

The historical source evidence is still retained through Expense metadata.

### Provider alias matching

Provider aliases are configuration-driven recognition rules.

Initial matching should be simple and deterministic:

- trim surrounding whitespace;
- compare case-insensitively;
- do not use fuzzy matching;
- do not automatically learn aliases from source data.

A supplier that does not match a configured provider alias remains a valid Vendor but is not product-sync capable.

### Product sync freshness override

Do not add a `--force` option to v0.5.7.

The existing CLI contract reserves `--commit` for persistent changes and explicitly rejects resurrecting `--force` as a general write/safety flag.

The first v0.5.7 implementation does not require an "ignore freshness" override.

### Product discovery

Product discovery may inspect:

```text
Expenses
Quotes
Invoices
```

but only when sufficient supplier/provider and item evidence exists.

If a supplier does not resolve to a configured Provider:

```text
retain/create Vendor as required for accounting
skip product sync
report if useful
```

Do not guess a provider.

### EOFY instant asset write-off threshold

The configured threshold is a reporting/classification aid for accountant review.

For a GST-registered business entitled to claim the relevant GST credit, threshold comparison should use asset cost excluding claimable GST.

The workbook should still expose gross amount, GST, GST-exclusive cost, business-use information and source identity so the accountant can verify treatment.

GoTradie must not become a depreciation engine.

## Explicitly unresolved

### Export overwrite and CLI/file-write semantics

Do not change or finalise report-export overwrite behaviour as part of this cross-audit patch.

The interaction between:

- generated BAS/EOFY/Financial filenames;
- existing CLI `--commit` rules;
- local file overwrite behaviour;
- read-only remote exports;

will be reviewed separately.

Until that discussion is complete, design and implementation prompts should not introduce a new overwrite rule for BAS, EOFY or Financial exports.

## Cross-audit rule

If a later slice depends on an earlier design decision, that dependency must be explicit rather than inferred.

The implementation agent should never need to choose between two contradictory authoritative documents.
