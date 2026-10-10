# Future Feature: Invoice Ninja Setup & Configuration Checks

**Target:** GoTradie v0.5.8 or later. **Not part of PR #11.**

## Purpose

Ensure Invoice Ninja remains correctly configured for GoTradie, particularly for supplier-managed Products. Detect settings changes before they cause incorrect synchronisation.

## Proposed CLI

- `gotradie invoiceninja setup` — inspect configuration, show proposed changes, and request confirmation before applying them. Never change settings silently.
- `gotradie invoiceninja check` — read-only configuration validation with clear errors and warnings.

## Initial Checks

| Invoice Ninja setting | Expected value | Severity |
|---|---|---|
| Auto-update Products | **Off** — invoice edits must not update the supplier catalogue | Error |
| Auto-fill Products | **On** (recommended) | Warning |
| Product custom field 1 | **Supplier** (text) | Error |
| Product custom field 2 | **Store** (text) | Error |
| Product custom field 3 | **Not Available** (boolean) | Error |
| Product custom field 4 | **Last Sync Date** (date) | Error |
| Supplier Vendor validation | Each configured Supplier resolves unambiguously to one active Invoice Ninja Vendor using its canonical name or configured aliases | Error |

Product identity remains **(Supplier custom_value1, native product_key)**. Vendor lookup is validation only; do not depend on Product `vendor_id`.

## Runtime Behaviour

- Run a lightweight, read-only configuration preflight before Product Sync, even if the supplier catalogue fingerprint is unchanged.
- Block synchronisation for incompatible settings or unresolved/ambiguous Vendors; report actionable errors.
- Report non-critical deviations as warnings.
- Consider optional periodic checks later; avoid repeated noisy alerts.

## Scope

A future feature only. Do not modify Product Sync behaviour or expand the current v0.5.7 PR to implement this.
