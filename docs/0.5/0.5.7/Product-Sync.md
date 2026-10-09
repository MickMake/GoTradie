# GoTradie v0.5.7 — Product Synchronisation

## Status

**Implementation-ready design — decisions resolved**

## v0.5.3 transition constraint

v0.5.3 deliberately deferred the Product identity/custom-field transition. It retained existing `BUNNINGS-<item number>` keys and the `bunnings_in` and `image_url` custom-field mappings through a temporary internal `BUNNINGS-` constant.

v0.5.7 replaces that interim arrangement. Migrate existing Bunnings Product records safely and preserve their identities and existing data. Do not silently discard, duplicate or reassign Products. The transition must be explicitly tested.

## Core rule

> Sync known Invoice Ninja Products first. Then look for products that are missing.

## CLI

```text
GoTradie sync refresh [--commit]
```

The command processes all configured Providers. Without `--commit`, it previews proposed Invoice Ninja changes and does not mutate remote records. With `--commit`, it may persist those changes. No Provider-selection flag is required in the first implementation.

`--force` remains reserved for overwriting existing output files. The first Product Sync implementation has no ignore-freshness override.

## Vendor and Provider

- **Vendor**: accounting supplier identity in Invoice Ninja.
- **Provider**: configured external catalogue/API source.
- A Vendor may exist without a Provider.
- Each configured Provider has one canonical official supplier name and optional aliases.
- Aliases are recognition inputs only; Store is separate metadata and must not become part of Vendor identity.
- Unknown/non-provider Vendors remain valid accounting entities, but Product Sync skips them.

Provider alias matching trims surrounding whitespace, compares case-insensitively, performs no fuzzy matching and never learns aliases automatically.

## Product identity

Invoice Ninja Product identity is the combination:

```text
(Vendor, Product)
```

- **Vendor** is the native Invoice Ninja `vendor_id`, referring to the canonical supplier Vendor record.
- **Product** is the native Invoice Ninja `product_key`, holding the supplier's exact identifier (SKU, I/N, PartNo, item code, stock code, etc.).
- Do not prepend Provider-specific prefixes such as `BUNNINGS-` to new identities.
- Both fields participate in matching and deduplication. Store, Last Sync Date, Not Available and Supply Unit never participate in identity.

The native `vendor_id` association is the source of supplier identity; do not duplicate it in a `Supplier` Product custom field. Preflight must verify that the installed Invoice Ninja Product API persists and returns `vendor_id` correctly.

### Existing Bunnings Product migration

Existing `BUNNINGS-<item number>` keys and `bunnings_in` Product custom values must be recognised during the migration. Identify the original supplier item number, resolve the canonical Bunnings Vendor, and migrate the existing Product record to native `(vendor_id, product_key)` identity without unnecessary replacement or duplication.

Handle collisions, missing identifiers or ambiguous Vendor mapping explicitly; do not guess. Preserve existing Product attributes. Retire the old `bunnings_in` custom-field mapping after a safe, verified transition.

## Native Product fields and custom fields

Use Invoice Ninja native Product fields wherever they already express the concept:

| Concept | Storage |
|---|---|
| Supplier | Native `vendor_id` |
| Supplier product identifier | Native `product_key` |
| Image URL | Native `product_image` |
| Store | Product custom field 1 |
| Not Available | Product custom field 2 |
| Supply Unit | Product custom field 3 |
| Last Sync Date | Product custom field 4 |

The existing Product custom `image_url` mapping is retired in favour of native `product_image`. Preserve existing image URLs during migration. The installed Invoice Ninja instance is confirmed to support URLs in `product_image`.

### Store

Optional provenance/location metadata, e.g. `Castle Hill`. Not part of Vendor identity or Product matching.

### Not Available

A boolean/toggle field for current *known* Product availability:

```text
provider positively confirms not available -> true
provider positively confirms available     -> false
provider/API/source request fails           -> unchanged
availability cannot be determined           -> unchanged
```

A Product may disappear and reappear. On confirmed reappearance retain its identity, set Not Available to false and record the successful sync date. Do not delete/recreate Products because availability changed. Do not use `discontinued` as a generic state.


### Supply Unit

A **free-form text field** for the Product's supply unit or pack description. The user will experiment with values before deciding on any convention. Preserve user-entered content; do not require a format, validate or parse units, perform conversions or add pack-size calculation logic in v0.5.7. It is metadata only and never part of Product identity.

