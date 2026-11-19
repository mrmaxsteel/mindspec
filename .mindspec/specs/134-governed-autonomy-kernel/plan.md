---
status: Draft
spec_id: 134-governed-autonomy-kernel
version: "1"
adr_citations: [ADR-0045, ADR-0023, ADR-0040, ADR-0038, ADR-0037, ADR-0035, ADR-0043]
work_chunks:
  - {id: 1, depends_on: [], key_file_paths: [internal/next/kernel/kernelid/kernelid.go,
      internal/next/kernel/kernelid/witness.go, internal/next/kernel/idmode.go, internal/next/kernel/store.go,
      internal/next/kernel/records.go, internal/next/kernel/clock.go, internal/next/kernel/state.go,
      internal/next/kernel/contract.go, internal/next/kernel/pass.go, internal/next/kernel/lease.go,
      internal/next/kernel/submit.go, internal/next/kernel/kerneltest/fixture.go,
      internal/next/kernel/kerneltest/repo.go, internal/next/kernel/kerneltest/seed.go, cmd/mindspec/next_action.go,
      cmd/mindspec/lease.go, cmd/mindspec/submit.go, cmd/mindspec/ceremony_guard_test.go, internal/phase/phaselock.go,
      internal/bead/config.go, internal/harness/sandbox.go, internal/doctor/beads.go, .beads/config.yaml,
      .github/workflows/ci.yml, scripts/ci-install-bd.sh]}
  - {id: 2, depends_on: [], key_file_paths: [internal/config/config.go, cmd/mindspec/config.go,
      internal/redact/redact.go, cmd/mindspec/selfemit.go, cmd/mindspec/ceremony_guard_test.go,
      internal/panel/rounds.go]}
  - {id: 3, depends_on: [], key_file_paths: [cmd/mindspec/surface_scan_test.go,
      cmd/mindspec/surface_scan_branch_test.go, .mindspec/adr/ADR-0035-agent-error-contract.md,
      cmd/mindspec/adr0035_amendment_test.go]}
  - {id: 4, depends_on: [], key_file_paths: [.claude/skills/ms-bead-cycle/SKILL.md,
      plugins/mindspec/skills/ms-bead-cycle/SKILL.md, .claude/skills/ms-spec-autopilot/SKILL.md,
      plugins/mindspec/skills/ms-spec-autopilot/SKILL.md, internal/setup/skills.go,
      project-docs/user/guides/autonomy.md, README.md, plugins/mindspec/README.md]}
  - {id: 5, depends_on: [1, 2], key_file_paths: [internal/approve/plan.go, internal/approve/enrol.go,
      internal/phase/derive.go, internal/phase/lineage.go, internal/next/mode.go, .mindspec/adr/ADR-0023.md]}
  - {id: 6, depends_on: [1, 2], key_file_paths: [internal/next/kernel/reducer.go,
      internal/next/kernel/allowance.go, internal/next/kernel/output.go, internal/next/kernel/flags.go,
      internal/next/kernel/nextaction.go, internal/panel/tally.go]}
  - {id: 7, depends_on: [1], key_file_paths: [internal/next/kernel/lease.go, internal/next/kernel/submit.go,
      cmd/mindspec/lease.go, cmd/mindspec/submit.go, internal/rules/rules.yaml]}
  - {id: 8, depends_on: [6, 7], key_file_paths: [internal/next/kernel/reject.go, internal/next/kernel/halt.go,
      internal/next/kernel/final.go]}
  - {id: 9, depends_on: [2, 6], key_file_paths: [internal/next/kernel/closes.go, internal/next/kernel/audit.go,
      internal/approve/impl.go, internal/approve/impl_final_gate.go]}
  - {id: 10, depends_on: [2, 6, 7], key_file_paths: [cmd/mindspec/panel.go, cmd/mindspec/panel_defer.go,
      cmd/mindspec/panel_rebind.go, internal/panel/create.go, internal/panel/submit.go, internal/panel/refute.go]}
  - {id: 11, depends_on: [6, 7], key_file_paths: [internal/next/kernel/judge.go]}
  - {id: 12, depends_on: [5, 6, 7, 8, 9, 10, 11], key_file_paths: [cmd/mindspec/next.go, internal/next/select.go,
      internal/complete/complete.go, internal/complete/panel_presence.go, internal/approve/plan.go,
      internal/approve/impl_final_gate.go, internal/approve/lifecycle_gate.go,
      internal/harness/lifecycle_scenario_test.go]}
  - {id: 13, depends_on: [8, 9, 10, 11, 12], key_file_paths: [cmd/mindspec/kernel_e2e_test.go]}
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

