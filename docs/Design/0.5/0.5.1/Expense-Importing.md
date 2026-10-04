# Historical Expense Importing and Supplier-Account Settlement

Status: **Closed — accepted architecture and bookkeeping invariant**  
Software version: `v0.5.1`

## Purpose and authority

This document defines the durable Invoice Ninja bookkeeping model used by GoTradie after historical import. It supersedes any earlier design which requires the original spreadsheet, an in-memory import allocation result, or a separate GoTradie ledger to reconstruct supplier-account settlement.

The words **must**, **must not**, **may**, and **should** are deliberate. The current historical importer is complete when its import-specific behaviour is correct. The Invoice-Ninja-only integrity test later in this document is a target architecture and post-migration integrity test; it does not block completion of the current import work.

The central rules are:

> An Invoice Ninja Expense records what was purchased. An Invoice Ninja Bank Transaction records money paid to settle a supplier account. A settlement is not another purchase.

> After a successful import, Invoice Ninja is the durable source of truth. The spreadsheet is an import source, not an operational ledger.

## System of record

The source spreadsheet is migration input. It may be corrected and re-imported while migration is being validated, but it must not remain a runtime dependency.

After successful import:

- Invoice Ninja Expenses are the durable purchase records;
- marked Invoice Ninja Bank Transactions/Transactions are the durable supplier-settlement records;
- stable GoTradie identities and account markers stored in Invoice Ninja make both record types deterministic and idempotent;
- the spreadsheet may be archived;
- GoTradie must not require the spreadsheet to produce supplier reconciliation, BAS, or EOFY output;
- GoTradie must not maintain a separate persistent cache, SQLite database, allocation ledger, or side database.

The target architecture is not complete until GoTradie can reproduce the required accounting result from Invoice Ninja alone. This is a post-migration integrity goal and does not block completion of the current historical import work.

## Purchase records: Invoice Ninja Expenses

One source purchase creates one Invoice Ninja Expense.

In `v0.5.1`, `Invoice` and `Receipt` are imported as purchase-side records. `Adjustment` is recognised but deliberately deferred rather than being given invented accounting behaviour. A future version may define Adjustment handling if a real requirement exists.

The Expense owns the analytical and tax meaning of the purchase, including:

- purchase date;
- Supplier and Store context;
- Job Number and Child Job Number;
- Expense Category;
- analytical Option / Tax Detail;
- Tax Treatment;
- Business %;
- BAS Treatment;
- item number and description;
- quantity, unit, and unit price;
- source ex-GST, GST, and inc-GST totals;
- business amount and business GST;
- source currency and notes;
- supplier invoice/reference number;
- receipt or source-document identity and attachment;
- stable GoTradie source identity; and, for Expenses participating in supplier-account settlement, supplier-account identity.

Current mapping terminology is preserved:

| Source concept | Invoice Ninja representation |
|---|---|
| Supplier + Store | Vendor display name may use `Supplier - Store` when Store is present |
| Date | Expense date |
| Category | Expense Category |
| Option | Expense custom analytical field / Tax Detail |
| Tax Treatment | Expense custom field |
| Business % | Expense custom field |
| BAS Treatment | Expense custom field |
| Business Amount | Expense amount |
| Business GST | Expense tax amount |
| Invoice Number / purchase reference | Transaction Reference |
| Store, job fields, item detail, source totals, notes, source file | Private Notes or equivalent durable fields |
| Receipt filename | Private attached document |
| Card Holder | Not currently imported in v0.5.1 |

Job Number and Child Job Number must remain on the purchase record. A numeric Job Number may additionally resolve to an Invoice Ninja Project, but project linkage must not be the only durable copy.

Receipt uploads remain private (`is_public=false`). `Should be Invoiced` remains OFF/untouched. `Add Documents to Invoice` remains OFF.

An immediately paid purchase remains an Expense and may be marked Paid using its truthful purchase/payment date and Invoice Ninja Payment Type. It must not be duplicated merely to manufacture settlement history.

