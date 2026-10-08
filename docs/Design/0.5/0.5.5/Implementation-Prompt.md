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

Follow the global output-resolution and `--force` rules in `docs/Design/Command-Line-Spec.md`.

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
- Use configured instant asset write-off threshold for review classification.
- Threshold comparison uses asset cost excluding claimable GST.
- Preserve source IDs and accountant-review evidence.
- Do not calculate depreciation.

EOFY financial-year selection and workbook structure remain outside the Accounting Dataset.

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
