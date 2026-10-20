# Review Panels

The review panel is MindSpec's verifier: a parallel, multi-model, multi-lens review whose decision is computed by the `mindspec` binary — not negotiated in a chat window. This guide covers how panels work, how to configure them, and the rules that keep them honest.

The design premise is **maker ≠ verifier**. The agent that implements a bead makes exactly one commit and cannot merge, close, or approve its own work. The panel judges the diff; the binary tallies the verdicts; only then does the merge gate open.

## Anatomy of a panel

A panel run produces a directory of artifacts that live in the repo, next to the work they judged:

```
.mindspec/specs/<spec-slug>/reviews/<panel-slug>/
├── panel.json                  # the gate's source of truth
├── BRIEF.md                    # what reviewers were asked to judge
├── <slot>-round-1.json                  # one verdict per reviewer per round
│                                        # (e.g. R1-round-1.json, codex-r4-round-1.json)
└── consolidated-round-1.md     # the merged change list, if a fix round is needed
```

`panel.json` records the target (bead, spec, plan, final branch diff, or PR), the round number, `expected_reviewers`, the reviewer roster, `reviewed_head_sha`, and the subject digest. `reviewed_head_sha` identifies the commit the panel judged. Ad-hoc panels (repo reviews, pre-release checks) use `.mindspec/reviews/<panel-slug>/` on flat layouts. Other layouts use `review/<panel-slug>/`.

Each reviewer writes a verdict JSON: `reviewer_id`, a verdict (`APPROVE` / `REQUEST_CHANGES` / `REJECT`), `confidence`, `rationale`, `concrete_changes_required`, structured `findings`, and an optional `hard_block`.

## Six lenses, two model families

