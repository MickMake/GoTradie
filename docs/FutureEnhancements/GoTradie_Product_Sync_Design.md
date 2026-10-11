# GoTradie - Product Sync CLI and Provider Design

**Status:** Agreed design proposal; implementation pending  
**Context:** GoTradie v0.5.7 Product Sync  
**Date:** 12 October 2026

## 1. Purpose

Provide a simple, supplier-aware way to bring products and public retail prices into Invoice Ninja and keep them up to date.

GoTradie is **not** intended to harvest every product from every supplier's API or become a generic website crawler.

The design distinguishes:

1. **Source retrieval** - how data is obtained (HTTP, HTTPS, FTP, local file).
2. **Source decoding** - how data is interpreted (CSV, XLSX, XLS, or a supplier API).
3. **Field mapping** - how source fields become GoTradie product fields.
4. **Product synchronisation** - identity matching, change detection, dry-run and updates in Invoice Ninja.

These are separate responsibilities. Retrieval should not dictate file format, and file decoding should not dictate sync behaviour.

## 2. CLI

### Proposed command

```text
GoTradie sync [provider] [product-key] [--commit]
```

- **Dry-run by default.** No Invoice Ninja writes unless `--commit` is supplied.
- **No provider specified:** Run only configured providers with `auto_sync: true`.
- **Provider specified:** Run that provider explicitly, whether or not `auto_sync` is enabled.
- **Product key specified:** Sync only that product from the selected provider. An explicitly requested product may be created if missing from Invoice Ninja.

### Examples

```bash
# Run all providers opted in to automatic sync (dry-run)
GoTradie sync

# Run all opted-in providers and write changes
GoTradie sync --commit

# Sync NST's complete catalogue
GoTradie sync nst

# Sync one NST catalogue item
GoTradie sync nst 23423434

# Refresh Bunnings products already recorded in Invoice Ninja
GoTradie sync bunnings

# Fetch and sync one Bunnings product by item number
GoTradie sync bunnings 2582847

# Use the specialised Bunnings website adapter instead
GoTradie sync bunnings-web 2582847

# Explicitly commit a single-product update
GoTradie sync bunnings 2582847 --commit
```

`GoTradie sync bunnings` **must never attempt to import the entire Bunnings catalogue**.

### Commands kept separate

```bash
GoTradie ninja export products products.csv
GoTradie ninja import products products.csv
```

These are direct **Invoice Ninja maintenance/import-export tools**, intended for backup, editing, migration and repair. They remain independent of supplier provider configuration and are **not** replaced by Product Sync.

### Legacy Product Sync commands

The proposed unified CLI replaces the user-facing concepts of:

- `sync refresh`
- `sync import <bunnings-item-number>`
- `--web` for selecting Bunnings website lookup

A compatibility/deprecation period is an **implementation decision not yet made**. The target interface should not require those forms.

## 3. Provider behaviour is determined by source type

No separate `scope` setting is required.

| Provider source | `GoTradie sync <provider>` | `GoTradie sync <provider> <product-key>` |
| --- | --- | --- |
| CSV catalogue | Process all catalogue records | Select one matching record |
| XLSX catalogue | Process all catalogue records | Select one matching record |
| Other supported spreadsheet format (e.g. XLS) | Process all catalogue records | Select one matching record |
| Supplier API | Refresh **only existing Invoice Ninja products** belonging to that supplier | Fetch and sync the specified product; create if missing |
| Specialised Bunnings web adapter | Refresh **only existing Bunnings products** | Look up the specified Bunnings product |

For catalogue files, processing all records means **create new / update existing** as appropriate. It does **not** imply deleting Invoice Ninja products that are absent from the latest file.

For API/website lookup providers, product keys for bulk refresh come from **Invoice Ninja's existing product list**, filtered by supplier identity. Do not enumerate or scrape an entire online catalogue.

For a single product from a catalogue file, it is acceptable to retrieve and parse the catalogue to find that one key, but **only that product is synchronised**.

## 4. `auto_sync` - explicit opt-in

```yaml
auto_sync: true
```

- Boolean provider setting.
- **Default: `false`.**
- Used **only** when running `GoTradie sync` without a provider name.
- Does not prevent `GoTradie sync <provider>` from running explicitly.
- Normally only one source for a given supplier should participate in the automatic sync. For example, Bunnings API on; Bunnings website off.

No `include_in_all` and no `scope` setting.

## 5. Proposed provider configuration

```yaml
providers:
  bunnings:
    name: Bunnings
    type: api
    auto_sync: true
    # Existing Bunnings API authentication/environment settings go here.

  bunnings-web:
    name: Bunnings
    type: web
    auto_sync: false
    url: https://www.bunnings.com.au
    # Uses a Bunnings-specific website extraction adapter.

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
```

**This configuration is illustrative of the agreed target design**, not a claim that all keys/types are already accepted by v0.5.7.

The NST mapping deliberately does **not** map `PRICE (EX GST)` to `cost`: the requirement is to sync the **public GST-inclusive selling price** shown to customers. The supplier discount is not represented by this mapping.

