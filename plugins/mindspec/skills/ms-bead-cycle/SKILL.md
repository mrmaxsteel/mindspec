---
name: ms-bead-cycle
description: Single-bead orchestrator — runs impl → panel → fix → re-panel → merge until the bead lands
---

# Bead Cycle Orchestrator

Drive one bead from claim to merge using the panel review loop. Compose the smaller skills; don't reimplement them.

## Inputs

- `bead-id` (required).
- `max-rounds` (default `3`) — stop iterating after this many panel rounds.
- `prompt-path` (optional) — pre-staged impl prompt at `<repo>/review/prep/bead<N>_impl_prompt.md`.

## Sequence

```
/ms-bead-impl          (round 1 implementation)
  ↓ commit SHA + flagged deviations
/ms-panel-create       (round 1 BRIEF.md)
  ↓
/ms-panel-run          (round 1 — 6 reviewers fan out)
  ↓ wait for completions
/ms-panel-tally        (round 1 verdicts)
  ↓ decide:
       ≥5/6 APPROVE  → /ms-bead-merge → exit
       3-4 APPROVE   → /ms-bead-fix → continue
       ≤2 APPROVE    → flag user, /ms-bead-fix → continue
       any REJECT    → halt, ask user

/ms-bead-fix           (round 1 → round 2 fix commit)
  ↓ new commit SHA + flagged deviations
/ms-panel-create       (round 2 BRIEF.md, surfaces fix-author deviations)
  ↓
/ms-panel-run          (round 2 — same 6 reviewers verify their own asks)
  ↓
/ms-panel-tally        (round 2 verdicts)
  ↓ decide as above

... iterate until APPROVE or max-rounds reached ...

/ms-bead-merge         (panel-approved bead, mindspec complete)
```

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
- Don't merge with a known REQUEST_CHANGES (even at 5/6 APPROVE). The dissenting verdict's `concrete_changes_required` either gets folded in or filed as a follow-up bead before merge.

## Then

- Successful merge → caller (`/ms-spec-autopilot`) picks the next bead.
- Halt → return to user with the panel state and the bead worktree intact.
