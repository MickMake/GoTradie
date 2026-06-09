# GoTradie Supplier Import Overview

Version plan: v0.6.x  
Primary repo: `MickMake/GoBunningsNinja`  
Status: Design / implementation planning document

---

## 1. Executive summary

Add a new `supplier` command family to GoTradie / GoBunningsNinja for importing arbitrary supplier CSV/XLSX price lists into Invoice Ninja products.

The core workflow is:

```bash
GoTradie supplier init xyz-stupid bloody.csv
GoTradie supplier check xyz-stupid bloody.csv
# edit suppliers/xyz-stupid.yaml if needed
GoTradie supplier check xyz-stupid bloody.csv
GoTradie supplier import xyz-stupid bloody.csv --commit
```

The feature should support messy vendor price lists without polluting the existing `bunnings` commands or the native Invoice Ninja CSV commands.

Bunnings remains a separate top-level command because it is a different beast: it has live search/lookup/sync behaviour, not just file-based supplier import.

Native Invoice Ninja commands remain native-only:

```bash
GoTradie ninja export products products.csv
GoTradie ninja import products products.csv
```

The new supplier workflow converts arbitrary supplier files through editable YAML profiles and then previews or commits create/update operations into Invoice Ninja.

---

## 2. Design goals

1. Keep the user workflow short and practical.
2. Support CSV and XLSX supplier price lists.
3. Generate an editable YAML mapping profile from supplier files.
4. Let `init` guess, `check` validate, and `import` act.
5. Keep Invoice Ninja writes safe by default.
6. Preserve the existing `--commit` safety model.
7. Avoid changing native Invoice Ninja import/export behaviour.
8. Keep Bunnings as its own command family.
9. Keep supplier mapping/orchestration in `GoBunningsNinja`, not in SDK repos.
10. Implement in small versioned phases starting at v0.6.0.

---

## 3. Command model

### v1 command family

```bash
GoTradie supplier init <supplier> <file> [--commit]
GoTradie supplier check <supplier> <file>
GoTradie supplier import <supplier> <file> [--commit]
GoTradie supplier list
```

### Command meanings

| Command | Meaning |
|---|---|
| `supplier init` | Inspect a supplier CSV/XLSX file and create a starter YAML profile. |
| `supplier check` | Validate a supplier file against its YAML profile and show mapped output/errors. |
| `supplier import` | Preview or commit create/update operations into Invoice Ninja products. |
| `supplier list` | List available supplier profiles. |

---

## 4. Why `import` should create and update

`supplier import` should do both create and update.

It should mean:

> Read this supplier price list and make Invoice Ninja products match it.

Do not split v1 into:

```bash
GoTradie supplier import ...
GoTradie supplier sync ...
```

That adds ceremony and does not match the real workflow. A separate `supplier sync` may be useful later if GoTradie gains cached supplier catalogues, historical supplier snapshots, or scheduled refreshes. For v0.6.x, the user has a file in hand, so `supplier import` is the right verb.

---

## 5. Safety model

All existing safety rules remain:

```text
Commands preview or refuse risky writes by default.
Use --commit to make persistent changes.
```

### Supplier safety behaviour

| Command | Default behaviour | With `--commit` |
|---|---|---|
| `supplier init` | Create new profile only; refuse overwrite. | May overwrite existing profile. |
| `supplier check` | Validate and report only. | Not needed in v1. |
| `supplier import` | Preview only. | Create/update Invoice Ninja products. |
| `supplier list` | Read-only. | Not applicable. |

### Import actions

Without `--commit`:

```text
would-create
would-update
unchanged
error
```

With `--commit`:

```text
created
updated
unchanged
error
```

---

## 6. YAML profile model

The profile should be built around this mental model:

```text
input   = how to read the supplier file
aliases = helper values pulled from the supplier file
fields  = Invoice Ninja product fields to write
```

Preferred v1 shape:

```yaml
supplier: reece

input:
  type: auto
  sheet: Products

aliases:
  sku:
    field: Item Code
    required: true

fields:
  product:
    field: "REECE-{sku}"
    required: true

  description:
    field: Description
    required: true

  price:
    field: "[3]"
    required: true

  default_quantity:
    field: "=1"

  max_quantity:
    field: "=999"

  image_url:
    field: Image URL
```

### Profile sections

| Section | Purpose |
|---|---|
| `supplier` | Supplier/profile name. |
| `input` | File reading hints: type, sheet, header row, etc. |
| `aliases` | Named helper values used by field templates. |
| `fields` | Invoice Ninja fields to populate. |

### Expression rules

| Syntax | Meaning |
|---|---|
| `field: Description` | Use supplier column named `Description`. |
| `field: "[3]"` | Use source file column index 3, 1-based. |
| `field: "{sku}"` | Use alias value named `sku`. |
| `field: "REECE-{sku}"` | Literal/template with alias substitution. |
| `field: "=1"` | Literal value `1`. |

