# Expense Importing — Operational UX and Batching

Status: **Draft design for v0.5.2**  
Software version: `v0.5.2`

## Purpose

`v0.5.1` established the accounting model and durable Invoice Ninja representation for historical Expense import and supplier-account settlement.

`v0.5.2` does not redesign that model.

Its purpose is to make the importer practical to use against the real historical dataset, including CSV files containing thousands of purchase line items.

During real-world testing of `v0.5.1`, the accounting behaviour was broadly correct, but the importer had poor operational feedback:

- a large import could appear to have stopped working while remote API operations continued;
- row results were only displayed after the entire import call returned;
- successful rows produced more detail than was useful while errors were not prominent enough;
- manually splitting a large CSV into smaller files provided a useful operational workaround;
- multiple Expense line items belonging to the same physical receipt caused the same receipt file to be uploaded repeatedly;
- CSV row position was found to have leaked into parts of imported-record reconstruction even though row position is not a durable property of an Expense.

`v0.5.2` addresses those problems.

The guiding rule is:

> A large Expense import should visibly make progress, should be safely repeatable, and should not require the operator to wonder whether it has stopped working.

---

## Relationship to v0.5.1

The accepted `v0.5.1` Expense-import architecture remains authoritative.

In particular, `v0.5.2` must not change these rules:

- an Expense represents what was purchased;
- a supplier Account Payment represents settlement and may create a Bank Transaction;
- ordinary paid Expenses do not require synthetic Bank Transactions;
- Invoice Ninja remains the durable source of truth;
- GoTradie must not introduce a persistent side ledger, SQLite database, checkpoint database, or allocation cache;
- supplier settlement remains deterministic and fail-safe;
- preview remains the default;
- `--commit` remains the only persistent-write flag;
- stable identities must remain idempotent across reruns.

`v0.5.2` is therefore primarily an importer execution and user-interface slice.

---

## 1. Live import progress

### Problem

`v0.5.1` performs the import and returns a complete result set before the command-line application displays the result table.

For a large import this can result in long periods with no visible output.

Testing showed that approximately 100 real Expense line items can take around 20 seconds to process against Invoice Ninja.

At that rate a 3,000-line historical import can spend long periods appearing idle even though useful work is being performed.

This is not acceptable operational feedback.

### Required behaviour

The importer must report useful progress while the import is running.

Normal successful-row output should be deliberately concise.

Example:

```text
NEW        17/3000  30/09/2020  IFTTT                         $5.61  20200930-Receipt-2919-1280.pdf
EXISTING   18/3000  30/10/2020  IFTTT                         $5.66  20201031-Receipt-2215-9799.pdf
NEW        19/3000  24/11/2020  BackBlaze                    $13.61  BackBlazeB2B-2020.pdf
```

Useful fields are:

```text
State
Progress
Date
Supplier
Business Amount
Receipt filename
```

Exact widths and formatting are implementation details.

The objective is that one glance answers:

> Is it still running, what is it currently doing, and is it creating something new?

---

## 2. Import result state

The first visible field should describe what GoTradie concluded about the row.

At minimum:

```text
NEW
EXISTING
ERROR
```

Additional states may be used where they correspond to real importer behaviour:

```text
UPDATED
DEFERRED
```

The normal success path should remain concise.

Do not print the complete internal mutation list for every successful row unless explicitly requested through a future verbose mode.

Preview mode should use truthful equivalent terminology:

```text
WOULD-CREATE
EXISTING
WOULD-UPDATE
ERROR
```

Preview output must not imply that a write actually occurred.

---

## 3. Error reporting

Errors should be visually distinct and more verbose than successful rows.

Example:

```text
ERROR      127/3000  14/07/2024  Bunnings - Dural             $129.00  Bunnings-12345.pdf
           Category: Foo
           Error: unknown Invoice Ninja Expense Category "Foo"
```

An error should identify at least:

- input position for operator diagnostics;
- Date;
- Supplier;
- Business Amount;
- File Name;
- the actual error.

Errors must not be hidden inside a large success table.

---

## 4. End-of-import summary

Each import should finish with a concise summary.

Example:

```text
Import complete

Processed:    3000
New:          2841
Existing:      103
Updated:        47
Deferred:        2
Errors:          7
```

For batched operation, a short summary may also be displayed after each batch.

```text
Batch 4 complete: 98 new, 2 existing, 0 errors
```

---

## 5. Batched execution

### Problem

The current importer treats the supplied CSV as one import operation.

During testing, manually splitting the source CSV into approximately 100-line chunks and supplying the header to each chunk proved to be a practical workaround.

`v0.5.2` should automate that operational model.

### CLI

Proposed option:

```text
--batch-size N
```

Example:

```bash
GoTradie ninja import expenses expenses.csv \
    --receipts-root receipts \
    --batch-size 100 \
    --commit
```