Thirteen beads; one bead is one session. Bead 1 fixes every contract the other beads build against, so most beads wait
only for Bead 1 and run side by side. The critical path is 1 -> 6 -> {8, 9, 10, 11} -> 12 -> 13 (five beads). Kernel
code goes in `internal/next/kernel`, which the workflow manifest owns. Bead 1 is the largest bead: a split adds a chain
link. The CI bd install stays in it: its tests fail without bd, so both land in one merge; alone it is trivial work.

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
before closes, `WithCASRetry` on each status write). On an empty registry it writes nothing; it returns every rule and
store error. Entry points and verbs call `Pass(kernel.Rules(), ...)`; `Rules()` is `DefaultRegistry` unless
`kerneltest.WithRules(t, reg)` replaces it for one test (restored by `t.Cleanup`; no `t.Parallel`). Bead 1's dispatcher
tests use their own `NewRegistry`, so later rules do not change them; Bead 7's failing rule and Bead 10's counting rule
use `WithRules`. A test over the default registry asserts only writes and output that no later rule bead changes.
**Modes.** A rule's mode is `ordinary`, `halt` or `flag`. A `flag` rule returns flags only; a write from it makes `Pass`
return an error. When `State.Halted()`, `Pass` runs only `halt` and `flag` rules, in order, reports `halted`, and with
`apply` writes only the `halted` closes and receipts of `halt` rules; it creates no bead. Flags are read-only, so a
halted read shows every flag. Otherwise `Pass` runs every rule. No bead after Bead 1 edits the dispatcher. **Rule
list.** Bead 1 owns the list in `contract.go`: each name with its mode, in this order (owner in brackets). `ordinary`:
`missing-link` (6), `recreate` (6), `tally` (6), `supersede` (8), `refused-round` (8), `final-round` (8), `judge-apply`
(11), `judge-supersede` (11), `judge-create` (11), `round-open` (6), `slots` (6), `implement` (6), `retry-attempt` (9),
`complete-entry` (6). `halt`: `halt` (8). `flag`: `run-marker-missing`, `foreign-child-open`, `foreign-blocker`,
`id-collision` (6), `unreceipted-close`, `raw-close`, `forced-close`, `raw-claim` (9). Later beads only register a
listed name with its listed mode; they do not edit the list. **Seeders.** Bead 1 adds the `kerneltest` seeders
`PassGate`, `Authorise` (an authorised round: round, subject, slot count, paths), `OpenDecision`, `Answer`, `Register`
and `Repaired` (a repaired receipt). The bead that builds a producer pins the seeder with
`TestS134SeedMatch_<seeder>[<case>]`: real producer vs seeder on twin fixtures, normalising only bead ids, wall-clock
times and token hashes and comparing every other metadata key and value; on a mismatch the bead corrects the seeder.
Pins: `Enrolled` Bead 5; `PassGate` Bead 6; `Authorise` each producer: Bead 6 (closed implement step, closed fix), Bead
8 (supersede, final round 1), Bead 11 (`re-panel`); `Answer`, `Repaired` Bead 7; `OpenDecision` (reject and limit kinds)
Bead 8; `Register` Bead 10. Bead 6's round opening is the one writer of the authorisation on a fix close; Bead 8 creates
the fix only.

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

## Testing Strategy

- **Test names.** Each new test starts with `TestS134<Mechanism>_`; no base-tree test matches `S134`. Each
  Verification line with a `TestS134` prefix must list its new tests in `go test -v` output. No `TestS134` test reads
  `testing.Short()`.
