# GoTradie v0.5.4 — Implementation Prompt

**Objective:** Implement [BAS export and shared Accounting Dataset](./BAS-Export.md) as the authoritative feature design.

**Implementation Workflow:** [Implementation Workflow](../../Implementation-Workflow.md)

**Suggested branch:** `v0.5.4-bas-export`

## Common implementation rules

- Follow the global [Implementation Workflow](../../Implementation-Workflow.md) for implementation workflow.

## Slice-specific implementation work

- Inspect current Invoice Ninja invoices, customer payments, expenses, supplier transactions/settlements, configuration and XLSX libraries.
- Implement the minimum shared Accounting Dataset in this slice; keep source identity and accounting facts independent from workbook presentation.
- Build BAS export using that shared dataset and the authoritative selection, calculation, output and exception rules.
- Do not implement EOFY or Financial exports in this slice.

## Focused acceptance evidence

Use deterministic examples for period/FY selection, boundary dates, cash versus accrual recognition, partial customer receipts, partial supplier settlement and FIFO allocation, GST rounding, missing/ambiguous evidence, source traceability, workbook totals, incomplete-report status and overwrite safety.

**Design blocker:** The accepted BAS design still describes ATO due-date verification/cache and selection rules. Resolve that document against the agreed config-driven BAS/reporting/submission dates **before coding**. Do not implement both approaches or decide the selection rule inside this prompt.