### Foreign-currency purchase values

GoTradie `v0.5.1` does not perform foreign-exchange conversion.

Where a source purchase is denominated in a foreign currency, the import source is expected to provide the already-converted company-currency accounting values in:

- `Business Amount`;
- `Business GST`.

These values become the Invoice Ninja Expense amount and tax amount.

`Total Inc GST`, source GST totals, and `$ Currency` remain source metadata and identity inputs.

Foreign-currency supplier-account settlement is a separate deferred scenario documented in [Expense-Importing-Possible-Scenarios.md](./Expense-Importing-Possible-Scenarios.md).

## Supplier-account payments: Invoice Ninja Bank Transactions

An `Account Payment` represents money leaving the business to settle a supplier trade account. On committed import it must become a durable Invoice Ninja Bank Transaction/Transaction with withdrawal direction.

The Transaction owns settlement facts, including:

- payment date;
- supplier-account identity;
- gross amount paid;
- withdrawal direction;
- bank/payment reference;
- Payment Type where supported and truthful;
- stable GoTradie payment identity and marker;
- source-document identity or attachment where supported.

An account-payment row must not create or overwrite purchase analytics such as Category, Job Number, Tax Treatment, Business %, BAS Treatment, item detail, or purchase GST.

An account payment must never create:

- another Expense;
- another GST-bearing purchase;
- an Invoice Ninja customer Payment;
- a GoTradie-only durable allocation record.

Only Transactions carrying the stable GoTradie supplier-settlement marker may participate in automatic supplier-account settlement. GoTradie must not guess that every unrelated withdrawal to a vaguely similar name is an account payment.

### v0.5.1 settlement safety prerequisites

Supplier `Account Payment` import is deliberately guarded.

When settlement records are present, GoTradie requires:

- exactly one active Invoice Ninja manual bank account named `GoTradie`;
- that account must not be remote-backed;
- auto-sync must be disabled;
- the current Invoice Ninja company must expose a usable default currency;
- no active auto-convert DEBIT bank rule may be able to turn the imported withdrawal into another Expense;
- vendor-paid notifications must be disabled before GoTradie performs payment-state writes.

These are safety prerequisites, not a request for GoTradie to create or manage a banking integration.

## Invoice Ninja Payments are customer receipts

Invoice Ninja Payments are accounts-receivable records: money received from customers and applied to customer invoices or credits.

They must not be used for:

- supplier payments;
- Bunnings trade-account settlements;
- expense payment allocations;
- bank withdrawals;
- any other accounts-payable event.

The shared word “payment” does not make these objects interchangeable. One is money in; the other is money out. Treating them as cousins because they share a surname is how accounting goblins obtain tenure.

## Supplier-account identity

Settlement is keyed by a durable GoTradie supplier-account identity, not fuzzy vendor-name matching.

This matters because purchase display names may include Store:

```text
Bunnings - Dural
Bunnings - Online
```

Both may belong to the same `Bunnings` trade account. Store remains purchase metadata and must not split a supplier account accidentally.

The durable marker on each participating Expense and Transaction must allow GoTradie to recover the common supplier account without consulting the spreadsheet. Ordinary immediately paid Expenses that do not participate in supplier-account settlement do not require a supplier-account marker. Marker matching must be exact and versioned so later formatting changes do not silently alter identity.

## Deterministic settlement reconstruction

GoTradie must reconstruct supplier-account settlement from durable Invoice Ninja Expenses and marked supplier Transactions.

For each supplier account:

1. Load eligible Expenses and marked withdrawal Transactions from Invoice Ninja.
2. Order Transactions by payment date and stable GoTradie payment identity.
3. For each Transaction, consider purchases dated on or before its payment date.
4. Allocate in FIFO order: oldest purchase date first.
5. For purchases on the same date, use stable GoTradie purchase identity as the tie-breaker.
6. Calculate in integer minor currency units; do not allocate using binary floating-point arithmetic.
7. Allocate no more than the Transaction remainder or Expense outstanding amount.
8. Carry a partial allocation forward to later Transactions without marking the Expense fully paid.
9. Report unapplied Transaction amounts and unresolved or ambiguous records explicitly.
10. Never skip an older eligible unresolved Expense merely to make a later Expense appear settled.

The same Invoice Ninja state must always produce the same allocation result.

## Native expense-to-transaction linking

Invoice Ninja's native Expense-to-Transaction linking is not adequate as the correctness mechanism for partial expense allocations.

Therefore:

- GoTradie must not require a native partial link;
- absence of a native link must not prevent deterministic settlement;
- a native link, if present, may be treated as UI assistance but not as the authoritative allocation ledger;
- BAS, EOFY, paid-state, and reconciliation logic must be reproducible from Expenses, marked Transactions, and stable GoTradie identities.

## Paid and Unpaid state

For supplier-account-managed Expenses:

- **Unpaid** means not fully settled;
- **Paid** means fully settled.

An Expense may remain shown as Unpaid while one or more partial payments have been economically allocated to it. That intermediate UI state is acceptable and must not cause the partial settlement to be ignored in BAS or reconciliation calculations.

When FIFO reconstruction shows that an Expense is fully settled, GoTradie may mark it Paid and record the truthful final settlement date for tidy Invoice Ninja state. This update is a convenience derived from the durable records; it is not the source of the allocation truth.

If later correction of an Expense or Transaction changes the deterministic result, GoTradie must report the changed conclusion rather than preserving a stale GoTradie-side allocation.

Historical import and any Paid/Unpaid or payment-date changes made by GoTradie must not trigger vendor-facing emails or notifications.

## BAS and EOFY invariants

Expenses represent purchases. Supplier Transactions represent settlement only. Exports must not count both as purchases.

### Cash GST accounting

For cash-basis GST:

- actual payment Transaction dates determine timing for supplier-account purchases;
- actual allocated amounts determine the proportion treated as paid in each period;
- the GST and business-use attributes come from the underlying Expense;
- proportional GST must be derived from the Expense and allocated payment, with deterministic rounding and cumulative reconciliation to the Expense total;
- the Transaction itself contributes no second GST purchase amount.

An Expense remaining Unpaid in the Invoice Ninja UI does not erase a genuine partial payment from the BAS period in which it occurred.

### Non-cash GST accounting

For non-cash accounting, the purchase/invoice date and Expense tax treatment drive the GST event. Later supplier Transactions settle the liability and must not create another GST event.

### EOFY and reconciliation

EOFY and reconciliation output must preserve the same separation:

```text
Expenses      = what was purchased
Transactions  = when and how the supplier account was settled
```

Supplier-account reporting should support:

```text
opening balance
+ purchases
- settlement Transactions
= calculated closing balance
```

Outstanding Expenses, partially settled Expenses, unapplied Transactions, and ambiguous markers must be reported rather than silently forced into balance.

## Canonical examples

### Bunnings trade account

Several Bunnings purchases create separate Expenses because each purchase owns its category, job, GST, business percentage, receipt, and source identity:

```text
Expenses
$120
$85
$340
$62
$190
```

The later account payment creates one marked withdrawal Transaction:

```text
Transaction
Bunnings account payment    $797
```

The Transaction settles the five Expenses by supplier/date/FIFO. It is not a sixth Expense and it is not an Invoice Ninja customer Payment.

### BlueCarve partial payments

```text
Expenses
$7,014
$380

Transactions
$2,000
$5,000
$394
```

FIFO produces:

```text
$2,000 -> first Expense; $5,014 remains
$5,000 -> first Expense; $14 remains
$394   -> $14 finishes first Expense, then $380 finishes second Expense
```

