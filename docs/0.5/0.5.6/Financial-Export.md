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

Generated Financial output follows the global output-directory and overwrite rules in `docs/Command-Line-Spec.md`.

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

The Summary or equivalent report-metadata area must expose `Report Status`.

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

Financial date filtering is source-record filtering.

Do not use BAS GST recognition dates or EOFY recognition dates to decide whether a raw Financial-export row belongs in a selected date range.

Use the entity's own source date:

```text
invoice / income record
    -> invoice date

customer payment
    -> payment date

expense
    -> expense date

supplier transaction / supplier payment
    -> transaction/payment date

quote, if included
    -> quote date
```

Reference/master entities are not date-filtered merely because the export has a date range:

```text
customers
vendors
products
projects/jobs
```

They may still be included when needed to preserve relationships or source context for the filtered transactional rows.

This means related records can legitimately fall on different sides of a range boundary.

Example:

```text
invoice date  = 28 June
payment date  = 5 July

July-only Financial export:
    invoice row  -> excluded from the invoice/income sheet
    payment row  -> included in the payments sheet
```

That is expected raw-data behaviour, not an inconsistency.

### Missing or ambiguous dates

Do not silently assign a different date merely to make a record fit the requested range.

If a transactional record that requires date filtering has no usable source date, surface it in Exceptions with enough source identity to diagnose it.

Where an entity has multiple source dates, use the specific date rule above rather than choosing whichever date happens to fall inside the requested range.

### BAS-style ranges

When `--fy` / `--period` is used for Financial export, first resolve those selectors to a concrete start/end date range using the BAS calendar mapping rules.

Then apply the same source-date rules defined in this section.

BAS-style selector syntax does not convert Financial export into BAS accounting recognition.

## Exceptions and report status

Financial export follows the shared generated-report severity and exit-status contract in `docs/Command-Line-Spec.md`.

Rules:

```text
INFO/WARNING
    -> workbook remains valid
    -> exit 0

ERROR
    -> if technically possible, write diagnostic workbook
    -> mark Report Status = INCOMPLETE prominently
    -> record error in Exceptions
    -> exit 1

execution failure
    -> workbook need not be written
    -> exit 1
```

For Financial export, an issue is an ERROR when required requested data cannot be represented faithfully or when silently omitting it would make the export misleading.

Missing optional metadata that does not compromise the requested raw data may remain a WARNING.

Never silently omit an errored record and produce a Financial workbook that appears complete.

## Accounting Dataset

Reuse the shared Accounting Dataset minimum contract defined in v0.5.4 wherever calculated accounting facts are needed.

Financial export may also present raw Invoice Ninja fields directly when source fidelity is useful for diagnosis.

Do not force raw diagnostic sheets through the Accounting Dataset if that would lose source information.

When the shared dataset is used, consume its existing:

```text
source identity
party identity
amounts and GST
business-use percentage
relevant source/event dates
recognition events
payment/settlement relationships
exception state/details
```

Do not add report ranges, FY labels, workbook sheet names or output filenames to the Accounting Dataset.

Financial selection/filtering and workbook presentation remain exporter concerns.

## Deferred analysis

The first version does not need dashboards, charts, margin calculations, trend analysis, KPI frameworks, concentration analysis, or management commentary.

## Scope guardrail

Do not turn this slice into a dashboard project, KPI framework, BI system, general ledger, or second accounting database.

## Design rule

> Financial export filters raw records by their own source dates; accounting recognition belongs to BAS and EOFY.
