# GoTradie v0.5.6 - Implementation Prompt

Implement the accepted Financial Data Export design.

Primary contract:

```text
docs/Design/0.5/0.5.6/Financial-Export.md
```

Relevant contracts:

```text
docs/Design/0.5/0.5.4/BAS-Export.md
docs/Design/0.5/0.5.5/EOFY-Export.md
docs/Design/0.5/README.md
```

## Mandatory preflight

Before making any code changes:

1. Fetch latest `origin/main`.
2. Verify every earlier slice branch/PR through v0.5.5 has been merged or otherwise explicitly handled.
3. If any earlier slice branch/PR is not merged, STOP and report it.
4. Inspect current Invoice Ninja entity access, Accounting Dataset, CLI export structure, XLSX support and tests.
5. Identify which Invoice Ninja entities and relationships can be exported faithfully with the current SDK/model.
6. State:
   - intended implementation;
   - proposed branch;
   - likely files/packages to change;
   - any genuine source-data gaps.
7. STOP and wait for approval before creating the branch or changing code.

Suggested branch:

```text
v0.5.6-financial-export
```

After approval, branch from latest `origin/main` only.

## Required CLI behaviour

Implement:

```text
GoTradie ninja export financial
GoTradie ninja export financial --fy 2027
GoTradie ninja export financial --from 2026-01-01 --to 2026-12-31
```

Rules:

- no period flags = all available financial data;
- `--fy` = one financial year;
- `--from` and `--to` together = explicit date range;
- selection modes are mutually exclusive;
- there is no `--all` flag;
- partial `--from`/`--to` input is invalid.

## Workbook behaviour

Create a raw-data/diagnostic XLSX workbook.

Include the financial entities and relationships described by `Financial-Export.md` and actually available from the current Invoice Ninja model, including the equivalent of:

```text
Income
Expenses
Payments
Supplier Transactions
Customers
Vendors
Products
Projects-Jobs
Exceptions
```

Preserve useful raw identifiers and relationships so the workbook can be used to diagnose Invoice Ninja state.

Use the Accounting Dataset where useful for derived accounting values, but do not hide or replace raw source evidence needed for troubleshooting.

## Filtering

Implement deterministic period filtering for date-bearing financial entities.

- `--fy` resolves to Australian financial-year boundaries.
- `--from/--to` uses inclusive explicit boundaries unless existing export conventions dictate otherwise.
- all-data mode applies no period restriction.
- ambiguous/undated records that cannot be safely classified must remain visible rather than silently disappearing.

Document entity-specific date semantics in tests/code comments where necessary.

## XLSX scope

Keep formatting functional and cheap:

- stable headers;
- useful date/number formats;
- deterministic row ordering where practical;
- filters/frozen headers if already easy with the selected XLSX library.

Do not add dashboards, charts or presentation-heavy formatting.

## Tests

Add focused tests covering at least:

- no flags exports all available data;
- `--fy` restriction;
- `--from/--to` restriction;
- mutually exclusive selection modes;
- absence/rejection of `--all`;
- partial explicit ranges rejected;
- Australian FY boundary handling;
- preservation of record IDs/relationships;
- visibility of ambiguous/undated records;
- required XLSX sheets/headers;
- deterministic output ordering where practical.

## Scope exclusions

Do not implement:

```text
dashboards
charts
KPIs
margin engine
trend engine
BI layer
general ledger
persistent financial database
Product Sync
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

Finish with a concise completion report covering implemented behaviour, tests, verification results, and deliberately deferred analysis features.
