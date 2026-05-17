# Changes

## v0.5

- Aligned app version metadata/docs to `v0.5` and updated SDK dependency targets to `GoBunnings v0.5.0` and `GoInvoiceNinja v0.5.0`.
- Added `bunnings` command namespace with `find`, `get`, and `lookup`.
- Retired advertised top-level `search`; moved guarded cross-system behavior under `sync search` and introduced `sync refresh` / `sync import`.
- Kept compatibility for legacy `add-in` command with deprecation notice.


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
- Export commands now refuse to overwrite existing files unless `--force` is supplied.
- Import commands now take `-` for stdin or a filename for file input.
- Import commands now fail clearly if the requested import file does not exist.
- Added a reusable `--force` pattern for commands that need explicit overwrite behaviour.
- Kept product and client imports as dry-run by default.
- Marked quote, invoice, and payment CSV handling as export-only.

## v0.1

- Created standalone `GoBunningsNinja` client project.
- Added CLI entrypoint at `cmd/bunnings-ninja`.
- Added Bunnings-to-Invoice Ninja sync commands:
  - `sync`
  - `add-in`
  - `search`
- Added dry-run defaults for write operations.
- Added guarded Bunnings search imports to avoid accidental bulk product creation.
- Added config file support with file values overriding environment variables.
- Added initial Invoice Ninja product and client CSV export/import commands.
- Added `README.md`.
- Added `VERSION`.
- Added `CHANGES.md` retrospectively.
- Added `WEIRD_STUFF.md` with integration notes and prompts for upstream package fixes.
- Set `go 1.22`.
- Added local filesystem `replace` directives for `GoBunnings` and `GoInvoiceNinja`.
