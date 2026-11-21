---
adr_citations:
    - ADR-0045
    - ADR-0023
    - ADR-0040
    - ADR-0038
    - ADR-0037
    - ADR-0035
    - ADR-0043
approved_at: "2026-11-19T09:06:53Z"
approved_by: user
bead_ids:
    - mindspec-crsz.1
    - mindspec-crsz.2
    - mindspec-crsz.3
    - mindspec-crsz.4
    - mindspec-crsz.5
    - mindspec-crsz.6
    - mindspec-crsz.7
    - mindspec-crsz.8
    - mindspec-crsz.9
    - mindspec-crsz.10
    - mindspec-crsz.11
    - mindspec-crsz.12
    - mindspec-crsz.13
spec_id: 134-governed-autonomy-kernel
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/next/kernel/kernelid/kernelid.go
        - internal/next/kernel/kernelid/witness.go
        - internal/next/kernel/idmode.go
        - internal/next/kernel/store.go
        - internal/next/kernel/records.go
        - internal/next/kernel/clock.go
        - internal/next/kernel/state.go
        - internal/next/kernel/contract.go
        - internal/next/kernel/pass.go
        - internal/next/kernel/lease.go
        - internal/next/kernel/submit.go
        - internal/next/kernel/kerneltest/fixture.go
        - internal/next/kernel/kerneltest/repo.go
        - internal/next/kernel/kerneltest/seed.go
        - cmd/mindspec/next_action.go
        - cmd/mindspec/lease.go
        - cmd/mindspec/submit.go
        - cmd/mindspec/ceremony_guard_test.go
        - internal/phase/phaselock.go
        - internal/bead/config.go
        - internal/harness/sandbox.go
        - internal/doctor/beads.go
        - .beads/config.yaml
        - .github/workflows/ci.yml
        - scripts/ci-install-bd.sh
    - depends_on: []
      id: 2
      key_file_paths:
        - internal/config/config.go
        - cmd/mindspec/config.go
        - internal/redact/redact.go
        - cmd/mindspec/selfemit.go
        - cmd/mindspec/ceremony_guard_test.go
        - internal/panel/rounds.go
    - depends_on: []
      id: 3
      key_file_paths:
        - cmd/mindspec/surface_scan_test.go
        - cmd/mindspec/surface_scan_branch_test.go
        - .mindspec/adr/ADR-0035-agent-error-contract.md
        - cmd/mindspec/adr0035_amendment_test.go
    - depends_on: []
      id: 4
      key_file_paths:
        - .claude/skills/ms-bead-cycle/SKILL.md
        - plugins/mindspec/skills/ms-bead-cycle/SKILL.md
        - .claude/skills/ms-spec-autopilot/SKILL.md
        - plugins/mindspec/skills/ms-spec-autopilot/SKILL.md
        - internal/setup/skills.go
        - project-docs/user/guides/autonomy.md
        - README.md
        - plugins/mindspec/README.md
    - depends_on:
        - 1
        - 2
      id: 5
      key_file_paths:
        - internal/approve/plan.go
        - internal/approve/enrol.go
        - internal/phase/derive.go
        - internal/phase/lineage.go
        - internal/next/mode.go
        - .mindspec/adr/ADR-0023.md
    - depends_on:
        - 1
        - 2
      id: 6
      key_file_paths:
        - internal/next/kernel/reducer.go
        - internal/next/kernel/allowance.go
        - internal/next/kernel/output.go
        - internal/next/kernel/flags.go
        - internal/next/kernel/nextaction.go
        - internal/panel/tally.go
    - depends_on:
        - 1
      id: 7
      key_file_paths:
        - internal/next/kernel/lease.go
        - internal/next/kernel/submit.go
        - cmd/mindspec/lease.go
        - cmd/mindspec/submit.go
        - internal/rules/rules.yaml
    - depends_on:
        - 6
        - 7
      id: 8
      key_file_paths:
        - internal/next/kernel/reject.go
        - internal/next/kernel/halt.go
        - internal/next/kernel/final.go
    - depends_on:
        - 2
        - 6
      id: 9
      key_file_paths:
        - internal/next/kernel/closes.go
        - internal/next/kernel/audit.go
        - internal/approve/impl.go
        - internal/approve/impl_final_gate.go
    - depends_on:
        - 2
        - 6
        - 7
      id: 10
      key_file_paths:
        - cmd/mindspec/panel.go
        - cmd/mindspec/panel_defer.go
        - cmd/mindspec/panel_rebind.go
        - internal/panel/create.go
        - internal/panel/submit.go
        - internal/panel/refute.go
    - depends_on:
        - 6
        - 7
      id: 11
      key_file_paths:
        - internal/next/kernel/judge.go
    - depends_on:
        - 5
        - 6
        - 7
        - 8
        - 9
        - 10
        - 11
      id: 12
      key_file_paths:
        - cmd/mindspec/next.go
        - internal/next/select.go
        - internal/complete/complete.go
        - internal/complete/panel_presence.go
        - internal/approve/plan.go
        - internal/approve/impl_final_gate.go
        - internal/approve/lifecycle_gate.go
        - internal/harness/lifecycle_scenario_test.go
    - depends_on:
        - 8
        - 9
        - 10
        - 11
        - 12
      id: 13
      key_file_paths:
        - cmd/mindspec/kernel_e2e_test.go
    - depends_on:
        - 13
      id: 14
      key_file_paths:
        - internal/next/kernel/kernelid/kernelid.go
        - internal/next/kernel/records.go
        - internal/next/kernel/contract.go
        - internal/next/kernel/output.go
        - internal/next/kernel/pass.go
        - internal/next/kernel/submit.go
        - internal/next/kernel/reject.go
        - internal/next/kernel/reducer.go
        - internal/next/kernel/kerneltest/seed.go
        - internal/next/kernel/testdata/rules.json
        - internal/next/kernel/testdata/output.json
    - depends_on:
        - 14
      id: 15
      key_file_paths:
        - internal/next/kernel/verify.go
        - internal/next/kernel/reject.go
        - internal/next/kernel/final.go
        - internal/next/kernel/allowance.go
        - internal/next/kernel/reducer.go
        - internal/next/kernel/gate_check.go
        - internal/panel/refute.go
        - cmd/mindspec/panel.go
        - cmd/mindspec/panel_defer.go
    - depends_on:
        - 14
        - 15
        - 17
      id: 16
      key_file_paths:
        - internal/next/kernel/judge.go
        - internal/next/kernel/reducer.go
        - internal/next/kernel/submit.go
        - cmd/mindspec/panel_kernel.go
    - depends_on:
        - 14
      id: 17
      key_file_paths:
        - internal/next/kernel/lease.go
        - internal/next/kernel/submit.go
        - internal/next/kernel/closes.go
        - cmd/mindspec/lease.go
        - cmd/mindspec/submit.go
        - cmd/mindspec/next_kernel.go
        - cmd/mindspec/panel_submit.go
        - internal/next/kernel/actor_test.go
        - cmd/mindspec/kernel_actor_test.go
        - cmd/mindspec/ceremony_guard_test.go
    - depends_on:
        - 15
        - 16
      id: 18
      key_file_paths:
        - internal/next/kernel/brief.go
        - internal/next/kernel/reducer.go
        - cmd/mindspec/brief.go
        - cmd/mindspec/panel_kernel.go
        - cmd/mindspec/panel_prompt.go
        - internal/panel/create.go
        - internal/panel/prompt.go
        - cmd/mindspec/ceremony_guard_test.go
    - depends_on:
        - 17
        - 18
      id: 19
      key_file_paths:
        - internal/next/kernel/lens.go
        - internal/next/kernel/reducer.go
        - cmd/mindspec/panel_kernel.go
        - cmd/mindspec/panel_prompt.go
        - internal/panel/create.go
        - internal/panel/panel.go
    - depends_on:
        - 15
        - 17
        - 19
      id: 20
      key_file_paths:
        - internal/next/kernel/audit.go
        - internal/approve/impl.go
        - internal/approve/impl_final_gate.go
    - depends_on:
        - 14
        - 15
        - 16
        - 17
        - 18
        - 19
        - 20
      id: 21
      key_file_paths:
        - cmd/mindspec/kernel_e2e_test.go
        - plugins/mindspec/skills/ms-bead-cycle/SKILL.md
        - plugins/mindspec/skills/ms-spec-autopilot/SKILL.md
        - plugins/mindspec/skills/ms-bead-impl/SKILL.md
        - plugins/mindspec/skills/ms-bead-fix/SKILL.md
        - plugins/mindspec/skills/ms-panel-tally/SKILL.md
        - internal/setup/historical_skills
        - internal/setup/review_discipline_test.go
        - internal/rules/rules.yaml
        - internal/rules/rules_test.go
        - project-docs/user/guides/autonomy.md
---
# Plan: 134-governed-autonomy-kernel

## ADR Fitness

Each touchpoint ADR stays sound; none needs a superseding ADR.

- **ADR-0045, ADR-0040.** Applied. Bead selection, halt rows and the round limit move from skills into the binary
  (Beads 4, 6, 8); `panel.one-fixer` and `lifecycle.complete-by-orchestrator` stop being advisory (Bead 7).
- **ADR-0023.** Applied, and narrowed for enrolled specs. The `step` type, tuple metadata and receipts stay in bd;
  evidence stays in git (Beads 1, 5). Its phase-derivation discriminator (`issue_type == "task"` is lifecycle work)
  gives way, for an enrolled spec only, to recorded work-item ids and kernel ids; a pre-kernel spec keeps it. Bead 5
  adds the amendment note to ADR-0023's discriminator section and states the rule in the core `interfaces.md` section
  "Phase derivation".
- **ADR-0038.** Applied. `--allow-not-ready` joins `escapeHatchFlags` and `EscapeHatchTokens` (Bead 2).
- **ADR-0037.** Applied. The gate legs stay; `panel submit` checks a lease; `panel create` is the one round writer
  (Bead 10).
- **ADR-0043.** Applied. Round counts come from the ledger over every registration (Bead 2). Receipts are bd metadata.
- **ADR-0035.** Amended. HC-4 gains the `next-action` exit codes 3, 4 and 5 (Bead 3, with its pin test). Each new
  refusal keeps one `recovery:` line.

## Decomposition

Thirteen beads, and Beads 14-21 of Amendment 1; one bead is one session. Bead 1 writes first every contract the other beads build against, so most beads
wait only for Bead 1 and run side by side; a later bead amends a contract under "Amendments" (Contracts first). The
critical path is 1 -> 6 -> {8, 9, 10, 11} -> 12 -> 13 (five beads). Kernel code goes in `internal/next/kernel`, which
the workflow manifest owns. Bead 1 is the largest bead: a split adds a chain link. The CI bd install stays in it: its
tests fail without bd, so both land in one merge; alone it is trivial work.

**Parallel waves.** The lane runs up to 4 beads at once. An edge exists only where a bead needs another bead's code to
compile or its behaviour to test.

| Wave | Beads | Why each edge exists |
|------|-------|----------------------|
| 1 | 1, 2, 3, 4 | No edges. 3 and 4 touch only tests, the ADR, skills and docs; 4 names verbs whose flags Bead 1 fixes in the contract. |
| 2 | 5, 6, 7 | 5 -> 1 (store, leaf witness, fixture), 5 -> 2 (`config.RoundLimit`, `panel.lease_ttl`); 6 -> 1 (state, contract), 6 -> 2 (`panel.RoundsForGate` for the allowance); 7 -> 1 (store, state, the `Pass` dispatcher). 7 does not need 6: `submit` runs the Bead 1 dispatcher (Contracts first). None merges before the integration gate. |
| 3 | 8, 9, 10, 11 | 8 -> 6 (pass runner), 8 -> 7 (halt retry through `submit --answer`); 9 -> 6 (flags in the pass), 9 -> 2 (registry token in the audit); 10 -> 6 (pass, allowance), 10 -> 7 (close as `submit` does), 10 -> 2 (round counter); 11 -> 6 (pass), 11 -> 7 (judgement check in `submit`). 10 does not need 8: its tests seed authorised rounds and decisions with `kerneltest.Authorise` and `OpenDecision`. |
| 4 | 12 | 12 -> 5 (enrolment on), 12 -> 6 (pass), 12 -> 7 (lease function), 12 -> 9 (`raw-close` and `forced-close` in `complete`), 12 -> 11 (`kernel.AcceptedSubject` for the `complete` and final-gate subject); 12 -> 8, 10: enrolment turns on only after every kernel rule and verb has merged. |
| 5 | 13 | 13 drives every verb end to end: 8 (decisions, final gate), 9 (audit), 10 (panel verbs), 11 (judge), 12 (`next`, `complete`). |

**Contracts first.** Bead 1 adds the types and entry points that later beads fill: the record types, `State` and its
reader, the output types of contracts §4, the rule registry, the artifact-check registry of `submit`
(`RegisterArtifactCheck`), `LeaseDeps`, and the entry points `NextAction`, `Lease`, `Submit` and `Answer`, each in its
own file and returning `ErrNotBuilt` until its bead fills it. **Rule registry.** A registry is a value:
`kernel.NewRegistry(names...)` fixes its own order, and its `Register(name, mode, fn)` refuses an unlisted or repeated
name. `DefaultRegistry` is `NewRegistry` over the names `contract.go` lists; `RegisterRule` registers into it, and rule
beads fill it through `init` in their own files. `Pass(reg, state, apply)` in `pass.go` is the real dispatcher: it runs
the rules of `reg` in that registry's order, never in registration order, and with `apply` writes their result (receipts
before closes, `WithCASRetry` on each status write). Each rule reads `State` with the results of the earlier rules of
this pass applied in memory, with and without `apply`; `Pass` returns its write list in both modes, and with `apply`
it writes the same list that the read-only pass returns. On an empty registry it writes nothing; it returns every rule
and store error. Entry points and verbs call `Pass(kernel.Rules(), ...)`; `Rules()` is `DefaultRegistry` unless
`kerneltest.WithRules(t, reg)` replaces it for one test (restored by `t.Cleanup`; no `t.Parallel`). Bead 1's dispatcher
tests use their own `NewRegistry`, so later rules do not change them; Bead 7's failing rule and Bead 10's counting rule
use `WithRules`. A test over the default registry asserts only writes and output that no later rule bead changes.
**Modes.** A rule's mode is `ordinary`, `halt` or `flag`. A `flag` rule returns flags only; a write from it makes `Pass`
return an error. When `State.Halted()`, `Pass` runs only `halt` and `flag` rules, in order, reports `halted`, and with
`apply` writes only the `halted` closes and receipts of `halt` rules; it creates no bead. Flags are read-only, so a
halted read shows every flag. Otherwise `Pass` runs every rule. **Rule list.** Bead 1 owns the list in `contract.go`:
each name with its mode, in this order (owner in brackets). `ordinary`: `recreate` (6), `missing-link` (6), `tally` (6),
`supersede` (8), `refused-round` (8), `final-round` (8), `judge-apply` (11), `judge-supersede` (11), `judge-create`
(11), `round-open` (6), `slots` (6), `implement` (6), `retry-attempt` (9), `complete-entry` (6). `halt`: `halt` (8).
`flag`: `run-marker-missing`, `foreign-child-open`, `foreign-blocker`, `id-collision` (6), `unreceipted-close`,
`raw-close`, `forced-close`, `raw-claim` (9). (Amendment 1: Bead 14 replaces this list.) A rule bead registers a listed name with its listed mode. **Seeders.**
Bead 1 adds the `kerneltest` seeders `PassGate`, `Authorise` (an authorised round: round, subject, slot count, paths),
`OpenDecision`, `Answer`, `Register`, `Repaired` (a repaired receipt) and `Judge` (an open change judge: parent the work
item, tuple naming the gate, subject commit). The bead that builds a producer pins the seeder with
`TestS134SeedMatch_<seeder>[<case>]`: real producer vs seeder on twin fixtures, normalising only bead ids, wall-clock
times and token hashes and comparing every other metadata key and value; on a mismatch the bead corrects the seeder.
Pins: `Enrolled` Bead 5; `PassGate` Bead 6; `Authorise` each producer: Bead 6 (closed implement step, closed fix), Bead
8 (supersede, final round 1), Bead 11 (`re-panel`); `Answer`, `Repaired` Bead 7; `OpenDecision` (reject and limit kinds)
Bead 8; `Register` Bead 10; `Judge` Bead 11. Bead 6's round opening is the one writer of the authorisation on a fix
close; Bead 8 creates the fix only.
**Amendments.** Bead 1 writes these contracts first; they are not frozen. A later bead may amend Bead 1's contract files
(`pass.go`, `contract.go`, `records.go`, `state.go`, `store.go` and the `kerneltest` seeders) when one of its own tests
exposes a gap in them. The amendment keeps every earlier bead's tests green, and its report names each amendment. It may
extend an earlier bead's tests (golden values, new rows) but never removes or weakens an assertion; its report names
each edited test. A rule add or reorder updates Bead 1's golden rule list in the same commit. Every bead, amending or
not, merges only after the cycle (not the implementer) rebases it onto the current spec branch and runs `go test -short
./...` there, just before `complete`; so beads of one wave merge in sequence, and a later one runs on the earlier ones'
changes. This adds no dependency edge. Bead 1 owns the rule list order and modes: an amendment that adds or reorders a
rule says why in its report and keeps the order the contracts imply (`recreate` first; `tally` before `refused-round`
and `final-round`; `supersede` before `round-open`; `judge-supersede` before `judge-create`; `judge-create` and
`retry-attempt` before `complete-entry`) and each listed mode.

**Shared files.** `cmd/mindspec` registers each verb in its own file; Bead 1 adds `next_action.go`, `lease.go` and
`submit.go` with their final flags and the `_HelpFlags` and `HelpGolden` lines, so no later bead edits those pins.
Beads 1 and 2 both edit `cmd/mindspec/ceremony_guard_test.go`, in different test functions (`_HelpFlags`,
`_ConfigKeys`). A domain doc that two beads of one wave edit gets two separate sections; the bead that merges second
rebases and keeps both.

**Package boundary.** The leaf `internal/next/kernel/kernelid` imports the standard library only and holds the id
grammar and the one witness check; `internal/phase` and the kernel import it, and no other copy exists. The kernel
imports `kernelid`, `bead`, `phase`, `config`, `panel` and leaf packages only, never `internal/next`, `approve`,
`complete` or `cmd/`. `internal/panel` stays a leaf (`TestPanelLeafImports_StdlibPlusTermsafeOnly`). The kernel takes
the worktree helper as `kernel.LeaseDeps.EnsureWorktree`; `cmd/mindspec` wires `next.EnsureWorktree` into it and runs
the pass for the panel verbs. `TestS134Boundary_` (Bead 1) fails on a forbidden non-test import of `kernelid` or the
kernel.

