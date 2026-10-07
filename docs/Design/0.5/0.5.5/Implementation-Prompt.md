# GoTradie v0.5.5 - Implementation Prompt

Implement the accepted EOFY export design.

Primary contract:

```text
docs/Design/0.5/0.5.5/EOFY-Export.md
```

Relevant contracts:

```text
docs/Design/0.5/0.5.4/BAS-Export.md
docs/Design/0.5/0.5.1/Expense-Importing.md
docs/Design/0.5/README.md
```

## Mandatory preflight

Before making any code changes:

1. Fetch latest `origin/main`.
2. Verify every earlier slice branch/PR through v0.5.4 has been merged or otherwise explicitly handled.
3. If any earlier slice branch/PR is not merged, STOP and report it.
4. Inspect the current Accounting Dataset, Invoice Ninja income/expense/payment/transaction access, CLI export structure, configuration, XLSX support, and tests.
5. State:
   - intended implementation;
   - proposed branch;
   - likely files/packages to change;
   - any genuine blocking ambiguity or missing source data.
6. STOP and wait for approval before creating the branch or changing code.

Suggested branch:

```text
v0.5.5-eofy-export
```

After approval, branch from latest `origin/main` only.

## Required behaviour

Implement:

```text
GoTradie ninja export eofy
GoTradie ninja export eofy --fy 2027
```

Rules:

- no period flags = most recently completed financial year;
- `--fy YYYY` = requested financial year;
- `--all` is invalid;
- output is XLSX;
- filename is `FY<year>-EOFY.xlsx`.

## Accounting rules

- Reuse the Accounting Dataset from v0.5.4.
- Existing Invoice Ninja Expense Categories are authoritative.
- Do not introduce a second tax-category mapping layer.
- Do not automatically recategorise Expenses.
- Preserve source IDs/traceability.
- Do not duplicate BAS/GST calculation logic where the Accounting Dataset already owns it.

## Capital and asset review

Read the configured instant asset write-off threshold from hierarchical config.

Use it to identify purchases that require capital/asset review.

Preserve, where available:

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

Do not calculate depreciation, pooling, decline in value, disposal adjustments, or final tax-law eligibility.

The exporter presents review evidence; the accountant decides treatment.

## Workbook

Produce a practical accountant-facing workbook containing the information described in `EOFY-Export.md`, including:

```text
Summary
Income
Expenses
Capital-Asset Review
GST Reconciliation
Exceptions
Supporting Detail
```

Exact internal helpers and modest sheet-layout details are implementation decisions. Do not reopen design unless required source data is genuinely unavailable or contradictory.

## Tests

Add focused tests covering at least:

- default resolution to the most recently completed financial year;
- `--fy` override;
- invalid `--all`;
- Expense Category preservation;
- threshold boundary behaviour;
- below-threshold and at/above-threshold review classification;
- no depreciation calculation;
- GST reconciliation reuse from the Accounting Dataset;
- exception propagation;
- deterministic workbook contents/order where practical;
- required XLSX sheets and filename.

Use deterministic fixtures and integer minor units where appropriate.

## Scope exclusions

Do not implement:

```text
personal tax-return preparation
ATO lodgement
depreciation schedules
asset pools
new category mapping
Financial export
Product Sync
a persistent accounting side database
```

## Verification

Before completion run:

```text
gofmt
go vet
go test
go build
```

Perform evidence-backed review/fix loops, maximum 3.

Update relevant documentation/changelog if required by repository convention.

Finish with a concise completion report covering implemented behaviour, tests, verification results, and any deliberately deferred items.
