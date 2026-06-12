---
name: ms-bead-cycle
description: Single-bead orchestrator — pick + claim (step 0) → impl → panel → fix → re-panel → merge until the bead lands
---

# Bead Cycle Orchestrator

Drive one bead from claim to merge using the panel review loop. Compose the smaller skills; don't reimplement them. The cycle owns step 0 (pick + claim) and the merge terminal (`mindspec complete`) directly — these were previously the separate `/ms-bead-next` and `/ms-bead-merge` skills, folded in here so the orchestrator that owns sequencing also owns the bead's entry and exit.

Include the standard guardrails (AGENTS.md § Bead-loop guardrails) — the orchestrator rules (only the cycle runs `mindspec complete`, after the panel gate passes; never raw-merge a bead branch; one push at end-of-spec).

## Inputs

- `bead-id` (optional) — if absent, step 0 picks the next eligible bead for the active spec.
- `spec-slug` (optional) — restrict step 0 to a given spec; otherwise honour the active spec.
- `max-rounds` (default `3`) — stop iterating after this many panel rounds.
- `prompt-path` (optional) — pre-staged impl prompt at `<repo>/review/prep/bead<N>_impl_prompt.md`.

## Step 0 — pick + claim the next bead

1. **List ready beads scoped to the spec.**
   ```bash
   bd ready 2>&1 | grep "<spec-prefix>"
   ```
   `bd ready` already filters to beads with no blocking deps. **Cross-check against the plan's `Depends on:` lines** for each bead — `bd` and the plan can disagree (lola spec-050: `bd` showed Bead 5 ready while the plan said it depended on Beads 1-4). When they disagree, **trust the plan**, and fix `bd` (`bd update <bead> --add-dep <blocker>`) before proceeding.

2. **Disambiguate multiple active specs.** If `mindspec state show` reports more than one active spec, ask the user which to focus on — never guess.

3. **Pick deterministically.** Among the plan-eligible ready beads, pick the lowest-numbered one (`.3` before `.4` before `.5`). If two beads of the same depth are eligible, pick the one with the smaller `bd id`.

4. **Claim.**
   ```bash
   mindspec next --spec <spec-slug> --pick <K> --force
   ```
   This claims the bead, creates `bead/<id>` off the spec branch, and creates a worktree at `<spec-worktree>/.worktrees/worktree-<bead-id>/`. The claim-failure and worktree-setup-failure recovery recipes are emitted by the CLI itself (point-of-use errors) — follow them if `mindspec next` reports a failure.

   Belt-and-braces fallback for CLI-down failures the CLI error paths never reach (e.g. the whole `mindspec` binary is unavailable):
   ```bash
   bd update <bead-id> --claim --status in_progress
   git -C <spec-worktree> worktree add .worktrees/worktree-<bead-id> -b bead/<bead-id> <spec-branch>
   ```

5. **Verify state.** `bd show <bead-id>` shows `in_progress`; the worktree dir exists; `git -C <worktree> branch --show-current` reports `bead/<bead-id>`.

Don't claim multiple beads at once — the cycle is serial per bead by design (parallelism happens *inside* each cycle, across reviewers + impl agent).

## Sequence

```
step 0                 (pick + claim the next eligible bead, create worktree)
  ↓ bead-id + worktree + branch
/ms-bead-impl          (round 1 implementation — Phase A stages the prompt, Phase B dispatches)
  ↓ commit SHA + flagged deviations
/ms-panel-run          (round 1 — step 0 writes panel.json + BRIEF, then 6 reviewers fan out)
  ↓ wait for completions
/ms-panel-tally        (round 1 verdicts — single decision authority)
  ↓ decide:
       ≥ N−1 APPROVE   → merge terminal → exit
       below threshold → /ms-bead-fix → continue
       any REJECT / HARD block → halt (see /ms-panel-tally § halt-recover)

/ms-bead-fix           (round 1 → round 2 fix commit)
  ↓ new commit SHA + flagged deviations
/ms-panel-run          (round 2 — step 0 re-bumps round + reviewed_head_sha in one write)
  ↓
/ms-panel-tally        (round 2 verdicts)
  ↓ decide as above

... iterate until APPROVE or max-rounds reached ...

merge terminal         (panel-approved bead — mindspec complete)
```

## Merge terminal — `mindspec complete`

The panel gate passed (`/ms-panel-tally` decided merge; the pre-complete hook enforces it). Close the bead in `bd`, merge `bead/<id>` into the spec branch, and remove the bead worktree — this is `mindspec complete` plus a one-line verify.