A batch size of `100` is expected to be a sensible working value, but it should remain configurable.

### Behaviour

Conceptually:

```text
read header

read next N data rows
process batch
report batch

read next N data rows
process batch
report batch

repeat until EOF
```

This should behave substantially like the current manual process of splitting the file into smaller CSV files.

The importer must not require the operator to physically create those files.

Batching is an execution convenience.

It must not create a new accounting model or durable batch state.

---

## 6. Re-entrant behaviour

Batching depends on the importer remaining safely re-entrant.

If an import stops after some batches have successfully committed, the supported recovery action is:

```text
Run the original import again.
```

Already imported records should resolve as existing records.

No persistent GoTradie resume database is required.

No `.state` file is required.

No checkpoint ledger is required.

Invoice Ninja and the stable GoTradie identities remain sufficient to determine what has already been imported.

---

## 7. Optional pause between batches

Optional flag:

```text
--pause
```

Example:

```bash
GoTradie ninja import expenses expenses.csv \
    --receipts-root receipts \
    --batch-size 100 \
    --pause \
    --commit
```

Example output:

```text
Batch 6 complete: 100 processed, 98 new, 2 existing, 0 errors.

Continue? [Y/n]
```

Without `--pause`, batches continue automatically.

`--pause` creates no persistent state.

---

## 8. CSV row numbers are diagnostic only

CSV row position must never be part of the durable identity of an Expense or supplier settlement record.

A row number describes:

> where this record happened to appear in this particular input file

It does not describe:

> what accounting record this is.

Permitted uses include:

```text
[1847/3000]
```

and:

```text
CSV row 1847: unknown category
```

CSV row number must not determine:

- durable Expense identity;
- duplicate identity;
- stable occurrence identity;
- supplier-account identity;
- settlement allocation;
- reconciliation order;
- whether a later run recognises an existing record.

`v0.5.2` must remove any remaining durable reconstruction dependence on `GoTradie source row`.

The existing source-row note may be retained temporarily for debugging or historical compatibility, but it must not be relied upon for accounting correctness or identity.

---

## 9. Duplicate purchase identity

Removing source-row dependence exposes a legitimate edge case.

Two different purchase line items might theoretically contain identical values across every field used by the stable purchase identity.

For example:

```text
Nails   1   EA   $10
Nails   1   EA   $10
```

The importer must not solve this by treating CSV row number as durable identity.

If the existing stable purchase fields cannot distinguish genuinely different source records, GoTradie must either:

1. derive another genuine source-level discriminator; or
2. explicitly report the identity as ambiguous.

The exact solution should be chosen after examining real examples from the historical data.

Do not introduce a general surrogate-ID framework merely because the edge case can be imagined.

---

## 10. Receipt deduplication

### Problem

A single physical receipt may contain multiple Expense line items.

The source CSV can therefore legitimately contain multiple rows with the same:

```text
File Name
```

In `v0.5.1`, each Expense can independently upload that receipt.

This can result in several identical copies of the same PDF being stored in Invoice Ninja.

That is unnecessary.

### Invoice Ninja limitation

Invoice Ninja Expense documents are attached to individual Expense records.

There is no currently identified native mechanism allowing several Expenses to reference one already-uploaded Expense document as a shared attachment.

GoTradie should therefore not fabricate a native shared-document relationship.

### v0.5.2 behaviour

One Expense should become the receipt-document owner.

Example:

```text
Receipt: Bunnings-12345.pdf

Expense A
  actual document attached

Expense B
  Source file: Bunnings-12345.pdf
  Receipt owner: Expense A

Expense C
  Source file: Bunnings-12345.pdf
  Receipt owner: Expense A
```

All Expenses must retain the original source filename.

The receipt itself should normally be uploaded only once.

---

## 11. Receipt ownership must be re-entrant

Receipt deduplication must continue to work across:

- batches;
- interrupted imports;
- reruns;
- manually sliced source files.

A rerun must be able to discover that a receipt has already been uploaded.

Receipt ownership therefore cannot exist only in an in-memory map.

The required information must be recoverable from Invoice Ninja.

Possible durable information includes:

- existing Expense documents;
- stable Expense identities;
- durable source filename metadata.

The implementation should prefer existing durable records over assuming that the first occurrence in the current process owns the file.

---

## 12. Supplier-account settlement and batching

The `v0.5.1` supplier-account rules remain unchanged.

Batching must not weaken:

- supplier Account Payments remain Bank Transactions;
- allocation remains deterministic;
- older eligible unresolved purchases must not be silently skipped;
- partial settlement remains supported;
- archived/deleted marked records remain fail-safe;
- no GoTradie side ledger may be introduced.

Ordinary immediately-paid purchase rows can be processed independently in batches without changing their accounting meaning.

Supplier-account settlement is more sensitive because an Account Payment may depend on purchase history outside the current batch.

