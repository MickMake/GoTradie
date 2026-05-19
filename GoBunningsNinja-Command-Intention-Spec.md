# GoBunningsNinja Command Intention Spec

Version: `v0.5`  
Status: Draft command contract / behaviour lock-in  
Primary repository: `MickMake/GoBunningsNinja`

This document defines the intended command-line behaviour for `bunnings-ninja`.

The goal is to prevent accidental behavioural drift, flag multiplication, and future "creative adjustments" where the CLI grows a second head and starts asking for biscuits.

---

## 1. Repository ownership rules

`GoBunningsNinja` is the CLI/application that composes:

- `GoBunnings`
- `GoInvoiceNinja`

### `GoBunnings` owns

- Bunnings API authentication
- Bunnings transport/client logic
- Bunnings request/response models
- Bunnings product/search/pricing/inventory helpers
- Bunnings-specific parsing and error handling

### `GoInvoiceNinja` owns

- Invoice Ninja API authentication
- Invoice Ninja transport/client logic
- Invoice Ninja request/response models
- Invoice Ninja services
- Pagination helpers
- Upload helpers
- Invoice Ninja API error handling

### `GoBunningsNinja` owns

- CLI commands
- Flag parsing
- Config loading
- User-facing workflow behaviour
- Sync/import/export orchestration
- Bunnings-to-Invoice Ninja mapping
- Preview/default safety behaviour
- `--commit` handling for persistent changes

Cross-system behaviour belongs in `GoBunningsNinja`, not in either SDK.

---

## 2. Global command rules

| Rule | Intention |
|---|---|
| No command should make remote Invoice Ninja changes by default. | Default behaviour is preview/safe mode. |
| `--commit` is the only flag that makes Invoice Ninja updates real. | Avoid `--apply`, `--force`, `--dry-run`, and other synonym-goblins. |
| Export commands may create a new file by default. | Creating a new named export is safe enough. |
| Export commands must not overwrite an existing file unless `--commit` is supplied. | Prevent accidental file clobbering. |
| Bunnings SDK commands are read-only. | They discover or fetch Bunnings data only. |
| `GoBunningsNinja` owns mapping, sync decisions, import/export workflow, and CLI safety. | Do not move orchestration into the SDK repos. |

### Standard wording

Use this wording consistently in CLI help, README, and future docs:

```text
Commands preview or refuse risky writes by default.
Use --commit to make persistent changes.
```

### Removed/rejected flags

These should not be reintroduced casually:

```text
--dry-run
--apply
--force
```

Replacement:

```text
--commit
```

Meaning:

> Make the persistent change that would otherwise be previewed or refused.

For Invoice Ninja workflows, `--commit` means API updates.  
For exports, `--commit` means overwriting an existing local file.

---

## 3. Global options

### `--config <path>`

Example:

```bash
bunnings-ninja --config ./gobunningsninja.conf ninja export products products.csv
```

#### User intention

Use a specific configuration file.

#### Programming intention

Configuration should be loaded in this order:

1. `--config <path>`
2. `GOBUNNINGSNINJA_CONFIG`
3. `./gobunningsninja.conf`, if present

Config file values override environment variables.

#### Must not

- Change command behaviour beyond configuration loading.
- Imply `--commit`.
- Be required for commands that do not need config, such as `version`.

---

## 4. Bunnings commands

Bunnings commands are read-only. They must not require Invoice Ninja credentials and must not modify Invoice Ninja.

---

## `bunnings find <query>`

Example:

```bash
bunnings-ninja bunnings find "merbau decking" --limit=10
```

### User intention

Search Bunnings for products using a fuzzy/natural-ish query and return machine-readable CSV.

### Programming intention

This is a read-only discovery command.

It should:

1. Validate Bunnings configuration.
2. Create a `bunnings.Service`.
3. Search Bunnings with the query string.
4. Hydrate results where possible.
5. Print CSV rows:

```text
IN,Description,Unit,PricePerUnit,ImageURL
```

### Must not

- Create Invoice Ninja products.
- Update Invoice Ninja products.
- Require Invoice Ninja credentials.
- Accept `--commit`.
- Do clever cross-system matching.

