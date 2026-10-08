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
4. Inspect Invoice Ninja invoice/payment/expense/transaction access, settlement reconstruction, configuration, CLI export handling and XLSX support.
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
```

Supported `reporting_period` values:

```text
monthly
quarterly
yearly
```

Supported `gst_basis` values:

```text
cash
accrual
```

## CLI

Implement for monthly and quarterly reporting:

```text
GoTradie ninja export bas
GoTradie ninja export bas --fy 2027
GoTradie ninja export bas --period 1
GoTradie ninja export bas --period Jul
GoTradie ninja export bas --fy 2025 --period 1
GoTradie ninja export bas --fy 2025 --period Jul
```

Resolve:

```text
no --fy, no --period
    -> current FY + current period

--fy CURRENT
    -> current FY + current period

--fy NON-CURRENT
    -> fail; --period required

--period VALUE
    -> current FY + selected period

--fy YEAR --period VALUE
    -> YEAR + selected period
```

Never infer a historical or future BAS period from today's date.

The current date may determine the BAS period only when the selected FY is the current Australian FY.

### `--period`

Accept either:

```text
integer period number
standard three-letter English month abbreviation
```

Accepted month values:

```text
Jan
Feb
Mar
Apr
May
Jun
Jul
Aug
Sep
Oct
Nov
Dec
```

Parsing is case-insensitive.

Do not accept full month names, `Sept`, partial names, or fuzzy aliases.

Integer interpretation:

```text
monthly   -> 1-12, July through June
quarterly -> 1-4, Q1 through Q4
yearly    -> invalid
```

Month interpretation:

```text
monthly:
    Jul -> 1
    Aug -> 2
    Sep -> 3
    Oct -> 4
    Nov -> 5
    Dec -> 6
    Jan -> 7
    Feb -> 8
    Mar -> 9
    Apr -> 10
    May -> 11
    Jun -> 12

quarterly:
    Jul/Aug/Sep -> 1
    Oct/Nov/Dec -> 2
    Jan/Feb/Mar -> 3
    Apr/May/Jun -> 4
```

For yearly reporting:

```text
GoTradie ninja export bas
    -> current FY

GoTradie ninja export bas --fy YEAR
    -> selected FY
```

Reject every `--period` value in yearly mode.

Also reject:

```text
--from
--to
--all
--month
--quarter
```

BAS export does not modify Invoice Ninja and does not use `--commit`.

Local output follows the global `--force` overwrite rule.

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

The workbook must state resolved FY, reporting-period type, reporting-period number where applicable, period start/end, and GST basis.

## Tests

Cover at least:

- current-FY/current-period default;
- explicit current `--fy` with no period;
- rejection of non-current `--fy` without period for monthly reporting;
- rejection of non-current `--fy` without period for quarterly reporting;
- explicit non-current `--fy` with integer period;
- explicit non-current `--fy` with month period;
- monthly integer mapping 1-12;
- quarterly integer mapping 1-4;
- monthly three-letter month mapping;
- quarterly month-to-quarter mapping;
- case-insensitive month parsing;
- rejection of full month names;
- rejection of `Sept`;
- rejection of invalid/partial month names;
- yearly current-FY default;
- yearly explicit current/historical/future `--fy`;
- yearly rejection of integer `--period`;
- yearly rejection of month `--period`;
- invalid integer period values;
- rejection of `--from/--to`;
- rejection of `--month` and `--quarter`;
- cash GST basis;
- accrual GST basis;
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
