# GoTradie

`GoTradie` is a small CLI client that connects the local `GoBunnings` and `GoInvoiceNinja` packages.

Version: `v0.5.4`

The goal is deliberately modest: refresh Invoice Ninja products from Bunnings product data, add selected Bunnings products safely, and export/import selected Invoice Ninja CSV data without turning the accounts into a surprised octopus.

## Requirements

- Go `1.25`
- Local checkout of `GoBunnings`
- Local checkout of `GoInvoiceNinja` v0.5 or later

## Multi-repo development

Develop the three repos side by side in one parent folder with a local Go workspace:

```text
GoNinjaWorkspace/
├── go.work
├── GoBunnings/
├── GoInvoiceNinja/
└── GoTradie/
```

From the parent folder:

```bash
git clone git@github.com:MickMake/GoTradie.git

go work init ./GoBunnings ./GoInvoiceNinja ./GoTradie
go work sync
```

The local `go.work` file replaces the old committed absolute-path `replace` directives. Keep `go.mod` portable and use the workspace for local multi-repo editing.
The local `go.work` file is the preferred way to develop the three repos together. `GoTradie/go.mod` may also use relative local `replace` directives for simple single-machine development.

See `ECOSYSTEM.md` for the dependency rules and release workflow.

## Build

```bash
go build ./cmd/GoTradie
```

## Configuration

Operational commands load one mandatory, strict YAML file:

```text
~/.GoTradie/config.yaml
```

Copy `config.yaml.example` there and supply the settings required by the commands you use. Unknown YAML fields are rejected. BAS cadence, GST basis, EOFY basis and the asset threshold must be explicit.

Only these secret environment variables may override YAML:

```text
INVOICE_NINJA_TOKEN
BUNNINGS_CLIENT_SECRET
```

Legacy flat configuration, `--config`, `GOTRADIE_CONFIG` and generic environment overrides are not supported from v0.5.3 onward.

## Commands

All write-capable commands preview or refuse risky writes by default. Add `--commit` when you want to make a persistent change in InvoiceNinja.

## Bunnings data source

By default, Bunnings product/search data is fetched from the Bunnings API.

Add `--web` to supported Bunnings-backed commands to use the website-derived retrieval path instead:

```bash
GoTradie bunnings get 0123456 --web
GoTradie bunnings lookup 0123456 --web
GoTradie bunnings find "merbau decking" --web
GoTradie sync refresh --web
GoTradie sync import 0123456 --web
GoTradie sync search "merbau decking" --web
```

`--web` only changes the Bunnings data source. It does not imply `--commit`, does not modify Invoice Ninja by itself, and does not silently fall back to the API.

### Sync existing Invoice Ninja products

```bash
GoTradie sync
GoTradie sync refresh
GoTradie sync refresh --commit
```

### Add or refresh by Bunnings IN

```bash
GoTradie sync import 0123456
GoTradie sync import --commit 0123456
```

The old `add-in` command still routes to `sync import`, but new usage should prefer the grouped `sync import` form.

### Search Bunnings products safely

Preview only:

```bash
GoTradie sync search "merbau decking"
```

Preview selected results for import:

```bash
GoTradie sync search "merbau decking" --create --select=0123456,0987654
```

Import selected results:

```bash
GoTradie sync search "merbau decking" --create --select=0123456,0987654 --commit
```

Bulk importing all returned search results requires `--all --yes --commit` and remains hard-capped by the search limit.

## Invoice Ninja CSV commands

The grouped command format is:

```bash
GoTradie ninja export <target> <file|->
GoTradie ninja import <target> <file|->
```

Exports use a positional destination:

```bash
GoTradie ninja export products products.csv
GoTradie ninja export products -
```

Exports do not overwrite files unless `--force` is used:

```bash
GoTradie ninja export products products.csv --force
```

Imports use a positional source:

```bash
GoTradie ninja import products products.csv
cat products.csv | GoTradie ninja import products -
```

Imports preview by default. Use `--commit` to update Invoice Ninja:

```bash
GoTradie ninja import products products.csv
GoTradie ninja import products --commit products.csv
```

Available export targets:

```text
products
clients
quotes
invoices
payments
bas
```

### BAS workbook

Generate the natural, most recently completed BAS reporting period:

```bash
GoTradie ninja export bas
```

Explicit selections override the default:

```bash
GoTradie ninja export bas --fy 2025
GoTradie ninja export bas --fy 2027 --period 1
GoTradie ninja export bas --period September
```

The workbook is written to `exports.directory` when configured, otherwise the
current directory. It contains `Summary`, `Sales`, `Purchases` and `Exceptions`
sheets and refuses to replace its deterministic filename unless `--force` is
supplied. Accounting errors still produce an `INCOMPLETE` diagnostic workbook
when technically possible and exit with status 1.

Available import targets:

```text
products
clients
expenses
```

Expense imports require a named CSV file and a non-blank, file-unique
`Import ID` for every non-empty row. The complete file is preflighted before
execution, then each row and a final summary are reported. Stdin is not
accepted for Expense imports.

A receipt directory can be indexed recursively. Each non-empty `File Name`
must match one filename exactly; missing and duplicate matches are reported
without guessing. Reused receipt content is uploaded once with durable
ownership recorded in Invoice Ninja.

```bash
GoTradie ninja import expenses purchases.csv --receipts-root receipts
GoTradie ninja import expenses purchases.csv --receipts-root receipts --commit
GoTradie ninja import expenses purchases.csv --batch-size 100 --pause --commit
```

`--batch-size` and optional `--pause` apply to ordinary Expense-only files.
Any file containing Account Payment rows uses settlement-safe whole-file
execution so batching cannot change supplier FIFO settlement.

Quote, invoice, and payment commands are export-only.

## CSV columns

### Products

```text
ID, Product, Description, Price, Default Quantity, Max Quantity, Image URL
```

`Max Quantity` is currently exported blank and ignored on import until the upstream `GoInvoiceNinja` package confirms or exposes a matching Invoice Ninja API field.

### Clients

```text
ID, Name, Address, Contact 1 First Name, Contact 1 Last Name, Contact 1 Email, Contact 1 Phone, ...
```

The client export creates as many repeated contact column groups as needed for the current data.

### Quotes

```text
ID, Number, Client ID, Client Name, Status, Date, Valid Until, Subtotal, Discount, Tax, Total, Balance, Public Notes, Private Notes
```

### Invoices

```text
ID, Number, Client ID, Client Name, Status, Date, Due Date, Subtotal, Discount, Tax, Total, Balance, Paid To Date, Public Notes, Private Notes
```

### Payments

```text
ID, Client ID, Client Name, Invoice ID, Invoice Number, Date, Amount, Applied, Refunded, Transaction Reference, Payment Type, Status, Private Notes
```

## Notes

For complete Invoice Ninja exports, this version uses `GoInvoiceNinja` v0.5.1 `ListAll` helpers rather than single-page `List` calls.
