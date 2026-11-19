---
status: Draft
spec_id: 134-governed-autonomy-kernel
version: "1"
adr_citations: [ADR-0045, ADR-0023, ADR-0040, ADR-0038, ADR-0037, ADR-0035, ADR-0043]
work_chunks:
  - {id: 1, depends_on: [], key_file_paths: [internal/next/kernel/ids.go, internal/next/kernel/idmode.go,
      internal/next/kernel/store.go, internal/next/kernel/records.go, internal/next/kernel/clock.go,
      internal/next/kernel/witness.go, internal/next/kernel/kerneltest/fixture.go, internal/phase/phaselock.go,
      internal/bead/config.go, internal/harness/sandbox.go, internal/doctor/beads.go, .beads/config.yaml]}
  - {id: 2, depends_on: [], key_file_paths: [internal/config/config.go, cmd/mindspec/config.go,
      internal/redact/redact.go, cmd/mindspec/selfemit.go, cmd/mindspec/ceremony_guard_test.go]}
  - {id: 3, depends_on: [1, 2], key_file_paths: [internal/approve/plan.go, internal/approve/enrol.go,
      internal/phase/derive.go, .mindspec/adr/ADR-0023.md]}
  - {id: 4, depends_on: [1], key_file_paths: [internal/next/kernel/reducer.go, internal/next/kernel/state.go,
      internal/next/kernel/output.go, internal/next/kernel/flags.go, cmd/mindspec/next_action.go,
      internal/panel/tally.go]}
  - {id: 5, depends_on: [4], key_file_paths: [internal/next/kernel/lease.go, internal/next/kernel/submit.go,
      cmd/mindspec/lease.go, cmd/mindspec/submit.go, internal/next/worktree.go, internal/rules/rules.yaml]}
  - {id: 6, depends_on: [4, 5], key_file_paths: [internal/next/kernel/reject.go, internal/next/kernel/halt.go,
      internal/next/kernel/final.go, internal/panel/rounds.go, internal/next/kernel/submit.go]}
  - {id: 7, depends_on: [2, 4, 5], key_file_paths: [internal/next/kernel/closes.go, internal/next/kernel/audit.go,
      internal/approve/impl.go, internal/approve/impl_final_gate.go]}
  - {id: 8, depends_on: [5, 6], key_file_paths: [cmd/mindspec/panel.go, cmd/mindspec/panel_defer.go,
      internal/panel/create.go, internal/panel/submit.go, internal/panel/refute.go]}
  - {id: 9, depends_on: [3, 7, 8, 10], key_file_paths: [cmd/mindspec/next.go, internal/next/select.go,
      internal/complete/complete.go, internal/complete/panel_presence.go, internal/approve/plan.go,
      internal/harness/lifecycle_scenario_test.go]}
  - {id: 10, depends_on: [5, 8], key_file_paths: [internal/next/kernel/judge.go, cmd/mindspec/panel_rebind.go,
      internal/panel/create.go]}
  - {id: 11, depends_on: [9], key_file_paths: [.claude/skills/ms-bead-cycle/SKILL.md,
      plugins/mindspec/skills/ms-bead-cycle/SKILL.md, .claude/skills/ms-spec-autopilot/SKILL.md,
      plugins/mindspec/skills/ms-spec-autopilot/SKILL.md, internal/setup/skills.go,
      .mindspec/adr/ADR-0035-agent-error-contract.md, project-docs/user/guides/autonomy.md,
      cmd/mindspec/surface_scan_test.go, .github/workflows/ci.yml]}
  - {id: 12, depends_on: [9], key_file_paths: [cmd/mindspec/kernel_e2e_test.go]}
---
# Plan: 134-governed-autonomy-kernel

## ADR Fitness

Each touchpoint ADR stays sound; none needs a superseding ADR.

- **ADR-0045, ADR-0040.** Applied. Bead selection, halt rows and the round limit move from skills into the binary
  (Beads 4, 6, 11); `panel.one-fixer` and `lifecycle.complete-by-orchestrator` stop being advisory (Bead 5).
- **ADR-0023.** Applied, and narrowed for enrolled specs. The `step` type, tuple metadata and receipts stay in bd;
  evidence stays in git (Beads 1, 3). Its phase-derivation discriminator (`issue_type == "task"` is lifecycle work)
  gives way, for an enrolled spec only, to recorded work-item ids and kernel ids; a pre-kernel spec keeps it. Bead 3
  adds the amendment note to ADR-0023's discriminator section and states the rule in the core `interfaces.md` section
  "Phase derivation".
- **ADR-0038.** Applied. `--allow-not-ready` joins `escapeHatchFlags` and `EscapeHatchTokens` (Bead 2).
- **ADR-0037.** Applied. The gate legs stay; `panel submit` checks a lease; `panel create` is the one round writer
  (Bead 8).
- **ADR-0043.** Applied. Round counts come from the ledger over every registration (Bead 6). Receipts are bd metadata.
- **ADR-0035.** Amended. HC-4 gains the `next-action` exit codes 3, 4 and 5 (Bead 11, with its pin test). Each new
  refusal keeps one `recovery:` line.

## Decomposition

Twelve beads, because the spec adds four verbs, changes six and rewrites skills; one bead is one session. The longest
chain is 1 -> 4 -> 5 -> 6 -> 8 -> 10 -> 9 -> 12. Each link carries state the next bead reads: the reducer needs the
store; `submit` runs a reducer pass; the halt retry and the final fix check edit `submit`; the panel verbs call
`submit` and the round counter; the re-panel round goes through the rewritten `panel create`; `next` and `complete`
need every gate rule; the end-to-end test needs every verb. Waves: {1, 2}, {3, 4}, {5}, {6, 7}, {8}, {10}, {9},
{11, 12}; Bead 3 can run in any wave after {1, 2}. Kernel code goes in `internal/next/kernel`, which the workflow
manifest already owns.

**Pre-kernel path until Bead 9.** Bead 3 builds enrolment behind the unexported package variable
`approve.enrolOnApprove` (default false). Its tests set it in-process; no verb, flag, env var or key reads it. Until
Bead 9 deletes the variable, `plan approve`, `next`, `complete`, `impl approve` and the panel verbs keep today's path
for every real spec, so a spec-branch binary enrols nothing and the existing lifecycle tests stay green. Every new
refusal in Beads 4-10 applies only when the witness (Bead 1) says the spec is enrolled; Beads 4-10 test on the
`kerneltest` fixture. Bead 9 deletes the variable, so `plan approve` enrols every new spec, and updates the existing
tests in the same bead.

