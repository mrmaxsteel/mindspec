---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 134-governed-autonomy-kernel: Governed Autonomy Kernel

*Code citations: `origin/main` at `371dc360` (v0.19.0), 2026-11-15; the named symbol is authoritative. Normative:
[contracts.md](contracts.md) (§1-§12 operator rulings; §13 readings where a ruling is silent; §14 the hand path).*

## Goal

After this spec, the binary decides the next step of a plan-approved spec, and an orchestrator only executes it.

- `mindspec next-action` reads the plan tree and prints the next steps. It writes nothing unless `--apply` is given.
- `--apply` creates kernel beads (type `step`) just in time and enrols the spec. Enrolment is one-way.
- `mindspec lease` and `mindspec submit` claim and close a step with a fenced token and a receipt.
- In an enrolled spec, the warnings of today become refusals. A spec that nobody enrols behaves as today.
- The inert `loop:` config block is deleted. The round limit is `panel.max_rounds` and counts across all panels.

## Background

**Starting state (`371dc360`).** No `next-action`, `lease` or `submit` verb exists. The `Loop` doc comment in
`internal/config/config.go` says every `loop:` key is INERT; `Load` refuses any `loop.halt.on_reject` but `halt`.
`panel.one-fixer` and `lifecycle.complete-by-orchestrator` (`internal/rules/rules.yaml`) are advisory;
`panel_presence` defaults to `warn`. `classifyChild` (`internal/phase/derive.go`) treats only `task`, `epic` and
empty types as lifecycle children. `plan approve` writes bd blocks links from the plan's `depends_on`.

**Already delivered (not re-specified):** cross-spec finalize merge (`mindspec-blp6`, `77daf455`); config key
quoting and escapes (`mindspec-4xzi`, `mindspec-naq0` item 1: `2fbcbf76`, `09c43ec3`); `FindOrphanedClosedBeads`
(`bf41b40f`); panel substrate and execution (`d15787fd`, `77efdd85`); readiness (`ecce834f`, `462e3db8`); document
and final gates (`907c7cd0`); `panel create --reviewer` (`1130dfa7`).

## Impacted Domains

- **workflow**: `cmd/**` (new `next-action`, `lease`, `submit`; changed `panel create|submit|refute|defer`,
  `complete`, `next`, `impl approve`), `internal/{panel,complete,approve,next,lifecycle,doctor,setup,rules}/**`,
  `.claude/skills/**`, `plugins/mindspec/**` (skills, `README.md`); docs: `README.md`,
  `project-docs/user/guides/autonomy.md` and the `loop.halt` mentions in `.mindspec/domains/workflow/*.md`.
- **core**: `internal/config/**` (delete `loop:`; add `panel.max_rounds`, `panel.lease_ttl`), `internal/phase/**`,
  `internal/redact/**`, `.mindspec/domains/core/interfaces.md` (the `Loop` struct and the `Load` refusal list).
- **execution**: `internal/executor/**` (finalize allow-set read only; no new export dependency).

## ADR Touchpoints

- [ADR-0045](../../adr/ADR-0045-binary-owns-sequence-and-rules.md): the reducer is the binary owning sequence.
  [ADR-0040](../../adr/ADR-0040-orchestration-layering-ratchet.md): skills loop the verbs; no rule lives in a skill.
- [ADR-0023](../../adr/ADR-0023.md): beads stay the single state authority; `step` is a non-lifecycle child type.
  [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md): gate passes on the tally; presence policy.
- [ADR-0043](../../adr/ADR-0043-panel-disposition-telemetry-store.md): rounds are counted from the git ledger.
  [ADR-0038](../../adr/ADR-0038-friction-reporter.md): `--allow-not-ready` joins the friction registries.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md): recovery lines; HC-4 status clause for `next-action` (OQ8).

## Requirements

Every requirement holds the graph model (contracts §2) and rules B1-B13 (contracts §3).

