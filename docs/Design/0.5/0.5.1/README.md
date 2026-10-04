# GoTradie Design Documents

This directory contains authoritative behaviour and architecture contracts for GoTradie features where intent matters as much as implementation.

The aim is to stop later work from reconstructing the premise from code, old chats, sedimentary layers, or the position of the moon.

## Documents

### [Expense-Importing.md](./Expense-Importing.md)

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
