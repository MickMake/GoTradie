# GoTradie v0.5.7 — Implementation Prompt

**Objective:** Implement [Product synchronisation](./Product-Sync.md) as the authoritative feature design.

**Implementation Workflow:** [Implementation Workflow](../../Implementation-Workflow.md)

**Suggested branch:** `v0.5.7-product-sync`

## Common implementation rules

- Follow the global [Implementation Workflow](../../Implementation-Workflow.md) for implementation workflow.

## Slice-specific implementation work

- Inspect current Product Sync and Bunnings error/availability handling before redesigning it.
- Verify the actual Invoice Ninja Product custom-field API representation and safe read/write behaviour.
- Inspect the actual NST CSV and source metadata before assuming field names or parsing rules.
- Implement Provider adapters, existing-product refresh, evidence-based discovery and narrow source-fingerprint caching to the design; do not create a durable parallel product catalogue.
- Preserve preview-by-default behaviour and avoid unrelated historical Vendor identity migration.

## Focused acceptance evidence

Test preview versus `--commit`; all configured Providers; canonical Supplier/Product matching, aliases and Store separation; ordering and per-product successful sync dates; known availability versus unknown/error; file fetch/parse once; content-hash change/unchanged handling; source errors and retry; and duplicate prevention.

**Open question:** The design does not clearly define whether a changed file's successful fingerprint is committed only after all affected Products are processed. Flag partial-failure/retry semantics before implementing the cache to prevent skipped Products.
