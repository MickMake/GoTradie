# GoTradie v0.5.7 — Implementation Prompt

## Objective

Implement the accepted v0.5.7 Product Synchronisation design.

Primary design contract:

```text
docs/Design/0.5/0.5.7/Product-Sync.md
```

Configuration dependency:

```text
docs/Design/0.5/0.5.3/Hierarchical-Configuration.md
```

Series context:

```text
docs/Design/0.5/README.md
```

Cross-release CLI contract:

```text
docs/Design/Command-Line-Spec.md
```

Keep the design simple. Do not turn Product Sync into a second catalogue database or general reconciliation platform.

## Mandatory preflight

Before making any code changes:

1. Fetch the latest `origin/main`.
2. Verify all earlier slice branches and pull requests are merged into `main`.
3. If any earlier slice branch or PR is not merged, STOP and report it.
4. Inspect current `sync refresh`, the Bunnings provider/service, Invoice Ninja Product mapping, Expense/Quote/Invoice access, Product metadata fields available, and hierarchical provider configuration from v0.5.3.
5. Verify current Bunnings behaviour around missing location, missing price, API errors, and not-found/discontinued ambiguity.
6. Inspect the actual North Shore Timber CSV before locking its field mapping.
7. State the intended implementation, proposed branch name, likely files/packages, and any data-model ambiguity, especially sync metadata storage.
8. STOP and wait for approval before creating the branch or modifying code.

Suggested branch name:

```text
v0.5.7-product-sync
```

After approval, branch from latest `origin/main` only.

## Required behaviour

Implement the product-sync flow in two phases.

### Phase 1 — existing Invoice Ninja Products

Process existing Invoice Ninja Products first.

For each Product:

1. identify provider;
2. determine provider item number;
3. inspect sync metadata;
4. determine whether provider fetch is required;
5. fetch only when needed;
6. reconcile Product fields;
7. update sync metadata after successful sync;
8. add `(provider, item)` to the in-memory known set.

This phase should establish the known catalogue before any discovery scan.

### Phase 2 — discover missing Products

Then scan:

```text
Expenses
Quotes
Invoices
```

For each supplier product candidate:

1. identify vendor/provider;
2. determine item number;
3. check the in-memory known set;
4. skip immediately if already known;
5. otherwise resolve through the provider;
6. create the missing Invoice Ninja Product;
7. add it to the known set.

Repeated historical references must not cause repeated provider calls.

## Provider routing

Provider selection should be based on configured provider/vendor aliases.

Examples:

```text
Bunnings           -> Bunnings provider
North Shore Timber -> NST CSV provider
```

Do not scatter supplier-specific `if` chains through the sync engine.

If no provider matches, report/skip. Do not guess.

## Provider states

Model provider results clearly enough to distinguish at least:

```text
available
discontinued
unknown/ambiguous
error
```

Required behaviour:

| Provider result | Action |
| --- | --- |
| Available | Create/update Product |
| Discontinued | Archive existing Product; never delete |
| Unknown / ambiguous | Report; no lifecycle change |
| Temporary/provider error | Report; no lifecycle change |
| Provider unavailable | Skip/report |

A failed request must never be treated as discontinued.

## Archive provenance

Do not blindly unarchive a Product simply because the provider later returns it.

Automatic restoration is allowed only when GoTradie can establish that GoTradie previously archived it because the provider reported it discontinued.

If the current Invoice Ninja Product model does not provide a safe metadata location, STOP and report the options before inventing persistence elsewhere.

Do not add a GoTradie database for this.

## Sync metadata

Store enough durable metadata on the Invoice Ninja Product to determine whether a provider fetch is needed.

At minimum:

```text
provider
provider item number
last successful sync
provider state
```

The exact storage mechanism must use an appropriate Invoice Ninja Product field or metadata mechanism.

Do not consume scarce custom fields casually.

Distinguish `discovered` from `successfully synced`.

## Freshness

Implement a deliberately simple freshness policy.

Conceptually:

```text
never synced       -> fetch
stale              -> fetch
recently synced    -> skip
forced refresh     -> fetch
```

Do not build scheduling machinery.

If the design does not yet specify the exact default freshness interval, STOP and request that decision rather than inventing a complex configurable policy.

## North Shore Timber

Implement North Shore Timber as the first generic CSV catalogue provider.

Source:

```text
https://www.nst.net.au/nst/DownloadCSV
```

For one sync run:

1. fetch the CSV once;
2. parse once;
3. build an in-memory map keyed by supplier item code;
4. satisfy all NST lookups from that map.

No persistent NST cache.

Verify the actual CSV field names before coding the mapping.

## Generic CSV provider

Initial generic CSV provider scope:

- source URL;
- vendor aliases;
- item field;
- description field;
- price field.

Do not initially add regex engines, HTML selector engines, embedded scripting, transformation languages, workflow expressions, or persistent catalogue storage.

> Config describes the source. Code implements behaviour.

## Bunnings hardening

While implementing the provider abstraction, preserve or improve Bunnings behaviour so that missing location is explicit, missing price is explicit, API/provider errors are not swallowed, zero is not silently substituted for unknown price, and not-found/discontinued/error states remain distinguishable.

Do not broaden this into unrelated Bunnings feature work.

## Scope exclusions

Do not implement a persistent product database, SQLite catalogue storage, background sync, automatic scheduling, a generic web scraper, arbitrary HTML parsing rules, unrelated Invoice Ninja refactors, or BAS/EOFY/Financial work.

## Tests

Add tests covering at least:

- existing Products processed before discovery;
- known-set deduplication;
- repeated Expense/Quote/Invoice references causing no duplicate provider calls;
- provider alias routing;
- unknown provider handling;
- fresh vs stale sync behaviour;
- forced refresh;
- available Product create/update;
- discontinued Product archive;
- provider error causing no lifecycle change;
- safe archive provenance behaviour;
- NST catalogue fetched once per run;
- NST in-memory lookups;
- Bunnings missing-price/error handling;
- no persistent side database.

Use fakes for providers where practical.

## Verification

Before declaring complete:

```text
gofmt
go vet
go test
go build
```

Then perform an evidence-backed review.

Maximum review/fix loops: **3**.

If material issues remain after three loops, STOP and report them.

## Completion report

Report branch used, files/packages changed, provider abstraction implemented, metadata storage chosen, freshness behaviour, discovery sources implemented, NST behaviour, Bunnings hardening, lifecycle/discontinued behaviour, tests added, verification results, deferred issues, and readiness for PR/review.

> First sync the products we already have. Then look for the products we do not.
