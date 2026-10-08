# GoTradie v0.5.5 - EOFY Export

## Status

**Planned - implementation-ready design**

## Purpose

Produce an EOFY business tax-preparation XLSX workbook using the shared **Accounting Dataset** introduced in v0.5.4.

This is not a complete personal income tax return. It is a business-side preparation workbook that presents Invoice Ninja data in a form an accountant can review and use.

## Command

```text
GoTradie ninja export eofy
GoTradie ninja export eofy --fy 2027
```

`--fy` accepts `YYYY`.

With no `--fy`, export the most recently completed financial year.

EOFY supports financial-year selection only.

Reject:

```text
--period
--from
--to
--all
--month
--quarter
```

A BAS-style period has no useful meaning for EOFY preparation.

## Output

Generated EOFY output follows the global output-directory and overwrite rules in `docs/Design/Command-Line-Spec.md`.

If no explicit output path is supported or supplied:

```text
exports.directory, if configured
otherwise current working directory
```

Filename:

```text
FY2027-EOFY.xlsx
```

The filename is deterministic. Do not append timestamps or automatic collision suffixes.

## Expense categories

Existing Invoice Ninja Expense Categories are authoritative.

Do not introduce a second EOFY category mapping layer or automatically recategorise Expenses.

## Capital and asset review

Use a configurable instant asset write-off threshold, for example:

```yaml
eofy:
  instant_asset_writeoff_threshold: 20000
```

The threshold is data, not a boolean "write-off year" flag.

Use the configured threshold to separate ordinary immediate-write-off candidates from items that require capital/depreciation review.

Preserve enough evidence for accountant review, including where available:

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

GoTradie must not become a depreciation engine.

## Workbook

At minimum include the equivalent of:

```text
Summary
Income
Expenses
Capital-Asset Review
GST Reconciliation
Exceptions
Supporting Detail
```

## Accounting Dataset

Reuse the shared Accounting Dataset from v0.5.4.

Do not create a parallel EOFY accounting engine.

## Source of truth

Invoice Ninja remains the runtime source of truth.

## Scope guardrail

Do not add personal tax-return preparation, depreciation calculation, asset pooling, ATO lodgement, a second accounting database, or an EOFY-specific category mapping system.

## Design rule

> Calculate once, present for EOFY.
