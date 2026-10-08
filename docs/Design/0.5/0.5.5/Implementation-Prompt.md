# GoTradie v0.5.5 — Implementation Prompt

Implement the accepted EOFY export design.

Primary contract:

```text
docs/Design/0.5/0.5.5/EOFY-Export.md
```

## Mandatory preflight

Before changing code:

1. Fetch latest `origin/main`.
2. Verify all earlier slice branches/PRs through v0.5.4 are merged.
3. If not, STOP and report it.
4. Inspect Accounting Dataset, Income/Expense/Payment/Transaction access, config, CLI export handling and XLSX support.
5. State intended implementation, branch, likely files/packages and blocking ambiguity.
6. STOP and wait for approval.

Suggested branch:

```text
v0.5.5-eofy-export
```

## CLI

Implement:

```text
GoTradie ninja export eofy
GoTradie ninja export eofy --fy 2027
```

`--fy` accepts `YYYY`.

No `--fy` means the most recently completed financial year.

Reject:

```text
--period
--from
--to
--all
--month
--quarter
```

EOFY export does not modify Invoice Ninja and does not use `--commit`.

Local output follows the global `--force` overwrite rule.

## Accounting rules

- Reuse Accounting Dataset.
- Invoice Ninja Expense Categories are authoritative.
- Use configured instant asset write-off threshold for review classification.
- Threshold comparison uses asset cost excluding claimable GST.
- Preserve source IDs and accountant-review evidence.
- Do not calculate depreciation.

## Verification

Run:

```text
gofmt
go vet
go test
go build
```

Maximum review/fix loops: 3.
