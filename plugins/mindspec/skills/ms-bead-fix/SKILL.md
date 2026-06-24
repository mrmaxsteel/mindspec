---
name: ms-bead-fix
description: Dispatch a fix-up subagent for a mindspec bead with the consolidated panel-review change list
---

# Fix Subagent Dispatch

Given a consolidated `concrete_changes_required` list from `/ms-panel-tally`, dispatch one subagent to fold the fixes into a single follow-up commit on the same bead branch.

Include the standard guardrails (AGENTS.md § Bead-loop guardrails) in the fix-subagent prompt — the fences (one commit, no `mindspec complete`, no `git push`, no scope creep, tests must PASS, report SHA + counts + deviations) live there, not here.

## Inputs

- `bead-id` (required) — e.g. `lola-8gbp.2`.
- `bead-worktree` (required) — absolute path.
- `panel-slug` (required).
- `round` (required) — the round whose verdicts you're acting on; the fix commit goes into round-<N+1>'s review scope.
- `consolidated-path` — usually `<spec-dir>/reviews/<panel-slug>/consolidated-round-<N>.md`.

> `<spec-dir>` is the spec's flat directory `<repo>/.mindspec/specs/<spec-slug>/`; panel artifacts are co-located under `<spec-dir>/reviews/` (spec 106 flat layout).

## Steps

1. **Compose the fix-subagent prompt.** Include:

   - Bead id, branch, worktree path, current HEAD SHA.
   - Files in scope (from the bead plan section + the round-<N> verdicts).
   - Shared modules and imports the fix should reuse (don't reimplement what earlier beads already provided).
   - The consolidated change list, grouped by criticality.
   - For each change, the rationale from the relevant verdict (1-2 sentence reviewer quotes are fine — they make the why crisp).
   - The standard guardrails pointer (AGENTS.md § Bead-loop guardrails).
   - **Explicit deviation policy**: if the subagent can't address an item literally (e.g. a test requires editing a different bead's data, or the requested refactor breaks a downstream contract), it should flag the deviation in its report and document it in the commit message under `Deviations:`. The next-round panel BRIEF surfaces these as "Fix-author deviations (assess these)".

   **Artifact-gate rule (do NOT mark a HARD-block finding ADDRESSED via a PR-body edit alone).** If a consolidated item names a measurement artifact that does not exist at its stated path, the fix subagent MUST NOT mark it ADDRESSED until the artifact actually exists — PR-body precision is necessary but not sufficient. If it can't produce the artifact in scope, it lands any body-precision improvements, returns the artifact-gate findings UNCHANGED, and flags "PARTIAL — artifact at `<path>` still missing; HARD block stands". The orchestrator then commissions the artifact run separately. See `/ms-panel-tally` § Artifact gates for the full rule and the lola-f4a8 case.

2. **Commit message template:**
   ```
   impl(<spec>, bead <N>): fold in panel round-<N> fixes — <2-3 word summary>

   - <change 1 — one line>
   - <change 2 — one line>
   ...

   Deviations: <name + reason, or "none">
   ```

3. **Dispatch.** Spawn a `general-purpose` `Agent` with the prompt. Foreground if the orchestrator is idle; background if there's other parallel work.

4. **On return, capture:**
   - New commit SHA
   - Test summary (pass/fail/skip)
   - Any flagged deviations — these go into the next BRIEF's deviations section

## Commit-gate coverage (C2-1)

mindspec blocks direct commits to **protected branches** (any mode) and to `spec/<slug>` branches **during implement mode** — `bead/<id>` branches always pass, so the fix subagent's single commit on the bead branch lands without any escape hatch. The CLI's own block message carries the full when-is-it-legitimate context (point-of-use); no skill prose duplicates the workaround. For panel-driven bead fix-ups, the commit lands on the bead branch and merges through `mindspec complete` — the gate is not in your way.

(Final-review fix-ups that must land directly on the spec branch are a different case — see `/ms-spec-final-review`, which routes them through the documented `MINDSPEC_ALLOW_MAIN=1` escape hatch.)

## Anti-patterns

- Don't dispatch separate subagents for separate items in the consolidated list. One subagent, one commit. Coordinated state is easier to review than fragmented commits.
- Don't ask the fix subagent to also run the next-round panel. Separation of concerns.
- Don't pass the raw verdict JSONs — pass the consolidated MD. The subagent doesn't need 18 lines of duplicated asks.
- Don't let the subagent rewrite the bead branch history. Keep the original implementation commit and append the fix commit; the panel can see both.

## Then

Hand off back to `/ms-bead-cycle`, which dispatches `/ms-panel-run` step 0 (round-<N+1> — re-bumps round + reviewed_head_sha).