**Tracker housekeeping.** The spec's Scope items (close `mindspec-4xzi`, `mindspec-naq0` and `mindspec-sxjc`;
re-scope `mindspec-qu9z`) are for the orchestrator at the final review, after it checks that each copy matches. No
implementer bead does them.

## Testing Strategy

- **Test names.** Each new test starts with `TestS134<Mechanism>_`; no base-tree test matches `S134`. Each
  Verification line with a `TestS134` prefix must list its new tests in `go test -v` output. No `TestS134` test reads
  `testing.Short()`.
- **Green between beads.** Each bead's Verification runs `go test -short` on the whole of every package it touches,
  plus the named pin tests. A bead that changes a pinned surface updates the pin test in the same bead; it never
  leaves a pin red for a later bead. Pins: `TestCeremonyNonInflation_ConfigKeys`, `_HelpFlags`, `_NextFlags`
  (`cmd/mindspec/ceremony_guard_test.go`); `TestReadyCheck_CycleSkillCopiesMatch` (`cmd/mindspec/bead_ready_test.go`);
  `internal/lint/destructive_guidance_test.go`; `internal/setup/{readiness_skill_pin,review_discipline,skills}_test.go`;
  `internal/bead/config_test.go`; `internal/bootstrap/bootstrap_test.go`; `internal/doctor/beads_test.go`;
  `TestApprovePlan*`; `internal/harness/lifecycle_scenario_test.go`.
- **Real bd.** Scenario tests run real bd 1.3.4 in a scratch repo through `harness.NewSandbox`
  (`internal/harness/sandbox.go`). They fail, not skip, when bd is absent, as `cmd/mindspec/next_ready_gate_test.go`.
  From Bead 1, `NewSandbox` writes `types.custom: "gate,step"` through `bead.EnsureBeadsConfig`.
- **Fake bd.** Fault tests put a script named `bd` first on `PATH`, as `nextHonestyFakeBD`
  (`cmd/mindspec/next_selection_test.go`) and `installFakeBD` (`cmd/mindspec/verb_output_test.go`). A forwarding
  wrapper logs argv, calls real bd and injects a cycle refusal, exit 13 or `kill -9 $PPID` on a named write. No test
  starts a model CLI.
- **bd seams.** The kernel store runs bd through a runner it is given (default `bead.RunBD`). `enrol.go` gives it the
  approve package's `planRunBDFn`, so `approve.SetPlanRunBDForTest` covers enrolment writes; `phase.SetRunBDForTest`
  covers derivation reads.
- **Clock.** Bead 1 adds `kernel.Now` (a `func() time.Time` seam) and `kerneltest.SetClock`. Every lease, expiry,
  `retry_at` and timed answer reads `kernel.Now`. In-process tests move the clock; binary tests never wait and read
  the recorded expiry.