- **Green between beads.** Each bead's Verification runs `go test -short` on the whole of every package it touches,
  plus the named pin tests. A bead builds and keeps `go test -short ./...` green on its own merge, in any order its wave
  allows. A bead that changes a pinned surface updates the pin test in the same bead. Pins:
  `TestCeremonyNonInflation_ConfigKeys`, `_HelpFlags`, `_NextFlags` (`cmd/mindspec/ceremony_guard_test.go`);
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
  the id resolver `ID` and the seeders (pinned as Contracts first says). A bead that corrects a seeder runs every
  package with a consumer of it; a later bead of its wave rebases onto the correction.
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
  expiry, token hash, claim base), gate round (round, subject, slot ids), take reason, decision answer, defer,
  judgement, and re-panel (slot count, paths). Producers write and the audit reads only through these types. A golden
  JSON test pins each shape.
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
  submit" (7), "Close flags and audit" (9), "Panel verbs" (10). workflow `architecture.md`: "Enrolment" (5), "Rounds,
  decisions and halt" (8), "Change judge" (11). workflow `runbook.md`: "Kernel loop" (12, then 13).
- **Dogfood boundary.** The lane runs lifecycle and panel verbs on the released binary, never the spec-branch binary.
  The lane session runs in a worktree and loads `.claude/skills` from it, so after Bead 4 merges its `ms-bead-cycle`
  and `ms-spec-autopilot` name verbs the released binary lacks. The lane kit drives beads by its own protocol and does
  not invoke either skill for this spec.
- **Gate.** The final review runs, from the spec worktree, `make test`; `git fetch origin main` then
  `MINDSPEC_SURFACE_BASE=$(git merge-base HEAD origin/main) go test -tags surfacescan ./cmd/mindspec/ -run
  '^TestS134SurfaceBranch_' -v` (after main is merged into the spec branch, the merge base is that main commit, so
  main's lines are not scanned); and the two AC15 `git grep` commands. It checks that the spec PR's CI `test` job is
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
   `OpenDecision`, `Answer`, `Register`, `Repaired`) and `TestS134Boundary_` (Package boundary). Add
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
- [ ] Every dispatcher test runs `Pass` on its own `NewRegistry` with test names, never on `DefaultRegistry`. `Pass` on
      an empty registry writes nothing (same snapshot) and reports `halted` on a seeded `halt` answer; a test rule's
      create and close are written receipt first; a rule error and a bd error each come back from `Pass`.
- [ ] Registry: `Register` of a name outside the registry's list, or of a name twice, returns an error; `RegisterRule`
      of a name `contract.go` does not list, or with a mode other than its listed mode, returns an error. A golden test
      pins the `contract.go` names, order and modes of Contracts first. `WithRules` swaps `Rules()` for one test and
      restores `DefaultRegistry` after it.
- [ ] Order: two test rules registered in the reverse of their fixed order run in the fixed order; a dispatcher that
      runs rules in registration order fails the test.
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
   `plan_approve` registrations, abandoned included, from the ledger. Redispatch is free. `internal/panel` stays a leaf.
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
1. Add `reducer.go`: the core rules, registered with the Bead 1 `Pass` dispatcher, which turn `State` into a write list
   and the contracts §4 output. Core rules: implement authorisation (§13.3), round opening on a closed implement step or
   a closed fix and the `panel create` entry in `run` (§13.7; the one writer of these two authorisations; a close with
   no valid receipt authorises nothing), slots after `panel create`, the inline tally, a passing gate close (frozen),
   the `complete` entry, re-creation of a deleted kernel bead (its rounds stay in the ledger) and `missing-link`: with
   `--apply` it writes each missing mindspec link that passes the B7 check (§8). A work item is done only when its gate
   closed `passed` and its branch is merged; any other state releases no dependent (B4). The slot rule creates slots
   only for a registered round that matches the gate's authorised round. A subject move before `panel create` updates
   the authorised subject only (§13.7).
