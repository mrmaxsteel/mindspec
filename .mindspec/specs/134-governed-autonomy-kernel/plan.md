---
status: Draft
spec_id: 134-governed-autonomy-kernel
version: "1"
adr_citations: [ADR-0045, ADR-0023, ADR-0040, ADR-0038, ADR-0037, ADR-0035, ADR-0043]
work_chunks:
  - {id: 1, depends_on: [], key_file_paths: [internal/next/kernel/ids.go, internal/next/kernel/idmode.go,
      internal/next/kernel/store.go, internal/next/kernel/witness.go, internal/next/kernel/kerneltest/fixture.go,
      internal/phase/phaselock.go, internal/bead/bdcli.go, internal/doctor/beads.go]}
  - {id: 2, depends_on: [], key_file_paths: [internal/config/config.go, cmd/mindspec/config.go,
      internal/redact/redact.go, cmd/mindspec/selfemit.go]}
  - {id: 3, depends_on: [1, 2], key_file_paths: [internal/approve/plan.go, internal/approve/enrol.go,
      internal/phase/derive.go, internal/bead/planbead.go]}
  - {id: 4, depends_on: [1], key_file_paths: [internal/next/kernel/reducer.go, internal/next/kernel/state.go,
      internal/next/kernel/output.go, internal/next/kernel/flags.go, cmd/mindspec/next_action.go,
      internal/panel/tally.go]}
  - {id: 5, depends_on: [4], key_file_paths: [internal/next/kernel/lease.go, internal/next/kernel/submit.go,
      cmd/mindspec/lease.go, cmd/mindspec/submit.go, internal/rules/rules.yaml]}
  - {id: 6, depends_on: [4], key_file_paths: [internal/next/kernel/reject.go, internal/next/kernel/halt.go,
      internal/next/kernel/rounds.go, internal/panel/panel.go]}
  - {id: 7, depends_on: [2, 4], key_file_paths: [internal/next/kernel/closes.go, internal/next/kernel/audit.go,
      internal/approve/impl.go, internal/approve/impl_final_gate.go]}
  - {id: 8, depends_on: [5, 6], key_file_paths: [cmd/mindspec/panel.go, cmd/mindspec/panel_submit.go,
      cmd/mindspec/panel_refute.go, cmd/mindspec/panel_defer.go, internal/panel/create.go, internal/panel/submit.go]}
  - {id: 9, depends_on: [3, 7, 8, 10], key_file_paths: [cmd/mindspec/next.go, internal/next/select.go,
      internal/complete/complete.go, internal/complete/panel_presence.go, cmd/mindspec/kernel_e2e_test.go]}
  - {id: 10, depends_on: [5], key_file_paths: [internal/next/kernel/judge.go, cmd/mindspec/panel_rebind.go]}
  - {id: 11, depends_on: [], key_file_paths: [.claude/skills/ms-bead-cycle/SKILL.md,
      plugins/mindspec/skills/ms-bead-cycle/SKILL.md, .claude/skills/ms-spec-autopilot/SKILL.md,
      plugins/mindspec/skills/ms-spec-autopilot/SKILL.md, internal/bead/config.go, internal/setup/skills.go,
      .mindspec/adr/ADR-0035-agent-error-contract.md, project-docs/user/guides/autonomy.md]}
---
# Plan: 134-governed-autonomy-kernel

## ADR Fitness

Each touchpoint ADR stays sound; none needs a superseding ADR.

- **ADR-0045, ADR-0040.** Applied. Bead selection, halt rows and the round limit move from skills into the binary
  (Beads 4, 6, 11); `panel.one-fixer` and `lifecycle.complete-by-orchestrator` stop being advisory (Bead 5).
- **ADR-0023.** Applied. The `step` type, tuple metadata and receipts stay in bd; evidence stays in git (Beads 1, 3).
- **ADR-0038.** Applied. `--allow-not-ready` joins `escapeHatchFlags` and `EscapeHatchTokens` (Bead 2).
- **ADR-0037.** Applied. The gate legs stay; `panel submit` checks a lease; `panel create` is the one round writer
  (Bead 8).
- **ADR-0043.** Applied. Round counts come from the ledger over every registration (Bead 6). Receipts are bd metadata.
- **ADR-0035.** Amended. HC-4 gains the `next-action` exit codes 3, 4 and 5 (Bead 11, with its pin test). Each new
  refusal keeps one `recovery:` line.

## Decomposition

Eleven beads, because the spec adds four verbs, changes six and rewrites skills; one bead is one session. The chain
1 -> 4 -> 5 -> 8 -> 9 is five deep: the reducer needs the store, `submit` runs a reducer pass, the panel verbs call
`submit`, and the end-to-end bead needs every verb. Width stays at three: waves {1, 2, 11}, {3, 4}, {5, 6, 7},
{8, 10}, {9}. Kernel code goes in `internal/next/kernel`, which the workflow manifest already owns.

## Testing Strategy

