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

## EOFY accounting basis

EOFY income and expense recognition is controlled by:

```yaml
eofy:
  accounting_basis: cash
```

Supported values:

```text
cash
accrual
```

The selected value determines the EOFY recognition rules applied by the Accounting Dataset.

EOFY recognition must be deterministic from `eofy.accounting_basis`.

`eofy.accounting_basis` is independent of:

```text
bas.gst_basis
```

Do not infer EOFY recognition from the BAS GST basis.

Do not reuse `bas.gst_basis` as an EOFY setting.

The configured accounting basis must be shown in the generated workbook so the recognition basis used to produce the figures is explicit.

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

Use the configured instant asset write-off threshold:

```yaml
eofy:
  accounting_basis: cash
  instant_asset_writeoff_threshold: 20000
```

The threshold is tax data for the selected financial year, not a boolean "write-off year" flag and not a timeless hard-coded constant.

GoTradie must follow the ATO threshold mechanics rather than invent its own asset-cost test.

### Threshold-test cost

For an asset under review:

```text
threshold-test cost
    = relevant asset cost
    - GST input tax credits the business is entitled to claim
```

If no GST credit is claimable, do not remove GST from the threshold-test cost.

If only part of the GST credit is claimable, remove only that claimable amount.

Compare the **entire resulting asset cost** against the configured threshold.

Do not reduce the threshold-test cost by private or non-business use.

### Business-use percentage

Business-use percentage is applied after the threshold test.

Conceptually:

```text
eligible under threshold
    -> threshold-test cost is below the applicable threshold

deductible/review amount
    -> taxable/business-use portion of the eligible cost
```

An asset does not become eligible merely because its business-use-adjusted amount falls below the threshold.

### Relevant financial year

Instant asset write-off eligibility is tied to the financial year in which the asset is first used or installed ready for use.

The configured threshold must therefore be the threshold applicable to the selected EOFY financial year.

Do not assume the threshold is identical across all years.

### Workbook evidence

For each capital/asset-review item, preserve enough information to show, where available:

```text
first-used / installed-ready-for-use date
supplier
description
source asset/expense cost
GST amount
claimable GST credit used in the threshold calculation
threshold-test cost
configured threshold
business-use percentage
business-use-adjusted amount
Invoice Ninja Expense Category
source record identity
classification/result
```

The workbook must state the configured instant asset write-off threshold used for the report.

GoTradie must not become a depreciation engine. Items that are not straightforward instant-write-off candidates remain capital/depreciation review items.

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

The workbook must identify:

```text
financial year
period start/end
EOFY accounting basis
instant asset write-off threshold used
generated timestamp
source = Invoice Ninja
```

## Accounting Dataset

Reuse the shared Accounting Dataset contract defined in v0.5.4.

Do not create a parallel EOFY accounting engine.

EOFY consumes the dataset's:

```text
source identity
party identity
amounts and GST
business-use percentage
relevant source/event dates
EOFY recognition event/date
payment/settlement relationships where relevant
exception state/details
```

The Accounting Dataset must apply EOFY income/expense recognition according to `eofy.accounting_basis`.

BAS GST timing remains controlled separately by `bas.gst_basis`.

EOFY owns financial-year selection and workbook presentation. Those report-selection concepts do not belong in the shared dataset.

## Source of truth

Invoice Ninja remains the runtime source of truth.

## Scope guardrail

Do not add personal tax-return preparation, depreciation calculation, asset pooling, ATO lodgement, a second accounting database, or an EOFY-specific category mapping system.

## Design rule

> Follow the ATO asset-cost test; do not invent a GoTradie version of tax law.
