# GoTradie — Implementation Workflow

This workflow applies to every implementation slice. Slice-specific prompts must reference this file rather than duplicate these instructions.

## 1. Preflight — no code changes

1. Fetch the latest `origin/main` and inspect the current branch and working tree.
2. Check earlier slice branches and pull requests. Confirm predecessors are merged into `main` or explicitly retired. Stop and report unresolved active predecessors.
3. Read the slice's authoritative design, the global CLI contract, and the slice implementation prompt. When instructions conflict or the design is ambiguous, stop and ask rather than inventing an interpretation.
4. Inspect existing implementation, tests, affected Go modules, and available integrations.
5. Present a concise plan showing:
   - proposed branch name and base commit;
   - affected files/packages and expected scope;
   - intended approach and how existing code will be reused;
   - proposed new packages, interfaces, dependencies, persistence, caches, or abstractions;
   - design ambiguities, external-data gaps, and risks.
6. **STOP. Obtain explicit user approval before creating a branch or modifying code.**

## 2. Implementation

1. After approval, fetch the latest `origin/main`. If `main` has advanced materially since preflight, reassess the plan before proceeding.

   Create the agreed feature branch explicitly without inheriting upstream tracking:

   ```bash
   git fetch origin main
   git switch --no-track -c <branch-name> origin/main
   ```

   Record the current remote `main` commit before publishing:

   ```bash
   git ls-remote --heads origin main
   ```

   Publish the feature branch using an explicit destination:

   ```bash
   git push -u origin HEAD:refs/heads/<branch-name>
   ```

   Verify that:
   - The current branch is `<branch-name>`.
   - Its upstream is `origin/<branch-name>`, NEVER `origin/main`.
   - The remote feature branch exists.
   - The remote `main` commit remains unchanged.

   Use `git branch -vv` and `git ls-remote --heads origin` to verify these conditions.

   **Git safety rules:**
   - NEVER push implementation commits directly to `main`.
   - NEVER rely on implicit push destinations. Always specify the feature branch destination explicitly.
   - NEVER configure a feature branch to track `origin/main`.
   - NEVER force-push or rewrite `main`.
   - All implementation changes must reach `main` through a reviewed PR with explicit user approval.
   - If authentication, branch tracking, push destinations, or remote state are unexpected, STOP and report the issue before proceeding.

2. Implement only the approved slice and its necessary tests and documentation changes.
3. Prefer existing facilities. Do not introduce speculative infrastructure or build later slices early.
4. Any unapproved substantial change to architecture, persistence, external dependencies, or scope requires another approval checkpoint. Do not silently expand the work.
5. Add focused tests using deterministic fixtures and fakes where appropriate.
6. Run formatting, vetting, tests, and build for each affected Go module using the appropriate module-relative commands: `gofmt`, `go vet`, `go test`, and `go build`. Record commands and outcomes. Report checks that cannot be run.

## 3. Critical self-review — maximum three cycles

After implementation and initial verification, perform a **critical review of the actual diff** against the accepted design and approved plan. Passing tests alone does not constitute a review.

For each cycle:

1. Inspect the changed code and tests for correctness, boundary cases, data integrity, regressions, security, and design compliance.
2. Assess proportionality: unnecessary abstractions, duplicated logic, added dependencies, persistence, or code outside slice scope.
3. Record concrete findings with file/line references and supporting evidence. Distinguish defects from preferences.
4. Fix substantiated findings, rerun relevant verification, then review the changed diff again.

Stop when review passes or after **three review/fix cycles**, whichever occurs first. Never loop indefinitely. Report unresolved findings; do not claim a clean review if issues remain.

## 4. Completion and handoff

Provide:
- branch, base commit, and resulting commit(s);
- summary of behaviour implemented and files changed;
- verification commands and results;
- review cycles, findings, fixes, and unresolved risks;
- any approved design deviations or external checks not performed.

**STOP for user approval before merging.** Do not merge, expand to another slice, or change accepted design without explicit approval.

## Authority

The slice design document defines **what** to implement. Its implementation prompt defines slice-specific tasks and evidence. The global CLI contract defines common CLI semantics. This file defines **how** implementation work is conducted.

If these sources conflict, stop and identify the conflict rather than silently choosing a rule.