This is pure Bunnings discovery. A shovel, not a combine harvester.

### Flags

| Flag | Meaning |
|---|---|
| `--limit=N` | Maximum search results. Should remain capped. |

---

## `bunnings get <IN...>`

Example:

```bash
bunnings-ninja bunnings get 0123456 0987654
```

### User intention

Fetch one or more exact Bunnings item numbers and print CSV.

### Programming intention

This is a read-only exact lookup command.

It should:

1. Validate Bunnings configuration.
2. For each supplied Bunnings item number, call exact product lookup.
3. Output CSV rows.
4. Include errors as row data rather than stopping the whole command, where practical.

### Must not

- Search fuzzily.
- Modify Invoice Ninja.
- Require Invoice Ninja credentials.
- Accept `--commit`.

---

## `bunnings lookup <IN...>`

Example:

```bash
bunnings-ninja bunnings lookup 0123456
```

### User intention

Fetch one or more exact Bunnings products and display them in a human-readable format.

### Programming intention

This is the human-readable sibling of `bunnings get`.

It should:

1. Validate Bunnings configuration.
2. Fetch each exact item number.
3. Print detail blocks, not CSV.

Expected detail shape:

```text
IN: 0123456
Title: ...
Description: ...
Unit: ...
PricePerUnit: ...
ImageURL: ...
```

### Must not

- Create or update Invoice Ninja.
- Produce CSV.
- Accept `--commit`.

---

## 5. Sync commands

Sync commands orchestrate Bunnings and Invoice Ninja together. They require both SDKs and belong in `GoBunningsNinja`.

---

## `sync refresh`

Examples:

```bash
bunnings-ninja sync refresh
bunnings-ninja sync refresh --commit
```

Also allowed as shorthand:

```bash
bunnings-ninja sync
```

### User intention

Refresh existing Invoice Ninja products that are already linked to Bunnings item numbers.

### Programming intention

This is a cross-system sync command.

It should:

1. Validate both Bunnings and Invoice Ninja configuration.
2. Load existing Invoice Ninja products.
3. Identify products linked to Bunnings item numbers using the configured custom field.
4. Fetch current Bunnings product data.
5. Compare Bunnings data with Invoice Ninja product data.
6. Print a result table:

```text
IN    ProductKey    Action    Changes/Error
```

### Default behaviour

Without `--commit`, this command must preview only.

Expected action wording:

```text
would-update
unchanged
error
```

### Commit behaviour

With `--commit`, this command may update Invoice Ninja products.

Expected action wording:

```text
updated
unchanged
error
```

### Must not

- Create unrelated products.
- Search Bunnings by free text.
- Update anything without `--commit`.

### Flags

| Flag | Meaning |
|---|---|
| `--commit` | Persist Invoice Ninja product updates. |

---

## `sync import <IN>`

Examples:

```bash
bunnings-ninja sync import 0123456
bunnings-ninja sync import --commit 0123456
```

Legacy alias:

```bash
bunnings-ninja add-in 0123456
```

### User intention

Add or refresh a single Invoice Ninja product from a known Bunnings item number.

### Programming intention

This is a single-product guarded import.

It should:

1. Validate both Bunnings and Invoice Ninja configuration.
2. Fetch the exact Bunnings product by item number.
3. Map Bunnings product fields into the Invoice Ninja product model.
4. Find whether the Invoice Ninja product already exists.
5. Preview or commit create/update behaviour.
6. Print one result row.

### Default behaviour

Without `--commit`, preview only.

### Commit behaviour

With `--commit`, create or update the matching Invoice Ninja product.

### Must not

- Accept multiple item numbers.
- Fuzzy search.
- Bulk import.
- Modify anything without `--commit`.

### Flags

| Flag | Meaning |
|---|---|
| `--commit` | Persist the single product create/update. |

---

## `sync search <query>`

Examples:

```bash
bunnings-ninja sync search "merbau decking" --limit=10
bunnings-ninja sync search "merbau decking" --create --select=0123456,0987654
bunnings-ninja sync search "merbau decking" --create --select=0123456,0987654 --commit
bunnings-ninja sync search "merbau decking" --create --all --yes --commit
```

