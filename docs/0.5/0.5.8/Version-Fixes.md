# GoTradie v0.5.8 - Historical Review Fixes

**Status:** Review triage ledger; no fixes have been authorised or applied by this document.  
**GitHub baseline inspected:** 12 October 2026, `main` tree `127f949df43c85c8d0d34b2d4bb20d1669db06f6`.

## Purpose

Reconcile review feedback from the earlier v0.5 slices **without blindly reopening every old review**. Some findings were fixed later, some were closed by explicit owner decision, and some remain unresolved on GitHub. A thread's resolved flag is not proof that current `main` is correct; an unresolved flag is not proof that a defect still exists.

At the inspection date, **all repository PRs #1-#11 were merged** and no open PRs were returned. However, the old branch `feature/ninja-expense-import-2` still **diverges from `main` by one branch-only commit** (93 commits behind). The two unmatched changes reported are historical `docs/Design/Expense-Importing-Possible-Scenarios.md` and `docs/Design/review-feature-ninja-expense-import-2.md`. Its PR #6 is merged, but the extra branch commit is **not** fully merged. **STOP before any code implementation until the owner explicitly confirms retirement or handling of this leftover branch work.** This offline ZIP does not change the repository. Recheck all branch/PR state during the implementation preflight.

## Status terms

- **GH unresolved:** GitHub review thread is still open. Verify the issue against current `main` before changing code.
- **GH resolved:** GitHub thread is closed. Inspect later commits/tests before marking independently verified.
- **Owner decision:** Owner explicitly waived/rejected a hypothetical change; do not reopen without new evidence and approval.
- **Verified fixed / confirmed defect / deferred / not applicable:** Fill in **after** checking current code and running focused tests. Do not infer these from a review comment alone.

## 1. Unresolved GitHub review threads

These are the immediate candidates for inspection, **not an automatic implementation checklist**.