**Pre-kernel path until Bead 12.** Bead 5 builds enrolment behind the unexported package variable
`approve.enrolOnApprove` (default false). Its tests set it in-process; no verb, flag, env var or key reads it. Until
Bead 12 deletes the variable, `plan approve`, `next`, `complete`, `impl approve` and the panel verbs keep today's path
for every real spec, so a spec-branch binary enrols nothing and the existing lifecycle tests stay green. Every new
refusal in Beads 1 and 5-11 applies only when the witness says the spec is enrolled; those beads test on the
`kerneltest` fixture. Bead 12 deletes the variable, so `plan approve` enrols every new spec, and updates the existing
tests in the same bead.

**Tracker housekeeping.** The spec's Scope tracker items are for the orchestrator at the final review, not a bead.

**Amendment 1 (operator rulings of 2026-11-21).** Beads 14-21 add the verify step, the judge at a tally, late
submissions, repair receipts, the binary-rendered briefs and the lens step (contracts B8-B10, §4-§7, §9, §11,
§13.2, §13.7, §13.8, §13.10, §13.11). They run after Bead 13 on the merged kernel and change its code and tests in
place; the earlier beads' text stays as written, and the notes "Amendment 1" in Beads 6, 8, 9, 10, 11 and 13 name what
the new beads replace. Bead 14 updates the rule list to this order (owner in brackets). `ordinary`: `recreate` (6),
`missing-link` (6), `judge-apply` (11, 16), `tally` (6, 16), `refused-round` (8, 15, 16), `verify-apply` (15, 16),
`final-round` (8), `judge-supersede` (11), `judge-create` (11, 16), `round-open` (6), `lens-create` (19), `slots`
(6), `implement` (6), `retry-attempt` (9, 17), `complete-entry` (6). `supersede` is removed. `halt` and the `flag`
rules do not change. Order constraints: `recreate` first; `judge-apply` before `tally`; `tally` before
`refused-round`, `verify-apply` and `final-round`; `judge-supersede` before `judge-create`; `judge-create` before
`round-open`; `round-open` and `final-round` before `lens-create`; `lens-create` before `slots`; `judge-create` and
`retry-attempt` before `complete-entry`. Bead 16 adds the moved-subject guard (§13.2) to `tally`, `refused-round`,
`verify-apply` and the `panel refute` and `panel defer` retally. The ruleset pin stays `kernel-1`: no release carries the earlier list.
Seeders: Bead 14 adds `Verify` (a closed verify with its receipt and returns) and `Lens` (a closed lens step with its
record); Bead 15 pins `Verify`; Bead 16 pins `Authorise` on a verify that leaves no reproduced finding under a
`re-panel` and `Judge` on an open gate; Bead 19 pins `Lens`.

| Wave | Beads | Why each edge exists |
|------|-------|----------------------|
| 6 | 14 | 14 -> 13: it changes the merged contracts and goldens. |
| 7 | 15, 17 | 15 -> 14 (kinds, records, rule list); 17 -> 14 (lease and receipt records). |
| 8 | 16 | 16 -> 15 (a refused tally under a judgement creates the verify), 16 -> 17 (the subject checks in `submit.go` and the retry keys). |
| 9 | 18 | 18 -> 15 (verify and fix briefs read the verify step metadata), 18 -> 16 (change-judge brief reads the open-gate judge). |
| 10 | 19 | 19 -> 18 (the slot body shows the lens), 19 -> 17 (lens retries and the generation rule). |
| 11 | 20 | 20 -> 15, 17, 19 (it counts their records). |
| 12 | 21 | 21 drives every new verb end to end. |

## Testing Strategy

- **Test names.** Each new test starts with `TestS134<Mechanism>_`; no base-tree test matches `S134`. Each
  Verification line with a `TestS134` prefix must list its new tests in `go test -v` output. No `TestS134` test reads
  `testing.Short()`.
- **Green between beads.** Each bead's Verification runs `go test -short` on the whole of every package it touches,
  plus the named pin tests. A bead builds and keeps `go test -short ./...` green on its own merge, in any order its wave
  allows: before each `complete`, the cycle rebases the bead onto the current spec branch and runs `go test -short
  ./...` there ("Amendments", Contracts first). A bead that changes a pinned surface updates the pin test in the same
  bead. Pins: `TestCeremonyNonInflation_ConfigKeys`, `_HelpFlags`, `_NextFlags` (`cmd/mindspec/ceremony_guard_test.go`);
  `TestReadyCheck_CycleSkillCopiesMatch` (`cmd/mindspec/bead_ready_test.go`);
  `internal/lint/destructive_guidance_test.go`; `internal/setup/{readiness_skill_pin,review_discipline,skills}_test.go`;
  `internal/bead/config_test.go`; `internal/bootstrap/bootstrap_test.go`; `internal/doctor/beads_test.go`;
  `TestApprovePlan*`; `internal/harness/lifecycle_scenario_test.go`.
- **Real bd.** `TestS134` tests that need bd call `kerneltest.NewRepo(t)`: a scratch git repo, `bd init` and
  `EnsureBeadsConfig`. It calls `kerneltest.RequireBD(t)`, which fails (never skips) when `bd` is absent or `bd
  --version` is not 1.3.4, and it fails when `bd init` fails. These tests do not use `harness.NewSandbox`, which skips
  under `-short` with no `bin/mindspec` and only logs a missing bd. Binary tests build the binary with
  `buildMindspecBinary`. From Bead 1, `NewSandbox` writes `types.custom: "gate,step"` through `EnsureBeadsConfig`.
