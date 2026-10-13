# GoTradie v0.5.10 - Xero Authentication

**Status:** Design draft for review. No implementation authorised by this document.

**Branch when approved:** `v0.5.10-goxero-auth`  
**Version:** `v0.5.10`  
**Implementation prompt:** [Implementation-Prompt.md](./Implementation-Prompt.md)

## 1. Purpose and stop point

Make the smallest useful direct connection from GoTradie to Xero:

1. Extend GoTradie's configuration.
2. Authenticate using `GoTradie auth xero`.
3. Read Xero's connected organisations to verify the credentials work.
4. **Stop.**

This is the foundation for later receipt-to-existing-bill matching, not that feature itself. No Xero accounting writes or financial-data imports belong in v0.5.10.

## 2. Module boundaries

```text
MickMake/GoTradie (monorepo; Go 1.25)
  apps/GoTradie/          existing CLI / orchestration
  apps/GoXero/            NEW reusable Go library
  apps/GoBunnings/        existing; unchanged
  apps/GoInvoiceNinja/    existing; unchanged
  go.work                 include apps/GoXero after approval
```

Allowed new dependency: `GoTradie -> GoXero`.

`GoXero` must not import GoTradie, GoBunnings or GoInvoiceNinja. `GoXero` owns Xero protocol details, not the application's configuration files, terminal interactions or workflows.

| Responsibility | Owner |
|---|---|
| Xero authorisation URL, PKCE verifier/challenge, state, code exchange | GoXero |
| Xero access/refresh token HTTP exchange, connection discovery | GoXero |
| CLI `auth xero`, browser launch, localhost callback lifetime | GoTradie |
| YAML parsing, file-based credential storage, chosen organisation, user messages | GoTradie |
| Bunnings and Invoice Ninja behaviour | Existing modules, unchanged |

No abstract authentication-provider framework, cross-provider adapters or separate `goxero` executable.

## 3. Configuration contract

GoTradie's existing mandatory, strictly validated configuration remains at:

```text
~/.GoTradie/config.yaml
```

These are the **new fields**, shown as a fragment to merge with existing settings (not a replacement for required BAS/EOFY/tax configuration):

```yaml
verbose: true

providers:
  xero:
    auth:
      client_id: "1DD5ADD9AF034AB2AC8EF8077B2919D7"
      redirect_uri: "http://localhost:8080/callback"
      scopes:
        - offline_access
        - accounting.invoices.read
```

- `verbose` is at the **root**; default is `false` if omitted. For this slice, it controls only safe additional Xero-auth diagnostics. Other CLI behaviour need not be expanded.
- `providers.xero.auth.client_id` is the registered public Xero client ID (not a secret).
- `redirect_uri` must match the Xero application registration exactly and use a loopback callback; its current registered value is shown above.
- `scopes` is a YAML sequence. `offline_access` enables refresh tokens; `accounting.invoices.read` anticipates the first read-only billing use. The connection check itself uses `/connections`.
- Do **not** request attachments write permissions in this slice. Adding permissions for receipt uploads will require later approval and a new consent flow.
- Xero PKCE does not use a client secret. Access/refresh tokens are **never** stored in this YAML file.

The existing `providers` map currently represents supplier/Product Sync providers and validates their `name`, `type` and fields. Xero is a special, non-product entry under this same parent. Extend strict parsing/validation narrowly so that Xero does not appear as a supplier, require `fields.product`, enter Product Sync iteration, or affect provider aliases. Avoid a provider-schema rewrite. Keep existing Bunnings fields where they are; moving Bunnings settings under `auth` is **explicitly deferred**.

Existing required configuration sections and secret-override behaviour remain unchanged. No new environment-variable overrides or CLI flags.

## 4. Registered Xero application

| Setting | Value |
|---|---|
| App name | GoTradie |
| App type | Mobile or desktop app |
| OAuth grant | Authorisation code with PKCE (`S256`) |
| Client ID | `1DD5ADD9AF034AB2AC8EF8077B2919D7` |
| Redirect URI | `http://localhost:8080/callback` |
| Client secret | None |

Use the Xero standard authorization endpoint, token endpoint, and `/connections` endpoint. Request granular OAuth scopes appropriate for an app created in 2026; verify current accepted scopes during implementation preflight rather than silently widening them.

## 5. CLI behaviour

Exactly one new command:

```sh
GoTradie auth xero
```

### First run (no usable saved credentials)

1. GoTradie loads the existing strict YAML and the Xero auth settings.
2. GoTradie starts a **short-lived loopback-only** HTTP listener at the configured callback address.
3. GoXero creates a fresh unpredictable PKCE verifier/challenge and separate random `state`, then returns the Xero authorisation URL.
4. GoTradie opens the system browser. If opening fails, it can print the URL for the user to open on the **same host**; no headless/remote-login feature is promised.
5. The callback receives the code and returned state. GoXero checks the state before exchanging the code. Reject missing/mismatched state, OAuth errors and timed-out callbacks.
6. GoXero exchanges the code for access and refresh tokens. GoTradie immediately saves the tokens safely.
7. GoXero calls read-only `GET https://api.xero.com/connections` with the access token.
8. GoTradie selects an accounting organisation: automatically if exactly one; numbered terminal choice if several; error if none. Persist the chosen `tenantId` and display name.
9. Show connection success and shut down the callback listener.

### Subsequent run

1. Load previously stored tokens and chosen organisation.
2. If necessary, refresh the access token. Save **both** newly returned tokens before any further operation because Xero rotates the refresh token.
3. Call `/connections` and verify the saved organisation still appears. If it does, display success without opening a browser.
4. If credentials have been revoked or cannot be refreshed, offer a fresh interactive authorisation through the same command; never silently claim a valid connection.

