# GoTradie v0.5.6 — Implementation Prompt

Implement the accepted Financial Data Export design.

Primary contract:

```text
docs/Design/0.5/0.5.6/Financial-Export.md
```

Cross-audit and CLI contracts:

```text
docs/Design/0.5/Cross-Audit.md
docs/Design/Command-Line-Spec.md
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

Implement:

```text
GoTradie ninja export financial
GoTradie ninja export financial --fy 2027
GoTradie ninja export financial --from 2026-01-01 --to 2026-12-31
```

No period flags means all available financial data.

Financial export does not modify Invoice Ninja and therefore does not use `--commit`.

Local output follows the global export rule:

- create new output normally;
- refuse to replace existing output unless `--force` is supplied.

There is no `--all` flag.

## Workbook

Keep v1 raw-data/diagnostic focused.

Do not add dashboards, charts, KPI frameworks, margin engines or BI layers.

Preserve source IDs and useful relationships.

## Verification

Run:

```text
gofmt
go vet
go test
go build
```

Maximum review/fix loops: 3.
