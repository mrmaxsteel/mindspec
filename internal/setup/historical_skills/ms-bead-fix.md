---
name: ms-bead-fix
description: Dispatch a fix-up subagent for a mindspec bead with the consolidated panel-review change list
---

# Fix Subagent Dispatch

Given a consolidated `concrete_changes_required` list from `/ms-panel-tally`, dispatch one subagent to fold the fixes into a single follow-up commit on the same bead branch.

## Inputs

- `bead-id` (required) — e.g. `lola-8gbp.2`.
- `bead-worktree` (required) — absolute path.
- `panel-slug` (required).
- `round` (required) — the round whose verdicts you're acting on; the fix commit goes into round-<N+1>'s review scope.
- `consolidated-path` — usually `<repo>/review/<panel-slug>/consolidated-round-<N>.md`.

## Steps

1. **Compose the fix-subagent prompt.** Include:

   - Bead id, branch, worktree path, current HEAD SHA.
   - Files in scope (from the bead plan section + the round-<N> verdicts).
   - Shared modules and imports the fix should reuse (don't reimplement what earlier beads already provided).
   - The consolidated change list, grouped by criticality.
   - For each change, the rationale from the relevant verdict (1-2 sentence reviewer quotes are fine — they make the why crisp).
   - **Explicit deviation policy**: if the subagent can't address an item literally (e.g. a test requires editing a different bead's data, or the requested refactor breaks a downstream contract), it should:
     - Flag the deviation explicitly in its report
     - Document the deviation in the commit message under `Deviations:`
     - The next-round panel BRIEF will surface these as "Fix-author deviations (assess these)".

   **Anti-pattern: do NOT mark an artifact-gate finding as ADDRESSED via a PR-body edit alone.**

   If a consolidated `concrete_changes_required` item names a measurement artifact, cost projection, drift report, or regression baseline that does not exist at the stated path on the spec branch, the fix subagent MUST refuse to mark it ADDRESSED until the artifact actually exists. PR-body precision (naming the path, naming the ops-handoff mechanism) is necessary but not sufficient.

   If the fix subagent cannot produce the artifact within its scope, it should:
   1. Land any body-precision improvements it can.
   2. Return the artifact-gate findings UNCHANGED in its report.
   3. Flag explicitly: "PARTIAL — body precision improved; artifact at `<path>` still missing; HARD block stands".

   The orchestrator then commissions the artifact run as a separate work unit (not part of the fix-up subagent's job).

   Real failure case: spec-050 R1+R3+R5 final-review flagged AC8c `cost_projection.json` missing. Fix subagent's round-2 edit named the artifact's landing path in the PR body. F5 round-2 flipped to APPROVE on body precision. PR merged. lola-f4a8 was the bill.

2. **Constraints to enforce on the subagent:**
   - **One commit.** Not three small commits; a single fold-in commit on the bead branch.
   - **No scope creep.** Address the consolidated list; don't fix unrelated nits.
   - **No `mindspec complete`.** The cycle owns the merge.
   - **No `git push`.** Leave the commit local.
   - **Tests must still pass.** Run the bead's test scope before reporting back.

3. **Commit message template:**
   ```
   impl(<spec>, bead <N>): fold in panel round-<N> fixes — <2-3 word summary>

   - <change 1 — one line>
   - <change 2 — one line>
   ...

   Deviations: <name + reason, or "none">
   ```

4. **Dispatch.** Spawn a `general-purpose` `Agent` with the prompt. Foreground if the orchestrator is idle; background if there's other parallel work.

5. **On return, capture:**
   - New commit SHA
   - Test summary (pass/fail/skip)
   - Any flagged deviations — these go into the next BRIEF's deviations section

## Working around the implement-mode commit gate

mindspec blocks direct commits to `spec/<slug>` and `bead/<id>` branches when the spec is in implement mode — this is a guardrail against accidental scope creep on the wrong branch. For panel-driven chore fixes (fix-up commits that fold consolidated reviewer asks into the bead branch), `mindspec complete` is the right path; the gate only blocks commits *outside* the bead-cycle loop.

For final-review fix-ups that need to land on the spec branch directly (not on a fresh bead branch — e.g. PR-body precision corrections, stray-file reverts), use the escape hatch:

```bash
MINDSPEC_ALLOW_MAIN=1 git commit -m "..."
```

Surfaced by lola spec-050 final-review fix commits `1bb9751` (revert stray files + PR body precision) and `04d26f5` (lola-90pp test fix to unblock CI).

## Anti-patterns

- Don't dispatch separate subagents for separate items in the consolidated list. One subagent, one commit. Coordinated state is easier to review than fragmented commits.
- Don't ask the fix subagent to also run the next-round panel. Separation of concerns.
- Don't pass the raw verdict JSONs — pass the consolidated MD. The subagent doesn't need 18 lines of duplicated asks.
- Don't let the subagent rewrite the bead branch history. Keep the original implementation commit and append the fix commit; the panel can see both.

## Then

Hand off back to `/ms-bead-cycle`, which dispatches `/ms-panel-create` (round-<N+1>) → `/ms-panel-run`.