- **bd in CI.** The CI `test` job runs `go test -short -race ./...` and today has no bd, so the existing real-bd tests
  skip under `-short`. Bead 1 adds `scripts/ci-install-bd.sh <dir>`: it downloads
  `https://github.com/gastownhall/beads/releases/download/v1.3.4/beads_1.3.4_linux_amd64.tar.gz`, checks its SHA-256
  `175c26904bde3ff9168d2282ff6b56f17b17f60c22fa5acb3dacc9fb3465303a` (the release `checksums.txt` line), installs `bd`
  to `<dir>` and runs `bd --version | grep -F 1.3.4`. The `test` job runs it before the tests with `$RUNNER_TEMP/bin`
  and adds that dir to `GITHUB_PATH`. After the check it prints `bd-installed: ` and the `bd --version` output. The
  asset holds one glibc ELF binary; `ubuntu-latest` has glibc. Bead branches are never pushed. Bead 1's bead-level check is local: `go build ./...` and, with bd 1.3.4 first on `PATH`
  and no `bin/mindspec` (the CI `test` job's state), `go test -short -race ./...`.
- **Integration gate.** About 15 bd-gated test files first run in Linux CI. After Bead 1 merges, the orchestrator
  pushes the spec branch (opening the spec PR as a draft if none exists). This mid-spec push is an exception, granted
  by the operator for this spec, to the one-push-at-end-of-spec rule of `AGENTS.md`. Before any wave-2 bead merges,
  the run for the pushed commit passes three checks: `gh run list --workflow ci.yml --branch
  spec/134-governed-autonomy-kernel --json databaseId,headSha,conclusion` names a run whose `headSha` is the pushed
  commit; `gh run view <id> --json jobs` shows `conclusion: success` for each `test` job; and
  `gh run view <id> --log | grep -E 'bd-installed: .*1\.3\.4'` matches. Wave-2 beads may be implemented and reviewed
  meanwhile. A red run is repaired by a fix bead or a Bead 1 follow-up commit on the spec branch first.
- **Fake bd.** Fault tests put a script named `bd` first on `PATH`, as `nextHonestyFakeBD`
  (`cmd/mindspec/next_selection_test.go`) and `installFakeBD` (`cmd/mindspec/verb_output_test.go`). A forwarding
  wrapper logs argv, calls real bd and injects a cycle refusal or exit 13 on a named write. When a mindspec binary
  subprocess calls bd, the wrapper kills it with `kill -9 $PPID`. No test starts a model CLI.
- **Kill in-process (Bead 5).** Before Bead 12 enrolment runs only in-process. `enrol.go` has an unexported stage hook
  (`enrolStageHook func(stage string)`, default no-op) called after each write group of contracts §8. A kill test
  re-runs the test binary (`os.Args[0] -test.run '^TestS134EnrolHelper$'`, env `S134_KILL_AT=<stage>`); the helper
  returns at once when `S134_KILL_AT` is unset, so the plain `-run '^TestS134Enrol'` run writes nothing through it.
  With the variable set, the helper sets `enrolOnApprove`, sets the hook to `os.Exit(137)` at that stage and runs
  `plan approve`. The process dies with no deferred cleanup, as a real kill does. The parent reads the store and runs
  the rerun in-process.
- **Kill by seeding (Beads 6-11).** A test that needs the state a kill leaves (a receipt with no close, a step with no
  link) seeds it through the store and record types, then reruns. The kernel has no kill seam.
- **bd seams.** The kernel store runs bd through a runner it is given (default `bead.RunBD`). `enrol.go` gives it the
  approve package's `planRunBDFn`, so `approve.SetPlanRunBDForTest` covers enrolment writes; `phase.SetRunBDForTest`
  covers derivation reads.
- **Clock.** Every lease, expiry, `retry_at` and timed answer reads `kernel.Now` (Bead 1; `kerneltest.SetClock`).
  Binary tests never wait; they read the recorded expiry.
- **Shared fixture.** Bead 1 adds `kerneltest`: `Enrolled` (spec `x`, items 2 and 3 on item 1, built through the
  store with the record types, in the final shape of contracts §8: items, `mindspec_links`, plan-order links,
  `mindspec_run`, approval record, gates and the links to `<epic>.final`), `PreKernelSpec` (the v0.19.0 shape: epic,
  items and plan links, no witness), `Snapshot` (the spec's snapshot, compared byte for byte), the forwarding fake bd,
  the id resolver `ID` and the seeders (pinned as Contracts first says). A bead that corrects a seeder follows
  "Amendments" (Contracts first).
- **Ids in tests (OQ1).** Bead 1 commits the probe result as `kernel.NativeIDs`; later beads read only that constant
  and run on that branch. Every `<computed id>` in a bd argv or assertion is `kerneltest.ID(t, <computed id>)`: the
  computed id with native ids, else the bd id whose `mindspec_key` is the computed id. Fallback forms: AC14's foreign
  `bd create --id <1>.review.r1.R1` becomes a foreign `bd create --metadata '{"mindspec_key":"<1>.review.r1.R1"}'`
  with no tuple, and `panel create 1` still prints `id-collision`; AC14's foreign `<1>.review.1` is a foreign `bd
  create --parent ID(<1>.review)`; AC13's "computed ids" are the beads with a valid key and tuple; a fake-bd link
  write is a `dep add` whose ids both resolve to computed ids; "`<slot>.a2` exists" is `Lookup(<slot>.a2)` found.
- **Record shapes.** (Amendment 1: Bead 14 adds the records of contracts §5, §6, §9, §13.7, §13.10 and §13.11; no
  slot closes `superseded` by a new subject.) Bead 1 defines every kernel metadata format as a named Go type and key constant in
  `internal/next/kernel/records.go`: tuple, `mindspec_key`, `mindspec_close` (receipt: kind, path, commit, blob, plus
  `repaired`, `rebind` and `carry_overs`), `mindspec_from`, `mindspec_links`, `mindspec_run`, lease (generation,
  expiry, token hash, claim base), gate round (round, subject, slot ids), take reason, decision answer, defer,
  judgement, and re-panel (slot count, paths). Producers write and the audit reads only through these types. A golden
  JSON test pins each shape. **Receipt validity**, per return kind, has one check, `kernel.ValidReceipt`: a close
  with an artifact (`submit` or `panel submit` of an implement step, slot, fix or judge) is valid when it names path,
  holding commit and blob id and they resolve. A valid `submit` of an implement step or fix writes return kind
  `passed`. A close the pass itself writes with no artifact (a slot attempt closed
  `superseded` at the round N+1 authorisation, a step or decision closed `halted`, a fix or limit decision closed `superseded` by a
  passing refute or defer retally, a judge closed `superseded`, a gate closed by the tally) carries a receipt of its
  kind with its reason fields (the new subject, the halt answer, the retally, the rebind) and no artifact fields; that
  receipt is valid. A decision answer's receipt holds the answer record and is valid. `ValidReceipt` decides only
  receipt validity, not authorisation: `unreceipted-close` uses it alone; the round opening also needs a close
  `passed` under an open gate (Bead 6).
- **Snapshot diffs.** With a foreign note, a foreign bug and a raw claim present, a snapshot diff around each new or
  changed verb changes only computed ids and links between them (Beads 6, 7, 10, 11, 12; all verbs in Bead 13).
- **Surface rule (AC16).** "Internal record number" = an ADR, spec, requirement (R, B, AC, OQ id), section sign, bead
  or issue number. No added non-test Go line (comments too), CLI text or skill line holds one or names
  `.beads/issues.jsonl`. A changed line counts as added: a bead removes the reference from each line it changes.
  Bead 3 adds the scan. It constrains only this spec's lines, so it is not in plain `go test ./...`: the branch run
  is behind build tag `surfacescan`. The permanent guard for skill and embedded text stays
  `TestProductSurface_NoInternalRefs` (`internal/setup/product_surface_test.go`); each bead that edits that text keeps
  it green.
- **Doc sync.** Each bead that changes `internal/**` or `cmd/**` updates a domain doc section of each owning domain
  (core: `internal/{config,phase,redact}`; execution: `internal/{bead,harness,executor}`; workflow: the rest), because
  the released `complete` refuses code with no domain-doc change. Each bead edits only its own named section. Section
  owners: core `interfaces.md`: "Phase lock" (1), "Config keys" (2), "Phase derivation" (5). execution
  `interfaces.md`: "bd types and sandbox" (1). workflow `interfaces.md`: "Kernel store and ids" (1), "Config show and
  friction" (2), "Round counter" (2), "Surface scan" (3), "Setup and skills" (4), "next-action" (6), "lease and
  submit" (7, 17), "Close flags and audit" (9, 20), "Panel verbs" (10), "Briefs" (18), "Lens step" (19); "Kernel
  store and ids" also 14. workflow `architecture.md`: "Enrolment" (5), "Rounds, decisions and halt" (8), "Change judge"
  (11, 16), "Verify step" (15). workflow `runbook.md`: "Kernel loop" (12, then 13, then 21).
- **Dogfood boundary.** The lane runs lifecycle and panel verbs on the released binary, never the spec-branch binary.
  The lane session runs in a worktree and loads `.claude/skills` from it, so after Bead 4 merges its `ms-bead-cycle`
  and `ms-spec-autopilot` name verbs the released binary lacks. The lane kit drives beads by its own protocol and does
  not invoke either skill for this spec.
- **Gate.** The final review runs, from the spec worktree, `make test`; `git fetch origin main` then
  `MINDSPEC_SURFACE_BASE=$(git merge-base HEAD origin/main) go test -tags surfacescan ./cmd/mindspec/ -run
  '^TestS134SurfaceBranch_' -v` (after main is merged into the spec branch, the merge base is that main commit, so
  main's lines are not scanned); and the AC15 and AC16 skill checks. It checks that the spec PR's CI `test` job is
  green with its bd step printing 1.3.4.

## Bead 1: Kernel foundation and contracts

**Steps**
1. Add `step` to the required `types.custom` set. `bead.EnsureBeadsConfig` treats `types.custom` as a set: it adds
   `gate` and `step` when missing and keeps every other value, `--force` included; it replaces no value of a key
   outside the required set. `doctor` reports a missing member. `harness.NewSandbox` calls `EnsureBeadsConfig` after
   `bd init`. Add `step` to the repo's `.beads/config.yaml`. `kernel.RequireStepType(root)` reads `types.custom` from
   `.beads/config.yaml` with the `bead` package reader; its refusal names `mindspec setup`.
2. Probe bd (OQ1). In a sandbox, run `bd create --id <epic>.1.impl.a1 --parent <epic>.1 --type step`, and the same for
   the `.review`, `.final`, `.decide.` and `.judge.` shapes. Commit the result as `kernel.NativeIDs` in `idmode.go`,
   with the bd version and the observed output in its doc comment. `TestS134Probe_` runs the probe on real bd and
   fails when the result differs from the constant.
3. Add the leaf `internal/next/kernel/kernelid` (standard library imports only). It holds the id grammar (format and
   parse each contracts §2 kind with the optional `.a<k>` suffix, and the exact-match test for "kernel-shaped"; a
   bd-numbered child such as `<w>.review.1` is not kernel-shaped) and the one witness check `kernelid.Enrolled`. Its
   input is the epic's `mindspec_run` presence, the ids and `mindspec_key` values of the epic subtree, and the spec's
   `panel.json` `orchestrated` flag (read by `kernelid.PanelOrchestrated(specDir)`). It returns enrolled and
   marker-missing.
4. Add `records.go` (the record shapes of the Testing Strategy), `clock.go` (`kernel.Now`), `DefaultPolicy` (round
   limit 5, lease TTL 2h) and `store.go` over a given bd runner. `Create` takes a computed id, type (`step` or bd's
   `decision`), parent, tuple, label `mindspec` and a `[mindspec]` title prefix. With `NativeIDs` false, bd assigns the
   id, `mindspec_key` holds the computed id and `Lookup` scans the epic subtree. Add `SetStatusIf` (bd `--if-status`;
   exit 13 is `ErrCASMismatch`), `WithCASRetry` (read again, run once more, a second mismatch refuses; every kernel
   status write uses it), `WriteReceipt`, `AddLink` (computed ids only) and `Collision` (`id-collision`).
5. Add `state.go`: read the epic subtree by computed and recorded ids (`mindspec_links`), the git refs it names, the
   panel ledger and verdicts, `mindspec_run` (or `DefaultPolicy` with marker-missing) and `kernel.Now`; expose open
   decisions, answers and `Halted()`. Add the contracts of "Contracts first": output types, both registries
   (`NewRegistry`, `DefaultRegistry`, `RegisterRule`, `Rules`), `LeaseDeps`, the `Pass` dispatcher in `pass.go`, and the
   entry points in `nextaction.go`, `lease.go` and `submit.go`. Add `cmd/mindspec/next_action.go`, `lease.go` and
   `submit.go`: each registers its verb with its final flags, resolves the spec, refuses a pre-kernel spec naming
   today's verbs and calls its entry point. Update `_HelpFlags` and `HelpGolden`.
6. Export `phase.WithPhaseLock(root, fn)` over `acquirePhaseLock`. Add `kerneltest` (`Enrolled`, `PreKernelSpec`,
   `Snapshot`, `ID`, `SetClock`, fake bd, `NewRepo`, `RequireBD`, `WithRules`, and the seeders `PassGate`, `Authorise`,
   `OpenDecision`, `Answer`, `Register`, `Repaired`, `Judge`) and `TestS134Boundary_` (Package boundary). Add
   `scripts/ci-install-bd.sh` and its CI step (Testing Strategy "bd in CI").
7. Update the doc sections core "Phase lock", execution "bd types and sandbox", workflow "Kernel store and ids", and
   the existing tests that pin `types.custom: "gate"`.

**Verification**
- [ ] `go test -short ./internal/next/kernel/... -run '^TestS134(Probe|IDs|Records|Store|Witness|State|Contract|Dispatch|Seed|Boundary)_' -v` passes and lists the new tests (none skips under `-short`).
- [ ] `go test ./internal/bead/... ./internal/doctor/... ./cmd/mindspec/... -run '^TestS134(Setup|Doctor|VerbStub)_' -v` passes and lists the new tests.
- [ ] `PATH=<a dir with git and no bd> go test -short ./internal/next/kernel/... -run '^TestS134Probe_'` exits non-zero naming bd.
- [ ] `go test -short ./cmd/mindspec/... ./internal/harness/... ./internal/bead/... ./internal/doctor/... ./internal/phase/... ./internal/setup/... ./internal/bootstrap/...` passes (includes `_HelpFlags`, `HelpGolden`).
- [ ] `go build ./...` exits 0. With bd 1.3.4 first on `PATH` and no `bin/mindspec`, `go test -short -race ./...` passes.
- [ ] `go list -deps ./internal/next/kernel/kernelid | grep mindspec` prints only `.../internal/next/kernel/kernelid`.
- [ ] After the merge, the integration gate (Testing Strategy) passes before any wave-2 bead merges.

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
      `gate,spike2,step`, and so does plain `mindspec setup` on that repo. A writer that replaces the value under
      `--force` fails a test. `doctor` reports `step` missing.
- [ ] AC6 primitive: `kernelid.Enrolled` finds each of the three witnesses alone, a kernel id held only in
      `mindspec_key` included; with none it reports pre-kernel; with a witness and no `mindspec_run` it reports
      marker-missing.
- [ ] AC6 (part): `next-action y`, `lease` and `submit` on `PreKernelSpec` each exit non-zero naming today's verbs,
      same snapshot.
- [ ] `State` read from `Enrolled` lists items 1-3, the gates and the plan links; a foreign `task` under the epic is
      foreign. Each seeder writes only through the record types; a golden test pins each seeded record.
- [ ] Each record type round-trips to its golden JSON; a renamed key fails the test.
- [ ] Pass receipts: a golden test pins one pass-written receipt of each kind (slot, fix and judge closed `superseded`,
      a step closed `halted`, a gate closed by the tally), each with its reason fields and no artifact fields;
      `ValidReceipt` accepts each. It refuses a `submit` slot receipt with no blob id, one whose holding commit
      does not exist, and one whose blob id does not match the path at its holding commit. A check that requires
      path, commit and blob on every kind fails the test. A golden test pins the receipt a `submit` of an implement
      step writes: kind `passed`, path, commit, blob.
- [ ] Every dispatcher test runs `Pass` on its own `NewRegistry` with test names, never on `DefaultRegistry`. `Pass` on
      an empty registry writes nothing (same snapshot) and reports `halted` on a seeded `halt` answer; a test rule's
      create and close are written receipt first; a rule error and a bd error each come back from `Pass`.
- [ ] Registry: `Register` of a name outside the registry's list, or of a name twice, returns an error; `RegisterRule`
      of a name `contract.go` does not list, or with a mode other than its listed mode, returns an error. A golden test
      pins the `contract.go` names, order and modes of Contracts first. `WithRules` swaps `Rules()` for one test and
      restores `DefaultRegistry` after it.
- [ ] Order: two test rules registered in the reverse of their fixed order run in the fixed order; a dispatcher that
      runs rules in registration order fails the test.
- [ ] Visibility: test rule A creates a step; a later test rule B finds that step in its `State` and a test rule before
      A does not, with and without `apply`. The write list `Pass` returns without `apply` equals the writes that `Pass`
      with `apply` makes (snapshot diff). The same holds when rule A closes a step with its receipt and writes gate
      metadata (an authorised round): rule B sees the close and the metadata in both modes, and the read-only write
      list equals the writes with `apply`. A dispatcher that gives every rule the input `State`, or that overlays only
      creates, fails the test.
- [ ] Halt mode: on a seeded `halt` answer, an `ordinary` test rule that creates a step is not called and the snapshot
      shows no create; a `halt` test rule is called and its `halted` close is written; a `flag` test rule is called and
      its flag is in the output. On a state that is not halted all three are called. A `flag` rule that returns a write
      makes `Pass` return an error and write nothing.
- [ ] Boundary: a planted kernel import of `internal/next`, or a planted `kernelid` import of `internal/phase`, makes
      `TestS134Boundary_` fail. CI provisions bd: the `test` job runs the checksummed install before `go test`.

**Depends on**
None

## Bead 2: Config honesty, registries and the round counter

**Steps**
1. Delete `type Loop`, `LoopContext` and every `loop:` default in `internal/config/config.go`. `Load` keeps a
   non-strict decode of the removed keys and warns once per removed key that is set.
2. Add `panel.max_rounds` (default 5) and `panel.lease_ttl` (default `2h`). Add `config.RoundLimit()`: it reads
   `panel.max_rounds`; when that is unset it reads `loop.halt.max_rounds_per_bead` and the warning names the value.
3. Rewrite the `config show` render in `cmd/mindspec/config.go`. Each key has one truthful label. Removed keys show as
   ignored. `runner:` shows as read by a skill. Update `TestCeremonyNonInflation_ConfigKeys` and
   `internal/config/config_test.go` to the new key set in this bead.
4. Add `allow-not-ready` to `escapeHatchFlags` (`cmd/mindspec/selfemit.go`) and to `redact.EscapeHatchTokens`.
5. Add `panel.RoundsForGate` in `internal/panel/rounds.go`, the one round counter: rounds of one gate over every
   registration of its work item, of every `final_review` registration, or of a spec's `spec_approve` or
   `plan_approve` registrations, abandoned included, from the ledger. Redispatch adds no round. `internal/panel` stays a leaf.
6. Update core "Config keys", workflow "Config show and friction" and "Round counter". Remove `type Loop struct` and the
   removed key names from `.mindspec/domains/**`. Surface text cites no internal record number.

**Verification**
- [ ] `go test ./internal/config/... -run '^TestS134Config_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134ConfigShow_|^TestS134Friction_' -v` passes and lists the new tests.
- [ ] `go test ./internal/panel/... -run '^TestS134Rounds_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/config/... ./internal/redact/... ./internal/panel/... ./cmd/mindspec/...` passes (includes `TestCeremonyNonInflation_ConfigKeys`, `ForBead` and `TestPanelLeafImports_StdlibPlusTermsafeOnly`).
- [ ] `git grep -n -E "max_rounds_per_bead|panel_deadlock_rounds|max_consecutive_impl_failures|gate_authority|on_reject|type Loop struct" -- .mindspec/domains` exits 1.

**Acceptance Criteria**
- [ ] AC12 (config part): `loop.gate_authority`, `loop.halt.on_reject: fix`, `loop.halt.panel_deadlock_rounds` and
      `loop.halt.max_consecutive_impl_failures` load with exit 0 and one warning each. `config show` labels them ignored
      and labels `runner:` read by a skill.
- [ ] AC12 (alias): `loop.halt.max_rounds_per_bead: 4` gives `RoundLimit()` 4; with `panel.max_rounds: 6` it gives 6.
- [ ] AC11 (registries): both registries hold `allow-not-ready`; a journal event with that token is kept.
- [ ] AC15 (domains): the `.mindspec/domains` `git grep` exits 1. A loader that forces `on_reject: halt` fails a test.
- [ ] AC7 (counter): 3 rounds on slug A, `panel abandon` A, 2 on slug B: `RoundsForGate` gives 5 for the gate; the
      same holds for `final_review` registrations and for `spec_approve` and `plan_approve` registrations; a redispatch
      adds no round.

**Depends on**
None

## Bead 3: Surface scan and the ADR-0035 amendment

**Steps**
1. Add the HC-4 amendment section to ADR-0035 (exit 3, 4, 5 for `next-action`) and its pin test in
   `cmd/mindspec/adr0035_amendment_test.go`.
2. Add the surface scan in two files. Untagged `cmd/mindspec/surface_scan_test.go` holds the matcher and the diff
   function and their tests (`TestS134SurfaceMatcher_`, `TestS134SurfaceDiffFn_`, temp repos only). Tagged
   `cmd/mindspec/surface_scan_branch_test.go` (`//go:build surfacescan`) holds `TestS134SurfaceBranch_`. Plain `go
   test ./...` does not build the tagged file, so the scan never runs on main or a later spec.
3. The diff function takes a base, a head and a main ref. It scans each added or changed line of non-test Go files
   under `cmd/` and `internal/` (not `_test.go`, not `testdata/`) and of `.claude/skills/**` and
   `plugins/mindspec/skills/**`. The base is `371dc360` (the spec's base) unless `MINDSPEC_SURFACE_BASE` sets it. The
   base must be an ancestor of the head, must not be the head, must have `371dc360` as an ancestor (or be it) and must
   be reachable from `origin/main`; otherwise the test fails and names the rule. A missing base or a missing
   `origin/main` fails and names `git fetch origin main` or a full-history fetch; it never skips.
4. Use RE2 (Go `regexp`); check context characters in code. Classes: ADR `\bADR[- ]?[0-9]{4}\b`; spec
   `\b[Ss]pec[ _-]?[0-9]{3}\b` and `\bS[0-9]{3}\b`; requirement `\b(AC|OQ)[0-9]+\b` and `\b[RB][0-9]{1,2}\b`, legal
   only as a whole slot token (`"R3"` with a quote on each side, or `.R3`); section `§`; issue `#[0-9]+\b` after line
   start, a space, a double quote, a backtick or one of `([{,;:`; bead id `mindspec-[a-z0-9]{3,8}(\.[0-9]+)*` with no
   letter, digit, `.` or `-` before it and no letter, digit or `-` after it, except the allowlist words `owned`,
   `required`, `managed`, `level`, `fixture`, `driven`, `approved`, `written`, `started`, `rendered`, `internal`,
   `emitted`, `complete`, `build`; `issues\.jsonl`. A new word such as `mindspec-kernel` fails the scan loudly; the bead
   that adds the word either rewords it or adds it to the allowlist in its own commit, and never allowlists a word that
   is a real bead id. The permanent guard `TestProductSurface_NoInternalRefs` stays.
5. Update workflow "Surface scan". Text cites no internal record number.

**Verification**
- [ ] `go test ./cmd/mindspec/... -run '^TestS134Surface|TestADR0035Amendment' -v` passes and lists the new tests.
- [ ] `git fetch origin main && MINDSPEC_SURFACE_BASE=$(git merge-base HEAD origin/main) go test -tags surfacescan ./cmd/mindspec/ -run '^TestS134SurfaceBranch_' -v` passes on the bead branch.
- [ ] `go test -short ./cmd/mindspec/...` passes.

**Acceptance Criteria**
- [ ] AC16 (matcher): one planted line per class makes it fail (`ADR-0035`, `ADR 0035`, `ADR0035`, `spec 134`,
      `spec_134`, `S134`, `AC14`, `(R3`, `"B7: ..."`, `"R11 forbids"`, `§8`, `#7`, `#146`, `"#146 broke"`,
      `mindspec-xaoq`, `mindspec-4qd`, `mindspec-2fy.1`, `.beads/issues.jsonl`); `"R1"`, `.R3`, `mindspec-owned`,
      `.mindspec-scratch`, `"#ffffff"` and `"%#v"` do not.
- [ ] AC16 (diff function, temp repo): one added line per path class: a non-test Go file under `cmd/`, one under
      `internal/`, a `SKILL.md` under each skill root (each fails); a `_test.go` file and a `testdata/` file (each
      passes). An unchanged base line passes. A missing base fails. A base equal to the head fails; a base that is a
      spec-branch commit not reachable from the main ref fails; a base set to a merged main commit passes that
      commit's lines.
- [ ] The ADR-0035 pin test finds exit codes 3, 4 and 5 and goes red without the section.

**Depends on**
None

## Bead 4: Skills, guides and setup copies

**Steps**
1. Rewrite both copies of `ms-bead-cycle`: loop `next-action --apply`, `lease`, dispatch, `submit` and each `run` entry.
   Remove the spec-wide halt rows of contracts §13.6 from both copies of `ms-bead-cycle` and `ms-spec-autopilot`; keep
   the rows §13.6 keeps. Remove default-3 round text and deleted keys from every skill.
2. Add the old skill text to `historical_skills` so existing installs refresh. Make each `.claude/skills` copy
   byte-equal to its `plugins/mindspec/skills` source. Update the skill pin tests in this bead.
3. Update `project-docs/user/guides/autonomy.md`, `README.md` and `plugins/mindspec/README.md` for the one path and `bd
   ready --exclude-label mindspec`.
4. Update workflow "Setup and skills". Skill, guide and setup text cites no internal record number.

**Verification**
- [ ] `go test ./internal/setup/... -run '^TestS134Skills_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/setup/... ./internal/lint/... ./plugins/mindspec/... ./cmd/mindspec/...` passes (includes `TestReadyCheck_CycleSkillCopiesMatch`, `TestProductSurface_NoInternalRefs`, the destructive-guidance and skill pin tests).
- [ ] The AC15 `git grep` over `.claude/skills plugins/mindspec/skills plugins/mindspec/README.md README.md project-docs/user/guides` exits 1.
- [ ] `for s in plugins/mindspec/skills/*/; do n=$(basename $s); cmp $s/SKILL.md .claude/skills/$n/SKILL.md || echo $n; done` prints nothing.

**Acceptance Criteria**
- [ ] AC15: the skills-and-docs `git grep` exits 1. Both `ms-bead-cycle` copies name `next-action --apply`, `lease`
      and `submit`; `cmp` of each skill pair exits 0. After plain `mindspec setup` on an old install, each installed
      skill copy matches its source (`TestS134Skills_` over `historical_skills`).

**Depends on**
None

## Bead 5: Enrolment at plan approve

**Steps**
1. Add `internal/approve/enrol.go`, called from `plan approve` when `enrolOnApprove` is true. Under
   `phase.WithPhaseLock`, write in contracts §8 order: each work item (record its id in `bead_ids` right after its
   create), `mindspec_links`, the B7 cycle check, the plan-order links, `mindspec_run` (round limit from
   `config.RoundLimit()`, TTL from `panel.lease_ttl`, rules `kernel-1`), the approval record, the gates `<w>.review`
   and `<epic>.final` and each work-item link to `<epic>.final`. Use the record types and `planRunBDFn`.
2. Implement the §8 rerun rules. Reuse recorded ids, create only missing chunks, refuse every other plan change with
   its name and no write. Refuse a rerun after the approval record and name the enrolment. Never select by
   `--parent`. Before `mindspec_run`, a cycle refusal names the link, the foreign link that closes it and its bd
   removal; after it, a refused link is left for `--apply`.
3. Refuse without `step` (`kernel.RequireStepType`). Add label `mindspec` and the `[mindspec]` prefix to the epic and
   each work item. Make every spec-id parse of a title skip the tag: `phase.ParseSpecFromTitle`, `phase.titleSpecID`
   and the work-item parse `next.parseSpecID`.
4. Replace the type-based `classifyChild` count in `internal/phase/derive.go` for an enrolled spec. `phase` calls
   `kernelid.Enrolled` and the `kernelid` grammar; it does not import the kernel and holds no copy of either. Only
   recorded work items and kernel ids count; a foreign `task` under the epic does not. A pre-kernel spec keeps today's
   rule, and a `plan approve` rerun on it does not enrol and selects by recorded `bead_ids`.
5. Add `TestS134Enrol_FixtureMatches` (Testing Strategy "Shared fixture"), and the stage hook and the helper-process
   kill of "Kill in-process"; the AC2 (b), (d), (e) kills use it.
6. Update workflow "Enrolment", core "Phase derivation" and the ADR-0023 amendment note. Refusals cite no internal
   record number.

**Verification**
- [ ] `go test ./internal/approve/... -run '^TestS134Enrol' -v` passes and lists the new tests.
- [ ] `go test ./internal/phase/... ./internal/next/... -run '^TestS134(Derive|Title)_' -v` passes and lists the new tests.
- [ ] `go list -deps ./internal/phase | grep -E 'internal/next/kernel$'` exits 1 (phase imports only the leaf).
- [ ] `go test -short ./internal/approve/... ./internal/phase/... ./internal/next/... ./internal/panel/... ./cmd/mindspec/...` passes (includes `TestApprovePlan*` on the pre-kernel path and every fixture-consumer test merged so far).

**Acceptance Criteria**
- [ ] AC2 (enrolment): `plan approve` creates exactly `<w>.review` per item, `<epic>.final`, `mindspec_links` and
      `mindspec_run`. A second run exits non-zero, names the enrolment and leaves the snapshot unchanged.
- [ ] AC2 (a): a fake bd refuses a plan-order `dep add` as a cycle: exit non-zero naming the link; no gate,
      `mindspec_run` or approval. A foreign `bd create --parent <epic>`, then a normal rerun: exit 0, no second work
      item, that child unchanged.
- [ ] AC2 (b): killed (helper process, exit 137) after the creates, foreign links item 3 blocks a bug blocks item 1:
      the rerun exits non-zero naming a chain link and its bd removal; the call log has no link write.
- [ ] AC2 (d), (e): each listed plan edit after a kill gives the exit and snapshot the AC names; a chunk 4 added after a
      kill gives exactly one item 4 and one item-4-on-1 link.
- [ ] The fixture test goes red when `Enrolled` or enrolment changes a key, a link or a gate.
- [ ] AC5 (part): `bd close <w>` exits non-zero while `<w>.review` is open.
- [ ] AC6 (part): spec `y` from `PreKernelSpec`, a foreign `bd create --parent <y-epic>`, then `plan approve y`: no
      witness, that child unchanged.
- [ ] AC6 (derivation): on the enrolled fixture with `mindspec_run` deleted and each kernel step force-closed and
      retyped `chore`, phase derivation does not count a foreign `task` under the epic; a derivation that falls back to
      the type rule fails the test.
- [ ] A work-item title `[mindspec] [<spec id>] <heading>` resolves to its spec id in `next.parseSpecID`; the epic title
      parses as before.
- [ ] AC12 (pin): default config pins 5 and `2h`; `panel.max_rounds: 3` and `panel.lease_ttl: 30m` pin 3 and `30m`;
      only `loop.halt.max_rounds_per_bead: 4` pins 4. AC13 (part): epic and items carry the label and prefix.
- [ ] AC15 (part): with `types.custom: gate,spike2`, `plan approve` exits non-zero naming `mindspec setup`.

**Depends on**
Bead 1, Bead 2

## Bead 6: Reducer core and `next-action`

**Steps**
1. Add `reducer.go`: the core rules, registered with the Bead 1 `Pass` dispatcher, which turn `State` into a write
   list and the contracts §4 output. Core rules: implement authorisation (§13.3), round opening and the `panel create`
   entry in `run` (§13.7; the one writer of these two authorisations) only on an implement step or fix closed `passed`
   with a valid receipt, under a gate that is still open (a close with no valid receipt, or closed `superseded` or
   `halted`, authorises nothing; a passed gate gets no round), slots after `panel create`, the inline tally, a passing
   gate close (frozen), the `complete` entry (not listed while a change judge of that gate is open, as `complete`
   refuses then; the judge is matched by its tuple, a child of the work item, §13.2), re-creation of a deleted kernel
   bead (its rounds stay in the ledger) and `missing-link`: with `--apply` it writes each missing mindspec link that
   passes the B7 check (§8). A work item is done only when its gate closed `passed` and its branch is merged; any other
   state releases no dependent (B4). The slot rule creates slots only for a registered round that matches the gate's
   authorised round. A subject move before `panel create` updates the authorised subject only (§13.7).
2. Add `allowance.go`: `Allowance(state, gate)` is the pinned limit plus each `continue:<n>` answer; rounds count
   through `panel.RoundsForGate`. A round past the allowance gets no `run` entry.
3. Add `flags.go` (rules in mode `flag`) for `run-marker-missing`, `foreign-child-open`, `foreign-blocker` (B7 check on
   each missing link, naming the first foreign link and its bd removal) and `id-collision`. No rule finishes a
   decision, slot or fix close that an open foreign child stopped: after that child closes, the recovery is a rerun
   of the same `submit` (or `panel submit`), which closes the open step with its receipt (§6).
4. Add `output.go`: schema `mindspec.next-action/v1`, `revision` digest (no display time, no free text), `waiting`
   entries with lease generation and expiry, `retry_at` from the earliest expiry, statuses and exits 0/3/4/5/1, order
   plan-kind-slot, argv arrays, `termsafe` on text.
5. Fill `NextAction` in `nextaction.go`. Without `--apply` it writes nothing. With `--apply` it runs `Pass` with apply
   once under the phase lock and writes nothing when nothing changed. Refuse a spec with no approved plan, and
   `--apply` with bd silent or no `step`.
6. Update workflow "next-action". Output and refusals cite no internal record number.

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Reducer|Allowance|Flags|Output|SeedMatch)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134NextAction_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/panel/... ./cmd/mindspec/...` passes.

**Acceptance Criteria**
- [ ] AC1: `next-action x --json` twice: same bytes and snapshot, one action, item 1 `implement`, `"exists":false`. A
      change in each input class changes `revision`: a bd subtree field, a git ref the subtree names, a ledger line,
      a verdict file and the clock past an expiry. A display time does not.
- [ ] AC2 (apply): the first `--apply` creates exactly `<1>.impl.a1`; a second, or a rerun on the state a kill at its
      first write leaves, gives the same snapshot and `revision`.
- [ ] A live lease gives a `waiting` entry and `retry_at` at its expiry; moving the clock past it changes the output.
      `--apply` with a bd that exits non-zero on every call exits non-zero and writes nothing.
- [ ] A fake bd answers 13 to the first `--if-status` close of an `--apply`: exit 0, one close.
- [ ] A deleted `<1>.impl.a1` is created again by the next `--apply`. After `bd delete` of `<epic>.final`, one
      `--apply` creates it again with each work-item link to it, and a second `--apply` writes nothing; a rule order
      that runs `missing-link` before `recreate` fails the test.
- [ ] Complete entry: after `kerneltest.PassGate`, a commit on `bead/1` and `<1>.review.judge.a1` seeded open on that
      commit by `kerneltest.Judge` (parent `<1>`, tuple naming gate `<1>.review`), `run` lists no `complete 1` (later
      judge rules write nothing on this state). A rule that lists `complete` with a judge open, or that looks for the
      judge among the children of `<1>.review`, fails; "Core rules, positive" holds the case with no judge.
- [ ] Core rules, positive: with a seeded receipted close of `<1>.impl.a1`, `run` lists `panel create 1`; after
      `kerneltest.Register` of round 1 with 3 slots, `--apply` creates exactly `<1>.review.r1.R1`..`R3`; with 4 rounds
      counted and a closed `<1>.review.r5.fix` seeded, `run` lists `panel create 1`; after `kerneltest.PassGate`,
      `run` lists `complete 1`.
- [ ] Core rules, no receipt: a raw `bd close <1>.impl.a1` (no receipt) gives no `panel create 1` in `run`, and
      `--apply` writes no gate round record; the same for a raw-closed `<1>.review.r5.fix` with 4 rounds counted. A
      round opening that accepts any close fails the test.
- [ ] Core rules, frozen gate: after `kerneltest.PassGate` with `<1>.review.r2.fix` closed `superseded` with its pass
      receipt (the state a passing refute or defer retally leaves), `--apply` writes no gate round record and `run`
      lists no `panel create 1`. A round opening keyed on `ValidReceipt` alone fails the test.
- [ ] Core rules, gate still open: with `<1>.review.r2.fix` closed `passed` with its receipt and round 2 authorised
      (`kerneltest.Authorise`) but not registered, then `kerneltest.PassGate` (the state a passing `panel refute` of
      round 1 leaves), `run` lists no `panel create 1` and lists `complete 1`. A round opening that checks the close
      kind and receipt but not the gate state fails the test.
- [ ] Exits: on a seeded all-done fixture (every gate `passed`, every branch merged, the epic closed) `next-action x`
      prints `"status":"done"` and exits 5. In `output.go`, a result with no action, waiting entry, decision or `run`
      entry on a spec that is not done gives `indeterminate` and exit 1. A mapping that exits 0 for either fails.
- [ ] Missing link: after `bd dep remove` of the item-3-to-`<epic>.final` link, `--apply` writes it again; with a
      foreign link that closes a cycle on it, `--apply` writes no link and flags `foreign-blocker` naming that link.
- [ ] Done = passed and merged: after `kerneltest.PassGate` with item 1 open and `bead/1` not merged, `--apply` creates
      no `<2>.impl.a1` and lists no action for items 2 or 3. A rule that counts a passed gate as done fails this test.
- [ ] Authorised slots: `kerneltest.Register` of round 2 while the gate's authorised round is 1: `--apply` creates no
      `<1>.review.r2.R<s>`.
- [ ] Subject move (Amendment 1: Bead 19 also checks the fresh lens attempt): after a receipted close of `<1>.impl.a1`, a commit on `bead/1` before `panel create`: `--apply`
      sets the gate's authorised subject to the new commit, keeps round 1, creates no slot and lists one `panel create
      1`; `RoundsForGate` is unchanged.
- [ ] `--apply` with `types.custom: gate` exits non-zero naming `mindspec setup`; `next-action` on a spec with no
      approved plan exits non-zero; each writes nothing.
- [ ] `TestS134SeedMatch_PassGate` matches a real passing tally; `TestS134SeedMatch_Authorise` matches the round 1
      record the round opening writes on a seeded close of `<1>.impl.a1` with its receipt, and the round N record it
      writes on a seeded close of `<1>.review.r<N>.fix` with its receipt (no real `submit`).
- [ ] AC6 (part): with each kernel step force-closed and retyped `chore` and no `mindspec_run` or `panel.json`,
      `next-action` flags `run-marker-missing`.
- [ ] AC7 (allowance): with 5 rounds counted and a `continue:1` answer (seeded), the allowance is 6; with no answer the
      pass lists no `panel create` for that gate.
- [ ] AC13 (part): a foreign note under the epic gets no action and a `foreign-child-open` flag. A snapshot diff around
      `next-action --apply` changes only computed ids and links between them.
- [ ] AC14 (part): a foreign `<1>.review.1`: after a passing tally the gate stays open and `next-action` exits 3
      flagging `foreign-child-open`; after its close the gate closes `passed`.

**Depends on**
Bead 1, Bead 2

## Bead 7: `lease` and `submit`

**Steps**
1. Fill `Lease` in `lease.go`: under the phase lock write `in_progress`, a monotonic generation, the expiry
   (`kernel.Now` plus the TTL pinned in `mindspec_run`, never the live config) and a token hash with `SetStatusIf` on
   the status read. Print the token once. Support `--renew`, `--release` and `--take --reason` (records the take
   reason).
2. For an implement step, `Lease` calls the injected `LeaseDeps.EnsureWorktree` to create `bead/<w>` and its worktree,
   then records the claim base (the `bead/<w>` head at the claim) in the lease record. `cmd/mindspec/lease.go` passes
   `next.EnsureWorktree` (the helper `next` uses today). Bead 12 makes `next <w>` call this same function.
3. Refuse a foreign bead, a gate, a decision, a held or blocked ancestor or upstream (an open epic, work item or gate
   does not hold), a live lease (name `lease --take`), a halted spec and an unanswered decision (§13.8). On a halted
   spec `submit` refuses too, names the halt and writes nothing (no pass); only a retry of the receipted `halt` answer
   proceeds.
4. Fill `Submit` and `Answer` in `submit.go`: check the generation, the subject, the artifact blob at its commit and its
   fit per kind through the artifact-check registry. Register the checks for a verdict, a fix commit past the round
   subject on the branch the gate names (`bead/<w>`, or the spec branch for `<epic>.final`) and an implement commit
   past the recorded claim base. Write the receipt, close, call `Pass` once (§13.9); a pass result `halted` exits 4.
   `Pass` is the Bead 1 dispatcher, so the real path works without Bead 6; the tests assert only receipt, close and
   exit, which no core rule changes, so they pass with Bead 6 merged before or after.
   Same receipt again: exit 0, no change. `submit <decision> --answer <opt>` takes a listed option, needs no token and
   writes the answer record. Use `WithCASRetry`. Repair an unreceipted close when the evidence and current generation
   match (receipt `repaired: true`).
5. Set `enforced_by` of `panel.one-fixer` and `lifecycle.complete-by-orchestrator` in `internal/rules/rules.yaml` to the
   enforcing verbs. `sequence.bead-impl-prompt` keeps `advisory` and its staged marker; the kernel adds no check for it.
6. Update workflow "lease and submit". Output and refusals cite no internal record number.

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Lease|Submit|SeedMatch)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134(Lease|Submit)Cmd_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/rules/... ./cmd/mindspec/...` passes.

**Acceptance Criteria**
- [ ] AC3 (lease): `lease <1>.impl.a1` prints generation 1; a second lease exits non-zero naming `lease --take`; with
      the clock past the TTL `lease` prints generation 2. A foreign bug blocking item 1: `lease <1>.review.r1.R1` exits
      non-zero naming it.
- [ ] A table test: `lease` on a foreign bead, a gate, a decision, a `PreKernelSpec` item and a step of a halted spec
      (seeded `halt` answer) each exits non-zero, no write.
- [ ] Pinned TTL: on the fixture (pinned `2h`) with `panel.lease_ttl: 10m` in the config, `lease` records the expiry
      `kernel.Now` plus 2h, and `--renew` moves it to `kernel.Now` plus 2h. A `Lease` that reads the config fails.
- [ ] `--renew` with the current token moves the expiry to `kernel.Now` plus the TTL; `--renew` or `--release` with an
      old token exits non-zero, lease unchanged; `--release` with the current token frees the step, and a new `lease`
      then exits 0.
- [ ] AC4: each listed bad `submit` exits non-zero with the step open. A commit on `bead/1` made before the lease,
      ahead of the spec branch, is refused as not past the claim base. A valid `submit` writes `mindspec_close` =
      path, commit, blob id; a rerun exits 0, another artifact exits non-zero, each with `bd show --json` unchanged.
      On a seeded receipt with no close (the state a kill leaves): step open with its receipt; a rerun closes it.
- [ ] Return kind: a valid `submit` of `<1>.impl.a1` writes a receipt of kind `passed`. A `Submit` that writes an
      empty or other kind fails the test.
- [ ] `submit` calls `Pass` exactly once after the close (a spy on the seam). Through the real dispatcher: binary
      `submit` of a valid implement commit exits 0; `submit --answer halt` on a seeded halt decision exits 4;
      in-process, a failing test rule (through `kerneltest.WithRules`) makes `submit` exit non-zero with the receipt and
      close kept.
- [ ] `TestS134SeedMatch_Answer` matches the records `submit --answer` writes; `TestS134SeedMatch_Repaired` matches the
      `repaired` receipt a real `submit` writes after a raw slot close.
- [ ] Halted spec: with a seeded `halt` answer, `submit` of a valid implement commit and `submit` of a verdict each
      exit non-zero naming the halt, same snapshot.
- [ ] Final fix (seeded final round): a final fix commit on a bead branch is refused; one on the spec branch is
      accepted.
- [ ] AC10 (part): `submit` with a valid verdict and token after a raw slot close writes a `repaired` receipt.
- [ ] AC13 (part): after `bd update <1>.impl.a1 --claim`, `lease <1>.impl.a1` exits 0. A snapshot diff around `lease`
      and `submit` changes only computed ids and links between them.
- [ ] AC14 (part): a fake bd answers 13 to the first `--if-status` write: `submit` exits 0 with one receipt and one
      close; always 13: non-zero, step open with at most its receipt.

**Depends on**
Bead 1

## Bead 8: Refused rounds, decisions, halt and the final gate

*Amendment 1: Bead 14 removes step 2 and its tests (AC9 reducer first sentence, `TestS134SeedMatch_AuthoriseSupersede`);
Bead 15 moves the fix and REJECT decision of step 1 behind the verify step.*

**Steps**
1. Add `reject.go` (rules `supersede` and `refused-round`, mode `ordinary`): a refused tally below the allowance creates
   `<gate>.r<N+1>.fix`; a REJECT also creates `<gate>.decide.reject.r<N>` (`fix`, `halt`); at the allowance only
   `<gate>.decide.limit.r<N>` (`continue:<n>`, `halt`). `next-action` lists the decision, not the fix, until `--answer
   fix`. `continue:<n>` raises the allowance and creates the fix. These rules write no round authorisation: Bead 6's
   round opening authorises round N+1 when the fix closes.
2. Add B10 supersede to `reject.go`: a new subject during round N closes its open slots `superseded` and authorises
   round N+1; a commit in an open fix does not.
3. Add `halt.go`, its rule `halt` registered in mode `halt` (Contracts first): from the answer's receipt close every
   open step and decision under the epic `halted`, except a step with an open foreign child and its kernel ancestors.
   Status `halted` wins over `wait`; exit 4. A retry of the `halt` answer and `--apply` close the receipted decision
   and every other closable step `halted`, exit 4 and write nothing else (the pass does the closes; `submit` maps
   `halted` to exit 4). Each `halt` option text says it stops every work item.
4. Add `final.go` (rule `final-round`, mode `ordinary`): final gate round 1 on the spec branch head when the last item
   is done (B4), and B9 and the decision grammar for `<epic>.final` (§13.1).
5. Update workflow "Rounds, decisions and halt". Text cites no internal record number.

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Reject|Halt|Supersede|Final|SeedMatch)_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/panel/... ./cmd/mindspec/...` passes (includes `ForBead`).

**Acceptance Criteria**
- [ ] AC7 (reducer): 3 rounds on slug A, `panel abandon` A, 2 on slug B (seeded with `kerneltest.Register`), round 5
      refused: `next-action` lists one decision (`continue:<n>`, `halt`) and `run` has no `panel create`. After `submit
      --answer continue:1`, the pass creates `<1>.review.r6.fix`, writes no round authorisation and `run` lists no
      `panel create`; a rule that authorises on the answer fails here. After a close of that fix with its receipt is
      seeded, the pass (Bead 6's round opening) authorises round 6 and `run` lists `panel create 1`. The same pair for
      `<epic>.final` and `<epic>.final.r6.fix`.
- [ ] Decision with a foreign child: a foreign `bd create --parent` the seeded `<1>.review.decide.reject.r1`, then
      `submit --answer fix`: the answer receipt is written, the decision stays open flagged `foreign-child-open`, and
      the next `next-action` lists `<1>.review.r2.fix`. A reducer that waits for the decision's bd close fails.
- [ ] AC8 (reducer): REJECT on item 2, `--apply`: a decision under item 2 whose `halt` text says it stops the spec, and
      an action for item 3.
- [ ] AC9 (reducer): a commit on `bead/2` in round 2 with no fix open: one pass marks round-2 slots `superseded` and
      lists `panel create 2`; a second `--apply` creates no fix for the superseded round and lists the same `panel
      create 2`; in an open fix, none. A foreign child under `<2>.review`, a second REJECT, answer `halt`:
      other open steps `halted`, `<2>.review` open flagged `foreign-child-open` (a `flag` rule, run in halt mode),
      `next-action x` exits 4 on each read.
- [ ] AC9 (halt kill): in a new fixture, the `halt` answer's receipt is seeded with no close (the state a kill after the
      receipt leaves): `--apply` closes the decision and the open steps `halted` and exits 4. A retry of `submit
      --answer halt` exits 4 and closes the remaining closable steps; a retry that exits 0 fails the test. Item 1 is
      done in the fixture (`kerneltest.PassGate` and a git merge of `bead/1`). Control: on a twin fixture with no halt
      receipt, `next-action` (no `--apply`) lists `<2>.impl.a1`; the test fails when the control does not. The snapshot
      diff of each `--apply` on the halted spec shows only `halted` closes and their receipts, and no create.
- [ ] After the last item is done, the pass authorises final round 1 on the spec branch head. With the last item's gate
      `passed` but its branch not merged, the pass authorises no final round.
- [ ] A halt pass never closes a foreign bead (selection by computed id). New strings: no internal record number.
- [ ] `TestS134SeedMatch_OpenDecision` matches the decision a real REJECT pass creates and the limit decision a real
      pass at the allowance creates (id, options, tuple). `TestS134SeedMatch_AuthoriseSupersede` and `_AuthoriseFinal`
      match the round N+1 record a real supersede writes and the final round 1 record a real pass writes after the last
      item is done.

**Depends on**
Bead 6, Bead 7

## Bead 9: Unreceipted closes, raw closes and the audit table

*Amendment 1: Bead 17 replaces the retry for a close whose evidence fits (AC10 part, "an old token"); Bead 20 adds audit
rows.*

**Steps**
1. Add `closes.go`: the `flag` rule `unreceipted-close` flags a kernel close with no receipt or one that
   `ValidReceipt` refuses (Record shapes); the `ordinary` rule `retry-attempt` creates `<id>.a<k+1>` with
   `mindspec_from` on the next pass, unless a valid `repaired` receipt exists. An unreceipted close whose retry
   `<id>.a<k+1>` closed with a valid receipt counts as repaired: no flag, no further retry, and `impl approve` does
   not refuse for it.
2. Flag `raw-close` (a work item closed by bd after its gate passed, not merged), `forced-close` (closed over an open
   child) and `raw-claim` (a bd claim with no lease), each in a `flag` rule. Flags come from current state and are never
   stored. Export the `raw-close` and `forced-close` predicates for `complete`. A `raw-close` or `forced-close` item
   releases no dependent.
3. Add `audit.go`: count each escape hatch from the journal, and deferrals, `--take`, timed decision answers, raw
   claims, repaired unreceipted closes, carry-overs and the slot count of each re-panel from the record types.
4. In `internal/approve/impl.go`, for an enrolled spec, print the audit table and refuse on an open step, an open
   decision, an unrepaired unreceipted close, `raw-close`, `forced-close` or an open foreign child under the epic.
   Check before any write. Keep the finalize allow-set to recorded ids. A pre-kernel spec keeps today's path.
5. Update workflow "Close flags and audit". Text cites no internal record number.

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Closes|Audit)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/approve/... -run '^TestS134ImplApprove_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/approve/... ./cmd/mindspec/...` passes (includes `ApproveImpl`;
      the Bead 9 rules join the default registry, so every verb test in `cmd/mindspec` runs them).

**Acceptance Criteria**
- [ ] AC10 (part): `bd close <slot>` with no verdict, or with a verdict and an old token, flags `unreceipted-close` and
      the next `--apply` creates `<slot>.a2`. With a `repaired` receipt seeded by `kerneltest.Repaired` (pinned to
      `submit` in Bead 7), `--apply` creates no `<slot>.a2`.
- [ ] Pass receipts: on records seeded through the record types, a slot closed `superseded` (new subject), a fix
      closed `superseded` (retally) and a judge closed `superseded` naming a rebind raise no `unreceipted-close` flag,
      and `--apply` creates no `.a2` for any of them. A check that requires path, commit and blob on every kind fails.
- [ ] AC10 (part): after item 1's gate passes (`kerneltest.PassGate`), `bd close <1>` flags `raw-close`, `impl approve`
      exits non-zero naming it, and `--apply` creates no `<2>.impl.a1`. In a fresh fixture `bd close --force <1>` flags
      `forced-close` and `--apply` creates no `<2>.impl.a1`.
- [ ] A table test: `impl approve` on an enrolled spec with each of an open step, an open decision, an unrepaired
      unreceipted close, `raw-close`, `forced-close` and an open foreign child exits non-zero naming it, same snapshot;
      a refusal set that misses one row fails the test. Pass row: a raw slot close whose `<slot>.a2` is closed with a
      valid receipt (seeded): `impl approve` does not refuse for it; a check that reads only `repaired` fails.
- [ ] AC11 (audit): records of each listed event, written through the record types, give `impl approve` and a rerun
      exit 0, each printing count 1; the re-panel row shows 3 slots.
- [ ] AC13 (part): `next-action` flags `raw-claim`. A foreign note under the epic at the end: `impl approve` exits
      non-zero naming it and writes nothing.

**Depends on**
Bead 2, Bead 6

## Bead 10: Panel verbs as kernel steps

*Amendment 1: Bead 15 adds the open verify to the step 4 retally close; Bead 18 replaces the BRIEF stub of an enrolled
spec; Bead 19 adds the lens refusal to step 1.*

**Steps**
1. Make `panel create` the one round writer for an enrolled spec: take the subject from the gate, refuse another
   `--target`, a round the pass has not authorised (the gate's authorised round is not `RoundsForGate` + 1, so a
   registered round or no record at all refuses), a round past `kernel.Allowance`, an
   open slot on the current subject, an unanswered decision on the
   gate, a passed gate, an id collision (`id-collision`) and a halted spec. Write `"orchestrated": true` in
   `panel.json`. Run one pass (§13.9). The kernel calls live in `cmd/mindspec`; `internal/panel` stays a leaf.
2. On a re-panelled gate (a re-panel record on `<gate>.a<k+1>`), select the highest attempt, open exactly n slots,
   record the paths as the round's scope and refuse any other `--reviewers` count. Make a successful `panel rebind`
   run one pass.
3. For `--gate spec_approve` or `plan_approve`, count rounds with `panel.RoundsForGate` and, past `panel.max_rounds`,
   print one warning naming the count and proceed (count and warn, no refusal).
4. Make `panel submit` require `--lease` of the current generation and close the slot as `submit` does. Update
   `_HelpFlags`. Make a passing `panel refute` or `panel defer` retally close the gate `passed` and its open fix or
   limit decision `superseded` (Bead 16: after an open-gate `re-panel` judgement it authorises round N+1 instead);
   `panel defer` writes the defer record.
5. Keep every refusal of today. A pre-kernel spec keeps today's panel rules. Update workflow "Panel verbs".
6. Each test seeds the full state the producers leave (fix, decision, answer, authorised round), and every seeded
   close carries its receipt, so the passes write nothing whether Beads 8, 9 and 11 are merged or not.

**Verification**
- [ ] `go test ./cmd/mindspec/... -run '^TestS134(Panel(Create|Submit|Refute|Defer|Rebind|Repanel)|SeedMatch)_' -v` passes and lists the new tests.
- [ ] `go test -short ./cmd/mindspec/... ./internal/panel/...` passes (includes `PanelCreate|PanelSubmit|PanelDefer|PanelRefute|PanelRebind`, `_HelpFlags` and `TestPanelLeafImports_StdlibPlusTermsafeOnly`).

**Acceptance Criteria**
- [ ] AC3 (part): `panel submit` without `--lease` exits non-zero; the slot stays open. With a valid `--lease` and
      verdict it closes the slot with a receipt.
- [ ] AC5 (part): after a pass, `panel create <w>` exits non-zero; so does `panel create 1 --target <another ref>`.
      With `r2.fix` or a limit decision open (seeded), a passing `panel refute` of the only blocking finding closes the
      gate `passed` and the fix or decision `superseded`; a passing `panel defer` does the same. After each, the
      `next-action` output lists no `panel create 1` and lists `complete 1` in `run`.
- [ ] AC7 (verb): 3 rounds on slug A, `panel abandon` A, 2 on slug B: round 3 of B exits non-zero. The test then seeds
      the state the real path leaves after the answer: `<1>.review.r6.fix` closed with its receipt (Bead 8 creates it)
      and round 6 with `kerneltest.Authorise` (Bead 6's round opening on that close). After `submit --answer
      continue:1` on the seeded limit decision (its pass creates no fix: the fix exists), one more round runs and the
      next exits non-zero. The same for the final gate (`<epic>.final.r6.fix` closed with its receipt, round seeded
      with `Authorise` on the spec branch head). With no authorised round, `panel create` exits non-zero naming it.
- [ ] Round authorisation: (i) a seeded REJECT round 1, decision answered `fix`, `<1>.review.r2.fix` open; (ii) a seeded
      refused round 1 with no REJECT, all its slots closed with receipts, `<1>.review.r2.fix` seeded open: in each,
      `panel create 1` exits non-zero naming the unauthorised round, same snapshot (no registration, `round` line, slot
      or gate round record). After a close of the fix with its receipt is seeded and round 2 is authorised (seeded with
      `Authorise`, the record Bead 6's round opening writes on that close), it exits 0. A presence-only check fails the
      test.
- [ ] A sixth `plan_approve` round exits 0 with one warning naming the count; a refusal fails the test.
- [ ] AC8 (verbs): with a seeded unanswered REJECT decision and fix, `lease` on the fix and `panel create 2` exit
      non-zero; after `submit --answer fix`, `lease` exits 0.
- [ ] AC9 (verb): on a halted spec (seeded `halt` answer), `panel create 2` exits non-zero, same snapshot.
- [ ] AC14 (part): a foreign kernel-shaped `<1>.review.r1.R1`: `panel create 1` exits non-zero printing
      `id-collision`; that bead is unchanged.
- [ ] AC18 (b, verb): with a seeded 2-slot re-panel record and paths on `<1>.review.a2`, `panel create 1` registers
      exactly 2 slots on `<1>.review.a2` with those paths as scope; `panel create 1 --reviewers 6` exits non-zero.
- [ ] AC13 (part): a snapshot diff around `panel create`, `panel submit`, `panel refute`, `panel defer` and `panel
      rebind` changes only computed ids and links between them.
- [ ] In-process, a test rule (through `kerneltest.WithRules`) counts calls: a successful `panel rebind` calls it once,
      a refused one never. Bead 13 tests the judge close of that pass. `TestS134SeedMatch_Register` matches `panel
      create`.

**Depends on**
Bead 2, Bead 6, Bead 7

## Bead 11: Change judge

*Amendment 1: Bead 16 extends steps 2-4 to an open gate at a tally, adds the path cover check to step 3 and keeps the
judge artifact in a registered panel directory.*

**Steps**
1. Add `judge.go` (rules `judge-apply`, `judge-supersede` and `judge-create`, mode `ordinary`): compute the accepted
   subject (§13.2) from the passing panel's subject as `panel rebind` refreshes it and the last `carry-over`, exported
   as `kernel.AcceptedSubject(state, gate)` for the highest attempt. A commit that changes only a panel directory of the
   gate does not move it.
2. When the newest subject commit of a passed gate is not the accepted subject, create `<gate>.judge.a<k>` (kind
   `change-judge`, child of the gate's parent) unless the item is merged or the epic is closed. Close an open judge
   `superseded` when its subject is not the newest commit or is the accepted subject; its receipt names the rebind.
3. Register the judgement artifact check for `submit`: `carry-over` with a reason, or `re-panel` with n slots (2 to the
   passing round's slot count) and the changed paths. Check the shape, the old subject, the new subject and the frozen
   receipt.
4. On `carry-over` add the old subject, the new subject and the judge receipt to the gate receipt. On `re-panel` create
   `<gate>.a<k+1>` with `mindspec_from` and the re-panel record, and authorise an n-slot round whose `run` entry
   carries the paths.
5. Update workflow "Change judge". Text cites no internal record number.

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Judge|SeedMatch)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134JudgeCmd_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./cmd/mindspec/...` passes.

**Acceptance Criteria**
- [ ] AC18 (judge): after a 3-slot pass, a `.mindspec/config.yaml` commit on `bead/1`: `--apply` creates exactly
      `<1>.review.judge.a1` and no round. A second commit before its submit: the next pass closes a1 `superseded` and
      creates a2 on the newest commit; `submit` to a1 exits non-zero.
- [ ] AC18 (a): `carry-over` with a reason keeps the gate `passed`; its receipt names both subjects and the judge
      receipt. A commit of the judgement file creates no judge.
- [ ] AC18 (b, pass): `re-panel` with 2 slots and paths gives `<1>.review.a2` (`mindspec_from` set) and a `panel create
      1` entry that names the paths.
- [ ] AC18 (c): 1 or 4 slots, another value, `carry-over` with no reason, `re-panel` with no paths each exit non-zero,
      step open, no round; 3 slots exits 0 and authorises a 3-slot round.
- [ ] AC18 (d, pass): with a rebind seeded in the panel ledger as `panel rebind` writes it, the pass closes the open
      judge `superseded`; the receipt names the rebind.
- [ ] Final gate: after `<epic>.final` passes, a commit on the spec branch makes the pass create
      `<epic>.final.judge.a1`; a `carry-over` to it keeps the final gate `passed`.
- [ ] No judge after the end: with item 1's gate passed and `bead/1` merged (git merge in the test), a commit on
      `bead/1` gives no judge on `--apply`; with `<epic>.final` passed and the epic closed (seeded), a commit on the
      spec branch gives no final judge. A rule that ignores the merge or the epic close fails the test.
- [ ] `TestS134SeedMatch_AuthoriseRepanel` matches the record a real `re-panel` judgement writes: the next round
      (`RoundsForGate` plus 1) on `<1>.review.a2`, the new subject, the slot count and the paths.
      `TestS134SeedMatch_Judge` matches `kerneltest.Judge` to the judge a real pass creates (id, parent `<1>`, tuple,
      subject); on a mismatch Bead 11 corrects the seeder (Amendments).
- [ ] AC13 (part): a snapshot diff around a judge `submit` changes only computed ids and links.

**Depends on**
Bead 6, Bead 7

## Bead 12: `next`, `complete` and enrolment on

**Steps**
1. Make `next <w>` (in `cmd/mindspec/next.go`, which wires `next.EnsureWorktree` as `lease.go` does) call
   `kernel.Lease` for `<w>.impl.a<k>` and print the token; with a live lease refuse and name `lease --take`. Select
   only approved work items by computed id; resolve the spec without `--spec` and skip a halted spec. A pre-kernel spec
   selects by recorded `bead_ids`. Update `_NextFlags` if flags change.
2. Make `complete` refuse unless the gate closed `passed` and no child is open. On `raw-close` merge and clear it; on
   another closed unmerged item or `forced-close` refuse with `bd reopen <w>`. A rerun on a merged item exits 0.
   `MINDSPEC_SKIP_PANEL=1` with no passing gate refuses and names it. Run one pass (§13.9).
3. Enforce the bead panel gate for every enrolled spec whatever `panel_presence` says; a pre-kernel spec keeps today.
   For an enrolled spec the gate's subject comes from the kernel gate, not from today's stale-head rule: `complete`
   takes the highest attempt of `<w>.review`, its passing round and slot count, and `kernel.AcceptedSubject` (the
   passing subject, or the newer subject of a carry-over or a rebind). A `bead/<w>` head that is the accepted subject,
   or differs from it only in panel directories, passes; any other moved head refuses, and the refusal names the head
   and the accepted subject. For an enrolled spec `complete` runs all its checks and names each one that fails, so an
   open child and a moved head are both named. `impl approve`'s final-gate check (`internal/approve/impl_final_gate.go`,
   `lifecycle_gate.go`) does the same for `<epic>.final` and the spec branch head. A pre-kernel spec keeps today's rule.
4. Delete `approve.enrolOnApprove`: `plan approve` enrols every new spec. In the same bead update every existing test
   that runs `plan approve`: a test that pins today's verbs seeds `kerneltest.PreKernelSpec`; a test that pins `plan
   approve` asserts the enrolment. Files: `cmd/mindspec/{approval_gates,bead_ready,lettered_spec,next_selection,
   next_ready_gate,panel_preepic_gate,verb_output}_test.go`, `internal/harness/{lifecycle_scenario,scenario}_test.go`,
   `internal/approve/plan*_test.go`, and Bead 5's enrolment tests: the kill helper keeps the stage hook and no longer
   sets the deleted variable.
5. Add fault and verb tests for AC2 (c), AC3, AC6, AC9 and AC10 below, with a snapshot diff around `next` and
   `complete`. Update workflow "Kernel loop". Text cites no internal record number.

**Verification**
- [ ] `go test ./cmd/mindspec/... -run '^TestS134(Next|Complete|EnrolFault)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/complete/... -run '^TestS134Complete_' -v` passes and lists the new tests.
- [ ] `go test ./internal/approve/... -run '^TestS134ImplFinalGate_' -v` passes and lists the new tests.
- [ ] `go test -short ./cmd/mindspec/... ./internal/next/... ./internal/complete/... ./internal/approve/... ./internal/harness/...` passes (includes `_NextFlags`, `TestApprovePlan*` and the lifecycle scenario).

**Acceptance Criteria**
- [ ] AC2 (c): `plan approve` killed after the `<epic>.final` create, foreign links `<epic>.final` blocks a bug blocks
      item 3: `--apply` exits 0 without the item-3-to-final link and flags `foreign-blocker` naming the link. After `bd
      dep remove`, the flag is gone and `--apply` writes the link.
- [ ] AC2 (part): after the gate pass and before `complete 1`, `--apply` creates no `<2>.impl.a1`; then the pass of
      `complete 1` creates exactly `<2>.impl.a1` and `<3>.impl.a1`.
- [ ] AC18 (accepted subject): with a carry-over receipt seeded through the record type on `<1>.review` and `bead/1`
      moved to its new subject, `complete 1` exits 0 (the stale-head rule fails this); with no carry-over it exits
      non-zero naming the head and the accepted subject (a refusal for the open judge alone fails). A commit on `bead/1`
      that changes only the panel directory: `complete 1` exits 0. The same cases for `<epic>.final` in the `impl
      approve` final-gate check (`TestS134ImplFinalGate_`).
- [ ] AC18 (judge open): with `<1>.review.judge.a1` open after a config commit, `complete 1` and `impl approve` each
      exit non-zero naming it. After a carry-over, `complete 1` exits 0 and the next `--apply` creates no judge.
- [ ] AC3 (part): `next 1` with no `--spec` resolves `x` and prints a token (generation 1); a second `next 1`, and
      separately `lease <1>.impl.a1` after `next 1`, each exit non-zero naming `lease --take`.
- [ ] AC6 (part): `panel_presence: warn`, kernel steps force-closed and retyped `chore`, no `mindspec_run` or
      `panel.json`: `complete` with no passing panel exits non-zero. On `PreKernelSpec` `y`, `complete` with no panel
      warns and exits 0; `next` does not select the foreign child.
- [ ] AC9 (verbs): on the halted spec (seeded `halt` answer) `lease`, `complete` and `next 2` exit non-zero, same
      snapshot. `panel tally`, `panel verify`, `config show` and `doctor` are not refused for the halt. With a second
      enrolled spec `z` active, `next` with no argument and no `--spec` skips the halted `x` and selects an item of `z`.
- [ ] `complete 1` again after its merge exits 0 with the same snapshot.
- [ ] AC10 (part): `complete 1` exits 0 and clears `raw-close`; on `forced-close` it exits non-zero printing `bd reopen
      <1>`; after `bd reopen <1>` and a gate pass (`kerneltest.PassGate`) it exits 0 and `impl approve` does not name
      the flag.
- [ ] AC11 (part): `MINDSPEC_SKIP_PANEL=1 complete` with no passing gate exits non-zero and names it.
- [ ] AC13 (part): a snapshot diff around `next` and `complete` changes only computed ids and links between them.

**Depends on**
Bead 5, Bead 6, Bead 7, Bead 8, Bead 9, Bead 10, Bead 11

## Bead 13: End-to-end kernel path

*Amendment 1: Bead 21 updates the AC17 and AC11 cases for the lens and verify steps and the new audit rows.*

**Steps**
1. Add `cmd/mindspec/kernel_e2e_test.go`: build the binary, sandbox with `step`, `plan approve` spec `x`, and drive
   only `next-action --apply`, `lease`, `submit` (the test writes verdict files as a fake reviewer) and each `run`
   entry to `impl approve` exit 0 and `"status":"done"`.
2. Keep a foreign note, a foreign bug and a raw claim present for the whole run. Take a snapshot diff around each
   verb: `next-action --apply`, `lease`, `submit`, `next`, `panel create`, `panel submit`, `panel refute`, `panel
   defer`, `panel rebind`, `complete`, `impl approve`.
3. Produce each AC11 event through its real verb and read the audit table of `impl approve` and its rerun.
4. Run the AC12 pin and AC18 verb cases on enrolled fixtures made by this binary.
5. Update workflow "Kernel loop" with the loop the test drives. When a test here finds a defect in another bead's
   code, fix it in this bead in that bead's files and list each fix in the commit message.

**Verification**
- [ ] `go test ./cmd/mindspec/... -run '^TestS134E2E_' -v` passes and lists the new tests.
- [ ] `go test -short ./...` passes, and the Bead 2 `.mindspec/domains` `git grep` exits 1.
- [ ] `git fetch origin main && MINDSPEC_SURFACE_BASE=$(git merge-base HEAD origin/main) go test -tags surfacescan ./cmd/mindspec/ -run '^TestS134SurfaceBranch_' -v` passes.

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
      slots. A producer that writes a record under another key makes the test fail. After the repaired `submit`,
      `--apply` creates no `<slot>.a2`.
- [ ] Round authorisation (real path): a REJECT on round 1 answered `fix`: `panel create 1` exits non-zero until the
      fix is submitted, then exits 0.
- [ ] AC12 (pinned): enrolled with defaults, then `panel.max_rounds: 99` and `panel.lease_ttl: 10m`: round 6 exits
      non-zero, and a new lease records an expiry 2h after its issue time.
- [ ] AC18 (verbs): with a judge open, `complete 1` and `impl approve` exit non-zero naming it. After a carry-over,
      `complete 1` exits 0 and creates `<2>.impl.a1`, `impl approve` names neither gate nor judge, and a pass after each
      creates no judge. After a re-panel pass, `complete 1` exits 0. With an ordinary commit and a judge open, a
      successful `panel rebind` closes the judge `superseded` in its pass, its receipt names the rebind, and
      `complete 1` exits 0.

**Depends on**
Bead 8, Bead 9, Bead 10, Bead 11, Bead 12

## Bead 14: Contract amendment for verify and lens

**Steps**
1. In `kernelid`, add `.r<N>.verify` and `.r<N>.lens` to the grammar and `Format`, with `.a<k>` attempts.
2. In `records.go`, add the kinds `verify` and `lens` (and to the `RegisterArtifactCheck` kind switch in `submit.go`,
   with a close-write hook per kind that every receipt path runs before the receipt, §6; Beads 15 and 19 fill it.
   For a verify only (§6 closing pin), after every other §6 check and the §13.10 record checks (`bead.CheckDeferral`
   included) and on a step with no valid receipt: have the hook
   build and size-check its writes, then write `mindspec_closing` {generation, record_sha256, at} when absent;
   refuse a submit, late submission or take whose generation or `record_sha256` differs, naming
   `mindspec_closing`, and write only its `conflict`; while `conflict` is set refuse each submit and write nothing, and
   read it again before the receipt write; a matching rerun reuses its `at` and runs the hook again. On a step with a
   valid receipt no submit makes a close write and the pin writes nothing; `submit` runs under the phase lock) and the records: verify record and its returns (`ground`, `ac`, `reason`, `evidence`, `target`), lens record,
   `mindspec_reproduced`, lease `ended`, take event `via`, `generation`, `revoked_token_hash` and `subject`, receipt
   `generation`, `superseded_generation`, `via` and `retally: "verify"`, the verify receipt's `at`, `mindspec_closing` (with `conflict`), gate round `lenses`, `lens`, `verify`, `prior` (new; `verify` only for a verify closed `passed`, `read`, path -> sha256, and `asks`, the rendered
   asks) and `judged` (judge, value, new subject), the verify step's `mindspec_findings`, `mindspec_read` and
   `mindspec_returns`, and the `slots` field (slot, attempt) of the `mindspec_decision` record (`DecisionRecord`).
   The lens and verify record decoders refuse an unknown field. Pin each with a golden JSON file; the `prior`
   golden omits each absent key (`verify`, and `read` and `asks` when empty).
