
# Changes

## Unreleased

## v0.5.7

- Added preview-by-default Product Sync across configured Bunnings API and
  generic CSV/XLSX Providers.
- Adopted native `(vendor_id, product_key)` Product identity and native
  `product_image`, with collision-safe in-place migration of legacy Bunnings
  Products.
- Fixed Product custom fields as Store, Not Available, Supply Unit and Last
  Sync Date while preserving arbitrary Supply Unit text.
- Added oldest-first Product refresh, explicit available/not-available/error
  handling and per-Product successful sync dates.
- Added content-hash freshness for file catalogues; successful fingerprints
  advance only after every required Product succeeds, so partial runs retry the
  complete catalogue.
- NST-specific mappings and live verification of Invoice Ninja native Product
  fields remain required pre-merge checks.

## v0.5.6

- Added `ninja export financial` for unrestricted, financial-year/period and
  explicit source-date range XLSX exports.
- Preserved raw invoice, expense, payment, supplier transaction and related
  master-data fields while reusing the shared Accounting Dataset for calculated
  facts and exceptions.
- Added deterministic filenames, overwrite protection and diagnostic
  `INCOMPLETE` workbooks when date-filtered records have missing or invalid
  source dates.

## v0.5.5

- Added `ninja export eofy` with explicit or most-recently-completed financial
  year selection, deterministic filenames and `--force` overwrite safety.
- Reused the shared Accounting Dataset for independent cash/accrual EOFY
  recognition and source/category traceability.
- Added accountant-review XLSX sheets for income, expenses, capital assets,
  GST reconciliation, exceptions and supporting detail.
- Preserved `Capital Check` classifications and used Expense purchase date as
  the explicit first-used date assumption. Asset threshold testing removes only
  claimable GST before applying business use, and exact-threshold items remain
  capital/depreciation review items.
- Missing evidence that can alter EOFY figures or asset classification produces
  an `INCOMPLETE` diagnostic workbook and exit 1.

## v0.5.4

- Added `ninja export bas` with monthly, quarterly and yearly financial-year
  selection, deterministic filenames and `--force` overwrite safety.
- Added a shared in-memory Accounting Dataset with source identity, customer
  payment allocations, supplier FIFO settlement and accounting exceptions.
- Added cash and accrual GST recognition, proportional partial-payment
  allocation using integer cents, and traceable Summary, Sales, Purchases and
  Exceptions workbook sheets.
- BAS accounting errors produce an `INCOMPLETE` diagnostic workbook and exit 1;
  warnings remain successful.
- The no-flag default selects the most recently completed reporting period by
  period end date, including the previous financial year at FY boundaries.

## v0.5.3

- Replaced optional flat configuration with mandatory strict YAML at
  `~/.GoTradie/config.yaml`; removed `--config`, `GOTRADIE_CONFIG` and generic
  environment overrides while retaining only the two supported secret overrides.
- Added hierarchical Bunnings, Product Sync, BAS, EOFY and export settings with
  complete monthly, quarterly and yearly BAS cadence validation.
- Preserved existing `BUNNINGS-` Product keys, lookup, synchronisation, import,
  `bunnings_in` and `image_url` behaviour through a temporary internal constant.
- Added deterministic Provider alias resolution and kept Store as separate
  Expense metadata rather than part of Vendor identity.
- Retired the ERPNext migration exporter and its configuration, tests and docs.

## v0.5.2

- Implemented the v0.5.2 Expense-import UX with required durable `Import ID`
  identity, named-file-only input and complete local preflight before remote
  reads or writes.
- Added arithmetic and suspicious-duplicate warnings, live per-row progress,
  richer error context, truthful preview states and final summaries.
- Added configurable sequential batches and optional pauses for ordinary
  Expense-only files while retaining whole-file supplier settlement whenever
  Account Payment rows are present.
- Added correction-by-`Import ID`, legacy v0.5.1 identity migration and
  content-derived receipt deduplication with durable owner markers.
- Existing Account Payments now reject Supplier, Payment Type or Payment
  Reference drift before source values can affect durable settlement.
- Receipt owners remain valid when identical local content is renamed; the
  historical Invoice Ninja attachment filename is no longer treated as identity.
- Preserved integer-cent FIFO settlement, stopped settlement execution after a
  failed prerequisite write, and added no checkpoint database or concurrent
  mutation path.

## v0.6 planning docs

- Added supplier import overview document.
- Added supplier import implementation prompt document.
- Captured v0.6.x staged implementation plan for supplier CSV/XLSX price list import.
- Captured proposed `supplier` command family: `init`, `check`, `import`, and `list`.
- Captured YAML profile model using `input`, `aliases`, and `fields`.


## v0.5.1

