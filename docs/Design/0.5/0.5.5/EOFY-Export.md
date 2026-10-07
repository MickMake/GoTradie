# GoTradie v0.5.5 - EOFY Export

## Status

**Planned - implementation-ready design**

## Purpose

Produce an EOFY business tax-preparation XLSX workbook using the shared **Accounting Dataset** introduced in v0.5.4.

This is not a complete personal income tax return. It is a business-side preparation workbook that presents Invoice Ninja data in a form an accountant can review and use.

## Command

```text
GoTradie ninja export eofy
```

With no period flags, export the **most recently completed financial year**.

Override:

```text
GoTradie ninja export eofy --fy 2027
```

`--all` is invalid for EOFY export.

## Output

Filename:

```text
FY2027-EOFY.xlsx
```

## Expense categories

Existing Invoice Ninja Expense Categories are authoritative.

Those categories originate from the accepted spreadsheet/import workflow and have already been approved for the user's accounting needs.

Do not:

- introduce a second EOFY category mapping layer;
- automatically recategorise Expenses;
- replace Invoice Ninja categories with inferred tax categories.

## Capital and asset review

Use a configurable instant asset write-off threshold, for example:

```yaml
eofy:
  instant_asset_writeoff_threshold: 20000
```

The threshold is data, not a boolean "write-off year" flag.

Use the configured threshold to separate ordinary immediate-write-off candidates from items that require capital/depreciation review.

The workbook must preserve enough evidence for the accountant to determine final treatment, including where available:

```text
purchase date
supplier
description
amount
GST
business-use percentage
Invoice Ninja Expense Category
source record identity
```

Items below the configured threshold may be presented as potential immediate-write-off candidates, subject to accountant review.

Items at or above the configured threshold must be clearly surfaced for capital/depreciation review.

GoTradie must not become a depreciation engine. It must not calculate:

```text
depreciation schedules
pool balances
decline in value
disposal adjustments
asset-register tax treatment
```

The threshold is a presentation/classification aid only; final tax treatment remains an accountant decision.

## Workbook

Produce a practical accountant-facing workbook. At minimum it should include the equivalent of:

```text
Summary
Income
Expenses
Capital-Asset Review
GST Reconciliation
Exceptions
Supporting Detail
```

The implementation may adjust exact sheet names or split/merge supporting sheets where this improves usability, provided the accepted information and traceability are preserved.

### Summary

Include at least:

```text
financial year
period start/end
gross business income
total expenses
net business result
GST totals/reconciliation summary
capital-asset review count/value
generated timestamp
source = Invoice Ninja
```

### Income

Present income detail supporting the EOFY totals with source identifiers and dates sufficient to trace each amount back to Invoice Ninja.

### Expenses

Present Expenses grouped or sortable by the authoritative Invoice Ninja Expense Category. Preserve GST and source-record information.

### Capital-Asset Review

Present threshold-flagged purchases separately without changing their underlying Invoice Ninja Expense Category.

### GST Reconciliation

Reuse the Accounting Dataset rather than reimplementing GST calculations. Present enough information to compare annual values with BAS-period reporting.

### Exceptions

Surface unresolved or materially incomplete accounting records rather than silently guessing.

Examples include missing category/tax treatment, unresolved supplier settlement state, unsupported currency treatment, or accounting records whose state prevents reliable reporting.

### Supporting Detail

Preserve source IDs and evidence needed to trace workbook figures back to Invoice Ninja.

## Accounting Dataset

Reuse the shared Accounting Dataset from v0.5.4.

Do not create a parallel EOFY accounting engine.

## Source of truth

Invoice Ninja remains the runtime source of truth.

The workbook must be reproducible from Invoice Ninja data without relying on the historical spreadsheet at runtime.

## Scope guardrail

Do not add:

```text
personal tax-return preparation
depreciation engine
asset pooling engine
ATO lodgement
a second accounting database
an EOFY-specific category mapping system
```

## Design rule

> Calculate once, present for EOFY.