- **Test names.** Each new test starts with `TestS134<Mechanism>_`; no base-tree test matches `S134`. Each
  Verification line with a `TestS134` prefix must list its new tests in `go test -v` output.
- **Real bd.** Scenario tests run real bd 1.3.4 in a scratch repo through `harness.NewSandbox`
  (`internal/harness/sandbox.go`). They fail, not skip, when bd is absent, as `cmd/mindspec/next_ready_gate_test.go`.
- **Fake bd.** Fault tests put a script named `bd` first on `PATH`, as `nextHonestyFakeBD`
  (`cmd/mindspec/next_selection_test.go`) and `installFakeBD` (`cmd/mindspec/verb_output_test.go`). A forwarding
  wrapper logs argv, calls real bd and injects a cycle refusal, exit 13 or `kill -9 $PPID` on a named write. Unit tests
  use seams such as `phase.SetRunBDForTest` and `approve.SetPlanRunBDForTest`. No test starts a model CLI.
- **Shared fixture.** Bead 1 adds `kerneltest`: an enrolled spec (items 2 and 3 on item 1) built through the store,
  and `Snapshot` (the spec's snapshot, compared byte for byte). Beads 4-8 and 10 use it; Bead 9 uses `plan approve`.
- **Surface rule (AC16).** "Internal record number" = an ADR, spec, requirement, section, bead or issue number. No
  added non-test Go line (comments too), CLI text or skill line holds one.
- **Dogfood boundary.** The lane runs lifecycle and panel verbs on the released binary. Each bead that changes
  `internal/**` updates a domain doc, because the released `complete` refuses code with no domain-doc change.
- **Gate.** Each bead runs its named tests and the tests of the files it touches. The final review runs `make test`.

## Bead 1: Kernel foundation: bd probe, ids and store

**Steps**
1. Probe bd (OQ1). In a sandbox with `types.custom: step`, run `bd create --id <epic>.1.impl.a1 --parent <epic>.1 --type
   step`, and the same for `.review`, `.final`, `.decide.` and `.judge.` shapes. Record the result as the constant
   `kernel.NativeIDs` in `internal/next/kernel/idmode.go`, with the bd version and the observed output in its doc
   comment. `TestS134Probe_` runs the probe on real bd and fails when the result differs from the constant.
2. Add the id grammar in `ids.go`: format and parse each §2 kind, the attempt suffix `.a<k>`, and the exact-match test
   for "kernel-shaped". A bd-numbered child such as `<w>.review.1` is not kernel-shaped.
3. Add `store.go` over `bead.RunBD`. `Create` takes a computed id, type, parent, tuple metadata, label `mindspec` and a
   title that starts `[mindspec]`. Branch on `kernel.NativeIDs`: if true, the bead id is the computed id; if false, bd
   assigns the id and metadata `mindspec_key` holds the computed id, and `Lookup` resolves a key by a scan of the epic
   subtree. Later beads call only `Lookup` and `Create`, never a raw id.
4. Add `SetStatusIf` (bd `--if-status` compare-and-set; exit 13 is `ErrCASMismatch`), `WriteReceipt` (`mindspec_close`,
   before the close), `AddLink` (blocks between computed ids only) and `Collision` (a kernel-shaped bead with no
   matching tuple gives `id-collision`).
5. Add `witness.go` (§8: `mindspec_run`, a computed kernel id under the epic, or `panel.json` with `"orchestrated":
   true`), the `RunMarker` struct and `DefaultPolicy` (round limit 5, lease TTL 2h). Export `phase.WithPhaseLock(root,
   fn)` over the existing `acquirePhaseLock`.
6. `doctor` reports a missing `step` in `types.custom`. Add `kernel.RequireStepType(root)`; its refusal names `mindspec
   setup`. Add `kerneltest` (fixture, snapshot, forwarding fake bd).
7. Update `.mindspec/domains/{workflow,core}/interfaces.md`. Text cites no internal record number (AC16).

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Probe|IDs|Store|Witness)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/doctor/... -run '^TestS134Doctor_' -v` passes and lists the new tests.
- [ ] `go test ./internal/phase/... -run 'PhaseLock|^TestS134PhaseLock_'` passes.
- [ ] `go build ./...` exits 0.

**Acceptance Criteria**
- [ ] OQ1: the probe result is a committed constant; `TestS134Probe_` goes red if bd differs. Fallback-branch tests
      force `NativeIDs` false through a seam.
- [ ] The grammar accepts every §2 shape with and without `.a<k>` and rejects `<w>.review.1`, `<prefix>-<hash>` and
      `<epic>.<n>` (AC14 primitive).
- [ ] AC14 primitives: `Collision` refuses a foreign `bd create --id <1>.review.r1.R1` bead and leaves it unchanged;
      `SetStatusIf` returns `ErrCASMismatch` on bd exit 13 and writes nothing else.
- [ ] Every `Create` sets label `mindspec` and the `[mindspec]` title prefix (AC13 primitive).
- [ ] Witness detection finds each of the three witnesses alone; with none it reports pre-kernel (AC6 primitive).

**Depends on**
None

## Bead 2: Config honesty and escape-hatch registries

**Steps**
1. Delete `type Loop`, `LoopContext` and every `loop:` default in `internal/config/config.go`. `Load` keeps a non-strict
   decode of the removed keys and warns once per removed key that is set.
2. Add `panel.max_rounds` (default 5) and `panel.lease_ttl` (default `2h`). Add `config.RoundLimit()`: it reads
   `panel.max_rounds`; when that is unset it reads `loop.halt.max_rounds_per_bead` and the warning names the value.
3. Rewrite the `config show` render in `cmd/mindspec/config.go`. Each key has one truthful label. Removed keys show as
   ignored. `runner:` shows as read by a skill.
4. Add `allow-not-ready` to `escapeHatchFlags` (`cmd/mindspec/selfemit.go`) and to `redact.EscapeHatchTokens`.
5. Update `.mindspec/domains/core/interfaces.md`. Remove `type Loop struct` and the removed key names from
   `.mindspec/domains/**`. Surface text cites no internal record number (AC16).

**Verification**
- [ ] `go test ./internal/config/... -run '^TestS134Config_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134ConfigShow_|^TestS134Friction_' -v` passes and lists the new tests.
- [ ] `go test ./internal/redact/... ./cmd/mindspec/... -run 'EscapeHatch|RedactEnum|Friction'` passes.
- [ ] `git grep -n -E "max_rounds_per_bead|panel_deadlock_rounds|max_consecutive_impl_failures|gate_authority|on_reject|type Loop struct" -- .mindspec/domains` exits 1.

**Acceptance Criteria**
- [ ] AC12 (config part): `loop.gate_authority`, `loop.halt.on_reject: fix`, `loop.halt.panel_deadlock_rounds` and
      `loop.halt.max_consecutive_impl_failures` load with exit 0 and one warning each. `config show` labels them ignored
      and labels `runner:` read by a skill.
- [ ] AC12 (alias): `loop.halt.max_rounds_per_bead: 4` gives `RoundLimit()` 4; with `panel.max_rounds: 6` it gives 6.
- [ ] AC11 (registries): both registries hold `allow-not-ready`; a journal event with that token is kept.
- [ ] AC15 (domains): the `.mindspec/domains` `git grep` exits 1. A loader that forces `on_reject: halt` fails a test.

**Depends on**
None

## Bead 3: Enrolment at plan approve

**Steps**
1. Add `internal/approve/enrol.go`. Under `phase.WithPhaseLock`, write in §8 order: each work item (record its id in
   `bead_ids` right after its create), `mindspec_links`, the B7 cycle check, the plan-order links, `mindspec_run` (round
   limit and lease TTL from `config.RoundLimit()` and `panel.lease_ttl`, rules `kernel-1`), the approval record, the
   gates `<w>.review` and `<epic>.final` and each work-item link to `<epic>.final`.
2. Implement the rerun rules of §8 and OQ20. Reuse recorded ids, create only missing chunks, refuse every other plan
   change with its name and no write. Refuse a rerun after the approval record and name the enrolment. Never select by
   `--parent`. Before `mindspec_run`, a cycle refusal names the link, the foreign link that closes it and its bd
   removal; after it, a refused link is left for `--apply`.
3. Refuse without `step` (`kernel.RequireStepType`). Add label `mindspec` and the `[mindspec]` prefix to the epic and
   each work item. Make `phase.ParseSpecFromTitle` skip the tag.
4. Replace the type-based `classifyChild` count in `internal/phase/derive.go`: for an enrolled spec only recorded work
   items and kernel ids count; a foreign `task` under the epic does not. A pre-kernel spec keeps today's rules, and a
   `plan approve` rerun on it does not enrol and selects by recorded `bead_ids`.
5. Update `.mindspec/domains/workflow/architecture.md` (enrolment) and `.mindspec/domains/core/interfaces.md` (phase
   derivation). Refusals cite no internal record number (AC16).

**Verification**
- [ ] `go test ./internal/approve/... -run '^TestS134Enrol_' -v` passes and lists the new tests.
- [ ] `go test ./internal/phase/... -run '^TestS134Derive_' -v` passes and lists the new tests.
- [ ] `go test ./internal/approve/... ./internal/phase/... -run 'ApprovePlan|Derive|ParseSpecFromTitle'` passes.

**Acceptance Criteria**
- [ ] AC2 (enrolment): `plan approve` creates exactly `<w>.review` per item, `<epic>.final`, `mindspec_links` and
      `mindspec_run`. A second run exits non-zero, names the enrolment and leaves the snapshot unchanged.
- [ ] AC2 (a): a fake bd refuses a plan-order `dep add` as a cycle: exit non-zero naming the link; no gate,
      `mindspec_run` or approval. A foreign `bd create --parent <epic>`, then a normal rerun: exit 0, no second work
      item, that child unchanged.
- [ ] AC2 (b): killed after the creates, foreign links item 3 blocks a bug blocks item 1: the rerun exits non-zero
      naming a chain link and its bd removal; the call log has no link write.
- [ ] AC2 (d), (e): each listed plan edit after a kill gives the exit and snapshot the AC names; a chunk 4 added after a
      kill gives exactly one item 4 and one item-4-on-1 link.
- [ ] AC5 (part): `bd close <w>` exits non-zero while `<w>.review` is open.
- [ ] AC6 (part): spec `y` approved by v0.19.0, a foreign `bd create --parent <y-epic>`, then `plan approve y`: no
      witness, that child unchanged.
- [ ] AC12 (pin): default config pins round limit 5 and TTL `2h`. AC13 (part): epic and items carry the label and prefix.
- [ ] AC15 (part): with `types.custom: gate,spike2`, `plan approve` exits non-zero naming `mindspec setup`.

**Depends on**
Bead 1, Bead 2

## Bead 4: Reducer core and `next-action`

**Steps**
1. Add `state.go`: read the epic subtree by computed and recorded ids, the git refs it names, the panel ledger and
   verdicts, `mindspec_run` (or `DefaultPolicy` with `run-marker-missing`) and the clock. Expose open decisions and
   `Halted()` (a receipted `halt` answer) for Beads 5-8.
2. Add `reducer.go`: one pure pass from state to a write list and the §4 output. Cover implement authorisation (§13.3),
   round opening and the `panel create` entry in `run` (§13.7), slot creation after `panel create`, the inline tally, a
   passing gate close (frozen), the final gate round 1 and the `complete` entry.
3. Add `flags.go` for `run-marker-missing`, `foreign-child-open`, `foreign-blocker` (B7 check on each missing link,
   naming the first foreign link and its bd removal) and `id-collision`. Bead 7 adds the close and claim flags.
4. Add `output.go`: schema `mindspec.next-action/v1`, `revision` digest (no display time, no free text), statuses and
   exits 0/3/4/5/1, order plan-kind-slot, argv arrays, `termsafe` on text.
5. Add `cmd/mindspec/next_action.go`. Without `--apply` it writes nothing. With `--apply` it runs one pass under the
   phase lock, writes receipts before closes, and writes nothing when nothing changed. Refuse a spec with no approved
   plan, a pre-kernel spec (name today's verbs) and `--apply` with no `step`.
6. Update `.mindspec/domains/workflow/interfaces.md`. Output and refusals cite no internal record number (AC16).

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Reducer|Flags|Output)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134NextAction_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run 'HelpGolden|Commands'` passes.

**Acceptance Criteria**
- [ ] AC1: `next-action x --json` twice: same bytes and snapshot, one action, item 1 `implement`, `"exists":false`. A
      changed input changes `revision`; a display time does not.
- [ ] AC2 (apply): the first `--apply` creates exactly `<1>.impl.a1`; a second, or a rerun after a kill at its first
      write, gives the same snapshot and `revision`.
- [ ] AC2 (c): killed after the `<epic>.final` create, foreign links `<epic>.final` blocks a bug blocks item 3:
      `--apply` exits 0 without the item-3-to-final link and flags `foreign-blocker` naming the link. After `bd dep
      remove`, the flag is gone and `--apply` writes the link.
- [ ] AC6 (part): with each kernel step force-closed and retyped `chore` and no `mindspec_run` or `panel.json`,
      `next-action` flags `run-marker-missing`. `next-action y` on a v0.19.0 spec exits non-zero, same snapshot.
- [ ] AC13 (part): a foreign note under the epic gets no action and a `foreign-child-open` flag. A snapshot diff around
      `next-action --apply` changes only computed ids and links between them.
- [ ] AC14 (part): a foreign `<1>.review.1`: after a passing tally the gate stays open and `next-action` exits 3
      flagging `foreign-child-open`; after its close the gate closes `passed`.

**Depends on**
Bead 1

## Bead 5: `lease` and `submit`

**Steps**
1. Add `lease.go`: under the phase lock write `in_progress`, a monotonic generation, the expiry and a token hash with
   `SetStatusIf` on the status read. Print the token once. Support `--renew`, `--release` and `--take --reason`.
2. Refuse a foreign bead, a gate, a decision, a held or blocked ancestor or upstream (an open epic, work item or gate
   does not hold), a live lease (name `lease --take`), a halted spec and an unanswered decision (§13.8).
3. Add `submit.go`: check the generation, the subject, the artifact blob at its commit and its fit per kind (verdict,
   fix commit, implement commit past the claim base). Write the receipt, close, run one pass (§13.9). Same receipt
   again: exit 0, no change. `submit <decision> --answer <opt>` takes a listed option and needs no token.
4. On `ErrCASMismatch` read again and run once more; a second mismatch refuses, step open with at most its receipt.
   Repair an unreceipted close when the evidence and current generation match (§9).
5. Add `cmd/mindspec/lease.go` and `cmd/mindspec/submit.go`. Set `enforced_by` of `panel.one-fixer` and
   `lifecycle.complete-by-orchestrator` in `internal/rules/rules.yaml` to the enforcing verbs.
6. Update `.mindspec/domains/workflow/interfaces.md`. Output and refusals cite no internal record number (AC16).

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Lease|Submit)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134(Lease|Submit)Cmd_' -v` passes and lists the new tests.
- [ ] `go test ./internal/rules/... -run 'Rules'` passes.

**Acceptance Criteria**
- [ ] AC3 (lease): `lease <1>.impl.a1` prints generation 1; a second lease exits non-zero naming `lease --take`; after
      the TTL `lease` prints generation 2. A foreign bug blocking item 1: `lease <1>.review.r1.R1` exits non-zero naming
      it.
- [ ] AC4: each listed bad `submit` exits non-zero with the step open. A valid `submit` writes `mindspec_close` = path,
      commit, blob id; a rerun exits 0, another artifact exits non-zero, each with `bd show --json` unchanged. Killed
      between receipt and close: step open with its receipt; a rerun closes it.
- [ ] AC10 (part): `submit` with a valid verdict and token after a raw slot close writes the receipt; no `.a2`.
- [ ] AC13 (part): after `bd update <1>.impl.a1 --claim`, `lease <1>.impl.a1` exits 0.
- [ ] AC14 (part): a fake bd answers 13 to the first `--if-status` write: `submit` exits 0 with one receipt and one
      close; always 13: non-zero, step open with at most its receipt.

**Depends on**
Bead 4

## Bead 6: Refused rounds, decisions and halt

**Steps**
1. Add `rounds.go` and `panel.RoundsForGate`: count rounds per gate over every registration of the work item (or every
   `final_review` registration), abandoned included, from the ledger. Redispatch is free.
2. Add `reject.go`: a refused tally below the limit creates `<gate>.r<N+1>.fix`; a REJECT also creates
   `<gate>.decide.reject.r<N>` (`fix`, `halt`); at the limit only `<gate>.decide.limit.r<N>` (`continue:<n>`, `halt`).
   `next-action` lists the decision, not the fix, until `--answer fix`. `continue:<n>` raises the allowance.
3. Add B10 supersede: a new subject during round N closes its open slots `superseded` and authorises round N+1; a commit
   in an open fix does not.
4. Add `halt.go`: from the answer's receipt close every open step and decision under the epic `halted`, except a step
   with an open foreign child and its kernel ancestors. Status `halted` wins over `wait`; exit 4. A retry of the answer
   and `--apply` finish the halt and write nothing else. Each `halt` option text says it stops every work item.
5. Apply B9 and the decision grammar to `<epic>.final` (§13.1). Update `.mindspec/domains/workflow/architecture.md`.
   Text cites no internal record number (AC16).

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Rounds|Reject|Halt|Supersede)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/panel/... -run '^TestS134Rounds_|ForBead' -v` passes and lists the new tests.

**Acceptance Criteria**
- [ ] AC7 (reducer): 3 rounds on slug A, `panel abandon` A, 2 on slug B: `next-action` lists one decision
      (`continue:<n>`, `halt`) and `run` has no `panel create`; after `continue:1` one more round is authorised. The
      same holds for the final gate.
- [ ] AC8 (reducer): REJECT on item 2, `--apply`: a decision under item 2 whose `halt` text says it stops the spec, and
      an action for item 3.
- [ ] AC9 (reducer): a commit on `bead/2` in round 2 with no fix open: one pass marks round-2 slots `superseded` and
      lists `panel create 2`; in an open fix, none. A foreign child under `<2>.review`, a second REJECT, answer `halt`:
      other open steps `halted`, `<2>.review` open flagged `foreign-child-open`, `next-action x` exits 4. Killed after
      the `halt` receipt: `--apply` closes the decision and open steps `halted`, exits 4.
- [ ] A halt pass never closes a foreign bead (selection by computed id). New strings: no internal record number.

**Depends on**
Bead 4

## Bead 7: Unreceipted closes, raw closes and the audit table

**Steps**
1. Add `closes.go`: flag `unreceipted-close` (a kernel close with no or a bad receipt) and create `<id>.a<k+1>` with
   `mindspec_from` on the next pass unless Bead 5's repair applies.
2. Flag `raw-close` (a work item closed by bd after its gate passed, not merged), `forced-close` (closed over an open
   child) and `raw-claim` (a bd claim with no lease). Flags come from current state and are never stored. A `raw-close`
   or `forced-close` item releases no dependent (B4).
3. Add `audit.go`: count each escape hatch, deferrals, `--take`, timed decision answers, raw claims, repaired
   unreceipted closes, carry-overs and the slot count of each re-panel, from durable records.
4. In `internal/approve/impl.go` print the audit table and refuse on an open step, an open decision, an unrepaired
   unreceipted close, `raw-close`, `forced-close` or an open foreign child under the epic. Check before any write. Keep
   the finalize allow-set to recorded ids.
5. Update `.mindspec/domains/workflow/interfaces.md` and `.mindspec/domains/execution/interfaces.md` (allow-set note).
   Text cites no internal record number (AC16).

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Closes|Audit)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/approve/... -run '^TestS134ImplApprove_' -v` passes and lists the new tests.
- [ ] `go test ./internal/approve/... -run 'ApproveImpl'` passes (regression).

**Acceptance Criteria**
- [ ] AC10 (part): `bd close <slot>` with no verdict, or with a verdict and an old token, flags `unreceipted-close` and
      the next `--apply` creates `<slot>.a2`. After item 1's gate passes, `bd close <1>` flags `raw-close` and `impl
      approve` exits non-zero naming it. `bd close --force <1>` flags `forced-close`.
- [ ] AC11 (audit): one of each listed event gives `impl approve` and a rerun exit 0, each printing count 1; the
      re-panel row shows 3 slots.
- [ ] AC13 (part): `next-action` flags `raw-claim`. A foreign note under the epic at the end: `impl approve` exits
      non-zero naming it and writes nothing.

**Depends on**
Bead 2, Bead 4

## Bead 8: Panel verbs as kernel steps

**Steps**
1. Make `panel create` the one round writer for an enrolled spec: take the subject from the gate, refuse another
   `--target`, a round past the allowance (`kernel.RoundsForGate`), an open slot on the current subject, an unanswered
   decision on the gate, a passed gate, an id collision (`id-collision`) and a halted spec. Write `"orchestrated": true`
   in `panel.json`. Run one pass (§13.9).
2. Make `panel submit` require `--lease` of the current generation and close the slot as `submit` does.
3. Make a passing `panel refute` or `panel defer` retally close the gate `passed` and its open fix or limit decision
   `superseded`.
4. Keep every refusal of today. A pre-kernel spec keeps today's panel rules.
5. Update `.mindspec/domains/workflow/interfaces.md`. Refusals cite no internal record number (AC16).

**Verification**
- [ ] `go test ./cmd/mindspec/... -run '^TestS134Panel(Create|Submit|Refute|Defer)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run 'PanelCreate|PanelSubmit|PanelDefer|PanelRefute|PanelVerbs'` passes (regression).
- [ ] `go test ./internal/panel/...` passes.

**Acceptance Criteria**
- [ ] AC3 (part): `panel submit` without `--lease` exits non-zero; the slot stays open.
- [ ] AC5 (part): after a pass, `panel create <w>` exits non-zero. With `r2.fix` or a limit decision open, `panel
      refute` of the only blocking finding closes the gate `passed` and the fix or decision `superseded`.
- [ ] AC7 (verb): 3 rounds on slug A, `panel abandon` A, 2 on slug B: round 3 of B exits non-zero; after `continue:1`
      one more round runs and the next exits non-zero; the same for the final gate.
- [ ] AC8 (verbs): before the answer, `lease` on the fix and `panel create 2` exit non-zero; after `--answer fix`,
      `lease` exits 0.
- [ ] AC14 (part): a foreign `bd create --id <1>.review.r1.R1`: `panel create 1` exits non-zero printing `id-collision`;
      that bead is unchanged.

**Depends on**
Bead 5, Bead 6

## Bead 9: `next`, `complete` and the end-to-end path

**Steps**
1. Make `next <w>` take the lease of `<w>.impl.a<k>` as `lease` does and print the token; with a live lease refuse and
   name `lease --take`. Select only approved work items by computed id; resolve the spec without `--spec`, and skip a
   halted spec. A pre-kernel spec selects by recorded `bead_ids`.
2. Make `complete` refuse unless the gate closed `passed` and no child is open. On `raw-close` merge and clear it; on
   another closed unmerged item or `forced-close` refuse with `bd reopen <w>`. A rerun on a merged item exits 0.
   `MINDSPEC_SKIP_PANEL=1` with no passing gate refuses and names it. Run one pass (§13.9).
3. Enforce the bead panel gate for every enrolled spec whatever `panel_presence` says; a pre-kernel spec keeps today.
4. Add `cmd/mindspec/kernel_e2e_test.go`: build the binary, plan-approve spec `x` with this binary, and drive only
   `next-action --apply`, `lease`, `submit` and each `run` entry to `impl approve` exit 0.
5. Add a test that scans the added non-test Go lines and skill lines of the branch for internal record numbers and
   `.beads/issues.jsonl`. Update `.mindspec/domains/workflow/runbook.md`. Text cites no internal record number (AC16).

**Verification**
- [ ] `go test ./cmd/mindspec/... -run '^TestS134(Next|Complete|E2E|Surface)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/complete/... -run '^TestS134Complete_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run 'Next|CompleteCheck'` and `go test ./internal/complete/...` pass (regression).
- [ ] `git diff main...HEAD -U0 -- cmd internal ':!*_test.go' | grep '^+' | grep -n -E 'ADR-[0-9]{4}|[Ss]pec [0-9]{3}|mindspec-[a-z0-9]{4}|issues\.jsonl'` exits 1.

**Acceptance Criteria**
- [ ] AC17: only `next-action --apply`, `lease`, `submit` and `run` entries reach `impl approve` exit 0 and
      `"status":"done"`. After `submit <1>.impl.a1`, `run` lists `panel create 1`; after it, `--apply` lists slots
      `<1>.review.r1.R<s>`, `ForBead` counts 1 round, a rerun of `panel create 1` exits non-zero; the export runs.
      AC2 (part): the pass of `complete 1` creates exactly `<2>.impl.a1` and `<3>.impl.a1`.
- [ ] AC3 (part): `next 1` with no `--spec` resolves `x` and prints a token (generation 1); a second `next 1` exits
      non-zero naming `lease --take`.
- [ ] AC6 (part): `panel_presence: warn`, kernel steps force-closed: `complete` with no passing panel exits non-zero.
      On spec `y`, `complete` with no panel warns and exits 0; `next` does not select the foreign child.
- [ ] AC9 (verbs): on the halted spec `lease`, `complete`, `next 2`, `panel create 2` exit non-zero, same snapshot.
- [ ] AC10 (part): `complete 1` exits 0 and clears `raw-close`; on `forced-close` it exits non-zero printing `bd reopen
      <1>`; after `bd reopen <1>` and a gate pass it exits 0 and `impl approve` does not name the flag.
- [ ] AC11 (part): `MINDSPEC_SKIP_PANEL=1 complete` with no passing gate exits non-zero and names it. AC12 (pinned):
      enrolled with defaults, then `panel.max_rounds: 99` and `panel.lease_ttl: 10m`: round 6 exits non-zero and a
      new lease runs 2h.
- [ ] AC18 (verbs): with a judge open, `complete 1` and `impl approve` exit non-zero naming it. After a carry-over,
      `complete 1` exits 0 and creates `<2>.impl.a1`, `impl approve` names neither gate nor judge, and a pass after each
      creates no judge. After a re-panel pass or a rebind, `complete 1` exits 0.
- [ ] AC16: the surface scan test passes and goes red on a planted `ADR-0035` string.

**Depends on**
Bead 3, Bead 7, Bead 8, Bead 10

## Bead 10: Change judge

**Steps**
1. Add `judge.go`: compute the accepted subject (§13.2) from the passing panel's subject as `panel rebind` refreshes it
   and the last `carry-over`. A commit that changes only a panel directory of the gate does not move it.
2. When the newest subject commit of a passed gate is not the accepted subject, create `<gate>.judge.a<k>` (kind
   `change-judge`, child of the gate's parent) unless the item is merged or the epic is closed. Close an open judge
   `superseded` when its subject is not the newest commit or is the accepted subject; its receipt names the rebind.
3. Extend `submit` for a judgement: `carry-over` with a reason, or `re-panel` with n slots (2 to the passing round's
   slot count) and the changed paths. Check the shape, the old subject, the new subject and the frozen receipt.
4. On `carry-over` add the old subject, the new subject and the judge receipt to the gate receipt. On `re-panel` create
   `<gate>.a<k+1>` with `mindspec_from` and authorise an n-slot round whose `run` entry carries the paths.
5. Make a successful `panel rebind` run one pass. Update `.mindspec/domains/workflow/architecture.md`. Text cites no
   internal record number (AC16).

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134Judge_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134JudgeCmd_|PanelRebind' -v` passes and lists the new tests.

**Acceptance Criteria**
- [ ] AC18 (judge): after a 3-slot pass, a `.mindspec/config.yaml` commit on `bead/1`: `--apply` creates exactly
      `<1>.review.judge.a1` and no round. A second commit before its submit: the next pass closes a1 `superseded` and
      creates a2 on the newest commit; `submit` to a1 exits non-zero.
- [ ] AC18 (a): `carry-over` with a reason keeps the gate `passed`; its receipt names both subjects and the judge
      receipt. A commit of the judgement file creates no judge.
- [ ] AC18 (b): `re-panel` with 2 slots and paths gives `<1>.review.a2` (`mindspec_from` set), one 2-slot round, and a
      `panel create 1` entry that names the paths.
- [ ] AC18 (c): 1 or 4 slots, another value, `carry-over` with no reason, `re-panel` with no paths each exit non-zero,
      step open, no round; 3 slots exits 0.
- [ ] AC18 (d): a successful `panel rebind` closes the judge `superseded` in its pass; the receipt names the rebind.

**Depends on**
Bead 5

## Bead 11: Skills, guides, setup and the ADR amendment

**Steps**
1. Rewrite both copies of `ms-bead-cycle`: loop `next-action --apply`, `lease`, dispatch, `submit` and each `run` entry.
   Remove the spec-wide halt rows of §13.6 from both copies of `ms-bead-cycle` and `ms-spec-autopilot`; keep the rows
   that §13.6 keeps. Remove default-3 round text and deleted keys from every skill.
2. Add the old skill text to `historical_skills` so existing installs refresh. Make each `.claude/skills` copy
   byte-equal to its `plugins/mindspec/skills` source.
3. Change `bead.EnsureBeadsConfig` and `mindspec setup`: add `step` to `types.custom` and keep every value there,
   `--force` included. Replace no value of a key outside the required set.
4. Update `project-docs/user/guides/autonomy.md`, `README.md` and `plugins/mindspec/README.md` for the one path and `bd
   ready --exclude-label mindspec`.
5. Add the HC-4 amendment section to ADR-0035 (exit 3, 4, 5 for `next-action`) and its pin test in
   `cmd/mindspec/adr0035_amendment_test.go`. Skill, guide and setup text cites no internal record number (AC16).

**Verification**
- [ ] `go test ./internal/bead/... ./internal/setup/... -run '^TestS134Setup_' -v` passes and lists the new tests.
- [ ] `go test ./internal/setup/... ./plugins/mindspec/... -run 'Refresh|Historical|SkillParity|Workflow'` passes.
- [ ] `go test ./cmd/mindspec/... -run 'TestADR0035Amendment' -v` passes and lists the new pin test.
- [ ] The AC15 `git grep` over `.claude/skills plugins/mindspec/skills plugins/mindspec/README.md README.md project-docs/user/guides` exits 1.
- [ ] `for s in plugins/mindspec/skills/*/; do n=$(basename $s); cmp $s/SKILL.md .claude/skills/$n/SKILL.md || echo $n; done` prints nothing.

**Acceptance Criteria**
- [ ] AC15: the skills-and-docs `git grep` exits 1 (Bead 2 owns the domains grep). Both `ms-bead-cycle` copies name `next-action --apply`, `lease` and
      `submit`; `cmp` of each skill pair exits 0. After plain `mindspec setup` on an old install with `types.custom:
      gate,spike2`, it holds `gate,spike2,step` and each installed skill copy matches its source.
- [ ] A setup that replaces `types.custom` under `--force` fails a test. The ADR-0035 pin test finds exit codes 3, 4
      and 5 and goes red without the section. Added skill lines hold no internal record number (AC16).

**Depends on**
None

## Provenance

Each `-run` pattern runs in the package its bead's Verification names.

| Acceptance Criterion | Bead(s) | Verified By |
|---------------------|---------|-------------|
| AC1 | 4 | `-run '^TestS134NextAction_'` |
| AC2 | 3, 4, 9 | `-run '^TestS134Enrol_'` (enrolment, a, b, d, e); `-run '^TestS134NextAction_'` (apply, c); `-run '^TestS134E2E_'` |
| AC3 | 5, 8, 9 | `-run '^TestS134Lease'`; `-run '^TestS134PanelSubmit_'`; `-run '^TestS134Next_'` |
| AC4 | 5 | `-run '^TestS134(Submit|SubmitCmd)_'` |
| AC5 | 3, 8 | `-run '^TestS134Enrol_'`; `-run '^TestS134Panel(Create|Refute)_'` |
| AC6 | 3, 4, 9 | `-run '^TestS134Enrol_'`; `-run '^TestS134NextAction_'`; `-run '^TestS134(Complete|Next)_'` |
| AC7 | 6, 8 | `-run '^TestS134Rounds_'`; `-run '^TestS134PanelCreate_'` |
| AC8 | 6, 8 | `-run '^TestS134Reject_'`; `-run '^TestS134PanelCreate_'` |
| AC9 | 6, 9 | `-run '^TestS134(Halt|Supersede)_'`; `-run '^TestS134E2E_'` |
| AC10 | 5, 7, 9 | `-run '^TestS134Submit_'`; `-run '^TestS134(Closes|ImplApprove)_'`; `-run '^TestS134Complete_'` |
| AC11 | 2, 7, 9 | `-run '^TestS134Friction_'`; `-run '^TestS134Audit_'`; `-run '^TestS134Complete_'` |
| AC12 | 2, 3, 9 | `-run '^TestS134Config'`; `-run '^TestS134Enrol_'`; `-run '^TestS134E2E_'` |
| AC13 | 1, 3, 4, 5, 7 | `-run '^TestS134Store_'`; `-run '^TestS134Enrol_'`; `-run '^TestS134Flags_'`; `-run '^TestS134Lease_'`; `-run '^TestS134ImplApprove_'` |
| AC14 | 1, 4, 5, 8 | `-run '^TestS134Store_'`; `-run '^TestS134Flags_'`; `-run '^TestS134Submit_'`; `-run '^TestS134PanelCreate_'` |
| AC15 | 2, 3, 11 | `-run '^TestS134Enrol_'`; `-run '^TestS134Setup_'`, the `git grep` and `cmp` lines; Bead 2 domains `git grep` |
| AC16 | all; 9 | each bead's message tests; `-run '^TestS134Surface_'` and the `git diff` grep |
| AC17 | 9 | `-run '^TestS134E2E_'` |
| AC18 | 10, 9 | `-run '^TestS134Judge'`; `-run '^TestS134E2E_'` |
| Validation proofs | final review | `make test` on the spec branch |
