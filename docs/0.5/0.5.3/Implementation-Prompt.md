# GoTradie v0.5.3 — Implementation Prompt

**Objective:** Implement [Hierarchical configuration](./Hierarchical-Configuration.md) as the authoritative feature design.

**Implementation Workflow:** [Implementation Workflow](../../Implementation-Workflow.md)

**Suggested branch:** `v0.5.3-hierarchical-config`

## Common implementation rules

- Follow the global [Implementation Workflow](../../Implementation-Workflow.md) for implementation workflow.

## Slice-specific implementation work

- Inspect existing flat configuration consumers and their tests before replacing the loader.
- Update configuration parsing, validation, secret override handling, and consumers as specified by the design.
- Check that existing Invoice Ninja, Bunnings Product lookup/sync/import, Image URL and tax paths still work under the replacement configuration.
- Retire the approved ERPNext command, implementation, tests, configuration and documentation inventory; do not add ERPNext YAML fields.
- Preserve existing `BUNNINGS-` Product keys through the approved temporary internal constant and retain the `bunnings_in` and `image_url` custom-field mappings.
- Do not implement the v0.5.7 Product identity or custom-field transition.
- Update CLI help and affected application documentation; do not introduce backwards compatibility.

## Focused acceptance evidence

Test mandatory-file handling; unknown/invalid fields; complete monthly/quarterly/yearly BAS definitions; required accounting settings; valid hierarchy and provider mapping; defaults/YAML/secret precedence; rejection of generic environment overrides; unchanged Bunnings Product behaviour; and migration failures that should be explicit rather than silent.
