# Historical Expense Importing

Status: Design baseline for the historical expense/payment importer  
Software version: Not yet assigned

## Purpose

This document locks down the intent of GoTradie's historical expense importer before further paid/unpaid work is implemented.

The existing expense import mapping is already producing the desired Invoice Ninja expense record. The paid/unpaid work must extend that behaviour without redesigning or degrading the purchase analytics.

The central rule is:

> A purchase row is the authoritative analytical record of the purchase. A later account payment settles that purchase; it does not replace, reclassify, or recreate it.

## Existing purchase mapping: preserve it

One spreadsheet purchase row creates one Invoice Ninja expense.

The current mapping deliberately preserves the analytical resolution of each individual purchase row, including:

- purchase date;
- supplier and store context;
- Job Number;
- Child Job Number;
- expense Category;
- analytical Option;
- Tax Treatment;
- Business %;
- BAS treatment;
- item number and description;
- quantity, unit and unit price;
- source ex-GST, GST and inc-GST totals;
- source currency;
- business amount and business GST;
- source notes;
- source filename;
- receipt/document attachment;
- supplier invoice/reference number;
- stable source identity for idempotency.

Current Invoice Ninja mapping includes:

| Spreadsheet concept | Invoice Ninja representation |
|---|---|
| Supplier + Store | Vendor display name currently uses `Supplier - Store` when Store is present |
| Date | Expense date |
| Category | Expense Category |
| Option | Expense custom analytical field / Tax Detail |
| Tax Treatment | Expense custom field |
| Business % | Expense custom field |
| BAS Treatment | Expense custom field; currently derived by importer logic |
| Business Amount | Expense amount |
| Business GST | Expense tax amount |
| Invoice Number / purchase reference | Transaction Reference |
| Store, Job Number, Child Job Number, item detail, source totals, notes, source file | Private Notes |
| Receipt filename | Private attached document |

Job Number and Child Job Number must always remain attached to the purchase row. Numeric Job Numbers may additionally resolve to an Invoice Ninja Project under the current importer behaviour, but Project linkage must never be the only copy of the job analytics.

Receipt uploads remain private (`is_public=false`). `Should be Invoiced` remains OFF/untouched. `Add Documents to Invoice` remains OFF.

Deleted/trashed imported expenses must not permanently block re-import of the same source purchase.

## Document Type

`Document Type` is a controlled field. The canonical values are exactly:

```text
Invoice
Receipt
Adjustment
Account Payment
```

The source spreadsheet should be normalised to those values. The importer should trim surrounding whitespace but otherwise reject unknown document types rather than guessing that several vaguely similar accounting phrases probably mean the same thing.

Historical source labels such as `TAX INVOICE`, `ADJUSTMENT NOTE`, `CR/ADJ NOTE`, `DEBIT MEMO` and `TAX ADJUSTMENT` belong in source-cleanup/migration work, not in the long-term canonical vocabulary.

### Purchase document types

`Invoice`, `Receipt`, and `Adjustment` are purchase-side records.

- `Invoice` creates a normal expense row.
- `Receipt` creates a normal expense row.
- `Adjustment` is an adjustment to purchase/account value, not a supplier payment. Its financial direction must come from the signed amount, not from trying to infer meaning from old free-text document labels.

### Account Payment

`Account Payment` is not another expense.

It represents money actually paid against a supplier trade account. It must not create a duplicate expense or duplicate GST transaction.

## Payment Type

`Payment Type` must use Invoice Ninja's payment-type vocabulary exactly. GoTradie should not invent aliases such as `Bunnings Account / Trade Account` as payment methods.

Examples already present in the source data include:

```text
Visa Card
PayPal
```

Other values must use the exact Invoice Ninja label supported by the target Invoice Ninja instance/API.

### Purchase rows

For `Invoice` and `Receipt` rows, the intended simple rule is:

- non-blank valid `Payment Type` = paid immediately at purchase;
- blank `Payment Type` = unpaid supplier-account purchase.

No separate Payment Status column is required for the historical importer unless a later real requirement proves otherwise.

For an immediately paid purchase:

```text
Expense date = purchase date
Mark Paid = ON
Payment date = purchase date
Payment Type = spreadsheet Payment Type
```

For an unpaid supplier-account purchase:

```text
Expense date = purchase date
Mark Paid = OFF
Payment date = blank
Payment Type = blank
```

