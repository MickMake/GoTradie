# GoTradie v0.5 Design Series

This directory groups design and closeout material for the GoTradie `v0.5.x` release series.

The purpose is simple: each implementation slice gets a clear home, a clear status, and a clear boundary so later work does not have to reconstruct history from branches, pull requests, or old chats.

## Releases

| Version | Status | Scope |
|---|---|---|
| [v0.5.1](./0.5.1/) | Closed | Historical Expense import and supplier-account settlement |
| [v0.5.2](./0.5.2/) | Design drafted | Expense Import ID/preflight, operational UX, batching and receipt deduplication |
| [v0.5.3](./0.5.3/) | Planned | Hierarchical YAML configuration and migration from flat config |
| [v0.5.4](./0.5.4/) | Planned | Shared Accounting Dataset and BAS XLSX export |
| [v0.5.5](./0.5.5/) | Planned — design incomplete | EOFY preparation XLSX export |
| [v0.5.6](./0.5.6/) | Planned — design incomplete | Financial analysis XLSX export |
| [v0.5.7](./0.5.7/) | Planned | Product synchronisation, provider routing and catalogue sources |

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

## v0.5.3

`v0.5.3` replaces the current flat `key=value` configuration with hierarchical YAML.

It is infrastructure only. It must not quietly absorb BAS, EOFY, Financial or Product Sync implementation.

The old flat config remains readable for this transition release so existing installations can move deliberately to YAML.

## v0.5.4

`v0.5.4` introduces the shared **Accounting Dataset** and uses BAS export as its first concrete consumer.

The Accounting Dataset owns reusable accounting calculations. The BAS workbook owns BAS-specific presentation.

This slice should implement only the shared accounting machinery required by BAS and already-known later reporting needs. It must not become a speculative general accounting framework.

## v0.5.5

`v0.5.5` will produce the EOFY preparation XLSX workbook using the Accounting Dataset from `v0.5.4`.

Its current document is a roadmap-level design, not yet an implementation-complete specification.

Before implementation begins, its workbook structure and required outputs must be completed and accepted.

## v0.5.6

`v0.5.6` will produce the Financial Analysis XLSX workbook using the Accounting Dataset from `v0.5.4`.

Its current document is a roadmap-level design, not yet an implementation-complete specification.

Before implementation begins, its workbook structure and required outputs must be completed and accepted.

## v0.5.7

`v0.5.7` redesigns Product Sync around a simple two-phase rule:

> Sync known Invoice Ninja Products first. Then look for products that are missing.

It depends on the hierarchical provider configuration introduced in `v0.5.3`, but does not depend on the accounting-reporting slices.

If implementation scope grows materially, keep the release boundary but implement internally in small phases rather than creating one large reconciliation engine.
