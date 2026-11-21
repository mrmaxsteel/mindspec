---
approved_at: "2026-11-19T06:51:51Z"
approved_by: operator (lane chat 2026-11-18)
status: Approved
---
# Spec 134-governed-autonomy-kernel: Governed Autonomy Kernel

*Code citations: `origin/main` at `371dc360` (v0.19.0), 2026-11-15; the named symbol is authoritative. Normative:
[contracts.md](contracts.md) (§1-§12 operator rulings; §13 readings the operator confirmed on 2026-11-18, but for
the applied readings §8 and §13.2 name; the operator rulings of 2026-11-21 that contracts.md §1 names).*

## Goal

Mindspec has one path: the binary decides the next step of a plan-approved spec, and an orchestrator executes it.
`plan approve` enrols every new spec; warnings become refusals. `next-action` prints the next steps; `--apply` makes
kernel beads (type `step`) just in time; `lease` and `submit` claim and close a step with a fenced token and a receipt.
The binary renders every brief from durable records. An independent verify step closes each finding of a refused round
as reproduced, refuted or deferred; only reproduced findings go to a fix. The inert `loop:` block goes. People and agents that do not use mindspec share the bd store with no conflict.

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

1. **Read-only reducer.** `next-action [<spec>] [--json]` writes nothing and prints `mindspec.next-action/v2` (§4).
   Failure: two skills pick the next bead by different rules (ADR-0045 Verification; #146, #187).
2. **Just-in-time steps.** `--apply` does one idempotent reduction under the phase lock; it resumes after a crash
   (B5); a cycle link is refused before enrolment, flagged after (B7, §8). Failure: a run stalls (`mindspec-qu9z`).
3. **Lease and submit.** `lease` (§5) refuses a held or blocked step, ancestor or upstream; `submit` (§6) checks
   evidence and writes the receipt before the close; it accepts a late submission of a revoked generation (§6).
   Each new lease generation records its declared actor (`--actor`, §5); a verifier is not a reviewer of its round or
   an author of its subject, and a fixer is not its verifier (§13.10). Failure: bd accepts `--claim` on a blocked item; subagents run
   `bd close`; `panel.one-fixer` and `lifecycle.complete-by-orchestrator` are advisory.
4. **Existing verbs are kernel steps.** The §7 verbs are the commands `run` lists; each closes the step it names and
   keeps its refusals. Failure: today `next`, `complete` and the skills each follow their own rules.
5. **Bead gate as child.** Enrolment creates `<w>.review` per work item and `<epic>.final`; a passed gate is frozen
   (B12); an agent step judges a later subject change that `panel rebind` does not accept, after a pass or at a tally:
   carry-over or a re-panel of 2 or more slots whose paths cover the diff (§13.2). A commit during a round does not end
   it (B10). Failure: nothing deters a raw work-item close (`mindspec-xaoq`).
6. **Enrol at plan approve.** `plan approve` enrols every new spec and pins its policy (§8). Nothing clears it; an
   enrolled spec always enforces the panel gate. Failure: `panel_presence` defaults to `warn`; `loop:` is editable.
7. **Round limit across panels.** A gate counts every registration of its work item or spec (§10, §13.1). Failure: a
   new slug restarts at round 1 (`mindspec-ztax`).
8. **Local decisions and cascades.** A REJECT that the verify reproduces adds a `fix`/`halt` decision for that gate
   only (§13.8); a refuted veto passes; `halt` stops the spec (B11, §13.4). A complete, refused round gets a verify;
   only its reproduced findings go to the fix, and no approval count resolves a finding. Failure: `Load` forces
   `on_reject: halt`; the `review.reproduce` rule is advisory.
9. **Unreceipted closes and audit.** §9 and §11: a close whose evidence still fits gets its receipt under `submit
   --take`, with no retry; `--allow-not-ready` joins `escapeHatchFlags` and
   `EscapeHatchTokens`. Failure: closes not persisted (`mindspec-2u0u`); `--allow-not-ready` is in no registry.
10. **Config honesty.** `Load` warns once per removed `loop:` key and ignores it (alias: §10); `config show` labels
    each key truthfully. Failure: inert keys look live; `runner:` is mislabelled (`mindspec-aili`).
11. **Shared bd store.** A person or agent that does not use mindspec works in the same store with bd and git only,
    with no mindspec command or setup, and nothing conflicts (§12). Failure: a `bd create --parent <epic>` child
    counts as lifecycle work (`classifyChild`); bd has no default view filter; the finalizer once merged another
    spec's bead (`77daf455`); risk: a concurrent bd write loses or doubles a step.
12. **Skills, docs and setup.** `ms-bead-cycle` loops `next-action --apply`, `lease`, dispatch, `submit` and each
    `run` entry; skills and docs drop deleted keys and spec-wide halt rows (§13.6); `setup` adds `step` and keeps
    other `types.custom` entries. Each brief comes from the binary (§13.11): `panel prompt` for a slot, `brief <step>`
    for a lens, verify, fix or change judge; a lens step supplies slot lenses. Failure: skills name default-3 rounds;
    `setup --force` replaces `types.custom`; the `panel create` entry leaves the BRIEF stub, so `panel prompt` refuses,
    and a fix brief carries no findings.

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
- [ ] AC2 (R2, R5, R6): `plan approve` creates exactly `<w>.review` per item, `<epic>.final`, `mindspec_links` and
  `mindspec_run`; a second run exits non-zero naming the enrolment, same snapshot. The first `--apply` creates exactly
  `<1>.impl.a1`; a second, or a rerun after a kill at its first write, gives the same snapshot and `revision`. The
  pass of `complete 1` creates exactly `<2>.impl.a1` and `<3>.impl.a1`. Fresh fixtures: (a) a fake bd refuses a
  plan-order `dep add` as a cycle: exit non-zero naming the link; no gate, `mindspec_run` or approval. Then a foreign
  `bd create --parent <epic>` and a normal-bd rerun: exit 0, no second work item, that child unchanged. (b) With a
  call-logging bd wrapper, killed after the creates, foreign links item 3 blocks a bug blocks item 1: the rerun exits
  non-zero naming a chain link and its bd removal; the log has no link write. (c) Killed after the `<epic>.final`
  create, foreign links `<epic>.final` blocks a bug blocks item 3: `--apply` exits 0 without the item-3-to-final link;
  `next-action` flags `foreign-blocker` naming the `<epic>.final`-blocks-bug link. After `bd dep remove` of that link,
  that flag is gone and `--apply` writes the item-3-to-final link. (d) Killed after the plan-order links, the plan
  edited to drop the item-2-on-1 link, or separately to add an item-3-on-2 link: the rerun exits non-zero naming it,
  same snapshot. (e) Killed after the plan-order links, a chunk 4 on item 1 added, the rerun killed again after its
  links: a further rerun exits 0 with exactly one item 4 and one item-4-on-1 link. Then chunk 4 edited to depend on
  item 2, or separately its link dropped: the next rerun exits non-zero naming it, same snapshot.
- [ ] AC3 (R3, R4): `next 1 --actor A` (no `--spec`) resolves spec `x` and prints a token (generation 1); `next 1`
  with no `--actor` exits non-zero; a second `next 1 --actor B` or `lease <1>.impl.a1 --actor B` exits non-zero naming
  `lease --take`; after the TTL, `lease` prints generation 2. A foreign bug
  blocking item 1: `lease <1>.review.r1.R1` exits non-zero naming it. `panel submit` without `--lease` exits
  non-zero; the slot stays open.
- [ ] AC4 (R3, B5, B13): each exits non-zero, step open: an old subject, another slot, a changed blob, a verdict to a fix, an implement commit not past the claim base. A valid `submit`
  writes `mindspec_close` = path, commit, blob id; a rerun exits 0 and another artifact exits non-zero, each with
  `bd show --json` unchanged. After `lease --take` and a generation-2 `submit`, a generation-1 `submit` of a different
  verdict exits non-zero, the step closed and its receipt unchanged; the same through `panel submit`. Killed between
  receipt and close: step open with its receipt; a rerun closes it.
- [ ] AC5 (R5, B12): `bd close <w>` exits non-zero (open child). After a pass, `panel create <w>` exits non-zero. With
  `r1.verify`, `r2.fix` or a limit decision open, `panel refute --actor` of the only blocking finding closes the gate `passed` and the verify,
  fix or decision `superseded`.
- [ ] AC6 (R6): `panel_presence: warn`; each kernel step closed by `bd close --force` and retyped `chore`; no
  `mindspec_run` or `panel.json`: `complete` with no passing panel exits non-zero (closed ids witness); `next-action`
  flags `run-marker-missing`. Spec `y` approved by v0.19.0: `next-action y` exits non-zero, same snapshot; `complete`
  with no panel warns, exits 0; a foreign `bd create --parent <y-epic>`, then `plan approve y`: no witness, that
  child unchanged and `next` does not select it.
- [ ] AC7 (R7): 3 rounds on slug A, `panel abandon` A, 2 on slug B: round 3 of B exits non-zero; `next-action` lists
  one decision (`continue:<n>`, `halt`) and `run` has no `panel create`; after `continue:1` one more round runs and
  the next exits non-zero. The same holds for the final gate.
- [ ] AC8 (R8): REJECT on item 2, `--apply`: `<2>.review.r1.verify` and no fix or decision; its brief lists the
  REJECT verdict's `veto` finding. A `deferred` return for the veto exits non-zero, step open. A verify that returns
  `reproduced` for the veto: a decision under item 2 (text: `halt` stops the spec), an action for item 3. Before the answer, `lease` on the fix and `panel create 2` exit non-zero; after `--answer fix`, `lease` exits 0.
  Fresh fixture, the veto `refuted` and no other finding reproduced: the gate closes `passed`, receipt `retally:
  "verify"`, no fix or decision; `complete 2` exits 0.
- [ ] AC9 (R8): a commit on `bead/2` in round 2: no slot closes `superseded` and no round is authorised; each slot's
  `submit` exits 0. A foreign child under `<2>.review`, a second REJECT on item 2, answer
  `halt`: other open steps are `halted`, `<2>.review` stays open flagged `foreign-child-open`; `next-action x` exits 4
  on each read; `lease`, `complete`, `next 2`, `panel create 2` exit non-zero, snapshot unchanged. In a new
  fixture, killed after the `halt` receipt: `next-action x --apply` closes the decision and open steps `halted`, exits 4.
- [ ] AC10 (R9): `bd close <slot>` with no verdict: flag `unreceipted-close`, and the next `--apply` creates
  `<slot>.a2`. With a valid verdict and token, `submit` writes the receipt; no `.a2`. A leased slot raw-closed with a
  fitting verdict at its registered path and no token: the pass flags `unreceipted-close`, creates no `.a2` and lists `submit <slot>
  --take --actor <actor> --reason`; that submit exits 0 with a receipt `taken: true` and one take event `via:
  "receipt"` with the step's current generation, and the lease generation does not change. `submit --take` exits
  non-zero, step unchanged, on an open step, on a step with a valid receipt and with a verdict of another subject; a
  leased, raw-closed verify with its record at `reviews/kernel/<step id>.json` gets a receipt from `submit --take`. A
  never-leased verify, raw-closed with a fitting record: `run` lists no `submit --take`, `submit --take` exits
  non-zero, and the pass creates `.verify.a2`. Raw-closed
  with no fitting evidence: a lens retry is the step `mindspec_round.lens` names, a fix retry keeps
  `mindspec_reproduced`, a verify retry is the step `mindspec_round.verify` names; after each retry's `submit`, the
  next step (`panel create`, the round, the gate pass) proceeds. After
  item 1's gate passes, `bd close <1>` flags `raw-close`: `impl approve` exits non-zero naming it, no `<2>.impl.a1`,
  then `complete 1` exits 0 and clears it. In a fresh fixture, `bd close --force <1>` flags `forced-close`: `complete 1`
  exits non-zero printing `bd reopen <1>`; after `bd reopen <1>` and a gate pass, `complete 1` exits 0 and `impl
  approve` does not name the flag.
- [ ] AC11 (R9): one each of `--allow-not-ready`, `--allow-doc-skew`, `--override-adr`, `--allow-net-deletion`, `lease
  --take`, `panel defer`, a decision answer, a repaired unreceipted close, a `carry-over`, a 3-slot `re-panel` after a
  passing round of at least 3 slots, a late submission on the step that `lease --take` took, a receipt under `submit
  --take`, a verify with one return of each type: `impl approve` and a rerun exit 0 and print takeovers 2 (the lease
  take and the receipt take), unreceipted closes 2 (the repaired close and the `submit --take` receipt), deferrals 2
  (the `panel defer` and the verify's `deferred` return), `lens-steps` equal to the fixture's gate rounds (one
  Details entry `<gate> r<N>: 1` each), and 1 in
  each other row (the re-panel row also 3 slots), `verify-reproduced`, `verify-refuted`, `verify-deferred`,
  `late-submissions` and `take-receipts` included;
  both registries hold `allow-not-ready`. `MINDSPEC_SKIP_PANEL=1 complete`, no passing gate: non-zero, names it.
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
  and `"status":"done"`. After `submit <1>.impl.a1`, `--apply` creates `<1>.review.r1.lens`
  and `run` has no `panel create 1`; after the lens `submit`, `run` lists `panel create 1`; after it, `--apply` lists slots
  `<1>.review.r1.R<s>`, `ForBead` counts 1 round, a rerun of `panel create 1` exits non-zero; the finalize export runs.
- [ ] AC18 (R5, B12): after item 1's gate passes on 3 slots, a `.mindspec/config.yaml` commit on `bead/1` (rebind
  refuses): `--apply` creates exactly `<1>.review.judge.a1` and no round; `complete 1` and `impl approve` exit
  non-zero naming it. A second commit before its submit: the next pass closes a1 `superseded` and creates
  `<1>.review.judge.a2` on the newest commit; `submit` to a1 exits non-zero. (a) `submit` to a2 of `carry-over` with a
  reason: the gate stays `passed`, its receipt names the old and new subject and the judge receipt. A commit of the
  judgement file: the next pass creates no judge. `complete 1` exits 0 and creates `<2>.impl.a1`; `impl approve` names
  neither the gate nor a judge; a pass after each of `complete 1` and `impl approve` creates no judge. (b) Fresh
  fixture, `re-panel` with 2 slots and paths: `<1>.review.a2` (`mindspec_from` set) gets one 2-slot round on the new
  subject, and the `run` entry for `panel create 1` names the paths; after its pass, `complete 1` exits 0. (c) Fresh
  fixture, judge open; each exits non-zero, step open, no round: 1 or 4 slots, a value other than `carry-over` or
  `re-panel`, `carry-over` with no reason, `re-panel` with no paths; then 3 slots exits 0 and authorises a 3-slot
  round. (d) Fresh fixture, an ordinary commit, judge open: a successful `panel rebind` closes the judge `superseded`
  in its pass, its receipt names the rebind; `complete 1` exits 0.

- [ ] AC19 (R8, B9, §13.10): round 1 of item 2 refused on two REQUEST_CHANGES findings: `--apply` creates
  `<2>.review.r1.verify`, no fix; its brief lists both findings. Fresh fixtures, all slots closed: a stray REJECT
  `R9-round-1.json` in the panel directory gives the verify with a `veto` finding for that file; a VOID verdict, an
  INVALID verdict, and (all other slots APPROVE) a panel-directory commit that edits one closed slot's verdict file
  before the last slot submits each give no verify and that slot's next attempt `<2>.review.r1.R<s>.a2`, whose APPROVE
  submit passes the gate; slots R2 and R5 VOID on `.a1`, `.a2` and `.a3` and R3 APPROVE on `.a1` give no slot attempt and one decision
  `<2>.review.decide.uncounted.r1` whose `slots` lists R2 and R5 with attempt 3; while it is open, a panel-directory
  commit edits R3's closed verdict file: no R3 attempt; `redispatch` creates `.a4` of R2 and R5 and no R3 attempt, and
  VOID on both `.a4` then gives `.a5` of R2 and R5 and R3 `.a2`, not a decision (mutations: a fourth attempt with no
  decision; `redispatch` serves one listed slot or R3; the answer leaves R2's count at 3). Fresh fixture, the same
  decision open, then a commit S: `--apply` creates no judge; after `redispatch` and APPROVE on both `.a4`, the pass
  creates `<2>.review.judge.a1` and does not close the gate (mutation: judge case (a) acts while the decision is open); a `hard_block`
  REQUEST_CHANGES verdict gives only its `veto` finding. Each exits non-zero, step open: a verify record that omits or
  repeats a finding, has no returns or has an unknown field, a `ccr[0]` return for the `hard_block` slot, a return
  other than the three, `refuted` with a `ground` outside its three values, no `reason` or evidence that resolves to
  nothing, `deferred` with no target or a target that fails `bead.CheckDeferral` (no deferral record written), a
  verify lease with the actor of a round-1 slot or of `<2>.impl.a1`, including that actor's first generation after a
  later expiry replacement, and `panel refute` with `--actor` of a round-1 slot or (with `<2>.review.r2.fix` leased by
  F) `--actor F`, naming the fix step. One `refuted`, one `reproduced`: a refutation record by the verify,
  `<2>.review.r2.fix` with `mindspec_reproduced` naming only the second, a fix lease with the verify's actor exits
  non-zero, and the fix brief names only the second. After the fix closes, `mindspec_round.prior.verify` names
  `<2>.review.r1.verify` and the round-2 slot brief shows both findings with their returns (mutation: `round-open`
  writes no `prior`, and the brief shows no returns). Fresh fixture, findings f and g: `panel refute --actor Y` of g while
  `<2>.review.r1.verify` is open, then the verify refutes f and reproduces g: the gate stays open and the fix's
  `mindspec_reproduced` names g (mutation: the filter keeps a verb refutation made before the verify closed). Fresh fixture, the fix created and leased by F, then a commit
  that edits a closed round-1 slot's verdict file and one that adds a stray `R9-round-1.json`: `--apply` creates no
  `.verify.a2` and no slot attempt, the fix keeps its `mindspec_reproduced`, after the fix closes `panel create 2`
  exits 0 (mutation: the changed-finding trigger acts on round 1), and the round-2 slot brief shows the round-1
  findings and returns that the verify record names and no text from the edit or from `R9-round-1.json` (mutation: the
  brief reads the verdict files at the branch head). No public event leases a verify after its fix exists, so the
  fixture seeds `.verify.a2`: its lease with `--actor F` exits non-zero naming the fix step, and another actor's lease
  exits 0. After round 2 is refused, a
  `<2>.review.r2.verify` lease with the actor of `<2>.review.r2.fix` exits non-zero.
  Fresh fixture, both `refuted`: the gate closes `passed`, receipt `retally: "verify"`, no fix. The same at
  `<epic>.final`. At the limit, a `reproduced` return: only the limit decision.
- [ ] AC20 (R5, B10, §13.2, §13.7): a commit on `bead/2` during round 1, then the last verdict: `--apply` creates
  `<2>.review.judge.a1` (old subject = the round subject) and the gate stays open. `carry-over`: a passing tally closes
  the gate `passed` on the new subject and `complete 2` exits 0. Fresh fixture, `re-panel` 2 whose paths omit one changed path: non-zero naming
  it; with all paths: round 2 of 2 slots on the gate, scoped to the paths. Fresh fixture, round refused, `carry-over`:
  the verify subject is the new subject; after a `reproduced` return, the fix `submit --commit J` (J = the new subject) exits non-zero, and a commit
  past J exits 0. Fresh fixture, verify open, a commit and a `carry-over`: the verify closes `superseded`,
  `<2>.review.r1.verify.a2` opens on the new subject, and a receipt of a1 does not pass the gate. Fresh fixture, a1
  refutes a finding and closes, then a commit and a `carry-over`: `mindspec_round.verify` names a2; a2 reproduces it:
  a1's refutation discharges nothing, the gate stays open and the fix is created. Fresh fixture, open gate, `re-panel`
  2 on J, a verify with no reproduced return authorises round 2 (record `prior` = round 1), then a commit S before
  `panel create`: the judge's old subject is J and its paths start at J; its `re-panel` withdraws round 2 (`prior`
  restored), retires the verify, opens one `.verify.a2` on S, and an all-`refuted` a2 authorises round 2 again on S, with
  the latest judgement's slot count and scope and a fresh lens attempt; the gate stays open until that round passes
  (mutation: the gate closes after a2). After a passed-gate `re-panel` and before `panel create`, an ordinary commit:
  the pass closes `<2>.review.a2` and its lens step `superseded` and creates a judge on the newest commit whose brief
  diff starts at the accepted subject; no full round is authorised; its `re-panel` creates `<2>.review.a3` and the lens
  step `.a2`, which `mindspec_round.lens` names. When the lens step of `<2>.review.a2` had closed `passed`, it keeps
  its receipt and is retired the same way; a `re-panel` with another slot count gives a lens attempt with that count's
  entries. No judge follows a closed judgement, on each loop path: after a
  passed-gate `re-panel`, the scoped round's authorisation, its passing tally and `complete 2` create no judge; on an
  open gate, a refused tally on a moved subject creates the judge and no verify until it closes; after its `re-panel`,
  the verify retally with no reproduced return, the scoped round 2's authorisation and its tally create no judge.
  With `<2>.review.r2.fix` open after an applied verify, a fix commit, then `panel refute --actor X` of the
  reproduced finding: no open-gate judge and no `.verify.a2`; the gate closes `passed` on round 1's review subject, the
  fix closes `superseded`, and the next pass opens `<2>.review.judge.a1` under case (b) (mutation: judge case (a) acts
  on a frozen round). With that fix open after a verify that reproduced f and g, a commit that edits R2's round-1
  verdict (g) to APPROVE, and in a fresh fixture a commit that adds an APPROVE `R9-round-1.json`, then `panel refute
  --actor X` of f: the gate and the fix stay open (mutation: a frozen retally with no coverage check passes on the
  stray file). Fresh fixture,
  open gate, round 1 refused on a moved subject, `panel refute` of its finding while the judge is open, `re-panel` 2:
  the round-2 slot body shows the round-1 finding with its refutation (mutation: the brief reads only `prior.verify`). A commit of only lens or verify records creates no judge, before and after the first
  registration.
- [ ] AC21 (R3, R9, §6, §9): generation 1 leased, `lease --take` gives generation 2; `submit` of generation 1 with a
  valid verdict exits 0: the receipt names generation 1, generation 2's lease record ends `superseded`, and a
  generation-2 `submit` of another verdict exits non-zero. The receipt names generation 1's actor. Fresh fixture, a
  lens step: generation 1 leased, `lease --take`, then a commit moves its authorised subject: the pass closes the lens
  step `superseded` and opens `.lens.a2` on the new subject; the generation-1 `submit` exits non-zero; a `submit` to
  `.a2` and then `panel create` exit 0.
- [ ] AC22 (R12, §13.11): the `panel create 1` entry has no `--brief-file`; after it, `panel prompt` of each slot exits
  0 and prints the managed header, the files in scope and its lens from its `focus` in `panel.json`; the adversary
  slot has no `focus` and prints its configured lens. A 3-slot scoped round on the default roster (adversary fourth)
  has roster R1, R2, R3 (R3 = configured slot 4, lens `adversarial`); its lens record names R1 and R2 only; `panel
  create`, the slot steps, each `submit`, the tally, `CheckPassedGate` and `complete` exit 0 on it (mutation: take the
  first n configured slots). With two adversary slots and n=2, the lens record has an empty `lenses` list. A round-2 slot prompt lists the paths changed since round 1's
  subject and the line about refuted or deferred findings. At `<epic>.final`, the prompt's summary is from the spec
  and its files in scope are the diff from the final base. Each
  lens `submit` exits non-zero, step open: a lens for the adversary slot, a missing slot, an empty lens. `panel create
  1` before the lens close exits non-zero naming the lens step; with `--brief-file`, non-zero. `brief` of the verify,
  fix and change judge prints the items §13.11 lists. On a pre-kernel spec, and with `--gate adhoc` on the enrolled
  spec, `panel create` with a filled `--brief-file` exits 0; `panel create kernel` exits non-zero.

## Validation Proofs
The scratch-repo tests for AC1-AC22 each pass and fail on a named mutation; `make test` exits 0.

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
- [x] OQ20: a rerun on an edited plan (B6, §8). Decided (operator, 2026-11-18): before the approval record a rerun
  refuses any plan change but missing items; the operator edits the plan only after removing the epic's children.

## Approval

- **Status**: APPROVED
- **Approved By**: operator (lane chat 2026-11-18)
- **Approval Date**: 2026-11-19
- **Notes**: Approved via mindspec approve spec