# GoTradie v0.5.7 — Implementation Prompt

Primary design contract:

```text
docs/0.5/0.5.7/Product-Sync.md
```

Relevant configuration contract:

```text
docs/0.5/0.5.3/Hierarchical-Configuration.md
```

## Mandatory preflight

Before making any code changes:

1. Fetch latest `origin/main`.
2. Verify all earlier slice branches/PRs are merged or explicitly handled.
3. If any earlier slice branch/PR is not merged, STOP and report it.
4. Inspect current Product Sync, Bunnings behaviour, Invoice Ninja Product custom fields, Vendor handling, Expense supplier/store metadata, Quote/Invoice product evidence, Provider config, and any existing download/cache mechanism.
5. Verify Bunnings missing-location, missing-price, API-error and not-available behaviour.
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

1. load existing Invoice Ninja Products;
2. resolve Supplier to configured Provider and discard Products whose Supplier has no sync method;
3. sort eligible Products with no `Last Sync Date` first, then oldest `Last Sync Date` first;
4. sync existing Products in that order;
5. update `Last Sync Date` only after successful processing of that specific Product;
6. build an in-memory `(supplier,product)` known set;
7. then inspect Expenses, Quotes and Invoices where sufficient Provider/Supplier/Product evidence exists;
8. create/fetch only missing known-Provider Products.

## Product identity and custom fields

Use this Invoice Ninja Product contract:

```text
Product        = exact supplier product identifier
Supplier       = canonical supplier name
Store          = optional provenance/location metadata
Last Sync Date = date this specific Product was last successfully synced
Not Available  = boolean/toggle availability state
```

Only `(Supplier, Product)` participates in Product matching and deduplication.

Do not prepend Provider-specific prefixes to Product.

Supplier terminology such as SKU, I/N, PartNo, Item Code, or Stock Code all map to the Invoice Ninja `Product` concept.

`Store`, `Last Sync Date`, and `Not Available` must not participate in identity, matching, or deduplication.

`Last Sync Date`:

- is a date field;
- records the local calendar date of a successful Product sync;
- is not updated for a failed refresh attempt;
- belongs to the Invoice Ninja Product, not a Provider file/list/API call;
- is not updated merely because a file source was checked and found unchanged.

`Not Available`:

- becomes `true` only when the Provider positively confirms not available;
- becomes `false` when the Provider positively confirms available;
- remains unchanged on Provider/API/source error;
- remains unchanged when availability cannot be determined.

A Product that disappears and later reappears remains the same Product. Do not delete/recreate it due solely to availability changes.

Do not introduce a generic `discontinued` Product state.

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

Use Product `Last Sync Date` to order eligible Products with never-synced first and oldest-synced next.

### File-backed Providers

For downloadable catalogue files:

1. fetch/inspect the source;
2. retain HTTP metadata such as ETag/Last-Modified where useful;
3. calculate a deterministic content hash;
4. compare it with the previous successful source fingerprint;
5. if unchanged, avoid reparsing/reapplying Product data;
6. if changed, parse once and process affected Products individually from the resulting in-memory representation.

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

Source-cache freshness and Product freshness are separate.

An unchanged file may update source-cache check state but must not update Product `Last Sync Date`.

## Provider states

Implement available, not_available, unknown and error.

Confirmed not available sets `Not Available`.

Confirmed available clears `Not Available`.

Unknown/error makes no availability-state change.

Failed requests are never interpreted as not available.

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

## Syncable Product fields

The first implementation may update:

```text
Product
Description
Cost
Price
Quantity
Image URL
Vendor
Product custom fields
```

`Quantity` must represent the Invoice Ninja/default line-item quantity concept, not stock-on-hand.

Do not implement supplier inventory/stock-level synchronisation in v0.5.7.

## Configurable Provider field mappings

Use Invoice Ninja Product concepts as mapping keys.

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

`product` is mandatory for configurable syncing Providers.

Built-in Providers such as Bunnings may define their mappings in code.

Remove/ignore the old Product-prefix model. Do not preserve `BUNNINGS-` or another prefix as part of Product identity.

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
- absence of Product prefixes;
- Store excluded from identity;
- never-synced-first ordering;
- oldest-Last-Sync-Date-first ordering;
- Last Sync Date success-only updates;
- unchanged file does not update Product Last Sync Date;
- Not Available true on confirmed unavailable;
- Not Available false on confirmed reappearance;
- Not Available unchanged on error/unknown;
- absence of generic discontinued state;
- Provider alias resolution;
- canonical Supplier/Vendor identity;
- unknown Vendor skip-sync behaviour;
- configurable Product-field mapping;
- API/Product freshness;
- file-source fingerprint unchanged path;
- file-source fingerprint changed path;
- hash-based change detection;
- file fetch/parse once per run;
- stock/inventory sync excluded;
- absence of persistent Product catalogue/database.

## Verification

Run `gofmt`, `go vet`, `go test`, and `go build`.

Maximum review/fix loops: 3.
