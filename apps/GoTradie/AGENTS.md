# GoTradie Agent Rules

Canonical global CLI contract:

```text
docs/Design/Command-Line-Spec.md
```

This file is a short operational summary for coding agents. It must not redefine or override the global CLI contract.

## Persistent changes

Use:

```text
--commit
```

for persistent Invoice Ninja application or database changes.

Without `--commit`, Invoice Ninja write-capable operations preview or refuse writes.

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

If this summary and `docs/Design/Command-Line-Spec.md` ever appear to disagree, stop and correct this summary rather than inventing a third interpretation.
