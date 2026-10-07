# GoTradie v0.5.7 Product Sync Documentation Changes

This bundle contains the Product Sync decisions locked during review.

## CLI

```text
GoTradie sync refresh [--commit]
```

- Processes all configured Providers.
- Preview by default.
- `--commit` persists Invoice Ninja Product changes.
- No Provider-selection flag initially.
- No Product Sync `--force`.

## Product identity

```text
Product        = supplier SKU
Supplier       = canonical supplier name
Store          = metadata only
Last Sync Date = last successful provider sync date
Not Available  = availability toggle
```

Only `(Supplier, Product)` participates in matching/deduplication.

## Availability

- Confirmed unavailable/discontinued -> `Not Available = true`
- Confirmed available/reappeared -> `Not Available = false`
- Error/unknown -> leave existing availability unchanged
- Reappearance does not create a new Product

## Freshness

- Bunnings/API-backed Providers: per-Product freshness.
- File-backed Providers: source-file freshness.
- Content hash is the preferred authoritative source-change detector.
- ETag/Last-Modified may be used as optimisation hints.

## Cache boundary

A small persistent source-fingerprint cache is allowed for file-backed Providers.

It may store source metadata/fingerprints only.

It must not become a second Product catalogue, Product-state database, or accounting database.

## Files changed

- `docs/Design/0.5/README.md`
- `docs/Design/0.5/Cross-Audit.md`
- `docs/Design/0.5/0.5.7/Product-Sync.md`
- `docs/Design/0.5/0.5.7/Implementation-Prompt.md`
