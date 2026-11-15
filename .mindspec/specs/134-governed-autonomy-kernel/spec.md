---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 134-governed-autonomy-kernel: Governed Autonomy Kernel

*Citation scope: code citations were checked against `origin/main` at `371dc360` (v0.19.0) on 2026-11-15. The named
symbol is authoritative. The source pack (`.tmp_agent/spec134-sources.md`, sections A-H) holds the rationale.*

## Goal

After this spec, the binary decides the next step of a plan-approved spec, and an orchestrator only executes it.

- `mindspec next-action` reads the plan tree and prints the next steps. It writes nothing unless `--apply` is given.
- `--apply` creates kernel beads (type `step`) just in time and enrols the spec. Enrolment is one-way.
- `mindspec lease` and `mindspec submit` claim and close a step with a fenced token and a receipt.
- In an enrolled spec, the warnings of today become refusals. A spec that nobody enrols behaves as today.
- The inert `loop:` config block is deleted. The round limit is `panel.max_rounds` and counts across all panels.

## Background

**Starting state (`371dc360`).** No `next-action`, `lease` or `submit` verb exists (`cmd/mindspec/`). The `Loop` doc
comment in `internal/config/config.go` says every `loop:` key is INERT. `loop.halt.max_rounds_per_bead` (default 3)
and `loop.gate_authority` are unread. `Load` refuses any `loop.halt.on_reject` other than `halt`. The rules
`panel.one-fixer` and `lifecycle.complete-by-orchestrator` in `internal/rules/rules.yaml` are `enforced_by: advisory`.
`enforcement.panel_presence` defaults to `warn`. Phase derivation (`classifyChild`, `internal/phase/derive.go`)
treats only `task`, `epic` and empty types as lifecycle children.

**Decided scope.** The operator decided pack sections A-C on 2026-11-15. This spec states them as requirements. The
graph model, rules B1-B13 and the verb contracts are pack sections A and B. This spec cites them as "pack §A/§B"
and does not retype them. bd 1.3.4 facts (pack §E): bd refuses a close of any parent with open children unless
`--force`; bd views inherit holds; bd does not refuse `--claim` on a blocked item; bd refuses a non-epic blocking an
epic.

**Already delivered (not re-specified):**

- Cross-spec finalize merge (`mindspec-blp6`): `77daf455`. R15 adds the general "computed ids only" test.
- Config key quoting and `config show` escapes (`mindspec-4xzi`, `mindspec-naq0` item 1): `2fbcbf76`, `09c43ec3`.
  Deleting `gate_authority` (R14) moots the rest of both beads.
- Orphaned-closed-bead floor `FindOrphanedClosedBeads` (`bf41b40f`, `internal/lifecycle/orphans.go`) and self-orphan
  handling in `complete`. R11 builds on them.
- Panel substrate and execution (`d15787fd`, `77efdd85`), readiness and `next --pick` (`ecce834f`, `462e3db8`),
  document and final gates with the presence policy (`907c7cd0`), `panel create --reviewer` (`1130dfa7`).

## Impacted Domains

- **workflow**: `cmd/**` (new verbs `next-action`, `lease`, `submit`; changed `panel create`, `panel submit`,
  `complete`, `next`, `impl approve`), `internal/panel/**` (round count per gate), `internal/complete/**`,
  `internal/approve/**` (audit table), `internal/next/**`, `internal/lifecycle/**`, `internal/doctor/**`,
  `internal/setup/**` (register `step`), `internal/rules/**`, `.claude/skills/**`, `plugins/mindspec/**` (skills).
- **core**: `internal/config/**` (delete `loop:`, add `panel.max_rounds`, `panel.lease_ttl`), `internal/phase/**`
  (phase lock; `step` stays non-lifecycle), `internal/redact/**` (escape-hatch registry).
- **execution**: `internal/executor/**` (finalize allow-set read only; no new export dependency).

## ADR Touchpoints

- [ADR-0045](../../adr/ADR-0045-binary-owns-sequence-and-rules.md): the reducer is the binary owning sequence.
- [ADR-0040](../../adr/ADR-0040-orchestration-layering-ratchet.md): skills loop the verbs; no rule lives in a skill.
- [ADR-0023](../../adr/ADR-0023.md): beads stay the single state authority; `step` is a non-lifecycle child type.
- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md): gate passes on the tally; presence policy.
- [ADR-0043](../../adr/ADR-0043-panel-disposition-telemetry-store.md): rounds are counted from the git ledger.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md): refusals carry a recovery line and an exit code.
- [ADR-0038](../../adr/ADR-0038-friction-reporter.md): `--allow-not-ready` joins the friction registries.

## Requirements

Every requirement holds the pack §A graph model and rules B1-B13 and every pack §B contract.