3. Raise the `next-action` schema to `mindspec.next-action/v2` in `pass.go` and in `actorOutput` (`submit.go`); set the
   kind order to implement, lens, review-slot, verify, fix, change-judge, decision in `output.go`. Update the v1 literals that `internal/next/kernel/output_test.go`
   (~:80) and `records_test.go` (~:199) pin.
4. In `submit.go`, delete the slot refusals "step subject or round is stale" (`checkVerdict`) and "step subject is
   stale" (`activeActorRound`) that compare a slot with the branch head (B10: a slot's subject is its round subject).
5. Replace the rule list in `contract.go` (Decomposition, Amendment 1) and `testdata/rules.json`. Delete the rule
   `supersede` from `reject.go` and its tests; delete the `resetScope` branch of `roundOpen` that restores a full round
   after a re-panel (Bead 16 adds its replacement). Register `verify-apply` and `lens-create` as rules that write
   nothing until Beads 15 and 19 fill them.
6. In `store.go`, make `Create` (~:306), `WriteMetadata` and `SetStatusMetadataIf` give bd the merged metadata as
   `--metadata @<file>` (a uniquely named file per call in a kernel temporary directory outside the repository, mode
   0600, removed after the bd call; each pass removes the files whose process no longer runs) and refuse merged
   metadata over 16 MiB (a kernel limit) before any bd call with `metadata-too-large`, naming the gate, the step, the largest
   key and the size (§13.10). No stored text is cut. Make the fake bd runner in
   `internal/next/kernel/kerneltest/fake.go` (~:99, ~:145) read the metadata from `@<file>` during the call.
