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

Use mandatory BAS configuration:

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

Do not use the old `bas.frequency` name.

## CLI

Implement:

```text
GoTradie ninja export bas
GoTradie ninja export bas --fy 2027
GoTradie ninja export bas --period 2
GoTradie ninja export bas --fy 2027 --period 2
```

Defaults:

```text
no --fy     -> current Australian financial year
no --period -> current reporting period
```

Interpret `--period` according to configured `bas.reporting_period`:

```text
monthly   -> 1-12, numbered July through June
quarterly -> 1-4, numbered Q1 through Q4
yearly    -> --period is invalid
```

Reject:

```text
--from
--to
--all
--month
--quarter
```

Arbitrary date-range selection belongs to the Financial/dump export, not BAS.

BAS export does not modify Invoice Ninja and therefore does not use `--commit`.

Local output follows the global export rule:

- create new output normally;
- refuse to replace existing output unless `--force` is supplied.

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

The workbook must state the resolved financial year, reporting-period type, reporting-period number where applicable, period start/end, and GST basis.

## Tests

Cover at least:

- current-FY default;
- explicit `--fy`;
- current-period default;
- monthly period 1-12 mapping;
- quarterly period 1-4 mapping;
- yearly rejection of `--period`;
- invalid period values;
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
