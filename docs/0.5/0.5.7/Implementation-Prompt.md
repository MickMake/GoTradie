# GoTradie v0.5.7 — Implementation Prompt

**Objective:** Implement [Product synchronisation](./Product-Sync.md) as the authoritative feature design.

**Implementation Workflow:** [Implementation Workflow](../../Implementation-Workflow.md)

**Suggested branch:** `v0.5.7-product-sync`

## Common implementation rules

- Follow the global [Implementation Workflow](../../Implementation-Workflow.md).
- Run preflight against the current repository and current `origin/main`, checking for unmerged PRs. State intended code changes and branch, and stop for approval before editing code.
- Preserve preview-by-default behaviour; only `--commit` mutates Invoice Ninja.
- Keep the implementation focused and avoid unrelated Vendor or historical accounting migrations.

## Decisions settled for this slice

- Product identity is `(Supplier, product_key)`, where Supplier is the canonical Provider name stored in Product `custom_value1`.
- Native Product `vendor_id` is not used for identity, matching or persistence. Existing Invoice Ninja Vendor lookup remains a required validation step for each Provider on every invocation.
- Product images use native `product_image`; the installed Invoice Ninja instance supports external image URLs.
- Four Product custom fields: **Supplier**, **Store**, **Not Available**, **Last Sync Date** (in that order).
- Supply Unit is removed from Product Sync. GoTradie has not previously populated Invoice Ninja Products, so no legacy Product migration is required.
- For CSV/XLSX Providers, persist the new successful content hash **only after 100% of required Product updates succeed**. On partial failure, leave the old successful hash and retry the entire file on the next run. No incremental per-product checkpoint is needed.
- Successful fingerprints include canonical Supplier name, configured aliases and source field mappings, but not resolved Vendor IDs.

## Slice-specific implementation work

- Inspect current Product Sync, Product API read/write support, and Bunnings availability/error handling.
- Preserve native `product_image` support and validate each configured Supplier against one unambiguous active Invoice Ninja Vendor without modifying Vendor records.
- Inspect actual NST CSV/source metadata before assuming field names or parsing rules.
- Implement Provider adapters, existing-Product refresh, evidence-based missing-Product discovery and the narrow source-fingerprint cache described in the design.
- Preserve Provider canonical names and aliases, normalise recognised aliases on successful Product updates, and keep Store separate from Supplier identity.
- Use the existing Invoice Ninja Product records as the authoritative Product state, not a second catalogue.
- Update CLI help, tests and release documentation for the migration.

## Focused acceptance evidence

Test preview versus `--commit`; all configured Providers; `(Supplier, product_key)` identity; same-key/different-Supplier separation; duplicate rejection; alias recognition and canonicalisation; blank/unknown Supplier isolation; required Vendor validation, including unchanged files; final custom-field allocation; native image URL preservation; oldest-first successful Product sync dates; available versus not available versus unknown/error; CSV/XLSX parse-once behaviour; mapping/Supplier/alias fingerprint invalidation; unchanged and changed content hashes; **partial failure without hash advancement and full-file retry**; source errors and duplicate prevention.