7. Apply the kernel-directory exclusion (§13.11), so the verify and lens record commits of Beads 15 and 19 move no
   subject: `effectiveSubjectAt` (`judge.go`; today it exempts only registered panel directories and returns the head
   early with none) leaves `<spec dir>/kernel/` out also when the gate has no registration; `checkFix` (`submit.go`)
   refuses a fix commit that changes only the panel and kernel directories; `panel.Subject`
   (`internal/panel/subject.go`) appends `<specs>/*/kernel/**` after its four exclusions (`ToolArtifactPaths` in
   `artifacts.go` reads `Exclusions[0]`). Update the five-entry list in `TestSubject_PerGate`
   (`internal/panel/subject_test.go`), the "excludes exactly" text of the spec-130 amendment in
   `.mindspec/adr/ADR-0037-panel-gate-enforced-contract.md` and in `.mindspec/domains/workflow/interfaces.md`, and the
   phrases that `internal/validate/adr_spec130_subject_test.go` asserts. In `isDocFile`
   (`internal/validate/docsync.go`) do not count `<specs>/*/kernel/**` as documentation.
8. Add the seeders `Verify` and `Lens`. Update workflow "Kernel store and ids".

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(IDs|Contract|Records|Output|Reducer|Submit|Store)_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/panel/... ./internal/validate/... ./cmd/mindspec/...` passes.

**Acceptance Criteria**
- [ ] `<w>.review.r2.verify`, `<w>.review.r2.lens` and `<epic>.final.r1.verify.a2` parse and format; `<w>.review.verify`
      and `<w>.review.r2.lenses` do not.
- [ ] The output golden names `mindspec.next-action/v2`; actions of all six kinds sort in the §4 order.
- [ ] The rule golden equals the Amendment 1 list; it has no `supersede`.
- [ ] A fake bd runner sees `create` and `update` with `--metadata @<file>` and no argv string over 128 KiB for a
      verify create whose `mindspec_findings` holds 9 slots x 100 findings of 2048 bytes, and for a 1 MiB update; the
      file has mode 0600 during the call and is gone after it; two calls use two file names, and a seeded file of a
      process that no longer runs is gone after the next pass. Mutations in which `Create` or `WriteMetadata` passes
      the JSON inline fail. 17 MiB of metadata refuses with `metadata-too-large` and no bd call, naming the gate, the
      key and the size (a mutation without the bound fails).
- [ ] Closing pin: a seeded verify submit with a recording hook writes `mindspec_closing` before the hook's first
      write; killed after the hook, a matching rerun keeps that `at` and the hook runs again; a submit of another
      record, or of another generation, exits non-zero naming `mindspec_closing`, writes only its `conflict` and no
      receipt, and a later matching submit then exits non-zero with no receipt. A bad record, a refused token and an
      oversized record write no `mindspec_closing`. After the receipt, a submit of another record or generation exits
      non-zero, writes no `conflict` and the next pass creates no `.a2`. Mutations that accept the differing record, or
      write the conflict on a receipted step, fail.
- [ ] Subject: a golden tree with no `kernel/` directory keeps its digest, and a tree that differs from it only under
      `<specs>/<spec>/kernel/` has the same digest (a mutation that drops the exclusion fails); `ToolArtifactPaths` is
      unchanged. After a gate passes, a commit of only kernel records opens no judge, also on a gate with no
      registration; a fix `submit` of a commit that changes only kernel records exits non-zero, step open.
- [ ] AC9 (reducer, part): a commit on `bead/2` during round 2: `--apply` closes no slot and authorises no round, and
      each slot's `submit` of a verdict on the round subject exits 0 (`TestS134Submit_`).

**Depends on**
Bead 13

## Bead 15: Verify step

**Steps**
1. Change `refusedRound` (`reject.go`; bead and final gates share it): when every slot step of round N is closed
   `passed`, the tally is complete (`Complete()`, which it already checks, and no roster verdict INVALID) and it does
   not pass, create `<gate>.r<N>.verify` (kind `verify`, child of the gate) on the gate's review subject (§6), write
   its id to `mindspec_round.verify`, pin `mindspec_findings` (full texts) and `mindspec_read` from the files it read on disk
   (§13.10; a pass without `--apply` computes them and writes nothing), and create no fix or decision. When the tally does not count a roster verdict
   (§13.10 incomplete tally), create that slot's next attempt `<gate>.r<N>.R<s>.a<k+1>` and no verify. At the cap (§13.10: one slot or more
   with 3 uncounted attempts above the attempt recorded for it by the highest decision that lists it and was answered
   `redispatch`) create no
   slot attempt and one `<gate>.decide.uncounted.r<N>` (later ones `.a<k+1>`) whose `slots` lists each capped slot and
   its highest attempt: extend `decisionWrite` (`reject.go`, today it no-ops on an existing id) with the options
   `redispatch`, `halt` for cause `uncounted` and an attempt suffix. While it is open, create no slot attempt of round
   N; `lease.go` already refuses every step of the gate (cause not `limit`) and stays as it is. On `redispatch`
   create the next attempt of each listed slot whose verdict is still uncounted. Keep the skips of a round mismatch, `PassRule`'s
   intact check (`Result.intactError`, which `IntegrityErr` does not cover) and an unreadable registration. Once round N's verify is applied
   (§13.10 frozen round), neither rule nor the changed-finding trigger acts on round N (Bead 16 adds judge case (a)); at the round N+1
   authorisation close each open slot attempt of round N `superseded`. A verify is not a round: `Allowance` and
   `RoundsForGate` do not count it.
2. Add `verify.go`: a kernel finding enumerator (§13.10) over round N's verdict files: one `veto` finding for each
   REJECT or `hard_block` roster verdict and each off-roster file that `verdictFileRE` matches
   (`panel.Result.Unexpected`) and that is REJECT, `hard_block` or INVALID (as `unexpectedVeto` reads it), with `finding_sha256` = sha256 of the verdict file's bytes; for every other roster verdict,
   the `ccr[<i>]` findings and the schema-1 `slot` finding that `panel.Findings` gives. `panel.Findings` itself does
   not change. `refusedRound` writes nothing and returns an internal error when the enumerator returns no finding.
   The artifact check for `submit` (shape and existence only, each refusal names its condition, step open): unknown
   fields, coverage, a non-empty list, no `deferred` for a `veto`, the `refuted` fields as `panel refute` checks them,
   and the `bead.CheckDeferral` checks on each `deferred` target, all before the pin. In the verify close-write
   hook (Bead 14; it runs before the receipt on every receipt path), write each `ccr` or `slot` return that is `refuted`
   as `panel refute` writes a refutation and each `deferred` return as `panel defer` writes a deferral, each with its
   disposition row (a deferral bound to the slot's verdict file by `source_verdict` and `source_file_sha256`, as
   `panel defer` binds it), with `verify_step` = the step and `enrolled: true`, the new fields `verify_step` (`string`)
   and `enrolled` (`bool`) of `panel.Refutation` and `panel.Deferral` (`internal/panel/panel.go`; each `omitempty`, no custom JSON encoding, so `==` in
   `checkCompletenessAt` still works; a pre-kernel record has neither), and write every return to `mindspec_returns`. The hook builds and
   size-checks every write before the pin (Bead 14, §13.10). A matching rerun writes `mindspec_returns` and the
   `mindspec_deferrals` entries again by key and appends identical ledger records and rows (`created_at` =
   `mindspec_closing.at`), which the ledger keeps once (§6). In `recordKey` (`internal/panel/disposition_store.go`)
   add `verify_step` to the refutation and deferral key when it is non-empty, else `created_at` when `enrolled`; append
   the same suffix (`:<verify_step>`, else `:<created_at>`) after the final `:refutation` or `:deferral` segment of the
   disposition row id in the shared row builder and both `DeferralRowID` shapes; a row's `created_at` equals its
   event's; a record without the fields keeps today's key and row id (§13.10). No later rule reads the record file.
3. Fill `verify-apply`: with no `reproduced` return, close the gate `passed` (receipt `retally: "verify"`), a refuted
   REJECT included (Bead 16 adds the branch under a `re-panel` judgement). Make `passed()` and
   `gatePassEvidence` (`reducer.go`) and `CheckPassedGate` (`gate_check.go`) accept that receipt on the §13.10
   conditions; `panel.PassRule` and the refute and defer refusals do not change, and `panel tally` stays advisory on an
   enrolled gate. Else, below the allowance, create `<gate>.r<N+1>.fix` with `mindspec_reproduced`, and for a
   reproduced REJECT veto `<gate>.decide.reject.r<N>` (another reproduced veto: the fix only); at the allowance, only
   the limit decision; `continue:<n>` creates the fix with `mindspec_reproduced`.
4. Make a passing `panel refute` or `panel defer` retally close an open verify `superseded` (after an open-gate
   `re-panel` judgement it authorises round N+1, Bead 16). Add the current-verify
   filter (§13.10): drop refutations and deferrals whose `verify_step` names a verify with no valid receipt or is set and is not the current verify of their round,
   and, once a verify attempt of their round is closed `passed`, those with an empty `verify_step` whose `created_at`
   is not after the receipt `at` of the highest such attempt, compared as parsed times (§13.10), from a deep copy of
   `Result.Panel` and its `Refutations` and `Deferrals` slices (`passrule.go` and
   `tally.go` read them; `PassRule` does not change; `panel_refute.go` writes the tallied `Panel` back, so never edit it)
   before `PassRule` in `tally`, `refusedRound`, `gatePassEvidence`, `CheckPassedGate` and the
   `panel refute` and `panel defer` retally (`cmd/mindspec/panel_kernel.go`). The verify receipt `at` is
   `mindspec_closing.at` (RFC3339Nano UTC from `kernel.Now`, Bead 14); `panel_refute.go` and `panel_defer.go` write `created_at` from `kernel.Now`
   (today `time.Now`; production `kernel.Now` is `time.Now` and the verbs keep `.UTC().Format(time.RFC3339Nano)`, so
   `created_at` bytes do not change on a pre-kernel spec). A golden test pins a same-second pair (`...:05Z` before `...:05.5Z`) and an equal pair (not
   after). Before the verify is applied, retire it when its
   `mindspec_returns` no longer cover round N's findings or its `mindspec_closing` has a `conflict` (the new attempt
   copies `mindspec_findings` and `mindspec_read`), and open `.verify.a<k+1>` (on an incomplete tally, clear
   `mindspec_round.verify`; `refused-round` opens the attempt when the tally is complete and refused again). The refutation and deferral writers
   (`cmd/mindspec/panel_refute.go`, `panel_defer.go`, or a helper they share with the verify close) carry
   `verify_step` (empty for the verbs, the verify step for a verify close) and, on an enrolled gate, `enrolled: true`,
   and build the disposition row id (step 2). On an enrolled gate the dedupe (slot, round, finding, `finding_sha256`,
   not the ledger key; `panel defer` also
   compares `SourceVerdict` and `SourceFileSHA256`) of the verbs (the `found` loop in `panel_refute.go` and `panel_defer.go`) and of the
   `mindspec_deferrals` history that `panel_kernel.go` appends (today keyed on panel, round, slot, finding and target
   only; add `finding_sha256`, `SourceVerdict`, `SourceFileSHA256` and `VerifyStep` to `kernel.DeferRecord` in
   `internal/next/kernel/records.go` and its `testdata/record-keys.json` golden) runs before the append and compares
   only records that the filter keeps (§13.10). The verify close does not dedupe.
5. In `gatherPanelState` (`internal/instruct/panelstate.go`), for a spec that is enrolled (§8 witness), read the
   kernel state through a seam that opens the bd store as `completeKernelStateFn` (`internal/complete/kernel.go`) does
   and `kernel.ReadState`, and report `kernel.CheckPassedGate` for the gate in place of `panel.PanelGateDecision`, whose
   step (9) in `internal/panel/gate.go` blocks on any REJECT whatever its refutations. On a bd read failure it reports
   a notice that names the error and no halt. `verdict()` stays pure, `HasIncompletePanel` stays file-system only (the
   SessionStart hook reads no bd), and `gate.go` and the pre-kernel path do not change.
6. Update the existing tests that the verify step changes (a refused round now gives a verify, not a fix or decision):
   `cmd/mindspec` `kernel_e2e_test.go`, `kernel_final_regression_test.go`, `panel_kernel_test.go`,
   `panel_repanel_later_test.go`, `kernel_canonical_test.go` and `panel_kernel_regression_test.go`, and
   `internal/next/kernel` `reject_test.go` and the final-gate tests, which lease and submit the verify.
7. Update workflow "Verify step" and "Rounds, decisions and halt". Text cites no internal record number.

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Verify|Reject|Final|SeedMatch)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134(VerifyCmd|Panel(Refute|Defer))_' -v` passes and lists the new tests.
- [ ] `go test ./internal/instruct/... -run '^TestS134Instruct_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/panel/... ./internal/instruct/... ./cmd/mindspec/...` passes.

