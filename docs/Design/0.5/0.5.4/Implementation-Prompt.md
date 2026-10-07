# GoTradie v0.5.4 — Implementation Prompt

## Objective

Implement the accepted v0.5.4 BAS export design, including the minimum shared **Accounting Dataset** required by BAS and known later reporting needs.

Primary design contract:

```text
docs/Design/0.5/0.5.4/BAS-Export.md
```

Relevant accounting contract:

```text
docs/Design/0.5/0.5.1/Expense-Importing.md
```

Series context:

```text
docs/Design/0.5/README.md
```

Cross-release CLI contract:

```text
docs/Design/Command-Line-Spec.md
```

Do not reinterpret accepted accounting invariants without explicit approval.

## Mandatory preflight

Before making any code changes:

1. Fetch the latest `origin/main`.
2. Verify all earlier slice branches and pull requests are merged into `main`, including v0.5.3.
3. If any earlier slice branch or PR is not merged, STOP and report it.
4. Inspect current Invoice Ninja invoice/payment/expense/transaction access, existing tax export code, supplier-settlement reconstruction, configuration after v0.5.3, and current XLSX dependency situation.
5. State the intended implementation, proposed branch name, likely files/packages, and any data/API gaps that block correct BAS calculation.
6. STOP and wait for approval before creating the branch or modifying code.

Suggested branch name:

```text
v0.5.4-bas-export
```

After approval, branch from latest `origin/main` only.

## Required implementation

Add:

```text
GoTradie ninja export bas --from YYYY-MM-DD --to YYYY-MM-DD
```

and:

```text
GoTradie ninja export bas --fy YYYY --quarter N
```

Australian financial-year quarters are:

```text
Q1 = 1 Jul - 30 Sep
Q2 = 1 Oct - 31 Dec
Q3 = 1 Jan - 31 Mar
Q4 = 1 Apr - 30 Jun
```

`FY2027` means:

```text
1 July 2026 - 30 June 2027
```

The expected initial GST basis is cash.

## Accounting Dataset

Introduce a shared in-memory Accounting Dataset used by BAS and intended for later reuse by EOFY and Financial exports.

It should own only reusable accounting facts/calculations needed by BAS now and already-known EOFY/Financial needs later.

It must not become a general ledger or speculative accounting framework.

Core reusable concerns include:

- income recognition;
- expense recognition;
- GST attribution;
- cash timing;
- partial customer payments;
- supplier-account allocation;
- partial supplier payments;
- deterministic rounding;
- business-use percentages;
- GST treatment.

Do not persist the Accounting Dataset.

Invoice Ninja remains the durable source of truth.

## BAS calculation

Implement:

```text
G1  Total sales
1A  GST on sales
1B  GST on purchases
```

and calculate:

```text
Net GST position = 1A - 1B
```

For cash GST:

### Sales

- customer payment timing controls inclusion;
- partial customer payments contribute only the paid proportion attributable to the BAS period;
- GST allocation must be proportional and deterministic;
- every contribution must be traceable to Invoice Ninja records.

### Purchases

For ordinary immediately-paid Expenses:

```text
Expense payment date + Expense GST attributes = BAS contribution
```

For supplier-account purchases:

```text
Expense = purchase and GST attributes
Marked supplier Transaction = payment timing
```

Supplier Transactions must never be counted as additional purchases.

Partial supplier-account payments must contribute proportional GST based on the underlying Expense using the accepted settlement rules.

## Existing accounting invariants

Preserve:

```text
Expenses = what was purchased
Supplier Transactions = when supplier-account purchases were settled
Customer Payments = when customer invoices were paid
```

The historical spreadsheet must not be required at runtime.

Do not create SQLite allocation state, a persistent GoTradie accounting cache, a second accounting database, duplicate purchase records, or alternative settlement truth.

## XLSX output

Primary output:

```text
FY2027-Q1-BAS.xlsx
```

Workbook sheets:

```text
Summary
Sales
Purchases
Exceptions
```

### Summary

Include Financial year, Quarter, Period start, Period end, GST basis, Generated timestamp, Source, G1, 1A, 1B, and Net GST position.

### Sales

Provide source-level evidence for every contribution to G1/1A.

### Purchases

Provide source-level evidence for every contribution to 1B.

### Exceptions

Surface unresolved states explicitly, including ambiguous supplier settlement, unapplied supplier payment, missing payment date, missing GST treatment, unsupported foreign-currency settlement, archived/deleted marked accounting records, and other unresolved accounting state.

Material exceptions must prevent the output from appearing authoritative without warning.

## Scope exclusions

Do not implement direct ATO lodgement, PAYG withholding, PAYG instalments, FBT, payroll, complete personal tax-return generation, EOFY workbook, Financial workbook, product syncing, a general ledger, or speculative future Accounting Dataset features.

## Tests

Add tests covering at least:

- FY/quarter date resolution;
- explicit date ranges;
- cash-basis full customer payment;
- cash-basis partial customer payment;
- ordinary paid Expense;
- supplier-account Expense settled in full;
- partial supplier-account settlement;
- GST proportional allocation and rounding;
- no double-counting of supplier Transactions;
- exception generation;
- deterministic repeated output from identical Invoice Ninja state;
- XLSX workbook/sheet creation.

Use integer minor units where accounting allocation requires deterministic rounding.

## Verification

Before declaring complete:

```text
gofmt
go vet
go test
go build
```

Then perform an evidence-backed review.

Maximum review/fix loops: **3**.

If material issues remain after three loops, STOP and report them.

## Completion report

Report branch used, files/packages changed, Accounting Dataset shape, BAS calculations implemented, workbook sheets/output, tests added, verification results, exceptions/deferred limitations, and readiness for PR/review.

> If a BAS number cannot be traced back to supporting Invoice Ninja records, the report is not finished.
