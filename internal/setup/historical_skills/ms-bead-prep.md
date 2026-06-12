---
name: ms-bead-prep
description: Draft a pre-staged implementation prompt for a bead by reading its plan section + shared-helper signatures from prior beads
---

# Bead Prep — pre-staged implementation prompts

Compose a structured implementation prompt for `<bead-id>` and stage it at `<repo>/review/prep/bead<N>_impl_prompt.md`. `/ms-bead-impl` picks it up via its existing `prompt-path` input.

**Why this skill exists.** On lola spec-050, pre-staged prompts were the single biggest quality lever on bead-1 impl-subagent output. A subagent given a structured prompt with explicit files, helpers, ACs, and boundaries produced a passing-first-round implementation; the same subagent given a one-line "implement bead 1" produced rework on round 2. The pattern is documented in `plugins/mindspec/FINDINGS.md` (item 3) and assumed by `/ms-bead-impl` (its `prompt-path` input). This skill makes the staging step a first-class operation rather than ad-hoc orchestrator improvisation.

## Inputs

- `bead-id` (required) — e.g. `lola-8gbp.3`.
- `spec-slug` (optional) — derived from the bead's parent epic if absent.

## Steps

1. **Resolve spec context.**
   ```bash
   bd show <bead-id>
   ```
   Capture: parent epic id, plan path (`.mindspec/docs/specs/<spec-slug>/plan.md`), spec path, declared Rs/ACs, declared `Depends on:` beads.

2. **Extract the plan section.** Grep the bead's `## Bead N — ...` header out of `plan.md` and read through to the next `## Bead` header (or EOF). Capture:
   - Files in scope (look for "Files:" or "Touches:" lines).
   - Tests to add (from the plan's AC list — usually `AC<n>` references).
   - Public API the bead introduces (function names, class names, exported constants).

3. **Read the spec section (if the bead claims specific Rs/ACs).** For each Rn / ACn the bead claims, grep `spec.md` for the matching definition and quote the success criteria verbatim.

4. **Walk the dependency chain.** For each "Depends on:" bead the plan declares:
   - Find the merged helper module(s) — usually under the same package as the bead's primary file (use `git log --oneline bead/<dep-id>` or read the merged commit to locate paths).
   - Read each helper file and extract its public API:
     - Function signatures (with type hints).
     - Exported class names + their public method signatures.
     - Key module-level constants.
   - Note import paths the new bead should use to consume these (no reimplementation).

5. **Compose the prompt.** Write to `<repo>/review/prep/bead<N>_impl_prompt.md` with this skeleton:

   ```markdown
   # Bead <N> implementation prompt — <spec-slug>

   ## Identity
   - Bead id: <bead-id>
   - Branch: bead/<bead-id>
   - Worktree: <abs path>
   - Spec branch: spec/<spec-slug>

   ## Read first
   - `.mindspec/docs/specs/<spec-slug>/spec.md` §R<n> + §AC<n>
   - `.mindspec/docs/specs/<spec-slug>/plan.md` §Bead <N>

   ## Files in scope
   - `<path>` — <one-line purpose>
   - ...

   ## Shared helpers to REUSE (do not reimplement)
   - `from <module> import <name>` — <signature> — landed in bead <M>
   - ...

   ## Tests to add
   - `<test path>::<test name>` — covers AC<n>
   - ...

   ## Boundaries — what NOT to do
   - Do NOT run `mindspec complete` — the cycle owns the merge.
   - Do NOT `git push` — leave the commit local.
   - Do NOT reach beyond the files-in-scope list — no unrelated cleanup, no opportunistic refactors.
   - Do NOT reimplement helpers listed above — import them.

   ## Expected commit shape
   ```
   impl(<spec-slug>, bead <N>): <2-5 word summary>

   - <change 1>
   - <change 2>

   Deviations: <list any prompt items you couldn't address literally, or "none">
   ```

   ## Report back with
   - Commit SHA on `bead/<bead-id>`.
   - Test counts: passed / failed / skipped (run the bead's test scope, not the full repo suite).
   - Flagged deviations from this prompt (one line each + reason).
   ```

6. **Verify file landed.**
   ```bash
   ls -l <repo>/review/prep/bead<N>_impl_prompt.md
   ```

## Anti-patterns

- Don't paraphrase the spec/plan in the prompt — quote verbatim so the subagent reads the canonical text, not your summary.
- Don't list helpers without import paths. "There's a helper in bead 2" is worse than no mention; the subagent will reimplement it.
- Don't include the dep beads' full diffs. Signatures + import paths are sufficient; the subagent reads source if it needs more.
- Don't omit the "what NOT to do" section. Subagents will scope-creep without explicit fences.
- Don't bake test results into the prompt ("the tests pass" before they're written). Specify expected test names; let the subagent run them.

## Then

Hand off to `/ms-bead-impl` with `prompt-path: <repo>/review/prep/bead<N>_impl_prompt.md`. `/ms-bead-impl` will dispatch a `general-purpose` subagent with the prompt as its instructions.