**Acceptance Criteria**
- [ ] AC19 (instruct): a refuted REJECT closes the gate `passed` (`retally: "verify"`) and `mindspec instruct` reports
      no halt for it; a mutation that keeps `PanelGateDecision` on an enrolled gate fails.
- [ ] AC19 (shape): each listed bad verify record exits non-zero naming the condition, step open, with no deferral
      record for a failed `bead.CheckDeferral`; the refutation and deferral records name the verify step in
      `verify_step`; `mindspec_reproduced` lists only the reproduced findings.
- [ ] AC19 (veto files): with all slots closed, a stray REJECT `R9-round-1.json` gives the verify with one `veto`
      finding for that file. A `hard_block` REQUEST_CHANGES verdict gives only its `veto` finding, a `ccr[0]` return
      for it exits non-zero, and the fix brief carries its `ccr` texts.
- [ ] AC19 (incomplete): a VOID verdict, an INVALID verdict, and (all other slots APPROVE) a panel-directory commit
      that edits one closed slot's verdict file before the last slot submits each give no verify and the slot's `.a2`,
      whose APPROVE submit passes the gate. A mutation that creates the verify on an incomplete tally fails the test.
      R2 and R5 VOID on `.a1`-`.a3` and R3 APPROVE on `.a1` give no slot attempt and one
      `<w>.review.decide.uncounted.r1` whose `slots` is [R2 attempt 3, R5 attempt 3]; while it is open, a
      panel-directory commit edits R3's closed verdict file and the pass creates no R3 attempt; `redispatch` creates
      `.a4` of both and no R3 attempt; VOID on both `.a4` gives `.a5` of each and R3 `.a2`. Mutations that redispatch
      with no decision, serve one listed slot or R3, or leave the listed counts at 3 fail. With the same decision open,
      a commit S gives no judge; after `redispatch` and APPROVE on both `.a4`, the judge and no gate close (a mutation
      in which judge case (a) acts while the decision is open fails).