Therefore:

> `v0.5.2` must not silently use reduced batch visibility to produce a different settlement result from the accepted `v0.5.1` rules.

The implementation may choose the simplest safe behaviour.

For example:

- normal Expense rows use true batch processing;
- files containing supplier Account Payment rows may retain the existing settlement-safe processing path if necessary.

If safe batch settlement is not implemented initially, say so rather than guessing.

---

## 13. Progress numbering

Progress numbering is operator information only.

Examples:

```text
17/3000
1847/3000
```

It may represent physical source row or logical non-empty record number.

The implementation should choose whichever produces the clearest output.

It must not become part of durable accounting identity.

---

## 14. Existing versus new detection

The importer already uses stable markers to identify previously imported records.

`v0.5.2` should expose that distinction visibly.

Rows should normally classify as:

```text
existing
new
update
error
deferred
```

The stable marker itself remains an implementation detail and should not clutter normal progress output.

---

## 15. Output timing

The command must display progress as work completes.

It must not accumulate all normal row output and print it only after the entire import finishes.

The implementation may use callbacks, progress events, an iterator, or another simple mechanism.

The observable design requirement is:

> Completed work becomes visible promptly.

This is not fundamentally a stdout buffering problem.

The current problem is that results are not produced until the importer finishes.

---

## 16. Performance

`v0.5.2` is not a performance-optimisation release.

Do not introduce concurrent Invoice Ninja writes merely to make the import faster.

Current throughput is acceptable if progress is visible.

Sequential processing remains preferred because it is:

- understandable;
- deterministic;
- easier to recover;
- easier to diagnose;
- safer for accounting mutations.

---

## 17. Preview behaviour

Batching and progress must work in preview mode as well as commit mode.

Example:

```bash
GoTradie ninja import expenses expenses.csv \
    --receipts-root receipts \
    --batch-size 100
```

Possible output:

```text
WOULD-CREATE   1/3000  ...
EXISTING       2/3000  ...
WOULD-UPDATE   3/3000  ...
ERROR          4/3000  ...
```

Preview must continue to perform no persistent Invoice Ninja writes.

---

## 18. CLI compatibility

Existing usage remains valid:

```bash
GoTradie ninja import expenses <file|-> [--receipts-root <dir>] [--commit]
```

Proposed `v0.5.2` syntax:

```bash
GoTradie ninja import expenses <file|-> \
    [--receipts-root <dir>] \
    [--batch-size <n>] \
    [--pause] \
    [--commit]
```

No new persistent-write flag is introduced.

`--commit` remains the only switch permitting writes.

---

## 19. Failure behaviour

Errors should favour safe continuation where doing so cannot corrupt accounting meaning.

A malformed ordinary Expense row may be reported while later independent rows continue.

A failure involving supplier-account settlement may require a batch or import to stop if continuing would make settlement indeterminate.

The importer must distinguish:

```text
row failed but later rows remain independent
```

from:

```text
continuing would make settlement indeterminate
```

The latter must fail safely.

---

## 20. Non-goals

`v0.5.2` does not:

- redesign the Expense bookkeeping model;
- create a general accounting engine;
- populate Invoice Ninja Transactions for every paid Expense;
- implement bank-statement import;
- add concurrent Expense writes;
- add a persistent import database;
- create a resume/checkpoint file;
- redesign supplier FIFO settlement;
- solve foreign-currency supplier-account settlement;
- define Adjustment accounting;
- add a general shared-document subsystem;
- infer identity from CSV position.

---

## 21. Acceptance criteria

`v0.5.2` Expense importing is ready to close when:

1. A large CSV can be imported while useful progress appears continuously.
2. Successful rows show a concise state/date/supplier/amount/receipt summary.
3. Existing records are visibly distinguishable from newly created records.
4. Errors receive expanded useful context.
5. A final import summary is produced.
6. `--batch-size N` can process ordinary historical purchase rows in practical chunks.
7. Restarting the original import after interruption does not intentionally duplicate already-imported Expenses.
8. CSV row position is not required for durable record identity or reconstruction.
9. Repeated line items using one receipt do not normally upload multiple identical physical receipt files.
10. Receipt ownership remains discoverable after restart/rerun.
11. Supplier-account batching does not weaken any `v0.5.1` settlement invariant.
12. Preview remains write-free.
13. `--commit` remains the sole persistent-write switch.
14. Existing non-batched command usage continues to work.

---

## 22. Design principle

The intended operator experience is:

```text
Start import.

Immediately see what GoTradie is doing.

Watch each item resolve as NEW, EXISTING or ERROR.

Let it work through manageable batches.

If it stops, fix the problem and run the same source again.

Do not maintain a second ledger.
Do not manually remember where it stopped.
Do not wonder for five minutes whether it has died.
```

That is the point of `v0.5.2`.
