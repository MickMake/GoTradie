# GoTradie - Product Sync CLI and Provider Design

**Status:** Agreed v0.5.8 design proposal; implementation pending  
**Context:** Follow-up to the closed v0.5.7 Product Sync slice  
**Date:** 12 October 2026

## 1. Purpose and boundaries

Bring supplier products and public retail prices into Invoice Ninja using a small, predictable CLI. Keep the existing v0.5.7 Product identity and synchronisation rules.

Separate four responsibilities:

1. **Retrieval:** HTTP, HTTPS, FTP, or a local file.
2. **Decoding:** CSV, XLSX, or a built-in supplier adapter.
3. **Field mapping and description formatting:** source columns become named GoTradie values; a template formats Description.
4. **Synchronisation:** match existing Invoice Ninja Products, preview changes, and optionally commit.

The retrieval scheme must not determine the decoder. Do not build a generic website crawler or a second Product database.

## 2. Product Sync CLI

```text
GoTradie sync [provider] [product-key] [--commit]
```

- No provider: run only providers with `auto_sync: true` (default `false`).
- Provider supplied: run exactly that configured provider, whether or not it is opted into automatic sync.
- Product key supplied: synchronise only that key under the selected provider. A missing Product may be created.
- Preview is the default. **Only `--commit` permits writes to Invoice Ninja.**
- Do **not** add `--provider` to `sync` or `ninja import`.

Examples:

```bash
GoTradie sync
GoTradie sync --commit
GoTradie sync nst
GoTradie sync nst BBD025100
GoTradie sync bunnings
GoTradie sync bunnings 2582847
GoTradie sync bunnings 2582847 --commit
```

The Bunnings source is selected through its configuration, **not** through `bunnings-web` as a separate provider and not through `--web` on `sync`.

### Separate Invoice Ninja maintenance commands

```text
GoTradie ninja export products products.csv
GoTradie ninja import products products.csv
```

These remain native Invoice Ninja import/export commands. Do not connect them to supplier providers, Product Sync field mappings, or description templates. Do not redesign them for this slice.

### Existing Product Sync forms

The target `sync` interface replaces the older `sync refresh`, `sync import <Bunnings-item>`, and `--web` source selection on Product Sync. Update affected help, tests, and CLI examples together; do not silently retain competing interfaces. Unrelated Bunnings commands are outside this CLI change.

## 3. Selection by source format

| Provider type | `sync <provider>` | `sync <provider> <product-key>` |
| --- | --- | --- |
| CSV/XLSX catalogue | Create/update products from catalogue records | Select only the matching catalogue record |
| API provider | Refresh only existing Invoice Ninja Products for the supplier | Fetch that product; create if missing |
| Bunnings `mode: api` | Refresh only existing Bunnings Products | Fetch that product through the Bunnings API |
| Bunnings `mode: web` | Refresh only existing Bunnings Products | Fetch that product through the existing Bunnings website adapter |

File catalogues may be retrieved and parsed when only one product key was requested, but only that one Product may be written. API-backed sources must not enumerate an entire supplier catalogue. Never delete Invoice Ninja Products merely because the latest file omits them.

## 4. Provider configuration

```yaml
providers:
  bunnings:
    name: Bunnings
    type: bunnings
    mode: api       # exactly one of: api | web
    auto_sync: true
    aliases:
      - Bunnings Warehouse
    # Keep existing API authentication/environment settings here.
    # API-only settings apply in api mode.

  nst:
    name: North Shore Timber
    type: csv
    aliases:
      - NST
    auto_sync: true
    url: https://www.nst.net.au/nst/DownloadCSV
    fields:
      product: CODE
      description: DESCRIPTION
      price: "PRICE (INC GST)"
      category: CATEGORY
      unit: UNIT
    description_template: '{{.description}}{{if .category}} - {{.category}}{{end}}{{if .unit}} ({{.unit}}){{end}}'
```

This YAML describes the agreed **target**, not a claim about what v0.5.7 already parses.

