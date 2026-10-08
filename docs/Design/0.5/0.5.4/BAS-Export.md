# GoTradie v0.5.4 — BAS Export

## Status

**Planned — implementation-ready design**

## Purpose

Produce a BAS-oriented XLSX workbook from Invoice Ninja data.

This slice also introduces the shared **Accounting Dataset** that later EOFY and Financial exports must reuse.

GST accounting basis is configured explicitly as either cash or accrual.

## Configuration

BAS configuration is mandatory in `~/.GoTradie/config.yaml`.

```yaml
bas:
  reporting_period: quarterly
  gst_basis: cash
```

Supported `reporting_period` values:

```text
monthly
quarterly
yearly
```

Supported `gst_basis` values:

```text
cash
accrual
```

Missing or unsupported values are configuration errors. GoTradie must not guess or silently fall back.

## Commands

For monthly and quarterly reporting:

```text
GoTradie ninja export bas
GoTradie ninja export bas --fy 2027
GoTradie ninja export bas --period 1
GoTradie ninja export bas --period Jul
GoTradie ninja export bas --fy 2025 --period 1
GoTradie ninja export bas --fy 2025 --period Jul
```

BAS does not support:

```text
--from
--to
--all
--month
--quarter
```

Arbitrary date-range export belongs to the Financial/dump export.

## Financial year selection

If `--fy` is omitted, use the current Australian financial year.

Financial years are named for the calendar year in which they end.

```text
FY2027 = 1 July 2026 to 30 June 2027
```

For monthly and quarterly reporting:

```text
no --fy, no --period
    -> current FY + current BAS period

--fy CURRENT
    -> current FY + current BAS period

--fy NON-CURRENT
    -> fail; --period is required

--period VALUE
    -> current FY + selected BAS period

--fy YEAR --period VALUE
    -> selected FY + selected BAS period
```

An explicit `--fy` must never cause GoTradie to infer a historical or future BAS period from today's date.

When `--fy` is supplied without `--period` for monthly or quarterly reporting, the selected FY must be the current FY. Otherwise the command fails.

When both `--fy` and `--period` are supplied, the selected period is resolved within the explicitly selected FY regardless of whether that FY is current, historical, or future.

## Reporting-period selection

For monthly and quarterly reporting, `--period` accepts either:

```text
an integer period number
a standard three-letter English month abbreviation
```

Accepted month abbreviations:

```text
Jan
Feb
Mar
Apr
May
Jun
Jul
Aug
Sep
Oct
Nov
Dec
```

Month abbreviations are case-insensitive.

Do not accept full month names, `Sept`, partial names, or fuzzy matching.

A month value selects the BAS period containing that month according to configured `bas.reporting_period`.

### Monthly reporting

| Period | Month | Selector |
| ---: | --- | --- |
| 1 | July | `Jul` |
| 2 | August | `Aug` |
| 3 | September | `Sep` |
| 4 | October | `Oct` |
| 5 | November | `Nov` |
| 6 | December | `Dec` |
| 7 | January | `Jan` |
| 8 | February | `Feb` |
| 9 | March | `Mar` |
| 10 | April | `Apr` |
| 11 | May | `May` |
| 12 | June | `Jun` |

Examples:

```text
GoTradie ninja export bas --period Jul
    -> current FY, period 1

GoTradie ninja export bas --fy 2025 --period Jan
    -> FY2025, period 7
    -> January 2025
```

Integer values outside `1-12` are invalid.

### Quarterly reporting

| Period | Quarter | Months | Dates |
| ---: | --- | --- | --- |
| 1 | Q1 | Jul, Aug, Sep | 1 Jul – 30 Sep |
| 2 | Q2 | Oct, Nov, Dec | 1 Oct – 31 Dec |
| 3 | Q3 | Jan, Feb, Mar | 1 Jan – 31 Mar |
| 4 | Q4 | Apr, May, Jun | 1 Apr – 30 Jun |

These are equivalent under quarterly reporting:

```text
--period 1
--period Jul
--period Aug
--period Sep
```

Example:

```text
GoTradie ninja export bas --fy 2025 --period Jan
    -> FY2025, period 3
    -> 1 January 2025 to 31 March 2025
```

Integer values outside `1-4` are invalid.

### Yearly reporting

When `bas.reporting_period` is `yearly`, there is no BAS sub-period.

