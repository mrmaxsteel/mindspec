---
name: ms-spec-autopilot
description: Whole-spec orchestrator — cycle every ready bead until the spec is done, then approve impl
---

# Spec Autopilot

The headline skill. Take a mindspec-approved plan and drive it bead-by-bead to completion. Each bead goes through `/ms-bead-cycle`. The autopilot is responsible for sequencing — choosing the next bead, handling parallel windows, and knowing when to stop.

## Inputs

- `spec-slug` (optional) — defaults to the active spec from `mindspec state show`.
- `max-rounds-per-bead` (default `3`) — passed through to `/ms-bead-cycle`.
- `parallel-window` (default `false`) — set `true` to fan out across independent beads when the plan dep graph allows.

## Sequence

```
loop:
  /ms-bead-next             (find + claim next eligible bead)
    ↓ no bead available?
        if all spec beads closed → /ms-impl-approve → exit
        else → halt (deps not satisfiable, ask user)

  /ms-bead-cycle <bead-id>  (run the bead end-to-end)
    ↓ merged?
        yes → loop
        halted → exit, return state to user
```

## Parallel-window handling (opt-in)

When `parallel-window: true`, after each `/ms-bead-merge`:

1. Read the plan dep graph. Find all beads whose blockers are now satisfied.
2. If 2+ beads are eligible, fan out:
   - Dispatch `/ms-bead-cycle` for each in a background `Agent`.
   - Wait for all to complete before picking the next wave.
3. Coordination caveats:
   - Two beads in the same module can race on the spec branch's HEAD. Pick beads in disjoint file-sets only.
   - Bead 4 / Bead 5 in many plans share `models.py` or `resolution.py` — fan out only if their plan §"Files touched" doesn't overlap.

Default `parallel-window: false`. The serial cycle is the safe default; parallelism is opt-in once you've inspected the plan.

## Sequencing rules

- **Trust the plan dep graph over `bd ready`.** `bd ready` only knows the explicit `bd add-dep` edges; the plan's `**Depends on**:` lines are the authoritative source. If `bd ready` shows a bead the plan says is blocked, `bd update <bead> --add-dep <blocker>` to fix `bd` rather than working around it.

- **Pick the lowest-depth bead first.** Among eligible beads, choose the one with the shortest dep-path back to Bead 1. Front-loads the chain, exposes downstream deps soonest.

- **One halted bead halts the spec.** If `/ms-bead-cycle` returns halted, don't skip to the next bead. The halt usually signals the plan needs revision, not just that one bead is hard.

## Stop conditions

| Condition | Action |
|:----------|:-------|
| All spec-owned beads closed | `/ms-impl-approve` → exit |
| Any bead REJECT verdict | Halt, return to user with verdict JSONs |
| `max-rounds-per-bead` exceeded on any bead | Halt, leave bead `in_progress` |
| `bd ready` shows beads but none satisfy plan deps | Halt, report dep-graph mismatch |
| Two consecutive impl-subagent failures on same bead | Halt, ask user |

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
- Don't `git push` mid-spec to "save progress". Push at end-of-spec only (single CI run per spec — see project AGENTS.md rule).
- Don't claim two beads at once unless `parallel-window: true` AND the plan dep graph + file-set explicitly allow it.
- Don't run `/ms-impl-approve` while beads are still open. The skill checks this, but failing the check wastes time.

## Composition

This skill is a thin loop around `/ms-bead-next` → `/ms-bead-cycle`. The decision-making lives in the smaller skills; autopilot just sequences them. If you find yourself adding logic here, ask whether it belongs in `/ms-bead-cycle` or `/ms-bead-next` first.
