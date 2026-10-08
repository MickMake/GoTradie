# GoTradie v0.5.6 — Implementation Prompt

Implement the accepted Financial Data Export design.

Primary contract:

```text
docs/Design/0.5/0.5.6/Financial-Export.md
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

Local output follows the global `--force` overwrite rule.

## Workbook

Keep v1 raw-data/diagnostic focused.

Do not add dashboards, charts, KPI frameworks, margin engines or BI layers.

Preserve source IDs and useful relationships.

## Tests

Cover at least:

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
- local overwrite safety.

## Verification

Run:

```text
gofmt
go vet
go test
go build
```

Maximum review/fix loops: 3.
