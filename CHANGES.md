# Changes

## 0.1 - Initial client release

### Added retrospectively from first generated zip
- Created standalone `GoBunningsNinja` client project.
- Added CLI entrypoint at `cmd/bunnings-ninja`.
- Added `sync` command to refresh existing Invoice Ninja products linked to Bunnings item numbers.
- Added `add-in <IN>` command to add or refresh one Invoice Ninja product by Bunnings item number.
- Added `search <query>` command to preview Bunnings search results and optionally import selected products.
- Added search import guardrails:
  - preview-only by default;
  - `--dry-run=true` by default;
  - hard search import cap handled by the Bunnings service;
  - requires `--select=...` or `--all --yes` for imports.
- Wired `GoBunnings` and `GoInvoiceNinja` as dependencies.
- Added `README.md` project usage notes.
- Added `WEIRD_STUFF.md` with observations and copy/paste fix prompts for the API package chats.

### Changed in this zip
- Removed bundled copies of `GoBunnings` and `GoInvoiceNinja` from the deliverable zip.
- Set Go version to `1.22`.
- Added local filesystem `replace` directives:
  - `github.com/MickMake/GoBunnings => /Volumes/home/mick/Documents/GoLang/Tradie/GoBunnings`
  - `github.com/MickMake/GoInvoiceNinja => /Volumes/home/mick/Documents/GoLang/Tradie/GoInvoiceNinja`
- Added `VERSION` file with version `0.1`.
- Added `gobunningsninja.conf.example`.
- Added config file support via `--config`, `GOBUNNINGSNINJA_CONFIG`, or default `./gobunningsninja.conf`.
- Config file values override environment variables.
- Split configuration validation so `ninja-*` CSV commands only require Invoice Ninja config, while Bunnings sync/search commands still require Bunnings config.

### Added CSV commands
- Added `ninja-products-export` to export Invoice Ninja products as CSV with columns:
  - `ID`
  - `Product`
  - `Description`
  - `Price`
  - `Default Quantity`
  - `Max Quantity`
  - `Image URL`
- Added `ninja-products-import` to read the same CSV and update changed product fields.
- Added `ninja-clients-export` to export Invoice Ninja clients as CSV with columns:
  - `ID`
  - `Name`
  - `Address`
  - repeated contact columns: `Contact N First Name`, `Contact N Last Name`, `Contact N Email`, `Contact N Phone`
- Added `ninja-clients-import` to read the same CSV and update changed client/contact fields.
- CSV imports default to `--dry-run=true`; pass `--dry-run=false` to apply changes.

### Notes
- `Image URL` is stored using the configured Invoice Ninja product custom field, default `custom_value2`.
- `Max Quantity` is exported as a blank column and currently ignored on import because the uploaded `GoInvoiceNinja` product model does not expose a max quantity field.
- Client `Address` exports a combined billing address. On import, changed values are written to `address1` while preserving other structured address fields where possible.
