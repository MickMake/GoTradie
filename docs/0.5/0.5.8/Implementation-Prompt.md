# GoTradie v0.5.8 - Implementation Prompt

**Status:** Draft for approval; do not implement until preflight and user approval.

**Suggested branch:** `v0.5.8-provider-sync` (create only after approval, from fresh `origin/main`).

## Objective

Implement the agreed follow-up Product Sync behaviour described in:

- [`../../FutureEnhancements/GoTradie_Product_Sync_Design.md`](../../FutureEnhancements/GoTradie_Product_Sync_Design.md) - authoritative v0.5.8 feature design.
- [`../../Command-Line-Spec.md`](../../Command-Line-Spec.md) - global CLI semantics.
- [`../../Implementation-Workflow.md`](../../Implementation-Workflow.md) - mandatory implementation/review/merge process.
- [`Version-Fixes.md`](./Version-Fixes.md) - historical PR review reconciliation, **not** a list of automatically approved changes.

The closed v0.5.7 Product Sync design remains authoritative for identity, supplier validation, availability, and existing sync protections except where v0.5.8 explicitly supersedes its CLI/provider configuration.

## 1. Mandatory preflight - STOP before changes

1. Fetch current `origin/main`, inspect branch/worktree and the actual implementation and tests.
2. Check outstanding PRs and previous slice branches; if any active predecessor is not merged into `main` or explicitly retired, **STOP and ask**.
3. Compare the accepted v0.5.8 design with the repository, configuration parser, Bunnings API/Web adapters, file providers, Invoice Ninja client, and global CLI contract.
4. Identify any conflicting documents, unimplemented transport, CLI interactions, and new dependencies. Do not guess around an ambiguity.
5. Present the exact proposed branch name, current base commit, files/packages to change, intended tests, and any proposed dependency. **STOP and obtain explicit user approval before creating a branch or editing repository code.**

**Known preflight blocker as of 12 October 2026:** `feature/ninja-expense-import-2` has one branch-only documentation commit despite PR #6 being merged. Do not begin implementation until the owner confirms the extra branch changes are retired or otherwise handled. Recheck against latest `main`.

This ZIP contains drafting material only. It does not grant permission for implementation or merging.

## 2. Product Sync CLI and provider configuration

- Implement `GoTradie sync [provider] [product-key] [--commit]`.
- Without a provider, run only configured `auto_sync: true` providers; default is `false`.
- With a provider, run only that provider, independent of its `auto_sync` setting.
- With a product key, synchronise only that product.
- Keep dry-run as default. `--commit` is the sole Invoice Ninja write permission.
- Do **not** add a `--provider` flag.
- Leave `GoTradie ninja import/export` as native Invoice Ninja maintenance, not supplier Product Sync.
- Replace the old Product Sync `refresh`/`import` source-selection forms and sync-specific `--web` usage as specified; update CLI help and affected contract examples. Do not change unrelated Bunnings CLI features.

## 3. One Bunnings provider, one source mode

- Accept **one** `providers.bunnings` entry, `type: bunnings`, with an explicit `mode: api | web`.
- Reuse the existing API and Bunnings website adapters. Route through the **same** product-matching/update pipeline.
- Both modes resolve to the same canonical Supplier and native Product key; no `bunnings-web` duplicate provider entry.
- Validate mode-specific settings only for the selected mode.
- `sync bunnings` refreshes known Bunnings Products. `sync bunnings <product-key>` looks up that one item.
- Preserve v0.5.7 duplicate rejection and Vendor validation. Do not invent mode-change migration, persistence, or cache invalidation work.

## 4. Source retrieval and decoding

- Preserve current CSV/XLSX decoding and HTTP(S) catalogue retrieval.
- Add `ftp://` and `file://` retrieval for **the same CSV/XLSX decoders**.
- Start with anonymous/read-only public FTP; do not introduce credential management without separate approval.
- Use explicit absolute local file URLs. No `--file` argument is required.
- Keep retrieval separate from format decoding and field mapping. No generic website crawling, legacy XLS decoder, or downloader plugin framework.
- If FTP support needs a new Go dependency, report the exact dependency and justification in preflight before adding it.

## 5. Expanded fields and common description template

