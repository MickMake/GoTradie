# GoTradie v0.5.4 — BAS Export

## Status

**Planned — implementation-ready design**

## Purpose

Produce a BAS-oriented XLSX workbook from Invoice Ninja data.

This slice also introduces the shared **Accounting Dataset** that later EOFY and Financial exports must reuse.

The immediate accounting basis is expected to be cash GST accounting.

## Commands

Support explicit dates:

```text
GoTradie ninja export bas --from 2026-07-01 --to 2026-09-30
```

Support Australian financial-year quarter selection:

```text
GoTradie ninja export bas --fy 2027 --quarter 1
```

`FY2027` means:

```text
1 July 2026 to 30 June 2027
```

Quarter boundaries are:

| Quarter | Period |
| --- | --- |
| Q1 | 1 Jul – 30 Sep |
| Q2 | 1 Oct – 31 Dec |
| Q3 | 1 Jan – 31 Mar |
| Q4 | 1 Apr – 30 Jun |

## Configuration

Expected structured configuration:

```yaml
bas:
  frequency: quarterly
  gst_basis: cash
```

The generated workbook must clearly state the configured GST basis and reporting period.

## Output

Suggested filename:

```text
FY2027-Q1-BAS.xlsx
```

The financial year comes first so related files sort together naturally.

Workbook sheets:

```text
Summary
Sales
Purchases
Exceptions
```

## Summary sheet

Include:

```text
Financial year
Quarter
Period start
Period end
GST basis
Generated timestamp
Source
G1
1A
1B
Net GST position
```

For Simpler BAS, the primary GST fields are:

```text
G1  Total sales
1A  GST on sales
1B  GST on purchases
```

The workbook may calculate:

```text
Net GST position = 1A - 1B
```

## Sales

The Sales sheet must show each contribution to G1 and 1A.

For cash GST accounting:

- actual customer payment timing determines inclusion;
- partial customer payments contribute only the appropriate proportion for the BAS period;
- GST is allocated proportionally and deterministically.

The sheet must contain enough detail to trace every reported amount back to Invoice Ninja records.

## Purchases

The Purchases sheet must show each contribution to 1B.

For ordinary immediately-paid Expenses:

```text
Expense payment date + Expense GST attributes = BAS contribution
```

For supplier-account purchases:

```text
Expense = purchase and GST attributes
Marked supplier Transaction = payment timing
```

The supplier Transaction must not be counted as another purchase.

For cash GST accounting, partial supplier-account payments contribute proportional GST based on the underlying Expense.

## Supplier settlement

The existing accounting invariant remains:

```text
Expenses = what was purchased
Supplier Transactions = when supplier-account purchases were settled
Customer Payments = when customer invoices were paid
```

Supplier settlement reconstruction must remain deterministic and must use Invoice Ninja as the durable source of truth.

## Exceptions

Anything that could make the BAS unsafe or incomplete must be surfaced explicitly.

Examples:

```text
ambiguous supplier settlement
unapplied supplier payment
missing payment date
missing GST treatment
unsupported foreign-currency settlement
archived/deleted marked accounting record
other unresolved accounting state
```

Material exceptions should cause the command to report failure rather than silently produce authoritative-looking figures.

## Accounting Dataset

This slice introduces the shared **Accounting Dataset** used by BAS, EOFY and Financial reporting.

Conceptually:

```text
Invoice Ninja
      |
      v
Accounting Dataset
      |
      +--> BAS workbook
      |
      +--> EOFY workbook (v0.5.5)
      |
      +--> Financial workbook (v0.5.6)
```

The Accounting Dataset owns reusable accounting facts and calculations.

The BAS exporter owns BAS-specific period selection, BAS labels, workbook layout and presentation.

Rules that should exist once in the Accounting Dataset include:

```text
income recognition
expense recognition
GST attribution
cash versus non-cash timing
partial customer payments
supplier-account allocation
partial supplier payments
rounding
business-use percentages
GST treatment
```

The dataset must remain intentionally narrow.

Do not pre-build a general accounting framework for hypothetical future reports. Add only the shared accounting behaviour required by BAS and already-known EOFY/Financial needs.

## Source of truth

After migration, the BAS must be reproducible from Invoice Ninja alone.

Do not introduce:

- a second accounting database;
- SQLite allocation state;
- a persistent GoTradie side ledger;
- dependence on the historical import spreadsheet.

## Immediate scope

Implement:

1. explicit date-range selection;
2. `--fy` plus `--quarter`;
3. cash GST basis;
4. G1;
5. 1A;
6. 1B;
7. partial customer payment treatment;
8. ordinary Expense treatment;
9. supplier-account settlement treatment;
10. partial supplier payment treatment;
11. XLSX output;
12. Summary, Sales, Purchases and Exceptions sheets;
13. traceable audit detail;
14. explicit exceptions;
15. the minimum shared Accounting Dataset needed to support the above and known later reporting slices.

Do not initially implement:

```text
direct ATO lodgement
PAYG withholding
PAYG instalments
FBT
payroll
general ledger
complete tax-return generation
another accounting database
speculative accounting abstractions
```

## Design rule

> If a BAS number cannot be traced back to supporting Invoice Ninja records, the report is not finished.
