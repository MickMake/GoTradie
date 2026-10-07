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

Configured Providers have one canonical official supplier name plus accepted aliases.

Aliases are recognition inputs only.

When an incoming supplier matches a configured Provider alias:

```text
Vendor = Provider canonical name
Store  = separate Expense metadata
```

The canonical Provider name is the Vendor identity.

Store/location must never be appended to or encoded into Vendor identity.

Unknown/non-provider Vendors remain valid accounting entities but are skipped by Product Sync.

Provider alias resolution is deterministic:

- trim surrounding whitespace;
- compare case-insensitively;
- no fuzzy matching;
- no automatic alias learning.

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

Reporting and analytics may use Vendor, Store, or Vendor + Store without changing Vendor identity.

## Persistence

Sync metadata lives in Invoice Ninja Product records.

Do not add a GoTradie Product Sync database or persistent catalogue cache.
