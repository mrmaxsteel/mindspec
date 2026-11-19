---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 134-governed-autonomy-kernel: Governed Autonomy Kernel

*Code citations: `origin/main` at `371dc360` (v0.19.0), 2026-11-15; the named symbol is authoritative. Normative:
[contracts.md](contracts.md) (§1-§12 operator rulings; §13 readings the operator confirmed on 2026-11-18, but for
the applied readings §13.2 names).*

## Goal

Mindspec has one path: the binary decides the next step of a plan-approved spec, and an orchestrator executes it.
`plan approve` enrols every new spec; warnings become refusals. `next-action` prints the next steps; `--apply` makes
kernel beads (type `step`) just in time; `lease` and `submit` claim and close a step with a fenced token and a receipt.
The inert `loop:` block goes. People and agents that do not use mindspec share the bd store with no conflict.

## Background

At `371dc360` no `next-action`, `lease` or `submit` exists. The `Loop` comment (`internal/config/config.go`) calls every
`loop:` key INERT. `panel.one-fixer` and `lifecycle.complete-by-orchestrator` (`internal/rules/rules.yaml`) are
advisory; `panel_presence` defaults to `warn`. `classifyChild` (`internal/phase/derive.go`) counts every `task` or
untyped child as lifecycle work; `bd create` defaults to `task`. `EnsureBeadsConfig` (`internal/bead/config.go`)
replaces a differing `types.custom` under `--force`. Already delivered: `77daf455`, `bf41b40f`, panels, final gates.

## Impacted Domains

- **workflow**: `cmd/**` (new `next-action`, `lease`, `submit`; changed lifecycle and panel verbs),
  `internal/{panel,complete,approve,next,lifecycle,doctor,setup,rules,bead}/**`, `.claude/skills/**`,
  `plugins/mindspec/**`, `README.md`, `project-docs/user/guides/autonomy.md`, `.mindspec/domains/workflow/*.md`.
- **core**: `internal/{config,phase,redact}/**` (delete `loop:`; add `panel.max_rounds`, `panel.lease_ttl`),
  `.mindspec/domains/core/interfaces.md`. **execution**: `internal/executor/**` (finalize allow-set, read only).

## ADR Touchpoints

[ADR-0045](../../adr/ADR-0045-binary-owns-sequence-and-rules.md); [ADR-0023](../../adr/ADR-0023.md) (`step`);
[ADR-0040](../../adr/ADR-0040-orchestration-layering-ratchet.md); [ADR-0038](../../adr/ADR-0038-friction-reporter.md);
[ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md); [ADR-0035](../../adr/ADR-0035-agent-error-contract.md);
[ADR-0043](../../adr/ADR-0043-panel-disposition-telemetry-store.md). Amends ADR-0035 HC-4: `next-action` exits by §4.

## Requirements

Every requirement holds contracts §2 and B1-B13 (§3). A bare `§<n>` cites contracts.md.

