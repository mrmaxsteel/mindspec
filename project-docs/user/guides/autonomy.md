# The Autonomy Ladder

MindSpec treats autonomy as a ladder you climb deliberately, not a switch you flip. Each level hands one more class of decision to the machinery — and each handoff is only safe because the level below it already works. This guide walks the ladder rung by rung: what each level does, what it requires, and what can go wrong.

The design follows one invariant everywhere: **configuration selects who holds a gate, never what the gate checks.** The decision matrix — what evidence is required, what counts as a pass — lives in the `mindspec` binary and is identical at every level. Climbing the ladder never loosens a gate; it only changes who is standing at it.

```
Level 0   Interactive      you approve every gate
Level 1   Assisted loop    you approve spec + plan; panels verify every bead
Level 2   Governed loop    panels hold every gate; named halts; handoff log
Level 3   Scheduled loop   cron-triggered; budget ceilings per wake
Level 4   Fleet            parallel across specs (serial within each — by design)
```

## Level 0 — Interactive

The default. You approve every phase transition yourself:

```bash
mindspec spec create <slug>     # draft the spec (the grill interrogates it)
mindspec spec approve <id>      # you approve → Plan Mode
mindspec plan approve <id>      # you approve → beads created
mindspec next                   # claim a bead, fresh worktree
mindspec complete <bead-id>     # gates run, bead merges
mindspec impl approve <id>      # you approve → spec merges to main
```

Even at level 0 the guardrails are fully active: the freshness gate, doc-sync, ADR divergence, and the panel gate on `complete`. Level 0 is not "MindSpec off" — it's the loop with you as the trigger and every authority.

**Stay here until:** the lifecycle feels routine on your project and panels are consistently passing beads without surprises.

## Level 1 — Assisted loop

```
/ms-spec-autopilot
```

You approve the spec and the plan; the autopilot then cycles every ready bead — claim, implement with a fresh agent, panel review, fix rounds if needed, merge — followed by a final review of the whole spec branch. You're still in the room; you're just not typing between beads.

What level 1 changes:

- **A fresh subagent per bead.** The orchestrator spawns a new implementation agent for every bead rather than aging one session across the spec. This isn't hygiene theater: implementation quality measurably degrades as context accumulates, and MindSpec's session-freshness gate exists to block exactly that failure.
- **Panels become the per-bead verifier.** Every bead merge is decided by a six-reviewer panel (see the [review panels guide](review-panels.md)); you review the panel's decisions rather than every diff.
- **The upstream gates stay yours.** `spec approve`, `plan approve`, and `impl approve` still wait for you.

**Rules the loop enforces on itself:** a bead gets at most a bounded number of fix rounds before halting; a REJECT verdict halts rather than triggering an auto-fix; the implementing agent never merges its own work.

**Stay here until:** autopilot runs complete without you intervening mid-spec, and reading panel verdicts has convinced you the panels catch what you would have caught.

## Level 2 — Governed loop

Level 2 is where gate authority itself is delegated — a scoped grant, written as configuration:

```yaml
# .mindspec/config.yaml
panel:
  reviewers: [{family: claude, count: 3}, {family: codex, count: 3}]
  approve_threshold: "n-1"
loop:
  enabled: true
  gate_authority:              # who may pass each gate unattended: panel | human
    spec_approve:  panel
    plan_approve:  panel
    bead_merge:    panel
    impl_approve:  panel
    # there is no panel_skip key — skipping the verifier is never delegable,
    # and the loader refuses any config that tries to add one
  halt:
    max_rounds_per_bead: 3
    max_consecutive_impl_failures: 2
    panel_deadlock_rounds: 2
    on_reject: halt            # the only accepted value — a REJECT always halts
  budget: {max_beads_per_wake: 4}
  handoff_log: .mindspec/loop/AUTOPILOT-LOG.md
```

With `gate_authority: panel` on a gate, `approve` runs a panel against that target (a spec, a plan, or the final branch diff) and accepts the panel's decision under the same quorum, staleness, and threshold semantics the bead-merge gate has always used. The config is refused at load time if it tries to weaken the floor: `panel_skip` under `gate_authority`, a non-`halt` value for `on_reject`, or an always-pass threshold are all rejected before the loop starts.

### Halt conditions

A governed loop's most important behavior is stopping. Every halt is named, logged, and leaves state that a human (or a fresh controller) can pick up — nothing about a halt is exceptional or unrecoverable, because lifecycle state is derived from beads and git, never from the halted session's memory.

