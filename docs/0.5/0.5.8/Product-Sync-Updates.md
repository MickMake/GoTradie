# GoTradie v0.5.8 - Product Sync Updates

## Status

**AGREED DESIGN - implementation pending.** This is the authoritative v0.5.8 Product Sync design. It supersedes the v0.5.7 CLI and provider configuration only where explicitly stated below. The existing [v0.5.7 Product Sync design](../0.5.7/Product-Sync.md) continues to govern product identity, vendor validation, availability, freshness and safety invariants.

## Purpose and scope

v0.5.8 is primarily a **configuration and CLI integration** update to the existing Product Sync implementation. Reuse current GoTradie, GoBunnings and Invoice Ninja facilities. Make the smallest code changes necessary for the features specified here; do not refactor, extend or redesign unrelated behaviour.

- Select the supplier by a positional CLI argument and opt providers in to all-provider sync explicitly.
- Choose the existing Bunnings API or Web adapter using one Bunnings provider's configuration.
- Allow CSV/XLSX catalogue columns to feed both Invoice Ninja Product data and configurable Description formatting.
- Apply one shared Description template mechanism to all providers.
- Keep catalogue retrieval transport independent of CSV/XLSX decoding.

## 1. Product Sync CLI

```text
GoTradie sync [provider] [product-key] [--commit]
```

- `GoTradie sync` selects only providers with `auto_sync: true`; the default is `false`.
- `GoTradie sync <provider>` selects that provider only, regardless of `auto_sync`.
- `GoTradie sync <provider> <product-key>` selects one product. A missing Product can be created on `--commit`.
- Every form previews by default. Only `--commit` permits Invoice Ninja writes.
- A CSV/XLSX provider with no product key synchronises its catalogue entries. A Bunnings provider with no key refreshes **existing Invoice Ninja Products only**; never enumerate the Bunnings catalogue.
- Never delete Invoice Ninja Products merely because a catalogue row is absent.

Examples:

```bash
GoTradie sync
GoTradie sync --commit
GoTradie sync nst
GoTradie sync nst BBD025100 --commit
GoTradie sync bunnings
GoTradie sync bunnings 2582847
GoTradie sync bunnings 2582847 --commit
```

The target interface replaces legacy Product Sync `sync refresh`, `sync import <IN>` and sync-specific `--web` selection. Do not add `--provider` or a second `bunnings-web` provider. The existing `GoTradie bunnings find/get/lookup` commands are separate, read-only Bunnings utilities and are **not redesigned by this slice**.

These commands are independent native Invoice Ninja maintenance operations and remain unchanged:

```text
GoTradie ninja export products products.csv
GoTradie ninja import products products.csv
```

Their Product CSV import/export is **not** supplier catalogue import and must not consume provider configuration or Description templates.

## 2. One Bunnings provider, one mode

Bunnings is a special-case built-in adapter with one canonical Supplier and a mutually exclusive mode setting. Do not add a generic web-scraping provider or two separate Bunnings providers.

```yaml
providers:
  bunnings:
    name: Bunnings
    type: bunnings
    mode: api            # api | web
    auto_sync: true
    description_template: '{{.description}}'
    # Existing Bunnings API settings belong here when mode is api.
```

- `mode: api` uses the existing Query Item/Pricing API adapter.
- `mode: web` uses the existing Bunnings website adapter.
- Both modes use the same `(Supplier, product_key)` identity and the existing sync/update pipeline.
- Validate settings required by the selected mode, rather than requiring API credentials for Web mode.
- Switching modes must not create a second Supplier, migrate Product identities, create new persistence or introduce mode-specific cache logic.
- Do **not** add Bunnings API endpoints, requests, field extraction, product search, or other Bunnings features beyond the minimum wiring for mode selection and Description templating.

## 3. Catalogue configuration and field mapping

A CSV/XLSX provider uses `fields` to map supplier column headers onto predictable GoTradie field names. The same mapped values can be used by the Description template; there is no second `extras` configuration.

For the NST catalogue, the source headers are:

```text
CATEGORY, CODE, DESCRIPTION, UNIT, PRICE (INC GST), PRICE (EX GST)
```

Example:

```yaml
providers:
  nst:
    name: North Shore Timber
    type: csv
    aliases: [NST]
    auto_sync: true
    url: https://www.nst.net.au/nst/DownloadCSV
    fields:
      product: CODE
      description: DESCRIPTION
      price: 'PRICE (INC GST)'
      category: CATEGORY
      unit: UNIT
    description_template: '{{.description}}{{if .category}} - {{.category}}{{end}}{{if .unit}} ({{.unit}}){{end}}'
```

Core mappings:

| GoTradie field | Source/meaning | Handling |
| --- | --- | --- |
| `product` | Supplier SKU/I/N/item identifier | Required for file providers; goes to native Invoice Ninja `product_key`. |
| `description` | Base Product description | Required for file providers; formatted into Invoice Ninja Description. |
| `price` | Public GST-inclusive retail price | Required for the supported NST/file pricing route; goes to native Product Price. |
| `supplier` | Configured canonical Supplier name | System-supplied, not mapped from a CSV column; validates against one active Invoice Ninja Vendor. |

Existing supported mapped concepts such as `cost` and `image_url` keep their v0.5.7 meanings. Do not map NST's `PRICE (EX GST)` to `cost` or infer supplier discount from it. Extra mappings such as `category`, `unit`, or future supplier-specific headers are **template inputs only** unless an explicit core Product mapping already exists. They must not silently create Product custom fields.

**GST boundary:** The supported NST mapping uses `PRICE (INC GST)`. Exclusive-only supplier prices, GST conversion and changes to Invoice Ninja Tax fields are deferred. An exclusive price does **not** imply a zero GST rate. Do not implement new tax rules in v0.5.8.

## 4. Common Description templates

All providers (Bunnings API, Bunnings Web, CSV and XLSX) use the **same rendering mechanism**. Templates reference GoTradie's mapped field names, not raw CSV column names or Bunnings API JSON paths.

```yaml
description_template: '{{.description}}'
```

- No configured template means exactly the base `description` (equivalent to `{{.description}}`).
- Use Go standard `text/template`, with substitution and simple conditionals. Do not create a general scripting language or add a templating dependency.
- The template context uses the configured, case-sensitive, lowercase GoTradie keys shown here (`.description`, `.unit`, etc.).
- Any mapped CSV/XLSX field may be referenced in its provider's template; there is no separate list of permitted extra columns.
- `supplier` comes from the provider; the other core values come from the existing adapter or source mapping.
- Reject unknown/unmapped tags with a clear configuration error rather than silently writing malformed descriptions.
- Known optional fields missing from an individual record produce an empty value, so `{{if .unit}}...{{end}}` works.
- Render only the native Invoice Ninja Product Description. Do not change matching, image storage, pricing, tax, Vendor records, availability, or custom-field allocation.

### Built-in Bunnings template values

Bunnings has a built-in field mapping and **does not need configurable `fields` headers**. Only values already exposed by the current adapters are in scope:

| GoTradie template key | Bunnings API source | Bunnings Web source | Notes |
| --- | --- | --- | --- |
| `.product` | `itemNumber` | Website SKU/item number | Already stored as Product key. |
| `.description` | `description.productDescription`, falling back to `description.sectionDescription` when blank | Website product description | **Default and only content when no template is configured.** |
| `.unit` | `saleUnitOfMeasure` | Website Unit, when available | Optional; may be empty on Web. |
| `.price` | Existing Pricing adapter | Existing website price | Use existing semantics; **no** GST changes. |
| `.image_url` | Existing derived image URL | Existing website image URL | Existing native image storage remains unchanged. |
| `.supplier` | Canonical configured name | Canonical configured name | System-provided. |

Do not append the Bunnings I/N to Description: the native Product field already shows it. Do not prepend `Title`, combine the two descriptions, or add special Bunnings formatting. The existing `productNotes` behaviour must simply be replaced by the common Description-rendering path; the default result is **Description alone**.