### User intention

Search Bunnings, inspect candidate products, then optionally import selected products into Invoice Ninja.

### Programming intention

This is a guarded discovery-to-import workflow.

It has two modes.

### Mode 1: search preview

When `--create` is not supplied:

1. Search Bunnings.
2. Print result list:

```text
Bunnings search results
IN    Title
...
Preview only. To import, re-run with --create --select=IN1,IN2 --commit
```

No Invoice Ninja changes are possible in this mode.

### Mode 2: selected import preview/commit

When `--create` is supplied:

1. Search Bunnings.
2. Select products using either:
   - `--select=IN1,IN2`, or
   - `--all --yes`.
3. Map selected products to Invoice Ninja product requests.
4. Preview or commit create/update behaviour.

### Default behaviour

Even with `--create`, this command previews unless `--commit` is supplied.

### Commit behaviour

With `--commit`, selected products may be created or updated in Invoice Ninja.

### Bulk safety rule

`--all` must require `--yes`.

This rule stays even with `--commit`, because bulk import is where goblins learn spreadsheets.

### Must not

- Import unselected search results.
- Allow `--all` without `--yes`.
- Commit anything without `--commit`.
- Treat a plain search as an import.

### Flags

| Flag | Meaning |
|---|---|
| `--limit=N` | Maximum search results; should stay capped. |
| `--create` | Enter import workflow after searching. |
| `--select=IN1,IN2` | Select specific Bunnings item numbers from search results. |
| `--all` | Select all returned results, subject to cap. |
| `--yes` | Required confirmation for `--all`. |
| `--commit` | Persist selected product create/update operations. |

---

## 6. Invoice Ninja CSV commands

Invoice Ninja CSV commands are grouped under:

```bash
bunnings-ninja ninja export ...
bunnings-ninja ninja import ...
```

Exports are read-remote/write-local.  
Imports are read-local/write-remote, but only with `--commit`.

---

## `ninja export products <file|->`

Examples:

```bash
bunnings-ninja ninja export products products.csv
bunnings-ninja ninja export products products.csv --commit
bunnings-ninja ninja export products -
```

### User intention

Export Invoice Ninja products to CSV.

### Programming intention

This is a read remote / write local output command.

It should:

1. Validate Invoice Ninja configuration.
2. Fetch all active products, paginating through Invoice Ninja.
3. Write CSV to:
   - a new file, or
   - stdout when path is `-`.

### Output columns

```text
ID,Product,Description,Price,Default Quantity,Max Quantity,Image URL
```

### File overwrite rule

| Case | Behaviour |
|---|---|
| File does not exist | Create it. |
| File exists, no `--commit` | Refuse. |
| File exists, with `--commit` | Overwrite. |
| Output is `-` | Write stdout; `--commit` irrelevant. |

### Must not

- Modify Invoice Ninja.
- Use `--force`.
- Overwrite files without `--commit`.

---

## `ninja import products <file|->`

Examples:

```bash
bunnings-ninja ninja import products products.csv
bunnings-ninja ninja import products --commit products.csv
cat products.csv | bunnings-ninja ninja import products -
```

### User intention

Preview or apply product CSV changes into Invoice Ninja.

### Programming intention

This is a CSV-driven Invoice Ninja product update workflow.

It should:

1. Validate Invoice Ninja configuration.
2. Read CSV from file or stdin.
3. Require product import columns.
4. For each row:
   - load the existing Invoice Ninja product by ID,
   - build an update payload,
   - compare existing values to CSV values,
   - report differences.
5. Commit only when `--commit` is supplied.

### Default behaviour

Preview only.

Expected action wording:

```text
would-update
unchanged
error
```

### Commit behaviour

With `--commit`, update changed products.

Expected action wording:

```text
updated
unchanged
error
```

### Must not

- Create new products from blank IDs.
- Update without `--commit`.
- Guess product IDs.
- Import quote/invoice/payment CSVs.

### Flags

| Flag | Meaning |
|---|---|
| `--commit` | Persist product updates into Invoice Ninja. |

---

