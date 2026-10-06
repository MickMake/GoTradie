# GoTradie v0.5.6 — Financial Analysis Export

## Purpose

Produce an XLSX workbook for business management and financial analysis.

This report is for understanding the business rather than lodging tax.

It must reuse the shared accounting calculation layer introduced in v0.5.4.

## Command

```text
GoTradie ninja export financial --fy 2027
```

## Output

Suggested filename:

```text
FY2027-Financial.xlsx
```

The financial year comes first so it sorts beside BAS and EOFY outputs.

## Shared accounting layer

This slice must consume the same underlying accounting dataset used by BAS and EOFY.

Do not recalculate the same accounting events independently.

Core rules that remain shared include:

```text
income recognition
expense recognition
GST treatment
payment timing
supplier settlement
business-use percentages
rounding
```

## Likely analysis areas

Useful workbook content may include:

```text
profit and loss
monthly revenue
monthly expenses
expense category analysis
vendor analysis
customer analysis
job/project analysis
materials versus labour
margin analysis where data permits
trends
```

The exact workbook structure should be decided when this slice begins, based on what can be calculated reliably from Invoice Ninja.

## Scope guardrail

The purpose is useful business analysis, not an attempt to recreate a complete accounting suite.

Do not introduce:

```text
another persistent accounting database
general-ledger machinery merely for reporting
duplicated BAS/EOFY accounting logic
```

## Design rule

> Calculate once, present for running the business.
