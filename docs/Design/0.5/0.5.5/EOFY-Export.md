# GoTradie v0.5.5 — EOFY Export

## Purpose

Produce an EOFY business tax-preparation XLSX workbook using the shared accounting calculation layer introduced in v0.5.4.

This is not intended to generate a complete personal income tax return.

It should provide the business figures and supporting evidence required for EOFY preparation.

## Command

```text
GoTradie ninja export eofy --fy 2027
```

## Output

Suggested filename:

```text
FY2027-EOFY.xlsx
```

The financial year comes first so it sorts beside BAS and Financial reports for the same year.

## Shared accounting layer

This slice must reuse the accounting dataset/calculation layer created for BAS.

It must not independently reimplement:

```text
income recognition
expense recognition
GST attribution
cash/non-cash timing
supplier settlement
partial payments
rounding
business-use percentages
```

Where EOFY treatment differs from BAS presentation, that difference should be expressed in the EOFY reporting layer rather than by duplicating core accounting logic.

## Likely workbook areas

The workbook should provide business tax-preparation information such as:

```text
business income
expense categories
GST reconciliation
capital/asset review items
outstanding debtors where relevant
outstanding creditors where relevant
exceptions
supporting detail
```

The exact sheet structure should be decided when this slice begins, based on the accounting data available from Invoice Ninja and the practical EOFY information required.

## Source of truth

The workbook must be reproducible from Invoice Ninja alone.

The historical import spreadsheet must not be required at runtime.

Do not introduce a second accounting database or side ledger.

## Scope guardrail

This slice produces business EOFY preparation output.

It does not attempt to generate:

```text
the user's complete personal tax return
personal deductions unrelated to GoTradie
other personal income
ATO direct lodgement
a general ledger/accounting package
```

## Design rule

> Calculate once, present for EOFY.
