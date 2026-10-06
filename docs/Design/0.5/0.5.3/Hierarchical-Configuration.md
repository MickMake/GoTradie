# GoTradie v0.5.3 — Hierarchical Configuration

## Status

**Planned — implementation-ready design**

## Purpose

Replace the current flat `key=value` configuration with a structured hierarchical configuration suitable for GoTradie's growing feature set.

The immediate driver is provider/catalogue configuration, but the new structure should also provide a clean home for BAS and other future settings.

The implementation should remain deliberately small.

## Goals

- Move configuration to YAML.
- Preserve environment-variable overrides for secrets and deployment-specific values.
- Represent related settings hierarchically.
- Provide a clean `providers:` structure for future product-sync sources.
- Provide a clean `bas:` structure for accounting settings.
- Keep configuration declarative.
- Avoid creating a programmable configuration language.

## Proposed structure

```yaml
invoice_ninja:
  url: ...
  token: ...

tax:
  name: GST
  rate: 10

bas:
  frequency: quarterly
  gst_basis: cash

product_sync:
  prefix: BUNNINGS-

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

The exact North Shore Timber CSV column names must be verified against the actual CSV before implementation.

## Secrets

Structural configuration belongs in YAML.

Secrets should remain overrideable through environment variables, for example:

```text
INVOICE_NINJA_TOKEN
BUNNINGS_CLIENT_SECRET
```

This allows the YAML file to be stored without embedding credentials.

## Provider/catalogue configuration

Configuration should describe:

- provider type;
- vendor aliases;
- source URL where applicable;
- field mapping for generic catalogue providers.

For a generic CSV catalogue, the initial field mapping should remain small:

```text
item
description
price
```

Do not initially add:

```text
arbitrary regex transforms
HTML selector languages
embedded scripting
row-expression languages
generic workflow logic
```

The rule is:

> Config describes the source. Code implements behaviour.

## Compatibility and migration

`v0.5.3` is a transition release.

The existing flat config format must remain readable for this release so current installations can migrate without a hard cut-over.

Rules:

1. YAML is the preferred configuration format from `v0.5.3` onward.
2. Existing flat config remains accepted during `v0.5.3`.
3. Environment variables continue to override file-based values where already supported.
4. Behaviour must not silently change merely because configuration moved to YAML.
5. If both legacy and YAML configuration are supplied, precedence must be deterministic and documented.
6. Removal of legacy flat-config support is a later explicit decision, not part of this slice.

The implementation should not build a generic migration framework. Supporting the old reader alongside the new YAML reader for the transition is sufficient.

## Scope guardrail

This slice is configuration infrastructure only.

Do not implement:

- BAS export;
- EOFY export;
- Financial export;
- redesigned product sync;
- provider fetch engines beyond what is required to prove the configuration model.

The intended outcome is a stable configuration foundation for later slices.

## Design rule

> Hierarchical where the data is hierarchical; boring everywhere else.
