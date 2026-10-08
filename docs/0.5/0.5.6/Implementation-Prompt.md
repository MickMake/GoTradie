# GoTradie v0.5.6 — Implementation Prompt

**Objective:** Implement [Financial data export](./Financial-Export.md) as the authoritative feature design.

**Implementation Workflow:** [Implementation Workflow](../../Implementation-Workflow.md)

**Suggested branch:** `v0.5.6-financial-export`

## Common implementation rules

- Follow the global [Implementation Workflow](../../Implementation-Workflow.md) for implementation workflow.

## Slice-specific implementation work

- Inspect Invoice Ninja entity access, shared Accounting Dataset, CLI selection code and XLSX support.
- Export raw source records and necessary relationships alongside shared calculated facts; avoid a duplicate accounting engine.
- Keep this a diagnostic/raw-data export, not a reporting dashboard.

## Focused acceptance evidence

Test unrestricted export; FY/period and explicit range selectors, including invalid combinations; source-date filtering by entity; records crossing date boundaries; missing dates; related master-data retention; source fidelity; output naming and overwrite protection.

**Open question:** Confirm the default financial year when `--period` is supplied without `--fy`, and the exact mapping of period selectors under different BAS reporting cadences. Resolve any ambiguity in the design, not here.