The gate sets the default panel size: **6 reviewers for spec, plan, final, and adhoc reviews; 4 for bead reviews**. Slots alternate between Claude and Codex in round-robin order. Reviewers run in parallel. To override these defaults, set `panel.reviewers` in `.mindspec/config.yaml`, or set `panel.gates.<gate>.reviewers` for one gate. Cross-family review is deliberate: each model family has systematic blind spots, and a family split on a verdict (one family approves, the other doesn't) is itself reported as a signal.

Six reviewers would be pointless as six clones, so each slot carries a distinct lens:

| Slot | Lens | The question it asks |
|:-----|:-----|:---------------------|
| R1 | Author-of-record | Does the diff actually do what the plan said this bead would do? |
| R2 | Codebase pin | Do the files, functions, and tests named in the claims really exist — and pass? |
| R3 | Contract stability | Do prompts, schemas, and public interfaces stay coherent for downstream consumers? |
| R4 | Empirical prober | Run the validators and proofs by hand; believe nothing that wasn't executed. |
| R5 | Schema/type correctness | Are the data shapes, types, and edge cases right? |
| R6 | Next-bead integration | Will the *next* bead land cleanly on top of this one? |

The final review of a whole spec branch (run once, after the last bead merges) swaps in branch-level lenses: cumulative scope drift, inter-bead coherence, clean integration with main, PR-description accuracy, acceptance-criteria release readiness, and operator readiness.

## The decision matrix

Tallying is not a judgment call. The decision is a pure function in the binary — identical facts produce an identical decision, whether the panel was launched by a human, a skill, or an unattended loop.

- **Pass rule.** A panel passes only when it is fresh and intact, every roster slot has a verdict, and it has 0 REJECT, 0 `hard_block`, 0 INVALID and 0 unresolved REQUEST_CHANGES verdicts. The APPROVE count is not a decision input.
- **Each finding is fixed, refuted or deferred.** To fix findings, run a fix round. Consolidate the reviewers' `concrete_changes_required` lists into one change list. A fix agent applies it. Increase the round number and update `reviewed_head_sha` together. The panel reviews the new head. To refute a finding, record evidence with `mindspec panel refute`. To defer a finding, name an open follow-up bead with `mindspec panel defer`. Stale verdicts never count. If the panel passes apart from staleness and the new commits need no new review (for example, an ownership-only commit), run `mindspec panel rebind <slug> --reason '<who>: <why>'` instead. It moves the panel to the new tip with no new round and records the changed paths. It refuses a failing panel.
- **Any REJECT → halt.** A REJECT stops the track for a human. It is never auto-fixed: a rejection that gets quietly patched over is precisely the verification debt the panel exists to prevent.
- **Bounded rounds.** A bead that keeps failing its panel halts after a configured maximum rather than looping forever.

### Artifact hard gates

One rule outranks the vote count: if a finding names a **missing measurement artifact** — a benchmark that was promised, a cost projection, a drift report, a regression baseline — the panel blocks regardless of how many reviewers approved. The distinguishing test is: *could the missing artifact have caught a real defect?* If yes, no quantity of approvals substitutes for it. Agents can't vote evidence into existence, and neither can reviewers.

## Configuration

The panel machinery is configured in `.mindspec/config.yaml`:

```yaml
panel:
  reviewers:
    - {family: claude, count: 3}
    - {family: codex,  count: 3}
  substitution:
    claude_sub_on_quota: true   # if a codex slot hits a quota wall, a claude
                                # substitute fills the SAME slot id — the panel
                                # never silently shrinks

models:                          # optional per-phase model selection
  implement: <model-id>          # phases: implement, review, authoring,
  review: <model-id>             #         grill, final_review
  final_review: <model-id>

runner: claude-code-skills       # who executes the panel plumbing (see below)
```

Config load refuses a one-reviewer panel because it cannot pass. Panel skipping cannot be delegated to config.

`mindspec config show` prints the effective merged configuration.

## The panel verbs

The panel lifecycle is a set of agent-neutral CLI verbs — the contract any orchestrator can drive:

```bash
mindspec panel create <slug> --spec <id> --target <ref> [--gate bead --bead <id>] [--round N]
                                 # writes the panel dir, BRIEF, panel.json in one
                                 # operation; pins round, roster, reviewed_head_sha, subject digest
mindspec panel verify <slug>     # read-only: completeness + staleness report,
                                 # prints the same PASS/BLOCK the gate computes
mindspec panel tally <slug>      # renders the decision from the binary:
                                 # verdict table, decision, consolidated changes;
                                 # exit 0 on allow, non-zero (with recovery) on block
# Refute one finding with evidence. Omit --finding to cover the whole slot.
mindspec panel refute <slug> --slot R2 --round N --finding 'ccr[0]' --ground disproven --reason '<why>' --evidence <reference> --by '<name>'
# Defer a finding to an open follow-up bead.
mindspec panel defer <slug> --slot R2 --round N --finding 'ccr[0]' --bead <follow-up> --by '<name>'
# Move a passing panel to the new tip with no new round.
mindspec panel rebind <slug> --reason '<who>: <why>'
# Abandon a panel with an audited reason.
mindspec panel abandon <slug> --reason '<why>' --by '<name>'
```

The **runner** selects who executes the plumbing between those verbs — launching reviewers, collecting verdicts:

- `claude-code-skills` (default) — the `/ms-panel-run` and `/ms-panel-tally` skills orchestrate reviewers from your Claude Code session.
- `claude-code-workflow` — the `/ms-panel` workflow runs the whole round as one deterministic fan-out and returns a single compact result, keeping six reviewers' transcripts out of your session's context. Recommended once you're running loops.
- `external` — bring your own orchestrator; anything that can exec the panel verbs and write verdict JSONs can run a panel.

Whatever the runner, it is an adapter, never a second decision authority: the decision stays in the binary, and a runner cannot merge, complete, or skip.

## Escape hatch

`MINDSPEC_SKIP_PANEL=1` skips the panel gate on a `complete`. It is environment-only (never a flag, never printed in error messages), intended for humans in exceptional circumstances, and every use is recorded as an audit entry on the bead it skipped (`panel_gate_skipped`, with a timestamp). If you find yourself reaching for it twice, the panel configuration — not the gate — is what needs fixing.

## Cost and when to lighten the panel

Six frontier-model reviewers per round is a real cost, and it's the right default for exactly the reason it's expensive: it's the thing standing between semi-autonomous merges and unreviewed slop. If you want to economize, do it explicitly — a smaller `reviewers:` mix in config on low-stakes repos — rather than by skipping. The pass rule, artifact hard gates and REJECT rules apply at every panel size.

## Related

- [Autonomy guide](autonomy.md) — panels as gate authority at levels 2+
- [README § Review panels](../../../README.md#review-panels) — the short version
