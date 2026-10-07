# GoTradie Command Intention Spec

## Safety vocabulary

```text
--commit
    Persist changes to Invoice Ninja.

--force
    Overwrite existing local export output.
```

Without `--commit`, commands capable of changing Invoice Ninja remain preview-only.

Without `--force`, export commands may create new output but refuse to overwrite existing local output.

These flags are independent and must not substitute for one another.

## Examples

```bash
GoTradie sync refresh --commit
GoTradie ninja import expenses purchases.csv --commit

GoTradie ninja export products products.csv --force
GoTradie ninja export erpnext ./erpnext-export --force
```

Do not use `--force` to bypass Product Sync freshness or accounting validation.