Other NST columns currently left unmapped:

- `CATEGORY` and `UNIT` are valuable and should eventually appear in product descriptions, but description templating is **deferred**.
- `quantity` is **optional** and left unmapped when supplier availability is not supplied. GoTradie should not attempt to manage the tradie's own stock from supplier catalogues.

### Provider identity versus source

`bunnings` and `bunnings-web` are **different source configurations for the same supplier**.

Both must resolve to the same Invoice Ninja product identity:

```text
Canonical Supplier + Product Key
```

They must not create duplicate products merely because one uses the API and the other uses the website. Supplier/vendor names and configured aliases follow the existing Product Sync identity rules.

## 6. Catalogue retrieval and decoding

`url` describes **where to obtain the source**, not how to parse it.

Intended URL schemes:

- `https://` and `http://` - download a catalogue.
- `ftp://` - retrieve a catalogue from an FTP server.
- `file://` - read an already downloaded local catalogue.

The source format (`type: csv`, `type: xlsx`, etc.) selects the decoder independently of the retrieval mechanism. All supported catalogue formats use the same `fields` mapping structure.

**Implementation boundary:** v0.5.7 already supports configured CSV/XLSX HTTP(S) catalogue workflows. Additional formats such as legacy XLS and retrieval schemes such as FTP and `file://` are **planned capabilities, not confirmed implemented features**. Add them as actual supplier needs arise.

A separate `--file` CLI override has **not** been agreed as a requirement; local files can be modelled as provider URLs when `file://` support is implemented.

## 7. Website scraping

**Generic web product discovery/scraping is out of scope unless a real supplier requires it.**

Reasons:

- Suppliers use different page layouts, structured data, scripts and product identifiers.
- Products may be distributed across category pages and multiple pagination levels.
- Search strategies, pagination, category crawling and extraction rules are separate, potentially large projects.
- Sites can change without notice and break extraction.

The existing **Bunnings-specific website lookup** may be retained as a separate provider named `bunnings-web`. It is not a commitment to build a generic `type: web` crawling framework. A website provider needs its site URL, but URL alone is insufficient to define search and extraction behaviour.

For now, prioritise API providers and supplier-provided catalogue files.

## 8. Common Product Sync rules

Regardless of source:

1. Resolve supplier and product identity consistently using **Supplier + Product Key**.
2. Normalise provider results into the same internal product representation.
3. Compare against Invoice Ninja before proposing changes.
4. Report proposed creates/updates/errors in dry-run mode.
5. Write only when explicitly passed `--commit`.
6. Preserve supplier identity and existing Product Sync protections against ambiguous matches.
7. **Do not automatically delete** products simply because they are absent from a supplier's newest catalogue.
8. Keep GST handling consistent with the agreed public **GST-inclusive retail-price** convention; verify API price semantics per provider before assuming its values are GST-inclusive.

This pipeline should be shared. A new CSV or API provider should not need a new product-matching or update algorithm.

## 9. Explicit non-goals / deferred work

- Crawling the whole Bunnings catalogue (or any large supplier API).
- A general-purpose web crawler or configurable multi-site search engine.
- Category/page discovery rules for arbitrary retailers.
- NST `CATEGORY`/`UNIT` description templates (useful, but deferred until after testing).
- Automatic management of the tradie's stock quantity.
- Automatic deletion of unavailable or missing catalogue products.
- Redesigning `GoTradie ninja import/export`.
- Introducing a redundant `scope: catalogue|existing` configuration key.

## 10. v0.5.7 implementation gap / acceptance checklist

This document defines **desired behaviour**. The current v0.5.7 command handlers still use `sync refresh`, `sync import <Bunnings-IN>`, and `--web`; their behaviour must not be mistaken for the proposed CLI.

Implementation and tests should establish:

- [ ] `GoTradie sync` runs **only** providers with `auto_sync: true` (default false).
- [ ] `GoTradie sync <provider>` selects exactly that configured provider.
- [ ] `GoTradie sync <provider> <product-key>` syncs exactly one requested product.
- [ ] File catalogue providers enumerate all records when no key is supplied.
- [ ] API providers without a key refresh **only** their existing Invoice Ninja products.
- [ ] Both Bunnings source adapters use the same canonical supplier identity.
- [ ] `--commit` is the only switch that enables writes; dry-run remains the default.
- [ ] `ninja import/export` remains unchanged as a maintenance facility.
- [ ] File retrieval, decoding and field mapping are separable.
- [ ] Add `file://`, FTP and XLS only when implemented and tested; do not advertise prematurely.
- [ ] Missing catalogue rows do not trigger automatic product deletion.
- [ ] NST imports `CODE`, `DESCRIPTION`, and public `PRICE (INC GST)` correctly.
- [ ] Bunnings Live approval/credentials and any website extraction issues are treated as independent external/adapter concerns.

---

**Core design principle:** *One Product Sync engine; interchangeable data sources; simple commands; only sync what makes sense for each source.*
