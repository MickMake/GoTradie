# GoTradie v0.5.7 — Product Synchronisation

## Status

**Planned — implementation-ready design**

## Core rule

> Sync known Invoice Ninja Products first. Then look for products that are missing.

## CLI safety vocabulary

Product Sync changes Invoice Ninja only when `--commit` is supplied.

`--force` is reserved for overwriting existing local output files and is not a Product Sync freshness or write flag.

The first implementation has no "ignore freshness" override.

## Vendor and Provider

Vendor = accounting supplier identity.

Provider = configured external catalogue/API source.

A Vendor may exist without a Provider.

Configured Providers have one canonical name plus accepted aliases.

When a supplier resolves to a Provider, canonical Vendor identity is used and Store remains separate Expense metadata.

Unknown/non-provider Vendors remain valid accounting entities but are skipped by Product Sync.

## Freshness

```text
never synced       -> fetch
stale              -> fetch
recently synced    -> skip
```

Do not add a Product Sync `--force` meaning.

## Provider states

Distinguish:

```text
available
discontinued
unknown
error
```

Never interpret a failed request as discontinued.

## Store analytics

Store/location metadata must remain available independently from canonical Vendor identity.

## Persistence

Sync metadata lives in Invoice Ninja Product records.

Do not add a GoTradie Product Sync database or persistent catalogue cache.
