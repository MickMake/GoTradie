# GoTradie v0.5 Vendor Identity Documentation Changes

This bundle contains only the Vendor/Provider/Store decisions explicitly locked in review.

## Locked contract

- Vendor identity is the canonical official supplier name only.
- Provider aliases are recognition inputs only.
- An alias may resolve an incoming supplier to a Provider, but the Provider canonical name becomes the Vendor identity.
- Store/location is separate Expense metadata.
- Store/location must not be appended to or encoded into Vendor or Provider identity.
- No fuzzy supplier matching.
- No automatic alias learning.
- No historical-data migration or cleanup behaviour is introduced or discussed in these slices.

## Files changed

- `docs/Design/0.5/Cross-Audit.md`
- `docs/Design/0.5/0.5.3/Hierarchical-Configuration.md`
- `docs/Design/0.5/0.5.3/Implementation-Prompt.md`
- `docs/Design/0.5/0.5.7/Product-Sync.md`
- `docs/Design/0.5/0.5.7/Implementation-Prompt.md`