Do not infer supplier-specific account behaviour. The paid/unpaid state must come from the row data.

## Account Payment rows

An `Account Payment` row owns payment facts, not purchase analytics.

Relevant fields are:

- payment date;
- Supplier;
- gross payment amount;
- payment reference;
- Payment Type;
- optional supporting document/receipt.

For the current spreadsheet shape, `Total Inc GST` is the gross supplier-account payment amount. Purchase-analysis fields that happen to be populated by spreadsheet formulas are ignored for an `Account Payment` row.

In particular, an Account Payment row must not create or overwrite:

- Job Number;
- Child Job Number;
- Category;
- Option;
- Tax Treatment;
- GST classification;
- Business %;
- BAS treatment;
- item number or description;
- purchase quantity/unit;
- purchase transaction reference.

An Account Payment row must not create an Invoice Ninja client Payment object. Invoice Ninja client payments are accounts-receivable records; this is a supplier/accounts-payable payment event.

## Supplier account identity

Trade-account allocation is keyed by the spreadsheet `Supplier`, not by the exact Invoice Ninja Vendor display name.

This matters because current expense display names include Store, for example:

```text
Bunnings - Dural
Bunnings - Online
```

Those may still belong to the same supplier trade account.

`Store` remains purchase metadata. It must not accidentally split one supplier account into several unrelated account balances.

When processing an import file, GoTradie should use the source purchase rows and their stable source identities to associate existing Invoice Ninja expenses back to the source Supplier rather than attempting fuzzy vendor-name matching.

## Payment allocation

Account payments are allocated conservatively against eligible outstanding purchases for the same Supplier.

Initial rule:

1. Only purchases dated on or before the account-payment date are eligible.
2. Oldest outstanding purchase first.
3. For purchases with the same date, use stable source-row order as the tie-breaker.
4. Allocate only the amount actually available from the payment.
5. A fully covered expense may become paid on the actual account-payment date.
6. An uncovered expense remains unpaid.
7. Any unapplied payment remainder is reported explicitly.
8. Never fabricate a paid state simply because a payment exists somewhere on the supplier account.
9. Do not allocate past an earlier eligible purchase row which is invalid or otherwise cannot participate reliably.
10. Do not allocate past an older unpaid GoTradie-imported expense which is absent from the valid source purchase rows in the current input.

If either source data or existing imported history makes oldest-first allocation indeterminate, report the account payment as blocked rather than marking a later expense paid.

Example:

```text
18/09/2026 Purchase A   $71.84   unpaid
18/09/2026 Purchase B   $28.70   unpaid
01/11/2026 Account Pay  $71.84   Visa Card
```

Result:

```text
Purchase A -> fully allocated -> paid 01/11/2026
Purchase B -> no allocation   -> remains unpaid
Unapplied payment remainder    -> $0.00
Supplier account outstanding   -> $28.70
```

## Updating a fully settled Invoice Ninja expense

When a later account payment fully settles an expense, the purchase record remains the purchase record.

The importer may update the paid state and actual payment date. Payment Type may be applied where it truthfully describes the settlement.

The purchase `Transaction Reference` must not be replaced by the account-payment reference. It currently stores the supplier invoice/purchase reference and is part of the existing desired mapping.

The account-payment reference belongs in GoTradie's allocation/reconciliation detail and, if later mirrored into Invoice Ninja notes, must be additive rather than destructive.

## Partial and multiple payments

Invoice Ninja expenses are effectively binary paid/unpaid for this workflow. GoTradie therefore cannot use Invoice Ninja's paid flag as the complete accounting record when an expense is only partly settled.

Required behaviour:

- a partial allocation does not mark the expense fully paid;
- allocation facts must not be discarded;
- the allocation record must retain at least:
  - purchase source identity;
  - payment source identity;
  - allocated amount;
  - payment date;
  - payment method;
  - payment reference;
- later BAS and account-reconciliation logic must use the detailed allocation facts, not merely the Invoice Ninja paid flag.

If several account payments with different methods contribute to one purchase, GoTradie must not invent a single historical payment method that pretends to describe the whole settlement. The allocation detail is authoritative in that case.

The allocation detail is authoritative for the duration of the import/reconciliation calculation.

For the historical importer, allocation detail does not require a separate persistent datastore. It must be deterministically reproducible from:

- the source purchase rows;
- the source Account Payment rows;
- stable source identities;
- and relevant existing Invoice Ninja expense state.

