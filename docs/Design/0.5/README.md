# GoTradie v0.5 Design Series

This directory groups design and closeout material for the GoTradie `v0.5.x` release series.

## Releases

| Version | Status | Scope |
|---|---|---|
| v0.5.1 | Closed | Historical Expense import and supplier-account settlement |
| v0.5.2 | Implemented on `feature/ninja-expense-import-3` — pending review/merge | Expense import identity, preflight, UX, batching and receipt deduplication |
| v0.5.3 | Planned — implementation-ready | Hierarchical YAML configuration |
| v0.5.4 | Planned — implementation-ready | Accounting Dataset and BAS XLSX export |
| v0.5.5 | Planned — implementation-ready | EOFY preparation XLSX export |
| v0.5.6 | Planned — implementation-ready | Financial data XLSX export |
| v0.5.7 | Planned — implementation-ready | Product synchronisation and provider/catalogue sources |

See `Cross-Audit.md` for accepted cross-slice reconciliations.

## v0.5.3

Configuration precedence is:

```text
defaults
  ↓
~/.GoTradie/config.yaml
  ↓
explicitly supported secret environment variables
```

The YAML configuration file is mandatory.

Environment variables may override explicitly supported secrets only. They are not a second general-purpose configuration mechanism.

The configuration must include BAS frequency and GST basis.

Supported BAS frequencies:

```text
monthly
quarterly
yearly
```

Supported GST bases:

```text
cash
accrual
```

Provide structural homes for `invoice_ninja`, `tax`, `bas`, `eofy`, `product_sync`, and `providers`.

Provider config includes one canonical name and accepted aliases.

## v0.5.4

Introduce the shared Accounting Dataset and BAS XLSX output.

With no period flags, BAS exports the most recently completed BAS cycle according to configured BAS frequency.

## v0.5.5

EOFY preparation workbook using the Accounting Dataset.

With no period flags, EOFY exports the most recently completed financial year.

Existing Invoice Ninja Expense Categories are authoritative.

Capital/asset review may use a configurable instant asset write-off threshold, but GoTradie does not calculate depreciation.

## v0.5.6

A deliberately simple financial-data workbook.

With no period flags, export all available financial data.

Optional `--fy` or `--from/--to` restrict the dump.

First version is raw data/diagnostics, not dashboards.

## v0.5.7

Product Sync is invoked with:

```text
GoTradie sync refresh [--commit]
```

With no `--commit`, the command previews proposed Invoice Ninja changes.

Bunnings/API-backed freshness is product-oriented.

File-backed provider freshness is source-oriented and may use source metadata plus a durable content fingerprint to avoid reparsing unchanged source files.

Invoice Ninja Product identity is:

```text
Supplier + Product
```

where `Product` is the supplier SKU and `Supplier` is the canonical supplier name.

`Store`, `Last Sync Date`, and `Not Available` are Product metadata and do not participate in product matching or deduplication.

A Vendor may exist without a configured Provider.

Unknown/non-provider Vendors remain valid accounting entities but are skipped by Product Sync.

## Local export overwrite rule

BAS, EOFY and Financial exports follow the global CLI contract:

```text
new local output              -> create normally
existing local output         -> refuse
existing local output + force -> overwrite
```

`--commit` is unrelated to local file overwrite behaviour.
