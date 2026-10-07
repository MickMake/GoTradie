# GoTradie v0.5.7 — Product Synchronisation

## Status

**Planned — implementation-ready design**

## Purpose

Redesign product synchronisation so GoTradie refreshes known Invoice Ninja Products first, then discovers only products that are missing.

## Core rule

> First sync the products we already have. Then look for the products we do not.

## Vendor and Provider are different concepts

### Vendor

An Invoice Ninja Vendor is an accounting supplier identity.

A legitimate supplier may exist as a Vendor even when GoTradie has no configured catalogue/API integration for it.

### Provider

A Provider is an external product/catalogue source GoTradie knows how to query.

A Provider is configured with canonical name, type, aliases, and provider-specific settings.

## Canonical supplier identity

When a supplier name resolves to a configured Provider alias, GoTradie uses the Provider's canonical `name` as the stable Vendor identity.

Example:

```text
Incoming supplier: Bunnings Warehouse
Canonical Vendor:  Bunnings
Store:             Castle Hill
```

Store/location must remain separate Expense metadata.

Do not create separate Vendor identities merely because purchases came from different stores.

This intentionally replaces the older `Supplier - Store` Vendor naming approach for provider-mapped suppliers.

## Unknown suppliers

Unknown suppliers remain valid accounting Vendors.

If an Expense/import requires a Vendor and it does not yet exist, GoTradie may create it as required by the accounting workflow.

If that Vendor does not resolve to a configured Provider:

```text
retain/create Vendor
retain Expense/accounting data
skip product sync
```

Do not guess a Provider.

## Provider alias matching

Initial Provider matching should be deterministic:

- trim whitespace;
- compare case-insensitively;
- match against configured aliases;
- no fuzzy matching;
- no automatic alias learning.

## Phase 1 — Existing Invoice Ninja Products

For each existing Product:

1. identify its Provider;
2. determine provider item number;
3. inspect sync metadata;
4. fetch current provider data only when required;
5. update the Invoice Ninja Product if necessary;
6. record successful sync metadata;
7. add `(provider,item)` to the in-memory known set.

## Phase 2 — Discover missing Products

After existing Products are processed, inspect Expenses, Quotes and Invoices, but only where sufficient supplier/provider and item evidence exists.

If Provider + item is known: skip.

If the supplier does not resolve to a configured Provider: skip product sync.

If the Provider resolves and item is missing: fetch provider data, create the missing Invoice Ninja Product, and add it to the known set.

## Provider states

Distinguish:

```text
available
discontinued
unknown
error
```

A failed request must never be interpreted as discontinued.

## Archived Products

Do not blindly reactivate an archived Product.

Automatic restoration is only safe when GoTradie can establish that it previously archived the Product specifically because the Provider reported it discontinued.

## Sync metadata

Store enough metadata on the Invoice Ninja Product to determine whether a Provider request is required:

```text
provider
provider item number
last successful sync
provider state
```

The metadata lives in Invoice Ninja, not a new GoTradie database.

## Sync freshness

```text
never synced       -> fetch
sync data stale    -> fetch
recently synced    -> skip
```

Do not add a `--force` freshness override in the first implementation.

## North Shore Timber

Use `https://www.nst.net.au/nst/DownloadCSV`.

Fetch once per sync run, parse once, and build an in-memory item map.

No persistent catalogue cache.

## Generic CSV provider

Initial scope:

```text
source URL
canonical provider name
vendor aliases
item field
description field
price field
```

## Bunnings hardening

Product Sync must distinguish missing location, missing price, API/provider error, not found, and confirmed discontinued.

Unknown/missing price must not silently become zero.

## Business analytics

Store/location metadata must be preserved so later analysis can group spending by Vendor, Store, or Vendor + Store.

Canonicalising Vendor identity must not discard store information.

## Scope guardrail

Do not add persistent Product Sync DB/cache, fuzzy supplier matching, automatic alias learning, generic scraping, transformation DSLs, or `--force`.

## Design rule

> Canonical Vendor identity for accounting; configured Provider identity for sync; Store remains separate analytics metadata.