Import/reconciliation output must retain enough allocation detail to support inspection, GST/BAS calculation, and supplier-account reconciliation during that run.

If a future workflow requires allocations to persist independently of the source ledger, that must be designed separately.

## GST accounting basis

GoTradie needs one simple business-level setting:

```yaml
gst_accounting_basis: cash
```

or:

```yaml
gst_accounting_basis: non_cash
```

No UI is required.

The importer must preserve enough facts for later BAS calculations to use the chosen basis correctly:

- original purchase date;
- original purchase amount and GST;
- business-use amount and GST;
- actual payment dates;
- allocation amounts;
- outstanding amount.

For cash-basis work, actual payment/allocation timing matters. For non-cash work, the purchase-side facts remain available independently of later settlement.

The payment row itself does not create a second GST amount. GST remains attached to the underlying purchase/adjustment rows.

## Supplier-account reconciliation

The same source and allocation data should support:

```text
opening balance
+ purchases
- payments
= calculated closing balance
```

and report:

- outstanding purchases;
- applied payments;
- unapplied payment amounts;
- calculated supplier-account balance.

This is both an accounting check and a practical way to detect supplier-account discrepancies.

## Idempotency and source identity

The importer must remain safe to re-run.

Existing behaviour uses a stable source marker to associate a spreadsheet row with its imported expense. Duplicate rows must not silently create duplicate expenses, and deleted/trashed imported expenses must be re-importable when appropriate.

### Important implementation gap

The current importer computes the source marker from the entire source row. That becomes fragile now that canonical `Document Type` values and the new `Payment Type` column are being introduced: changing a non-identity field changes the row hash and can make an already imported purchase appear new.

Before the revised importer is committed against data that may already have been imported, source identity needs to remain stable across non-identity edits such as:

- `TAX INVOICE` becoming `Invoice`;
- adding or correcting `Payment Type`;
- other analytical corrections that do not make the purchase a different source transaction.

Do not solve this with fuzzy matching. Define a stable source identity from genuinely identifying source fields, or provide an explicit migration path for existing markers.

When otherwise-identical purchase facts occur on legitimate split rows, use stable source order within that collision group as the deterministic discriminator. The discriminator must not make corrections to Job Number, Category, Option, Document Type, or Payment Type look like a new purchase. Existing legacy markers remain valid migration aliases. Byte-identical Account Payment rows are ambiguous and must be reported rather than silently discarded.

## Known implementation gaps before paid/unpaid work is complete

The current branch still needs the following narrow changes to satisfy this design:

1. Purchase creation currently supplies the purchase date as `PaymentDate` for every expense; this must become conditional.
2. The current GoInvoiceNinja expense request model does not expose an expense Payment Type field, so exact Invoice Ninja Payment Type support will require the smallest appropriate SDK addition.
3. Account-payment rows need their own parse/allocation path and must not flow through normal expense creation.
4. Account-payment allocation needs a small deterministic in-memory representation and result/reporting shape suitable for BAS/reconciliation calculations; no new persistent datastore is required.
5. Source-marker identity must remain stable when the new canonical/document-payment fields are edited.

These are implementation gaps, not invitations to redesign the existing purchase mapping.

## CLI safety

The existing GoTradie CLI safety contract remains unchanged:

- preview/default behaviour is safe;
- `--commit` is the only flag that permits persistent remote changes;
- no additional `--apply`, `--force`, or alternate write flags should be invented for this importer.
- before a commit which could set an expense payment date, read the current Invoice Ninja company `notify_vendor_when_paid` setting and abort before any writes if it is enabled or cannot be determined reliably.

See [Command-Line-Spec.md](./Command-Line-Spec.md).

## Design guardrail

When changing this importer, ask one question first:

> Does this change preserve the original purchase row's analytical meaning?

If the answer is no, it needs a very good reason. The paid/unpaid work exists to model settlement timing correctly, not to turn a useful purchase ledger into accounting soup.

## Implementation Constraints

This importer is deliberately narrow in scope. Implement the behaviour described in this document without introducing a broader accounting system.

### Keep the existing expense model

The current purchase-to-Invoice-Ninja expense mapping is considered correct.

Do not redesign or replace the existing handling of:

- Supplier / vendor
- Purchase date
- Job Number
- Child Job Number
- Category
- Option
- Tax Treatment
- Business %
- BAS Treatment
- Item detail
- Source totals
- GST amounts
- Notes
- Receipt attachment
- Transaction reference
- Existing source/idempotency markers

