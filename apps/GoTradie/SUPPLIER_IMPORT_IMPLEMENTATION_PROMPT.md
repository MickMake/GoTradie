# GoTradie Supplier Import Implementation Prompt

Use this prompt to start an implementation chat for each v0.6.x phase.

Primary repository: `MickMake/GoBunningsNinja`  
Supporting SDK repositories: `MickMake/GoBunnings`, `MickMake/GoInvoiceNinja`  
Feature family: Supplier CSV/XLSX price list import into Invoice Ninja products

---

## 1. Role

You are the implementation leader for the next `GoTradie supplier` feature phase.

Your job is to inspect the repos, confirm the correct branch and version scope, implement the smallest safe change, update docs/changelog where appropriate, run available checks, push the branch, and raise a PR.

Do not make broad speculative changes. Keep the goblins in one paddock.

---

## 2. Repository context

This project has three Go repositories under `MickMake`:

1. `GoBunnings`
   - Reusable Bunnings API SDK.
   - Owns Bunnings auth/client/models/helpers.
   - Must not import `GoInvoiceNinja` or `GoBunningsNinja`.

2. `GoInvoiceNinja`
   - Reusable Invoice Ninja v5 API SDK.
   - Owns Invoice Ninja auth/client/models/services/helpers.
   - Must not import `GoBunnings` or `GoBunningsNinja`.

3. `GoBunningsNinja`
   - CLI/application layer.
   - Owns commands, config loading, dry-run/commit behaviour, mapping, sync/import/export workflows, and orchestration between SDKs.
   - May import both SDKs.

The supplier import feature belongs in `GoBunningsNinja` unless a genuinely reusable low-level SDK capability is required.

---

## 3. Required workflow before implementation

Before making changes:

1. Inspect the relevant repo or repos.
2. Check for existing unmerged PRs/branches against `main`.
3. Confirm `main` is the correct base.
4. Create a new branch from latest `main`.
5. State the branch name and implementation intent.
6. If there are unmerged PRs that might conflict, stop and report them before coding.

Suggested branch naming:

```text
feature/supplier-import-v0.6.0
feature/supplier-import-v0.6.1
feature/supplier-import-v0.6.2
feature/supplier-import-v0.6.3
feature/supplier-import-v0.6.4
feature/supplier-import-v0.6.5
```

---

## 4. Global rules to preserve

Preserve these behaviours:

1. Commands preview or refuse risky writes by default.
2. `--commit` is the only flag that makes persistent changes.
3. Do not introduce `--dry-run`, `--apply`, or `--force`.
4. Do not modify Invoice Ninja without `--commit`.
5. Do not overwrite files without `--commit`.
6. Keep Bunnings commands separate and read-oriented.
7. Keep native Invoice Ninja CSV import/export native-only.
8. Do not make `ninja import products` understand arbitrary supplier files.
9. Do not move supplier orchestration into SDK repos.
10. Update `CHANGES.md` for meaningful changes.

---

## 5. Target command family

The final v0.6.x command family should be:

```bash
GoTradie supplier init <supplier> <file> [--commit]
GoTradie supplier check <supplier> <file>
GoTradie supplier import <supplier> <file> [--commit]
GoTradie supplier list
```

If the binary is currently named `bunnings-ninja`, use the existing project naming convention in code/help/docs. The conceptual command group is `supplier`.

---

## 6. Desired end-user workflow

The final workflow should support:

```bash
GoTradie supplier init xyz-stupid bloody.csv
GoTradie supplier check xyz-stupid bloody.csv
# edit suppliers/xyz-stupid.yaml if needed
GoTradie supplier check xyz-stupid bloody.csv
GoTradie supplier import xyz-stupid bloody.csv --commit
```

Meaning:

1. Supplier sends a random CSV/XLSX price list.
2. User saves file locally.
3. `init` guesses a mapping profile.
4. `check` validates the mapping.
5. User edits YAML if needed.
6. `check` verifies the fix.
7. `import --commit` creates/updates Invoice Ninja products.

---

