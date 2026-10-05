# GoTradie v0.5.2

Status: **Design drafted — implementation not started**

## Purpose

`v0.5.2` improves the operational behaviour of the historical Expense importer established in `v0.5.1`.

The accounting model remains unchanged.

The primary work is:

- mandatory user-supplied `Import ID` identity for every imported source row;
- whole-file preflight validation before import execution;
- accounting sanity checks and warnings;
- removal of CSV row-number dependence from durable identity;
- named-file-only Expense import; stdin is not supported;
- live per-row import progress;
- concise NEW / EXISTING / ERROR reporting;
- expanded error detail;
- end-of-import summaries;
- configurable batch processing for ordinary Expense-only imports;
- settlement-safe whole-file processing when Account Payment rows are present;
- re-entrant behaviour without a GoTradie checkpoint database;
- receipt deduplication with durable receipt ownership.

## Design

### [Expense-Importing-UX.md](./Expense-Importing-UX.md)

Design for Expense-import identity, preflight validation, operational UX, batching and receipt deduplication.

The `v0.5.1` accounting and supplier-settlement architecture remains authoritative except where this document deliberately replaces the old row-derived source-identity mechanism.

## Starting point

`v0.5.2` starts from the closed `v0.5.1` Expense-import architecture.

New work must not weaken the accounting and supplier-settlement invariants established in `v0.5.1`.

## Implementation status

Implementation has not started.

When implementation begins, create a fresh `v0.5.2` implementation branch from current `main` and keep the slice bounded to this design.

## In scope

- replace row-derived durable purchase/payment identity with required `Import ID`;
- preflight the entire source file before processing;
- reject missing, blank or duplicate `Import ID` values;
- reject stdin for Expense import;
- report suspicious source/accounting arithmetic before import;
- preserve preview-by-default and `--commit` as the sole persistent-write flag;
- stream useful per-row progress;
- add practical execution batching for ordinary Expense-only files;
- preserve whole-file supplier-settlement visibility when Account Payment rows exist;
- make receipt upload ownership durable and re-entrant;
- retain compatibility with the accepted `v0.5.1` bookkeeping model.

## Deferred / non-goals

- general surrogate-ID generation by GoTradie;
- guessing identity from CSV position or occurrence order;
- automatic repair of suspicious source arithmetic;
- concurrent Invoice Ninja writes;
- persistent checkpoint/state databases;
- redesign of supplier FIFO settlement;
- general shared-document infrastructure;
- foreign-currency supplier-account settlement;
- Adjustment accounting.

## Working rule

Keep the slice small enough that it can be designed, implemented, reviewed, verified, and then closed cleanly.

The immediate objective is a boring, safe historical import that can get the BAS work done without requiring GoTradie to become a philosopher of identity.
