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
- **Main-integration risk** — main has moved since the spec branch was cut; cumulative drift on shared files (e.g. `core/config.py`, `requirements.txt`) may not be visible per-bead.
- **AC release-gate readiness** — the spec PR is the artifact that must surface gate evidence (measurement artifacts, operator sign-offs, follow-up bead IDs). Per-bead panels don't check the PR body.
- **Operator readiness** — runbooks for revert / rollout / on-call escalation should be coherent across the whole spec, not just within one bead.

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

3. **Create the final-review panel dir.**
   ```bash
   mkdir -p <repo>/review/<spec-slug>-final
   ```
   Write `BRIEF.md` with sections: PR link, spec.md scope summary, cumulative diff stat, list of merged beads (with round-N commit SHAs), known fix-author deviations from each round-2 panel that the final reviewer should re-assess in cumulative context.

4. **Fan out 6 reviewers with the FINAL-REVIEW lenses (different from bead lenses):**

   | Slot | Lens | Focus |
   |:-----|:-----|:------|
   | F1 | Cumulative scope | Does the merged diff stay within spec.md scope? Any beads land work spec.md didn't authorize? |
   | F2 | Inter-bead coherence | Did any round-2 fix break a contract a later bead approved against? Run the FULL regression suite on the spec branch HEAD. |
   | F3 | Main integration | Does the branch merge cleanly into current `main` HEAD? Any conflicts? Any post-cut main commits that should have been rebased through the spec branch? |
   | F4 | PR description accuracy | Does the PR body match the actual diff? Are claimed bead numbers right, file counts correct, follow-ups all filed in `bd`? |
   | F5 | AC release-gate readiness | For each AC the spec.md claims, identify the expected gate-evidence artifact path. For each path, check `[ -f <path> ]` on the spec branch. If the artifact does NOT exist at the path: emit a `concrete_changes_required` item of the form `"materialize <artifact-name> at <path>"` and mark as HARD-BLOCK (not a body-precision fix — see § "Artifact gates" below). PR-body naming the path is necessary but not sufficient. |
   | F6 | Operator readiness | Runbooks updated, revert MTTR claim verifiable, on-call escalation paths defined, follow-up beads filed with concrete IDs. |

   Each reviewer writes a verdict JSON to `<repo>/review/<spec-slug>-final/<slot>-round-1.json`.

## Artifact gates (HARD block, not body fix)

Some F5 findings name a measurement artifact (e.g. `cost_projection.json`) that the spec.md plan declared as a release-gate precondition. These are **HARD blocks** distinct from PR-body precision fixes:

| Finding shape | Treatment |
|:--------------|:----------|
| "Evidence path UNNAMED in PR body" | Soft fix — name the path in the PR body, F5 re-verifies. |
| "Evidence path NAMED but artifact MISSING at that path" | **HARD block** — orchestrator must commission the measurement run + land the artifact at the named path. Cannot be resolved by editing the PR body. |
| "Operator sign-off PENDING" | Soft fix — capture sign-off reference in PR body or as a PR comment. |

The distinguishing question: **could the missing artifact have caught a real defect?** If yes (measurement artifact, cost projection, drift report, regression baseline), it's a HARD block. If no (operator acknowledgement, follow-up bd link), it's a soft fix.

Real failure case: spec-050 F5 round 1 flagged AC8c `cost_projection.json` as missing. Round 2 fix was a PR-body precision update naming the artifact landing path. F5 round 2 flipped to APPROVE because the path was named. PR #522 merged. Today's Monday cron — the first post-spec-050 — burned $417 in one run because the alias-intersect prefilter has no cap, exactly what AC8c was meant to project. Postmortem: `bd show lola-f4a8`.

The HARD-block treatment forces: either run the measurement and produce the artifact, OR halt and ask the user to explicitly waive the gate (which is a recorded decision, not a silent default).

5. **Tally.** Same shape as `/ms-panel-tally`, plus an artifact-gate check:
   - **Any reviewer flagged a HARD-block artifact-gate** (see § "Artifact gates") → halt regardless of vote count. Orchestrator must materialize the artifact OR escalate to user for explicit waiver. PR-body fixes cannot satisfy this.
   - ≥5/6 APPROVE AND no HARD-block flags → proceed to `/ms-impl-approve`.
   - 3-4 APPROVE with concrete fixes → dispatch `/ms-bead-fix` style fix-up on the spec branch (NOT a new bead — direct commit to spec branch, then `git push --force-with-lease` to update the PR).
   - ≤2 APPROVE → halt, ask user. This is rare and usually means the spec was over-scoped or beads diverged from spec.md.
   - Any REJECT → halt, ask user.

6. **On APPROVE**, run `/ms-impl-approve` to close the spec lifecycle.

## What this skill is NOT

- Not a replacement for per-bead panels. The per-bead loop catches unit defects efficiently; the final panel catches the cumulative-only defects.
- Not a CI substitute. CI checks (typecheck, lint, test) should run on the spec PR independently; this panel adds human-judgment review on top.
- Not an authoritative deploy gate. Ops-side gates (`gate_status.json`, AC release-gate evidence) are separate from this panel's APPROVE.

## Anti-patterns

- Don't run the final panel before the last bead merges. Reviewers reading half-merged spec branches see noise, not signal.
- Don't reuse the per-bead BRIEFs. Each bead BRIEF was scoped to one increment; the final BRIEF must summarise the cumulative state.
- Don't ask the final reviewers to re-do the per-bead empirical probes. They have different lenses — don't waste their context running tests the bead panels already ran. Do ask F2 to run the cumulative regression once.

## Working around the implement-mode commit gate

Final-review fix-ups land on the spec branch directly (not on a fresh bead branch), which trips mindspec's implement-mode commit gate. The escape hatch:

```bash
MINDSPEC_ALLOW_MAIN=1 git commit -m "..."
```

Use this for the panel-driven chore commits that fold consolidated F-reviewer asks into the spec branch — e.g. PR-body precision corrections, stray-file reverts, CI-unblocking test fixes. The gate exists to prevent accidental scope creep on the wrong branch; the env var is the documented opt-out for the final-review fix loop where spec-branch commits are exactly the right path.

Surfaced by lola spec-050 final-review fix commits `1bb9751` (revert stray files + PR body precision) and `04d26f5` (lola-90pp test fix to unblock CI).

## Then

- APPROVE → `/ms-impl-approve <spec-slug>`
- REQUEST_CHANGES → consolidate concrete changes; dispatch fix subagent against the spec branch; push --force-with-lease; re-run the final panel.
- REJECT → halt, ask user.