1. **Read-only reducer.** `next-action [<spec>] [--json]` writes nothing and prints `mindspec.next-action/v1`
   (contracts §4). Failure: two skills pick the next bead by different rules (ADR-0045 Verification; #146, #187).
2. **Materialise and enrol.** `--apply` does one reduction under the phase lock (contracts §4, §8). It is idempotent,
   resumes after a crash (B5) and refuses a cycle-making link (B7) before any write. Implement steps: §13.10; review
   rounds: §13.14. A work item closed before enrolment gets no gate (§13.8). Failure: a run stalls (`mindspec-qu9z`).
3. **Lease with fencing.** `lease` claims per contracts §5. It refuses a held or blocked step, ancestor or upstream
   (B1, B2; contracts §13.1). Failure: bd accepts `--claim` on a blocked item; `panel.one-fixer` is advisory.
4. **Submit with receipts.** `submit` checks and closes per contracts §6 (B5, B13). Failure: subagents run
   `bd close`; `lifecycle.complete-by-orchestrator` is advisory.
5. **Hand path closes steps.** An unenrolled spec runs zero new commands. In an enrolled spec, the changed verbs
   (contracts §7, §13.2-§13.4, §14) close kernel steps, and `submit <decision> --answer <opt>` is the one new hand
   command. The gate never waits on `complete`. Failure: a hand fix deadlocks between `complete` and the gate.
6. **Bead gate as child.** Enrolment creates `<w>.review` under each open work item, and `<epic>.final`. A passed
   gate is frozen (B12; successor gate: §13.6). Failure: nothing deters a raw work-item close (`mindspec-xaoq`).
7. **Mode ratchet.** Orchestrated holds if ANY witness exists (contracts §8). No flag, env variable or key sets or
   clears it. Failure: `panel_presence` defaults to `warn`, so the last cut enforced nothing.
8. **Pinned policy.** Enrolment pins `round_limit` and `lease_ttl` in `mindspec_run`. Config edits do not change a
   pinned spec. Failure: every `loop:` key is inert and editable (`Loop` struct, `internal/config/config.go`).
9. **Round limit across panels.** Contracts §10, §13.5: a bead gate counts every registration of the work item
   (`ForBead`, `internal/panel/panel.go`), the final gate every `final_review` registration (`lifecycleRegistrations`).
   Failure: a new slug restarts at round 1 (`mindspec-ztax`; `CheckCreateRound` is per directory).
10. **Local decisions.** A REJECT adds a decision for that gate only, `fix` or `halt`; other items continue. There is
    no `accept` (§13.15). The `halt` option halts the whole spec (§13.7). Failure: `Load` forces `on_reject: halt`.
11. **Unreceipted closes.** Contracts §9. Failure: closes not persisted (`mindspec-2u0u`, `mindspec-yqdf`); raw closes.
12. **Cascades.** B10 (supersede, except during an open fix; then the next round, §13.11) and B11 (`halt`; after
    halt: §13.7). Failure: abandoned work stays dispatchable after a restart or halt.
13. **Audit table.** `impl approve` prints the contracts §11 table, which never blocks; it keeps today's refusals
    and adds those of contracts §7 and §9 (§13.12). `--allow-not-ready` joins `escapeHatchFlags` (`selfemit.go`)
    and `EscapeHatchTokens` (`internal/redact/redact.go`). Failure: it is in no registry (`mindspec-tgns`).
14. **Config honesty.** Delete the `loop:` block. `Load` warns once per removed key, including every `loop.halt.*`
    key, and ignores it, except the alias in contracts §13.9. `config show` prints one truthful label per key.
    Failure: inert keys look live (`Loop` comment); `runner:` is mislabelled (`mindspec-aili`).
15. **Kernel writes only computed ids.** Contracts §12. Failure class: the finalizer once merged another spec's bead.
16. **Skills and docs.** `ms-bead-cycle` loops `next-action --apply`, `lease`, dispatch, `submit` and each `run` entry.
    Both skills drop the deleted keys, `max-rounds` and every spec-wide halt row (§13.13); docs drop deleted keys.
    Failure: both copies of both skills name default-3 rounds, a deleted key and spec-wide halts.
17. **Setup.** `mindspec setup` registers `step`. `doctor` reports it missing and flags a non-plan `task` child of
    an epic, suggesting `bug` or `chore`. Failure: a builder `task` under an epic counts as lifecycle work.

**Constraints.** Consumer-facing text this spec defines (CLI output, refusals, recovery lines, `next-action` output,
skill text, setup files) cites no ADR, spec, requirement, bead or issue number. No new read or write of the committed
`.beads/issues.jsonl`; the `impl approve` finalize path stays, and step beads reach the export until the next spec
removes it. `next` excludes kernel types from selection.

## Migration

- Nothing enrols by itself; specs in flight stay hand-driven until `next-action --apply`. No version stamp exists.
- `loop:` keys load with one warning each. `panel_presence` keeps its `warn` default; a repo that needs a floor against
  an agent that never enrols sets `enforce`. Without `step`, `doctor` reports it and `--apply` refuses with a recovery.

## Scope

### In Scope
- The plan-approved tree of an enrolled spec (implement, bead review, fix, decision, final review); R1-R17.
- Tracker records after a check: close `mindspec-4xzi` and `mindspec-naq0`; re-scope `mindspec-qu9z` to "re-call
  `next-action`; an expired lease frees the step"; close `mindspec-sxjc` only if the `ms-impl-approve` copies match.

### Out of Scope
- Spec-draft and plan-draft steps (the epic is created at `spec approve`). The merge as a bead (#250).

## Non-Goals

- Authority table or `AWAIT_OPERATOR` card; grants DSL; blocking caps on escape hatches; per-wake ceilings, handoff
  log (`mindspec-s4cs`), `token_budget`; orchestrated readiness and grill defaults; replacing the lane kit ledger.
- Rejected: enrol on read; bd's lease or holder check; unfenced leases; ownership by type; custody marker, version
  stamp, `next --manual/--return`; per-edit operator decisions; auto-reopen; round beads; one review step per round;
  a `halt-bead` answer; per-directory round counts; `--ephemeral` steps.
- Later, in order: tracker-export removal (`mindspec-lzx4`); document-phase steps; parallel waves and cross-host
  lease CAS (`mindspec-m4ks`); a driver `mindspec run`; a restricted runner; skill consolidation; rebuildable worktree.

## Acceptance Criteria

All ACs run in a scratch repo and bd database on a plan-approved spec `x`: items 2 and 3 each depend on item 1.
"Snapshot" = full `bd list --all --json`, every dependency list, `git for-each-ref` and `git status --porcelain`.

- [ ] AC1 (R1): `next-action x --json` twice on a hand spec: the outputs are byte-identical with `"mode":"hand"` and
  an `implement` action for item 1 with `"exists":false`; the snapshot before and after is byte-identical.
- [ ] AC2 (R2, B8): the first `--apply` creates exactly `<w>.review` for each item, `<epic>.final`, `<1>.impl.a1` and
  `mindspec_run`; no fix, slot or item-2/3 implement step. A second `--apply` leaves the snapshot byte-identical and
  prints the same `revision`. A run killed after its first write, then rerun, ends with the uninterrupted snapshot.
  After item 1 closes, one `--apply` creates exactly `<2>.impl.a1` and `<3>.impl.a1` and no slot (§13.10).
- [ ] AC3 (R4, B5): `submit` on a slot, killed after the receipt write and before the bd close, then rerun: exit 0,
  the step is closed with that receipt, no `unreceipted-close` flag. At no kill point does the store hold a closed
  kernel step without `mindspec_close`; a mutation that closes first goes red.
- [ ] AC4 (R2, B7): a blocks link making item 1 wait on item 2 (by `bd dep add`, or a fake bd if bd refuses it), and
  separately a blocks link between the epic and item 1: `--apply` exits non-zero, names the link, and the snapshot
  and `bd show <epic> --json` stay byte-identical (no `mindspec_run`; mode stays `hand`).
- [ ] AC5 (R3, B1, B2): `lease <1>.impl.a1` under the open epic and open item 1 exits 0 with generation 1. A second
  `lease` exits non-zero. After the TTL, `lease` prints generation 2. With a builder bug B blocking item 1,
  `lease <1>.review.r1.R1` exits non-zero and names B.
- [ ] AC6 (R4, B13): each exits non-zero and leaves the step open: a generation-1 `submit` after `lease --take`; a
  verdict for the previous round's subject, or for another slot; a blob that differs at the named commit; a verdict to
  a fix step; an implement commit not past the claim base. A valid `submit` closes the slot with `mindspec_close` =
  path, commit, blob id, no verdict text. Rerun: exit 0; a different artifact: non-zero, `bd show --json` unchanged.
- [ ] AC7 (R5): with only `next` (run twice on item 1; the rerun exits 0), commits, `panel create`, `panel submit` and
  `complete`, `impl approve` exits 0, every kernel step is closed with `mindspec_close`, and `next-action x --json`
  prints `"status":"done"` and exits 5. With a live `lease` on an implement step, `next <w>` exits non-zero naming
  `lease --take`, and `panel create` without its `--lease` exits non-zero with the step open.
- [ ] AC8 (R6, B12): `bd close <work item>` exits non-zero with bd's open-child refusal. After a pass, `panel create`
  for that item exits non-zero naming the passed gate, and a later `--apply` leaves `bd show <w>.review --json`
  unchanged. A code commit plus `panel rebind` lets `complete` exit 0. A `.mindspec/config.yaml` commit instead:
  `complete` exits non-zero and its pass creates `<w>.review.a2` with `mindspec_from` = `<w>.review`; `panel create`
  exits 0 and continues the round count; after a pass, `complete` exits 0 (§13.6).
- [ ] AC9 (R7): enrol, close all steps, let leases expire, set `panel_presence: warn`, retype a step to `chore`;
  `complete` with no passing panel exits non-zero. Remove `mindspec_run` from the epic: `complete` still exits
  non-zero and `next-action` flags `run-marker-missing`. With only a spec `panel.json` holding `"orchestrated":
  true` (marker and kernel beads deleted), `complete` exits non-zero.
- [ ] AC10 (R8): enrol with limit 5 and TTL 2h; set `panel.max_rounds: 99` and `panel.lease_ttl: 10m`; `panel
  create` round 6 exits non-zero and a new lease expires 2h after it starts.
- [ ] AC11 (R9): 3 rounds on slug A, `panel abandon` A, 2 rounds on slug B; round 3 of slug B exits non-zero;
  `next-action` lists exactly one decision with `continue:<n>` and `halt`; `panel recover` of round 5 exits 0. After
  `continue:1`, one more round is created and the next one is refused.
- [ ] AC12 (R10): item 1 done; a REJECT tally on item 2, then `--apply`, lists a decision under item 2 and an action
  for item 3, and no step of item 3 changes. The decision text says `halt` stops the whole spec. Before the answer,
  `lease <2>.review.r<N+1>.fix` exits non-zero and names the decision. After `--answer fix`, `--apply` lists that fix
  as an action and `lease` on it exits 0 (§13.15).
- [ ] AC13 (R11, R13): `bd close <slot step>` with no verdict: `next-action` flags `unreceipted-close` and offers
  `<slot>.a2`; `impl approve` exits non-zero until resolved. A raw close of a slot with a valid verdict and the current
  generation: `submit` writes the receipt, no `.a2` exists, and the flag clears. A raw `bd close <w>` after its gate
  passed: `impl approve` exits non-zero and `next-action` flags `raw-close` on `<w>`. `bd close --force <w>` with
  `<w>.review` open: `complete` and `impl approve` exit non-zero and `next-action` flags `forced-close`.
- [ ] AC14 (R12): a commit on `bead/<w>` in round 2, no fix open: one pass marks round-2 slots `return: superseded`,
  sets the gate subject to it and lists `panel create <w>` in `run`, which makes counted round 3 (§13.11); at limit 2
  the pass gives the limit decision and no `run` entry. A commit during an open `r2.fix` supersedes no slot. Answer
  `halt`: every open step shows `halted`; `next-action` exits 4 twice; `lease`, `complete`, `next <w>`, `panel create
  <w>` exit non-zero naming the halt, snapshot unchanged; a builder `bug` child stays open.
- [ ] AC15 (R13): once each: `next --allow-not-ready=MF-1 --evidence <path>`, `--allow-doc-skew`, `--override-adr`,
  `--allow-net-deletion`, `lease --take`, `panel defer`, a decision answer, an unreceipted close, `MINDSPEC_SKIP_PANEL=1
  complete`: `impl approve` and a rerun print a count of 1 for each (`allow-not-ready: 1`); both registries hold it.
- [ ] AC16 (R14): a config with `loop.gate_authority`, `loop.halt.on_reject: fix`, `loop.halt.panel_deadlock_rounds`
  and `loop.halt.max_consecutive_impl_failures` loads with exit 0 and exactly one warning per key; `config show`
  labels them ignored and `runner:` as read by a skill. `loop.halt.max_rounds_per_bead: 4` alone enrols with limit 4
  and a warning naming 4; with `panel.max_rounds: 6` also set, limit 6; no key enrols with limit 5.
- [ ] AC17 (R15, B3, B6): a snapshot diff around each kernel verb: every changed id is computed, every new link is
  child or blocks in plan order or to `<epic>.final`, and a builder `task` under the epic is byte-unchanged.
- [ ] AC18 (R16): with K = `max_rounds_per_bead|panel_deadlock_rounds|max_consecutive_impl_failures|gate_authority` and
  H = `Any bead REJECT|consecutive impl|REJECT / HARD block|do not proceed to the next bead`, `git grep -n -i -E
  "$K|$H|on_reject|loop\.halt|max-rounds|One halted bead|keep going after a halt|default .?3" -- .claude/skills
  plugins/mindspec/skills plugins/mindspec/README.md README.md project-docs/user/guides` and `git grep -n -E
  "$K|on_reject|type Loop struct" -- .mindspec/domains` each exit 1 (`historical_skills` keeps the old text). Both
  `ms-bead-cycle` copies contain `next-action --apply`, `mindspec lease` and `mindspec submit`. `cmp` of each skill
  pair where both copies exist exits 0; `mindspec setup` refreshes an old install.
- [ ] AC19 (R17): without `step` registered, `doctor` prints a line naming it and `--apply` exits non-zero with the
  setup command as recovery; after `mindspec setup`, `--apply` exits 0.
- [ ] AC20 (constraints): in the lines this change adds to non-test Go files under `cmd/` and `internal/` and to the
  AC18 skill copies, no string literal or skill line holds an ADR, spec, requirement, bead or issue number, and no
  added line reads or writes `.beads/issues.jsonl`. The existing finalize export still runs at `impl approve`.
- [ ] AC21 (R2): item 1 closed by `mindspec complete` with presence `warn` and no panel, then the first `--apply`: no
  `<1>.review`, `mindspec_run` records item 1 as pre-enrolment, items 2 and 3 run, and `impl approve` exits 0 with
  `pre-enrolment: 1`.
- [ ] AC22 (R5): a refused tally makes `r2.fix`; `panel refute` of the only blocking finding closes the gate
  `passed` and the fix `superseded` in one pass; no child stays open; `complete` exits 0.
- [ ] AC23 (R9, R6): a refused final tally lists a `fix` under `<epic>.final`; after `round_limit` final rounds over
  two final-review slugs, `panel create` exits non-zero and a limit decision appears. Separately: final pass on commit
  A, then a `.mindspec/config.yaml` commit B: `impl approve` exits non-zero, its pass creates `<epic>.final.a2` (a
  mutation that accepts A's pass for B goes red); final `panel create` continues the count; a pass on B lets it exit 0.
- [ ] AC24 (R1-R5, B8): only `next-action --apply`, `lease`, `submit` and each printed `run` entry reach `impl approve`
  exit 0 and `"status":"done"`. After `submit <1>.impl.a1`, `--apply` lists `panel create` for item 1 in `run`; after
  it runs, `--apply` lists `<1>.review.r1.R<s>` slots, `ForBead` counts 1 round, and a rerun of it exits non-zero.

## Validation Proofs
The scratch-repo tests for AC1-AC24 each pass and fail on a named mutation; `make test` exits 0.

## Open Questions

- [ ] grill deferred: headless session
- [ ] OQ1: Does `bd create --id` accept word segments (`.review.r2.R3`)? Recommended: the first plan bead probes it;
  fallback: a bd-assigned id plus `mindspec_key` metadata.
- [ ] OQ2: lease refuses an "open ancestor or upstream" (§5), but every step has one (R2 F3, R4 ccr0, R5 ccr0). §13.1.
- [ ] OQ3: §7 omits hand `next`, leased `panel create`, refute after a fix (R3 F6; `panel_refute.go`). Use §13.2-§13.4.
- [ ] OQ4: no final-gate fix id exists; `ForBead` skips final panels (R3 F3, `internal/panel/panel.go`). Use §13.5.
- [ ] OQ5: B12 freezes a passed gate, but `panel rebind` refuses a `.mindspec/config.yaml` change, for bead and final
  gates (R3 F2, R4 ccr0, R2 F16; `cmd/mindspec/panel_rebind.go`). Recommended: successor gates (§13.6).
- [ ] OQ6: `halt` "ends orchestration" (§10), but the ratchet is one-way; verb lists left halt gaps (R3 F1, F14,
  F15, F17). Recommended: one halt rule over every verb (§13.7).
- [ ] OQ7: pre-enrolment closes are "recorded, not audited", yet enrolment makes every gate (codex R6-F1). Use §13.8.
- [ ] OQ8: exits 3/4/5 conflict with ADR-0035 HC-4; `indeterminate` = 1 equals a refusal (R3 F5, F11). Amend HC-4.
- [ ] OQ9: B8 has no implement outcome; parallel waves are deferred (R2 F14; pack §D). Use §13.10; else one per pass.
- [ ] OQ10: B10 is silent on the gate after a supersede (R2 F15). Recommended: authorise round N+1 (§13.11).
- [ ] OQ11: §7 and §9 list `impl approve` refusals but are silent on today's gates and skill halt rows (R1 F3, R3
  F13, R2 F13; `internal/approve/impl.go`). Use §13.12, §13.13.
- [ ] OQ12: B8 has no outcome for round 1 after an implement `submit`; only `panel create` writes the round `ForBead`
  counts (R2 F17, R3 F16; `internal/panel/create.go`). Recommended: a `panel create` `run` entry (§13.14).
- [ ] OQ13: B9 gives a refused round a fix, R10 a REJECT a decision (R2 F18; contracts §3, §10). Recommended: both;
  the open decision holds the fix until `--answer fix` (§13.15).

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