| Area | Review finding | Priority | Reference | Next verification |
| --- | --- | --- | --- | --- |
| v0.5.5 EOFY | Asset financial year cannot be inferred from purchase date without first-used evidence | P1 | [PR #9 comment](https://github.com/MickMake/GoTradie/pull/9#discussion_r4227114439) | Trace first-use data into `internal/eofy/report.go`; test purchase in one FY, first use in next. |
| v0.5.5 EOFY | `BAS Treatment = Review` must not be treated as a resolved GST-credit entitlement | P1 | [PR #9 comment](https://github.com/MickMake/GoTradie/pull/9#discussion_r4227114448) | Check `TaxKnown`/treatment state and threshold classification; unresolved entitlement must not yield a confident report. |
| v0.5.6 Financial export | Long bank transaction identifiers can lose digits in Excel numeric cells | P2 | [PR #10 comment](https://github.com/MickMake/GoTradie/pull/10#discussion_r4234488396) | Verify identifier cells remain exact text beyond Excel's 15-digit precision. |
| Earlier Bunnings Web | Website search may read product title from an `href` substring rather than full HTML | P2 | [PR #3 comment](https://github.com/MickMake/GoTradie/pull/3#discussion_r3286004247) | Check current `apps/GoBunnings/web.go` and web-mode search output; fix only if reproducible. |

The first three are from the v0.5.5 and v0.5.6 slices. The last predates the v0.5 slices but is relevant because v0.5.8 exposes Bunnings Web via one provider mode.

## 2. Earlier slice reviews currently resolved in GitHub

These findings should be **verified against today's code and regression tests**, not copied straight into a new fix patch.

### v0.5.1 - Expense importer and settlement ([PR #5](https://github.com/MickMake/GoTradie/pull/5), [PR #6](https://github.com/MickMake/GoTradie/pull/6))

**PR #5 - all five threads GH resolved**

- P1 identical legitimate ledger rows: owner originally retained the old source-row identity; v0.5.2 later introduced mandatory `Import ID`. Verify the final importer distinguishes separate supplied IDs rather than deduplicating by row contents.
- P2 preview of planned Vendor/Category/Project dependencies: owner reported fix `6e1e30d`; verify once against a multi-row fixture.
- P2 stdin import `-`: initially fixed in `6e1e30d`, but v0.5.2 deliberately **removed stdin support for Expense import**. Follow the newer accepted design; do not restore stdin.
- P1 Expense category JSON field: owner reported fix `3d559d7` (`expense_category_id`); verify client request/response tests.
- P2 explicit zero Business Amount: GH resolved; verify explicit zero is preserved rather than replaced by a computed amount.

**PR #6 - all three threads GH resolved**

- P1 an errored earlier supplier-account purchase must block unsafe allocation to later purchases.
- P1 same-day identical account payments need distinct stable identity. The later `Import ID` contract may supersede the reported collision; verify before proposing a separate discriminator.
- P1 archived/deleted marked Expenses must not silently disappear from settlement reconstruction and cause misallocation.

**Review focus:** Supplier-account payment correctness matters to cash-basis BAS. Prioritise focused tests here if the current implementation does not already demonstrate the invariant.

### v0.5.2 - Expense import operational work ([PR #7](https://github.com/MickMake/GoTradie/pull/7))

**Both threads GH resolved**

- P1 `UpdateExpenseRequest` must preserve sparse-update semantics and must not wipe accounting fields when only notes change.
- P2 receipt marker must survive reruns for non-owner sibling Expenses, including when receipts root is omitted.

Check the Invoice Ninja request model and receipt rerun tests before declaring either verified fixed.

### v0.5.3 - Hierarchical configuration

No v0.5.3-specific pull request appears in the repository's PR #1-#11 list. Do not manufacture a review finding. Validate existing configuration behaviour through its accepted design and current tests only when code touched by v0.5.8 requires it.

### v0.5.4 - BAS and Accounting Dataset ([PR #8](https://github.com/MickMake/GoTradie/pull/8))

**All three threads GH resolved**

- P1 cash-only payment/settlement exceptions must not make a correctly calculable accrual BAS incomplete.
- P1 allocation error dates must be attributed to the affected payment period, not merely the original invoice/expense period.
- P2 validate allocations even after the source total has been filled, to catch extra/duplicate allocations rather than silently stop.

**Review focus:** These affect correctness of BAS reports. Verify source-period selection and errors with narrow fixtures before reclassifying as done.

### v0.5.5 - EOFY ([PR #9](https://github.com/MickMake/GoTradie/pull/9))

Both threads are **GH unresolved**; see section 1. Do not infer the correct first-use date or GST entitlement without actual source evidence.

### v0.5.6 - Financial export ([PR #10](https://github.com/MickMake/GoTradie/pull/10))

The identifier-precision thread is **GH unresolved**; see section 1.

### v0.5.7 - Product Sync ([PR #11](https://github.com/MickMake/GoTradie/pull/11))

**All seven threads GH resolved.** The recorded owner responses identify these dispositions:

- **Owner decision (no code fix requested):** hypothetical legacy migration of a Product owned by an unrelated Vendor was explicitly rejected because GoTradie had not previously created those Products. The final v0.5.7 design has no legacy Product migration. Do not revive this scenario without a new requirement.
- **Reported fixed:** Bunnings pricing request failure propagation (`ac29463`).
- **Reported fixed:** native Product identity independent of blank Last Sync Date (`f51105c`).
- **Reported fixed:** duplicate explicit Bunnings keys selected for creation (`f51105c`).
- **Reported fixed:** native identity independent of blank availability (`91741e7`).
- **Reported fixed:** file-source fingerprint reflects configured field mappings (`91741e7`).
- **Reported fixed:** file-source fingerprint reflects configured Supplier name and aliases (`918c13d`).

Those last two are historical v0.5.7 file-source corrections, **not a request for new Bunnings-mode cache logic**. Verify them only where existing source handling is touched or tests provide a clear reproducer.

### Older pre-slice reviews ([PR #2](https://github.com/MickMake/GoTradie/pull/2))

Both threads GH resolved: valid quoted CSV output for Bunnings, and nonzero failure status when individual Bunnings lookups fail. Their behaviour can be spot-checked if v0.5.8 CLI work touches those paths; do not broaden the change without evidence.

## 3. Required reconciliation workflow

For each candidate that matters to this release:

1. Follow the linked review and read the **actual** current file, later commits and relevant tests on latest `main`.
2. Reproduce the issue with a minimal deterministic test where possible. Check design intent and explicit owner decisions.
3. Record finding, affected file/line, evidence, classification, and proposed remedy.
4. Mark one of: **verified fixed**, **confirmed defect**, **owner-deferred**, or **not applicable**.
5. Obtain approval for the confirmed fixes and branch scope before modifying code. Separate broader accounting work if it would derail the Product Sync slice.
6. Implement only approved fixes; run focused regression tests plus appropriate module-wide verification.
7. Review the actual diff with at most **three** review/fix cycles, reporting anything still unresolved.

## 4. Release guardrails

- An unaddressed review comment is not a licence for speculative redesign.
- A resolved review comment is not, by itself, test evidence.
- Do not reopen historical identity migrations by default.
- Do not change the accepted GST-inclusive NST pricing approach while reconciling unrelated reviews.
- Do not add provider flags, generic scraping, new persistence, or unrelated Invoice Ninja maintenance changes.
- Use the common implementation workflow: latest-main preflight, explicit user approval, feature branch, test evidence, and merge approval.

**Outcome expected:** An evidence-backed list of what was actually fixed, what genuinely still fails, and what the owner intentionally deferred. No archaeological dig should be allowed to consume the BAS deadline.
