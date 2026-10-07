# GoTradie v0.5 docs patch bundle

This bundle contains only the decisions locked during the v0.5 documentation review so far.

Included changes:

- `--commit` is only for persistent changes to Invoice Ninja / remote state.
- `--force` is only for overwriting existing local files.
- Remove stale wording that BAS/EOFY/Financial overwrite semantics are unresolved.
- Update v0.5.2 status to implemented on `feature/ninja-expense-import-3`, pending review/merge.
- v0.5.3 configuration precedence is `defaults -> ~/.GoTradie/config.yaml -> secret environment overrides`.
- `~/.GoTradie/config.yaml` is mandatory.
- Legacy flat configuration support is removed from the v0.5.3 design.
- Environment-variable overrides are supported only for explicitly defined secrets.
- BAS frequency and GST basis are mandatory configuration.
- BAS frequency values: `monthly`, `quarterly`, `yearly`.
- GST basis values: `cash`, `accrual`.
- No Vendor/Store changes are included yet; that discussion was intentionally paused.
