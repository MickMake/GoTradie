# Code review: `feature/ninja-expense-import-2`

> **Historical review snapshot — v0.5.1**
>
> This document records the branch review performed before PR #6 was merged.
> Findings were subsequently triaged and this file is retained as review evidence,
> not as an open work list.
>
> In particular:
>
> - F1 overstated the foreign-currency problem: normal Expense creation correctly uses `Business Amount` and `Business GST`; the remaining foreign-currency concern applies to supplier-account settlement arithmetic and is deferred.
> - F2 was an intentional schema change.
> - F3 is deliberate fail-safe settlement behaviour.
> - F4 is an accepted idempotent-rerun recovery model.
> - F5 is benign.
> - F6 repository-hygiene suggestions were not adopted as release blockers; `.idea` files and the `build.sh` double `clear` are intentional.
> - F7 is non-blocking test coverage.
>
> Remaining genuinely possible future scenarios are tracked in
> [Expense-Importing-Possible-Scenarios.md](./Expense-Importing-Possible-Scenarios.md).

## 1. Branch, status, and intent

**Branch:** `feature/ninja-expense-import-2` (tracks `origin/feature/ninja-expense-import-2`)
**Status:** working tree clean — no uncommitted changes.
**Diff vs main:** 9 commits, 24 files, +3942/−608. No `go.mod`/`go.sum` changes.

| Area | Files |
|---|---|
| GoInvoiceNinja (SDK) | `bank.go` (new), `companies.go` (new), `statics.go` (new) + tests; `client.go`, `expenses.go`, `files.go` (import reorder), CHANGES.md |
| GoTradie (importer) | `expense_import.go` (573→1423 L), `expense_settlement.go` (new, 764 L) + tests (910 + 793 L), `csv.go`, CHANGES.md |
| Other | `docs/Design/Expense-Importing.md` (major rewrite), `docs/Design/README.md`, `build.sh`, `.idea/*` (IDE files) |

**What the branch achieves.** It extends `ninja import expenses` with a new `Account Payment` document type. An `Account Payment` row becomes a DEBIT (withdrawal) Bank Transaction on a fixed, single, active, manual, non-autosync bank account named exactly "GoTradie"; it never creates an Expense or a customer Payment. Payments are allocated deterministically (FIFO by date, then stable identity, integer cents) against the same supplier's unpaid purchases; an expense is marked Paid only when fully settled, and paid state is set/cleared via a focused `UpdatePaymentStatus` call. Idempotency is via versioned SHA-256 markers in expense private notes, transaction descriptions, and payment identity hashes. The redesigned design doc (`docs/Design/Expense-Importing.md`) is consistent with the implementation — FIFO rules, integer cents, partial carry-forward, paid-only-when-fully-settled, no vendor notification, and rejection of archived/deleted marked records all match the code, and the SDK additions respect module boundaries (GoTradie → GoInvoiceNinja only).

**Constraint compliance:** no files modified, no commits, no branch switches, no destructive commands. `go build ./...`, `go vet ./...`, and `go test` were run in both modules — these write only to the Go build/test cache outside the repo; the working tree remains clean. Results: build and vet clean in both modules; all tests pass (`GoTradie/internal/ninja` ok — a cached result for this exact tree; `GoInvoiceNinja` ok, fresh, 0.54 s).

## 2. Findings

**No Critical or High findings.** The changed code is sequential (no concurrency surface), API usage is verified against the SDK's existing helpers, and the test suite is genuinely strong (≈1.5 k lines covering the account-resolution matrix, drift, idempotency, allocation arithmetic, and mutation-freedom of previews/preflights).

### F1 — Medium (assumption-based, verify): no currency validation before settlement math and transaction creation

