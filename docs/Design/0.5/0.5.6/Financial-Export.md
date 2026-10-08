# GoTradie v0.5.6 - Financial Data Export

## Status

**Planned - implementation-ready design**

## Purpose

Produce a general-purpose XLSX export of financial data from Invoice Ninja.

The first implementation is deliberately a **raw data and diagnostic export**, not a dashboard or management-analysis engine.

## Command

Default:

```text
GoTradie ninja export financial
```

With no selection flags, export all available financial data.

Financial export supports two mutually exclusive selection styles.

## BAS-style selection

Support:

```text
--fy YYYY
--period VALUE
--fy YYYY --period VALUE
```

Use the same period parsing as BAS.

Monthly:

```text
1..12
Jan..Dec
January..December
```

Quarterly:

```text
1..4
Jan..Dec
January..December
```

Yearly:

```text
--period invalid
```

`--period` is case-insensitive.

Where BAS-style selection is used, use the same FY and period mapping rules as BAS.

## Explicit date-range selection

Also support:

```text
--from YYYY-MM-DD
--to YYYY-MM-DD
--from YYYY-MM-DD --to YYYY-MM-DD
```

BAS-style selectors and explicit date-range selectors are mutually exclusive.

Reject combinations such as:

```text
--fy 2025 --from 2025-01-01
--period 2 --to 2025-06-30
--fy 2025 --period 2 --from 2024-10-01 --to 2024-12-31
```

There is no `--all` flag because all data is already the default.

## Output

Generated Financial output follows the global output-directory and overwrite rules in `docs/Design/Command-Line-Spec.md`.

If no explicit output path is supported or supplied:

```text
exports.directory, if configured
otherwise current working directory
```

Filenames are deterministic.

Financial-year export:

```text
FY2027-Financial.xlsx
```

Explicit bounded date range:

```text
Financial-2025-01-01-to-2025-06-30.xlsx
```

One-sided ranges:

```text
Financial-from-2025-01-01.xlsx
Financial-to-2025-06-30.xlsx
```

Unrestricted export:

```text
Financial-All.xlsx
```

Do not append timestamps or automatic collision suffixes.

## Workbook approach

Prefer faithful, well-structured data sheets over dashboards.

Expected sheets include the equivalent of:

```text
Income
Expenses
Payments
Supplier Transactions
Customers
Vendors
Products
Projects-Jobs
Exceptions
```

## Raw data and relationships

Preserve useful source fields and relationships, especially:

```text
Invoice Ninja record IDs
customer/vendor references
invoice/payment relationships
expense/supplier relationships
project/job references
product references
dates
amounts
GST/tax values
status/state
archived/deleted markers where relevant
```

## Filtering

When a date restriction is supplied, apply a deterministic documented date rule per entity.

Do not silently omit undated or unclassifiable records. Surface materially ambiguous cases.

## Accounting Dataset

Reuse the Accounting Dataset where it provides useful calculated financial values, while preserving raw Invoice Ninja data where needed for diagnosis.

## Deferred analysis

The first version does not need dashboards, charts, margin calculations, trend analysis, KPI frameworks, concentration analysis, or management commentary.

## Scope guardrail

Do not turn this slice into a dashboard project, KPI framework, BI system, general ledger, or second accounting database.

## Design rule

> Export the data cleanly first. Analyse it later when there is a real reason.