1. **Run the merge.**
   ```bash
   mindspec complete <bead-id> "<summary>"
   ```
   `<summary>` is a 1-2 sentence positional argument describing what landed, used for the merge commit message. `complete` runs from the repo root and is location-agnostic — do NOT `cd` into the bead worktree first. It auto-commits any stray staged files, closes the bead in `bd`, merges `bead/<id>` into the spec branch with a `Merge bead/<id>` commit, and removes the bead worktree.

   ADR-divergence and claim/worktree failures surface their own repair ladders in the CLI error (point-of-use) — follow those; no skill prose duplicates them.

2. **Verify the merge.**
   ```bash
   git -C <spec-worktree> log --oneline -3   # should show `Merge bead/<bead-id>` at top
   ```
   (`FormatResult` prints no merge SHA, so the `git log` is the merge-commit verify.)

3. **Do NOT push, do NOT raw-merge.** The user-controlled push gate is at end-of-spec (after `/ms-impl-approve`), not per-bead. **Never merge a bead branch with raw `git merge bead/<id>`** — it bypasses `bd` closure, worktree cleanup, AND the panel gate (no git hook fires on merge commits; raw merge is the obvious gate workaround). Only `mindspec complete` merges bead branches (AGENTS.md § Bead-loop guardrails).

> The Step-1 pre-merge checklist that `/ms-bead-merge` used to carry (bd-show / clean-tree / commits-visible rows, "if anything is uncommitted, abort") is **superseded by CLI checks**: 092 Bead 5's honest user-dirt blocking and `FormatResult`'s closure/worktree-removal output cover the bd-show / clean-tree / commits-visible rows, and the "if anything is uncommitted, abort" line is now MECHANIZED by the pre-complete hook's dirty-tree Block (Spec 093 Req 11). Don't re-add the checklist as prose.

**Partial-failure rule (verbatim):** "Don't proceed to the next bead if the merge failed mid-way (e.g. ADR check stopped between bd-close and the actual git merge). Resolve the failure first." Re-running `mindspec complete <id>` after a partial failure is the documented recovery path; the gate passes a deleted-branch / missing-worktree rerun through to `complete`'s own idempotent handling.

## Implementation notes

- **Panel slugs**: `<spec-prefix>-bead<N>` for round 1, `<spec-prefix>-bead<N>-r<R>` for round R≥2. Keeps each round's verdicts in its own dir, easier to diff.
- **Flagged deviations carry across rounds.** When the impl or fix subagent reports deviations from its prompt, those become "Fix-author deviations (assess these)" sections in the next BRIEF — the panel must explicitly assess whether the deviation is acceptable. Don't bury them.
- **Round-2 codex reviewers are different from claude.** Claude `Agent`s can re-read the round-1 verdict JSON inline. Codex CLI sessions need that path inserted into their prompt explicitly — they don't share conversation context across rounds.
- **Test-suite truth wins over reviewer opinion.** If the panel splits and one reviewer's REQUEST_CHANGES rests on a claim that tests pass disproves, the orchestrator should note the contradiction. Don't blindly trust either side; the synthesis is your job.

## Family-asymmetry handling

The headline signal is when all three Claudes APPROVE while one or more Codex REQUEST_CHANGES (or vice versa). Pay particular attention:

| Pattern | Likely meaning | Action |
|:--------|:---------------|:-------|
| 3 Claude APPROVE, 1+ Codex REQUEST_CHANGES with empirical evidence | The Codex actually ran something and found a bug the Claudes missed | Treat as REQUEST_CHANGES — trust the empirical check |
| 3 Codex APPROVE, 1+ Claude REQUEST_CHANGES on contract design | Claude is reading the diff at a level Codex skipped | Investigate the design claim; usually worth a fix |
| Unanimous APPROVE | Genuine consensus | Proceed to merge |
| Unanimous REQUEST_CHANGES | The implementation is genuinely off; fix or rewrite |

## Anti-patterns

- Don't skip the panel even when "the diff looks fine to me". The point of the panel is that *your* judgement is one of seven — don't pre-empt it.
- Don't run multiple fix-cycles inside a round. One panel → one fix → one re-panel. Compounding fixes within a round makes the next panel's job ambiguous.
- Don't exceed `max-rounds` silently. If you reach the cap, halt and report; the user may want to revise the plan or split the bead.
- Don't merge with a known REQUEST_CHANGES (even at the threshold). The dissenting verdict's `concrete_changes_required` either gets folded in or filed as a follow-up bead before merge.
- Don't run `mindspec complete` until `/ms-panel-tally` decided merge — the merge is destructive of the bead branch.

## Then

- Successful merge → caller (`/ms-spec-autopilot`) picks the next bead.
- Halt → return to user with the panel state and the bead worktree intact.
