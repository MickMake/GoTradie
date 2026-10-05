# GoTradie v0.5 Design Series

This directory groups design and closeout material for the GoTradie `v0.5.x` release series.

The purpose is simple: each implementation slice gets a clear home, a clear status, and a clear boundary so later work does not have to reconstruct history from branches, pull requests, or old chats.

## Releases

| Version | Status | Scope |
|---|---|---|
| [v0.5.1](./0.5.1/) | Closed | Historical Expense import and supplier-account settlement |
| [v0.5.2](./0.5.2/) | Design drafted | Expense Import ID/preflight, operational UX, batching and receipt deduplication |

## Working rule

A version directory describes the design and decisions for that slice.

Once a slice is closed:

- its accepted design documents remain as the record of what was intended;
- review material remains historical evidence, not an automatically open work list;
- explicitly deferred scenarios remain deferred unless a later real requirement brings them back;
- new feature work moves to the next version directory rather than quietly extending the closed slice.

Cross-release contracts such as `Command-Line-Spec.md` remain at `docs/Design/` rather than being duplicated into every version directory.

## v0.5.1

`v0.5.1` closes the historical Expense-import work implemented through PR #6.

Its directory contains:

- the accepted Expense-import architecture;
- the implementation/review record;
- deliberately deferred possible future scenarios.

Those deferred scenarios are **not unfinished v0.5.1 requirements**.

## v0.5.2

`v0.5.2` improves the operational usability and source safety of the historical Expense importer without changing the `v0.5.1` accounting model.

Current draft design:

- [Expense Importing — Identity, Preflight, Operational UX and Batching](./0.5.2/Expense-Importing-UX.md)

The draft now locks in:

- mandatory user-supplied `Import ID`;
- complete-file preflight before execution;
- no stdin for Expense import;
- arithmetic/source sanity checks;
- true batching only for ordinary Expense-only files;
- settlement-safe whole-file mode when Account Payment rows exist;
- durable receipt deduplication/ownership.

Implementation has not started.
