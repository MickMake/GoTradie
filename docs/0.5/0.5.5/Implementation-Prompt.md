# GoTradie v0.5.5 — Implementation Prompt

Implement the accepted EOFY export design.

Primary contract:

```text
docs/0.5/0.5.5/EOFY-Export.md
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

## EOFY accounting basis

Require:

```yaml
eofy:
  accounting_basis: cash
```

Supported values:

```text
cash
accrual
```

Apply EOFY income/expense recognition deterministically from `eofy.accounting_basis`.

Do not derive or infer EOFY recognition from:

```text
bas.gst_basis
```

Treat BAS GST timing and EOFY accounting recognition as separate configuration and separate accounting rules.

The workbook must state the resolved EOFY accounting basis.

## Output

Follow the global output-resolution and `--force` rules in `docs/Command-Line-Spec.md`.

Default filename:

```text
FYyyyy-EOFY.xlsx
```

Use optional `exports.directory` when configured; otherwise use the current working directory.

Do not append timestamps or automatic collision suffixes.

## Accounting rules

- Reuse the v0.5.4 Accounting Dataset minimum contract.
- Do not create EOFY-only duplicate accounting facts that already belong in the shared dataset.
- Apply income/expense recognition according to `eofy.accounting_basis`.
- Never substitute `bas.gst_basis` for EOFY accounting basis.
- Invoice Ninja Expense Categories are authoritative.
- Preserve source IDs and accountant-review evidence.
- Do not calculate depreciation.

EOFY financial-year selection and workbook structure remain outside the Accounting Dataset.

## Instant asset write-off review

Use:

```yaml
eofy:
  instant_asset_writeoff_threshold: 20000
```

Treat the configured value as the threshold applicable to the selected EOFY financial year.

Do not hard-code a universal threshold.

For each asset under review, calculate:

```text
threshold-test cost
    = relevant asset cost
    - GST input tax credits the business is entitled to claim
```

Rules:

```text
no GST credit entitlement
    -> subtract no GST

partial GST credit entitlement
    -> subtract only the claimable GST credit

private/non-business use
    -> do not reduce threshold-test cost

threshold eligibility
    -> compare entire threshold-test cost with configured threshold

business-use percentage
    -> apply after threshold eligibility, to deductible/review amount
```

Eligibility is tied to the financial year in which the asset is first used or installed ready for use.

The workbook must expose enough evidence to review the calculation, including:

```text
first-used / installed-ready-for-use date
source asset cost
GST amount
claimable GST credit used
threshold-test cost
configured threshold
business-use percentage
business-use-adjusted amount
source record ID
classification/result
```

Items that are not straightforward instant asset write-off candidates remain review items. Do not implement depreciation or pooling calculations.

## Tests

Cover at least:

- EOFY uses the shared dataset source IDs and recognition events;
- EOFY does not add FY labels or workbook presentation fields to the dataset;
- required `eofy.accounting_basis`;
- supported `cash`;
- supported `accrual`;
- rejection of unsupported EOFY accounting basis;
- EOFY recognition changes only according to `eofy.accounting_basis`;
- changing `bas.gst_basis` does not change EOFY recognition;
- workbook states EOFY accounting basis;
- full GST credit reduces threshold-test cost by the claimable GST credit;
- no GST credit leaves GST in threshold-test cost;
- partial GST credit removes only the claimable portion;
- private/business-use percentage does not alter threshold eligibility;
- business-use percentage affects the deductible/review amount after threshold testing;
- asset at or above threshold is not treated as below-threshold;
- first-used / installed-ready-for-use date determines the relevant financial year;
- workbook states the threshold used;
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
