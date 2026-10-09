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

- Product identity is `(vendor_id, product_key)` using native Invoice Ninja Product fields; no `Supplier` custom field.
- Product images use native `product_image`; the installed Invoice Ninja instance supports external image URLs.
- Four Product custom fields: **Store**, **Last Sync Date**, **Not Available**, **Supply Unit** (in that order).
- `Supply Unit` is **arbitrary free-form text**. Do not propose or implement a formatting convention, validation, unit parsing or conversion.
- Safely migrate existing `BUNNINGS-` keys, `bunnings_in` and `image_url` custom mappings, preserving existing Product records and image data. Do not create duplicate Products.
- For CSV/XLSX Providers, persist the new successful content hash **only after 100% of required Product updates succeed**. On partial failure, leave the old successful hash and retry the entire file on the next run. No incremental per-product checkpoint is needed.

## Slice-specific implementation work

- Inspect current Product Sync, Product API read/write support, and Bunnings availability/error handling.
- Verify the installed Invoice Ninja Product API round-trips native `vendor_id` and `product_image` correctly.
- Inspect actual NST CSV/source metadata before assuming field names or parsing rules.
- Implement Provider adapters, existing-Product refresh, evidence-based missing-Product discovery and the narrow source-fingerprint cache described in the design.
- Preserve Provider canonical names and aliases, and keep Store separate from Vendor identity.
- Use the existing Invoice Ninja Product records as the authoritative Product state, not a second catalogue.
- Update CLI help, tests and release documentation for the migration.

## Focused acceptance evidence

Test preview versus `--commit`; all configured Providers; `(vendor_id, product_key)` identity, aliases and Store separation; legacy Bunnings Product migration without duplication; native image URL preservation; arbitrary Supply Unit text; oldest-first successful Product sync dates; available versus not available versus unknown/error; CSV/XLSX parse-once behaviour; unchanged and changed content hashes; **partial failure without hash advancement and full-file retry**; source errors and duplicate prevention.