2. Add `allowance.go`: `Allowance(state, gate)` is the pinned limit plus each `continue:<n>` answer; rounds count
   through `panel.RoundsForGate`. A round past the allowance gets no `run` entry.
3. Add `flags.go` (rules in mode `flag`) for `run-marker-missing`, `foreign-child-open`, `foreign-blocker` (B7 check on
   each missing link, naming the first foreign link and its bd removal) and `id-collision`.
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
- [ ] A deleted `<1>.impl.a1` is created again by the next `--apply`.
- [ ] Core rules, positive: with a seeded receipted close of `<1>.impl.a1`, `run` lists `panel create 1`; after
      `kerneltest.Register` of round 1 with 3 slots, `--apply` creates exactly `<1>.review.r1.R1`..`R3`; with 4 rounds
      counted and a closed `<1>.review.r5.fix` seeded, `run` lists `panel create 1`; after `kerneltest.PassGate`,
      `run` lists `complete 1`.
- [ ] Core rules, no receipt: a raw `bd close <1>.impl.a1` (no receipt) gives no `panel create 1` in `run`, and
      `--apply` writes no gate round record; the same for a raw-closed `<1>.review.r5.fix` with 4 rounds counted. A
      round opening that accepts any close fails the test.
- [ ] Exits: on a seeded all-done fixture (every gate `passed`, every branch merged, the epic closed) `next-action x`
      prints `"status":"done"` and exits 5. In `output.go`, a result with no action, waiting entry, decision or `run`
      entry on a spec that is not done gives `indeterminate` and exit 1. A mapping that exits 0 for either fails.
- [ ] Missing link: after `bd dep remove` of the item-3-to-`<epic>.final` link, `--apply` writes it again; with a
      foreign link that closes a cycle on it, `--apply` writes no link and flags `foreign-blocker` naming that link.
- [ ] Done = passed and merged: after `kerneltest.PassGate` with item 1 open and `bead/1` not merged, `--apply` creates
      no `<2>.impl.a1` and lists no action for items 2 or 3. A rule that counts a passed gate as done fails this test.
- [ ] Authorised slots: `kerneltest.Register` of round 2 while the gate's authorised round is 1: `--apply` creates no
      `<1>.review.r2.R<s>`.
- [ ] Subject move: after a receipted close of `<1>.impl.a1`, a commit on `bead/1` before `panel create`: `--apply`
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
      lists `panel create 2`; in an open fix, none. A foreign child under `<2>.review`, a second REJECT, answer `halt`:
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

**Steps**
1. Add `closes.go`: the `flag` rule `unreceipted-close` flags a kernel close with no or a bad receipt; the `ordinary`
   rule `retry-attempt` creates `<id>.a<k+1>` with `mindspec_from` on the next pass, unless a valid `repaired`
   receipt exists.
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
- [ ] AC10 (part): after item 1's gate passes (`kerneltest.PassGate`), `bd close <1>` flags `raw-close`, `impl approve`
      exits non-zero naming it, and `--apply` creates no `<2>.impl.a1`. In a fresh fixture `bd close --force <1>` flags
      `forced-close` and `--apply` creates no `<2>.impl.a1`.
- [ ] A table test: `impl approve` on an enrolled spec with each of an open step, an open decision, an unrepaired
      unreceipted close, `raw-close`, `forced-close` and an open foreign child exits non-zero naming it, same snapshot;
      a refusal set that misses one row fails the test.
- [ ] AC11 (audit): records of each listed event, written through the record types, give `impl approve` and a rerun
      exit 0, each printing count 1; the re-panel row shows 3 slots.
- [ ] AC13 (part): `next-action` flags `raw-claim`. A foreign note under the epic at the end: `impl approve` exits
      non-zero naming it and writes nothing.

**Depends on**
Bead 2, Bead 6

## Bead 10: Panel verbs as kernel steps

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
   limit decision `superseded`; `panel defer` writes the defer record.
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
      gate `passed` and the fix or decision `superseded`; a passing `panel defer` does the same.
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
      `panel.json`: `complete` with no passing panel exits non-zero. On `PreKernelSpec` `y`, `complete` with no panel warns and exits 0; `next` does not select the foreign
      child.
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

