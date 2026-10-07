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
2. Verify all earlier slice branches/PRs through v0.5.3 are merged.
3. If not, STOP and report it.
4. Inspect Invoice Ninja invoice/payment/expense/transaction access, settlement reconstruction, configuration, CLI export handling and XLSX support.
5. State intended change, branch, likely files/packages and blocking ambiguity.
6. STOP and wait for approval.

Suggested branch:

```text
v0.5.4-bas-export
```

## CLI

Implement:

```text
GoTradie ninja export bas
GoTradie ninja export bas --fy 2027 --quarter 1
GoTradie ninja export bas --from 2026-07-01 --to 2026-09-30
```

No period flags means the most recently completed BAS cycle.

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

Implement G1, 1A, 1B, cash-basis customer payments, partial payments, ordinary Expenses, supplier-account settlement, proportional partial settlement and deterministic rounding.

## Verification

Run:

```text
gofmt
go vet
go test
go build
```

Maximum review/fix loops: 3.
