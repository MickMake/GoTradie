# GoTradie Stage 2 fixes — findings 1 and 2

This package combines the first two documentation-audit fixes.

## 1. v0.5.1 Vendor/Store supersession

The v0.5.1 document remains historically accurate. It is not rewritten to pretend
that store-qualified Vendor display names never existed.

Instead, it now explicitly says that v0.5.3 and later work must follow the
canonical Vendor rule in `docs/Design/0.5/Cross-Audit.md`:

- Vendor identity = canonical official supplier name
- Store/location = separate metadata
- Store/location must not be encoded into Vendor identity

This is a forward contract only. It does not introduce migration or cleanup work.

## 2. v0.5.2 closure

Git history confirms `feature/ninja-expense-import-3` is fully contained in
`main` (ahead by 0 commits).

The patch therefore:

- changes the series status for v0.5.2 to `Closed`;
- changes the v0.5.2 README status to `Closed`;
- removes the stale “pending review and merge” wording;
- changes the detailed UX design from `Draft` to `Closed — accepted v0.5.2 design`.

## Apply

From the repository root:

```bash
git apply GoTradie-stage2-fixes-01-02.patch
```

Then review with:

```bash
git diff
```
