# GoTradie v0.5 Design Series

This directory groups design and closeout material for the GoTradie `v0.5.x` release series.

The purpose is simple: each implementation slice gets a clear home, a clear status, and a clear boundary so later work does not have to reconstruct history from branches, pull requests, or old chats.

## Releases

| Version | Status | Scope |
|---|---|---|
| [v0.5.1](./0.5.1/) | Closed | Historical Expense import and supplier-account settlement |
| [v0.5.2](./0.5.2/) | Not started | Next implementation slice / remaining v0.5 work |

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

`v0.5.2` is the next working slice.

Its design should be written and reviewed in its own directory before the slice is treated as closed.
