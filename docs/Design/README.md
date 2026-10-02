# GoTradie Design Documents

This directory contains behaviour and design contracts for GoTradie features where the intent matters as much as the implementation.

The aim is to stop later work from reconstructing the premise from code, old chats, sedimentary layers, or the position of the moon.

## Documents

### [Expense-Importing.md](./Expense-Importing.md)

Design baseline for the historical Invoice Ninja expense importer, including:

- the existing purchase-row mapping that must be preserved;
- canonical `Document Type` values;
- exact Invoice Ninja `Payment Type` semantics;
- immediate-paid versus supplier-account purchases;
- `Account Payment` rows;
- conservative payment allocation;
- partial-payment requirements;
- GST accounting-basis requirements;
- supplier-account reconciliation;
- idempotency/source-identity constraints;
- known implementation gaps before the paid/unpaid work is complete.

### [Command-Line-Spec.md](./Command-Line-Spec.md)

The GoTradie command-line behaviour contract.

This file is the existing root `GoTradie-Command-Intention-Spec.md` moved/renamed into the design-document structure without changing its content.

It locks down the CLI safety model, especially:

- preview by default;
- `--commit` as the single persistent-write flag;
- command grouping and responsibilities;
- repository/SDK ownership boundaries.

## Working rule

These documents describe deliberate behaviour, not whatever happens to fall out of today's implementation.

When implementation and design disagree:

1. inspect the current GitHub branch and tests;
2. decide whether the code or the design is wrong;
3. change one deliberately;
4. update the other in the same piece of work.

Do not silently reinterpret a design contract because a nearby function looked persuasive.