1. **Read-only reducer.** `next-action [<spec>] [--json]` writes nothing and prints the `mindspec.next-action/v1`
   document (pack §B). Failure answered: two skills pick the next bead by different rules (ADR-0045 Verification;
   #146, #187). `next-action` is read-only by default because agents probe (`rules.yaml` lines 87-91).
2. **Materialise and enrol.** `--apply` does one reduction under the phase lock: create missing steps just in time
   (B8), tally inline, cascade, print. It writes `mindspec_run` on the epic on first use. It is idempotent and
   resumes after a crash (B5). It refuses a cycle-making link (B7). Failure: a stalled run needs a human restart
   (`mindspec-qu9z`).
3. **Lease with fencing.** `lease <step>` claims with a monotonic generation, expiry and token hash under bd
   compare-and-set (pack §B). It refuses a step with an open ancestor or upstream (B1, B2). Failure: bd accepts
   `--claim` on a blocked item (pack §E); `panel.one-fixer` is advisory (`rules.yaml` lines 26-30).
4. **Submit with receipts.** `submit <step> --lease <t> (--artifact <p> | --commit <oid>)` checks the generation,
   subject and artifact, writes the receipt (B5, B13), closes and runs one reducer pass. Failure: subagents run
   `bd close`; `lifecycle.complete-by-orchestrator` is advisory (`rules.yaml` lines 92-96).
5. **Hand path closes steps.** The hand verbs close kernel steps in an enrolled spec (see Hand Path). The gate never
   waits on `complete`. Failure: without it, a hand fix deadlocks between `complete` and the review gate.
6. **Bead gate as child.** Enrolment creates `<w>.review` as a child of each work item and `<epic>.final` (B12).
   Failure: nothing deters a raw work-item close (`mindspec-xaoq`).
7. **Mode ratchet.** Orchestrated holds if ANY witness exists (pack §B ratchet). No flag, env variable or key sets
   or clears it. Failure: `panel_presence` defaults to `warn`, so the last cut enforced nothing.
8. **Pinned policy.** Enrolment pins `round_limit` and `lease_ttl` in `mindspec_run`. Config edits do not change a
   pinned spec. Failure: every `loop:` key is inert and editable (`Loop` struct, `internal/config/config.go`).
9. **Round limit across panels.** Count rounds per gate over every panel registration of the work item, abandoned
   included (`ForBead`, `internal/panel/panel.go`), from the git ledger. At the limit, add a decision
   `continue:<n>` or `halt`. Redispatch is free. Failure: the spec-128 plan gate ran 8 rounds
   (`.mindspec/specs/128-*/reviews/spec-128-plan-approve/`); `max_rounds_per_bead` is unread; a new slug restarts at
   round 1 (`mindspec-ztax`; `CheckCreateRound` is per directory).
10. **Local decisions.** A REJECT adds a decision for that work item only, `fix` or `halt`. Other items continue.
    There is no `accept`. Failure: `Load` forces `on_reject: halt` (`config.go`); autopilot halts the whole spec.
11. **Unreceipted closes.** A kernel close with no or bad receipt is flagged `unreceipted-close`. With evidence and
    the current generation, `submit` writes the receipt. Else the next pass creates `<id>.a<k+1>` with
    `mindspec_from`. Failure: closes reported but not persisted (`mindspec-2u0u`, `mindspec-yqdf`); raw subagent
    closes.
12. **Cascades.** A new subject supersedes the open slots of the round (B10). `halt` closes every open step
    `halted` and no builder bead (B11). Failure: abandoned work stays dispatchable after a restart or halt.
13. **Audit table.** `impl approve` prints mode, policy and counts of every escape hatch, deferral, takeover,
    decision answer and unreceipted close, from durable records. It never blocks. `--allow-not-ready` joins both
    friction registries (`escapeHatchFlags`, `cmd/mindspec/selfemit.go`; `EscapeHatchTokens`,
    `internal/redact/redact.go`). Failure: `--allow-not-ready` is in no registry (`mindspec-tgns`).
14. **Config honesty.** Delete the `loop:` block. `Load` warns once per removed key and ignores it.
    `loop.halt.max_rounds_per_bead` is a warned read alias of `panel.max_rounds` (default 5) for one release.
    `config show` prints one truthful label per key. Failure: inert keys look live (`Loop` comment); `runner:` is
    mislabelled (`mindspec-aili`).
15. **Kernel writes only computed ids.** Mindspec writes only the epic, approved work items and kernel ids (pack §B
    ownership). It reads builder beads, respects them as blockers and reports `foreign-child-open`. Failure class:
    the finalizer once merged another spec's bead (`mindspec-blp6`).
16. **Skills.** `ms-bead-cycle` loops `next-action --apply`, `lease`, dispatch, `submit`. The autopilot drops "One
    halted bead halts the spec", "Don't keep going after a halt" and the default-3 rounds. Failure:
    `.claude/skills/ms-spec-autopilot/SKILL.md` lines 17, 38, 76; `.claude/skills/ms-bead-cycle/SKILL.md` line 16.
17. **Setup.** `mindspec setup` registers `step` in bd `types.custom`. `doctor` reports it missing and flags a
    non-plan `task` child of an epic, suggesting `bug` or `chore`. `--apply` refuses without `step`. Failure: a
    builder `task` under an epic counts as lifecycle work (`classifyChild`).

**Constraints on all requirements.**

- Consumer-facing text this spec defines (CLI output, refusals, recovery lines, `next-action` output, skill text,
  setup files) cites no ADR, spec, requirement, bead or issue number.
- No new read or write of the committed `.beads/issues.jsonl`. The `impl approve` finalize path stays as it is, so
  the next spec can remove the export. Step beads reach the export until then; this is accepted.
- `next` (both modes) excludes kernel types from selection. Default bd views hide `step` where mindspec sets them.
- Threat model (pack §B): the kernel governs progression, not OS access. A same-user process can forge an answer.
  An agent can avoid opting in, so the repo floor is `enforcement.panel_presence: enforce`. One controller host.

## Hand Path

A solo builder who works by hand needs zero new commands. A spec that nobody enrols never has kernel beads, and
every verb behaves as today, with warnings where an enrolled spec refuses. In an enrolled spec, each step closes
through an existing verb, and each verb runs one reducer pass:

| Step | Closed by |
|:-----|:----------|
| implement `<w>.impl.a<k>` | `mindspec next <w>`, commit, then `panel create` round 1 closes it and makes slots |
| slot `<w>.review.r<N>.R<s>` | `panel submit` for that slot (`--lease` only on a leased slot) |
| bead gate `<w>.review` | the reducer pass in `panel submit` on a passing tally; a refused tally makes `r<N+1>.fix` |
| fix `<w>.review.r<N>.fix` | commit on `bead/<w>`, then `panel create` round N closes it |
| merge | `mindspec complete <w>` (a `run` entry, never a bead); it refuses unless the gate closed `passed` |
| final gate `<epic>.final` | the reducer pass in `panel submit` of the final-review panel |

A decision appears only after a REJECT or the round limit. It closes by `submit <decision> --answer <opt>`. Only an
enrolled spec gets decisions, and enrolment already needed `--apply`. No `decide`, `halt`, `adopt`, `takeover`,
`mode` or `run start` verb exists.

## Migration

- Nothing enrols by itself. Specs in flight stay hand-driven until someone runs `next-action --apply`. Closes before
  enrolment are recorded, not audited. There is no version stamp.
- A config with `loop:` keys loads with one warning per key. `max_rounds_per_bead` is read as `panel.max_rounds`
  with a warning for one release, then ignored. `panel_deadlock_rounds` and `max_consecutive_impl_failures` are
  deleted.
- `mindspec setup` registers `step`. In a repo without it, `doctor` reports it and `--apply` refuses with the setup
  command as the recovery line.
- `enforcement.panel_presence` keeps its `warn` default. A repo that needs a floor against an agent that never
  enrols sets `enforce`; the ratchet does not replace that floor.

## Scope

### In Scope
- The plan-approved tree of an enrolled spec (implement, bead review, fix, decision, final review); R1-R17.

### Out of Scope
- Spec-draft and plan-draft steps (the epic is created at `spec approve`, `internal/approve/spec.go`).
- The merge as a bead: `complete` needs caller input and has its own recovery (#250).

## Non-Goals

**Not doing (pack §D).**

- Authority table or `AWAIT_OPERATOR` card; grants DSL or records; blocking caps on escape hatches; per-wake
  ceilings, handoff-log location (`mindspec-s4cs`), `token_budget`; orchestrated defaults for readiness and grill;
  replacing the lane kit ledger and wakes; new reads or writes of committed `.beads/issues.jsonl`.
- Rejected: enrol on read; bd's lease or a bd-actor holder check; unfenced leases; ownership by type; custody
  marker, version stamp, `next --manual/--return`; an operator decision per raw edit; auto-reopen; round beads; one
  review step per round; a `halt-bead` answer; per-directory round counts; `--ephemeral` steps.
- Later, in order: tracker-export removal (`mindspec-lzx4`, next spec); document-phase steps; parallel waves and
  cross-host lease CAS (`mindspec-m4ks`); a driver `mindspec run`; a restricted runner; skill consolidation
  (`mindspec-sxjc`); the worktree as a rebuildable cache.

## Acceptance Criteria

All ACs run in a scratch repo with a scratch bd database and a plan-approved spec `x` with three work items.

- [ ] AC1 (R1): `next-action x --json` twice on a hand spec prints byte-identical output with `"mode":"hand"` and an
  `implement` action for work item 1 with `"exists":false`; `bd list --all --json | wc -l` is unchanged.
- [ ] AC2 (R2, B8): the first `--apply` creates `<w>.review` per work item, `<epic>.final`, one implement step for
  item 1 and `mindspec_run` on the epic, and no fix or slot step. A second `--apply` creates 0 beads and prints the
  same `revision`. A run killed after its first write, then rerun, ends with the bead set of an uninterrupted run.
- [ ] AC3 (R2, B5): a run killed between the receipt write and the bd close, then rerun, leaves the step closed with
  that receipt and exit 0.
- [ ] AC4 (R2, B7): with a builder link that makes work item 2 block work item 1, `--apply` exits non-zero, names
  the cycle and creates 0 beads.
- [ ] AC5 (R3, B1, B2): `lease <child of a blocked item>` exits non-zero and names the blocker. A second `lease` on a
  live step exits non-zero. After the TTL, `lease` prints generation 2.
- [ ] AC6 (R4, B13): a generation-1 `submit` after `lease --take` exits non-zero and the step stays open. A valid
  `submit` closes the slot; `bd show --json` shows `mindspec_close` with path, commit and blob id, and no verdict
  text. A repeated `submit` exits 0 and `bd show --json` is byte-unchanged.
- [ ] AC7 (R5): an enrolled spec reaches `impl approve` with only `next`, `panel create`, `panel submit` and
  `complete`; `next-action x --json` then prints `"status":"done"` and exits 5.
- [ ] AC8 (R6, B12): `bd close <work item>` exits non-zero with bd's open-child refusal. After the gate closes
  `passed`, `panel create` for that work item exits non-zero.
- [ ] AC9 (R7): enrol, close all steps, let leases expire, set `panel_presence: warn`, retype a step to `chore`;
  `complete` with no passing panel exits non-zero.
- [ ] AC10 (R8): enrol with limit 5, set `panel.max_rounds: 99`; `panel create` round 6 exits non-zero.
- [ ] AC11 (R9): after 5 rounds on slug A, `panel create` slug B for the same work item exits non-zero and
  `next-action` lists exactly one decision with options `continue:<n>` and `halt`. `panel recover` of round 5 exits 0.
- [ ] AC12 (R10): a REJECT tally on item 2, then `--apply`, lists a decision under item 2 and an action for item 3.
- [ ] AC13 (R11): `bd close <slot step>` with no verdict; `next-action` flags `unreceipted-close` and offers
  `<slot>.a2`; `impl approve` exits non-zero until it is resolved.
- [ ] AC14 (R12): a commit on `bead/<w>` in round 2 with no fix open makes the round-2 slots show
  `return: superseded`. Answer `halt`: every open step shows `halted`, `next-action` exits 4, and a builder `bug`
  child of the epic stays open.
- [ ] AC15 (R13): `next --allow-not-ready=<id>` once; `impl approve` prints `allow-not-ready: 1`; a rerun prints 1.
- [ ] AC16 (R14): a config with `loop.gate_authority` and `loop.halt.on_reject: fix` loads with exactly one warning
  per key and exit 0; `config show` labels them ignored and labels `runner:` as read by a skill.
- [ ] AC17 (R15, B3, B6): a test diffs `bd list --all --json` and dependencies around each kernel verb; every changed
  id is in the computed set, every new link is child or blocks in plan order or to `<epic>.final`, and a builder
  `task` under the epic is byte-unchanged.
- [ ] AC18 (R16): `git grep -n -e "One halted bead" -e "keep going after a halt" -e "default .3."` over
  `.claude/skills` and `plugins/mindspec/skills` exits 1 (the `internal/setup/historical_skills` snapshots keep the
  old text so installs refresh); `cmp` of each skill copy pair exits 0; `mindspec setup` refreshes an old install.
- [ ] AC19 (R17): in a repo without `step` registered, `doctor` prints a line naming it and `--apply` exits non-zero
  with the setup command as recovery; after `mindspec setup`, `--apply` exits 0.
- [ ] AC20 (constraints): a test scans every new CLI string and skill text for ADR, spec, requirement, bead and issue
  number patterns and finds none; `git diff origin/main --stat -- .beads/issues.jsonl` shows no change by the code.

## Validation Proofs

- The scratch-repo acceptance tests for AC1-AC20: each passes and fails on a named mutation.
- `mindspec validate spec 134-governed-autonomy-kernel` exits 0 after the open questions close; `make test` exits 0.

## Open Questions

- [ ] grill deferred: headless session
- [ ] OQ1: Does `bd create --id` accept word segments such as `.review.r2.R3`? Recommended: the first plan bead
  probes it in a scratch database. Fallback: a bd-assigned id plus `mindspec_key` metadata that holds the computed id.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