- Continue mapping mandatory catalogue fields `product`, `description`, and the currently supported GST-inclusive `price`.
- Keep `supplier` derived from provider configuration and validated against one active Invoice Ninja Vendor; it is **not** a CSV field.
- Accept additional `fields` keys (e.g. `category`, `unit`) for description rendering without allocating extra Invoice Ninja fields.
- Implement a shared Go `text/template` renderer for **all** provider adapters. Default Description is the existing source description when no template is configured.
- Template keys come from GoTradie's normalised mapped field names, not raw CSV headings. Validate unknown references; optional absent values should allow conditional omission.
- For the real NST CSV, use `CODE`, `DESCRIPTION`, public `PRICE (INC GST)`, `CATEGORY`, and `UNIT` as documented.
- Do **not** implement exclusive-price GST calculation, modify the Invoice Ninja Tax field, or infer GST-free treatment from a supplier column. That is explicitly deferred.
- Do not add a configurable expression engine, extra field tiers, schema discovery, or a second Product data store.

## 6. Invoice Ninja setup and validation feature

The separate proposal [`../../FutureEnhancements/InvoiceNinjaSetup.md`](../../FutureEnhancements/InvoiceNinjaSetup.md) covers `invoiceninja check`, `invoiceninja setup`, and the Product Sync preflight. Before treating those commands as part of v0.5.8 implementation, confirm their inclusion and exact CLI namespace in the approval checkpoint; they were not part of the final Product Sync CLI decisions.

If included by explicit approval:

- Implement `check` as read-only validation with actionable errors/warnings.
- Implement `setup` as proposed changes, with explicit confirmation **and** `--commit` before writing Invoice Ninja configuration.
- Validate the four established Product custom fields and canonical Supplier-to-active-Vendor resolution.
- Run a lightweight read-only configuration preflight before Product Sync.
- Do not change native `ninja import/export` or create a new accounting model.

Do not silently expand the slice to include this separate feature.

## 7. Historical reviews - deliberate second phase

After the feature changes are agreed, work through [`Version-Fixes.md`](./Version-Fixes.md). The historical review comments **are evidence to verify, not instructions to patch everything**.

- Start with the unresolved EOFY and Financial export findings, plus the Bunnings Web finding relevant to the mode selection.
- Compare each comment with **current `main`**, current tests, later fixes, and any explicit owner decisions.
- Record `verified fixed`, `confirmed defect`, `owner-deferred`, or `not applicable`, including evidence.
- Propose only confirmed, in-scope fixes; ask before making unrelated accounting changes.
- Do not reopen owner-waived legacy migrations or re-implement work already addressed.
- Do not equate a resolved GitHub review thread with independently verified code correctness.

## 8. Focused verification

Tests should cover at least:

- No-argument `sync` opt-in selection; explicit provider and single-product selection.
- Preview versus `--commit`, failure handling, and untouched `ninja import/export` commands.
- Bunnings API/Web mode switching under one identity; no duplicate Product creation.
- Bunnings refresh of known Products only; explicit key fetch; provider Vendor resolution.
- CSV/XLSX reading over HTTP(S), public FTP, and `file://`; malformed/unavailable sources.
- NST real column headings, inclusive retail price mapping, and Description templating.
- Normalised template fields (`description`, `category`, `unit`, `supplier`), optional missing values, and invalid tag errors.
- Product identity, aliases, existing sync-date/error safeguards, and no automatic deletion.
- Regression tests for any **confirmed and approved** review fixes.

Run `gofmt`, `go vet`, `go test`, and `go build` in each affected Go module; report anything not runnable and why.

## 9. Implementation and handoff rules

- Keep changes small and reuse existing source adapters and synchronisation logic.
- Maximum **three** critical review/fix cycles against the actual diff, with evidence of findings and test results.
- Any extra dependency, persistent state, wider refactor, or new feature needs a fresh approval checkpoint.
- Update release documentation, v0.5 roadmap entry, CLI examples, and tests to match **implemented**, not merely proposed, behaviour.
- Never push directly to `main`; create the agreed branch from latest `origin/main` after approval and publish to that branch explicitly.
- Report base/branch/commits, verification, review outcomes, fixes and unresolved items.
- **STOP for user approval before merging.**

**Scope discipline:** v0.5.8 makes Product Sync usable for real suppliers without turning a BAS deadline into an enterprise software programme.