- [ ] AC19 (frozen): verify a1 reproduces f, the fix is leased by F, then commits edit a closed round-1 slot's verdict file and add
      a stray `R9-round-1.json`: `--apply` creates no `.verify.a2` and no slot attempt, the fix keeps its
      `mindspec_reproduced`, and after the fix closes `panel create 2` exits 0; a mutation that lets the changed-finding
      trigger act fails. A seeded open round-1 slot attempt closes `superseded` when round 2 is authorised. After a
      retally, `panel.json` still lists a retired verify's refutation (a mutation that filters the tallied `Panel` in
      place fails).
- [ ] AC20 (retired verify): a1 refutes a finding, then a commit and a `carry-over`: a2 reproduces it, a1's refutation
      discharges nothing, the gate stays open and the fix is created; a mutation that keeps a1's refutation fails.
- [ ] AC19 (outcomes): no `reproduced` return closes the gate `passed` with `retally: "verify"` and creates no fix, a
      refuted REJECT included; one reproduced REQUEST_CHANGES finding creates only the fix; one reproduced REJECT
      finding creates the fix and the reject decision. The same three at `<epic>.final`. A `panel refute` of g
      while the verify is open, then the verify refutes f and reproduces g: the gate stays open and the fix's
      `mindspec_reproduced` is [g]; a mutation that keeps that refutation fails. Then a second `panel refute` of g
      with other evidence appends a record and its row and the retally closes the gate `passed`; mutations
      that dedupe against all records or build the row id without `created_at` fail. At the allowance, a
      reproduced return creates only the limit decision, and `continue:1` creates the fix.
- [ ] AC8 (reducer): REJECT on item 2, `--apply`: the verify, whose findings include the `veto`, and no decision; a
      `deferred` veto return exits non-zero; after a verify that reproduces the veto, the decision whose `halt` text
      says it stops the spec; a refuted veto closes the gate `passed` and `CheckPassedGate` accepts it. A reproduced
      `hard_block` veto creates the fix and no decision.
- [ ] AC5 (part): with the verify open, a passing `panel refute` closes the gate `passed` and the verify `superseded`.
- [ ] Crash: findings f and g; a verify `submit` of record X (both `refuted`) killed after its close writes and
      before the receipt, then the same submit: one refutation record per finding with this `verify_step`,
      `mindspec_returns` equal to X and the receipt `at` of `mindspec_closing`. Fresh fixture, the same kill, then a
      submit of record Y (g `reproduced`) at a new commit: it exits non-zero naming `mindspec_closing`, the next pass
      creates `.verify.a2` with the same `mindspec_findings`, X's records discharge nothing, and after `.a2` closes with
      Y, which refutes f with evidence other than X's, the fix's `mindspec_reproduced` is [g] and the ledger holds f's
      refutation and row of each attempt. Fresh fixture, generation 1 killed the same way, `lease --take`, a generation-2 submit with f
      `reproduced`: it exits non-zero, the pass creates `.verify.a2`, and after `.a2` reproduces f, `panel refute`
      of g leaves the gate and the fix open. Mutations fail in which the rerun stamps a new `at` (the
      receipt `at` or a record's or row's `created_at` differs from `mindspec_closing.at`), a differing record is accepted, or
      the ledger key or row id has no `verify_step` (the `.a2` close is refused) or no `created_at` for an enrolled
      verb (AC19 (outcomes): the second `panel refute` append is refused).
- [ ] Encoding: golden ledger lines of a verify-close record (`"verify_step":"<step>"`, `"enrolled":true`), an
      enrolled verb record (`"enrolled":true`, no `verify_step`) and a pre-kernel record (neither), each with `record`,
      `spec` and `panel`, and their row ids (`:<verify_step>`, `:<created_at>`, today's id). A verify-close (bound)
      deferral, with a non-empty `verify_step`, keeps its slot covered in `checkCompletenessAt`. Mutations fail: a custom `MarshalJSON` on `Refutation` or
      `Deferral` (`record` dropped), and a `*string` `verify_step` (the slot uncovered).
- [ ] Bound: a round of 9 slots with 100 `ccr` items of 4096 bytes each gives the verify with each stored text in full
      (4096 bytes), and its all-`refuted` close writes `mindspec_returns`; a mutation that truncates the texts fails.
- [ ] Instruct: with the bd read failing, `mindspec instruct` prints the notice and no halt.
- [ ] `RoundsForGate` is the same before and after a verify close. `TestS134SeedMatch_Verify` matches a real verify
      close.

**Depends on**
Bead 14

## Bead 16: Change judge at a tally, re-judge and path cover

**Steps**
1. In `judge.go`, compute the accepted subject of an open gate (§13.2), with the panel-directory and kernel-directory
   exemption of Bead 14. `judge-create` skips a commit that a judge of the gate (any attempt, open or
   closed `passed`) has as its subject. Case (a) fires only when a tally or verify retally is due: every slot step of round N is closed `passed`, no
   `uncounted` decision of round N is open and round N is not frozen, or round N's current verify is closed `passed` and not applied (§13.2, §13.10); a frozen
   round gives no case (a) judge. Add the moved-subject guard: `tally`, `refused-round`, `verify-apply` and the
   `panel refute` and `panel defer` retally (`cmd/mindspec`) do not act on a gate while a judge is open or due;
   `judge-create` creates the judge (old subject = the accepted subject). While the fix of a frozen round is open, the
   guard does not hold the retally: it reads the verdict files on disk and binds the panel to round N's round
   subject; it passes only when `PassRule` passes, round N's current verify's `mindspec_returns` cover every finding the
   enumerator lists, and the round-N verdict files on disk are the set in the verify step's `mindspec_read`, each with the
   same sha256 (a changed, added or removed file fails it, committed or not, naming each path; §13.10);
   when it passes, the gate closes `passed` on round N's review subject, the fix closes `superseded`, and case (b)
   judges the later commits (after an open-gate `re-panel` judgement it authorises round N+1 and does not close the
   gate, step 3).
2. Apply the §6 subject model: add the review subject (`mindspec_round.judged` new subject, else `subject`); stamp it
   as `mindspec_subject` where the verify (`refusedRound`) and the fix (`fixWrite`, `reject.go`) are created; in
   `activeActorRound` (`submit.go`) compare a slot with the round subject, a verify or fix with the review subject and
   a lens with the authorised subject; panel evidence keeps the round subject; `checkFix` (`submit.go`; a commit
   past the fix step's subject that changes a path outside the panel and kernel directories, Bead 14, so the subject
   itself and a commit it contains fail), `gatePassEvidence`, `passed()` and `tally` (`reducer.go`) and
   `AcceptedSubject` and `effectiveSubjectAt` (`judge.go`; open gate: the review subject of `mindspec_round`, or, for a
   scoped round not yet registered, of its `prior`). The gate closes `passed` with the review subject in its receipt.
3. Fill the open-gate outcomes (§13.2): either value sets `judged` in the gate round record and retires the current
   verify that is not applied (§13.10) for `<gate>.r<N>.verify.a<k+1>` on the review subject; `carry-over` applies the held tally to the
   review subject; `re-panel` records the re-panel on the gate; a passing tally (`tally`), or a verify that leaves no
   reproduced finding (`verify-apply`), writes the round N+1 record itself (N+1 = the last registered round + 1; for a
   scoped round `roundOpen` skips the registered round N), copying the round N record into `prior` (§13.7: `verify` only for a verify closed `passed`) with `asks` and `read`
   by the §13.7 source rule (with `prior.verify`: findings and texts from its `mindspec_findings` joined with its
   `mindspec_returns` and the kept verb records, `read` copied from its `mindspec_read`; else from the files the rule
   read on disk), full texts, and writes no git object; a pass without `--apply` computes them and writes nothing; a passing `panel refute` or `panel defer` retally (`cmd/mindspec/panel_kernel.go`) does the same,
   with the `panel.json` the verb wrote, and does not close the gate; when it closes the verify `superseded`, `prior`
   has no `verify`; a refused tally creates the verify on
   the new subject. A round N+1 that a retally authorises past the allowance gives a limit decision, `.a<k+1>` after an
   earlier limit decision of round N (extend `decisionWrite`, which today no-ops on an existing id); its `continue:<n>`
   creates that round's lens step (§13.8). In `roundOpen`, when a closed fix authorises round N+1 over the round N
   record, copy that record, without its own `prior`, into `prior` with `asks` and `read` by the same rule (a closed
   fix always has a `prior.verify`, so `roundOpen` reads no verdict file), and keep `prior` on each later rewrite.
4. Replace the full-round restore after a `re-panel` (§13.7): a non-panel commit before `panel create` makes
   `judge-create` open a judge on the newest commit, old subject = the accepted subject; on its submit, close the
   unregistered successor `superseded` (open gate: withdraw the authorised round, restore `prior` as the gate round
   record), then apply §13.2, which retires the verify it names and creates one new attempt in the same pass. Bead 19 retires
   its lens attempt and gives the new authorisation a fresh one.
5. In the judgement check, compute `git diff --name-only` from the old to the new subject without the gate's panel
   directories and the kernel directory, and refuse a `re-panel` that misses a path, naming each. The judge artifact
   stays in a registered panel directory.
6. Update workflow "Change judge". Text cites no internal record number.

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Judge|Reducer|SeedMatch)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134(JudgeCmd|PanelRepanel|PanelRefute)_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/panel/... ./internal/validate/... ./cmd/mindspec/...` passes.

**Acceptance Criteria**
- [ ] AC20 (tally): a commit during round 1, then the last verdict: the judge, gate open, no gate close or verify. A
      commit that changes only a panel directory: no judge, the tally applies (Bead 14 covers kernel-record commits).
- [ ] AC20 (termination): after a passed-gate `re-panel`, the scoped round's authorisation, its passing tally and
      `complete` create no judge; on an open gate, after a `re-panel`, the verify retally with no reproduced return,
      the scoped round's authorisation and its tally create no judge. A new subject commit after either creates one.
      With `<2>.review.r2.fix` open after an applied verify, a fix commit, then `panel refute` of the
      reproduced finding: no open-gate judge and no `.verify.a2`; the gate closes `passed` on round 1's review subject,
      the fix closes `superseded`, and the next pass opens `<2>.review.judge.a1` under case (b). A mutation in which
      case (a) acts on a frozen round fails. Three fresh fixtures with that fix open, then `panel refute` of
      f: (i) the verify reproduced only f (R5's `ccr[0]`, other slots APPROVE), a commit edits R2's round-1 verdict;
      (ii) the same verify, a commit adds an APPROVE `R9-round-1.json`; (iii) the verify reproduced f and the `veto` of
      an off-roster INVALID `R9-round-1.json` (fix, no decision), a commit removes it; (iv) as (iii), but the removal
      is not committed; (v) as (iii), but R9 is never committed and is then moved off disk. In each `panel refute`
      exits 0 and the gate and the fix stay open. (i) fails on its digest whatever the coverage check does; a retally
      with no digest check fails (ii); a check over the files present only fails (iii); a set check over the HEAD tree
      fails (iv); a check against the verify record's commit fails (v). Open gate, round 1 refused on a moved subject, `re-panel`
      2, the verify on the new subject reproduces f, the fix open, then `panel refute` of f: the fix closes
      `superseded`, round 2 of 2 slots is authorised, `prior.verify` names the verify and the gate stays open. A
      mutation that compares the panel's subject with the review subject fails.
- [ ] AC20 (outcomes): `carry-over` with a passing tally closes the gate `passed` on the new subject, `passed()` and
      `CheckPassedGate` accept it and `complete` exits 0; with a refused tally, the verify subject is the new subject
      J, and after a `reproduced` return a fix `submit --commit J` exits non-zero and a commit past J exits 0.
      `re-panel` 2 with all paths and a passing tally: round 2 of 2 slots on the gate, scoped to the paths; the same
      after a verify with no reproduced return, and after a refused tally and `panel refute` of each finding
      while the verify is open (the verify closes `superseded`, `prior` has no `verify`, the gate stays open; a mutation
      in which the retally closes the gate fails). At the allowance under `re-panel` 2, the verify reproduces f and
      gives `decide.limit.r<N>`; `panel refute` of f closes it `superseded` and gives `decide.limit.r<N>.a2`;
      `continue:1` creates the lens step of round N+1 and no fix, and after its close `panel create` exits 0. Mutations
      that reuse the superseded decision id or make `continue` create a fix fail.
- [ ] AC20 (verify across a judge): a verify open, then a commit and a `carry-over`: the verify closes `superseded`,
      `.verify.a2` opens on the new subject, `mindspec_round.verify` names it, and a receipt of a1 neither passes the
      gate nor creates a fix; with a1 receipted before the judgement, a1 keeps its receipt and is retired.
- [ ] AC20 (withdrawn round): open gate, `re-panel` 2 on J, a verify with no reproduced return authorises round 2
      (`prior` = round 1), a commit S before `panel create`: the judge's old subject is J and its path cover starts at
      J (a mutation that reads the round 2 record's subject fails); its `re-panel` withdraws round 2, restores `prior`,
      opens one `.verify.a2` on S, and an all-`refuted` a2 authorises round 2 again on S with the latest judgement's
      slot count and scope and a fresh lens attempt; the gate stays open until that round passes. A mutation that
      closes the gate after a2 fails.
- [ ] AC19 (prior): after `<2>.review.r2.fix` closes, `mindspec_round.prior.verify` names `<2>.review.r1.verify`
      and `prior` holds `asks` and `read` equal to the verify's `mindspec_read`; a mutation in which `roundOpen` writes
      no `prior` fails. With the fix leased, f's verdict file replaced by APPROVE, a stray verdict added and no copy of
      the original left on disk: after the fix closes, `prior.asks` holds the verified findings only; a mutation that
      enumerates the files on disk when the fix closes fails. A pass with and
      without `--apply` leaves `git count-objects` unchanged, and the pass without `--apply` writes no bd record.
- [ ] AC20 (cover): a `re-panel` whose paths miss one changed path exits non-zero naming it, step open.
- [ ] AC10 (verify): a leased verify raw-closed with an all-`refuted` record: after `submit --repair` the gate closes
      `passed` (`retally: "verify"`), with the refutation records, and no `.verify.a2`; the retry of a verify raw-closed
      with no record, once submitted, closes the gate the same way, and the retry of a fix raw-closed with no commit,
      once submitted, authorises round 2. A verify retry that reproduces f, the fix open, then a commit edits a round-1
      verdict file and `panel refute` of f: the frozen-round retally compares the retry's `mindspec_read`
      and fails naming the path. Mutations in which the repair writes only the receipt, or the retry gets a wrong
      `mindspec_read`, fail.
- [ ] AC20 (re-judge): after a passed-gate `re-panel` and before `panel create`, a commit: no full round; a judge on
      the newest commit whose old subject is the accepted subject; on `carry-over`, `<gate>.a2` closes `superseded` and
      the frozen gate's receipt adds the carry-over.
- [ ] AC5 (part): with a judge open, a passing `panel refute` retally leaves the gate open.
- [ ] AC18 (c) still holds with paths that cover the diff. `TestS134SeedMatch_Judge` matches an open-gate judge, and
      `TestS134SeedMatch_Authorise` a verify that leaves no reproduced finding under a `re-panel`.

**Depends on**
Bead 14, Bead 15, Bead 17

## Bead 17: Late submissions and repair receipts

**Steps**
1. In `lease.go`, on `--take`, write the take event (§5).
2. In `submit.go`, apply the §6 generation rule for `submit` and `panel submit` (`cmd/mindspec/panel_submit.go`):
   when the step is open and no newer generation has a receipt or (verify) a `mindspec_closing`, write `generation` and `superseded_generation` in the receipt
   and set the current lease `ended: "superseded"`; else refuse naming the condition. A different result for a closed
   step refuses with the step metadata unchanged (no `conflict`).
3. Add `submit <id> --repair --reason <text> (--artifact | --commit)` for an unreceipted close only: run the §6
   checks, append a repair event (`mindspec_takes`, `via: "repair"`) with the current generation, issue no generation,
   run the kind's close-write hook (Bead 14), then write the receipt `repaired: true`, `via: "repair"`. The §9 receipt with no reopen runs the hook before its receipt too. Refuse it, naming the condition, on an open step, on a step that was never leased (generation 0), on a
   step with a valid receipt, and on evidence that does not resolve or fit its kind and subject.
4. In `closes.go`, before a retry, check the evidence at the step's fixed location (§9); when the step was leased and
   it fits and its `mindspec_closing` has no `conflict` and no `generation` other than the current one, create no
   `.a<k+1>` and add a new `submit --repair` run entry (none exists today, §9). In `retryAttempts`, copy the keys of the kind (§9): add
   `mindspec_reproduced` for a fix and `mindspec_findings` and `mindspec_read` for a verify, and point `mindspec_round.lens` or `mindspec_round.verify` at a lens or verify
   retry. The pass, `panel create` and `submit` read the highest attempt not closed `superseded` of a slot or fix and
   the attempt that `mindspec_round` names of a lens or verify.
5. Update `_HelpFlags` and `HelpGolden`, and workflow "lease and submit".

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Lease|Submit|Closes)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134(LeaseCmd|SubmitCmd|PanelSubmit|VerbStub)_|TestCeremonyNonInflation_HelpFlags' -v` passes.
- [ ] `go test -short ./internal/next/... ./cmd/mindspec/...` passes.

**Acceptance Criteria**
- [ ] AC21: a generation-1 submit after a take is accepted on an open step with an unchanged subject; the receipt names
      generation 1 and the superseded one; generation 2's lease ends `superseded`. After a generation-2
      submit the generation-1 submit exits non-zero naming the condition, with no `mindspec_closing.conflict`, no
      `.a2` after the next pass and the receipt unchanged; on a seeded verify, a generation-1 submit after a
      generation-2 `mindspec_closing` exits non-zero naming it, with no `conflict`, no `.a2` after the next pass and the
      pin unchanged (a lens has no pin: Bead 19 AC21). Mutations that write the conflict on the receipted step or from
      the refused revoked token fail.
- [ ] AC4 (part): after a take and a generation-2 submit, a generation-1 submit of a different result exits non-zero,
      the step closed, its receipt unchanged, no `conflict` written and no `.a2` after the next pass; the same through
      `panel submit`, and on a seeded verify.
