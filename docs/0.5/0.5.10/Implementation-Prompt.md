# GoTradie v0.5.10 - Implementation Prompt

**Objective:** Implement [Xero Authentication](./Xero-Auth.md) as the authoritative v0.5.10 design: configuration, `GoTradie auth xero`, a read-only connection check, and then stop.

**Implementation Workflow:** [Implementation Workflow](../../Implementation-Workflow.md)

**CLI contract:** [Command-Line Contract](../../Command-Line-Spec.md)

**Branch:** `v0.5.10-goxero-auth` (created from the latest `origin/main` only after approval).

**Release version:** `v0.5.10`.

**Status:** Planned. This prompt does not authorise branch creation or code changes.

## Common implementation rules

- Follow [docs/Implementation-Workflow.md](../../Implementation-Workflow.md) in full. Its preflight, explicit approval gates, branch/push safeguards, module verification, maximum three critical review/fix cycles, and merge approval are mandatory. Do not duplicate or supersede that workflow here.
- Read [Xero-Auth.md](./Xero-Auth.md), the [global CLI contract](../../Command-Line-Spec.md), repository `AGENTS.md`, `apps/GoTradie/AGENTS.md`, and the affected implementation before proposing edits.
- Inspect the latest `main`, existing branches and PRs. An earlier planned-but-unimplemented slice is not a prerequisite unless v0.5.10 genuinely depends on it. An active unmerged branch/PR remains subject to the global workflow and the user's branch hygiene rules.
- Present the exact affected packages/files, public GoXero API, configuration/validation changes, persistence approach, tests, proposed branch and base SHA. **STOP for explicit approval before creating the branch or changing code.**

## Decisions settled for this slice

- `GoXero` is an **independent reusable Go module/library** at `apps/GoXero/` (its `go.mod` scaffold already exists), not a CLI. `GoTradie` owns command dispatch, browser launch, localhost listener, user prompts and token-file persistence. Dependency: `GoTradie -> GoXero` only.
- The **only** new command is `GoTradie auth xero` (verb then provider). No other auth verbs, flags, separate executable or Xero data commands.
- Xero application type is **Mobile or desktop app**, OAuth 2.0 authorisation code with **PKCE S256**, with registered redirect `http://localhost:8080/callback`. No client secret.
- Add root-level `verbose: true|false` (default `false`) and `providers.xero.auth` with `client_id`, `redirect_uri` and `scopes` to the existing strict `~/.GoTradie/config.yaml` schema.
- Support the approved `XERO_CLIENT_ID` environment override: a non-blank value takes precedence over `providers.xero.auth.client_id` in YAML; an absent/blank value uses YAML. Preserve existing `INVOICE_NINJA_TOKEN` and `BUNNINGS_CLIENT_SECRET` overrides. Do not add any other environment overrides.
- Keep credentials outside YAML under `~/.GoTradie/keys/xero.json`; the `keys` directory is `0700` and credential file `0600`. Use safe atomic replacement when tokens rotate. No macOS Keychain dependency; retain macOS/Linux portability.
- `GoXero` handles PKCE/state, code exchange, refresh and Xero `/connections` HTTP calls. `GoTradie` reads/saves tokens, stores the chosen tenant and controls the CLI lifetime.
- On a repeat invocation, use saved credentials (refreshing if required) and check the connection without needlessly opening a browser. On no usable credentials, perform interactive authorisation.
- A successful read-only `GET https://api.xero.com/connections` for the selected organisation is the complete ping/acceptance operation. No bill, contact, item, attachment or tracking-category operation.
- Xero is **not** a Product Sync supplier. Existing provider validation, supplier identity and sync traversal must continue to behave as before. Do not move GoBunnings settings under `auth` in this slice.
- Honour the existing CLI semantics: do not invent `--dry-run`/`--apply`; `--commit` governs persistent Invoice Ninja / Xero accounting-data writes, while `--force` governs local output overwrite. `GoTradie auth xero` may persist local tokens without either flag and must not accept them.
- No tokens, refresh tokens, authorisation codes, PKCE verifiers or secret-bearing HTTP messages may be logged, including with `verbose: true`.

## Slice-specific implementation work

1. Inspect `apps/GoTradie/internal/config`, command dispatch/help and Product Sync provider traversal, and the existing module/workspace layout (`go.work`, `go.mod`, `AGENTS.md`). Determine the smallest way to accept `providers.xero.auth` without making Xero a product supplier or loosening existing strict YAML validation.
2. Propose and review the minimal GoXero public API described in [Xero-Auth.md](./Xero-Auth.md). Use existing Go HTTP and testing facilities where practical; no generic provider/authentication framework.
3. After implementation approval, build the library inside the existing `apps/GoXero` module scaffold (`go.mod` already exists). `go.work` already lists it. Add only the necessary GoTradie dependency/wiring and make required module-description updates (for example, root `AGENTS.md`, whose current module list predates GoXero). Do not recreate the module or workspace entry.
4. Add root `verbose` and nested `providers.xero.auth` configuration with the single approved `XERO_CLIENT_ID` override; keep all current config keys and supplier behaviour compatible. Do not modify GoBunnings or GoInvoiceNinja source.
5. Implement `GoTradie auth xero` with the short-lived loopback callback, PKCE/state validation, token exchange and refresh, organisation selection, safe credential persistence, and one read-only connections request.
6. Add a narrow command-help/config-example update and set the displayed GoTradie version to `v0.5.10`. Do not implement subsequent receipt matching or upload commands.
7. Run module-specific `gofmt`, `go vet`, `go test` and `go build` as required by the global workflow, then critically review the actual diff. Stop after the agreed slice and wait for approval before merge or further work.

## Focused acceptance evidence

- Strict YAML accepts root `verbose` and `providers.xero.auth`; rejects unknown/malformed fields. Verify `XERO_CLIENT_ID` override precedence and unset/blank fallback to YAML. Existing Bunnings and other Product Sync providers continue to parse, validate and sync independently; Xero is excluded from their traversal.
- First `GoTradie auth xero` invocation completes PKCE login and state-checked localhost callback, selects the only organisation automatically or prompts for a choice when there are several, and reports the selected connection.
- A second invocation after process restart uses persisted credentials; expired access tokens refresh with the newly rotated refresh token saved. A lost/invalid grant has a clear re-authorisation path.
- Credential directory/file permissions and atomic token replacement are checked. `verbose: true` adds safe diagnostics, never credentials.
- Missing callback, incorrect state, failed token exchange, no connected organisation and failed `/connections` check fail clearly without changing financial data.
- A fake Xero HTTP server and deterministic tests cover the path. One manual live Xero check may be performed with user-supplied consent; the test suite must not require Xero credentials.
- Verify there are **no** bill mutations, attachment uploads, tracking-category changes, GoBunnings moves, GoInvoiceNinja changes or speculative extra CLI commands.

**Completion gate:** Present files, branch/commits, test results, review findings and remaining risks. **STOP.** Do not merge or start the PDF-receipts slice without separate approval.