The paid/unpaid work extends this model; it does not replace it.

### No new persistence layer

Do not introduce:

- a database,
- a local ledger,
- an allocation datastore,
- an accounting framework,
- or another persistent state mechanism.

For historical import, payment allocation should be deterministically derived from the source data and the existing Invoice Ninja state.

If a future feature genuinely requires persistent allocation state, design that separately.

### Payment Type

`Payment Type` values must correspond exactly to Invoice Ninja payment types.

Rules:

- Perform an exact Invoice Ninja payment-type lookup.
- Store/use the corresponding Invoice Ninja payment type ID where required by the API.
- Do not perform fuzzy matching.
- Do not invent aliases.
- Do not silently substitute another payment method.
- An unknown non-blank Payment Type is a row error.

For a normal purchase:

- a recognised immediate-payment type means the expense is paid on the purchase date,
- a purchase intentionally recorded as unpaid has no payment date and is not marked paid.

For an `Account Payment` row, `Payment Type` describes how the supplier-account payment itself was made.

### Account Payment rows are not expenses

When:

```text
Document Type = Account Payment
```

the row represents money paid to a supplier account.

It must not:

- create another expense,
- create another GST purchase,
- overwrite purchase analytics,
- inherit Category/Option meaning from spreadsheet filler values,
- or manufacture an Invoice Ninja expense merely to represent the payment.

Purchase analytics remain attached to the original purchase rows.

### Account Payment supporting documents

An `Account Payment` row may contain a supporting filename or document reference.

Do not create a fake Invoice Ninja expense solely so that this document has somewhere to attach.

For the initial implementation:

- preserve/report the source filename,
- attach it only if there is a natural supported Invoice Ninja object for the payment workflow,
- otherwise leave it unattached.

Supporting-document storage can be improved later without changing accounting behaviour.

### Payment allocation

Allocate account payments conservatively:

1. Match the supplier.
2. Consider eligible outstanding supplier-account purchases.
3. Allocate oldest purchase date first.
4. For purchases on the same date, use stable source-row order.
5. Mark an expense paid only when fully covered.
6. Use the actual account-payment date as its payment date.
7. Leave partially covered expenses unpaid in Invoice Ninja.
8. Retain/report partial allocation information within the import result so later GST/BAS processing can distinguish paid and unpaid amounts.
9. Report any unapplied payment remainder.

Never fabricate a fully-paid state merely because Invoice Ninja has a binary paid/unpaid expense model.

### Source identity and idempotency

The current importer derives its source marker from the complete spreadsheet row.

Adding or changing fields such as `Document Type` and `Payment Type` must not cause an expense that was already imported from the same historical purchase to be imported again.

Before implementation, adjust source identity so that:

- the same underlying purchase retains stable identity,
- control/classification changes do not accidentally create duplicate expenses,
- existing legacy source markers can still be recognised where necessary,
- intentionally distinct spreadsheet rows from the same supplier invoice remain distinct.

Do not solve this by weakening duplicate detection across genuinely separate purchase rows.

### Adjustments

`Adjustment` is a valid `Document Type`, but its accounting behaviour must not be guessed.

In particular:

- `Adjustment` alone does not determine whether the amount is positive or negative.
- Its effect must come from explicit source data.
- Paid/unpaid behaviour must follow an explicitly defined rule rather than supplier-specific inference.

If the source data does not yet provide enough information to implement adjustments safely, report/defer them rather than inventing behaviour.

### CLI safety

Existing CLI safety behaviour remains unchanged:

- preview is the default,
- `--commit` is the only persistent-write flag,
- no Invoice Ninja changes occur without `--commit`.

Do not introduce additional write flags such as:

```text
--apply
--force
--dry-run
```

### Scope guardrails

Do not introduce, unless separately designed and approved:

- a UI,
- ERPNext integration,
- supplier-specific rules,
- fuzzy matching,
- an accounting engine,
- a general ledger,
- a reconciliation framework beyond the calculations required here,
- new architectural abstractions merely to support this importer.

Prefer small extensions to the existing expense-import path.

The objective is to correctly represent purchases, unpaid supplier-account purchases, and subsequent account payments while preserving the existing analytical detail.

If the implementation starts requiring a diagram of underground blast doors, it has probably wandered out of scope.
