# GoTradie Command-Line Contract

## Core rule

GoTradie uses distinct flags for distinct kinds of persistent effects:

```text
--commit
    Persist changes to Invoice Ninja / remote application state.

--force
    Overwrite existing local output files.
```

Do not use one as a synonym for the other.

## Write safety

| Operation | Default | Override |
|---|---|---|
| Invoice Ninja create/update/archive/write | Preview/refuse | `--commit` |
| Create new local export file | Allowed | none |
| Overwrite existing local export file | Refuse | `--force` |
| Read-only discovery/API lookup | Allowed | none |

### `--commit`

`--commit` is the only flag that permits persistent Invoice Ninja changes.

Without it, write-capable commands must preview or otherwise remain non-mutating.

Examples:

```bash
GoTradie sync refresh --commit
GoTradie sync import 0123456 --commit
GoTradie sync search "merbau decking" --create --select=0123456 --commit
GoTradie ninja import products products.csv --commit
GoTradie ninja import clients clients.csv --commit
GoTradie ninja import expenses purchases.csv --commit
```

### `--force`

`--force` permits replacement of existing local export output.

Examples:

```bash
GoTradie ninja export products products.csv --force
GoTradie ninja export clients clients.csv --force
GoTradie ninja export quotes quotes.csv --force
GoTradie ninja export invoices invoices.csv --force
GoTradie ninja export payments payments.csv --force
GoTradie ninja export erpnext ./erpnext-export --force
GoTradie ninja export tax ./tax-export --force
```

`--force` does not permit Invoice Ninja writes.

`--force` must not be used for Product Sync freshness, bypassing validation, or as another spelling of `--commit`.

## Export overwrite rule

For local exports:

| Case | Behaviour |
|---|---|
| Output does not exist | Create it |
| Output exists, no `--force` | Refuse |
| Output exists, `--force` | Replace it |
| Output is stdout (`-`) | Write stdout; `--force` irrelevant |

Generated BAS/EOFY/Financial output follows the same local-file rule.

## Removed/rejected meanings

Do not reintroduce:

```text
--dry-run
--apply
```

Do not use `--force` to mean:

```text
persist remote changes
ignore Product Sync freshness
bypass validation
```

## Plain-English contract

```text
Look around freely.
Preview remote changes safely.
Use --commit to change Invoice Ninja.
Use --force to replace an existing local file.
```
