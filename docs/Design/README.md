# GoTradie Design Documents

This directory contains authoritative behaviour and architecture contracts for GoTradie features where intent matters as much as implementation.

The aim is to stop later work from reconstructing the premise from code, old chats, sedimentary layers, or the position of the moon.

## Documents
### [v0.5 design series](./0.5/README.md)

Release/slice-specific design and closeout material for the `v0.5.x` series.

### [v0.5.1 Expense Importing](./0.5/0.5.1/Expense-Importing.md)

Accepted architecture for historical Expense import and supplier-account settlement. It locks in:

- Invoice Ninja as the durable source of truth after import;
- spreadsheets as migration sources which may be archived after validation;
- Expenses as purchase and tax records;
- Bank Transactions/Transactions as supplier-account withdrawals and settlement records;
- customer Payments as money received, never supplier payments;
- deterministic supplier/date/FIFO reconstruction from Invoice Ninja records and stable GoTradie markers;
- tolerance of intermediate Unpaid state during partial settlement;
- optional Paid-state tidying only after full settlement;
- cash and non-cash BAS timing without double-counting;
- the Bunnings and BlueCarve canonical examples;
- an Invoice-Ninja-only target architectural/post-migration integrity test;
- explicit rejection of a GoTradie cache/SQLite side ledger, duplicate settlement Expenses, misuse of customer Payments, and required native partial links.

The document supersedes earlier spreadsheet-dependent or import-run-only allocation designs. The Invoice-Ninja-only integrity test describes the target post-migration architecture; it does not block completion of the current historical importer.

Related v0.5.1 records:

- [Possible Future Scenarios](./0.5/0.5.1/Expense-Importing-Possible-Scenarios.md) — explicitly deferred, non-blocking scenarios.
- [Implementation Review](./0.5/0.5.1/Expense-Importing-Review.md) — historical review evidence, not an open work list.

### Command-Line-Spec.md

The existing GoTradie command-line behaviour contract remains authoritative and is not replaced by this package.

Its safety rules continue to apply, especially:

- preview by default;
- `--commit` as the single persistent-write flag;
- command grouping and repository/SDK ownership boundaries.

## Working rule

These documents describe deliberate behaviour, not whatever happens to fall out of today's implementation.

When implementation and design disagree:

1. inspect the current GitHub branch and tests;
2. decide whether the code or the accepted design is wrong;
3. obtain approval before replacing an accepted invariant;
4. update code, tests, and affected documentation together.

Do not silently reinterpret a design contract because a nearby function looked persuasive. Functions are excellent at being confident and have never once had to explain themselves to an accountant.