- `auto_sync` is an explicit opt-in, defaulting to `false`, and matters only when `sync` has no provider argument.
- Bunnings has **one** provider configuration and one canonical Supplier. `mode` selects either the existing API adapter or the existing Bunnings website adapter; never both in one run.
- `mode` is a Bunnings-specific option; do not add it to ordinary CSV/XLSX providers.
- Validate only credentials and settings relevant to the chosen Bunnings mode.
- Do not introduce a generic `type: web` implementation to support this special case.
- Do not add a redundant `scope` or `include_in_all` option.

## 5. Field mappings and description templates

`fields` maps source field names to **GoTradie names**. Known names feed Invoice Ninja Product fields; additional mapped names are available to `description_template` without creating more Invoice Ninja fields.

For NST the actual headings are:

```text
CATEGORY, CODE, DESCRIPTION, UNIT, PRICE (INC GST), PRICE (EX GST)
```

The example in section 4 maps `CODE`, `DESCRIPTION`, and `PRICE (INC GST)` to the required core Product data, plus `CATEGORY` and `UNIT` solely for description formatting.

### Core values

- `product` (required for a configurable catalogue): native Invoice Ninja `product_key`, the supplier's own SKU/item number.
- `description` (required): base Product description.
- `price` (required for the currently supported catalogue pricing path): public **GST-inclusive** retail price. NST uses `PRICE (INC GST)`.
- `supplier` (system-provided): canonical provider Supplier name, validated against Invoice Ninja Vendors. Do not map this from a CSV column.
- Existing supported Product mappings such as `cost` and `image_url` retain their v0.5.7 semantics. For NST, do not map the alternate tax-exclusive price into `cost`.

**Pricing boundary for v0.5.8:** Use the known GST-inclusive NST price. Handling an exclusive-only price, automatic GST conversion, or changing Invoice Ninja Tax configuration is **deferred**. An exclusive source price must not be interpreted as a 0% tax rate. See `docs/FutureEnhancements/Others.md` for later consideration.

### Additional fields

- Any additional `fields` entry (for example, `category` or `unit`) can be used in the template.
- The template uses GoTradie's **mapped keys**, not raw source column names. Keys are case-sensitive; use the same spelling as in `fields`.
- Bunnings API and Bunnings Web map their supported data into the same normalised values before templating. A supplier-specific adapter may provide additional named values; templating itself stays shared.
- `supplier` is always available to the template from provider configuration. Mapped `product` and `description` are always available when a record is valid.
- Do not guess CSV headings, add a separate `extras` hierarchy, or introduce a generic field-mapping language.

### Template behaviour

```yaml
description_template: '{{.description}}{{if .category}} - {{.category}}{{end}}{{if .unit}} ({{.unit}}){{end}}'
```

- Apply the same description-template mechanism to **all** providers, including Bunnings in either mode.
- When no template is configured, use the base `description` unchanged (equivalent to `{{.description}}`).
- Support ordinary Go `text/template` field substitution and conditionals. Avoid additional template dependencies or custom programming features.
- A template reference to an unknown/unmapped field is a configuration error.
- A supported optional field whose value is absent for an individual product is empty; the template can omit its surrounding punctuation using `if`.
- Description rendering must not alter Product identity, Supplier matching, or pricing.

The template defines **one final Invoice Ninja Description**; additional mapped fields are not separate custom fields.

## 6. Retrieval is independent from decoding

`type: csv` or `type: xlsx` selects the decoder. `url` selects the retrieval method.

| URL scheme | Retrieval |
| --- | --- |
| `https://` | Download remote source |
| `http://` | Download remote source |
| `ftp://` | Retrieve public catalogue via FTP |
| `file://` | Read local catalogue file |

The same CSV decoder must work with all supported retrieval schemes; likewise XLSX must not require a different retrieval architecture. In v0.5.8, add **FTP and local-file retrieval** to the existing HTTP(S) paths, with focused tests for CSV and XLSX.

Start with read-only public FTP catalogues; do not introduce FTP credential management without a real supplier need and separate approval. Local file URLs should be explicit `file:///absolute/path/catalog.csv` paths. No `--file` override is required.

Legacy XLS is **not** required for v0.5.8; add another decoder only when a real catalogue requires it. Do not introduce a generic downloader framework unrelated to these schemes.

