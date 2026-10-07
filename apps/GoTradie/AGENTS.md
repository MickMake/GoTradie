# GoTradie Agent Rules

## Persistent changes

Use:

```text
--commit
```

for persistent Invoice Ninja changes.

Without `--commit`, remote write-capable operations preview or refuse writes.

## Local export replacement

Use:

```text
--force
```

to overwrite an existing local output file or export directory contents.

Creating new export output does not require `--force`.

`--force` never permits Invoice Ninja writes.

## Flag meanings

```text
--commit = persist Invoice Ninja changes
--force  = replace existing local output
```

Do not introduce `--apply` or `--dry-run`.

Do not use `--force` as a Product Sync freshness override or validation bypass.