```text
GoTradie ninja export bas
    -> current FY

GoTradie ninja export bas --fy 2025
    -> FY2025
```

An explicit FY is valid even when it is not current because there is no sub-period to infer.

`--period` is always invalid for yearly reporting, whether supplied as an integer or month.

## Command-resolution matrix

For monthly and quarterly reporting:

```text
GoTradie ninja export bas
    -> current FY + current period

GoTradie ninja export bas --fy CURRENT
    -> current FY + current period

GoTradie ninja export bas --fy NON-CURRENT
    -> fail: --period required

GoTradie ninja export bas --period N
    -> current FY + period N

GoTradie ninja export bas --period Mon
    -> current FY + period containing Mon

GoTradie ninja export bas --fy YEAR --period N
    -> YEAR + period N

GoTradie ninja export bas --fy YEAR --period Mon
    -> YEAR + period containing Mon
```

The software derives a BAS period from today's date only when the selected FY is the current FY.

## Output

The workbook must identify:

```text
Financial year
Reporting period type
Reporting period number where applicable
Period start
Period end
GST basis
Generated timestamp
Source
```

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
Reporting period type
Reporting period
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

For Simpler BAS:

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

For cash GST accounting:

- actual customer payment timing determines inclusion;
- partial customer payments contribute only the appropriate proportion for the BAS period;
- GST is allocated proportionally and deterministically.

For accrual GST accounting:

- invoice/sale recognition timing determines inclusion;
- later customer payments must not create a second GST event.

The sheet must contain enough detail to trace every reported amount back to Invoice Ninja records.

## Purchases

For cash GST accounting, ordinary immediately-paid Expenses use:

```text
Expense payment date + Expense GST attributes = BAS contribution
```

For supplier-account purchases:

```text
Expense = purchase and GST attributes
Marked supplier Transaction = payment timing
```

The supplier Transaction must not be counted as another purchase.

Partial supplier-account payments contribute proportional GST based on the underlying Expense.

For accrual GST accounting, the purchase/Expense recognition date and GST attributes determine the GST event. Later supplier-account settlement Transactions do not create another GST event.

## Supplier settlement

```text
Expenses = what was purchased
Supplier Transactions = when supplier-account purchases were settled
Customer Payments = when customer invoices were paid
```

Supplier settlement reconstruction must remain deterministic and use Invoice Ninja as the durable source of truth.

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

```text
Invoice Ninja
      |
      v
Accounting Dataset
      |
      +--> BAS workbook
      +--> EOFY workbook (v0.5.5)
      +--> Financial workbook (v0.5.6)
```

The Accounting Dataset owns reusable accounting facts and calculations.

The BAS exporter owns BAS-specific period selection, BAS labels, workbook layout and presentation.

Shared rules include:

```text
income recognition
expense recognition
GST attribution
cash versus accrual timing
partial customer payments
supplier-account allocation
partial supplier payments
rounding
business-use percentages
GST treatment
```

Keep the dataset intentionally narrow.

## Source of truth

After migration, the BAS must be reproducible from Invoice Ninja alone.

Do not introduce another accounting database, SQLite allocation state, a persistent side ledger, or dependence on the historical import spreadsheet.

## Immediate scope

Implement:

1. monthly, quarterly or yearly BAS reporting-period selection;
2. `--fy` financial-year selection;
3. `--period` integer or standard three-letter month selection for monthly/quarterly reporting;
4. current FY when `--fy` is omitted;
5. current BAS period when appropriate;
6. failure when monthly/quarterly `--fy` selects a non-current FY without `--period`;
7. month-to-period resolution according to configured reporting period;
8. case-insensitive standard three-letter month parsing only;
9. rejection of `--period` for yearly reporting;
10. rejection of BAS `--from/--to`, `--month`, `--quarter`, and `--all`;
11. cash GST basis;
12. accrual GST basis;
13. G1, 1A and 1B;
14. partial customer payment treatment;
15. ordinary Expense treatment;
16. supplier-account settlement treatment;
17. partial supplier payment treatment;
18. XLSX output;
19. Summary, Sales, Purchases and Exceptions sheets;
20. traceable audit detail;
21. explicit exceptions;
22. the minimum shared Accounting Dataset required by BAS and known later reporting slices.

## Design rule

> If a BAS number cannot be traced back to supporting Invoice Ninja records, the report is not finished.
