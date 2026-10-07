# GoTradie v0.5.7 — Product Synchronisation

## Status

**Planned — implementation-ready design**

## Core rule

> Sync known Invoice Ninja Products first. Then look for products that are missing.

## CLI

The Product Sync command is:

```text
GoTradie sync refresh [--commit]
```

The command processes all configured Providers.

Without `--commit`, the command previews proposed Invoice Ninja changes and remains non-mutating.

With `--commit`, the command may persist the proposed Invoice Ninja Product changes.

The first implementation does not require a Provider-selection flag.

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

## Product identity

Invoice Ninja Product identity is:

```text
(Supplier, Product)
```

The fields mean:

```text
Product  = exact supplier SKU
Supplier = canonical supplier name
```

Both participate in product matching and deduplication.

The following Product fields are metadata only:

```text
Store
Last Sync Date
Not Available
```

They must never participate in product matching or deduplication.

## Product custom fields

### Supplier

`Supplier` contains the canonical supplier name.

Example:

```text
Bunnings
```

It participates in Product identity.

### Store

`Store` is optional provenance/location metadata.

Example:

```text
Castle Hill
```

It does not participate in Product identity.

### Last Sync Date

`Last Sync Date` is a date field.

It records the local calendar date of the last successful provider sync for that Product.

It must not change merely because a refresh was attempted.

A provider/API/source failure does not update `Last Sync Date`.

### Not Available

`Not Available` is a boolean/toggle field describing the Product's current known availability.

Update rules:

```text
provider positively confirms unavailable/discontinued -> true
provider positively confirms available                 -> false
provider/API/source request fails                       -> leave unchanged
availability cannot be determined                       -> leave unchanged
```

A Product may disappear and later reappear.

When it reappears:

```text
same Product identity
Not Available = false
Last Sync Date = successful sync date
```

Do not delete and recreate the Product merely because its availability changed.

## Freshness

Freshness depends on Provider type.

### API/product-oriented Providers

For API-backed Providers such as Bunnings, freshness is Product-oriented.

Each Product's `Last Sync Date` determines whether the Product should be refreshed according to configured freshness rules.

### File-backed Providers

For Providers whose catalogue is downloaded as a file, freshness is source-oriented.

The preferred sequence is:

1. obtain/download the source;
2. use HTTP source metadata where available as an optimisation hint;
3. calculate a deterministic content hash;
4. compare with the previously successful source fingerprint;
5. if unchanged, do not reprocess the catalogue;
6. if changed, parse the source once and refresh affected Products.

The content hash is the authoritative source-change detector when available.

ETag or Last-Modified may be retained and used as optimisation hints, but must not be the sole correctness mechanism where content hashing is practical.

## Source-fingerprint cache

A small persistent cache is permitted for file-backed Provider source freshness.

It may contain only source-fetch/fingerprint information such as:

```text
Provider identity
source URL
ETag
Last-Modified
content hash
last successful source check
```

This cache exists to answer:

> Has this provider source changed since the last successful fetch?

It must not contain or become:

```text
a second Product catalogue
a Product identity database
an accounting database
a replacement for Invoice Ninja Product state
```

Product state and Product sync metadata remain in Invoice Ninja.

## Provider states

Internally distinguish:

```text
available
discontinued
unknown
error
```

These states drive the Product metadata rules above.

Never interpret a failed request as discontinued or unavailable.

## Store analytics

Store/location metadata must remain available independently from canonical Vendor identity.

Reporting and analytics may use Vendor, Store, or Vendor + Store without changing Vendor or Product identity.

## Persistence

Product state and sync metadata live in Invoice Ninja Product records.

Do not add a persistent GoTradie product catalogue or product-state database.

The narrow file-source fingerprint cache described above is permitted and is not product state.
