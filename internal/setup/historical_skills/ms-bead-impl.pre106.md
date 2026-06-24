---
name: ms-bead-impl
description: Stage a pre-built implementation prompt for a claimed bead (Phase A), then dispatch a general-purpose subagent to land one PR-sized commit (Phase B)
---

# Bead Implementation Dispatch

Land one PR-sized implementation for a single claimed bead. You orchestrate; the subagent codes. Two phases: **A** stages a structured prompt, **B** dispatches the subagent with it. Phase A was previously the separate `/ms-bead-prep` skill; it is folded in here so prompt-staging is part of dispatch, not ad-hoc improvisation.

**Why staged prompts matter.** On lola spec-050, pre-staged prompts were the single biggest quality lever on impl-subagent output. A subagent given a structured prompt with explicit files, helpers, ACs, and boundaries produced a passing-first-round implementation; the same subagent given a one-line "implement bead 1" produced rework on round 2. The pattern is documented in `plugins/mindspec/FINDINGS.md` (item 3).

Include the standard guardrails (AGENTS.md § Bead-loop guardrails) in every subagent prompt — do not re-transcribe them here.

## Inputs

- `bead-id` (required) — the `bd` id, e.g. `lola-8gbp.3`.
- `prompt-path` (optional) — pre-staged implementation prompt at `<repo>/review/prep/bead<N>_impl_prompt.md`. If supplied, skip Phase A and read it; if absent, run Phase A to stage one.

## Phase A — stage the prompt

Skip this phase if `prompt-path` was supplied. Otherwise compose a structured prompt and stage it at `<repo>/review/prep/bead<N>_impl_prompt.md`.

1. **Resolve spec context.**
   ```bash
   bd show <bead-id>
   ```
   Capture: parent epic id, plan path (`.mindspec/docs/specs/<spec-slug>/plan.md`), spec path, declared Rs/ACs, declared `Depends on:` beads. Status must be `open` or `in_progress`.

2. **Extract the plan section.** Grep the bead's `## Bead N — ...` header out of `plan.md` and read through to the next `## Bead` header (or EOF). Capture:
   - Files in scope (look for "Files:" or "Touches:" lines).
   - Tests to add (from the plan's AC list — usually `AC<n>` references).
   - Public API the bead introduces (function names, class names, exported constants).

3. **Read the spec section (if the bead claims specific Rs/ACs).** For each Rn / ACn the bead claims, grep `spec.md` for the matching definition and quote the success criteria verbatim.

4. **Walk the dependency chain.** For each `Depends on:` bead the plan declares:
   - Find the merged helper module(s) — usually under the same package as the bead's primary file (use `git log --oneline bead/<dep-id>` or read the merged commit to locate paths).
   - Read each helper file and extract its public API: function signatures (with types), exported class names + public method signatures, key module-level constants.
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

   ## Guardrails
   Include the standard guardrails (AGENTS.md § Bead-loop guardrails) — the
   subagent prompt fences (no `mindspec complete`, no `git push`, no scope
   creep, no reimplementing helpers, exactly ONE commit ending
   `Deviations: <list or "none">`, tests must PASS, report SHA + counts).

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

## Phase B — dispatch the subagent

1. **Confirm the bead worktree.** The cycle's Step 0 (`/ms-bead-cycle`) already claimed the bead and created `<spec-worktree>/.worktrees/worktree-<bead-id>/` on branch `bead/<bead-id>`. Verify it exists; if it is missing, re-run `mindspec next --spec <slug>` (auto-recovers the worktree for an already-claimed bead).

2. **Load the prompt.** Read the staged `<repo>/review/prep/bead<N>_impl_prompt.md` (from `prompt-path` or Phase A).

3. **Dispatch.** Spawn a `general-purpose` `Agent` with the prompt as `prompt`. Run in background if you have other parallel work; otherwise foreground.
   - The subagent makes exactly ONE commit on the bead branch ending `Deviations: <list or "none">`.
   - It does NOT merge, push, or complete the bead (guardrails fences).

4. **On return, capture:**
   - Commit SHA → record in conversation.
   - Test summary (pass/fail/skip) → record.
   - Any flagged deviations → carry into the round-1 BRIEF (these become "fix-author deviation (A/B/...)" sections).

## Anti-patterns

- Don't paraphrase the spec/plan in the prompt — quote verbatim so the subagent reads canonical text, not your summary.
- Don't list helpers without import paths. "There's a helper in bead 2" is worse than no mention; the subagent will reimplement it.
- Don't omit the guardrails pointer. Subagents scope-creep without explicit fences.
- Don't ask the subagent to "iterate" or "fix issues as they come up" — that's the panel's job, not the impl agent's.
- Don't bake test results into the prompt ("the tests pass" before they're written). Specify expected test names; let the subagent run them.

## Then

Hand off to `/ms-panel-run` step 0 to set up the round-1 review panel.
