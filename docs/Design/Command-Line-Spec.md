# GoTradie Command-Line Contract

## Authority and scope

This document is the **single canonical contract for global GoTradie CLI semantics and safety rules that it explicitly defines**.

Slice-specific design documents may define their own commands, arguments and feature behaviour, but they must not redefine global flag meanings or contradict the safety rules in this document.

User documentation, application help and agent instructions may summarise these rules. They are not alternate CLI contracts.

## Core rule

GoTradie uses distinct flags for distinct kinds of persistent effects:

```text
--commit
    Permit persistent changes to Invoice Ninja application or database state.

--force
    Permit overwriting existing local files.
```

These meanings must never overlap.

## Write safety

| Operation | Default | Override |
|---|---|---|
| Invoice Ninja create/update/archive/write | Preview/refuse | `--commit` |
| Create new local output file | Allowed | none |
| Overwrite existing local output file | Refuse | `--force` |
| Read-only discovery/API lookup | Allowed | none |

### `--commit`

`--commit` is the only flag that permits persistent changes to Invoice Ninja application or database state.

Without it, write-capable commands must preview or otherwise remain non-mutating.

Examples:

```bash
GoTradie sync refresh --commit
GoTradie sync import 0123456 --commit
GoTradie sync search "merbau decking" --create --select=0123456 --commit
GoTradie ninja import products products.csv --commit
GoTradie ninja import clients clients.csv --commit
GoTradie ninja import expenses purchases.csv --commit
```

### `--force`

`--force` permits replacement of an existing local output file or directory where the command's output contract supports replacement.

Examples:

```bash
GoTradie ninja export products products.csv --force
GoTradie ninja export clients clients.csv --force
GoTradie ninja export quotes quotes.csv --force
GoTradie ninja export invoices invoices.csv --force
GoTradie ninja export payments payments.csv --force
GoTradie ninja export erpnext ./erpnext-export --force
GoTradie ninja export tax ./tax-export --force
```

`--force` does not permit Invoice Ninja writes.

`--force` must not be used for Product Sync freshness, bypassing validation, bypassing accounting safety, or as another spelling of `--commit`.

## Generated export output directory

Generated BAS, EOFY and Financial XLSX exports use this output-directory precedence:

```text
explicit output path, if the command supports one
    ↓
exports.directory, if configured
    ↓
current working directory
```

`exports.directory` is optional.

Example:

```yaml
exports:
  directory: ~/Documents/GoTradie
```

If `exports.directory` is absent, generated exports are written to the current working directory.

Do not require users to configure an export directory merely to use generated reports.

The resolved target path is the path used by the `--force` overwrite rule.

## Generated export filename contract

Generated XLSX filenames are deterministic and must not contain timestamps or automatically appended collision suffixes.

### BAS

Quarterly single-period BAS:

```text
FY2027-BAS-Q1.xlsx
FY2027-BAS-Q2.xlsx
FY2027-BAS-Q3.xlsx
FY2027-BAS-Q4.xlsx
```

Monthly single-period BAS:

```text
FY2027-BAS-Jul.xlsx
FY2027-BAS-Aug.xlsx
...
FY2027-BAS-Jun.xlsx
```

Historical `--fy` with no `--period`, which contains all periods for that FY:

```text
FY2025-BAS.xlsx
```

Yearly BAS:

```text
FY2027-BAS.xlsx
```

Do not use ambiguous names such as `P1`, because period numbers mean different things under monthly and quarterly reporting.

### EOFY

```text
FY2027-EOFY.xlsx
```

### Financial

Financial-year export:

```text
FY2027-Financial.xlsx
```

Explicit date range:

```text
Financial-2025-01-01-to-2025-06-30.xlsx
```

Unrestricted export:

```text
Financial-All.xlsx
```

Where a one-sided date range is supported, use:

```text
Financial-from-2025-01-01.xlsx
Financial-to-2025-06-30.xlsx
```

## Export overwrite rule

For local exports:

| Case | Behaviour |
|---|---|
| Output does not exist | Create it |
| Output exists, no `--force` | Refuse |
| Output exists, `--force` | Replace it |
| Output is stdout (`-`) | Write stdout; `--force` irrelevant |

Generated BAS/EOFY/Financial output follows the same local-file rule.

Do not automatically rename an output to avoid a collision.

## Generated report outcomes

BAS, EOFY and Financial reports use one shared severity and exit-status contract.

### Severities

```text
INFO
WARNING
ERROR
```

Do not add parallel user-facing severities such as `fatal`, `critical`, `severe` or `partial`.

#### INFO

Informational only.

An INFO item does not reduce confidence in the report.

Behaviour:

```text
report remains valid
workbook may record the information where useful
exit status = 0
```

#### WARNING

Something is imperfect or noteworthy, but the report remains trustworthy for its intended purpose.

A WARNING must not mean that reported accounting figures may secretly be materially wrong.

Behaviour:

```text
write workbook normally
record warning in Exceptions sheet
print warning to console
report status remains complete/valid
exit status = 0
```

Examples include stale-but-usable operational metadata such as ATO due-date verification.

If an uncertainty could materially alter the requested accounting result, classify it as ERROR instead.

#### ERROR

An accounting/data condition prevents GoTradie from confidently completing some part of the requested report.

Examples include:

```text
ambiguous supplier settlement
required GST treatment missing
required accounting date missing
payment cannot be allocated deterministically
unsupported transaction required by the calculation
```

Where the workbook can still be produced meaningfully:

```text
write the diagnostic workbook
mark report status = INCOMPLETE prominently on Summary
record the error in Exceptions
print the error to console
exit status = 1
```

Never silently omit an errored record and produce a workbook that appears complete.

### Execution failure

Some failures prevent a meaningful workbook from being produced at all, for example:

```text
invalid required configuration
Invoice Ninja unavailable before required data can be read
required source response cannot be parsed
output path cannot be written
XLSX creation fails
```

Behaviour:

```text
no workbook is required
print error to console
exit status = 1
```

An execution failure is not a fourth reporting severity; it is failure to produce the report.

### Exit-status contract

Keep exit status deliberately simple:

```text
0 = report completed successfully; INFO/WARNING items may exist
1 = report incomplete or report execution failed
```

Do not create a large catalogue of numeric exit codes for individual accounting exceptions.

## Removed/rejected meanings

Do not reintroduce:

```text
--dry-run
--apply
```

Do not use `--force` to mean:

```text
persist Invoice Ninja application or database changes
ignore Product Sync freshness
bypass validation
```

## Plain-English contract

```text
Look around freely.
Preview Invoice Ninja application or database changes safely.
Use --commit to change Invoice Ninja.
Use --force to overwrite an existing local file.
Generated reports go to exports.directory when configured, otherwise the current directory.
Warnings remain successful.
Accounting errors produce an INCOMPLETE diagnostic workbook when possible and exit 1.
Execution failures exit 1 and need not produce a workbook.
```
