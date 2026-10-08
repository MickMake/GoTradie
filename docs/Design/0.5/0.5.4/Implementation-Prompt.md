# GoTradie v0.5.4 — Implementation Prompt

Implement the accepted BAS export design.

Primary contract:

```text
docs/Design/0.5/0.5.4/BAS-Export.md
```

Cross-audit and CLI contracts:

```text
docs/Design/0.5/Cross-Audit.md
docs/Design/Command-Line-Spec.md
```

## Mandatory preflight

Before changing code:

1. Fetch latest `origin/main`.
2. Verify all earlier slice branches/PRs through v0.5.3 are merged or explicitly retired.
3. If an earlier slice branch/PR is still active and unmerged, STOP and report it.
4. Inspect Invoice Ninja accounting access, configuration, CLI export handling and XLSX support.
5. State intended change, branch, likely files/packages and blocking ambiguity.
6. STOP and wait for approval.

Suggested branch:

```text
v0.5.4-bas-export
```

## Configuration

Use:

```yaml
bas:
  reporting_period: quarterly
  gst_basis: cash
  ato_due_dates:
    verify_every_days: 30
```

## CLI selection

`--fy` accepts `YYYY`.

If omitted, derive the current Australian FY from today's date:

```text
30/06/2020 -> FY2020
01/07/2020 -> FY2021
```

Resolve:

```text
bas
    -> current FY + natural date-driven BAS period

bas --fy CURRENT
    -> current FY + natural date-driven BAS period

bas --fy HISTORIC
    -> all BAS periods in HISTORIC FY

bas --period VALUE
    -> current FY + selected BAS period

bas --fy YEAR --period VALUE
    -> YEAR + selected BAS period
```

The default/no-period workflow is date-driven using period-end and ATO lodgement due-date rules. Do not track whether a BAS has actually been lodged.

### `--period`

Case-insensitive.

Monthly BAS accepts:

```text
1..12
Jan..Dec
January..December
```

Quarterly BAS accepts:

```text
1..4
Jan..Dec
January..December
```

Quarterly month names map to their containing quarter.

Yearly BAS rejects `--period`.

Do not add fuzzy month parsing or non-standard aliases.

Reject BAS:

```text
--from
--to
--all
--month
--quarter
```

BAS export does not modify Invoice Ninja and does not use `--commit`.

Local output follows the global `--force` overwrite rule.

## ATO due-date verification

Use:

```text
bas.ato_due_dates.verify_every_days
```

Allow a small local operational cache such as:

```text
~/.GoTradie/cache/ato_due_dates.json
```

The cache may store only ATO due-date verification metadata and cached rules.

It must not store lodgement state or accounting data.

When verification is stale, attempt to refresh the ATO due-date rules.

If refresh fails:

```text
continue using existing cached/configured due dates
print a warning to stdout that due-date verification is stale
do not fail BAS generation solely because refresh failed
```

## Accounting Dataset

Build only the minimum shared in-memory Accounting Dataset required by BAS and known later reporting needs.

Do not add persistent accounting state.

## Workbook

Produce:

```text
Summary
Sales
Purchases
Exceptions
```

Implement G1, 1A, 1B, cash-basis customer payments, partial payments, ordinary Expenses, supplier-account settlement, proportional partial settlement, accrual-basis timing, and deterministic rounding.

For `--fy HISTORIC` with no period, output all periods in that FY and make each period clearly identifiable.

## Tests

Cover at least:

- FY derivation on 30 June and 1 July;
- no-flag natural-period selection before due date;
- no-flag selection after due date but before next period completion;
- current `--fy` with no period;
- historical `--fy` returning all periods;
- `--fy` + integer/short-month/full-month period;
- monthly integer/month parsing;
- quarterly integer/month-to-quarter parsing;
- case-insensitive month parsing;
- yearly rejection of `--period`;
- ATO verification interval;
- failed/stale ATO refresh prints stdout warning and continues;
- no lodgement-state persistence;
- cash/accrual GST basis;
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