- **File/function:** `apps/GoTradie/internal/ninja/expense_import.go` — `importAccountPaymentRow` (transaction created with `CurrencyID: state.companyCurrencyID` and the row's raw cents), `preparePurchaseRow`/`allocateAccountPayments` (`expense_settlement.go`), and `durableSupplierSettlementValues` (drift compares raw cents).
- **What's wrong:** the CSV `$ Currency` column is used *only* for the source-identity hash and the "Source currency" private-note line. There is no check that a row's currency equals the company currency, and no conversion. `parseExpenseMoney` deliberately strips `$`/`AUD`/`USD`/`NZD` symbols, so a USD or NZD row parses fine and its face value is silently treated as company currency — in the expense amount (`CreateExpenseRequest.CurrencyID` is never set), in the FIFO allocation, and in drift validation.
- **Why it matters:** if the master spreadsheet contains any foreign-currency row in a file that also contains Account Payments, you get silently wrong allocations, wrong paid-state, wrong amounts in the IN books, and eventually spurious "drift" errors — all undetectable from the output.
- **Smallest sensible fix:** in `prepareExpenseImportRow`, for every row that participates in settlement, reject (row error) unless `$ Currency` is blank or equals the company currency code.
- **Uncertainty, stated plainly:** it is not known whether the real data is single-currency; the design doc is silent on it (it lists "source currency" in the mapping table but prescribes nothing). If the master spreadsheet is single-currency AUD matching the company currency, this is a latent guard gap rather than a live bug.

### F2 — Low (intended breaking change): required columns grew

- **File/function:** `expense_import.go` — `ImportExpensesCSV` (`expenseImportRequiredColumns` at L21).
- **What's wrong:** the required set now includes `Document Type` and `Payment Type`; main required nine columns. Old-shape files fail with `missing required column`. The design doc explicitly wants these columns (canonical values; marker stability across their edits), so this appears deliberate — but it is a breaking change for anyone re-running the importer on an older export.
- **Fix/verify:** none in code; confirm the master spreadsheet actually contains both columns (manual item below).

### F3 — Low (by design, documented): one bad purchase hard-blocks a payment

- **File/function:** `expense_settlement.go` — `blockPaymentsWithInvalidPurchases`.
- **What's wrong (as a trade-off):** if any earlier-or-same-date purchase row has *any* error — including unrelated ones such as an unknown category — the Account Payment row gets a hard error instead of a partial allocation. A permanently broken purchase row will block every later same-supplier payment until it is fixed.
- **Why rated Low:** the accepted design doc mandates explicit reporting over guessing ("never skip an older eligible unresolved expense… report unapplied… explicitly") and the tests codify exactly this behaviour, so it's a known, documented choice rather than a bug. Optional refinement: scope the block to allocation-relevant errors (invalid amount/date) rather than any `row.err`.

### F4 — Low: three-stage commit has no rollback; one inconsistent fail-safe

- **File/function:** `expense_import.go` — `ImportExpensesCSV` commit path; `expense_settlement.go` — `blockPaymentsWithUnmarkedLegacyExpenses`.
- **What's wrong:** if a payment transaction create fails after some expenses were already created (stage 1), nothing is rolled back; stage-3 `UpdatePaymentStatus` errors only mark that row. The system self-heals on re-run (the committed-only recomputation clears desired paid state and the reconciler clears it remotely), and exit code 1 prompts the re-run — acceptable for a one-shot historical importer. Separately, `blockPaymentsWithUnmarkedLegacyExpenses` returns a whole-import error for remote-only legacy blocking but a row error for current-CSV rows — inconsistent, though fail-safe in both directions.
- **Fix:** none required; at most document the "re-run after partial commit" expectation in CHANGES.md.

### F5 — Low (benign): dry-run memoises pre-update expense state

- **File/function:** `expense_import.go` — `importExpenseRow` (dry-run branches call `rememberExpense` with the *current* remote state after `would-update`).
- **What's wrong:** later in-memory lookups within the same read-only preview see pre-update values. Harmless in practice (preview makes no writes and each run is a fresh process); noted for completeness only.

### F6 — Low: repository hygiene

- **What's wrong:** `.idea/*` IDE files committed at the repo root; junk commit messages (`dd`, `hiccup`, `blah blah PR6`); and — verified via `git diff 69a5964 d00d860` — commit `d00d860` largely *reverts* `69a5964` (20 ins/379 del against the prior commit's 379 ins/20 del in `expense_settlement.go`) before the final state is reached. Also `build.sh` now runs `go test ./...` without `set -e` (a failing test won't stop the build — it just prints) and adds `clear; clear`.
- **Fix:** interactive rebase/squash and drop the IDE files before merge; optionally add `set -e` to `build.sh`.

### F7 — Low: two test-coverage gaps

- No multi-currency scenario anywhere (ties to F1 — a single test asserting a USD row in a settlement file is rejected would also lock in the guard).
- No `>1` page pagination test for the new `bank_integrations`/`bank_transactions`/`bank_transaction_rules` listings (the shared `listAllWithValues` helper is exercised, but never for these specific endpoints beyond one page).

**Verified as *not* problems** (so you don't re-check them): SDK API surface in `bank.go`/`companies.go`/`statics.go` (pagination, required-field omitempty policy, pointer semantics of `NotifyVendorWhenPaid`); purchase source identity deliberately excludes `Document Type`/`Payment Type` while retaining the legacy v1 whole-row marker for migration matching — exactly as the design requires; bank-account safety gates (single active manual non-autosync account, distinct errors); DEBIT direction/positive-amount/drift validation; auto-convert-rule and `notify_vendor_when_paid` preflights (both fail with zero mutations); module boundaries; and sequential execution (no concurrency issues).

## 3. Verdict

**Overall confidence: high.** The change is well-fenced, deterministic, sequential, design-doc-aligned, and unusually well tested for its blast radius. The one genuinely open risk is the single-currency assumption (F1).

**Approve?** **Yes — with two pre-merge asks.** Nothing here is Critical/High. Ask the author to (1) add the currency guard from F1 *or* document the single-currency invariant in the design doc (whichever matches the real data), and (2) clean up the git history (F6). F2–F5 are informational; I would not block on them.

**Three most important things to verify manually:**

1. **Real preview → `--commit` → re-run** of `ninja import expenses` against the live Invoice Ninja instance with a file mixing purchases and Account Payments: confirm no duplicate expenses/transactions on re-run, paid state stable, and allocation/unapplied numbers match your spreadsheet expectation.
2. **The master spreadsheet:** confirm it contains the now-required `Payment Type` and `Document Type` columns (F2), and confirm it is **single-currency matching the company currency** — if any USD/NZD rows exist, F1 becomes a real bug to fix before commit (F1/F7).
3. **Live IN instance shape:** confirm there is exactly one active bank account named exactly "GoTradie" (manual, non-autosync), no active auto-convert DEBIT rules on it, and `notify_vendor_when_paid` is disabled — or accept that the importer's preflight errors are the intended outcome until it is (F3/F4 territory: these gates hard-fail the whole import).