### Last Sync Date

The local calendar date on which that individual Invoice Ninja Product was last successfully synchronised against Provider data. It belongs to the Product, not its API request, source file, list or catalogue.

- Multiple observations of the same Product resolve to the same `(Vendor, Product)` identity and one Last Sync Date.
- Do not advance the date on an attempted or failed refresh.
- A Provider/API/source failure does not update the date.
## Existing-Product refresh ordering

Invoice Ninja Products form the durable refresh queue:

1. Load existing Invoice Ninja Products.
2. Resolve each Product Vendor to a configured Provider.
3. Skip Products with no supported Provider sync method.
4. Sort Products missing Last Sync Date first.
5. Then sort remaining Products by Last Sync Date, oldest first.
6. Refresh in that order.
7. Advance an individual Product's Last Sync Date only after that Product successfully processes.

No separate Product scheduling database is needed.

## Syncable Invoice Ninja Product fields

Sync is responsible for catalogue/Product data, not supplier inventory. It may update relevant fields:

```text
Product (product_key)
Description
Cost
Price
Quantity
Image URL (product_image)
Vendor (vendor_id)
Product custom fields
```

`Quantity` means the Invoice Ninja Product/default line-item quantity where Provider data has the same meaning; it is not supplier stock-on-hand. Inventory, stock levels, store-level stock availability and stock notifications are deferred.

## Freshness

### API/product-oriented Providers

For API-backed Providers such as Bunnings, freshness is Product-oriented. Each Product's Last Sync Date determines relative refresh age.

### File-backed Providers

For downloaded catalogues (CSV/XLSX), source freshness uses a deterministic content hash:

1. Obtain/download the source.
2. Use HTTP ETag/Last-Modified as optional optimisation hints, not the only truth.
3. Calculate the content hash and compare with the **last successfully processed** hash.
4. If unchanged, skip unnecessary catalogue processing.
5. If changed, parse the source once and process its affected Products individually.
6. **Only after every required Product has been successfully processed**, persist the new successful content hash to disk.
7. If any Product fails, do **not** advance the persisted successful hash. The next run retries the catalogue, including Products already refreshed successfully. Reprocessing those Products is acceptable.

Avoid a complex partial-progress system. Supplier catalogue file sizes do not warrant per-product retry checkpoints.

### Source-fingerprint cache

A source check and Product sync are different events. Checking an unchanged source may update non-authoritative check metadata, but it must not change Product Last Sync Date or mark an incompletely processed source as successful.

A narrow persistent cache may record Provider identity, source URL, ETag, Last-Modified, content hash and last successful source check. **Only a hash representing fully successful Product processing may be used to skip future processing.** Any optional fetched-but-unprocessed fingerprint must not become the successful fingerprint.

The cache must not become a second Product catalogue, Product identity database, accounting database or alternative Product-state store. Product state and sync metadata remain in Invoice Ninja.

## Provider states

Internally distinguish `available`, `not_available`, `unknown` and `error`. A failed request must never be interpreted as `not_available`.

## Store analytics

Store/location data stays independent of canonical Vendor identity. Analytics may group by Vendor, Store or Vendor + Store without changing Product identity.

## Persistence

Product state and sync metadata belong in Invoice Ninja Product records. Do not add a GoTradie Product-state database. Only the narrow file-source fingerprint cache above is allowed.

## Provider field mapping

Built-in Providers may define source-to-Invoice-Ninja Product mappings in code. Configurable file/web Providers define mappings using Invoice Ninja Product concepts:

```yaml
fields:
  product: PartNo
  description: Description
  cost: TradePrice
  price: RetailPrice
  quantity: PackQuantity
  image_url: ImageURL
```

`product` is mandatory for a configurable syncing Provider. The Provider canonical `name` supplies Vendor identity, without repeating it in each source row. The `image_url` source-mapping concept targets native `product_image` and is **not** a Product custom field.

Last Sync Date and Not Available are derived from sync results, not blindly copied from source files. Supply Unit is initially arbitrary user-editable metadata; do not impose a parsing scheme.

## Deferred scope

Supplier inventory/stock-level synchronisation is deferred until a Provider model can truthfully represent location-specific and time-sensitive stock.