- Added preview-by-default `ninja import expenses` for the master purchase ledger.
- Added recursive receipt indexing through `--receipts-root`; `File Name` matches are exact and missing or ambiguous matches are reported.
- Expense imports preserve source-row idempotency, leave `should_be_invoiced` unset, and name vendors from Supplier plus Store.
- Expense imports derive a missing Business Amount from the total and percentage without replacing an explicit zero.
- Numeric Job Number values reuse an existing project or create one from the matching master quote client.
- Expense imports now enforce canonical Document Type values and exact Invoice Ninja Payment Type labels.
- Invoice and Receipt rows now preserve paid/unpaid timing, while Account Payment rows create idempotent withdrawal Transactions in the existing manual `GoTradie` bank account and allocate by supplier, purchase date, and stable source order without creating fake expenses or customer Payments.
- Supplier settlement fails before writes when the `GoTradie` account is missing, ambiguous, remote-backed, archived/deleted, sync-enabled, or exposed to an active auto-convert DEBIT rule.
- Partial supplier-account allocations remain unpaid in Invoice Ninja, mixed payment methods are retained in allocation detail, and unapplied payment remainders are reported.
- Purchase source identity now ignores Document Type, Payment Type and analytical corrections while retaining exact legacy-marker migration support.
- Adjustment rows are explicitly deferred instead of receiving invented accounting behaviour.
- Supplier payments now stop at invalid or missing older imported purchases instead of allocating around indeterminate account history.
- Commit imports fail before any writes when Invoice Ninja could email a vendor after an expense is marked paid.
- Added hidden `ninja export tax [directory] [--commit]` command for Mick's accounting workflow.
- Produces `Invoices.csv` and `Detail.csv` without changing the existing invoice or payment export contracts.
- Reuses existing Invoice Ninja invoice/payment pagination and line-item models.
- Keeps multiple payment dates when an invoice has partial payments across dates.
- Intentionally isolated: this exists to avoid adding two kitchens to an otherwise well-designed carport.
- Tax export now writes `Customers.csv`, references customers by ID from `Invoices.csv`, and restores the leading `Type` column in `Detail.csv`.


## v0.5

- Added `--web` support for Bunnings-backed commands (`bunnings get|lookup|find`, `sync refresh|import|search`) to select website-derived Bunnings data with no silent API fallback.
- Aligned app version metadata/docs to `v0.5` and updated SDK dependency targets to `GoBunnings v0.5.0` and `GoInvoiceNinja v0.5.0`.
- Added `bunnings` command namespace with `find`, `get`, and `lookup`.
- Added top-level `commands` extended help, with `extended-help` and `manual` aliases, covering each command with example output.
- Retired advertised top-level `search`; moved guarded cross-system behavior under `sync search` and introduced `sync refresh` / `sync import`.
- Kept compatibility for legacy `add-in` command with deprecation notice.
- Consolidated write/overwrite confirmation onto one `--commit` flag: commands preview or refuse risky writes by default, and only persist changes when `--commit` is supplied.


## v0.4

- Added `CHATGPT_CONTEXT.md` to document the app role, dependency boundaries, Go version, and local multi-repo workflow.
- Added `ECOSYSTEM.md` documenting the three-repo workspace structure, dependency direction, local `go.work` setup, release order, and boundary checklist.
- Removed committed absolute-path `replace` directives from `go.mod`.
- Updated `README.md` to describe local development with a parent-folder `go.work` workspace instead of machine-specific `replace` paths.
- Updated `README.md` to describe local development with a parent-folder `go.work` workspace.
- Preserved relative local `replace` directives in `go.mod` for simple single-machine development.


## v0.3

- Updated the project version to `v0.3`.
- Kept the zip as client-only; `GoBunnings` and `GoInvoiceNinja` remain external local dependencies via `go.mod` `replace` directives.
- Kept `go 1.22`.
- Updated Invoice Ninja product and client listing to use `GoInvoiceNinja` v0.2 `ListAll` pagination helpers.
- Added full-page exports for Invoice Ninja quotes, invoices, and payments.
- Changed the Invoice Ninja CSV command layout from flat command names to grouped subcommands:
  - `ninja export products <file|->`
  - `ninja import products <file|->`
  - `ninja export clients <file|->`
  - `ninja import clients <file|->`
  - `ninja export quotes <file|->`
  - `ninja export invoices <file|->`
  - `ninja export payments <file|->`
- Removed the `--out` flag from export commands.
- Export commands now take `-` for stdout or a filename for file output.
- Export commands refuse to overwrite existing files unless the explicit commit flag is supplied.
- Import commands now take `-` for stdin or a filename for file input.
- Import commands now fail clearly if the requested import file does not exist.
- Kept product and client imports as preview-by-default operations.
- Marked quote, invoice, and payment CSV handling as export-only.


## v0.1

- Created standalone `GoTradie` client project.
- Added CLI entrypoint at `cmd/GoTradie`.
- Added Bunnings-to-Invoice Ninja sync commands:
  - `sync`
  - `add-in`
  - `search`
- Added preview-by-default behaviour for write operations.
- Added guarded Bunnings search imports to avoid accidental bulk product creation.
- Added config file support with file values overriding environment variables.
- Added initial Invoice Ninja product and client CSV export/import commands.
- Added `README.md`.
- Added `VERSION`.
- Added `CHANGES.md` retrospectively.
- Added `WEIRD_STUFF.md` with integration notes and prompts for upstream package fixes.
- Set `go 1.22`.
- Added local filesystem `replace` directives for `GoBunnings` and `GoInvoiceNinja`.