## 7. Supplier profile format

Implement the profile around this mental model:

```text
input   = how to read the supplier file
aliases = helper values pulled from the supplier file
fields  = Invoice Ninja product fields to write
```

Preferred YAML shape:

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

### Expression syntax

| Syntax | Meaning |
|---|---|
| `field: Description` | Use supplier column named `Description`. |
| `field: "[3]"` | Use source file column index 3, 1-based. |
| `field: "{sku}"` | Use alias value named `sku`. |
| `field: "REECE-{sku}"` | Literal/template with alias substitution. |
| `field: "=1"` | Literal value `1`. |

Rules:

1. `{name}` refers to an alias, not directly to a raw supplier column.
2. Column indexes are 1-based.
3. Literal values should use the `=` prefix.
4. `fields` should contain only Invoice Ninja product output fields.
5. `aliases` should contain helper values used by fields.

---

## 8. Import semantics

`supplier import` should create and update.

It should mean:

> Read this supplier price list and make Invoice Ninja products match it.

Do not add `supplier sync` in v0.6.x unless explicitly requested later.

### Matching

Match Invoice Ninja products by generated `fields.product` value.

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

### Preview actions

Without `--commit`, output actions should be:

```text
would-create
would-update
unchanged
error
```

### Commit actions

With `--commit`, output actions should be:

```text
created
updated
unchanged
error
```

---

## 9. Suggested internal package design

Prefer keeping supplier logic in one application-level package:

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

Command routing can live with existing command routing code, likely under `internal/app` or equivalent.

Keep the implementation boring and testable. Exciting import code is usually a spreadsheet wearing a false beard.

---

## 10. Version phases

Implement one phase per branch/PR unless explicitly instructed otherwise.

---

### v0.6.0 — supplier command contract and profile foundation

Implement:

1. Supplier command documentation/spec updates.
2. Profile model structs.
3. YAML load/save support.
4. Expression parser/evaluator for field syntax.
5. Profile validation.
6. Unit tests for profile and expression behaviour.
7. `CHANGES.md` update.

Avoid:

1. Invoice Ninja writes.
2. Full CSV/XLSX import workflow.
3. Broad command tree refactors.

Acceptance criteria:

1. Profile YAML can be parsed.
2. Expressions can resolve named columns, indexed columns, aliases, templates, and literals.
3. Invalid profiles return useful errors.
4. Tests pass.

---

### v0.6.1 — supplier init

Implement:

1. File header reader for CSV.
2. XLSX header reader if practical; otherwise document CSV-only for this phase and schedule XLSX for the next phase.
3. Header guessing for common fields:
   - sku / item code / part number
   - description / product description / name
   - price / retail price / trade price
   - image URL
   - unit / UOM
4. `supplier init <supplier> <file>` command.
5. Generated YAML profile under an appropriate profile directory, for example `suppliers/<supplier>.yaml`.
6. Refuse overwrite unless `--commit`.
7. Tests for header guessing and overwrite behaviour.
8. `CHANGES.md` update.

Acceptance criteria:

1. Running `supplier init reece reece.csv` creates a starter YAML profile.
2. Existing profile is not overwritten unless `--commit` is present.
3. Generated profile uses `input`, `aliases`, and `fields` shape.

---

### v0.6.2 — supplier check

Implement:

1. `supplier check <supplier> <file>` command.
2. Load matching supplier YAML profile.
3. Read source rows.
4. Evaluate aliases and Invoice Ninja fields.
5. Validate required aliases/fields.
6. Detect duplicate generated product keys.
7. Print summary and sample mapped rows.
8. Print row-level errors clearly.
9. Tests for validation and duplicate detection.
10. `CHANGES.md` update.

Acceptance criteria:

1. `supplier check` does not require `--commit`.
2. `supplier check` does not write to Invoice Ninja.
3. Bad rows are reported without hiding good rows.
4. Output shows enough information for the user to edit the YAML and rerun check.

---

### v0.6.3 — supplier import preview

Implement:

1. `supplier import <supplier> <file>` preview mode.
2. Validate Invoice Ninja configuration.
3. Fetch existing Invoice Ninja products.
4. Match by generated `product` field.
5. Compare supplier-mapped values against existing product values.
6. Output `would-create`, `would-update`, `unchanged`, and `error`.
7. No remote writes without `--commit`.
8. Tests with fake/mocked Invoice Ninja service if practical.
9. `CHANGES.md` update.

Acceptance criteria:

1. Preview imports never create/update remote products.
2. Existing products are matched by generated product key.
3. Changes are reported clearly.
4. Missing required data becomes an `error` row.

---

### v0.6.4 — supplier import commit

Implement:

1. `supplier import <supplier> <file> --commit` write behaviour.
2. Create products that do not exist.
3. Update products that changed.
4. Leave unchanged products untouched.
5. Continue processing where safe when individual rows fail.
6. Output `created`, `updated`, `unchanged`, and `error`.
7. Tests around create/update decision logic.
8. `CHANGES.md` update.

Acceptance criteria:

1. Remote writes happen only with `--commit`.
2. Created and updated products use mapped field values.
3. Errors are reported per row where practical.
4. The command summary is clear enough for review/audit.

---

### v0.6.5 — polish and hardening

Implement:

1. Improved help text.
2. README examples.
3. Sample supplier profiles.
4. More header detection synonyms.
5. Better diagnostics for bad YAML, missing sheets, missing columns, and invalid prices.
6. More tests.
7. Final docs/spec alignment.
8. `CHANGES.md` update.

Acceptance criteria:

1. New user can follow docs from init to import.
2. Common mistakes produce useful error messages.
3. Spec, README, and command help agree.

---

## 11. PR creation requirements

When the phase is complete:

1. Commit changes to the feature branch.
2. Push the branch.
3. Open a PR against `main`.
4. Include in the PR description:
   - phase/version implemented
   - files changed
   - behaviour added
   - tests run
   - tests not run and why
   - known limitations
   - follow-up phase

Do not claim tests passed unless they actually passed.

---

## 12. Local validation commands

For changes only in `GoBunningsNinja`:

```bash
cd /path/to/GoNinjaWorkspace/GoBunningsNinja
git pull
go test ./...
go vet ./...
go build ./cmd/bunnings-ninja
```

If SDK boundaries are touched, validate all three repos:

```bash
cd /path/to/GoNinjaWorkspace

cd GoBunnings
git pull
go test ./...
go vet ./...

cd ../GoInvoiceNinja
git pull
go test ./...
go vet ./...

cd ../GoBunningsNinja
git pull
go test ./...
go vet ./...
go build ./cmd/bunnings-ninja
```

---

## 13. PR review prompt for branched verification chat

Use this after an implementation PR is raised:

```text
You are reviewing the last supplier import PR for GoTradie / GoBunningsNinja.

Confirm whether the PR is on track for its intended v0.6.x phase.

Check:
1. Correct repo and branch base.
2. No unplanned SDK boundary violations.
3. No accidental changes to Bunnings command behaviour.
4. No accidental changes to native Invoice Ninja CSV import/export.
5. `--commit` safety model preserved.
6. Implementation matches the phase scope.
7. Profile YAML shape uses input + aliases + fields.
8. Tests and docs are appropriate for the phase.
9. CHANGES.md is updated if behaviour changed.
10. Any risks, missing tests, or train derails.

Return:
- Verdict: on track / needs changes / risky
- Specific findings
- Required fixes
- Suggested follow-up items for later phases
```

---

## 14. Do not derail into these topics

Unless explicitly requested, do not implement:

1. `supplier sync`.
2. Supplier catalogue database/cache.
3. Scheduled refresh jobs.
4. Complex pricing/markup engine.
5. Native Invoice Ninja CSV export from `supplier check`.
6. Blank-ID create support in `ninja import products`.
7. Folding Bunnings into `supplier`.
8. Large CLI framework replacement.
9. Unrelated refactors.
10. Cleverness.

Cleverness is where simple tools go to become committee minutes.
