# GoTradie command intention spec

Version: v0.6-draft

This document defines intended CLI behaviour for `GoTradie`.

## Core safety model

Commands preview or refuse risky writes by default.

Use `--commit` to make persistent changes.

Do not add synonym flags such as:

- `--dry-run`
- `--apply`
- `--force`

`--commit` is the only flag that allows:

- Invoice Ninja writes
- overwriting existing local files

## Bunnings source model

Default Bunnings source:

```text
Bunnings API
```

With `--web`:

```text
Bunnings website-derived retrieval path
```

`--web` is a source selector, not a persistence flag.

`--web` must not:

- imply `--commit`
- silently fall back to the API
- change output format
- make Invoice Ninja credentials required for Bunnings-only commands
- update Invoice Ninja by itself

If `--web` is supplied and required website-derived data cannot be retrieved, fail clearly.

For batch operations, per-product failures are acceptable where practical, but incomplete website-derived data must not be presented as successful.

## Supported `--web` commands

### `bunnings get <IN...> --web`

Fetch one or more exact Bunnings item numbers using the website-derived retrieval path.

Output must match API-backed `bunnings get`:

```text
IN,Description,Unit,PricePerUnit,ImageURL
```

Must not:

- perform fuzzy search
- modify Invoice Ninja
- require Invoice Ninja credentials
- accept or require `--commit`

### `bunnings lookup <IN...> --web`

Fetch one or more exact Bunnings item numbers using the website-derived retrieval path.

Output must match API-backed `bunnings lookup`:

```text
IN: ...
Title: ...
Description: ...
Unit: ...
PricePerUnit: ...
ImageURL: ...
```

Must not:

- produce CSV
- modify Invoice Ninja
- require Invoice Ninja credentials
- accept or require `--commit`

### `bunnings find <query> --web`

Search using the website-derived retrieval path.

Output must match API-backed `bunnings find` CSV output.

Must not:

- modify Invoice Ninja
- require Invoice Ninja credentials
- accept or require `--commit`

### `sync refresh --web`

Refresh existing linked Invoice Ninja products using website-derived Bunnings data.

Without `--commit`, preview only.

With `--commit`, update Invoice Ninja products where required.

### `sync import <IN> --web`

Import or refresh a single known Bunnings item number using website-derived Bunnings data.

Without `--commit`, preview only.

With `--commit`, create or update the matching Invoice Ninja product.

### `sync search <query> --web`

Search using website-derived Bunnings data, then optionally import selected products.

Without `--create`, search/preview only.

With `--create`, selection is required using one of:

- `--select=IN1,IN2`
- `--all --yes`

Without `--commit`, selected imports are preview only.

With `--commit`, selected imports may create or update Invoice Ninja products.

Bulk import rule:

```text
--all requires --yes
```

This remains true even when `--commit` is supplied.

## Commands that must not accept `--web`

- `ninja export ...`
- `ninja import ...`
- `version`

## Plain-English contract

```text
Default Bunnings data source = API.
--web changes only the Bunnings data source.
--commit changes whether persistent writes are allowed.
No Invoice Ninja writes without --commit.
No silent API fallback when --web is requested.
No extra write-ish flags breeding in the skirting boards.
```