Both Expenses may then be marked Paid. During the intermediate state, the first Expense may remain shown as Unpaid despite $7,000 having been settled. No native partial Expense-to-Transaction link is required.

## Stable identity and idempotency

Every imported Expense and supplier Transaction must carry a durable, versioned GoTradie identity sufficient to:

- prevent duplicate creation on re-import;
- distinguish genuinely separate source records;
- survive corrections to non-identity classification fields;
- identify the supplier account exactly;
- provide stable ordering when dates are equal;
- allow a fresh GoTradie process to reconstruct settlement from Invoice Ninja alone.

Identity must not depend on the continued presence or filesystem location of the source spreadsheet. Fuzzy matching is not an identity strategy.

Deleted or trashed imported records must be handled explicitly.

In `v0.5.1`, an archived or deleted Expense carrying a GoTradie supplier-account or supplier-purchase marker causes settlement reconstruction to stop until that record is restored or explicitly resolved.

Likewise, an archived or deleted GoTradie-marked supplier Bank Transaction causes settlement reconstruction to stop.

Marked accounting history must not silently disappear from reconciliation or reappear as an active duplicate.

## Architectural integrity test: Invoice Ninja alone

This is the target post-migration integrity test. It does not block completion of the current historical importer; it defines the intended end state once supplier-settlement reconstruction and BAS/EOFY support are implemented.

The target architecture is accepted when all of the following are true:

1. Purchases exist as Invoice Ninja Expenses with their analytical, GST, receipt, and stable identity data.
2. Supplier-account payments exist as marked Invoice Ninja withdrawal Transactions with durable payment and supplier identities.
3. The original spreadsheet is removed from GoTradie's runtime environment or treated as unavailable.
4. No GoTradie cache, SQLite file, allocation database, or side ledger is present.
5. A fresh GoTradie process reads Invoice Ninja and deterministically reconstructs the same supplier/date/FIFO allocations.
6. It produces correct BAS and EOFY data without double-counting Expenses and Transactions.
7. Cash-basis output assigns proportional GST to actual payment periods; non-cash output uses purchase/invoice dates.
8. Re-running reconstruction is idempotent and produces the same result.

Once this integrity test passes, the historical spreadsheet may be archived as no longer operationally required.

## Explicitly rejected designs

The following are architectural violations unless this accepted decision is deliberately replaced in a later approved design:

| Rejected design | Reason |
|---|---|
| Persistent GoTradie cache, SQLite database, or side ledger | Creates a second source of truth and makes recovery depend on local state |
| Treating the spreadsheet as the permanent allocation ledger | Prevents archival and violates the Invoice-Ninja-only acceptance test |
| Invoice Ninja customer Payments for supplier settlements | Models money out as accounts-receivable money in |
| Another Expense for an account payment | Duplicates purchases and risks duplicate GST/EOFY amounts |
| Requiring native partial Expense-to-Transaction links | Invoice Ninja does not adequately represent the required partial allocations |
| Using Paid/Unpaid alone as allocation history | Loses intermediate cash timing and proportional GST information |
| Fuzzy supplier matching | Makes settlement nondeterministic and unsafe |
| Counting settlement Transactions as purchases | Double-counts expenditure and GST |

## CLI safety

Existing GoTradie CLI safety remains unchanged:

- preview is the default;
- `--commit` is the only flag permitting persistent Invoice Ninja changes;
- no alternate `--apply`, `--force`, or `--dry-run` write convention is introduced;
- a preview must describe proposed Expense, Transaction, marker, and Paid-state changes without making them.

## Scope guardrail

This design does not create a general ledger or accounting engine. It defines the minimum durable records and deterministic calculation needed for historical purchase import, supplier-account settlement, and correct BAS/EOFY exports.

When changing this workflow, ask:

> If the spreadsheet and this computer vanished after a successful import, could GoTradie still produce the correct result from Invoice Ninja alone?

If the answer is no, the change violates this design, regardless of how elegant its cache schema happens to look.
