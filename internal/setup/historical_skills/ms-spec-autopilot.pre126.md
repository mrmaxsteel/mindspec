---
name: ms-spec-autopilot
description: Whole-spec orchestrator — cycle every ready bead until the spec is done, then run impl approve
---

# Spec Autopilot

The headline skill. Take a mindspec-approved plan and drive it bead-by-bead to completion. Each bead goes through `/ms-bead-cycle` (which owns pick + claim in its step 0). The autopilot sequences: it loops the cycle until no beads remain, then approves impl.

**Beads run serially by design; parallelism lives inside each cycle** (across the 6 reviewers + the impl subagent). There is no fan-out-across-beads mode — the cycle is the unit of work, one bead at a time.

Include the standard guardrails (AGENTS.md § Bead-loop guardrails) — the orchestrator rules apply across the whole loop (one push at end-of-spec, never raw-merge).

## Inputs

- `spec-slug` (optional) — defaults to the active spec from `mindspec state show`.
- `max-rounds-per-bead` (default `3`) — passed through to `/ms-bead-cycle`.

## Sequence

```
loop:
  /ms-bead-cycle            (step 0 picks + claims the next eligible bead, then runs it end-to-end)
    ↓ no bead available?
        if all spec beads closed → /ms-spec-final-review → /ms-impl-approve → exit
        else → halt (deps not satisfiable, ask user)
    ↓ merged?
        yes → loop
        halted → exit, return state to user
```

## Sequencing rules

- **Trust the plan dep graph over `bd ready`.** `bd ready` only knows the explicit `bd add-dep` edges; the plan's `Depends on:` lines are the authoritative source. If `bd ready` shows a bead the plan says is blocked, `bd update <bead> --add-dep <blocker>` to fix `bd` rather than working around it. (The cycle's step 0 performs this cross-check.)

- **Pick the lowest-depth bead first.** Among eligible beads, choose the one with the shortest dep-path back to Bead 1. Front-loads the chain, exposes downstream deps soonest.

- **One halted bead halts the spec.** If `/ms-bead-cycle` returns halted, don't skip to the next bead. The halt usually signals the plan needs revision, not just that one bead is hard.

## Stop conditions

| Condition | Action |
|:----------|:-------|
| All spec-owned beads closed | `/ms-spec-final-review` → `/ms-impl-approve` → exit |
| Any bead REJECT verdict | Halt, return to user with verdict JSONs |
| `max-rounds-per-bead` exceeded on any bead | Halt, leave bead `in_progress` |
| `bd ready` shows beads but none satisfy plan deps | Halt, report dep-graph mismatch |
| Two consecutive impl-subagent failures on same bead | Halt, ask user |
| Bead returns an ACCEPTed `NOT READY` (spec 124 R5) | Halt at the bead level; surface the ordinal-numbered report + revision routing; do NOT proceed to the next bead |

## Reporting

After each bead merges, report to the user (concise):

```
✓ <bead-id> merged in <K> rounds (round 1: <verdict tally>, round K: 6/6 APPROVE)
  Commit: <merge sha>
  Test summary: <pass/fail/skip>
  Deviations folded into BRIEF for next round: <yes/no>

Next: <bead-id> claimed, /ms-bead-cycle starting.
```

After the spec is done:

```
✓ Spec <spec-slug> complete. <N> beads merged in <T> minutes total.
  Avg rounds per bead: <R>
  Beads that needed 0 fix rounds: <X> / <N>
  Beads that hit max-rounds: <Y> / <N>
  /ms-impl-approve executed; spec branch ready for review-mode merge to main.
```

## Anti-patterns

- Don't keep going after a halt. The plan needs human attention.
- Don't `git push` mid-spec to "save progress". Push at end-of-spec only (single CI run per spec — AGENTS.md § Bead-loop guardrails).
- Don't claim two beads at once. Beads run serially by design — the cycle is the unit of work.
- Don't run `/ms-impl-approve` while beads are still open. The skill checks this, but failing the check wastes time.

## Composition

This skill is a thin loop around `/ms-bead-cycle`. The decision-making lives in the cycle and the smaller skills; autopilot just sequences them. If you find yourself adding logic here, ask whether it belongs in `/ms-bead-cycle` first.
