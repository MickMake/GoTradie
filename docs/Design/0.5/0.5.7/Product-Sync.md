# GoTradie v0.5.7 — Product Synchronisation

## Purpose

Redesign product synchronisation so GoTradie refreshes known Invoice Ninja Products first, then discovers only products that are missing.

The design should support multiple supplier/provider sources without turning GoTradie into a second product database.

## Core rule

> First sync the products we already have. Then look for the products we do not.

## Phase 1 — Existing Invoice Ninja Products

Invoice Ninja Products are the primary source.

For each existing Product:

1. identify its supplier/provider;
2. determine its supplier item number;
3. inspect sync metadata;
4. fetch current supplier data only when required;
5. update the Invoice Ninja Product if necessary;
6. record successful sync metadata.

While processing, build an in-memory known set keyed conceptually by:

```text
provider + item number
```

Examples:

```text
bunnings + 0123456
nst + TPS025050
```

This phase should cover most known products.

## Phase 2 — Discover missing products

After existing Products have been processed, inspect:

```text
Expenses
Quotes
Invoices
```

For each supplier product reference:

```text
Is provider + item number already known?
```

If yes:

```text
skip
```

If no:

```text
resolve provider
fetch supplier data
create the missing Invoice Ninja Product
add it to the known set
```

Repeated historical references must not cause repeated provider requests.

## Provider routing

A discovered candidate should contain enough information to route it:

```text
Vendor: North Shore Timber
Item:   TPS025050
```

The sync layer then matches the vendor to a configured provider.

Examples:

```text
Bunnings           -> Bunnings API provider
North Shore Timber -> NST CSV catalogue provider
```

Provider aliases should be supported.

If no provider exists for a vendor, report/skip it. Do not guess.

The sync engine should not become a chain of supplier-specific `if` statements.

## Provider states

Supplier lookup results should conceptually distinguish:

```text
available
discontinued
unknown
error
```

Suggested behaviour:

| Provider result | Invoice Ninja action |
| --- | --- |
| Available | Create/update product |
| Discontinued | Archive existing product; do not delete |
| Unknown / ambiguous | Report; no lifecycle change |
| Temporary/provider error | Report; no lifecycle change |
| Provider unavailable | Skip/report |

A failed request must never be interpreted as "discontinued".

## Archived products

Do not blindly reactivate an archived product merely because a supplier later returns it.

A Product may have been manually archived.

Automatic restoration is only safe if GoTradie can establish that it previously archived the Product specifically because the supplier reported it discontinued.

That requires provenance metadata.

## Sync metadata

GoTradie should store enough metadata on the Invoice Ninja Product to know whether a provider request is needed.

At minimum:

```text
provider
provider item number
last successful sync
provider state
```

The important distinction is between:

```text
discovered
```

and:

```text
successfully synced
```

Finding the same product repeatedly in Expenses, Quotes or Invoices must not cause repeated supplier requests.

The metadata should live in Invoice Ninja rather than a GoTradie database.

The exact Invoice Ninja storage mechanism should be decided during implementation.

## Sync freshness

Conceptual policy:

```text
never synced       -> fetch
sync data stale    -> fetch
recently synced    -> skip
forced refresh     -> fetch
```

Prefer a simple refresh policy and a force option over elaborate scheduling/configuration.

## North Shore Timber

North Shore Timber is the first generic catalogue provider.

Catalogue sources:

```text
https://www.nst.net.au/nst/DownloadCSV
https://www.nst.net.au/#products
```

Prefer the CSV for machine use.

For a sync run:

1. download the catalogue once;
2. parse it;
3. build an in-memory map keyed by supplier item code;
4. satisfy all NST lookups from memory.

Conceptually:

```text
1 catalogue download
60 product lookups in memory
```

No persistent catalogue cache is required.

## Configuration

This slice relies on the hierarchical YAML configuration introduced in v0.5.3.

Example:

```yaml
providers:
  bunnings:
    type: api
    aliases:
      - Bunnings
      - Bunnings Warehouse

  nst:
    type: csv
    aliases:
      - North Shore Timber
      - NST
    url: https://www.nst.net.au/nst/DownloadCSV
    fields:
      item: PartNo
      description: Description
      price: Price
```

The exact NST field names must be verified before implementation.

## Generic CSV providers

Configuration should remain declarative.

A generic CSV provider initially needs only:

```text
source URL
vendor aliases
item field
description field
price field
```

Do not initially add:

```text
generic regex engines
arbitrary HTML selectors
embedded scripting
transformation languages
workflow expressions
```

The rule remains:

> Config describes the source. Code implements behaviour.

## Architecture guardrail

This feature must not grow into a second product database.

Avoid:

```text
persistent GoTradie catalogue storage
SQLite product ledgers
duplicate supplier/product databases
elaborate reconciliation queues
automatic supplier requests for every occurrence
```

Invoice Ninja remains the durable product store.

Provider data is fetched only to refresh or create Invoice Ninja Products.

## Design rule

> Sync known Invoice Ninja Products first. Then scan Expenses, Quotes and Invoices only for missing supplier products.
