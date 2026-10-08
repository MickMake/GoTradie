# GoTradie v0.5.5 — Implementation Prompt

**Objective:** Implement [EOFY export](./EOFY-Export.md) as the authoritative feature design.

**Implementation Workflow:** [Implementation Workflow](../../Implementation-Workflow.md)

**Suggested branch:** `v0.5.5-eofy-export`

## Common implementation rules

- Follow the global [Implementation Workflow](../../Implementation-Workflow.md) for implementation workflow.

## Slice-specific implementation work

- Inspect the v0.5.4 Accounting Dataset, Invoice Ninja expense categories and available asset evidence.
- Reuse shared accounting facts rather than building a second recognition or allocation engine.
- Generate the accountant-review workbook as specified in the design. Do not implement depreciation or pooling.

## Focused acceptance evidence

Test independent BAS/EOFY accounting bases; selected financial year; source/category traceability; no/full/partial GST credit entitlement; business-use treatment; first-used date and threshold boundary; exception severity; deterministic filenames and overwrite safety.

**Open question:** The design does not fully specify how to classify an asset when first-used date, GST entitlement or business-use evidence is unavailable. Flag it in preflight rather than inventing values or eligibility.