| Halt | Trigger | Typical recovery |
|:-----|:--------|:-----------------|
| `max_rounds_per_bead` | a bead fails its panel N times | read the round-N consolidated changes; fix by hand or re-scope the bead |
| `max_consecutive_impl_failures` | implementation agents keep failing before even reaching a panel | usually an under-specified bead — send the spec back through the grill |
| `panel_deadlock_rounds` | the panel can't reach quorum across rounds | a genuine judgment call has surfaced; it's yours |
| `on_reject` | any reviewer issues REJECT, or a hard-block artifact gate fires | read the rationale; a REJECT is never auto-fixed |
| CI failure *(level 3+)* | PR checks fail in a way the executor can't trivially retry | investigate; the merge was never attempted |
| monitor hit *(level 3+)* | a live detector fires (skipped gate, shortcut close, commit to main) | treat as a bug in the loop, not the work — file it |

### What deliberately stays human

Three things are not delegable at any level, and this is the mechanism that keeps unattended mistakes from compounding rather than a leftover of caution:

1. **Skipping a panel.** `MINDSPEC_SKIP_PANEL` is env-only, human-only, and audited on the bead record. A loop must never self-authorize skipping its own verifier.
2. **Overriding a REJECT.** `on_reject: halt` is the only accepted value. Auto-fixing a rejection is the definition of verification debt.
3. **Accepting missing evidence.** A finding that names a missing measurement artifact blocks regardless of votes. Agents can't vote evidence into existence.

### The handoff log

Every gate decision a panel makes in your place is appended to the handoff log (`.mindspec/loop/AUTOPILOT-LOG.md` in the profile above): the target, the round, the vote, the dissents, and any overrides or halts. Reviewing the log is the level-2 contract — your oversight moves from per-decision to per-batch, but it does not disappear. If reading the log ever surprises you, that's the signal to step back down a level and find out why.

**Stay here until:** governed runs complete or halt cleanly, and the handoff log reads the way you'd have decided yourself.

## Level 3 — Scheduled loop

Level 3 removes you as the trigger. A scheduler (cron, a CI schedule, a heartbeat) wakes a fresh controller, which rehydrates the loop's position from beads + git + panel state, works within its budget, and exits.

What level 3 adds on top of the governance profile:

- **Budget ceilings per wake** — `budget.max_beads_per_wake` (and a token budget, if configured) bound how much any single unattended wake can do. A wake that hits its ceiling exits cleanly mid-spec; the next wake resumes from derived state.
- **A pull-based supervisor surface** — `mindspec loop status` reports open panels and their rounds, the exact PASS/BLOCK each gate would return, beads remaining, budget consumed, escape hatches used this run, and halt state. Point your monitoring at it; MindSpec deliberately emits no telemetry stream of its own.
- **CI as a gate, not a hope** — the executor watches PR checks and re-verifies mergeability before any merge; a CI failure maps to a named halt rather than a retry loop.
- **Live monitors** — the failure detectors developed in MindSpec's behavioral test harness (skipped lifecycle steps, shortcut bead closes, forced bypasses, commits to main) run against the live event stream; any hit halts the loop.

**Controller hygiene:** the fresh-context rule applies to the orchestrator too, not just the workers. Controllers are re-instantiated per spec (or per phase) rather than grown — safe because a controller holds no unique state; everything rehydrates from the repo. A long-running controller accumulating hundreds of thousands of tokens of panel plumbing is a bug, not a badge.

**Stay here until:** you've reviewed several morning-after handoff logs and stopped finding anything you'd have decided differently.

## Level 4 — Fleet

A queue of specs, worked in parallel — one governed loop per spec, each with its own worktree lineage, panels, and handoff log.

The constraint that makes level 4 sane: **beads stay serial within a spec, by design.** Parallelism lives *across* specs, where the work is genuinely independent. The decomposition research MindSpec's plan gate is built on ([scaling-agent-systems.md](../../research/scaling-agent-systems.md)) is blunt about the ceiling — coordination overhead grows super-linearly with agent count, and 3–4 parallel agents is the sweet spot. A fleet of three governed loops that halt cleanly beats a swarm of twelve that need untangling.

## Climbing down

The ladder works in both directions, and stepping down is cheap: set a gate back to `human`, or `loop.enabled: false`, and you're at level 1; stop invoking autopilot and you're at level 0. The friction journal (`mindspec report`) is the compass for *when* — if escape-hatch use is concentrating somewhere, that's the part of the system that isn't ready for the rung you're on.

## Related

- [Review panels guide](review-panels.md) — the verifier every level depends on
- [README § The autonomy ladder](../../../README.md#the-autonomy-ladder) — the short version
- [Loop-engineering design notes](../../research/loop-engineering-adaptation.md) — the research and design history behind this ladder