The Bunnings API also contains other metadata (for example `brand`, category `familyTree`, barcodes, packaging, supplier information and compliance/document references). They are **not automatically template keys**. Do not extend adapters merely because the API schema describes those fields.

## 5. Retrieval and decoding

The configured provider `type` selects the existing CSV or XLSX **decoder**. Its `url` scheme selects the **retrieval transport**.

| `url` scheme | Retrieval |
| --- | --- |
| `https://`, `http://` | Existing remote download |
| `ftp://` | Read-only public/anonymous FTP download |
| `file://` | Local file (`file:///absolute/path/catalog.csv`) |

Both existing decoders work with all supported transports. Do not introduce legacy XLS decoding, a generic downloader plugin framework, FTP credential management, or an additional `--file` CLI switch. If FTP requires a dependency, include it in the preflight proposal and obtain approval before adding it.

Preserve v0.5.7's narrow **file-provider** successful-fingerprint safeguard. Changes to `fields` **or to `description_template`** must cause an unchanged CSV/XLSX file to be processed under the new mapping/template; do not advance its successful fingerprint on partial failure. This is specific to existing catalogue-file handling, **not** a request to add Bunnings caching.

## 6. Existing Product Sync invariants

The following continue unchanged from v0.5.7:

- Identity is `(canonical Supplier stored in Product custom_value1, native product_key)`; native Product `vendor_id` is not used for matching.
- Resolve each selected Supplier unambiguously against an active Invoice Ninja Vendor; lookup is validation, not a Vendor write.
- Preserve configured aliases, duplicate rejection, absence/deletion rules, native `product_image`, availability handling, Last Sync Date semantics and oldest-first refresh ordering.
- On an API/source failure, preserve the previous price, availability and successful-sync date. A failed lookup is not evidence that a Product is unavailable.
- No second Product database; no new Bunnings cache, migration or identity system.
- `--commit` authorises Invoice Ninja writes; `--force` applies only to permitted local-output overwrites, as specified in the global CLI contract.

## 7. Explicit exclusions and later ideas

**Not part of v0.5.8:** Richer `bunnings lookup` output (brand/manufacturer, category hierarchy, barcodes, physical dimensions, weight, pack quantities, reference documents), store-specific stock levels, product alternatives/recommendations, generic website extraction, new Bunnings API endpoints or field extraction, new Invoice Ninja setup/check commands, new GST pricing rules, legacy XLS, or any unrelated refactor.

These are ideas for separately approved future work, **not an implementation queue**. The sandbox API includes test placeholders, empty values and zero measurements; do not treat its sample metadata as reliable production evidence.

## 8. Acceptance evidence

- [ ] `sync` handles no-provider opt-in, one provider, one product key and dry-run/`--commit` correctly.
- [ ] File suppliers create/update catalogue entries, while Bunnings without a key refreshes only existing Products; no auto-delete.
- [ ] One Bunnings provider chooses exactly one existing adapter with `mode: api | web`; identity and Vendor validation are unchanged.
- [ ] Built-in Bunnings Description defaults to `productDescription` with `sectionDescription` fallback, **without** Title or appended I/N.
- [ ] The common Description template works for Bunnings API/Web, CSV and XLSX, with optional values/conditionals and clear unknown-tag errors.
- [ ] NST fields support `product`, `description`, GST-inclusive `price` and extra template fields `category`/`unit` without GST conversion.
- [ ] File-source fingerprint reacts to changes in mapping or Description template, while preserving complete-success semantics.
- [ ] CSV/XLSX work through HTTP(S), public FTP and `file://` with appropriate errors.
- [ ] Native `ninja import/export`, existing `bunnings find/get/lookup` and unrelated functionality remain unchanged.
- [ ] Tests verify no extra Bunnings API functionality, Product matching changes, database, stock subsystem or speculative abstractions were introduced.

**Core principle:** Reuse what exists. One sync pipeline, simple provider configuration and one shared Description renderer. No 'while I am here' changes.