A changed scope list does not upgrade existing tokens automatically; new scopes require fresh consent. Do not add `auth status`, `logout`, `ping`, `--tenant`, `--commit`, `--force` or other auth CLI commands in v0.5.10.

### Expected success output

```text
Xero authentication: OK
Organisation: <Xero organisation name>
Connection: OK
```

`/connections` is the ping. It is not a bill read, payment query or organisation mutation. No `xero-tenant-id` header is needed for this connection-discovery endpoint; store the selected `tenantId` for future tenant-scoped calls.

## 6. Credential storage

```text
~/.GoTradie/keys/           directory mode 0700
~/.GoTradie/keys/xero.json  regular file mode 0600
```

The file holds only what is required: current access token, refresh token, expiry and selected organisation identifiers (and any minimal metadata required to prevent mismatching an old token with a different client ID). It is not user-editable YAML configuration.

- Storage is **owned by GoTradie**, not by GoXero. GoXero receives tokens from the caller and returns newly issued tokens; it never assumes macOS Keychain or a specific token path.
- Use a temporary file in the same directory and atomic replacement for token updates so rotated refresh tokens are not lost by a partially written file. Apply restrictive permissions to the directory, temporary file and final file.
- Read permissions are a portability choice, **not encryption at rest**. Anyone with access to the user's account/files may be able to use the stored refresh token. Keep the file out of repositories, backups shared with others and verbose logs.
- Do not store PKCE verifiers, state or callback authorisation codes beyond the active login attempt.
- Avoid installing a general credential-provider framework. A later slice can replace GoTradie's storage without changing GoXero's OAuth protocol API.

## 7. Minimal GoXero public API

Keep the library API limited to authentication and connection discovery. The exact identifiers may be reviewed during implementation preflight, but the boundary and operations are fixed:

```go
// Illustrative public API shape; not an implementation.
type Config struct {
    ClientID    string
    RedirectURI string
    Scopes      []string
}

type Tokens struct {
    AccessToken  string
    RefreshToken string
    Expiry       time.Time
}

type Connection struct {
    TenantID   string
    TenantName string
    TenantType string
}

// AuthSession holds ephemeral verifier and expected state privately.
func NewClient(cfg Config, httpClient *http.Client) (*Client, error)
func (c *Client) BeginAuth() (session *AuthSession, authURL string, err error)
func (c *Client) CompleteAuth(ctx context.Context, session *AuthSession, code, returnedState string) (Tokens, error)
func (c *Client) Refresh(ctx context.Context, refreshToken string) (Tokens, error)
func (c *Client) Connections(ctx context.Context, accessToken string) ([]Connection, error)
```

Notes:

- GoXero creates and validates PKCE/state; GoTradie owns receiving the localhost callback.
- Use `context.Context` and an injectable `http.Client` so local HTTP test servers cover the flow without live Xero calls.
- The caller persists returned tokens, including every refresh token rotation.
- No public invoice/bill, attachment, tracking-category or generic accounting interfaces yet.

## 8. Verbose and essential error handling

`verbose: false` gives short user-facing progress and the success/failure result. `verbose: true` may include safe operation names, API endpoint paths, HTTP statuses, token-refresh **events** and callback lifecycle details.

Never print raw bearer tokens, refresh tokens, authorisation codes, PKCE verifier, OAuth state, Authorization headers or token response bodies. Basic failures should be clear: missing configuration, unable to bind the callback port, declined/invalid callback, token request failure, corrupt/unwritable credential file, no organisations, and failed connection check. No large retry policy or error-code catalogue.

## 9. Acceptance criteria

- Works with the existing strict YAML, while leaving Bunnings and existing Product Sync providers alone.
- OAuth2 PKCE `S256` and callback state checking work using deterministic fake HTTP services.
- First run obtains tokens; after a process restart the same selected organisation passes the `/connections` check without a new browser login.
- Expired access token refreshes; both rotated tokens are persisted safely; invalid grants lead to re-authorisation instead of fabricated success.
- When multiple connected organisations are returned, user selection is explicit and persisted. With one organisation, no prompt is needed.
- Linux/macOS credential location and permissions are supported without OS-specific Keychain APIs.
- `verbose: true` produces useful but secret-free diagnostics.
- Only one read-only API endpoint is exercised after authentication: `GET /connections`. No Xero bills, attachments, items or tracking metadata are modified.

## 10. Explicitly deferred

Receipt scanning/matching/uploading, existing-draft-bill updates, tracking categories, other Xero accounting API methods, migration of Bunnings credentials under `auth`, OS keychains, headless/remote login, new CLI auth commands, and general provider/auth abstractions.

## References

- [GoTradie global implementation workflow](../../Implementation-Workflow.md)
- [GoTradie global CLI contract](../../Command-Line-Spec.md)
- [v0.5.3 hierarchical configuration](../0.5.3/Hierarchical-Configuration.md)
- [Xero OAuth2 overview](https://developer.xero.com/documentation/guides/oauth2/overview)
- [Xero PKCE flow](https://developer.xero.com/documentation/guides/oauth2/pkce-flow/)
- [Xero token refresh and rotation](https://developer.xero.com/documentation/guides/oauth2/token-types/)
- [Xero tenants and connections](https://developer.xero.com/documentation/guides/oauth2/tenants/)
- [Xero OAuth2 scopes](https://developer.xero.com/documentation/guides/oauth2/scopes/)
