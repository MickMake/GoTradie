# GoTradie v0.5.8 - Implementation Prompt

**Objective:** Implement the agreed [Product Sync Updates](./Product-Sync-Updates.md) as the authoritative v0.5.8 feature design. This is **primarily a configuration and CLI integration slice**, not a Bunnings feature expansion.

**Implementation Workflow:** [Implementation Workflow](../../Implementation-Workflow.md)

**Suggested branch:** `v0.5.8-provider-sync` (from fresh `origin/main` after approval).

## Common implementation rules

- Follow the global [Implementation Workflow](../../Implementation-Workflow.md). Its preflight, explicit approval before branch creation/code changes, git safeguards, verification, maximum three review/fix cycles, and merge-approval checkpoint are mandatory; do not duplicate or override them here.
- Read [Product Sync Updates](./Product-Sync-Updates.md), the existing [v0.5.7 Product Sync design](../0.5.7/Product-Sync.md), and the [global CLI contract](../../Command-Line-Spec.md). The v0.5.8 design supersedes v0.5.7 only for the explicitly changed behaviours.
- This slice's **only** authoritative scope is the versioned release documentation linked here. Do not search for additional design requirements elsewhere or infer approval from a historical review comment.

## Decisions settled for this slice

- Target CLI is `GoTradie sync [provider] [product-key] [--commit]`, with `auto_sync: false` by default and dry-run by default. Do **not** add `--provider`; leave native `ninja import/export` unchanged.
- Configure **one** Bunnings provider with `mode: api | web` to select an existing adapter. Do not add a second Bunnings provider, new API calls, web scrapers or mode-specific caching.
- For all providers, optional `description_template` uses one shared Go `text/template` renderer over normalised, lowercase GoTradie fields. Default is exactly `{{.description}}`.
- Bunnings `.description` is currently `description.productDescription`, with fallback to `description.sectionDescription`. Do not prepend Title or append `Bunnings IN`; the item number is already the native Product key.
- Extend file-provider `fields` to allow additional mapped template inputs (NST `category`, `unit`) without extra Invoice Ninja Product fields. Required Product/Description/GST-inclusive Price and supplier identity remain as specified in the design.
- Keep CSV/XLSX decoding independent of HTTP(S), public FTP and local `file://` retrieval. Preserve existing successful file-source fingerprint behaviour; changing a template must reprocess unchanged file content.
- Do not add exclusive-price GST conversion or modify Invoice Ninja Tax behaviour. Use NST's existing GST-inclusive price source.
- Rich product lookups, barcodes, dimensions, weights, manufacturers, documentation links, inventory and alternatives are **future ideas**, not v0.5.8 work.

## Slice-specific implementation work

- Inspect current YAML parsing/validation, Product Sync commands, file providers, existing Bunnings API/Web adapters, Product update path and tests before proposing changes.
- During the mandatory preflight present the smallest concrete change set, proposed branch/base SHA, verification, and any FTP dependency. Stop for explicit approval before creating a branch or changing implementation code.
- Make the smallest necessary changes to configuration types/validation, CLI argument handling, the shared Description renderer and the existing adapters' wiring. **No unrelated refactoring** of GoBunnings, GoInvoiceNinja, Invoice Ninja tax, Bunnings lookup or Product Sync identity.
- Preserve source parsing, Product identity, Vendor validation, availability, Price, image, Last Sync Date, duplicate protection and partial-failure safeguards.
- Update CLI help, versioned release documentation and focused tests for implemented behaviour. Keep source adapters unchanged except for the narrowly required Description/mode integration.
- If the current implementation or API cannot support an agreed requirement without broader work, **STOP and present that obstacle** instead of inventing a workaround or expanding the slice.

## Focused acceptance evidence

- Provider selection: `sync` without provider, explicit provider, one product; `auto_sync` opt-in; preview versus `--commit`.
- File catalogue create/update versus Bunnings existing-only refresh; supplier/product identity, aliases, duplicate rejection and Vendor validation.
- Single Bunnings mode switch (`api | web`) reuses existing adapters with no Product duplication; normal Bunnings commands and Invoice Ninja maintenance remain unchanged.
- The default Bunnings Description is only the existing Description value (`productDescription`, falling back to `sectionDescription`); no Title or appended I/N.
- Common Description templates for API, Web, CSV and XLSX; dynamic source fields; optional values; conditional formatting; unknown-tag errors.
- NST's actual `CATEGORY`, `CODE`, `DESCRIPTION`, `UNIT` and `PRICE (INC GST)` headings; no new GST calculation or Invoice Ninja Tax mutation.
- File fingerprint reprocessing when `fields` or `description_template` changes, and no advancement on partial failure.
- CSV/XLSX retrieval over HTTP(S), public FTP and `file://`, including source failure tests.
- Evidence from `gofmt`, `go vet`, `go test`, `go build` for affected modules; critical review of the actual diff and a concise list of out-of-scope changes **not** made.