## `ninja export clients <file|->`

Examples:

```bash
bunnings-ninja ninja export clients clients.csv
bunnings-ninja ninja export clients clients.csv --commit
bunnings-ninja ninja export clients -
```

### User intention

Export Invoice Ninja clients and contacts to CSV.

### Programming intention

This is a read remote / write local output command.

It should:

1. Validate Invoice Ninja configuration.
2. Fetch all active clients, including contacts.
3. Determine max contact count.
4. Emit a dynamic CSV header with repeated contact column groups.

### Output columns

Base columns:

```text
ID,Name,Address
```

Repeated contact columns:

```text
Contact 1 First Name,Contact 1 Last Name,Contact 1 Email,Contact 1 Phone,...
```

### File overwrite rule

Same as product export:

| Case | Behaviour |
|---|---|
| File does not exist | Create it. |
| File exists, no `--commit` | Refuse. |
| File exists, with `--commit` | Overwrite. |
| Output is `-` | Write stdout. |

### Must not

- Modify Invoice Ninja.
- Overwrite files without `--commit`.

---

## `ninja import clients <file|->`

Examples:

```bash
bunnings-ninja ninja import clients clients.csv
bunnings-ninja ninja import clients --commit clients.csv
cat clients.csv | bunnings-ninja ninja import clients -
```

### User intention

Preview or apply client/contact CSV changes into Invoice Ninja.

### Programming intention

This is a CSV-driven Invoice Ninja client update workflow.

It should:

1. Validate Invoice Ninja configuration.
2. Read CSV from file or stdin.
3. Require client import columns.
4. For each row:
   - load the existing client by ID,
   - include contacts,
   - build an update payload,
   - compare existing values to CSV values,
   - report differences.
5. Commit only when `--commit` is supplied.

### Default behaviour

Preview only.

Expected actions:

```text
would-update
unchanged
error
```

### Commit behaviour

With `--commit`, update changed clients.

Expected actions:

```text
updated
unchanged
error
```

### Important current design note

The current client CSV model uses a single `Address` column.

That is simple, but potentially lossy because Invoice Ninja has structured address fields. This should be treated as intentional only if a flattened display/import address is genuinely desired.

Suggested future stable columns:

```text
ID,Name,Address1,Address2,City,State,PostalCode,CountryID,...
```

### Must not

- Create clients from blank IDs.
- Update without `--commit`.
- Guess client IDs.
- Silently restructure addresses in a surprising way.

---

## `ninja export quotes <file|->`

Examples:

```bash
bunnings-ninja ninja export quotes quotes.csv
bunnings-ninja ninja export quotes quotes.csv --commit
bunnings-ninja ninja export quotes -
```

### User intention

Export Invoice Ninja quotes to CSV.

### Programming intention

Read-only remote export.

It should:

1. Validate Invoice Ninja configuration.
2. Fetch all active quotes, including client where useful.
3. Write CSV.

### Output columns

```text
ID,Number,Client ID,Client Name,Status,Date,Valid Until,Subtotal,Discount,Tax,Total,Balance,Public Notes,Private Notes
```

### File overwrite rule

Use `--commit` to overwrite an existing file.

### Must not

- Import quotes.
- Update quotes.
- Modify Invoice Ninja.

---

## `ninja export invoices <file|->`

Examples:

```bash
bunnings-ninja ninja export invoices invoices.csv
bunnings-ninja ninja export invoices invoices.csv --commit
bunnings-ninja ninja export invoices -
```

### User intention

Export Invoice Ninja invoices to CSV.

### Programming intention

Read-only remote export.

It should:

1. Validate Invoice Ninja configuration.
2. Fetch all active invoices, including client where useful.
3. Write CSV.

### Output columns

```text
ID,Number,Client ID,Client Name,Status,Date,Due Date,Subtotal,Discount,Tax,Total,Balance,Paid To Date,Public Notes,Private Notes
```

### File overwrite rule

Use `--commit` to overwrite an existing file.

### Must not

- Import invoices.
- Update invoices.
- Modify Invoice Ninja.

---

## `ninja export payments <file|->`

Examples:

```bash
bunnings-ninja ninja export payments payments.csv
bunnings-ninja ninja export payments payments.csv --commit
bunnings-ninja ninja export payments -
```

### User intention

Export Invoice Ninja payments to CSV.

### Programming intention

Read-only remote export.

It should:

1. Validate Invoice Ninja configuration.
2. Fetch all active payments, including client and invoice data where useful.
3. Write CSV.

### Output columns

```text
ID,Client ID,Client Name,Invoice ID,Invoice Number,Date,Amount,Applied,Refunded,Transaction Reference,Payment Type,Status,Private Notes
```

### File overwrite rule

Use `--commit` to overwrite an existing file.

### Must not

- Import payments.
- Update payments.
- Modify Invoice Ninja.

---

## `version`

Example:

```bash
bunnings-ninja version
```

### User intention

Print the application version.

### Programming intention

No configuration required.

It should:

1. Print the compile/source version string.
2. Exit `0`.

### Must not

- Touch Bunnings.
- Touch Invoice Ninja.
- Read config.
- Require credentials.

---

## 7. Deprecated and rejected forms

## Deprecated but currently routed

```bash
bunnings-ninja add-in <IN>
```

### Intention

Legacy alias for:

```bash
bunnings-ninja sync import <IN>
```

It should print a deprecation notice and continue routing to the same implementation.

---

## Explicitly rejected legacy command names

```bash
bunnings-ninja ninja-products-export
bunnings-ninja ninja-products-import
bunnings-ninja ninja-clients-export
bunnings-ninja ninja-clients-import
```

### Intention

These should fail and tell the user to use grouped commands:

```bash
bunnings-ninja ninja export ...
bunnings-ninja ninja import ...
```

---

## 8. Suggested source-level structure

| Concern | Should live in |
|---|---|
| Command routing | `internal/app` |
| Flag parsing | `internal/app` |
| Config loading | `internal/config` |
| Bunnings API calls | `GoBunnings` SDK |
| Invoice Ninja API calls | `GoInvoiceNinja` SDK |
| Product sync workflow | `internal/syncer` |
| CSV import/export workflow | `internal/ninja` or app-specific workflow package |
| Bunnings-to-Invoice Ninja mapping | `GoBunningsNinja`, not either SDK |
| Safety/default preview behaviour | `GoBunningsNinja` |

---

## 9. Implementation guardrails

Future changes should preserve these guardrails:

1. One persistent-change flag only: `--commit`.
2. No remote Invoice Ninja writes without `--commit`.
3. No file overwrite without `--commit`.
4. Bunnings commands stay read-only.
5. Export-only targets stay export-only until deliberately designed otherwise.
6. SDK repos stay reusable and do not learn application-specific behaviour.
7. Mapping and orchestration stay in `GoBunningsNinja`.
8. CLI output should remain predictable and scriptable where CSV is promised.
9. Human-readable commands should stay human-readable.
10. Bulk imports require explicit selection or `--all --yes --commit`.

If a future change violates any of these, it should be treated as a deliberate design decision, not an incidental patch.

---

## 10. Local validation commands

After changing CLI behaviour, run:

```bash
cd /path/to/GoNinjaWorkspace/GoBunningsNinja
git pull
go test ./...
go vet ./...
go build ./cmd/bunnings-ninja
```

If SDK boundaries are touched, also run tests in the SDK repos:

```bash
cd /path/to/GoNinjaWorkspace

cd GoBunnings
git pull
go test ./...
go vet ./...

cd ../GoInvoiceNinja
git pull
go test ./...
go vet ./...

cd ../GoBunningsNinja
git pull
go test ./...
go vet ./...
go build ./cmd/bunnings-ninja
```

Do not claim tests passed unless they were actually run.

---

## 11. Plain-English contract

The CLI should behave like this:

```text
Look around freely.
Preview changes safely.
Only change things when I say --commit.
Never invent extra flags to mean the same thing.
Never overwrite my files unless I say --commit.
Never update Invoice Ninja unless I say --commit.
```

That is the whole magic trick. Everything else is just careful plumbing and occasionally telling a goblin to put the wrench down.