- **Shared fixture.** Bead 1 adds `kerneltest`: `Enrolled` (spec `x`, items 2 and 3 on item 1, built through the
  store with the Bead 1 record types), `PreKernelSpec` (the v0.19.0 shape: epic, items and plan links, no witness),
  `Snapshot` (the spec's snapshot, compared byte for byte), the forwarding fake bd and the id resolver `ID`. Bead 3
  tests that `Enrolled` equals a real `plan approve` snapshot after id normalisation.
- **Ids in tests (OQ1).** Bead 1 commits the probe result as `kernel.NativeIDs`; later beads read only that constant
  and run on that branch. Every `<computed id>` in a bd argv or assertion is `kerneltest.ID(t, <computed id>)`: the
  computed id with native ids, else the bd id whose `mindspec_key` is the computed id. Fallback forms: AC14's foreign
  `bd create --id <1>.review.r1.R1` becomes a foreign `bd create --metadata '{"mindspec_key":"<1>.review.r1.R1"}'`
  with no tuple, and `panel create 1` still prints `id-collision`; AC14's foreign `<1>.review.1` is a foreign `bd
  create --parent ID(<1>.review)`; AC13's "computed ids" are the beads with a valid key and tuple; a fake-bd link
  write is a `dep add` whose ids both resolve to computed ids; "`<slot>.a2` exists" is `Lookup(<slot>.a2)` found.
- **Record shapes.** Bead 1 defines every kernel metadata format as a named Go type and key constant in
  `internal/next/kernel/records.go`: tuple, `mindspec_key`, `mindspec_close` (receipt: kind, path, commit, blob, plus
  `repaired`, `rebind` and `carry_overs`), `mindspec_from`, `mindspec_links`, `mindspec_run`, lease (generation,
  expiry, token hash, claim base), take reason, decision answer, defer, and re-panel (slot count, paths). Producers
  write and the audit reads only through these types. A golden JSON test pins each shape.
- **Snapshot diffs.** Each bead that adds or changes a kernel verb asserts, with a foreign note, a foreign bug and a
  raw claim present, that a snapshot diff around that verb changes only computed ids and links between them (Beads 4,
  5, 8, 9, 10). Bead 12 asserts it for every verb on the end-to-end path.
- **Surface rule (AC16).** "Internal record number" = an ADR, spec, requirement (R, B, AC, OQ id), section sign, bead
  or issue number. No added non-test Go line (comments too), CLI text or skill line holds one or names
  `.beads/issues.jsonl`. Bead 11 adds the scan test.
- **Doc sync.** Each bead that changes `internal/**` or `cmd/**` updates a domain doc section of each owning domain
  (core: `internal/{config,phase,redact}`; execution: `internal/{bead,harness,executor}`; workflow: the rest), because
  the released `complete` refuses code with no domain-doc change. Each bead edits only its own named section. Section
  owners: core `interfaces.md`: "Phase lock" (1), "Config keys" (2), "Phase derivation" (3). execution
  `interfaces.md`: "bd types and sandbox" (1). workflow `interfaces.md`: "Kernel store and ids" (1), "Config show and
  friction" (2), "next-action" (4), "lease and submit" (5), "Close flags and audit" (7), "Panel verbs" (8, 10), "Setup
  and skills" (11). workflow `architecture.md`: "Enrolment" (3), "Rounds, decisions and halt" (6), "Change judge" (10).
  workflow `runbook.md`: "Kernel loop" (9, then 12). When two beads of one wave edit one file, the lower-numbered bead
  merges first and the other rebases on it.
- **Dogfood boundary.** The lane runs lifecycle and panel verbs on the released binary, never the spec-branch binary.
- **Gate.** The final review runs `make test`.

## Bead 1: Kernel foundation: bd probe, ids, records, store and the step type

**Steps**
1. Add `step` to the required `types.custom` set. `bead.EnsureBeadsConfig` treats `types.custom` as a set: it adds
   `gate` and `step` when missing and keeps every other value, `--force` included; it replaces no value of a key
   outside the required set. `doctor` reports a missing member. `harness.NewSandbox` calls `EnsureBeadsConfig` after
   `bd init`. Add `step` to the repo's `.beads/config.yaml`. `kernel.RequireStepType(root)` reads `types.custom` from
   `.beads/config.yaml` with the reader `doctor` uses; its refusal names `mindspec setup`.
2. Probe bd (OQ1). In a sandbox, run `bd create --id <epic>.1.impl.a1 --parent <epic>.1 --type step`, and the same for
   the `.review`, `.final`, `.decide.` and `.judge.` shapes. Commit the result as `kernel.NativeIDs` in `idmode.go`,
   with the bd version and the observed output in its doc comment. `TestS134Probe_` runs the probe on real bd and
   fails when the result differs from the constant.
3. Add the id grammar in `ids.go`: format and parse each contracts §2 kind with the optional `.a<k>` suffix, and the
   exact-match test for "kernel-shaped". A bd-numbered child such as `<w>.review.1` is not kernel-shaped.
4. Add `records.go` (the record shapes of the Testing Strategy) and `clock.go` (`kernel.Now`).
5. Add `store.go` over a given bd runner. `Create` takes a computed id, type (`step` or bd's `decision`), parent,
   tuple, label `mindspec` and a `[mindspec]` title prefix. With `NativeIDs` false, bd assigns the id, `mindspec_key`
   holds the computed id and `Lookup` scans the epic subtree. Add `SetStatusIf` (bd `--if-status`; exit 13 is
   `ErrCASMismatch`), `WithCASRetry` (read again, run once more, a second mismatch refuses; every kernel status write
   uses it), `WriteReceipt`, `AddLink` (computed ids only) and `Collision` (`id-collision`).
6. Add `witness.go` (`mindspec_run`, a computed kernel id under the epic, or `panel.json` with `"orchestrated": true`)
   and `DefaultPolicy` (round limit 5, lease TTL 2h). Export `phase.WithPhaseLock(root, fn)` over
   `acquirePhaseLock`. Add `kerneltest` (`Enrolled`, `PreKernelSpec`, `Snapshot`, `ID`, `SetClock`, fake bd).
7. Update the doc sections core "Phase lock", execution "bd types and sandbox", workflow "Kernel store and ids", and
   the existing tests that pin `types.custom: "gate"`.

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Probe|IDs|Records|Store|Witness)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/bead/... ./internal/doctor/... -run '^TestS134(Setup|Doctor)_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/phase/... ./internal/bead/... ./internal/doctor/... ./internal/bootstrap/... ./internal/setup/... ./internal/harness/...` passes.
- [ ] `go build ./...` exits 0.

**Acceptance Criteria**
- [ ] OQ1: the probe result is a committed constant; `TestS134Probe_` goes red if bd differs. Fallback-branch store
      tests force `NativeIDs` false through a seam; `kerneltest.ID` resolves under both branches.
- [ ] The grammar accepts every contracts §2 shape with and without `.a<k>` and rejects `<w>.review.1`,
      `<prefix>-<hash>` and `<epic>.<n>`.
- [ ] AC14 primitives: `Collision` refuses a foreign kernel-shaped bead (the fallback form when `NativeIDs` is false)
      and leaves it unchanged; `SetStatusIf` returns `ErrCASMismatch` on bd exit 13 and writes nothing else;
      `WithCASRetry` succeeds after one mismatch and refuses after two.
- [ ] AC13 primitive: every `Create`, a `decision` included, sets label `mindspec` and the `[mindspec]` prefix.
- [ ] AC15 (setup part): with `types.custom: gate,spike2`, `EnsureBeadsConfig` with and without `--force` gives
      `gate,spike2,step`. A writer that replaces the value under `--force` fails a test. `doctor` reports `step` missing.
- [ ] AC6 primitive: witness detection finds each of the three witnesses alone; with none it reports pre-kernel.
- [ ] Each record type round-trips to its golden JSON; a renamed key fails the test.

**Depends on**
None

## Bead 2: Config honesty and escape-hatch registries

**Steps**
1. Delete `type Loop`, `LoopContext` and every `loop:` default in `internal/config/config.go`. `Load` keeps a
   non-strict decode of the removed keys and warns once per removed key that is set.
2. Add `panel.max_rounds` (default 5) and `panel.lease_ttl` (default `2h`). Add `config.RoundLimit()`: it reads
   `panel.max_rounds`; when that is unset it reads `loop.halt.max_rounds_per_bead` and the warning names the value.
3. Rewrite the `config show` render in `cmd/mindspec/config.go`. Each key has one truthful label. Removed keys show as
   ignored. `runner:` shows as read by a skill. Update `TestCeremonyNonInflation_ConfigKeys` and
   `internal/config/config_test.go` to the new key set in this bead.
4. Add `allow-not-ready` to `escapeHatchFlags` (`cmd/mindspec/selfemit.go`) and to `redact.EscapeHatchTokens`.
5. Update core "Config keys" and workflow "Config show and friction". Remove `type Loop struct` and the removed key
   names from `.mindspec/domains/**`. Surface text cites no internal record number.

**Verification**
- [ ] `go test ./internal/config/... -run '^TestS134Config_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134ConfigShow_|^TestS134Friction_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/config/... ./internal/redact/... ./cmd/mindspec/...` passes (includes `TestCeremonyNonInflation_ConfigKeys`).
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
1. Add `internal/approve/enrol.go`, called from `plan approve` when `enrolOnApprove` is true. Under
   `phase.WithPhaseLock`, write in contracts §8 order: each work item (record its id in `bead_ids` right after its
   create), `mindspec_links`, the B7 cycle check, the plan-order links, `mindspec_run` (round limit from
   `config.RoundLimit()`, TTL from `panel.lease_ttl`, rules `kernel-1`), the approval record, the gates `<w>.review`
   and `<epic>.final` and each work-item link to `<epic>.final`. Use the Bead 1 record types and `planRunBDFn`.
2. Implement the §8 rerun rules. Reuse recorded ids, create only missing chunks, refuse every other plan change with
   its name and no write. Refuse a rerun after the approval record and name the enrolment. Never select by
   `--parent`. Before `mindspec_run`, a cycle refusal names the link, the foreign link that closes it and its bd
   removal; after it, a refused link is left for `--apply`.
3. Refuse without `step` (`kernel.RequireStepType`). Add label `mindspec` and the `[mindspec]` prefix to the epic and
   each work item. Make `phase.ParseSpecFromTitle` skip the tag.
4. Replace the type-based `classifyChild` count in `internal/phase/derive.go` for an enrolled spec: only recorded work
   items and kernel ids count; a foreign `task` under the epic does not. A pre-kernel spec keeps today's rule, and a
   `plan approve` rerun on it does not enrol and selects by recorded `bead_ids`.
5. Add `TestS134Enrol_FixtureMatches`: `kerneltest.Enrolled` equals a real enrolment snapshot after id normalisation.
6. Update workflow "Enrolment", core "Phase derivation" and the ADR-0023 amendment note. Refusals cite no internal
   record number.

**Verification**
- [ ] `go test ./internal/approve/... -run '^TestS134Enrol_' -v` passes and lists the new tests.
- [ ] `go test ./internal/phase/... -run '^TestS134Derive_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/approve/... ./internal/phase/...` passes (includes `TestApprovePlan*` on the pre-kernel path).

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
- [ ] The fixture test goes red when `Enrolled` or enrolment changes a key, a link or a gate.
- [ ] AC5 (part): `bd close <w>` exits non-zero while `<w>.review` is open.
- [ ] AC6 (part): spec `y` from `PreKernelSpec`, a foreign `bd create --parent <y-epic>`, then `plan approve y`: no
      witness, that child unchanged.
- [ ] AC12 (pin): default config pins round limit 5 and TTL `2h`. AC13 (part): epic and items carry the label and prefix.
- [ ] AC15 (part): with `types.custom: gate,spike2`, `plan approve` exits non-zero naming `mindspec setup`.

**Depends on**
Bead 1, Bead 2

## Bead 4: Reducer core and `next-action`

**Steps**
1. Add `state.go`: read the epic subtree by computed and recorded ids (`mindspec_links`), the git refs it names, the
   panel ledger and verdicts, `mindspec_run` (or `DefaultPolicy` with `run-marker-missing`) and `kernel.Now`. Expose
   open decisions and `Halted()` for Beads 5-8.
2. Add `reducer.go`: one pure pass from state to a write list and the contracts §4 output. Cover implement
   authorisation (§13.3), round opening and the `panel create` entry in `run` (§13.7), slots after `panel create`,
   the inline tally, a passing gate close (frozen), the `complete` entry and re-creation of a deleted kernel bead
   (its rounds stay in the ledger). A raw-closed work item releases no dependent (B4).
3. Add `flags.go` for `run-marker-missing`, `foreign-child-open`, `foreign-blocker` (B7 check on each missing link,
   naming the first foreign link and its bd removal) and `id-collision`. Bead 7 adds the close and claim flags.
4. Add `output.go`: schema `mindspec.next-action/v1`, `revision` digest (no display time, no free text), `waiting`
   entries with lease generation and expiry, `retry_at` from the earliest expiry, statuses and exits 0/3/4/5/1, order
   plan-kind-slot, argv arrays, `termsafe` on text.
5. Add `cmd/mindspec/next_action.go`. Without `--apply` it writes nothing. With `--apply` it runs one pass under the
   phase lock, writes receipts before closes, uses `WithCASRetry` for every status write and writes nothing when
   nothing changed. Refuse a spec with no approved plan, a pre-kernel spec (name today's verbs), and `--apply` with
   bd silent or no `step`. Update `_HelpFlags` for the new verb.
6. Update workflow "next-action". Output and refusals cite no internal record number.

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Reducer|Flags|Output)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134NextAction_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/panel/... ./cmd/mindspec/...` passes (includes `_HelpFlags`, `HelpGolden`).

**Acceptance Criteria**
- [ ] AC1: `next-action x --json` twice: same bytes and snapshot, one action, item 1 `implement`, `"exists":false`. A
      changed input changes `revision`; a display time does not.
- [ ] AC2 (apply): the first `--apply` creates exactly `<1>.impl.a1`; a second, or a rerun after a kill at its first
      write, gives the same snapshot and `revision`.
- [ ] A live lease gives a `waiting` entry and `retry_at` at its expiry; moving the clock past it changes the output.
      `--apply` with a bd that exits non-zero on every call exits non-zero and writes nothing.
- [ ] A fake bd answers 13 to the first `--if-status` close of an `--apply`: exit 0, one close.
- [ ] A deleted `<1>.impl.a1` is created again by the next `--apply`.
- [ ] AC6 (part): with each kernel step force-closed and retyped `chore` and no `mindspec_run` or `panel.json`,
      `next-action` flags `run-marker-missing`. `next-action y` on `PreKernelSpec` exits non-zero, same snapshot.
- [ ] AC13 (part): a foreign note under the epic gets no action and a `foreign-child-open` flag. A snapshot diff around
      `next-action --apply` changes only computed ids and links between them.
- [ ] AC14 (part): a foreign `<1>.review.1`: after a passing tally the gate stays open and `next-action` exits 3
      flagging `foreign-child-open`; after its close the gate closes `passed`.

**Depends on**
Bead 1

## Bead 5: `lease` and `submit`

**Steps**
1. Add `lease.go`: under the phase lock write `in_progress`, a monotonic generation, the expiry (`kernel.Now` plus the
   pinned TTL) and a token hash with `SetStatusIf` on the status read. Print the token once. Support `--renew`,
   `--release` and `--take --reason` (records the take reason).
2. For an implement step, `lease` calls `next.EnsureWorktree` (the helper `next` uses today) to create `bead/<w>` and
   its worktree, then records the claim base (the `bead/<w>` head at the claim) in the lease record. Bead 9 makes
   `next <w>` call this same lease function.
3. Refuse a foreign bead, a gate, a decision, a held or blocked ancestor or upstream (an open epic, work item or gate
   does not hold), a live lease (name `lease --take`), a halted spec, an unanswered decision (§13.8) and a pre-kernel
   spec (name today's verbs).
4. Add `submit.go`: check the generation, the subject, the artifact blob at its commit and its fit per kind (verdict,
   fix commit past the round subject, implement commit past the recorded claim base). Write the receipt, close, run
   one pass (§13.9). Same receipt again: exit 0, no change. `submit <decision> --answer <opt>` takes a listed option,
   needs no token and writes the answer record. Use `WithCASRetry`. Repair an unreceipted close when the evidence and
   current generation match (receipt `repaired: true`).
5. Add `cmd/mindspec/lease.go` and `cmd/mindspec/submit.go`; update `_HelpFlags`. Set `enforced_by` of
   `panel.one-fixer` and `lifecycle.complete-by-orchestrator` in `internal/rules/rules.yaml` to the enforcing verbs.
   `sequence.bead-impl-prompt` keeps `advisory` and its staged marker; the kernel adds no check for it.
6. Update workflow "lease and submit". Output and refusals cite no internal record number.

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Lease|Submit)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134(Lease|Submit)Cmd_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/rules/... ./cmd/mindspec/...` passes (includes `_HelpFlags`).

**Acceptance Criteria**
- [ ] AC3 (lease): `lease <1>.impl.a1` prints generation 1; a second lease exits non-zero naming `lease --take`; with
      the clock past the TTL `lease` prints generation 2. A foreign bug blocking item 1: `lease <1>.review.r1.R1` exits
      non-zero naming it.
- [ ] A table test: `lease` on a foreign bead, a gate, a decision and a `PreKernelSpec` item each exits non-zero, no write.
- [ ] AC4: each listed bad `submit` exits non-zero with the step open. A commit on `bead/1` made before the lease,
      ahead of the spec branch, is refused as not past the claim base. A valid `submit` writes `mindspec_close` =
      path, commit, blob id; a rerun exits 0, another artifact exits non-zero, each with `bd show --json` unchanged.
      Killed between receipt and close: step open with its receipt; a rerun closes it.
- [ ] AC10 (part): `submit` with a valid verdict and token after a raw slot close writes a `repaired` receipt.
- [ ] AC13 (part): after `bd update <1>.impl.a1 --claim`, `lease <1>.impl.a1` exits 0. A snapshot diff around `lease`
      and `submit` changes only computed ids and links between them.
- [ ] AC14 (part): a fake bd answers 13 to the first `--if-status` write: `submit` exits 0 with one receipt and one
      close; always 13: non-zero, step open with at most its receipt.

**Depends on**
Bead 4

## Bead 6: Rounds, refused rounds, decisions, halt and the final gate

**Steps**
1. Add `panel.RoundsForGate` in `internal/panel/rounds.go`, the one round counter: rounds of one gate over every
   registration of its work item, of every `final_review` registration, or of a spec's `spec_approve` or
   `plan_approve` registrations, abandoned included, from the ledger. Redispatch is free.
2. Add `reject.go`: a refused tally below the limit creates `<gate>.r<N+1>.fix`; a REJECT also creates
   `<gate>.decide.reject.r<N>` (`fix`, `halt`); at the limit only `<gate>.decide.limit.r<N>` (`continue:<n>`, `halt`).
   `next-action` lists the decision, not the fix, until `--answer fix`. `continue:<n>` raises the allowance.
3. Add B10 supersede: a new subject during round N closes its open slots `superseded` and authorises round N+1; a commit
   in an open fix does not.
4. Add `halt.go`: from the answer's receipt close every open step and decision under the epic `halted`, except a step
   with an open foreign child and its kernel ancestors. Status `halted` wins over `wait`; exit 4. In `submit.go`, a
   retry of the `halt` answer and `--apply` close the receipted decision and every other closable step `halted`, exit
   4 and write nothing else. Each `halt` option text says it stops every work item.
5. Add `final.go`: final gate round 1 on the spec branch head when the last item is done (B4), and B9 and the decision
   grammar for `<epic>.final` (§13.1). Extend the `submit` fix check so a final fix commit lands on the spec branch.
6. Update workflow "Rounds, decisions and halt". Text cites no internal record number.

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Reject|Halt|Supersede|Final)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/panel/... -run '^TestS134Rounds_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/panel/...` passes (includes `ForBead`).

**Acceptance Criteria**
- [ ] AC7 (reducer): 3 rounds on slug A, `panel abandon` A, 2 on slug B: `next-action` lists one decision
      (`continue:<n>`, `halt`) and `run` has no `panel create`; after `continue:1` one more round is authorised. The
      same holds for the final gate. `RoundsForGate` counts spec and plan gate registrations the same way.
- [ ] AC8 (reducer): REJECT on item 2, `--apply`: a decision under item 2 whose `halt` text says it stops the spec, and
      an action for item 3.
- [ ] AC9 (reducer): a commit on `bead/2` in round 2 with no fix open: one pass marks round-2 slots `superseded` and
      lists `panel create 2`; in an open fix, none. A foreign child under `<2>.review`, a second REJECT, answer `halt`:
      other open steps `halted`, `<2>.review` open flagged `foreign-child-open`, `next-action x` exits 4 on each read.
      Killed after the `halt` receipt: `--apply` closes the decision and open steps `halted`, exits 4. A retry of `submit
      --answer halt` exits 4 and closes the remaining closable steps; a retry that exits 0 fails the test.
- [ ] After the last item is done, the pass authorises final round 1 on the spec branch head; a final fix commit on a
      bead branch is refused and one on the spec branch is accepted.
- [ ] A halt pass never closes a foreign bead (selection by computed id). New strings: no internal record number.

**Depends on**
Bead 4, Bead 5

## Bead 7: Unreceipted closes, raw closes and the audit table

**Steps**
1. Add `closes.go`: flag `unreceipted-close` (a kernel close with no or a bad receipt) and create `<id>.a<k+1>` with
   `mindspec_from` on the next pass, unless Bead 5's repair wrote a valid `repaired` receipt.
2. Flag `raw-close` (a work item closed by bd after its gate passed, not merged), `forced-close` (closed over an open
   child) and `raw-claim` (a bd claim with no lease). Flags come from current state and are never stored. A `raw-close`
   or `forced-close` item releases no dependent (B4).
3. Add `audit.go`: count each escape hatch from the journal, and deferrals, `--take`, timed decision answers, raw
   claims, repaired unreceipted closes, carry-overs and the slot count of each re-panel from the Bead 1 record types.
4. In `internal/approve/impl.go`, for an enrolled spec, print the audit table and refuse on an open step, an open
   decision, an unrepaired unreceipted close, `raw-close`, `forced-close` or an open foreign child under the epic.
   Check before any write. Keep the finalize allow-set to recorded ids. A pre-kernel spec keeps today's path.
5. Update workflow "Close flags and audit". Text cites no internal record number.

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Closes|Audit)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/approve/... -run '^TestS134ImplApprove_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/approve/...` passes (includes `ApproveImpl`).

**Acceptance Criteria**
- [ ] AC10 (part): `bd close <slot>` with no verdict, or with a verdict and an old token, flags `unreceipted-close` and
      the next `--apply` creates `<slot>.a2`. After a repair by `submit`, `--apply` creates no `<slot>.a2`.
- [ ] AC10 (part): after item 1's gate passes, `bd close <1>` flags `raw-close`, `impl approve` exits non-zero naming
      it, and `--apply` creates no `<2>.impl.a1`. In a fresh fixture `bd close --force <1>` flags `forced-close` and
      `--apply` creates no `<2>.impl.a1`.
- [ ] AC11 (audit): records of each listed event, written through the Bead 1 types, give `impl approve` and a rerun
      exit 0, each printing count 1; the re-panel row shows 3 slots.
- [ ] AC13 (part): `next-action` flags `raw-claim`. A foreign note under the epic at the end: `impl approve` exits
      non-zero naming it and writes nothing.

**Depends on**
Bead 2, Bead 4, Bead 5

## Bead 8: Panel verbs as kernel steps

**Steps**
1. Make `panel create` the one round writer for an enrolled spec: take the subject from the gate, refuse another
   `--target`, a round past the allowance (`panel.RoundsForGate`), an open slot on the current subject, an unanswered
   decision on the gate, a passed gate, an id collision (`id-collision`) and a halted spec. Write `"orchestrated":
   true` in `panel.json`. Run one pass (§13.9).
2. For `--gate spec_approve` or `plan_approve`, count rounds with `panel.RoundsForGate` and, past `panel.max_rounds`,
   print one warning naming the count and proceed (count and warn, no refusal).
3. Make `panel submit` require `--lease` of the current generation and close the slot as `submit` does. Update
   `_HelpFlags`.
4. Make a passing `panel refute` or `panel defer` retally close the gate `passed` and its open fix or limit decision
   `superseded`; `panel defer` writes the defer record.
5. Keep every refusal of today. A pre-kernel spec keeps today's panel rules. Update workflow "Panel verbs".

**Verification**
- [ ] `go test ./cmd/mindspec/... -run '^TestS134Panel(Create|Submit|Refute|Defer)_' -v` passes and lists the new tests.
- [ ] `go test -short ./cmd/mindspec/... ./internal/panel/...` passes (includes `PanelCreate|PanelSubmit|PanelDefer|PanelRefute` and `_HelpFlags`).

**Acceptance Criteria**
- [ ] AC3 (part): `panel submit` without `--lease` exits non-zero; the slot stays open. With a valid `--lease` and
      verdict it closes the slot with a receipt.
- [ ] AC5 (part): after a pass, `panel create <w>` exits non-zero. With `r2.fix` or a limit decision open, a passing
      `panel refute` of the only blocking finding closes the gate `passed` and the fix or decision `superseded`; a
      passing `panel defer` does the same.
- [ ] AC7 (verb): 3 rounds on slug A, `panel abandon` A, 2 on slug B: round 3 of B exits non-zero; after `continue:1`
      one more round runs and the next exits non-zero; the same for the final gate.
- [ ] A sixth `plan_approve` round exits 0 with one warning naming the count; a refusal fails the test.
- [ ] AC8 (verbs): before the answer, `lease` on the fix and `panel create 2` exit non-zero; after `--answer fix`,
      `lease` exits 0.
- [ ] AC14 (part): a foreign kernel-shaped `<1>.review.r1.R1`: `panel create 1` exits non-zero printing
      `id-collision`; that bead is unchanged.
- [ ] AC13 (part): a snapshot diff around `panel create`, `panel submit`, `panel refute` and `panel defer` changes only
      computed ids and links between them.

**Depends on**
Bead 5, Bead 6

## Bead 9: `next`, `complete` and enrolment on

**Steps**
1. Make `next <w>` call the Bead 5 lease function for `<w>.impl.a<k>` and print the token; with a live lease refuse
   and name `lease --take`. Select only approved work items by computed id; resolve the spec without `--spec` and skip
   a halted spec. A pre-kernel spec selects by recorded `bead_ids`. Update `_NextFlags` if flags change.
2. Make `complete` refuse unless the gate closed `passed` and no child is open. On `raw-close` merge and clear it; on
   another closed unmerged item or `forced-close` refuse with `bd reopen <w>`. A rerun on a merged item exits 0.
   `MINDSPEC_SKIP_PANEL=1` with no passing gate refuses and names it. Run one pass (§13.9).
3. Enforce the bead panel gate for every enrolled spec whatever `panel_presence` says; a pre-kernel spec keeps today.
4. Delete `approve.enrolOnApprove`: `plan approve` enrols every new spec. In the same bead update every existing test
   that runs `plan approve`: a test that pins today's verbs seeds `kerneltest.PreKernelSpec`; a test that pins `plan
   approve` asserts the enrolment. Files: `cmd/mindspec/{approval_gates,bead_ready,lettered_spec,next_selection,
   next_ready_gate,panel_preepic_gate,verb_output}_test.go`, `internal/harness/{lifecycle_scenario,scenario}_test.go`,
   `internal/approve/plan*_test.go`.
5. Add fault and verb tests for AC2 (c), AC3, AC6, AC9 and AC10 below, with a snapshot diff around `next` and
   `complete`. Update workflow "Kernel loop". Text cites no internal record number.

**Verification**
- [ ] `go test ./cmd/mindspec/... -run '^TestS134(Next|Complete|EnrolFault)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/complete/... -run '^TestS134Complete_' -v` passes and lists the new tests.
- [ ] `go test -short ./cmd/mindspec/... ./internal/next/... ./internal/complete/... ./internal/approve/... ./internal/harness/...` passes (includes `_NextFlags`, `TestApprovePlan*` and the lifecycle scenario).

**Acceptance Criteria**
- [ ] AC2 (c): `plan approve` killed after the `<epic>.final` create, foreign links `<epic>.final` blocks a bug blocks
      item 3: `--apply` exits 0 without the item-3-to-final link and flags `foreign-blocker` naming the link. After `bd
      dep remove`, the flag is gone and `--apply` writes the link.
- [ ] AC2 (part): the pass of `complete 1` creates exactly `<2>.impl.a1` and `<3>.impl.a1`.
- [ ] AC3 (part): `next 1` with no `--spec` resolves `x` and prints a token (generation 1); a second `next 1` exits
      non-zero naming `lease --take`.
- [ ] AC6 (part): `panel_presence: warn`, kernel steps force-closed and retyped: `complete` with no passing panel exits
      non-zero. On `PreKernelSpec` `y`, `complete` with no panel warns and exits 0; `next` does not select the foreign
      child.
- [ ] AC9 (verbs): on the halted spec `lease`, `complete`, `next 2`, `panel create 2` exit non-zero, same snapshot.
- [ ] AC10 (part): `complete 1` exits 0 and clears `raw-close`; on `forced-close` it exits non-zero printing `bd reopen
      <1>`; after `bd reopen <1>` and a gate pass it exits 0 and `impl approve` does not name the flag.
- [ ] AC11 (part): `MINDSPEC_SKIP_PANEL=1 complete` with no passing gate exits non-zero and names it.
- [ ] AC13 (part): a snapshot diff around `next` and `complete` changes only computed ids and links between them.

**Depends on**
Bead 3, Bead 7, Bead 8, Bead 10

## Bead 10: Change judge and the re-panel round

**Steps**
1. Add `judge.go`: compute the accepted subject (§13.2) from the passing panel's subject as `panel rebind` refreshes it
   and the last `carry-over`. A commit that changes only a panel directory of the gate does not move it.
2. When the newest subject commit of a passed gate is not the accepted subject, create `<gate>.judge.a<k>` (kind
   `change-judge`, child of the gate's parent) unless the item is merged or the epic is closed. Close an open judge
   `superseded` when its subject is not the newest commit or is the accepted subject; its receipt names the rebind.
3. Extend `submit` for a judgement: `carry-over` with a reason, or `re-panel` with n slots (2 to the passing round's
   slot count) and the changed paths. Check the shape, the old subject, the new subject and the frozen receipt.
4. On `carry-over` add the old subject, the new subject and the judge receipt to the gate receipt. On `re-panel` create
   `<gate>.a<k+1>` with `mindspec_from` and the re-panel record, and authorise an n-slot round whose `run` entry
   carries the paths.
5. Make `panel create` on a re-panelled gate select the highest attempt `<gate>.a<k+1>`, open exactly n slots, record
   the paths as the round's scope and refuse any other `--reviewers` count. Make a successful `panel rebind` run one
   pass. Update workflow "Change judge" and "Panel verbs".

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134Judge_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134(JudgeCmd|Repanel)_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/panel/... ./cmd/mindspec/...` passes (includes `PanelRebind`).

**Acceptance Criteria**
- [ ] AC18 (judge): after a 3-slot pass, a `.mindspec/config.yaml` commit on `bead/1`: `--apply` creates exactly
      `<1>.review.judge.a1` and no round. A second commit before its submit: the next pass closes a1 `superseded` and
      creates a2 on the newest commit; `submit` to a1 exits non-zero.
- [ ] AC18 (a): `carry-over` with a reason keeps the gate `passed`; its receipt names both subjects and the judge
      receipt. A commit of the judgement file creates no judge.
- [ ] AC18 (b): `re-panel` with 2 slots and paths gives `<1>.review.a2` (`mindspec_from` set) and a `panel create 1`
      entry that names the paths; `panel create 1` then registers exactly 2 slots on `<1>.review.a2` with those paths
      as scope, and `panel create 1 --reviewers 6` exits non-zero.
- [ ] AC18 (c): 1 or 4 slots, another value, `carry-over` with no reason, `re-panel` with no paths each exit non-zero,
      step open, no round; 3 slots exits 0 and authorises a 3-slot round.
- [ ] AC18 (d): a successful `panel rebind` closes the judge `superseded` in its pass; the receipt names the rebind.
- [ ] AC13 (part): a snapshot diff around a judge `submit` and `panel rebind` changes only computed ids and links.

**Depends on**
Bead 5, Bead 8

## Bead 11: Skills, guides, setup copies, the ADR amendment and the surface scan

**Steps**
1. Rewrite both copies of `ms-bead-cycle`: loop `next-action --apply`, `lease`, dispatch, `submit` and each `run` entry.
   Remove the spec-wide halt rows of contracts §13.6 from both copies of `ms-bead-cycle` and `ms-spec-autopilot`; keep
   the rows §13.6 keeps. Remove default-3 round text and deleted keys from every skill.
2. Add the old skill text to `historical_skills` so existing installs refresh. Make each `.claude/skills` copy
   byte-equal to its `plugins/mindspec/skills` source. Update the skill pin tests in this bead.
3. Update `project-docs/user/guides/autonomy.md`, `README.md` and `plugins/mindspec/README.md` for the one path and `bd
   ready --exclude-label mindspec`.
4. Add the HC-4 amendment section to ADR-0035 (exit 3, 4, 5 for `next-action`) and its pin test in
   `cmd/mindspec/adr0035_amendment_test.go`.
5. Add `TestS134Surface_` in `cmd/mindspec/surface_scan_test.go`. It diffs from the pinned base commit `371dc360`
   (the spec's base) to `HEAD` and scans each added line of non-test Go files under `cmd/` and `internal/` (not
   `testdata/`) and of `.claude/skills/**` and `plugins/mindspec/skills/**`. Patterns, one per class: ADR
   `ADR-[0-9]{4}`; spec `[Ss]pec[ -]?[0-9]{3}\b`; requirement `\b(AC|OQ)[0-9]+\b` and `\b[RB][0-9]{1,2}\b` not
   preceded by `.` or `"` (slot ids `.R3`, `"R1"` stay legal); section `§`; issue `#[0-9]{2,}\b`; bead id
   `(^|[^A-Za-z0-9.-])mindspec-[a-z0-9]{4}(\.[0-9]+)*($|[^A-Za-z0-9-])`; `issues\.jsonl`. When the base commit is
   absent the test fails and names a full-history fetch; it never skips. Set `fetch-depth: 0` on the test job in
   `.github/workflows/ci.yml`.
6. Update workflow "Setup and skills". Skill, guide and setup text cites no internal record number.

**Verification**
- [ ] `go test ./cmd/mindspec/... -run '^TestS134Surface_|TestADR0035Amendment' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/setup/... ./internal/lint/... ./plugins/mindspec/... ./cmd/mindspec/...` passes (includes `TestReadyCheck_CycleSkillCopiesMatch`, the destructive-guidance and skill pin tests).
- [ ] The AC15 `git grep` over `.claude/skills plugins/mindspec/skills plugins/mindspec/README.md README.md project-docs/user/guides` exits 1.
- [ ] `for s in plugins/mindspec/skills/*/; do n=$(basename $s); cmp $s/SKILL.md .claude/skills/$n/SKILL.md || echo $n; done` prints nothing.

**Acceptance Criteria**
- [ ] AC15: the skills-and-docs `git grep` exits 1. Both `ms-bead-cycle` copies name `next-action --apply`, `lease`
      and `submit`; `cmp` of each skill pair exits 0. After plain `mindspec setup` on an old install, each installed
      skill copy matches its source.
- [ ] AC16: the scan passes on the branch. One planted line per class (`ADR-0035`, `spec 134`, `AC14`, `(R3`, `§8`,
      `#146`, `mindspec-xaoq`, `.beads/issues.jsonl`) makes it fail; `mindspec-owned` and `.mindspec-scratch` do not.
      A missing base commit makes it fail.
- [ ] The ADR-0035 pin test finds exit codes 3, 4 and 5 and goes red without the section.

**Depends on**
Bead 9

## Bead 12: End-to-end kernel path

**Steps**
1. Add `cmd/mindspec/kernel_e2e_test.go`: build the binary, sandbox with `step`, `plan approve` spec `x`, and drive
   only `next-action --apply`, `lease`, `submit` (the test writes verdict files as a fake reviewer) and each `run`
   entry to `impl approve` exit 0 and `"status":"done"`.
2. Keep a foreign note, a foreign bug and a raw claim present for the whole run. Take a snapshot diff around each
   verb: `next-action --apply`, `lease`, `submit`, `next`, `panel create`, `panel submit`, `panel refute`, `panel
   defer`, `complete`, `impl approve`.
3. Produce each AC11 event through its real verb and read the audit table of `impl approve` and its rerun.
4. Run the AC12 pin and AC18 verb cases on enrolled fixtures made by this binary.
5. Update workflow "Kernel loop" with the loop the test drives.

**Verification**
- [ ] `go test ./cmd/mindspec/... -run '^TestS134E2E_' -v` passes and lists the new tests.
- [ ] `go test -short ./cmd/mindspec/...` passes.

**Acceptance Criteria**
- [ ] AC17: only `next-action --apply`, `lease`, `submit` and `run` entries reach `impl approve` exit 0 and
      `"status":"done"`. After `submit <1>.impl.a1`, `run` lists `panel create 1`; after it, `--apply` lists slots
      `<1>.review.r1.R<s>`, `ForBead` counts 1 round, a rerun of `panel create 1` exits non-zero; the export runs.
- [ ] AC13: every listed snapshot diff changes only computed ids and links between them. `bd ready` titles each
      mindspec bead, decisions included, `[mindspec]`; `bd ready --exclude-label mindspec` shows the foreign beads and
      no epic, work item or kernel bead.
- [ ] AC11: one each of `--allow-not-ready`, `--allow-doc-skew`, `--override-adr`, `--allow-net-deletion`, `lease
      --take`, `panel defer`, a decision answer, a repaired unreceipted close, a `carry-over`, and a 3-slot `re-panel`
      after a passing round of 3 slots: `impl approve` and a rerun exit 0 and print 1 each; the re-panel row shows 3
      slots. A producer that writes a record under another key makes the test fail.
- [ ] AC12 (pinned): enrolled with defaults, then `panel.max_rounds: 99` and `panel.lease_ttl: 10m`: round 6 exits
      non-zero, and a new lease records an expiry 2h after its issue time.
- [ ] AC18 (verbs): with a judge open, `complete 1` and `impl approve` exit non-zero naming it. After a carry-over,
      `complete 1` exits 0 and creates `<2>.impl.a1`, `impl approve` names neither gate nor judge, and a pass after each
      creates no judge. After a re-panel pass or a rebind, `complete 1` exits 0.

**Depends on**
Bead 9

## Provenance

Each `-run` pattern runs in the package its bead's Verification names.

| Acceptance Criterion | Bead(s) | Verified By |
|---------------------|---------|-------------|
| AC1 | 4 | `-run '^TestS134NextAction_'` |
| AC2 | 3, 4, 9 | `-run '^TestS134Enrol_'` (enrolment, a, b, d, e, fixture); `-run '^TestS134NextAction_'` (apply); `-run '^TestS134(EnrolFault|Complete)_'` (c, complete pass) |
| AC3 | 5, 8, 9 | `-run '^TestS134Lease'`; `-run '^TestS134PanelSubmit_'`; `-run '^TestS134Next_'` |
| AC4 | 5 | `-run '^TestS134(Submit|SubmitCmd)_'` |
| AC5 | 3, 8 | `-run '^TestS134Enrol_'`; `-run '^TestS134Panel(Create|Refute|Defer)_'` |
| AC6 | 1, 3, 4, 9 | `-run '^TestS134Witness_'`; `-run '^TestS134Enrol_'`; `-run '^TestS134NextAction_'`; `-run '^TestS134(Complete|Next)_'` |
| AC7 | 6, 8 | `-run '^TestS134(Reject|Rounds)_'`; `-run '^TestS134PanelCreate_'` |
| AC8 | 6, 8 | `-run '^TestS134Reject_'`; `-run '^TestS134PanelCreate_'` |
| AC9 | 6, 9 | `-run '^TestS134(Halt|Supersede)_'`; `-run '^TestS134(Next|Complete)_'` |
| AC10 | 5, 7, 9 | `-run '^TestS134Submit_'`; `-run '^TestS134(Closes|ImplApprove)_'`; `-run '^TestS134Complete_'` |
| AC11 | 2, 7, 9, 12 | `-run '^TestS134Friction_'`; `-run '^TestS134Audit_'`; `-run '^TestS134Complete_'`; `-run '^TestS134E2E_'` |
| AC12 | 2, 3, 12 | `-run '^TestS134Config'`; `-run '^TestS134Enrol_'`; `-run '^TestS134E2E_'` |
| AC13 | 1, 3, 4, 5, 7, 8, 9, 10, 12 | `-run '^TestS134Store_'`; `-run '^TestS134Enrol_'`; `-run '^TestS134(Flags|NextAction)_'`; `-run '^TestS134Lease'`; `-run '^TestS134(Closes|ImplApprove)_'`; `-run '^TestS134Panel'`; `-run '^TestS134(Next|Complete)_'`; `-run '^TestS134JudgeCmd_'`; `-run '^TestS134E2E_'` |
| AC14 | 1, 4, 5, 8 | `-run '^TestS134Store_'`; `-run '^TestS134(Flags|NextAction)_'`; `-run '^TestS134Submit_'`; `-run '^TestS134PanelCreate_'` |
| AC15 | 1, 2, 3, 11 | `-run '^TestS134Setup_'`; Bead 2 domains `git grep`; `-run '^TestS134Enrol_'`; the Bead 11 `git grep` and `cmp` lines |
| AC16 | all; 11 | each bead's message tests; `-run '^TestS134Surface_'` with one red mutation per class |
| AC17 | 12 | `-run '^TestS134E2E_'` |
| AC18 | 10, 12 | `-run '^TestS134(Judge|JudgeCmd|Repanel)_'`; `-run '^TestS134E2E_'` |
| Validation proofs | final review | `make test` on the spec branch |
