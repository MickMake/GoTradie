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

  periods:
    Q1:
      bas_begin: "07-01"
      bas_end: "09-30"
      submit_begin: "10-01"
      submit_end: "10-28"

    Q2:
      bas_begin: "10-01"
      bas_end: "12-31"
      submit_begin: "01-01"
      submit_end: "02-28"

    Q3:
      bas_begin: "01-01"
      bas_end: "03-31"
      submit_begin: "04-01"
      submit_end: "04-28"

    Q4:
      bas_begin: "04-01"
      bas_end: "06-30"
      submit_begin: "07-01"
      submit_end: "07-28"
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

Missing or unsupported accounting-significant values are configuration errors. GoTradie must not guess or silently fall back.

## Core selection rule

BAS selection is deliberately date-driven.

GoTradie does **not** track whether a BAS has actually been lodged.

There is no BAS lodgement state in Invoice Ninja and no local lodgement-state file.

Selection uses only:

```text
current date
configured reporting period
ATO BAS period and due-date rules
explicit --fy
explicit --period
```

## Financial year

`--fy` accepts exactly:

```text
YYYY
```

If `--fy` is omitted, derive the Australian financial year from the current date.

Examples:

```text
30/06/2020 -> FY2020
01/07/2020 -> FY2021
```

Financial years are named for the calendar year in which they end.

```text
FY2027 = 1 July 2026 to 30 June 2027
```

## Default BAS period

If `--period` is omitted and no historical FY is explicitly selected, use the current date together with the configured reporting period and ATO due-date rules to select the natural BAS period.

A completed BAS period remains the natural period through its ATO lodgement due date.

If that due date has passed and the next BAS period has not yet completed, continue to select the most recently completed BAS period.

This is purely date-driven. GoTradie does not attempt to infer whether the BAS has already been lodged.

Example for quarterly reporting:

```text
8 October 2026
    -> FY2027 period 1
```

because Jul-Sep has completed and its ATO lodgement due date has not yet passed.

## Explicit historical financial year

If `--fy` selects a non-current financial year and `--period` is omitted:

```text
GoTradie ninja export bas --fy 2025
```

produce **all BAS periods within FY2025**.

For monthly reporting this means all 12 periods.

For quarterly reporting this means all 4 periods.

For yearly reporting this means the single FY2025 reporting year.

If `--fy` selects the current financial year and `--period` is omitted, use the natural date-driven BAS period.

If both `--fy` and `--period` are supplied, produce only the selected BAS period.

## `--period` formats

`--period` is case-insensitive.

For monthly BAS, accept:

```text
1 .. 12
Jan .. Dec
January .. December
```

For quarterly BAS, accept:

```text
1 .. 4
Jan .. Dec
January .. December
```

For quarterly BAS, a month name selects the quarter containing that month:

```text
Jul / July / Aug / August / Sep / September -> period 1
Oct / October / Nov / November / Dec / December -> period 2
Jan / January / Feb / February / Mar / March -> period 3
Apr / April / May / Jun / June -> period 4
```

For yearly BAS, `--period` is invalid.

Do not add fuzzy month parsing or non-standard aliases.

## Command-resolution matrix

```text
GoTradie ninja export bas
    -> current FY + natural date-driven BAS period

GoTradie ninja export bas --fy CURRENT
    -> current FY + natural date-driven BAS period

GoTradie ninja export bas --fy HISTORIC
    -> all BAS periods in that FY

GoTradie ninja export bas --period VALUE
    -> current FY + selected BAS period

GoTradie ninja export bas --fy YEAR --period VALUE
    -> selected FY + selected BAS period
```

BAS does not support:

```text
--from
--to
--all
--month
--quarter
```

Arbitrary date-range export belongs to the Financial export.

## ATO due dates

The no-flag/default BAS workflow depends on ATO BAS lodgement due-date rules.

GoTradie should periodically verify those rules according to:

```yaml
bas:
  ato_due_dates:
    verify_every_days: 30
```

A small local operational cache is permitted, for example:

```text
~/.GoTradie/cache/ato_due_dates.json
```

It may contain only:

```text
last successful verification date
cached ATO BAS due-date rules
source/rule identifier where available
```

It must not contain BAS lodgement state or accounting data.

If the verification interval has expired, GoTradie should attempt to refresh the ATO due-date rules.

If refresh fails or has not succeeded for longer than the configured interval:

```text
continue using the existing cached/configured dates
print a warning to stdout
do not fail BAS generation solely because the due-date refresh is stale
```

## Output

Generated BAS output follows the global output-directory and overwrite rules in `docs/Command-Line-Spec.md`.

