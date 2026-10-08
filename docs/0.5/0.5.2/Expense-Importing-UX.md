# Expense Importing — Identity, Preflight, Operational UX and Batching

Status: **Draft design for v0.5.2**  
Software version: `v0.5.2`

## Purpose

`v0.5.1` established the accounting model and durable Invoice Ninja representation for historical Expense import and supplier-account settlement.

`v0.5.2` does not redesign that model.

Its purpose is to make the importer practical and safe against the real historical dataset, including CSV files containing thousands of purchase line items.

The main problems addressed are:

- durable identity must not depend on CSV row position;
- the source should be validated before any import execution begins;
- large imports need visible live progress;
- ordinary imports need practical batching;
- supplier-account settlement must retain whole-file accounting visibility;
- repeated Expense rows belonging to the same physical receipt should not upload duplicate receipt documents.

The guiding rule is:

> A historical Expense import should fail early when the source is unsafe, visibly make progress when it runs, and be safely repeatable without maintaining a second ledger.

---

## Relationship to v0.5.1

The accepted `v0.5.1` Expense-import architecture remains authoritative.

In particular, `v0.5.2` must not change these rules:

- an Expense represents what was purchased;
- a supplier Account Payment represents settlement and may create a Bank Transaction;
- ordinary paid Expenses do not require synthetic Bank Transactions;
- Invoice Ninja remains the durable source of truth after import;
- GoTradie must not introduce a persistent side ledger, SQLite database, checkpoint database, or allocation cache;
- supplier settlement remains deterministic and fail-safe;
- preview remains the default;
- `--commit` remains the only persistent-write flag;
- rerunning the same logical source record must remain idempotent.

`v0.5.2` deliberately replaces the previous row-derived occurrence identity with an explicit source identifier supplied by the import file.

---

# 1. Mandatory `Import ID`

Every non-empty source row must contain a non-blank:

```text
Import ID
```

`Import ID` is the durable identity of the logical source record.

It applies to all supported Expense-import source row types, including:

- Invoice;
- Receipt;
- Account Payment;
- Adjustment or other supported/deferred row types.

The same column is used for all rows to reduce the chance of a manually maintained sheet assigning the wrong kind of identifier.

## Contract

An `Import ID`:

- is supplied by the user/source file;
- must be unique within the complete import file;
- must remain unchanged when the logical source record is corrected and re-imported;
- must not be derived from CSV row number, file position, current sort order, or batch position;
- may be opaque;
- does not need to encode accounting meaning;
- must be persisted durably into Invoice Ninja so a fresh GoTradie process can recover it without the original spreadsheet.

Examples of valid source values could include:

```text
000471
EXP-2020-00471
9f38a72c
```

GoTradie does not need to understand how the value was created.

The important semantics are:

```text
same Import ID
    = same logical source record

different Import ID
    = different logical source record
```

If a correction changes Date, Supplier, amount, classification or other mutable source values but retains the same `Import ID`, GoTradie must treat it as a correction/update to the same logical record, subject to the normal accounting safety rules.

## GoTradie must not generate missing IDs

If `Import ID` is missing, blank or duplicated, GoTradie must not invent an identifier.

The operator must correct the source file.

This intentionally moves source-record identity responsibility to the import source and removes occurrence-number heuristics from GoTradie.

---

# 2. Whole-file local preflight

Before import execution, GoTradie must read and inspect the complete named input file.

This happens irrespective of:

- preview or commit mode;
- `--batch-size`;
- the number of rows;
- whether Account Payment rows are present.

At minimum, local preflight must establish:

1. the required CSV structure exists;
2. `Import ID` column exists;
3. every non-empty row has a non-blank `Import ID`;
4. every `Import ID` is unique within the complete file;
5. required values can be parsed;
6. row/document types can be classified;
7. source arithmetic can be sanity-checked;
8. whether any Account Payment rows are present;
9. the execution mode to use after preflight.

No Invoice Ninja mutation may occur until preflight has completed successfully.

Prefer completing file-only validation before remote API work begins.

---

# 3. Expense import requires a named file

Expense import no longer accepts stdin.

Accepted shape:

```bash
GoTradie ninja import expenses expenses.csv [options]
```

Rejected:

```bash
cat expenses.csv | GoTradie ninja import expenses -
GoTradie ninja import expenses -
```

The reason is deliberate: the importer requires a complete source file as one preflight unit before execution planning begins.

Other unrelated GoTradie CSV import commands may continue to support stdin.

A stdin attempt should fail clearly, for example:

```text
Expense import requires a named file so the complete source can be preflighted before processing.
stdin is not supported.
```

---

# 4. Preflight errors

Preflight errors stop the complete import.

