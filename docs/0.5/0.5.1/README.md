# GoTradie v0.5.1

Status: **Closed**

Branch: `feature/ninja-expense-import-2`  
PR: `#6`  
Merge commit: `db579dee2ffe99732b4d80807ba044acc1673247`

## Purpose

`v0.5.1` closes the historical Expense-import and supplier-account settlement work.

The primary goal was to import MickMake historical purchase data into Invoice Ninja safely and idempotently, while preserving enough source and accounting information for BAS/EOFY work.

## Documents

### [Expense-Importing.md](./Expense-Importing.md)

Accepted architecture and bookkeeping model for:

- historical purchase Expenses;
- supplier-account payments;
- deterministic FIFO settlement;
- Invoice Ninja as the durable source of truth;
- stable GoTradie identities;
- Paid/Unpaid handling;
- BAS/EOFY separation between purchases and settlement.

### [Expense-Importing-Possible-Scenarios.md](./Expense-Importing-Possible-Scenarios.md)

Deferred scenarios discovered during implementation and review.

These are **not unfinished v0.5.1 requirements** and must not be treated as blockers merely because they are documented.

### [Expense-Importing-Review.md](./Expense-Importing-Review.md)

Historical implementation review retained as evidence of the review process.

Its findings were subsequently triaged. The review itself is not an open work list.

## Closure rule

`v0.5.1` is closed.

New feature work belongs in `v0.5.2` unless a genuine defect is discovered in released `v0.5.1` behaviour.

Do not reopen this slice simply because a later requirement touches the same code.