## 7. Website-specific extraction

No generic website scraper, site search engine, page discovery, pagination system, or `type: web` framework.

Bunnings Web is an existing specialised adapter selected with `bunnings.mode: web`. For other websites, prefer small, independent supplier-specific utilities which produce a CSV/XLSX format GoTradie already understands. Such utilities are **not** part of v0.5.8.

## 8. Product identity and sync invariants

Preserve the accepted v0.5.7 design:

1. Identity is `(canonical Supplier, native product_key)`; no provider-specific prefix.
2. Supplier is stored in Product custom field 1; Store, Not Available, and Last Sync Date keep their established fields.
3. Validate the configured canonical Supplier (and allowed aliases) against **one unambiguous active Invoice Ninja Vendor** for every selected provider. Vendor lookup is validation only; do not rely on native Product `vendor_id`.
4. Recognised aliases may be normalised to the canonical Supplier after a successful Product update.
5. Reject duplicate supplier/key identities; do not create duplicates when switching Bunnings modes.
6. Preview proposed creates, updates, skips, and errors. Only `--commit` writes.
7. Preserve existing successful-sync-date, availability, error, and file-source freshness behaviour. Do not interpret source failure as unavailability or a successful sync.
8. Do not automatically delete Product records absent from a catalogue.
9. Do not add new Product-state persistence or additional Bunnings-mode cache handling.

Use the **existing** Product Sync matching/update pipeline rather than implementing a second algorithm per transport or adapter.

## 9. Related Invoice Ninja setup checks

`docs/FutureEnhancements/InvoiceNinjaSetup.md` describes separate `invoiceninja check` and `invoiceninja setup` operations, plus a read-only configuration preflight before Product Sync. When implementing those checks, retain its four Product custom-field definitions and Vendor validation rules.

The global CLI contract remains authoritative: **`--commit` is required for any Invoice Ninja configuration write**. A setup command must also request explicit confirmation; inspection must never change settings silently. Keep this facility separate from `ninja import/export` and Product Sync provider selection.

## 10. Deferred and explicitly excluded

- `--provider` on Product Sync or native Invoice Ninja import.
- Separate `bunnings-web` provider configuration.
- General-purpose website scraping or additional website adapters.
- Exclusive-only GST price conversion and Tax-field redesign.
- Legacy XLS support until a supplier requires it.
- Supplier stock-management features or automatic product deletion.
- A second Product identity, Product-state database, or new cache architecture.
- Any change to native `ninja import/export` semantics.
- New persistence, generic plugins, or configuration abstractions invented for hypothetical providers.

## 11. v0.5.8 acceptance checklist

- [ ] `GoTradie sync` honours `auto_sync: true` with default false.
- [ ] `GoTradie sync <provider> [product-key]` selects the specified provider and optional single product.
- [ ] `--commit` alone enables Invoice Ninja writes; no `--provider` flag.
- [ ] Bunnings has exactly one provider configuration and `mode: api | web` selection.
- [ ] Either Bunnings mode uses the same Supplier/product identity and does not duplicate Products.
- [ ] File catalogue mode creates/updates catalogue rows; API/Bunnings mode refreshes only known Products unless a key is explicit.
- [ ] Required mappings reject missing `product`, `description`, or supported `price` data.
- [ ] NST maps public `PRICE (INC GST)` to Product Price, without new GST calculations.
- [ ] `category` and `unit` are available in description templates; optional absent values format cleanly.
- [ ] A shared template renderer works for CSV, XLSX, and Bunnings; unknown tags fail clearly.
- [ ] CSV and XLSX decoding operate independently of HTTP(S), FTP, and local-file retrieval.
- [ ] Supplier/vendor validation and the existing v0.5.7 Product protections remain intact.
- [ ] Native `ninja import/export` remains independent and unchanged.
- [ ] CLI help, global CLI examples, tests, and release notes describe the actual implemented interface.
- [ ] No generic web scraper, speculative supplier connectors, or unsupported tax rules are introduced.

---

**Design principle:** One Product Sync engine, one Bunnings provider with two modes, interchangeable file transports, and templates over named GoTradie fields. Implement what is needed, not what might one day be impressive.