If any fatal preflight error exists:

- no Expense is created or updated;
- no vendor/category/project is created;
- no Bank Transaction is created;
- no receipt is uploaded;
- no persistent Invoice Ninja write occurs.

Examples include:

```text
ERROR: missing required column "Import ID"
ERROR: row 1847 has no Import ID
ERROR: duplicate Import ID "000471" at rows 217 and 884
ERROR: row 92 contains an invalid Date
ERROR: row 103 contains an invalid numeric amount
```

Structural, identity and unparseable-data failures are errors rather than warnings.

There is no `--force` escape hatch for identity/preflight errors.

---

# 5. Preflight warnings and source sanity checks

The complete-file scan should also report suspicious source data that is parseable but may be wrong.

Warnings do not silently alter the input.

The initial checks should include, where the relevant operands exist and the comparison is meaningful:

## Source totals

```text
Total Ex GST + GST ~= Total Inc GST
```

Use integer cents or an explicit small rounding tolerance.

## Quantity and unit price

Where Quantity and Unit Price are present:

```text
Quantity x Unit Price ~= expected source line total
```

This is normally a warning rather than a fatal error because legitimate differences may occur through:

- discounts;
- freight;
- supplier rounding;
- tax-inclusive versus tax-exclusive unit pricing;
- source-sheet representation choices.

GoTradie must not "fix" the source total automatically.

## Business-use arithmetic

Where the fields are populated:

```text
Business Amount ~= Total Inc GST x Business % / 100
Business GST    ~= GST x Business % / 100
```

Also flag obviously suspicious relationships such as:

- Business % outside `0..100`;
- Business Amount materially exceeding the source gross amount where that cannot be valid;
- Business GST materially exceeding source GST where that cannot be valid.

An out-of-range or impossible value may be promoted to an error when GoTradie cannot interpret it safely.

## Duplicate-looking records with different IDs

If two rows have different `Import ID` values but are otherwise exact or near-exact duplicates across meaningful source fields, report a warning.

This is not automatically an error: two genuine purchases may legitimately look identical.

The purpose is to surface accidental duplicate IDs or copied rows for operator review.

## Receipt/source-file oddities

Where receipt files are supplied, useful warnings may include:

- the same source filename appearing against unrelated suppliers/dates;
- multiple different filenames resolving to identical receipt content;
- other obvious inconsistencies discovered cheaply during the complete-file scan.

The preflight should favour useful, explainable checks over a sprawling accounting-lint framework.

---

# 6. Preflight result

Before execution begins, GoTradie should show a concise summary.

Example:

```text
Preflight complete

Rows:       3042
Errors:        0
Warnings:     17
Mode:       batched
```

If Account Payment rows are present:

```text
Preflight complete

Rows:       3042
Errors:        0
Warnings:     17
Mode:       settlement-safe whole-file
```

Warnings should identify the source row and `Import ID` where practical.

If Errors is greater than zero, the command stops before execution.

---

# 7. CSV row numbers are diagnostic only

CSV row position may be displayed for diagnostics:

```text
CSV row 1847
1847/3000
```

It must never determine:

- durable Expense identity;
- duplicate identity;
- stable occurrence identity;
- supplier-account identity;
- settlement allocation;
- reconciliation order;
- whether a later run recognises an existing record.

`GoTradie source row` may remain temporarily as historical/debugging metadata for records already imported by `v0.5.1`, but it must not be required for `v0.5.2` accounting correctness or identity.

New durable matching is based on `Import ID`.

---

# 8. Existing Invoice Ninja identity validation

GoTradie must recover previously imported `Import ID` values from Invoice Ninja.

If one source `Import ID` resolves to more than one durable Invoice Ninja record of the same logical import domain, the state is ambiguous and the import must fail safely.

Example:

```text
ERROR: Import ID "000471" matches multiple existing Invoice Ninja Expenses
```

GoTradie must not choose an arbitrary winner.

The exact marker/storage encoding is an implementation detail, but the original `Import ID` value must remain durably recoverable or exactly matchable from Invoice Ninja state.

---

# 9. Re-entrant behaviour

If an import stops after some work has successfully committed, the supported recovery action is:

```text
Run the original import again.
```

Already imported records resolve through `Import ID`.

No persistent GoTradie resume database is required.

No `.state` file is required.

No checkpoint ledger is required.

Invoice Ninja and durable `Import ID` metadata remain sufficient to determine what has already been imported.

---

# 10. Live import progress

Once execution begins, useful progress must appear while the importer is running.

Normal successful-row output should be concise.

Example:

```text
NEW        17/3000  30/09/2020  IFTTT                         $5.61  000017
EXISTING   18/3000  30/10/2020  IFTTT                         $5.66  000018
NEW        19/3000  24/11/2020  BackBlaze                    $13.61  000019
```

Useful fields are:

```text
State
Progress
Date
Supplier
Business Amount
Import ID
```

Receipt filename may also be shown where useful.

Exact widths and formatting are implementation details.

The objective is that one glance answers:

> Is it still running, what is it currently doing, and is it creating something new?

---

# 11. Import result state

At minimum:

```text
NEW
EXISTING
ERROR
```

Additional truthful states may include:

```text
UPDATED
DEFERRED
```

Preview equivalents:

```text
WOULD-CREATE
EXISTING
WOULD-UPDATE
ERROR
```

Preview output must never imply that a write occurred.

Successful rows should not print the complete internal mutation list unless a future verbose mode explicitly asks for it.

---

# 12. Error reporting during execution

Execution errors should be visually distinct and more verbose than successful rows.

Example:

```text
ERROR      127/3000  14/07/2024  Bunnings - Dural             $129.00  000127
           File: Bunnings-12345.pdf
           Category: Foo
           Error: unknown Invoice Ninja Expense Category "Foo"
```

An error should identify at least:

- source row position for diagnostics;
- `Import ID`;
- Date;
- Supplier;
- Business Amount;
- File Name where present;
- the actual error.

---

# 13. End-of-import summary

Each completed execution should finish with a concise summary.

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

For true batched execution, a short summary may also be displayed after each batch.

```text
Batch 4 complete: 98 new, 2 existing, 0 errors
```

---

# 14. Batched execution for ordinary Expense-only files

`--batch-size N` is an execution convenience.

Example:

```bash
GoTradie ninja import expenses expenses.csv \
    --receipts-root receipts \
    --batch-size 100 \
    --commit
```

The complete source file is still preflighted first.

Only after successful preflight does execution operate in batches.

Conceptually:

```text
read and preflight complete file

process next N ordinary Expense rows
report batch

process next N ordinary Expense rows
report batch

repeat until EOF
```

Batching must not create durable batch state or change accounting identity.

A batch size around `100` is expected to be practical but remains configurable.

---

# 15. Optional pause between true batches

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

`--pause` creates no persistent state.

Its exact behaviour when the file is forced into settlement-safe whole-file mode must be explicit and must not imply that supplier settlement has been isolated into independent accounting batches.

---

# 16. Account Payments force settlement-safe whole-file mode

This is a fixed `v0.5.2` rule.

If the complete-file preflight detects any Account Payment row:

> The importer must use the existing settlement-safe whole-file planning/visibility model.

`--batch-size` must not reduce the purchase/payment history visible to supplier-account settlement.

The command should say so clearly, for example:

```text
Account Payment rows detected.
Using settlement-safe whole-file mode.
```

The importer may and should still emit live per-row progress while the whole-file settlement-safe plan executes.

True batch slicing applies only to ordinary Expense-only imports in `v0.5.2`.

This keeps supplier FIFO settlement deterministic and avoids turning execution batching into an accounting rule.

---

# 17. Receipt deduplication

A single physical receipt may legitimately support multiple Expense line items.

GoTradie should normally upload that physical receipt only once.

All related Expenses retain the original source filename as provenance.

## Receipt key

The durable receipt identity should be separate from the display/source filename.

Where the receipt file is available, prefer a deterministic content-derived key for the physical document.

This allows:

- the same physical receipt under a renamed file to remain the same receipt;
- two different files with the same filename to remain distinguishable if their contents differ.

The source filename remains stored independently.

The exact hash/encoding is an implementation detail.

---

# 18. Durable receipt ownership

Every Expense associated with the same physical receipt carries a shared durable receipt marker, conceptually:

```text
[GoTradie receipt:v1:<receipt-key>]
```

Exactly one Expense additionally carries the owner marker:

```text
[GoTradie receipt-owner:v1]
```

and that Expense owns the actual Invoice Ninja document attachment.

Sibling Expenses do not need to store a pointer to the owner's Invoice Ninja object ID.

The durable invariant is:

```text
one receipt key
    -> zero owners before upload
    -> exactly one owner after successful commit
    -> more than one owner = ERROR
```

If an owner marker exists but the expected receipt document is absent:

```text
ERROR: durable receipt state inconsistent
```

Do not silently invent a new owner.

Receipt ownership must remain discoverable across:

- batches;
- interrupted imports;
- reruns;
- manually sliced historical source files.

---

# 19. Existing versus new detection

`Import ID` is the authoritative record identity.

Rows should normally classify as:

```text
existing
new
update
error
deferred
```

Derived fingerprints may still be used for sanity checking or drift diagnostics, but they are not the durable answer to:

> Is this the same logical source record?

That question is answered by `Import ID`.

---

# 20. Output timing

Completed work must become visible promptly.

The implementation may use callbacks, progress events, an iterator or another simple mechanism.

The importer must not accumulate all normal row output and display it only after the complete import finishes.

This is not fundamentally a stdout buffering problem.

---

# 21. Performance

`v0.5.2` is not a performance-optimisation release.

Do not introduce concurrent Invoice Ninja writes merely to make the import faster.

Sequential processing remains preferred because it is:

- understandable;
- deterministic;
- easier to recover;
- easier to diagnose;
- safer for accounting mutations.

Whole-file preflight is expected to be cheap compared with remote Invoice Ninja operations.

---

# 22. Preview behaviour

Whole-file preflight, warnings, progress and execution-mode selection apply in preview as well as commit mode.

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

Preview performs no persistent Invoice Ninja writes.

---

# 23. CLI contract

Existing `v0.5.1` form:

```text
GoTradie ninja import expenses <file|-> [--receipts-root <dir>] [--commit]
```

`v0.5.2` form:

```text
GoTradie ninja import expenses <file> \
    [--receipts-root <dir>] \
    [--batch-size <n>] \
    [--pause] \
    [--commit]
```

Changes are deliberate:

- `<file>` must be a named file;
- stdin (`-`) is not accepted for Expense import;
- `Import ID` is a required CSV column;
- `--batch-size` controls execution batching only;
- `--pause` does not create durable state;
- `--commit` remains the only persistent-write switch.

---

# 24. Failure behaviour

There are two broad failure phases.

## Preflight failure

Any fatal local preflight error stops the entire import before execution.

This includes identity/schema failures such as missing or duplicate `Import ID`.

## Execution failure

Once execution begins, errors should favour safe continuation where doing so cannot corrupt accounting meaning.

A malformed or failed ordinary Expense row may be reported while later independent rows continue where safe.

A failure involving supplier-account settlement may require the whole execution to stop if continuing would make settlement indeterminate.

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

# 25. Non-goals

`v0.5.2` does not:

- redesign the Expense bookkeeping model;
- create a general accounting engine;
- generate missing `Import ID` values;
- infer identity from CSV row position;
- maintain occurrence counters as durable identity;
- automatically repair suspicious source arithmetic;
- populate Invoice Ninja Transactions for every paid Expense;
- implement bank-statement import;
- add concurrent Expense writes;
- add a persistent import database;
- create a resume/checkpoint file;
- redesign supplier FIFO settlement;
- solve foreign-currency supplier-account settlement;
- define Adjustment accounting;
- add a general shared-document subsystem.

---

# 26. Acceptance criteria

`v0.5.2` Expense importing is ready to close when:

1. Expense import requires a named source file and refuses stdin.
2. `Import ID` is a required column.
3. The complete file is scanned before execution.
4. Any blank or duplicate `Import ID` stops the import before persistent writes.
5. `Import ID` is durably recoverable from Invoice Ninja and is the authoritative logical source-record identity.
6. CSV row position is not required for durable identity or reconstruction.
7. Useful accounting/source sanity checks are reported during preflight without silently changing source values.
8. Fatal preflight errors and non-fatal warnings are clearly distinguished.
9. A large CSV can be imported while useful progress appears continuously.
10. Successful rows show a concise state/date/supplier/amount/identity summary.
11. Existing records are visibly distinguishable from newly created records.
12. Errors receive expanded useful context.
13. A final import summary is produced.
14. `--batch-size N` can process ordinary Expense-only historical rows in practical chunks after successful whole-file preflight.
15. Any file containing Account Payment rows uses settlement-safe whole-file accounting visibility.
16. Supplier-account processing does not weaken any `v0.5.1` settlement invariant.
17. Restarting the original import after interruption does not intentionally duplicate already-imported records.
18. Repeated line items using one physical receipt do not normally upload multiple identical receipt files.
19. Receipt ownership remains discoverable and internally consistent after restart/rerun.
20. Preview remains write-free.
21. `--commit` remains the sole persistent-write switch.

---

# 27. Design principle

The intended operator experience is:

```text
Give GoTradie one complete named source file.

Validate all identities first.
Point out suspicious arithmetic before touching the books.
Stop completely if the source is structurally unsafe.

Then run.

Immediately show what GoTradie is doing.
Batch ordinary Expenses where useful.
Keep supplier settlement whole-file safe.
If execution stops, fix the problem and run the same source again.

Do not maintain a second ledger.
Do not use row position as identity.
Do not guess missing IDs.
Do not wonder for five minutes whether it has died.
```

That is the point of `v0.5.2`.
