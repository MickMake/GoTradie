# GoTradie v0.5.7 — Implementation Prompt

Primary design contract:

```text
docs/Design/0.5/0.5.7/Product-Sync.md
```

Cross-audit contract:

```text
docs/Design/0.5/Cross-Audit.md
```

Relevant configuration contract:

```text
docs/Design/0.5/0.5.3/Hierarchical-Configuration.md
```

## Mandatory preflight

Before making any code changes:

1. Fetch latest `origin/main`.
2. Verify all earlier slice branches/PRs are merged or explicitly handled.
3. If any earlier slice branch/PR is not merged, STOP and report it.
4. Inspect current Product Sync, Bunnings behaviour, Invoice Ninja Product custom fields, Vendor handling, Expense supplier/store metadata, Quote/Invoice product evidence, Provider config, and any existing download/cache mechanism.
5. Verify Bunnings missing-location, missing-price, API-error and not-found/discontinued behaviour.
6. Inspect the actual NST CSV before locking field names.
7. State intended implementation, branch, likely files/packages, and any blocking metadata/source-data ambiguity.
8. STOP and wait for approval.

Suggested branch:

```text
v0.5.7-product-sync
```

## Required CLI

Implement:

```text
GoTradie sync refresh [--commit]
```

The command processes all configured Providers.

Without `--commit`, preview proposed Invoice Ninja changes only.

With `--commit`, persist the proposed Invoice Ninja Product changes.

Do not add a Provider-selection flag in the first implementation.

Do not add a Product Sync `--force` meaning.

## Required architecture

Implement the two-phase sync:

1. existing Invoice Ninja Products first;
2. build an in-memory `(supplier,product)` known set;
3. then inspect Expenses, Quotes and Invoices where sufficient Provider/Supplier/SKU evidence exists;
4. create/fetch only missing known-Provider Products.

## Product identity and custom fields

Use this Invoice Ninja Product contract:

```text
Product        = exact supplier SKU
Supplier       = canonical supplier name
Store          = optional provenance/location metadata
Last Sync Date = date of last successful provider sync
Not Available  = boolean/toggle availability state
```

Only `(Supplier, Product)` participates in Product matching and deduplication.

`Store`, `Last Sync Date`, and `Not Available` must not participate in identity, matching, or deduplication.

`Last Sync Date`:

- is a date field;
- records the local calendar date of a successful Product sync;
- is not updated for a failed refresh attempt.

`Not Available`:

- becomes `true` only when the Provider positively confirms unavailable/discontinued;
- becomes `false` when the Provider positively confirms available;
- remains unchanged on Provider/API/source error;
- remains unchanged when availability cannot be determined.

A Product that disappears and later reappears remains the same Product. Do not delete/recreate it due solely to availability changes.

## Vendor/Provider behaviour

- Vendors are accounting entities.
- Providers are configured catalogue/API integrations.
- A supplier may be a valid Vendor without having a Provider.
- If a required Vendor does not exist, preserve the accounting/import behaviour that creates it where appropriate.
- Unknown/non-provider Vendors do not participate in Product Sync.

When an incoming supplier matches a configured Provider alias:

```text
use Provider canonical official supplier name as Vendor/Supplier identity
preserve Store separately as metadata
```

Aliases are recognition inputs only and must not become alternate Vendor/Supplier identities.

Store/location must not be appended to or encoded into Vendor/Supplier identity.

Do not create store-specific Vendor variants.

Do not discard Store metadata.

## Provider aliases

Use configured aliases only.

Resolution is trim + case-insensitive exact matching after normalization.

Do not implement fuzzy matching or automatic alias learning.

## Freshness by Provider type

### API/product-oriented Providers

For Bunnings/API-backed Providers, freshness is Product-oriented.

Use Product `Last Sync Date` and configured freshness rules to determine whether a Product requires refresh.

### File-backed Providers

For downloadable catalogue files:

1. fetch/inspect the source;
2. retain HTTP metadata such as ETag/Last-Modified where useful;
3. calculate a deterministic content hash;
4. compare it with the previous successful source fingerprint;
5. if unchanged, avoid reparsing/reapplying Product data;
6. if changed, parse once and operate from the resulting in-memory representation.

Content hash is the authoritative source-change detector when practical.

HTTP metadata is an optimisation hint, not the sole correctness mechanism.

## Source-fingerprint cache

A small persistent cache is allowed solely for file-backed Provider source freshness.

It may contain:

```text
Provider identity
source URL
ETag
Last-Modified
content hash
last successful source check
```

It must not contain Product catalogue rows or become a second Product/accounting database.

Product state remains in Invoice Ninja.

## Provider states

Implement available, discontinued, unknown and error.

Confirmed unavailable/discontinued sets `Not Available`.

Confirmed available clears `Not Available`.

Unknown/error makes no availability-state change.

Failed requests are never interpreted as discontinued.

## Sync metadata

Use the existing Invoice Ninja Product custom fields:

```text
Supplier
Store
Last Sync Date
Not Available
```

Before implementation, inspect their actual API/custom-field representation and confirm they can be read/written safely.

Do not invent alternate duplicate metadata fields if these existing fields are usable.

## File-backed Provider processing

Fetch a file-backed source once per refresh run when required, parse once, and use an in-memory map/index for the run.

Do not persist the parsed catalogue.

Verify actual source fields before implementation.

## Tests

Cover at least:

- `sync refresh` preview behaviour;
- `sync refresh --commit`;
- all-configured-Provider processing;
- existing-Products-first behaviour;
- `(Supplier, Product)` identity/deduplication;
- Store excluded from identity;
- Last Sync Date success-only updates;
- Not Available true on confirmed unavailable;
- Not Available false on confirmed reappearance;
- Not Available unchanged on error/unknown;
- Provider alias resolution;
- canonical Supplier/Vendor identity;
- unknown Vendor skip-sync behaviour;
- API/Product freshness;
- file-source fingerprint unchanged path;
- file-source fingerprint changed path;
- hash-based change detection;
- file fetch/parse once per run;
- absence of persistent Product catalogue/database.

## Verification

Run `gofmt`, `go vet`, `go test`, and `go build`.

Maximum review/fix loops: 3.
