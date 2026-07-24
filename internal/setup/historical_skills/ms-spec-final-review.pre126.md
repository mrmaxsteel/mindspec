---
name: ms-spec-final-review
description: Final panel review of the WHOLE spec branch vs main — runs once after the last bead merges, before /ms-impl-approve
---

# Spec Final Review

A focused panel that reviews the cumulative spec-branch diff against `main`, not the per-bead increments. Catches what bead-by-bead review can't: scope drift, inter-bead coherence regressions, PR-description accuracy, main-integration risk, operator-runbook readiness, AC release-gate evidence.

This skill runs ONCE per spec, between "last bead merged" and `/ms-impl-approve`.

## Why it's distinct from `/ms-bead-cycle` panels

Per-bead panels see one commit at a time. They reliably catch unit defects but cannot catch:

- **Cumulative scope drift** — every bead approved individually but the whole spec implements more than spec.md asked for.
- **Inter-bead coherence regressions** — Bead 4 approved against Bead 3 at the time, but Bead 3 was later patched in round-2 to a contract Bead 4 silently no longer satisfies.
- **PR-description accuracy** — bead-by-bead never asks "does the PR body match the actual diff?"
- **Main-integration risk** — main has moved since the spec branch was cut; cumulative drift on shared files may not be visible per-bead.
- **AC release-gate readiness** — the spec PR is the artifact that must surface gate evidence (measurement artifacts, operator sign-offs, follow-up bead IDs).
- **Operator readiness** — runbooks for revert / rollout / on-call escalation should be coherent across the whole spec.

## Inputs

- `pr-number` (required) — the spec PR to review.
- `spec-slug` (required) — to read `spec.md` for the scope-drift check.
- `merge-base-against` (default `origin/main`) — the cumulative-diff anchor.

## Steps

1. **Refresh main.** `git fetch origin` so the cumulative-diff is against the actual current `main`, not a stale checkout.

2. **Compute the cumulative diff.**
   ```bash
   git diff origin/main...spec/<spec-slug> --stat
   git diff origin/main...spec/<spec-slug> --name-only | head -50
   ```
   Record the totals — files touched, +X/-Y lines, file count by directory. The bead-level panels saw this incrementally; the final reviewers see it as one thing.

3. **Create the panel via `/ms-panel-run` step 0** with `target=spec/<spec-slug>`. Step 0 creates `<spec-dir>/reviews/<spec-slug>-final/` (where `<spec-dir>` is the spec's flat directory `<repo>/.mindspec/specs/<spec-slug>/` — reviews are co-located under the spec per the spec 106 flat layout), writes `panel.json` (`bead_id` null, `expected_reviewers` 6, `reviewed_head_sha` = the spec-branch tip), and writes `BRIEF.md`. Do NOT hand-roll `mkdir` + BRIEF — routing through step 0 is what makes the final-review panel emit `panel.json` so it appears in `mindspec instruct --panel-state` and the gate plumbing.

   The BRIEF (composed by step 0, with the final-review specifics) carries: PR link, spec.md scope summary, cumulative diff stat, list of merged beads (with round-N commit SHAs), known fix-author deviations from each round-2 panel that the final reviewer should re-assess in cumulative context.

4. **Fan out 6 reviewers with the FINAL-REVIEW lenses (different from bead lenses):**

   | Slot | Lens | Focus |
   |:-----|:-----|:------|
   | F1 | Cumulative scope | Does the merged diff stay within spec.md scope? Any beads land work spec.md didn't authorize? |
   | F2 | Inter-bead coherence | Did any round-2 fix break a contract a later bead approved against? Run the FULL regression suite on the spec branch HEAD. |
   | F3 | Main integration | Does the branch merge cleanly into current `main` HEAD? Any conflicts? Any post-cut main commits that should have been rebased through the spec branch? |
   | F4 | PR description accuracy | Does the PR body match the actual diff? Are claimed bead numbers right, file counts correct, follow-ups all filed in `bd`? |
   | F5 | AC release-gate readiness | For each AC the spec.md claims, identify the expected gate-evidence artifact path and check `[ -f <path> ]` on the spec branch. If the artifact does NOT exist at the path: emit a `concrete_changes_required` item `"materialize <artifact-name> at <path>"` with `"hard_block": true`. PR-body naming the path is necessary but not sufficient (see `/ms-panel-tally` § Artifact gates). |
   | F6 | Operator readiness | Runbooks updated, revert MTTR claim verifiable, on-call escalation paths defined, follow-up beads filed with concrete IDs. |

   Each reviewer writes a verdict JSON to `<spec-dir>/reviews/<spec-slug>-final/<slot>-round-1.json`.

5. **Tally through `/ms-panel-tally`** — the single decision authority. The artifact-gate HARD-block rule, the N−1 threshold, and the halt-recovery procedure all live there; the F5 "evidence path NAMED but artifact MISSING → HARD block" finding is a `"hard_block": true` verdict the tally halts on regardless of vote count. On APPROVE → proceed to `/ms-impl-approve`.

## What this skill is NOT

- Not a replacement for per-bead panels. The per-bead loop catches unit defects efficiently; the final panel catches the cumulative-only defects.
- Not a CI substitute. CI checks (typecheck, lint, test) should run on the spec PR independently; this panel adds human-judgment review on top.
- Not an authoritative deploy gate. Ops-side gates are separate from this panel's APPROVE.

## Final-review fix-ups land on the spec branch (escape hatch)

Final-review fix-ups land on the spec branch directly (not on a fresh bead branch), which trips mindspec's implement-mode commit gate. This is the **legitimate** direct-spec-branch case (PR-body precision corrections, stray-file reverts, CI-unblocking test fixes), so use the documented escape hatch:

```bash
MINDSPEC_ALLOW_MAIN=1 git commit -m "..."
```

Do NOT use the escape hatch to land feature code outside a bead branch. (The CLI's own block message carries this same legitimacy context at point-of-use.) Surfaced by lola spec-050 final-review fix commits `1bb9751` (revert stray files + PR body precision) and `04d26f5` (lola-90pp test fix to unblock CI).

## Anti-patterns

- Don't run the final panel before the last bead merges. Reviewers reading half-merged spec branches see noise, not signal.
- Don't reuse the per-bead BRIEFs. Each bead BRIEF was scoped to one increment; the final BRIEF must summarise the cumulative state.
- Don't ask the final reviewers to re-do the per-bead empirical probes. They have different lenses — do ask F2 to run the cumulative regression once.
- Don't hand-roll the panel dir. Route through `/ms-panel-run` step 0 so `panel.json` lands.

## Then

- APPROVE → `/ms-impl-approve <spec-slug>`
- REQUEST_CHANGES → consolidate via `/ms-panel-tally`; dispatch a fix subagent against the spec branch (escape hatch above); push --force-with-lease; re-panel via `/ms-panel-run` step 0.
- REJECT / HARD block → halt (see `/ms-panel-tally` § After a halt — recovery).
