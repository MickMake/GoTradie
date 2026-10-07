# GoTradie v0.5 Design Series

This directory groups design and closeout material for the GoTradie `v0.5.x` release series.

## Releases

| Version | Status | Scope |
|---|---|---|
| v0.5.1 | Closed | Historical Expense import and supplier-account settlement |
| v0.5.2 | Design drafted | Expense import identity, preflight, UX, batching and receipt deduplication |
| v0.5.3 | Planned | Hierarchical YAML configuration |
| v0.5.4 | Planned | Accounting Dataset and BAS XLSX export |
| v0.5.5 | Planned - implementation-ready | EOFY preparation XLSX export |
| v0.5.6 | Planned - implementation-ready | Financial data XLSX export |
| v0.5.7 | Planned | Product synchronisation and provider/catalogue sources |

## Working rule

Each version directory owns one implementation slice. Closed slices remain historical contracts; new feature work moves to the next slice rather than quietly extending old work.

Cross-release contracts such as `Command-Line-Spec.md` remain at `docs/Design/`.

## v0.5.3

Replace flat `key=value` configuration with hierarchical YAML. Keep legacy flat config readable for this transition release. Do not absorb later feature implementation.

## v0.5.4

Introduce the shared **Accounting Dataset** and BAS XLSX output. With no period flags, BAS exports the most recently completed BAS cycle.

## v0.5.5

EOFY preparation workbook using the Accounting Dataset. With no period flags, EOFY exports the most recently completed financial year. Existing Invoice Ninja Expense Categories are authoritative. Capital/asset review may use a configurable instant asset write-off threshold, but GoTradie does not calculate depreciation.

## v0.5.6

A deliberately simple financial-data workbook. With no period flags, export all available financial data. Optional `--fy` or `--from/--to` restrict the dump. First version is raw data/diagnostics, not dashboards.

## v0.5.7

Redesign Product Sync around one rule:

> Sync known Invoice Ninja Products first. Then look for products that are missing.
