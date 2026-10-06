# GoTradie v0.5.5 — EOFY Export

## Status

**Planned — roadmap-level design; must be completed before implementation**

## Purpose

Produce an EOFY business tax-preparation XLSX workbook using the shared **Accounting Dataset** introduced in v0.5.4.

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

## Accounting Dataset

This slice must reuse the Accounting Dataset created in v0.5.4.

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

The workbook is expected to provide business tax-preparation information such as:

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

These areas are directional only.

Before implementation begins, this design must be completed with:

- accepted sheet names;
- required fields and totals per sheet;
- treatment of GST-inclusive versus GST-exclusive figures;
- treatment of capital/asset items;
- treatment of debtors/creditors if included;
- exception rules;
- traceability requirements;
- acceptance examples.

Until those decisions are made, this document is a roadmap contract rather than an implementation specification.

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
