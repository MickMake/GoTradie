# GoTradie v0.5.6 - Financial Data Export

## Status

**Planned - implementation-ready design**

## Purpose

Produce a general-purpose XLSX export of financial data from Invoice Ninja.

The first implementation is deliberately a **raw data and diagnostic export**, not a dashboard or management-analysis engine.

It should be useful for two equally valid jobs:

1. export data for later spreadsheet analysis;
2. inspect Invoice Ninja when something looks wrong and a faithful data dump is needed.

## Command

Default:

```text
GoTradie ninja export financial
```

With no period flags, export **all available financial data**.

Optional restrictions:

```text
GoTradie ninja export financial --fy 2027
GoTradie ninja export financial --from 2026-01-01 --to 2026-12-31
```

Selection modes are mutually exclusive:

```text
no period flags      = all available data
--fy                  = one financial year
--from + --to         = explicit range
```

There is no `--all` flag because all data is already the default.

## Output

For a restricted financial year:

```text
FY2027-Financial.xlsx
```

For an unrestricted dump, use a deterministic descriptive filename that clearly indicates an all-data financial export. The implementation should keep this simple and document the chosen name.

## Workbook approach

Prefer faithful, well-structured data sheets over dashboards.

Include the financial entities available from Invoice Ninja that materially help analysis and diagnosis. Expected sheets include the equivalent of:

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

If the current Invoice Ninja model exposes a more accurate entity name or a useful supporting entity, use that model rather than inventing a parallel abstraction.

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
archived/deleted markers where available and relevant
```

The export should be useful for answering questions such as:

> There is something fishy with Invoice Ninja. What records actually exist and how do they relate?

Do not transform away useful source evidence merely to make the workbook prettier.

## Filtering

When a date restriction is supplied, apply the most natural accounting/event date for each entity and document any entity-specific treatment in code/tests.

Do not silently omit undated or unclassifiable records. Put materially ambiguous cases in `Exceptions` or otherwise make them visible.

## Accounting Dataset

Reuse the Accounting Dataset where it provides useful calculated financial values, but keep raw Invoice Ninja data visible where needed for diagnosis.

Do not force every sheet through the Accounting Dataset if doing so would lose source fidelity.

## Deferred analysis

The first version does not need:

```text
dashboards
charts
margin calculations
trend analysis
KPI framework
customer concentration analysis
supplier concentration analysis
management commentary
```

Those can be added later if actual use proves them valuable.

## XLSX usability

Use basic spreadsheet usability only:

- clear sheet names;
- stable columns;
- useful headers;
- date/number formatting;
- filters/frozen header rows where already supported and cheap to provide;
- deterministic ordering where practical.

Do not spend the slice building presentation polish.

## Scope guardrail

Do not turn this slice into:

```text
a dashboard project
a KPI framework
a business-intelligence system
a general ledger
a second accounting database
```

## Design rule

> Export the data cleanly first. Analyse it later when there is a real reason.
