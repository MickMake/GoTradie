# GoBunningsNinja

`GoBunningsNinja` is a small CLI client that connects the local `GoBunnings` and `GoInvoiceNinja` packages.

Version: `v0.5`

The goal is deliberately modest: refresh Invoice Ninja products from Bunnings product data, add selected Bunnings products safely, and export/import selected Invoice Ninja CSV data without turning the accounts into a surprised octopus.

## Requirements

- Go `1.22`
- Local checkout of `GoBunnings`
- Local checkout of `GoInvoiceNinja` v0.5 or later

## Multi-repo development

Develop the three repos side by side in one parent folder with a local Go workspace:

```text
GoNinjaWorkspace/
├── go.work
├── GoBunnings/
├── GoInvoiceNinja/
└── GoBunningsNinja/
```

From the parent folder:

```bash
git clone git@github.com:MickMake/GoBunnings.git
git clone git@github.com:MickMake/GoInvoiceNinja.git
git clone git@github.com:MickMake/GoBunningsNinja.git

go work init ./GoBunnings ./GoInvoiceNinja ./GoBunningsNinja
go work sync
```

The local `go.work` file replaces the old committed absolute-path `replace` directives. Keep `go.mod` portable and use the workspace for local multi-repo editing.
The local `go.work` file is the preferred way to develop the three repos together. `GoBunningsNinja/go.mod` may also use relative local `replace` directives for simple single-machine development.

See `ECOSYSTEM.md` for the dependency rules and release workflow.

## Build

```bash
go build ./cmd/bunnings-ninja
```

## Configuration

Configuration is loaded from environment variables first, then from a config file. Config file values override environment variables.

Config lookup order:

1. `--config <path>`
2. `GOBUNNINGSNINJA_CONFIG`
3. `./gobunningsninja.conf`, if present

Example:

```bash
bunnings-ninja --config ./gobunningsninja.conf ninja export products products.csv
```

Required for Invoice Ninja commands:

```text
INVOICE_NINJA_TOKEN
```

Also required for Bunnings sync/search commands:

```text
BUNNINGS_CLIENT_ID
BUNNINGS_CLIENT_SECRET
```

Useful optional settings:

```text
INVOICE_NINJA_URL
BUNNINGS_ENV
BUNNINGS_COUNTRY
BUNNINGS_LOCATION
BUNNINGS_SCOPES
PRODUCT_PREFIX
BUNNINGS_IN_CUSTOM_FIELD
BUNNINGS_IMAGE_CUSTOM_FIELD
TAX_NAME
TAX_RATE
```

See `gobunningsninja.conf.example`.

## Commands

### Sync existing Invoice Ninja products

```bash
bunnings-ninja sync
bunnings-ninja sync --dry-run=false
```

### Add or refresh by Bunnings IN

```bash
bunnings-ninja add-in 0123456
bunnings-ninja add-in --dry-run=false 0123456
```

### Search Bunnings products safely

Preview only:

```bash
bunnings-ninja search "merbau decking"
```

Import selected results:

```bash
bunnings-ninja search "merbau decking" --create --select=0123456,0987654 --dry-run=false
```

Bulk importing all returned search results requires `--all --yes` and remains hard-capped by the search limit.

## Invoice Ninja CSV commands

The grouped command format is:

```bash
bunnings-ninja ninja export <target> <file|->
bunnings-ninja ninja import <target> <file|->
```

Exports use a positional destination:

```bash
bunnings-ninja ninja export products products.csv
bunnings-ninja ninja export products -
```

Exports do not overwrite files unless `--force` is used:

```bash
bunnings-ninja ninja export products products.csv --force
```

Imports use a positional source:

```bash
bunnings-ninja ninja import products products.csv
cat products.csv | bunnings-ninja ninja import products -
```

Imports default to dry-run:

```bash
bunnings-ninja ninja import products products.csv
bunnings-ninja ninja import products products.csv --dry-run=false
```

Available export targets:

```text
products
clients
quotes
invoices
payments
```

Available import targets:

```text
products
clients
```

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

For complete Invoice Ninja exports, this version uses `GoInvoiceNinja` v0.5 `ListAll` helpers rather than single-page `List` calls.
