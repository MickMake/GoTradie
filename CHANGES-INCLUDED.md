# GoTradie BAS Period Documentation Changes

This bundle contains only the BAS period/configuration decisions locked during review.

## Configuration

```yaml
bas:
  reporting_period: quarterly
  gst_basis: cash
```

`reporting_period` replaces `frequency`.

Supported values:

```text
monthly
quarterly
yearly
```

GST basis remains:

```text
cash
accrual
```

## BAS CLI

Supported:

```text
GoTradie ninja export bas
GoTradie ninja export bas --fy 2027
GoTradie ninja export bas --period 2
GoTradie ninja export bas --fy 2027 --period 2
```

Defaults:

```text
no --fy     -> current Australian financial year
no --period -> current reporting period
```

`--period` meaning:

```text
monthly   -> 1-12, July through June
quarterly -> 1-4, Q1 through Q4
yearly    -> invalid / not applicable
```

BAS explicitly does not support:

```text
--from
--to
--all
--month
--quarter
```

Arbitrary date ranges remain a Financial/dump export feature.

## Files changed

- `docs/Design/0.5/README.md`
- `docs/Design/0.5/Cross-Audit.md`
- `docs/Design/0.5/0.5.3/Hierarchical-Configuration.md`
- `docs/Design/0.5/0.5.3/Implementation-Prompt.md`
- `docs/Design/0.5/0.5.4/BAS-Export.md`
- `docs/Design/0.5/0.5.4/Implementation-Prompt.md`