Important rule:

> `{name}` refers to an alias, not directly to a raw supplier column name.

This avoids making templates depend on ugly supplier headers such as `Product/SKU`, `Item No.`, or `Supplier Stock #`.

---

## 7. Matching strategy

Invoice Ninja products should be matched by the Invoice Ninja `product` field generated from the supplier profile.

Example:

```yaml
aliases:
  sku:
    field: Item Code

fields:
  product:
    field: "REECE-{sku}"
```

Supplier row:

```text
Item Code = 12345
```

Invoice Ninja product key:

```text
REECE-12345
```

This avoids relying on native Invoice Ninja CSV IDs and allows supplier import to create new products as well as update existing products.

---

## 8. Suggested internal packages

Suggested source structure in `GoBunningsNinja`:

```text
internal/supplier/
  profile.go       # YAML profile model and loading
  detect.go        # header/index guessing for init
  reader.go        # CSV/XLSX reader abstraction
  expr.go          # field expression parsing/evaluation
  mapper.go        # supplier row -> Invoice Ninja product input
  check.go         # validation/reporting
  importer.go      # preview/commit create/update workflow
```

CLI routing remains wherever existing commands live, likely under `internal/app` or equivalent.

---

## 9. Version plan

### v0.6.0 — supplier command contract and profile foundation

Goals:

1. Add command/spec documentation for the `supplier` command family.
2. Add profile model.
3. Add YAML load/save.
4. Add expression parser for `field` values.
5. Add tests for expression parsing and profile validation.
6. Add `supplier list` if profile directory discovery is straightforward.

No Invoice Ninja writes yet.

---

### v0.6.1 — supplier init

Goals:

1. Add CSV reader.
2. Add basic XLSX reader if dependency is already acceptable; otherwise CSV first and XLSX in v0.6.2.
3. Add header detection.
4. Implement `supplier init <supplier> <file>`.
5. Generate starter YAML with guessed aliases/fields.
6. Refuse overwrite unless `--commit`.
7. Add tests around header guessing.

---

### v0.6.2 — supplier check

Goals:

1. Load supplier profile.
2. Read CSV/XLSX rows.
3. Evaluate aliases and fields.
4. Validate required aliases/fields.
5. Report row counts, valid rows, errors, duplicate product keys, and sample mapped output.
6. Keep output predictable and script-friendly.

No Invoice Ninja writes.

---

### v0.6.3 — supplier import preview

Goals:

1. Validate Invoice Ninja config.
2. Load existing Invoice Ninja products.
3. Match by generated `product` field.
4. Compare mapped supplier values to existing products.
5. Print preview actions: `would-create`, `would-update`, `unchanged`, `error`.
6. No writes without `--commit`.

---

### v0.6.4 — supplier import commit

Goals:

1. Implement create/update via Invoice Ninja API.
2. Persist only with `--commit`.
3. Print commit actions: `created`, `updated`, `unchanged`, `error`.
4. Add tests where practical; use mocks/fakes rather than live API.
5. Update README/CHANGES/spec.

---

### v0.6.5 — polish and hardening

Goals:

1. Improve command help.
2. Improve error messages.
3. Add sample supplier profiles.
4. Add more detection synonyms.
5. Add duplicate handling options if needed.
6. Tighten tests.
7. Review docs against implemented behaviour.

---

## 10. Explicit non-goals for v0.6.x

Do not add these unless deliberately promoted into scope:

1. `supplier sync` command.
2. Scheduled supplier refreshes.
3. Supplier catalogue history.
4. Complex pricing/markup engine.
5. Native Invoice Ninja CSV generation from `supplier check`.
6. Changing `ninja import products` to create blank-ID products.
7. Moving supplier logic into `GoBunnings` or `GoInvoiceNinja`.
8. Folding Bunnings under `supplier`.

These may be good later, but they are not v0.6.x foundations.

---

## 11. Review checklist for each PR

Each implementation PR should verify:

1. Is the change in the correct repo?
2. Does it preserve SDK boundaries?
3. Does it preserve the `--commit` safety model?
4. Does it avoid changing Bunnings command behaviour?
5. Does it avoid changing native Invoice Ninja import/export behaviour?
6. Does it update `CHANGES.md` for meaningful changes?
7. Does it update command/spec docs if CLI behaviour changed?
8. Are tests present for the new behaviour?
9. Were tests actually run?
10. Is the implementation small enough for review?

---

## 12. Final intended workflow

The desired user workflow is:

```bash
GoTradie supplier init xyz-stupid bloody.csv
GoTradie supplier check xyz-stupid bloody.csv
# Edit suppliers/xyz-stupid.yaml if init guessed wrong.
GoTradie supplier check xyz-stupid bloody.csv
GoTradie supplier import xyz-stupid bloody.csv --commit
```

Then go and have a beer.

If the software cannot support the beer step, the software has failed at a strategic level.