1. **Read-only reducer.** `next-action [<spec>] [--json]` writes nothing and prints `mindspec.next-action/v1` (§4).
   Failure: two skills pick the next bead by different rules (ADR-0045 Verification; #146, #187).
2. **Just-in-time steps.** `--apply` does one idempotent reduction under the phase lock; it resumes after a crash
   (B5); a cycle link is refused before enrolment, flagged after (B7, §8). Failure: a run stalls (`mindspec-qu9z`).
3. **Lease and submit.** `lease` (§5) refuses a held or blocked step, ancestor or upstream; `submit` (§6) checks
   evidence and writes the receipt before the close. Failure: bd accepts `--claim` on a blocked item; subagents run
   `bd close`; `panel.one-fixer` and `lifecycle.complete-by-orchestrator` are advisory.
4. **Existing verbs are kernel steps.** The §7 verbs are the commands `run` lists; each closes the step it names and
   keeps its refusals. Failure: today `next`, `complete` and the skills each follow their own rules.
5. **Bead gate as child.** Enrolment creates `<w>.review` per work item and `<epic>.final`; a passed gate is frozen
   (B12); an agent step judges a later subject change that `panel rebind` does not accept: carry-over or a re-panel of 2
   or more slots (§13.2). Failure: nothing deters a raw work-item close (`mindspec-xaoq`).
6. **Enrol at plan approve.** `plan approve` enrols every new spec and pins its policy (§8). Nothing clears it; an
   enrolled spec always enforces the panel gate. Failure: `panel_presence` defaults to `warn`; `loop:` is editable.
7. **Round limit across panels.** A gate counts every registration of its work item or spec (§10, §13.1). Failure: a
   new slug restarts at round 1 (`mindspec-ztax`).
8. **Local decisions and cascades.** A REJECT adds a `fix`/`halt` decision for that gate only (§13.8); `halt` stops
   the spec (B11, §13.4); B10 supersedes (§13.7). Failure: `Load` forces `on_reject: halt`.
9. **Unreceipted closes and audit.** §9 and §11; `--allow-not-ready` joins `escapeHatchFlags` and
   `EscapeHatchTokens`. Failure: closes not persisted (`mindspec-2u0u`); `--allow-not-ready` is in no registry.
10. **Config honesty.** `Load` warns once per removed `loop:` key and ignores it (alias: §10); `config show` labels
    each key truthfully. Failure: inert keys look live; `runner:` is mislabelled (`mindspec-aili`).
11. **Shared bd store.** A person or agent that does not use mindspec works in the same store with bd and git only,
    with no mindspec command or setup, and nothing conflicts (§12). Failure: a `bd create --parent <epic>` child
    counts as lifecycle work (`classifyChild`); bd has no default view filter; the finalizer once merged another
    spec's bead (`77daf455`); risk: a concurrent bd write loses or doubles a step.
12. **Skills, docs and setup.** `ms-bead-cycle` loops `next-action --apply`, `lease`, dispatch, `submit` and each
    `run` entry; skills and docs drop deleted keys and spec-wide halt rows (§13.6); `setup` adds `step` and keeps
    other `types.custom` entries. Failure: skills name default-3 rounds; `setup --force` replaces `types.custom`.

**Constraints.** Consumer-facing text (CLI output, refusals, recovery lines, `next-action` output, skills, setup
files) cites no ADR, spec, requirement, bead or issue number. No new use of the committed `.beads/issues.jsonl`.

## Migration

A spec past `plan approve` at release has no witness: it is pre-kernel and keeps today's rules; a `plan approve` rerun
on it does not enrol and selects by recorded `bead_ids`, not `--parent`. Without `step`, `plan approve` refuses (§2).

## Scope

### In Scope
- The plan-approved tree of an enrolled spec; the shared store. After a check: close `mindspec-4xzi`, `mindspec-naq0`,
  and `mindspec-sxjc` if copies match; re-scope `mindspec-qu9z` ("an expired lease frees the step").
### Out of Scope
- Spec-draft and plan-draft steps; the merge as a bead (#250). Authority table, grants DSL, blocking caps on escape
  hatches, per-wake ceilings, `token_budget`, lane kit ledger.
- Rejected: a hand mode or opt-in inside mindspec; enrol on read; bd's lease; unfenced leases; ownership by type;
  version stamp; auto-reopen; round beads; per-directory round counts; `--ephemeral` steps.
- Later: export removal (`mindspec-lzx4`); document steps; parallel waves (`mindspec-m4ks`); a driver; skill merge;
  plan amendment of a spec past `plan approve` (follow-up spec, `mindspec-3v6n`).

## Acceptance Criteria

Scratch repo and bd database; spec `x` plan-approved by this binary; items 2 and 3 depend on item 1. "Snapshot" = `bd
list --all --json`, every dependency list, `git for-each-ref`, `git status --porcelain`. "Foreign" = by `bd` only. "X
blocks Y" = Y waits for X.

- [ ] AC1 (R1): `next-action x --json` twice: same bytes and snapshot; one action, item 1 `implement`, `"exists":false`.
- [ ] AC2 (R2, R5, R6): `plan approve` creates exactly `<w>.review` per item, `<epic>.final` and `mindspec_run`; a
  second run exits non-zero naming the enrolment, same snapshot. The first `--apply` creates exactly `<1>.impl.a1`; a
  second, or a rerun after a kill at its first write, gives the same snapshot and `revision`. The pass of `complete 1`
  creates exactly `<2>.impl.a1` and `<3>.impl.a1`. Fresh fixtures: (a) a fake bd refuses a plan-order `dep add` as a
  cycle: exit non-zero naming the link; no gate, `mindspec_run` or approval. Then a foreign `bd create --parent
  <epic>` and a normal-bd rerun: exit 0, no second work item, that child unchanged. (b) With a call-logging bd
  wrapper, killed after the creates, foreign links item 3 blocks a bug blocks item 1: the rerun exits non-zero naming
  a chain link and its bd removal; the log has no link write. (c) Killed after the `<epic>.final` create, foreign links
  `<epic>.final` blocks a bug blocks item 3: `--apply` exits 0 without the item-3-to-final link; `next-action` flags
  `foreign-blocker` naming the `<epic>.final`-blocks-bug link. After `bd dep remove` of that link, that flag is gone
  and `--apply` writes the item-3-to-final link.
- [ ] AC3 (R3, R4): `next 1` (no `--spec`) resolves spec `x` and prints a token (generation 1); a second `next 1` or
  `lease <1>.impl.a1` exits non-zero naming `lease --take`; after the TTL, `lease` prints generation 2. A foreign bug
  blocking item 1: `lease <1>.review.r1.R1` exits non-zero naming it. `panel submit` without `--lease` exits
  non-zero; the slot stays open.
- [ ] AC4 (R3, B5, B13): each exits non-zero, step open: `submit` with generation 1 after `--take`, an old subject,
  another slot, a changed blob, a verdict to a fix, an implement commit not past the claim base. A valid `submit`
  writes `mindspec_close` = path, commit, blob id; a rerun exits 0 and another artifact exits non-zero, each with
  `bd show --json` unchanged. Killed between receipt and close: step open with its receipt; a rerun closes it.
- [ ] AC5 (R5, B12): `bd close <w>` exits non-zero (open child). After a pass, `panel create <w>` exits non-zero. With
  `r2.fix` or a limit decision open, `panel refute` of the only blocking finding closes the gate `passed` and the fix
  or decision `superseded`.
- [ ] AC6 (R6): `panel_presence: warn`; each kernel step closed by `bd close --force` and retyped `chore`; no
  `mindspec_run` or `panel.json`: `complete` with no passing panel exits non-zero (closed ids witness); `next-action`
  flags `run-marker-missing`. Spec `y` approved by v0.19.0: `next-action y` exits non-zero, same snapshot; `complete`
  with no panel warns, exits 0; a foreign `bd create --parent <y-epic>`, then `plan approve y`: no witness, that
  child unchanged and `next` does not select it.
- [ ] AC7 (R7): 3 rounds on slug A, `panel abandon` A, 2 on slug B: round 3 of B exits non-zero; `next-action` lists
  one decision (`continue:<n>`, `halt`) and `run` has no `panel create`; after `continue:1` one more round runs and
  the next exits non-zero. The same holds for the final gate.
- [ ] AC8 (R8): REJECT on item 2, `--apply`: a decision under item 2 (text: `halt` stops the spec), an action for item
  3. Before the answer, `lease` on the fix and `panel create 2` exit non-zero; after `--answer fix`, `lease` exits 0.
- [ ] AC9 (R8): a commit on `bead/2` in round 2, no fix open: one pass marks round-2 slots `superseded` and lists
  `panel create 2`; in an open fix, none. A foreign child under `<2>.review`, a second REJECT on item 2, answer
  `halt`: other open steps are `halted`, `<2>.review` stays open flagged `foreign-child-open`; `next-action x` exits 4
  on each read; `lease`, `complete`, `next 2`, `panel create 2` exit non-zero, snapshot unchanged. In a new
  fixture, killed after the `halt` receipt: `next-action x --apply` closes the decision and open steps `halted`, exits 4.
- [ ] AC10 (R9): `bd close <slot>` with no verdict, or a verdict with an old token: flag `unreceipted-close`, and
  the next `--apply` creates `<slot>.a2`. With a valid verdict and token, `submit` writes the receipt; no `.a2`. After
  item 1's gate passes, `bd close <1>` flags `raw-close`: `impl approve` exits non-zero naming it, no `<2>.impl.a1`,
  then `complete 1` exits 0 and clears it. In a fresh fixture, `bd close --force <1>` flags `forced-close`: `complete 1`
  exits non-zero printing `bd reopen <1>`; after `bd reopen <1>` and a gate pass, `complete 1` exits 0 and `impl
  approve` does not name the flag.
- [ ] AC11 (R9): one each of `--allow-not-ready`, `--allow-doc-skew`, `--override-adr`, `--allow-net-deletion`, `lease
  --take`, `panel defer`, a decision answer, a repaired unreceipted close, a `carry-over`, a 2-slot `re-panel`: `impl
  approve` and a rerun exit 0, print 1 each (the re-panel row also 2 slots); both registries hold `allow-not-ready`.
  `MINDSPEC_SKIP_PANEL=1 complete`, no passing gate: non-zero, names it.
- [ ] AC12 (R10): `loop.gate_authority`, `loop.halt.{on_reject: fix, panel_deadlock_rounds,
  max_consecutive_impl_failures}` load with exit 0, one warning each; `config show` labels them ignored and `runner:`
  read by a skill. `loop.halt.max_rounds_per_bead: 4` gives limit 4; with `panel.max_rounds: 6`, 6. On a spec enrolled
  with the defaults, set `panel.max_rounds: 99` and `panel.lease_ttl: 10m`: round 6 exits non-zero; a new lease runs 2h.
- [ ] AC13 (R11): foreign `bd create --parent <epic> note`, a bug blocking item 2, `bd update <1>.impl.a1 --claim`. A
  snapshot diff around each kernel verb changes only computed ids and links between them. `next-action` lists no
  action for the note and flags `raw-claim` and `foreign-child-open`; `lease <1>.impl.a1` exits 0. `bd ready` titles
  each mindspec bead `[mindspec]`; `bd ready --exclude-label mindspec` shows the foreign beads and no epic, work item
  or kernel bead. A foreign note under the epic at the end: `impl approve` exits non-zero naming it, writes nothing.
- [ ] AC14 (R11): a foreign `bd create --id <1>.review.r1.R1`: `panel create 1` exits non-zero printing
  `id-collision`; that bead is unchanged. A foreign `bd create --parent <1>.review` gives a foreign `<1>.review.1`:
  after a passing tally the gate stays open and `next-action` exits 3 flagging `foreign-child-open`; after its close
  the gate closes `passed`. A fake bd answers exit 13 to the first `--if-status` write: `submit` exits 0 with one
  receipt and one close; always 13: non-zero, the step open with at most its receipt.
- [ ] AC15 (R12): with K = `max_rounds_per_bead|panel_deadlock_rounds|max_consecutive_impl_failures|gate_authority`,
  `git grep -n -i -E "$K|Any bead REJECT|REJECT / HARD block|do not proceed to the next bead|on_reject|loop\.halt|
  max-rounds|One halted bead|keep going after a halt|default .?3" -- .claude/skills plugins/mindspec/skills
  plugins/mindspec/README.md README.md project-docs/user/guides` and `git grep -n -E "$K|on_reject|type Loop struct"
  -- .mindspec/domains` each exit 1 (`historical_skills` keeps old text). Both `ms-bead-cycle` copies name
  `next-action --apply`, `lease`, `submit`; `cmp` of each skill pair exits 0. With `types.custom: gate,spike2`, `plan
  approve` exits non-zero naming `mindspec setup`; after plain `mindspec setup` (an old install), it holds
  `gate,spike2,step` and each installed skill copy matches its source.
- [ ] AC16 (constraints): added non-test Go lines under `cmd/`, `internal/` and skill lines hold no ADR, spec,
  requirement, bead or issue number and do not touch `.beads/issues.jsonl`.
- [ ] AC17 (R1-R4, B8): only `next-action --apply`, `lease`, `submit` and each `run` entry reach `impl approve` exit 0
  and `"status":"done"`. After `submit <1>.impl.a1`, `run` lists `panel create 1`; after it, `--apply` lists slots
  `<1>.review.r1.R<s>`, `ForBead` counts 1 round, a rerun of `panel create 1` exits non-zero; the finalize export runs.
- [ ] AC18 (R5, B12): after item 1's gate passes on 3 slots, a `.mindspec/config.yaml` commit on `bead/1` (rebind
  refuses): `--apply` creates exactly `<1>.review.judge.a1` and no round; `complete 1` and `impl approve` exit
  non-zero naming it. A second commit before its submit: the next pass closes a1 `superseded` and creates
  `<1>.review.judge.a2` on the newest commit; `submit` to a1 exits non-zero. (a) `submit` to a2 of `carry-over` with a
  reason: the gate stays `passed`, its receipt names the old and new subject and the judge receipt; `complete 1` exits
  0 and creates `<2>.impl.a1`; `impl approve` names neither the gate nor a judge. (b) Fresh fixture, `re-panel` with
  2 slots: `<1>.review.a2` (`mindspec_from` set) gets one 2-slot round on the new subject; after its pass, `complete
  1` exits 0. (c) Each exits non-zero, step open, no round: 1 or 4 slots, a value other than `carry-over` or
  `re-panel`, `carry-over` with no reason, `re-panel` with no paths; then 3 slots exits 0 and authorises a 3-slot
  round. (d) Fresh fixture, an ordinary commit, judge open: a successful `panel rebind` and its pass close the judge
  `superseded`, its receipt names the rebind; `complete 1` exits 0.

## Validation Proofs
The scratch-repo tests for AC1-AC18 each pass and fail on a named mutation; `make test` exits 0.

## Open Questions

- [x] grill waived by the operator (2026-11-18)
- [x] OQ1: Does `bd create --id` take word segments? Decided (operator, 2026-11-18): the plan settles it; its first
  bead probes `bd create --id`; fallback `mindspec_key` (§12).
- [x] OQ2: every step has an open ancestor or upstream. Decided (operator, 2026-11-18): lease refuses held or
  blocked (§5).
- [x] OQ4, OQ6: final-gate fix id; halted spec; halt retry exit. Decided (operator, 2026-11-18): §13.1, §13.4.
- [x] OQ5: a passed gate is frozen; `panel rebind` refuses a config change. Decided (operator, 2026-11-18): a moved
  subject never by itself forces a new panel; an agent step picks carry-over or a smaller re-panel (§13.2).
- [x] OQ8: exits 3/4/5 conflict with ADR-0035 HC-4. Decided (operator, 2026-11-18): amend HC-4 (ADR Touchpoints).
- [x] OQ9, OQ11: no implement outcome; refusal sets; halt rows. Decided (operator, 2026-11-18): §13.3, §13.5-6.
- [x] OQ10, OQ12: no writer of the next round. Decided (operator, 2026-11-18): a `run` entry (§13.7, §13.9).
- [x] OQ13: a refused round: a fix; a REJECT: a decision. Decided (operator, 2026-11-18): both; it holds (§13.8).
- [x] OQ14: bd has no default view filter. Decided (operator, 2026-11-18): label and title mindspec beads; `bd ready
  --exclude-label mindspec` (§12).
- [x] OQ15-OQ18: enrol every new spec (§8); verbs stay as `run` commands (§7); a spec past `plan approve` at release
  keeps today's rules; an enrolled spec enforces the gate. Decided (operator, 2026-11-18): yes.
- [x] OQ19: an enrolled plan cannot change (§8). Decided (operator, 2026-11-18): accepted gap (Out of Scope).
- [x] OQ20: a rerun on an edited plan (B6, §8). Decided (operator, 2026-11-18): a rerun keeps recorded ids and adds
  only new chunks and links (§8); the operator edits the plan only after removing the epic's children.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
