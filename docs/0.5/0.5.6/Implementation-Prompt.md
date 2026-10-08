# GoTradie v0.5.6 — Implementation Prompt

Implement the accepted Financial Data Export design.

Primary contract:

```text
docs/0.5/0.5.6/Financial-Export.md
```

## Mandatory preflight

Before changing code:

1. Fetch latest `origin/main`.
2. Verify all earlier slice branches/PRs through v0.5.5 are merged.
3. If not, STOP and report it.
4. Inspect Invoice Ninja entity access, Accounting Dataset, CLI export handling and XLSX support.
5. State intended implementation, branch, likely files/packages and source-data gaps.
6. STOP and wait for approval.

Suggested branch:

```text
v0.5.6-financial-export
```

## CLI

Implement unrestricted export:

```text
GoTradie ninja export financial
```

Support BAS-style selection:

```text
GoTradie ninja export financial --fy 2025
GoTradie ninja export financial --period 2
GoTradie ninja export financial --period Jul
GoTradie ninja export financial --period July
GoTradie ninja export financial --fy 2025 --period 2
GoTradie ninja export financial --fy 2025 --period Jul
```

Support explicit date ranges:

```text
GoTradie ninja export financial --from 2025-01-01
GoTradie ninja export financial --to 2025-06-30
GoTradie ninja export financial --from 2025-01-01 --to 2025-06-30
```

BAS-style selectors and `--from`/`--to` are mutually exclusive.

Use the same case-insensitive period parsing and FY mapping rules as BAS.

No selection flags means all available financial data.

There is no `--all` flag.

Financial export does not modify Invoice Ninja and does not use `--commit`.

## Output

Follow the global output-resolution and `--force` rules in `docs/Command-Line-Spec.md`.

Use these deterministic filenames:

```text
financial year -> FYyyyy-Financial.xlsx
bounded range  -> Financial-YYYY-MM-DD-to-YYYY-MM-DD.xlsx
from only      -> Financial-from-YYYY-MM-DD.xlsx
to only        -> Financial-to-YYYY-MM-DD.xlsx
unrestricted   -> Financial-All.xlsx
```

Use optional `exports.directory` when configured; otherwise use the current working directory.

Do not append timestamps or automatic collision suffixes.

## Date filtering

Financial export is a raw-data/diagnostic export.

Filter transactional entities by their own source dates:

```text
invoice / income record
    -> invoice date

customer payment
    -> payment date

expense
    -> expense date

supplier transaction / supplier payment
    -> transaction/payment date

quote, if included
    -> quote date
```

Do not date-filter these reference/master entities merely because a date range is selected:

```text
customers
vendors
products
projects/jobs
```

They may be included as needed to preserve relationships or source context.

Do not use:

```text
BAS GST recognition date
EOFY recognition date
```

as the Financial-export inclusion date for raw transactional rows.

For BAS-style Financial selectors, first resolve `--fy` / `--period` to a concrete start/end date range, then apply the same source-date rules above.

If a transactional record has no usable required source date, do not guess another date. Surface it in Exceptions with source identity/details.

Related records may legitimately fall on opposite sides of a range boundary.

## Accounting Dataset and raw data

Reuse the v0.5.4 Accounting Dataset minimum contract for shared calculated accounting facts.

Preserve raw Invoice Ninja records alongside it where useful for diagnosis.

Do not duplicate shared accounting calculations inside Financial export.

Do not add report-selection or workbook-presentation fields to the Accounting Dataset.

## Workbook

Keep v1 raw-data/diagnostic focused.

Do not add dashboards, charts, KPI frameworks, margin engines or BI layers.

Preserve source IDs and useful relationships.

## Tests

Cover at least:

- invoice filtering uses invoice date;
- customer-payment filtering uses payment date;
- expense filtering uses expense date;
- supplier-transaction filtering uses transaction/payment date;
- quote filtering, if implemented, uses quote date;
- customers/vendors/products/projects/jobs are not independently date-filtered;
- a June invoice with a July payment can produce an excluded invoice row and included July payment row;
- BAS GST recognition date does not control Financial inclusion;
- EOFY recognition date does not control Financial inclusion;
- BAS-style Financial selectors resolve to a date range, then use source-date filtering;
- missing required source date is surfaced as an exception rather than guessed;
- shared dataset facts remain traceable to Invoice Ninja records;
- raw diagnostic data remains available where the dataset would lose source fidelity;
- Financial filtering does not mutate or decorate the Accounting Dataset with report-period fields;
- unrestricted default;
- `--fy`;
- integer `--period`;
- short month `--period`;
- full month `--period`;
- `--fy` + `--period`;
- `--from` only;
- `--to` only;
- `--from` + `--to`;
- rejection of `--fy` with `--from`/`--to`;
- rejection of `--period` with `--from`/`--to`;
- same monthly/quarterly/yearly period parsing as BAS;
- optional `exports.directory`;
- current-working-directory fallback;
- deterministic filename generation for every selection mode;
- refusal on existing output without `--force`;
- replacement with `--force`.

## Verification

Run:

```text
gofmt
go vet
go test
go build
```

Maximum review/fix loops: 3.
