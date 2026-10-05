# GoTradie v0.5.2

Status: **Design drafted — implementation not started**

## Purpose

`v0.5.2` improves the operational behaviour of the historical Expense importer established in `v0.5.1`.

The accounting model remains unchanged.

The primary work is:

- live per-row import progress;
- concise NEW / EXISTING / ERROR reporting;
- expanded error detail;
- end-of-import summaries;
- configurable batch processing;
- re-entrant batch behaviour;
- removal of CSV row-number dependence from durable identity;
- receipt deduplication across Expense line items.

## Design

### [Expense-Importing-UX.md](./Expense-Importing-UX.md)

Draft design for Expense-import operational UX, batching, identity cleanup and receipt deduplication.

This document remains draft until the final minor scope additions from real-world testing are agreed.

## Starting point

`v0.5.2` starts from the closed `v0.5.1` Expense-import architecture.

New work must not weaken the accounting and supplier-settlement invariants established in `v0.5.1`.

## Implementation status

Implementation has not started.

Once the design is accepted, create a fresh `v0.5.2` implementation branch and keep the slice bounded to the agreed scope.

## Scope

The exact scope of `v0.5.2` is still to be defined.

When the slice is selected, this README should be updated with:

- purpose;
- branch and PR;
- accepted design documents;
- explicit in-scope work;
- explicit deferred work;
- closure status.

## Working rule

Keep the slice small enough that it can be designed, implemented, reviewed, verified, and then closed cleanly.

The objective is not to predict every future requirement. It is to make the next piece of work understandable when returning to it a week later.
