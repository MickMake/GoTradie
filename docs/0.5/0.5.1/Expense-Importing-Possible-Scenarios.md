# Expense Importing — Possible Future Scenarios

Status: **Non-blocking design notes / possible future work**

## Purpose

This document records edge cases and limitations discovered while implementing and reviewing historical Expense import and supplier-account settlement.

These are **not current requirements** and must not be treated as blockers for the existing expense-import workflow.

They exist here so that, if one of these scenarios becomes real later, the reasoning is not rediscovered from old code reviews, chats, or archaeological excavation.

The current priority remains deliberately narrow:

> Import the historical Expenses correctly into Invoice Ninja so that the accounting data can be used for BAS and EOFY work.

Do not expand these notes into new architecture unless the corresponding real-world scenario actually occurs.

---

## Current normal workflow

The normal MickMake purchase workflow is:

1. A purchase is made.
2. It is paid at the same time.
3. The spreadsheet records the source purchase values.
4. Where the source currency is foreign, the spreadsheet converts the business-use accounting values into AUD.
5. GoTradie imports:
   - `Business Amount` as the Invoice Ninja Expense amount;
   - `Business GST` as the Invoice Ninja Expense tax amount.
6. The Expense is marked paid using the truthful payment date and Payment Type.

For example, a USD purchase may contain:

```text
Total Inc GST:   USD 5.00
Currency:        USD
Business Amount: AUD 6.42
Business GST:    AUD 0.58
```

The Invoice Ninja Expense uses the converted business values.

GoTradie does **not** currently perform foreign-exchange conversion itself. The import source is expected to provide the accounting values already converted where required.

This is intentional and is sufficient for the current workflow.

---

## Scenario 1 — Foreign-currency supplier-account settlement

### Status

**Possible future issue. Not part of the current workflow.**

### Current behaviour

Supplier-account settlement uses purchase gross values derived from:

```text
Total Inc GST
```

or, when reconstructing from Invoice Ninja:

```text
Source total inc GST
```

Account Payment Bank Transactions, however, are created using the Invoice Ninja company's currency.

This is fine when the purchase and company currency are the same.

It becomes ambiguous when all of the following are true:

1. the purchase is in a foreign currency;
2. the purchase is not paid immediately;
3. it participates in supplier-account settlement;
4. the later Account Payment Transaction is denominated in company currency.

For example:

```text
Purchase source total:        USD 100
Business/accounting currency: AUD
Later supplier payment:       AUD 150
```

The settlement engine must not compare `100` source-currency units directly with `150` company-currency units.

### Why `Business Amount` is not automatically the answer

`Business Amount` also applies `Business %`.

For example:

```text
Supplier invoice:  AUD 100
Business use:      50%
Business Amount:   AUD 50
```

The supplier is still owed AUD 100.

Therefore supplier settlement requires the **full supplier liability expressed in company currency before Business % is applied**.

### Possible future fix

If this scenario becomes real, introduce a durable value representing:

```text
Full purchase total in company currency
```

This could be supplied by the import source or derived by some deliberately designed mechanism.

Do not simply use `Business Amount`.

Do not add live FX services, currency conversion frameworks, or other machinery merely because they are theoretically possible.

### Trigger for fixing

Fix this only when GoTradie genuinely needs to settle a foreign-currency supplier account.

Ordinary immediately-paid foreign-currency Expenses are not affected.

---

## Scenario 2 — Multi-stage commit has no remote transaction rollback

### Status

**Known implementation characteristic. Acceptable for the current importer.**

### Current behaviour

A committed Expense import operates in stages:

1. create/update purchase Expenses;
2. create supplier Account Payment Bank Transactions;
3. reconcile derived Paid/Unpaid Expense state.

Invoice Ninja does not provide a transaction spanning all of these API operations.

If a later operation fails, earlier successful remote writes remain committed.

For example:

```text
Expenses 1–37 created successfully
Expense 38 fails
```

Expenses 1–37 remain in Invoice Ninja.

### Current recovery model

The importer is deliberately idempotent.

The expected recovery procedure is:

1. correct the cause of the failure;
2. run the same import again;
3. already-imported records are recognised by their stable GoTradie identities;
4. processing continues without intentionally duplicating them.

This is preferable to implementing a GoTradie-side transaction log or rollback database, which would create another durable source of truth.

### Possible future improvement

If real failures make recovery difficult, possible improvements include:

- clearer CLI guidance telling the user to correct the error and rerun;
- improved preflight checks to catch more failures before the first remote write;
- better reporting of which stages completed.

Do not introduce a persistent GoTradie transaction journal, SQLite database, or allocation ledger merely to simulate rollback.

### Trigger for fixing

Only revisit this if actual imports demonstrate that idempotent rerun is insufficient.

---

## Scenario 3 — Older CSV files without current classification columns

### Status

**Possible historical compatibility issue. Not a current problem.**

### Current behaviour

The Expense importer requires, amongst other fields:

```text
Document Type
Payment Type
```

These columns are present in the current master spreadsheet and carry meaningful bookkeeping information.

Older exported spreadsheets may pre-date these columns.

Such files will fail validation rather than having GoTradie guess their meaning.

### Possible future fix

If an older historical file genuinely needs to be imported, consider either:

- a one-off migration/preprocessing step that adds the required fields; or
- a deliberately defined legacy import mode.

Do not silently infer `Document Type` or `Payment Type` where doing so could alter accounting meaning.

### Trigger for fixing

Only implement compatibility when an actual historical source file requires it.

---

## Scenario 4 — Account Payment supporting documents

### Status

**Minor possible future enhancement.**

### Current behaviour

For an Account Payment row, GoTradie can preserve the source filename in the Bank Transaction description.

The current importer does not attach the supporting document to the Bank Transaction itself.

This does not affect settlement correctness.

### Possible future fix

If retaining supplier-account payment documents directly against Bank Transactions becomes useful, investigate whether the Invoice Ninja API provides a suitable durable attachment mechanism.

Do not invent a second GoTradie document store.

---

## Explicitly not future problems

The following were reviewed and should not be reopened merely because they look unusual.

### Earlier unresolved purchase blocks later settlement

This is deliberate.

GoTradie must not skip an older unresolved supplier-account purchase merely to make a later payment balance.

Explicit failure is safer than silently manufacturing reconciliation.

### Archived or deleted marked records

This is now deliberately handled.

If a GoTradie-marked supplier Expense or Bank Transaction is archived or deleted, settlement reconstruction stops and requires the record to be restored or explicitly resolved.

It must not silently disappear from history.

### Invoice Ninja native Expense-to-Transaction links

These remain unsuitable as the correctness mechanism for partial supplier settlement.

No change is required.

### GoTradie side ledger

Still rejected.

No SQLite database, persistent allocation cache, or second bookkeeping system should be introduced to solve these scenarios.

---

## Decision rule

Before implementing anything in this document, ask:

> Is this a real scenario we now have to support, or merely an interesting scenario we can imagine?

If it is merely imaginable, leave it here.

The existence of an edge case does not create an obligation to build a cathedral around it.
