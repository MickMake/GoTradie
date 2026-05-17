# GoBunningsNinja

Version: `0.1`

`GoBunningsNinja` is a small CLI/client that bridges:

- `GoBunnings` for Bunnings product lookup, search, and pricing.
- `GoInvoiceNinja` for Invoice Ninja product/client maintenance.

It is intentionally a thin orchestration layer. The API packages stay reusable and independent, which prevents the software equivalent of storing screws, nails, and one mysterious Allen key in the same labelled jar.

## Packaging note

This zip does **not** include `GoBunnings` or `GoInvoiceNinja`. It expects those packages to exist on your filesystem and uses `replace` directives in `go.mod`:

```go
replace github.com/MickMake/GoBunnings => /Volumes/home/mick/Documents/GoLang/Tradie/GoBunnings
replace github.com/MickMake/GoInvoiceNinja => /Volumes/home/mick/Documents/GoLang/Tradie/GoInvoiceNinja
```

## Build

```bash
go test ./...
go build ./cmd/bunnings-ninja
```

The module is set to:

```go
go 1.22
```

## Configuration

Configuration can come from environment variables and/or a config file.

Config precedence:

1. defaults;
2. environment variables;
3. config file values.

That means the config file overrides environment variables.

Use a config file explicitly:

```bash
bunnings-ninja --config ./gobunningsninja.conf sync
```

Or set:

```bash
export GOBUNNINGSNINJA_CONFIG="/path/to/gobunningsninja.conf"
```

If neither is set, the CLI will use `./gobunningsninja.conf` if present.

See `gobunningsninja.conf.example`.

### Required for `ninja-*` CSV commands

```bash
INVOICE_NINJA_TOKEN=...
```

### Also required for Bunnings sync/search commands

```bash
BUNNINGS_CLIENT_ID=...
BUNNINGS_CLIENT_SECRET=...
```

### Useful optional values

```bash
INVOICE_NINJA_URL=https://your.invoice-ninja.example
BUNNINGS_ENV=live
BUNNINGS_COUNTRY=AU
BUNNINGS_LOCATION=1234
BUNNINGS_SCOPES="scope1 scope2"
PRODUCT_PREFIX=BUNNINGS-
BUNNINGS_IN_CUSTOM_FIELD=1
BUNNINGS_IMAGE_CUSTOM_FIELD=2
TAX_NAME=GST
TAX_RATE=10
```

## Commands

### Version

```bash
bunnings-ninja version
```

### Refresh linked Invoice Ninja products

Dry-run preview:

```bash
bunnings-ninja sync
```

Apply updates:

```bash
bunnings-ninja sync --dry-run=false
```

Existing products are linked by:

1. configured Bunnings IN custom field, default `custom_value1`; or
2. an inferred number in the product key, such as `BUNNINGS-0123456`.

### Add or refresh one product by Bunnings IN

```bash
bunnings-ninja add-in 0123456
bunnings-ninja add-in --dry-run=false 0123456
```

### Search Bunnings

Preview:

```bash
bunnings-ninja search "merbau decking" --limit=10
```

Create/update selected products:

```bash
bunnings-ninja search "merbau decking" --create --select=0123456,0987654 --dry-run=false
```

Create/update all returned results, explicitly confirmed:

```bash
bunnings-ninja search "deck screws" --create --all --yes --limit=5 --dry-run=false
```

## CSV commands

The `ninja-*` commands only talk to Invoice Ninja.

### Export products

```bash
bunnings-ninja ninja-products-export --out products.csv
```

Columns:

```text
ID,Product,Description,Price,Default Quantity,Max Quantity,Image URL
```

Notes:

- `Product` maps to `product_key`.
- `Description` maps to `notes`.
- `Default Quantity` maps to `quantity`.
- `Image URL` maps to the configured image custom field, default `custom_value2`.
- `Max Quantity` is currently exported blank and ignored on import because the uploaded `GoInvoiceNinja` package does not expose a max quantity field.

### Import products

Dry-run preview:

```bash
bunnings-ninja ninja-products-import products.csv
```

Apply changes:

```bash
bunnings-ninja ninja-products-import --dry-run=false products.csv
```

### Export clients

```bash
bunnings-ninja ninja-clients-export --out clients.csv
```

Columns:

```text
ID,Name,Address,Contact 1 First Name,Contact 1 Last Name,Contact 1 Email,Contact 1 Phone,...
```

The number of contact column groups expands to fit the largest number of contacts found on any exported client.

### Import clients

Dry-run preview:

```bash
bunnings-ninja ninja-clients-import clients.csv
```

Apply changes:

```bash
bunnings-ninja ninja-clients-import --dry-run=false clients.csv
```

Address note: Invoice Ninja stores billing addresses in structured fields. This version exports one combined `Address` column. If changed on import, the value is written to `address1` while preserving the other structured fields where possible.

## Import guardrails

Search import is intentionally conservative:

- `search` is preview-only by default.
- `--create` still requires `--select=IN1,IN2` or `--all --yes`.
- `--limit` is hard-capped by the Bunnings service.
- `--dry-run` defaults to `true`.

That means a broad search cannot quietly create a thousand Invoice Ninja products while everyone involved is making tea.
