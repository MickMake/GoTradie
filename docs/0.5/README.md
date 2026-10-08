# GoTradie v0.5 Design Series

This directory groups design and closeout material for the GoTradie `v0.5.x` release series.

## Releases

| Version | Status                         | Scope |
|---|--------------------------------|---|
| v0.5.1 | Closed                         | Historical Expense import and supplier-account settlement |
| v0.5.2 | Closed                         | Expense import identity, preflight, UX, batching and receipt deduplication |
| v0.5.3 | Planned — implementation-ready | Hierarchical YAML configuration |
| v0.5.4 | Planned — implementation-ready | Accounting Dataset and BAS XLSX export |
| v0.5.5 | Planned — implementation-ready | EOFY preparation XLSX export |
| v0.5.6 | Planned — implementation-ready | Financial data XLSX export |
| v0.5.7 | Planned — implementation-ready | Product synchronisation and provider/catalogue sources |


Release/slice-specific design and closeout material for the `v0.5.x` series.

### [v0.5.1 Expense Importing](./0.5.1/Expense-Importing.md)

Accepted architecture for historical Expense import and supplier-account settlement. It locks in:

- Invoice Ninja as the durable source of truth after import;
- spreadsheets as migration sources which may be archived after validation;
- Expenses as purchase and tax records;
- Bank Transactions/Transactions as supplier-account withdrawals and settlement records;
- customer Payments as money received, never supplier payments;
- deterministic supplier/date/FIFO reconstruction from Invoice Ninja records and stable GoTradie markers;
- tolerance of intermediate Unpaid state during partial settlement;
- optional Paid-state tidying only after full settlement;
- cash and non-cash BAS timing without double-counting;
- the Bunnings and BlueCarve canonical examples;
- an Invoice-Ninja-only target architectural/post-migration integrity test;
- explicit rejection of a GoTradie cache/SQLite side ledger, duplicate settlement Expenses, misuse of customer Payments, and required native partial links.

The document supersedes earlier spreadsheet-dependent or import-run-only allocation designs. The Invoice-Ninja-only integrity test describes the target post-migration architecture; it does not block completion of the current historical importer.

Related v0.5.1 records:

- [Possible Future Scenarios](./0.5.1/Expense-Importing-Possible-Scenarios.md) — explicitly deferred, non-blocking scenarios.
- [Implementation Review](./0.5.1/Expense-Importing-Review.md) — historical review evidence, not an open work list.

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

The configuration must include BAS reporting period and GST basis.

Supported BAS reporting periods:

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

BAS period selection is driven by configured `bas.reporting_period`.

With no `--fy`, BAS uses the current Australian financial year.

With no `--period`, BAS uses the current reporting period within that financial year.

`--period` means:

```text
monthly   -> 1-12
quarterly -> 1-4
yearly    -> not applicable
```

BAS does not support arbitrary `--from/--to` date ranges. Arbitrary date-range export belongs to the Financial/dump export.

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