## Provenance

Each `-run` pattern runs in the package its bead's Verification names.

| Acceptance Criterion | Bead(s) | Verified By |
|---------------------|---------|-------------|
| AC1 | 6 | `-run '^TestS134NextAction_'` |
| AC2 | 5, 6, 12 | `-run '^TestS134Enrol_'` (enrolment, a, b, d, e, fixture); `-run '^TestS134NextAction_'` (apply); `-run '^TestS134(EnrolFault|Complete)_'` (c, complete pass) |
| AC3 | 7, 10, 12 | `-run '^TestS134Lease'`; `-run '^TestS134PanelSubmit_'`; `-run '^TestS134Next_'` |
| AC4 | 7 | `-run '^TestS134(Submit|SubmitCmd)_'` |
| AC5 | 5, 10 | `-run '^TestS134Enrol_'`; `-run '^TestS134Panel(Create|Refute|Defer)_'` |
| AC6 | 1, 5, 6, 12 | `-run '^TestS134(Witness|VerbStub)_'`; `-run '^TestS134(Enrol|Derive)_'`; `-run '^TestS134NextAction_'`; `-run '^TestS134(Complete|Next)_'` |
| AC7 | 2, 6, 8, 10 | `-run '^TestS134Rounds_'`; `-run '^TestS134Allowance_'`; `-run '^TestS134Reject_'`; `-run '^TestS134PanelCreate_'` |
| AC8 | 8, 10 | `-run '^TestS134Reject_'`; `-run '^TestS134PanelCreate_'` |
| AC9 | 7, 8, 10, 12 | `-run '^TestS134(Lease|Submit)_'`; `-run '^TestS134(Halt|Supersede)_'`; `-run '^TestS134PanelCreate_'`; `-run '^TestS134(Next|Complete)_'` |
| AC10 | 7, 9, 12 | `-run '^TestS134Submit_'`; `-run '^TestS134(Closes|ImplApprove)_'`; `-run '^TestS134Complete_'` |
| AC11 | 2, 9, 12, 13 | `-run '^TestS134Friction_'`; `-run '^TestS134Audit_'`; `-run '^TestS134Complete_'`; `-run '^TestS134E2E_'` |
| AC12 | 2, 5, 13 | `-run '^TestS134Config'`; `-run '^TestS134Enrol_'`; `-run '^TestS134E2E_'` |
| AC13 | 1, 5, 6, 7, 9, 10, 11, 12, 13 | `-run '^TestS134Store_'`; `-run '^TestS134Enrol_'`; `-run '^TestS134(Flags|NextAction)_'`; `-run '^TestS134Lease'`; `-run '^TestS134(Closes|ImplApprove)_'`; `-run '^TestS134Panel'`; `-run '^TestS134JudgeCmd_'`; `-run '^TestS134(Next|Complete)_'`; `-run '^TestS134E2E_'` |
| AC14 | 1, 6, 7, 10 | `-run '^TestS134Store_'`; `-run '^TestS134(Flags|NextAction)_'`; `-run '^TestS134Submit_'`; `-run '^TestS134PanelCreate_'` |
| AC15 | 1, 2, 4, 5, 13 | `-run '^TestS134Setup_'`; domains `git grep` (Beads 2, 13); the Bead 4 `git grep`, `cmp` and `-run '^TestS134Skills_'`; `-run '^TestS134Enrol_'` |
| AC16 | all; 3 | each bead's message tests; `-run '^TestS134Surface(Matcher|DiffFn)_'` (red mutations); `-tags surfacescan -run '^TestS134SurfaceBranch_'` |
| AC17 | 13 | `-run '^TestS134E2E_'` |
| AC18 | 10, 11, 12, 13 | `-run '^TestS134Panel(Create|Repanel)_'`; `-run '^TestS134(Judge|JudgeCmd)_'`; `-run '^TestS134(Complete|ImplFinalGate)_'`; `-run '^TestS134E2E_'` |
| Validation proofs | final review | the Testing Strategy "Gate" commands on the spec branch |
