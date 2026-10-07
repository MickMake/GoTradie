# GoTradie v0.5.7 — Implementation Prompt

Primary design contract:

```text
docs/Design/0.5/0.5.7/Product-Sync.md
```

Cross-audit contract:

```text
docs/Design/0.5/Cross-Audit.md
```

Relevant configuration contract:

```text
docs/Design/0.5/0.5.3/Hierarchical-Configuration.md
```

## Mandatory preflight

Before making any code changes:

1. Fetch latest `origin/main`.
2. Verify all earlier slice branches/PRs are merged or explicitly handled.
3. If any earlier slice branch/PR is not merged, STOP and report it.
4. Inspect current Product Sync, Bunnings behaviour, Invoice Ninja Product metadata capacity, Vendor handling, Expense supplier/store metadata, Quote/Invoice product evidence, and Provider config.
5. Verify Bunnings missing-location, missing-price, API-error and not-found/discontinued behaviour.
6. Inspect the actual NST CSV before locking field names.
7. State intended implementation, branch, likely files/packages, and any blocking metadata/source-data ambiguity.
8. STOP and wait for approval.

Suggested branch:

```text
v0.5.7-product-sync
```

## Required architecture

Implement the two-phase sync:

1. existing Invoice Ninja Products first;
2. build an in-memory `(provider,item)` known set;
3. then inspect Expenses, Quotes and Invoices where sufficient Provider/item evidence exists;
4. create/fetch only missing known-Provider products.

## Vendor/Provider behaviour

- Vendors are accounting entities.
- Providers are configured catalogue/API integrations.
- A supplier may be a valid Vendor without having a Provider.
- If a required Vendor does not exist, preserve the accounting/import behaviour that creates it where appropriate.
- Unknown/non-provider Vendors do not participate in Product Sync.

When an incoming supplier matches a configured Provider alias:

```text
use Provider canonical official supplier name as Vendor identity
preserve Store separately as Expense metadata
```

Aliases are recognition inputs only and must not become alternate Vendor identities.

Store/location must not be appended to or encoded into Vendor identity.

Do not create store-specific Vendor variants.

Do not discard store/location metadata.

## Provider aliases

Use configured aliases only.

Resolution is trim + case-insensitive exact matching after normalization.

Do not implement fuzzy matching or automatic alias learning.

## Provider states

Implement available, discontinued, unknown and error.

Confirmed discontinued may archive; never delete.

Unknown/error makes no lifecycle change.

Failed requests are never discontinued.

## Sync metadata

Persist provider, provider item number, last successful sync and provider state on Invoice Ninja Product.

Do not add a GoTradie database.

Before consuming Product custom fields, inspect current usage. If there is no safe metadata location, STOP and report the conflict.

## Freshness

Implement only never-synced/stale/recently-synced behaviour.

Do not add `--force`.

## NST

Fetch once per sync run, parse once, use in-memory map, no persistent cache.

Verify actual CSV fields before implementation.

## Tests

Cover existing-products-first, dedupe, alias resolution, canonical Vendor identity, Store preservation, unknown Vendor skip-sync behaviour, freshness, lifecycle states, NST fetch-once, and absence of persistent side DB/cache.

## Verification

Run `gofmt`, `go vet`, `go test`, and `go build`.

Maximum review/fix loops: 3.