- [ ] AC10 (part), with the Bead 14 seeder `Verify` and the generic retry in `closes.go`: a raw close of a leased slot with a fitting verdict at its registered path: no `.a2`, a `run` entry `submit
      <slot> --repair --reason`; it exits 0 with `via: "repair"`, its repair event has the current
      generation and the lease generation does not change. With no verdict, or a verdict on another subject, `.a2`.
      `submit --repair` on an open step, on a step with a valid receipt, and with a verdict of another subject exits
      non-zero, step unchanged; a leased, raw-closed verify with its `<spec dir>/kernel/<step id>.json` record gets its
      receipt, and a seeded lens or verify repair runs the kind's close-write hook before the receipt (a recording hook;
      a mutation in which the repair writes only the receipt fails); a never-leased one gets no `run` entry, `submit
      --repair` exits non-zero and the pass creates `.verify.a2`.
- [ ] AC10 (retry): on seeded steps (seeders `Lens`, `Verify`), raw-close a lens, a fix and a verify with no fitting
      evidence: the lens retry is the step `mindspec_round.lens` names, the fix retry keeps `mindspec_reproduced`, the
      verify retry is the step `mindspec_round.verify` names and keeps `mindspec_findings` and `mindspec_read` (Bead 16
      tests the frozen-round retally over it). A verify whose `mindspec_findings` is over 128 KiB retries through
      `Create` with `--metadata @<file>`. A mutation that copies only subject, round, registration and decision fails
      (Beads 16 and 19 test that the pass proceeds after each retry).

**Depends on**
Bead 14

## Bead 18: Briefs rendered by the binary

**Steps**
1. Add `internal/next/kernel/brief.go`: render the lens, verify, fix and change-judge briefs and the review-slot body
   from durable records (§13.11). The verify brief reads round N's findings, `veto` included; the fix brief reads only
   `mindspec_reproduced`, with the `ccr` texts of a reproduced `veto` verdict; the slot body lists the files in scope (§13.11: for round N>1 also the paths changed since
   the prior round's subject), the prior round's asks (§13.11, from `mindspec_round.prior`, which every round N+1 authorisation writes): rendered
   from `prior.asks` alone (Bead 16 writes it): each round-N finding with its text and its verify return, or the verb
   refute or defer records with an empty `verify_step` that the §13.10 filter keeps; it reads no verdict file, panel
   directory or commit; and the line on refuted or deferred findings. For `<epic>.final`, read the summary from the spec, the context from the epic's work items and
   the diff from the final base.
2. Add the read-only verb `mindspec brief <step>` (`cmd/mindspec/brief.go`); it refuses a step that is not an actor
   step of an enrolled spec.
3. Set each action's `brief` argv (§13.11) in `reducer.go`.
4. As the `run` entry of a kernel gate (bead or final gate of an enrolled spec), `panel create` passes the generated
   body to `panel.CreateInput` (a new `BriefBody` field; `internal/panel` stays a leaf) and refuses `--brief-file`.
   Every other `panel create` (a pre-kernel spec, the spec and plan gates, `--gate adhoc`) keeps the stub and
   `--brief-file`.
5. Update `_HelpFlags`, `HelpGolden` and workflow "Briefs".

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134Brief_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134(BriefCmd|PanelCreate|PanelPrompt)_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/panel/... ./cmd/mindspec/...` passes (includes `TestPanelLeafImports_StdlibPlusTermsafeOnly`).

**Acceptance Criteria**
- [ ] AC22 (briefs): after the `panel create 1` entry (no `--brief-file`), `panel prompt` of each slot exits 0 with the
      managed header and the files in scope; `--brief-file` exits non-zero. `brief` of a verify, fix and change judge
      prints the §13.11 items; the fix brief names no refuted finding.
- [ ] AC19 (part): the round-2 slot body shows each round-1 finding with its verify return and the paths changed since
      round 1's subject.
- [ ] AC19 (frozen brief): with the fix leased, f's verdict file is replaced by APPROVE, a stray `R9-round-1.json`
      is added and no copy of the original is left on disk; after the fix closes, the round-2 slot body shows the findings
      and returns that the verify pinned and no text from the edit or the stray file; f's 4096-byte text has a
      requirement after byte 2048, and the body shows it. Mutations that read the verdict files at the branch head,
      enumerate the files on disk when the fix closes, or truncate the stored text, fail. After `git gc --prune=now` and an edit of each round-1 verdict file on disk, the round-2 slot body is
      unchanged; a mutation that reads the panel directory at render fails.
- [ ] AC20 (passed prior): open gate, round 1 refused on a moved subject, `panel refute` of its finding while the
      judge is open and no commit after it, `re-panel` 2, then a commit replaces that round-1 verdict with APPROVE
      before `panel create 2`: the round-2 slot body shows the round-1 finding with its refutation. Mutations that read
      the asks only from `prior.verify`, read the panel directory at the head when `panel create` renders, or read
      `panel.json` at the pre-refutation HEAD, fail. After a refuted-each-finding retally with the verify open
      (Bead 16), the round-2 body lists each round-1 finding with its refutation; after the frozen retally under a
      `re-panel` (Bead 16), it shows f with its return `reproduced` and the later refutation.
- [ ] AC22 (final): at `<epic>.final`, the slot body's summary is from the spec and its files in scope are the diff
      from the final base.
- [ ] On a pre-kernel spec, and with `--gate adhoc` on the enrolled spec, `panel create` with a filled `--brief-file`
      exits 0; with the stub, `panel prompt` refuses.

**Depends on**
Bead 15, Bead 16

## Bead 19: Lens step

**Steps**
1. Add `lens.go` and fill `lens-create`: when a round is authorised (round opening, final round 1, a scoped round),
   create `<gate>.r<N>.lens` on the authorised subject and write its id to `mindspec_round.lens`. On each later
   authorisation of round N (a re-judge, a new slot count) and each move of its authorised subject before `panel
   create`, create `.lens.a<k+1>` on the current authorised subject (`mindspec_from` = the prior attempt), close an
   open prior attempt `superseded`, retire a passed one with its receipt unchanged, and point `mindspec_round.lens` at
   the new attempt (§13.7). Add the scoped-round selection (§13.11) that `panel create`
   (`cmd/mindspec/panel_kernel.go`, today `PanelGateReviewerSlots(gate, size)`) and `kerneltest/seed.go` use: from the
   configured slots in roster order, each adversary slot (configured lens `adversarial`) up to n, then the first other
   slots up to n; renumber them `R1`..`Rn` in configured order, each with its configured family, model and lens. The
   ledger, `panel.json` and the `R<i+1>` checks do not change. The adversary slot of a round is the roster entry with
   lens `adversarial`. A round of only adversary slots still gets its lens step (empty `lenses`).
2. Register the lens artifact check for `submit` (§13.11, shape only): no unknown field, one non-empty lens per slot of
   the round except an adversary slot, none for it; each refusal names the slot.
3. Make `panel create` resolve the lens step from `mindspec_round.lens` and refuse while it is not closed `passed`,
   naming it; list `panel create` in `run` only after the lens close. The lens close-write hook (Bead 14; it runs
   before the receipt on every receipt path) replaces `mindspec_round.lenses` whole with its lenses; `panel create` writes it from there to a new `focus` field of `RosterSlot` (`internal/panel/panel.go`, omitted when empty); `panel prompt`
   (`cmd/mindspec/panel_prompt.go`) prints Focus from it, else from the configured lens text. The adversary slot has
   no `focus`.
4. Update the existing tests that the lens step changes (`panel create` needs a closed lens step): the
   `kerneltest` seeders create a closed lens step for each authorised round, and `cmd/mindspec` `kernel_e2e_test.go`,
   `kernel_final_regression_test.go` and `panel_kernel_test.go` lease and submit it before `panel create`.
5. Update workflow "Lens step".

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134(Lens|SeedMatch)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS134Panel(Create|Prompt|Repanel)_' -v` passes.
- [ ] `go test -short ./internal/next/... ./internal/panel/... ./cmd/mindspec/...` passes.

**Acceptance Criteria**
- [ ] AC22 (lens): each bad lens record exits non-zero naming the slot, step open; `panel create 1` before the lens
      close exits non-zero naming it; after it, `panel prompt` shows each slot's lens and the adversary slot's
      configured lens.
- [ ] AC17 (part): after `submit <1>.impl.a1`, `--apply` creates `<1>.review.r1.lens` and `run` has no `panel create 1`.
- [ ] AC10 (lens): a leased lens step raw-closed with a fitting record: after `submit --repair`, `panel create` exits 0
      and each slot's `focus` is its drafted lens (a mutation in which the repair writes only the receipt fails); a lens
      raw-closed with no record gives the retry that `mindspec_round.lens` names, and after its submit `panel create`
      exits 0.
- [ ] A scoped `re-panel` round of 3 slots on the default roster (adversary fourth) has roster R1, R2, R3 (R3 =
      configured slot 4, lens `adversarial`); its lens step takes lenses for R1 and R2 only; `panel create`, the slot
      steps, each slot `submit`, the tally, `CheckPassedGate` and `complete` exit 0 on it, and a mutation that takes
      the first n configured slots fails. A roster with no adversary slot takes its first n slots, each with a lens;
      two adversary slots with n=2 give an empty `lenses` list.
- [ ] Re-judge: with the lens step open, a re-judge closes it `superseded`; with it closed `passed`, its receipt does
      not change. Each new authorisation creates `.lens.a2`, `mindspec_round.lens` names it and `panel create` reads
      it; a `re-panel` with another slot count gives `.lens.a2` that count's entries.
- [ ] AC21 (lens): a lens step leased at generation 1, `lease --take`, then a commit moves its authorised subject: the
      pass closes it `superseded` and opens `.lens.a2`; the generation-1 `submit` exits non-zero; a `submit` to `.a2`
      and `panel create` exit 0.
      Crash: a generation-1 lens submit of L1 killed after its close write and before the receipt, `lease --take`,
      then a generation-2 submit of L2 exits 0, and after `panel create` each slot's `focus` is from L2. A
      generation-1 submit after the generation-2 receipt and before `panel create` exits non-zero, leaves
      `mindspec_round.lenses` equal to L2, and the next pass creates no `.lens.a2`; a mutation that makes the close
      write before the closed-step refusal fails. A mutation that pins a lens (the
      generation-2 submit refused) fails.
      `TestS134SeedMatch_Lens` matches a real lens close.

**Depends on**
Bead 17, Bead 18

## Bead 20: Audit rows

**Steps**
1. In `audit.go`, add the rows of contracts §11 to `names`: `verify-reproduced`, `verify-refuted`, `verify-deferred`,
   `late-submissions`, `repair-receipts` and `lens-steps`, each from its record key; Count is the number of records the
   row names, and `lens-steps` gives one Details entry `<gate> r<N>: <k>` per gate round. The takeovers row counts every take and
   repair event (today only those with `Generation > 0`); the unreceipted-closes row counts receipts with `repaired`.
2. In `implCleanupArtifact` (`internal/approve/impl_final_gate.go`), accept `<spec dir>/kernel/*.json` as a
   finalize-export artifact.
3. Update workflow "Close flags and audit".

**Verification**
- [ ] `go test ./internal/next/kernel/... -run '^TestS134Audit_' -v` passes and lists the new tests.
- [ ] `go test ./internal/approve/... -run '^TestS134ImplFinalGate_' -v` passes and lists the new tests.
- [ ] `go test -short ./internal/next/... ./internal/approve/... ./cmd/mindspec/...` passes.

**Acceptance Criteria**
- [ ] AC11 (audit): records of each new event give `impl approve` and a rerun exit 0, printing the spec's AC11 counts
      (takeovers 2, unreceipted closes 2, deferrals 2 from one `panel defer` and one `deferred` verify return, 1 in
      each other row, the six new rows included) and `lens-steps` equal to the fixture's gate rounds. A producer that
      writes another key, and a mutation that counts all verify returns in one row, make the test fail.
- [ ] A bead worktree whose only dirty paths are kernel records gives the export-only recovery
      (`TestS134ImplFinalGate_`).

**Depends on**
Bead 15, Bead 17, Bead 19

## Bead 21: End-to-end loop, skills and docs

**Steps**
1. Update `kernel_e2e_test.go` so the loop drives the lens step, `panel create`, slots, a refused round, its verify, the
   fix and the next round, with `lease`, `next` and each brief from `brief` or `panel prompt`.
2. Rewrite `ms-bead-cycle`, `ms-spec-autopilot`, `ms-bead-impl`, `ms-bead-fix` and `ms-panel-tally` under
   `plugins/mindspec/skills` as harness adapters of at most 50 lines (spec R12): loop on `next-action --apply`; for
   each actor step, `lease`, then start a fresh agent with the output of the action's `brief` argv; return through
   `submit` and each `run` entry. Copy each to `.claude/skills` byte-equal, and add each old text to
   `internal/setup/historical_skills` so existing installs refresh. Remove each skill pin test assertion that pins
   removed text (`internal/setup/*_test.go`, `internal/lint/destructive_guidance_test.go`), or move it to the binary
   output that now carries the rule.
3. In `internal/rules/rules.yaml`, add rule `dispatch.fresh-agent` (scope orchestrator, `enforced_by: advisory`)
   with the dispatch rule of spec R8. State that `review.reproduce` applies to a pre-kernel spec and that an enrolled
   spec runs the verify step. Add the new id to `internal/rules/rules_test.go`.
4. Update `project-docs/user/guides/autonomy.md` and workflow "Kernel loop". Fix a defect found here in its bead's
   files and list each fix in the commit message.

**Verification**
- [ ] `go test ./cmd/mindspec/... -run '^TestS134E2E_' -v` passes and lists the new tests.
- [ ] `go test -short ./...` passes; `cmp` of each skill pair exits 0.

**Acceptance Criteria**
- [ ] AC17: only `next-action --apply`, `lease`, `submit` and `run` entries reach `impl approve` exit 0, with the lens
      and verify steps on the path.
- [ ] AC11: every row, old and new, prints the spec's AC11 count through its real verb.
- [ ] AC19-AC22 hold on fixtures made by this binary.
- [ ] AC15, AC16 (skills): the five adapters pass the line, word and `cmp` checks; no `SKILL.md` grows against
      `371dc360`; `mindspec instruct` renders `dispatch.fresh-agent`.

**Depends on**
Bead 14, Bead 15, Bead 16, Bead 17, Bead 18, Bead 19, Bead 20

## Provenance

Each `-run` pattern runs in the package its bead's Verification names.

| Acceptance Criterion | Bead(s) | Verified By |
|---------------------|---------|-------------|
| AC1 | 6 | `-run '^TestS134NextAction_'` |
| AC2 | 5, 6, 12 | `-run '^TestS134Enrol_'` (enrolment, a, b, d, e, fixture); `-run '^TestS134NextAction_'` (apply); `-run '^TestS134(EnrolFault|Complete)_'` (c, complete pass) |
| AC3 | 7, 10, 12 | `-run '^TestS134Lease'`; `-run '^TestS134PanelSubmit_'`; `-run '^TestS134Next_'` |
| AC4 | 7, 17 | `-run '^TestS134(Submit|SubmitCmd|PanelSubmit)_'` |
| AC5 | 5, 10, 15, 16 | `-run '^TestS134Enrol_'`; `-run '^TestS134Panel(Create|Refute|Defer)_'` |
| AC6 | 1, 5, 6, 12 | `-run '^TestS134(Witness|VerbStub)_'`; `-run '^TestS134(Enrol|Derive)_'`; `-run '^TestS134NextAction_'`; `-run '^TestS134(Complete|Next)_'` |
| AC7 | 2, 6, 8, 10 | `-run '^TestS134Rounds_'`; `-run '^TestS134Allowance_'`; `-run '^TestS134Reject_'`; `-run '^TestS134PanelCreate_'` |
| AC8 | 8, 10, 15 | `-run '^TestS134Reject_'`; `-run '^TestS134PanelCreate_'` |
| AC9 | 7, 8, 10, 12, 14 | `-run '^TestS134(Lease|Submit)_'`; `-run '^TestS134Halt_'`; `-run '^TestS134PanelCreate_'`; `-run '^TestS134(Next|Complete)_'`; `-run '^TestS134(Reducer|Submit)_'` (14) |
| AC10 | 7, 9, 12, 17 | `-run '^TestS134Submit_'`; `-run '^TestS134(Closes|ImplApprove)_'`; `-run '^TestS134Complete_'` |
| AC11 | 2, 9, 12, 13, 20, 21 | `-run '^TestS134Friction_'`; `-run '^TestS134(Audit|ImplFinalGate)_'`; `-run '^TestS134Complete_'`; `-run '^TestS134E2E_'` |
| AC12 | 2, 5, 13 | `-run '^TestS134Config'`; `-run '^TestS134Enrol_'`; `-run '^TestS134E2E_'` |
| AC13 | 1, 5, 6, 7, 9, 10, 11, 12, 13 | `-run '^TestS134Store_'`; `-run '^TestS134Enrol_'`; `-run '^TestS134(Flags|NextAction)_'`; `-run '^TestS134Lease'`; `-run '^TestS134(Closes|ImplApprove)_'`; `-run '^TestS134Panel'`; `-run '^TestS134JudgeCmd_'`; `-run '^TestS134(Next|Complete)_'`; `-run '^TestS134E2E_'` |
| AC14 | 1, 6, 7, 10 | `-run '^TestS134Store_'`; `-run '^TestS134(Flags|NextAction)_'`; `-run '^TestS134Submit_'`; `-run '^TestS134PanelCreate_'` |
| AC15 | 1, 2, 4, 5, 13, 21 | `-run '^TestS134Setup_'`; domains `git grep` (Beads 2, 13); the Bead 4 `git grep`, `cmp` and `-run '^TestS134Skills_'`; `-run '^TestS134Enrol_'`; the Bead 21 skill checks |
| AC16 | all; 3; 21 | each bead's message tests; `-run '^TestS134Surface(Matcher|DiffFn)_'` (red mutations); `-tags surfacescan -run '^TestS134SurfaceBranch_'`; the Bead 21 `git diff --numstat` check |
| AC17 | 13, 19, 21 | `-run '^TestS134E2E_'` |
| AC18 | 10, 11, 12, 13 | `-run '^TestS134Panel(Create|Repanel)_'`; `-run '^TestS134(Judge|JudgeCmd)_'`; `-run '^TestS134(Complete|ImplFinalGate)_'`; `-run '^TestS134E2E_'` |
| AC19 | 15, 18, 21 | `-run '^TestS134(Verify|VerifyCmd)_'`; `-run '^TestS134Brief_'`; `-run '^TestS134E2E_'` |
| AC20 | 14, 15, 16, 19, 21 | `-run '^TestS134Contract_'`; `-run '^TestS134Verify_'`; `-run '^TestS134(Judge|JudgeCmd)_'`; `-run '^TestS134Lens_'`; `-run '^TestS134E2E_'` |
| AC21 | 17, 19 | `-run '^TestS134(Submit|SubmitCmd|PanelSubmit)_'`; `-run '^TestS134Lens_'` |
| AC22 | 18, 19, 21 | `-run '^TestS134(Brief|BriefCmd|PanelPrompt)_'`; `-run '^TestS134(Lens|PanelCreate)_'`; `-run '^TestS134E2E_'` |
| Validation proofs | final review | the Testing Strategy "Gate" commands on the spec branch |