If no explicit output path is supported or supplied:

```text
exports.directory, if configured
otherwise current working directory
```

Filenames are deterministic.

Quarterly single-period BAS:

```text
FY2027-BAS-Q1.xlsx
FY2027-BAS-Q2.xlsx
FY2027-BAS-Q3.xlsx
FY2027-BAS-Q4.xlsx
```

Monthly single-period BAS:

```text
FY2027-BAS-Jul.xlsx
FY2027-BAS-Aug.xlsx
...
FY2027-BAS-Jun.xlsx
```

Historical `--fy` with no `--period`, containing all periods:

```text
FY2025-BAS.xlsx
```

Yearly BAS:

```text
FY2027-BAS.xlsx
```

Do not use generic period labels such as `P1` in filenames.

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
Report Status
```

When a historical `--fy` requests all BAS periods, the output must clearly separate or identify each period.

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
Report Status
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

## Exceptions and report status

BAS follows the shared generated-report severity and exit-status contract in `docs/Command-Line-Spec.md`.

Examples of BAS accounting errors include:

```text
ambiguous supplier settlement
unapplied supplier payment required for calculation
missing payment date required for calculation
missing GST treatment
unsupported foreign-currency settlement required for calculation
archived/deleted marked accounting record that affects the result
other unresolved accounting state that can materially alter BAS figures
```

A stale ATO due-date verification is an operational WARNING, not an accounting ERROR.

Rules:

```text
INFO/WARNING
    -> workbook remains valid
    -> exit 0

ERROR
    -> if technically possible, write workbook
    -> mark Report Status = INCOMPLETE prominently on Summary
    -> record error in Exceptions
    -> exit 1

execution failure
    -> workbook need not be written
    -> exit 1
```

Never silently omit an accounting error and produce a BAS workbook that appears complete.

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

The Accounting Dataset is an in-memory normalised view of accounting facts required by the reporting slices.

It is **not**:

```text
a database
a persistent ledger
a general ledger
a replacement for Invoice Ninja
a report-period model
a speculative accounting framework
```

### Minimum contract

The dataset must preserve enough information to represent these minimum fact groups.

#### Source identity

```text
source entity/record type
Invoice Ninja source record ID
```

Every derived accounting fact must remain traceable to its source record.

#### Party identity

Where relevant:

```text
customer identity
vendor/supplier identity
```

Do not invent a separate party master.

#### Amounts and GST

Where relevant:

```text
gross amount
net amount
GST amount
```

Preserve the source amounts needed to audit or reconstruct derived values.

#### Business use

Where relevant:

```text
business-use percentage
```

Do not silently assume 100% business use when the source explicitly provides another value.

#### Relevant dates

Preserve source/event dates needed by accounting rules, including where applicable:

```text
source transaction date
invoice/expense date
customer payment date
supplier settlement/payment date
GST recognition date/event
EOFY recognition date/event
```

Recognition events must be derived deterministically from the relevant configured accounting basis.

#### Payment and settlement relationships

Preserve allocation relationships needed for cash-basis and partial-payment calculations:

```text
invoice -> customer payment allocation
expense -> supplier settlement/payment allocation
```

Do not flatten these relationships into a single total if doing so would lose timing or allocation information.

#### Exception state

Accounting facts must be able to carry or reference unresolved state using the shared report severity contract:

```text
normal
INFO
WARNING
ERROR
```

The dataset must preserve enough detail for BAS, EOFY and Financial exporters to surface the underlying issue.

### What does not belong in the dataset

Do not store report-selection or presentation concepts in the Accounting Dataset:

```text
financial-year labels
BAS period numbers
quarter labels
workbook sheet names
output filenames
selected report range
```

The dataset provides dated accounting facts and recognition events.

BAS, EOFY and Financial exporters decide whether those facts belong in a requested reporting range and how to present them.

### Guardrail

> The Accounting Dataset contains normalised accounting facts, recognition events and source relationships only. It does not contain report periods, workbook structure, persistent accounting state, or speculative accounting abstractions.

The BAS exporter owns BAS-specific selection, ATO due-date handling, labels, workbook layout and presentation.

Keep the dataset intentionally narrow.

## Source of truth

After migration, the BAS must be reproducible from Invoice Ninja alone.

Do not introduce another accounting database, SQLite allocation state, a persistent side ledger, or dependence on the historical import spreadsheet.

The permitted ATO due-date cache is operational metadata only and is not accounting state.

## Design rule

> If a BAS number cannot be traced back to supporting Invoice Ninja records, the report is not finished.
