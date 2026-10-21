---
status: Draft
spec_id: 131-panel-execution-in-binary
version: "1"
adr_citations:
  - ADR-0045
  - ADR-0044
  - ADR-0037
  - ADR-0043
  - ADR-0040
  - ADR-0036
  - ADR-0030
  - ADR-0035
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/config/config.go
      - internal/config/runners.go
      - internal/config/lenses.go
      - internal/config/local.go
      - internal/config/review_tools.yaml
      - cmd/mindspec/config.go
      - internal/bootstrap/bootstrap.go
      - .mindspec/config.yaml
      - .mindspec/adr/ADR-0040-orchestration-layering-ratchet.md
  - id: 2
    depends_on: []
    key_file_paths:
      - internal/rules/rules.yaml
      - internal/rules/rules.go
      - cmd/mindspec/rules.go
      - internal/instruct/instruct.go
      - plugins/mindspec/skills/ms-bead-impl/SKILL.md
      - .claude/skills/ms-bead-impl/SKILL.md
      - internal/setup/readiness_skill_pin_test.go
      - .mindspec/adr/ADR-0044-panel-review-conduct.md
  - id: 3
    depends_on: []
    key_file_paths:
      - internal/panel/verdict_v2.go
      - internal/panel/submit.go
      - internal/panel/ledger.go
      - internal/panel/slotlayout.go
      - internal/panel/slot_lock_unix.go
      - internal/panel/slot_lock_windows.go
      - cmd/mindspec/panel_schema.go
      - cmd/mindspec/panel_disposition_store.go
      - .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md
  - id: 4
    depends_on: [1, 2, 3]
    key_file_paths:
      - internal/panel/gate.go
      - internal/panel/slotstatus.go
      - internal/panel/create.go
      - internal/workspace/slotroot.go
      - cmd/mindspec/panel.go
      - cmd/mindspec/panel_submit.go
      - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
  - id: 5
    depends_on: [1, 3]
    key_file_paths:
      - cmd/mindspec/panel_checkout.go
      - internal/executor/executor.go
      - internal/executor/mindspec_executor.go
      - cmd/mindspec/panel_abandon.go
      - cmd/mindspec/migrate.go
      - internal/doctor/ownership.go
      - internal/doctor/links.go
  - id: 6
    depends_on: [1, 2, 3]
    key_file_paths:
      - internal/panel/brief.go
      - internal/panel/prompt.go
      - cmd/mindspec/panel_prompt.go
      - .mindspec/adr/ADR-0045-binary-owns-sequence-and-rules.md
  - id: 7
    depends_on: [1, 3, 4, 5, 6]
    key_file_paths:
      - internal/panellaunch/launch.go
      - internal/panellaunch/supervisor_unix.go
      - internal/panellaunch/launch_windows.go
      - cmd/mindspec/panel_launch.go
      - cmd/mindspec/panel_recover.go
      - plugins/mindspec/workflows/ms-panel.js
      - .claude/workflows/ms-panel.js
      - plugins/mindspec/workflow_test.go
      - plugins/mindspec/scripts/codex_verdict_extract.sh
  - id: 8
    depends_on: [1, 2, 3]
    key_file_paths:
      - internal/doctor/panel_runners.go
      - internal/doctor/staged_markers.go
      - internal/doctor/stale_checkouts.go
      - cmd/mindspec/doctor.go
  - id: 9
    depends_on: [2, 4, 5, 6]
    key_file_paths:
      - plugins/mindspec/skills/ms-panel-run/SKILL.md
      - plugins/mindspec/skills/ms-panel-tally/SKILL.md
      - plugins/mindspec/skills/ms-bead-fix/SKILL.md
      - plugins/mindspec/skills/ms-bead-cycle/SKILL.md
      - plugins/mindspec/skills/ms-spec-final-review/SKILL.md
      - internal/setup/historical_skills
      - internal/setup/review_discipline_test.go
      - .mindspec/domains/workflow/interfaces.md
      - CHANGELOG.md
      - project-docs/user/guides/review-panels.md
---
# Plan: 131-panel-execution-in-binary

The binary runs the panel and takes in each verdict. This plan builds spec 131 R1–R13 in 9 beads. The longest dependency chain is 3 beads. Beads 1, 2 and 3 start at once. Then beads 4, 5, 6 and 8 start at once.

## Lane resolutions

- **Scope.** The beads build R1–R13 exactly. Each bead lists its R-items. Every spec Non-Goal stays out. No bead adds a round limit, `next-action`, `bead prompt`, an implementation or fix prompt template, `panel.gates.<g>.families`, a family minimum, a Windows process runner or the #252 edits. Owner: every bead; the reviewer of each bead checks it.
- **Operator rulings (OQ-5, OQ-6, OQ-7).** These are fixed. The plan does not reopen them.
  - OQ-5: Bead 4 makes `create` write `verdict_schema: 2` with no key or flag (`TestCreate_AlwaysV2`).
  - OQ-6: Bead 3 makes a later submission replace the earlier one and keeps every record (`TestPanelSubmit_Replace`). Bead 4 makes the gate read the latest only.
  - OQ-7: Bead 1 ships one default runner `{family: harness, kind: harness}`, deletes the two-name family literal in `resolveGateReviewers` and adds `TestNoFamilyLiterals`. A one-family panel is valid. The tool suggestion is advice only (Bead 1 for `config resolve`, Bead 8 for `doctor`).
- **Dogfood boundary.** See `## Testing Strategy`, first bullet. Owner: every bead and every panel of this lane.
- **Legacy rounds (R3.6).** Bead 4 owns the `verdict_schema` pin. Its tests are `TestGate_PreV2RoundKeepsV1Rules`, `TestGate_VerdictSchemaPinned` and `TestCreate_AlwaysV2`.
- **ADR amendments with their citing beads (R13).** ADR-0040 in Bead 1. ADR-0044 in Bead 2. ADR-0045 in Bead 2 (rule table, `instruct`, marker form) and Bead 6 (BRIEF and slot prompt templates), under one `## Amendment (Spec 131)` heading. ADR-0043 in Bead 3. ADR-0037 in Bead 4. Each bead adds its own pin test file `internal/validate/adr_spec131_<adr>_test.go` with a function named `TestADRAmendments_Spec131<ADR>`. So `go test ./internal/validate/... -run TestADRAmendments_Spec131` runs all of them. Each bead names the existing pin tests that must stay green.
- **Shared panel files.** The split is by mechanism: config (1), rules (2), verdict and ledger contract (3), gate and verify (4), slot clones (5), BRIEF and prompt (6), process supervision (7), doctor (8), skill adapter text (9). A dependency is declared only where a bead calls the code or reads the contract of another bead. Two beads that edit one file are kept apart only when the edits conflict. Beads 1 and 5 both edit `internal/executor/executor.go`; Bead 5 depends on Bead 1, so they do not run at once.
- **#252 dependency.** PR #267 (`fix/no-internal-refs-on-surface`) is an ancestor of the spec branch (by `git merge-base --is-ancestor`). Bead 9 checks that the #252 edits are present in the skill files before it rewrites them. If they are absent, Bead 9 includes them.

### Bead count

The spec has 13 requirements and about 150 named tests. It touches config, a new package, the panel leaf package, the executor, doctor, instruct, setup, skills and a JS workflow. Five beads would put more than 40 tests and three separate mechanisms in one bead. Nine beads keep each bead on one mechanism. The chain stays at 3. Wave 1 runs 3 beads at once, wave 2 runs 4 at once, and wave 3 runs 2 at once.

Dependency graph:

```
1, 2, 3       -> (none)
4             -> 1, 2, 3
5             -> 1, 3
6             -> 1, 2, 3
8             -> 1, 2, 3
7             -> 1, 3, 4, 5, 6
9             -> 2, 4, 5, 6
longest chain: 1 -> 4 -> 7 (also 3 -> 5 -> 9, 2 -> 6 -> 9): 3 beads
```

Wave 1: 1, 2, 3. Wave 2: 4, 5, 6, 8. Wave 3: 7, 9.

## ADR Fitness

- **ADR-0045 (binary owns sequence and rules).** Sound. This spec ships its step 1 (rule table) and step 3 (BRIEF and slot prompt), and starts step 5. Amended, no divergence.
- **ADR-0044 (panel review conduct).** Sound. Each Decision moves to a rule row. Decision 4 changes from `/tmp` to the slot or fixer directory. Amended, no divergence.
- **ADR-0037 (panel gate contract).** Sound. A v2 round counts the latest submission per slot. §8 still applies: submit binds bytes and does not authenticate. Amended.
- **ADR-0043 (disposition telemetry store).** Sound. Three new kinds join the one ledger. The `verdict` key holds `seq`. Amended.
- **ADR-0040 (orchestration layering ratchet).** Sound. Launchers become config data. The `runner:` key keeps its meaning. Amended.
- **ADR-0036 (ZFC).** Applied. Quota detection matches only the exit code and configured literals. No code judges reviewer output or names a tool.
- **ADR-0030 (executor boundary).** Applied. Clone, remote, `ls-files`, `check-ignore` and `cat-file` calls go through `internal/executor`. `internal/panel` stays a leaf: it may import only `internal/termsafe` and `internal/idvalidate` (`TestPanelLeafImports_StdlibPlusTermsafeOnly`). So config resolution, rule lookup and git calls happen in the cmd layer, which passes values or callbacks into `internal/panel`. Process supervision lives in a new package `internal/panellaunch`. `TestEnforcementHasNoGitLeaks` stays green.
- **ADR-0035 (agent error contract).** Applied. Every new refusal ends with one `recovery:` line. No divergence is proposed.

## Testing Strategy

- **Dogfood boundary.** This spec changes the panel verbs and the gate that this lane uses. Every panel of this lane runs on the installed v0.17.0 binary `~/.local/bin/mindspec`. No panel runs on a spec-branch build. Nobody installs a branch build. The R4.13 `panel.runners` block is safe for the released binary, because its config load ignores unknown keys.
- **No real model CLI.** Tests use temp repos (`t.TempDir()` with `git init`) and local fake runner scripts only. No test starts a real `codex`, `claude` or other model CLI. The R4.12 suggestion tests put a fake executable on a temp `PATH`.
- **Layers.**
  - Unit: `internal/config`, `internal/rules`, `internal/panel` (pure; fake resolvers and callbacks), `internal/workspace`.
  - Integration: `cmd/mindspec` tests drive the verbs in temp repos. Ledger fixtures seed `launch`, `verdict` and `substitution` records where a test needs a state that a later bead's verb makes.
  - Process: `internal/panellaunch` tests run fake runners under the supervisor. Unix-only tests use `_unix_test.go` or a `!windows` build tag. Windows tests use a `windows` build tag. `GOOS=windows go build ./...` runs in Bead 3 and Bead 7.
  - Workflow: Bead 7 runs `ms-panel.js` under `node` from Go with a stub harness API. The test fails, and does not skip, when `node` is absent.
- **Shared helpers.** Bead 3 adds `internal/panel/paneltest` helpers: a v2 panel fixture, a ledger seeder and a verdict builder. Later beads use them. Bead 4 adds a cmd-layer slot-clone helper for AC-9.
- **Shipped-surface rule.** Every bead that adds or changes surface text (skills, workflows, setup-written files, templates, rule table, data files, CLI help, errors and output, user manuals) cites no ADR-NNNN, spec number, R-id or §, bead ID or issue number. `TestSurfaceRefs_NoInternalReferences` (exists) and `TestProductSurface_NoInternalRefs` (Bead 2) enforce it. Each later bead keeps both green.
- **Gates for every bead.** `go build ./...`, `go test ./... -short`, `go vet ./...` and `gofmt -l .` (empty output). CI runs `-short` and the race tests.

## Bead 1: Panel config: runners, lenses, local file, isolation root

Scope: R4.1–R4.7 (config side), R4.9–R4.13, R5, R6.1, and the R4.10 and R6.2 `.gitignore` lines from `mindspec init`.

**Steps**
1. Add `panel.runners` to `internal/config` per spec "R4 — Runners and slot resolution" R4.1–R4.4: types, argv placeholder check, merge by family over the default `{family: harness, kind: harness}`, default `timeout_minutes` 60 for a process runner. Make zero-config fill go round-robin over the merged runner order. Delete the two-name literal in `resolveGateReviewers`.
2. Add family and model resolution to slot expansion: each slot gets `Family`; model order per R4.6; a `SubstituteModel(toFamily)` helper; model-safe refusal from every source. A model-only entry gets family `harness` (R4.7). A family with no runner refuses with the recovery `add a runner for the family to panel.runners`. `create` and `config resolve` both get this refusal through slot expansion.
3. Add `.mindspec/config.local.yaml` (R4.10): only `panel.runners` and `models`, a 65536-byte limit, strict keys, merge after `config.yaml`. Retire `panel.substitution` (R4.9) with the `on_quota` recovery. Delete `knownModels`, `KnownModels()` and the `config show` known-model warning (R4.5).
4. Add R5: `panel.lenses`, `panel.gates.<g>.lenses|acceptance`, the R5.4 and R5.5 default tables as data, lens-name validation, and a lens-only gate entry. Delete `defaultLenses`. Add `panel.isolation_root` with the R6.1 lexical refusals in load, and the tracked-value refusal through a new executor `ls-files` call at the cmd-layer config load.
5. Add `mindspec config resolve --gate G [--json]` with per-value source and the R4.7 model-only Warn. Add `internal/config/review_tools.yaml` and the R4.12 suggestion line. Add `TestNoFamilyLiterals` per R4.5. Make `mindspec init` add `config.local.yaml` and the effective isolation root to `.gitignore`.
6. Add the R4.13 `panel.runners` block to this repository's `.mindspec/config.yaml`. Append `## Amendment (Spec 131)` to ADR-0040 per R13.5, and add `TestADRAmendments_Spec131ADR0040`.

**Verification**
- [ ] `go test ./internal/config/... -run 'TestModelValue_Refusals|TestRunners_|TestSubstitution_Retired|TestNoFamilyLiterals|TestSlotModelResolution|TestReviewer_ModelOnlyIsHarness|TestSubstituteModel|TestLocalConfig_|TestLenses_|TestIsolationRoot_Validate|TestRepoConfig_Resolves'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestCreate_RefusesUnrunnableFamily|TestReviewer_ModelOnlyWarn|TestConfigResolve_|TestLocalConfig_Refusals'` passes (create and `config resolve` legs)
- [ ] `go test ./internal/bootstrap/... -run TestBootstrap_ScratchIgnoredExisting` passes
- [ ] `go test ./internal/validate/... -run 'TestADRAmendments_Spec131ADR0040'` passes, and `go test ./cmd/mindspec/... -run TestADR0040` and `go test ./internal/next/... -run TestGateReadiness` stay green
- [ ] `go test ./internal/setup/... -run 'TestRunClaude_|TestSurfaceRefs_'` stays green
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-5 in full: every listed load refusal; the 60-minute default; zero-config `bead` gives 4 `harness` slots; the merge-by-family order; `TestNoFamilyLiterals` passes on `G1-codex` and fails on a planted `"codex"`; `KnownModels` does not exist
- [ ] AC-6: the slot-model, model-only, unrunnable-family (create and `config resolve` legs), substitute-model, local-merge, local-refusal, provenance, tool-suggestion and repo-config items
- [ ] AC-7: the lens default, lens-only entry and unknown-lens items; no default focus text names a tool, language or library
- [ ] AC-8: load refuses an empty, absolute, `.`, `.git/x`, `.worktrees/x` and tracked root; `mindspec init` on an existing install adds the default ignore line
- [ ] AC-16: ADR-0040 carries the R13.5 amendment

**Depends on**
None

## Bead 2: Rule table and `mindspec rules`

Scope: R10.1–R10.7 except the `ms-bead-fix` prompt edit (Bead 9).

**Steps**
1. Add package `internal/rules` with one embedded YAML table per spec "R10 — Rule table" R10.1, holding at least the R10.2 rows. The `gates` field limits `review.ci-parity` to `final_review`. Add lookup by id, scope and gate.
2. Add `mindspec rules [--scope S] [--json]` and `mindspec rules show <id>`. An unknown id or scope refuses with a `recovery:` line. Leave a root-resolver hook so that Bead 4 can add slot-directory root discovery.
3. Make `instruct` render a `## Rules` section: orchestrator-scope rules in every mode, plus implementer-scope rules in implement mode (R10.6).
4. Change the `ms-bead-impl` staged-rule marker in both copies to `<!-- staged-rule: id=sequence.bead-impl-prompt; remove-by=YYYY-MM-DD -->` (R10.5). Add a marker parser. Update `readinessStagedRule` in `internal/setup/readiness_skill_pin_test.go` and the matching rows in `internal/setup/review_discipline_test.go`.
5. Add `TestProductSurface_NoInternalRefs` per R10.7: it scans rule fields, shipped data files and non-test string literals that this spec adds for `ADR-[0-9]`, `[Ss]pec [0-9]{3}`, `\bR[0-9]+\.[0-9]`, `#[0-9]{3}` and the bead-id pattern.
6. Append `## Amendment (Spec 131)` to ADR-0044 per R13.2. Start the ADR-0045 amendment with the step-1, step-5 and marker-form items (R13.1). Add `TestADRAmendments_Spec131ADR0044` and `TestADRAmendments_Spec131ADR0045Rules`.

**Verification**
- [ ] `go test ./internal/rules/... -run 'TestRuleTable_MinimumSet|TestStagedMarker_Parse'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestRulesCmd'` passes
- [ ] `go test ./internal/instruct/... -run TestInstruct_RulesSection` passes
- [ ] `go test ./internal/setup/... -run 'TestProductSurface_NoInternalRefs|TestMsBeadImplSkill_|TestReviewDiscipline_|TestSurfaceRefs_|TestLifecycleGateStep_'` passes
- [ ] `go test ./internal/validate/... -run 'TestADRAmendments_Spec131ADR004[45]|TestADRAmendments_Spec130_'` passes (the spec-130 pins, incl. the ADR-0045 marker text in `TestADRAmendments_Spec130_Verbs`, stay green because the amendment is append-only)
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-13: the table holds every R10.2 id; `rules --scope reviewer` lists exactly the reviewer rules; `rules show nope` refuses; `instruct` lists `lifecycle.no-probe-complete` in spec and implement mode and `review.derive` only in implement mode; the `ms-bead-impl` marker parses with `id=sequence.bead-impl-prompt`; the internal-reference scan is clean
- [ ] AC-16: ADR-0044 carries the R13.2 amendment; ADR-0045 carries the R13.1 rule-table items

**Depends on**
None

## Bead 3: Verdict v2, ledger kinds, submit core, slot layout and lock

Scope: R1, R2.4–R2.6 and R2.8 (core), the `verdict`, `launch` and `substitution` record kinds, the slot-directory layout, the slot lock (Terms, R7.3 lock tests, R7.9 lock-file rules).

**Steps**
1. Add the v2 verdict per spec "R1 — Verdict schema v2": one Go definition that gives the schema layer and the JSON Schema, plus the transport and relation layer. Each error names its JSON pointer. Add the R1.6 projection to `ccr[i]` and the slot hash. Add `mindspec panel schema`.
2. Add the three record kinds to the ledger (`IsVerbRecord`, `validateLedgerMap`, keys per the R2.5 and R7.6 tables). Add `Family` to `RosterSlot`. Make `panel disposition append` refuse the three kinds; `validate|check|query` accept them.
3. Add `panel.Submit` in the leaf package. It takes the target, the bytes and a resolver callback that returns the effective model, `runner_kind` and `runner_source`. Under the ledger lock it runs the pure R2.3 checks, then the R2.5 same-bytes repair or the append with `seq` and the temp-file rename, then the `submitted` outcome for an open `dispatched` attempt. Add a crash-injection hook for tests.
4. Add slot-layout path helpers (`<isolation_root>/<spec or adhoc>/<slug>/r<N>/<slot>/`, `slot.json`, `tree/`, `attempt-<k>/`, `.locks/…/<slot>.lock`) and the attempt-number rule of R7.2.
5. Add the slot lock: `flock` on Unix with no file deletion; on Windows, the R7.9 exclusive-create file with pid, stale-file rename and relink. Add the non-waiting lock test and the 1-second bounded retry (R7.3). Follow `disposition_lock_unix.go` and `disposition_lock_windows.go`.
6. Append `## Amendment (Spec 131)` to ADR-0043 per R13.4. Add `TestADRAmendments_Spec131ADR0043`.

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestVerdictV2_|TestPanelSubmit_Writes|TestPanelSubmit_Replace|TestPanelSubmit_CrashRepair|TestPanelSubmit_Race|TestSlotLock_'` passes
- [ ] `go test ./internal/panel/... -run 'TestPanelLeafImports_StdlibPlusTermsafeOnly|TestLedger|TestScan_'` stays green
- [ ] `go test ./cmd/mindspec/... -run 'TestDispositionAppend_RefusesV2Kinds|TestPanelSchema'` passes
- [ ] `go test ./internal/validate/... -run 'TestADRAmendments_Spec131ADR0043|TestADRAmendments_Spec130_'` passes
- [ ] `GOOS=windows go build ./...` passes
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-1 in full, including the 1 MiB padding case, the 4096-byte `tail` case and schema-versus-validator agreement
- [ ] AC-2 (code only): the R1.6 projection gives `ccr[i]` and the slot hash; Bead 4 owns the test, because the refute leg needs the v2 gate
- [ ] AC-3: submit writes the file and one `verdict` record with `seq: 1`; a same-bytes rerun adds no record; final APPROVE then final REQUEST_CHANGES both stay in the ledger; a crash after the append repairs on rerun; two concurrent submits leave the file that matches the latest record; `panel disposition append` refuses each new kind
- [ ] AC-16: ADR-0043 carries the R13.4 amendment

**Depends on**
None

## Bead 4: Gate, verify, submit CLI and slot root discovery

Scope: R2.1–R2.3, R2.7, R2.9 (CLI), R3, R4.6–R4.8 (create side), R6.9, R8.

**Steps**
1. Make `create` write `verdict_schema: 2` to `panel.json` and every new `round` event, and add it to the pinned-field list (R3.1). Write `family` on each roster entry. Add the first-v2-round family carry and its refusal (R4.8).
2. Change the gate per spec "R3 — The gate on a v2 round": v2-ness from the latest `round` event only; a slot is in only by R3.3; R3.5 discharge binding on the latest submission; R3.6 rounds with no `verdict_schema` keep spec 130's rules.
3. Add `mindspec panel submit` (long and short form) in `cmd/mindspec/panel_submit.go`. It resolves config, the effective runner and the effective model, and passes them to `panel.Submit`. Add the landed-binding refusal, the process-slot refusal, R2.9 attempt binding and the status print (R2.7). Add no `--via` flag.
4. Add R8 slot status to `panel verify`: the closed status set, the R8.2 rule order, R8.3 annotations, `--json` with `handoff_pending`, the R8.6 exit codes, and `termsafe` for every reviewer string. `tally` prints one line per `replaced` slot. Verify writes nothing.
5. Add R6.9 root discovery in `internal/workspace`: find the nearest ancestor with `slot.json` and `tree/`, check the exact path and the `slot.json` fields, and return the main root. Wire it into every `mindspec panel` verb and into `mindspec rules` through the Bead 2 hook.
6. Append `## Amendment (Spec 131)` to ADR-0037 per R13.3. Add `TestADRAmendments_Spec131ADR0037`.

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestVerdictV2_FindingProjection|TestGateV2_|TestGate_VerdictSchemaPinned|TestGate_PreV2RoundKeepsV1Rules|TestCreate_AlwaysV2|TestRoster_Family|TestCreate_FirstV2RoundFamily'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelSubmit_Refusals|TestPanelSubmit_ABA|TestPanelSubmit_NoViaFlag|TestPanelSubmit_LandedBindingRefused|TestPanelVerify_AnnotationsFromRecords|TestPanelVerify_SlotStatus|TestPanelVerify_DispatchedUnsubmitted|TestPanelVerify_HarnessTimeout|TestPanelVerify_Replaced|TestPanelVerify_ExitCodes|TestPanelVerify_V1Round|TestPanelVerify_JSONRoster|TestPanelVerify_Termsafe'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestSlotRoot_'` passes (a real `git clone --shared` slot built by the test helper)
- [ ] `go test ./internal/validate/... -run 'TestADRAmendments_Spec131ADR0037|TestADRAmendments_Spec130_'` passes
- [ ] `go test ./internal/complete/... -run 'TestReadinessIsolation_|TestPanelGate_AdHocPanelNeverGates'` and `go test ./internal/panel/... -run 'TestCreate_|TestScan_'` stay green
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-3: each R2.3 refusal leaves the panel directory and ledger byte-identical; the v1-round refusal names `--round`; A, B, A gives `seq` 1, 2, 3 and the gate counts A; a CLI submit on a process slot refuses; `--via` is an unknown flag; a wrong `model` refuses; the landed-binding refusal names `--round`; the local-file annotation items
- [ ] AC-2: `TestVerdictV2_FindingProjection` — findings `ccr[0]` and `ccr[1]` carry the sha256 of `a` and `b`; `panel refute --finding ccr[1]` discharges `ccr[1]` only
- [ ] AC-4 in full; the R3.6 pin is owned here (`TestGate_PreV2RoundKeepsV1Rules`, `TestGate_VerdictSchemaPinned`, `TestCreate_AlwaysV2`)
- [ ] AC-6: `create` writes `family`; the first-v2-round carry and its `--reviewers` refusal; earlier round records stay byte-identical
- [ ] AC-9 in full
- [ ] AC-11: every item that a seeded ledger can reach (slot status per R8.1, `dispatched` with a hand-written file, harness timeout, `replaced` annotations and the tally line, exit codes, v1 round, `--json`, `termsafe`, verify writes nothing)
- [ ] AC-16: ADR-0037 carries the R13.3 amendment

**Depends on**
Bead 1 (slot family, effective runner and model), Bead 2 (the `mindspec rules` root hook), Bead 3 (verdict, ledger kinds, `panel.Submit`, slot layout and lock test)

## Bead 5: Slot checkout, dispatch and cleanup

Scope: R6.2–R6.8.

**Steps**
1. Add executor calls for slot clones: `clone --shared --no-checkout`, detached checkout, `remote remove origin`, `check-ignore` and `cat-file -e`. Each runs with `-c core.fsmonitor=false -c core.hooksPath=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, and the `git rev-parse --local-env-vars` names removed (R6.4).
2. Add `mindspec panel checkout <slug> (--slot S | --all)` per spec "R6 — Slot clones and scratch" R6.2–R6.4: the symlink and ignore refusals, `slot.json` with the main root, reuse at the reviewed SHA, the `lstat`, `commondir`, `core.worktree`, `core.bare` and `alternates` refusals with the `--prune --slot` recovery. Plain checkout writes no ledger record.
3. Add `--dispatch [--replace]` (R6.5): exclusive `mkdir` of `attempt-<k>/`, `launch_failed` for earlier open `dispatched` attempts, then one `dispatched` event. Refuse a process slot, a final non-`VOID` slot without `--replace`, and a landed binding before any append. Add `--fixer` (R6.6).
4. Add `--prune [--slot S]`, and the cleanup in `create --round N+1` and `abandon` (R6.7): slot locks in sorted order before the ledger lock, the orphan check hook (a no-op until Bead 7 fills it), no symlink follow, and no slot-lock file deletion on Unix.
5. Make the registration lookup (`panel.Scan`), the `doctor` ownership and links walks and the `migrate` walks skip the resolved isolation root (R6.8).

**Verification**
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelCheckout_|TestPanelDispatch_StaleFileRefused'` passes
- [ ] `go test ./internal/bootstrap/... ./cmd/mindspec/... -run 'TestBootstrap_CustomRootIgnored'` passes
- [ ] `go test ./internal/panel/... ./internal/doctor/... ./cmd/mindspec/... -run 'TestWalks_SkipIsolationRoot'` passes
- [ ] `go test ./internal/lint/... -run TestEnforcementHasNoGitLeaks` and `go test ./internal/executor/...` stay green
- [ ] `go test ./internal/doctor/... -run 'TestOwnership|TestLinks'` and `go test ./cmd/mindspec/... -run 'TestMigrate|TestPanelAbandon|TestPanelCreate'` stay green
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-8: `checkout --all` on 4 slots, idempotent rerun, no `origin`; `git status` and `git stash` in a slot tree leave the main git directory unchanged; the other-SHA, git-redirect, `lstat`, fsmonitor and `GIT_DIR` items; `--prune --slot`; `create --round 2` and `abandon` cleanup; the `--dispatch` event and refusals; plain checkout after a final writes nothing; the symlinked and non-ignored root refusals; the custom-root `init` item; the walk items
- [ ] AC-8 tests owned here: `TestPanelCheckout_Creates`, `_GitStaysInSlot`, `_NoOrigin`, `_Refusals`, `_Prune`, `_HarnessDispatched`, `_DispatchFlag`, `_PlainWritesNoLedger`, `_GitRedirectRefused`, `_TreeLstat`, `_NoFsmonitor`, `_GitEnvScrubbed`, `TestPanelDispatch_StaleFileRefused`, `TestBootstrap_CustomRootIgnored`, `TestWalks_SkipIsolationRoot`

**Depends on**
Bead 1 (`panel.isolation_root`, runner kind), Bead 3 (slot layout, slot lock, `launch` record kind)

## Bead 6: BRIEF regions and slot prompt

Scope: R9.1–R9.4. R9.5 is adapter behaviour; Bead 9 writes it into the skills.

**Steps**
1. Add the four BRIEF regions per spec "R9 — BRIEF and slot prompt" R9.1: acceptance (from Bead 1 config), quoted spec ACs at the reviewed SHA (fenced, `termsafe`), conduct (reviewer-scope rules for the gate, from Bead 2, passed in by the cmd layer), and the verdict contract.
2. Add the R9.3 check: binary-authored text names no documentation path and no internal reference; every repository path it names exists at the reviewed SHA (`cat-file -e` through the executor), else `create` refuses. Paths in the quotation are not checked.
3. Add `mindspec panel prompt <slug> --slot S` (R9.4). It prints the BRIEF, slot id, effective family, model, lens and focus, the absolute slot directory and tree, the output instruction for the effective runner, and the scratch instruction. It writes nothing. A harness prompt with no `dispatched` event refuses with the `panel checkout --slot S --dispatch` recovery.
4. Add the process output instruction as a render function that takes the nonce. Bead 7 calls it with a new nonce for `attempt-<k>/prompt.md`.
5. Add round-2 prior items: the slot's latest-submission items on a v2 prior round; the parsed `<slot>-round-<N-1>.json` items on a v1 prior round; else `no prior-round items`.
6. Extend the ADR-0045 `## Amendment (Spec 131)` section with the step-3 template items (R13.1). Add `TestADRAmendments_Spec131ADR0045Templates`.

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestBrief_QuotedACs|TestBrief_Regions|TestBrief_PathCheck|TestBrief_AcceptanceRegion'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelPrompt_Render|TestPanelPrompt_PriorRound|TestPanelPrompt_PriorRoundV1'` passes
- [ ] `go test ./internal/panel/... -run 'TestCreate_|TestPanelLeafImports_StdlibPlusTermsafeOnly'` stays green
- [ ] `go test ./internal/validate/... -run 'TestADRAmendments_Spec131ADR0045|TestADRAmendments_Spec130_Verbs'` passes
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-12 in full
- [ ] AC-7: the BRIEF of each gate holds its R5.5 question (`TestBrief_AcceptanceRegion`)
- [ ] AC-16: ADR-0045 carries the R13.1 template items

**Depends on**
Bead 1 (lenses and acceptance data), Bead 2 (rule table), Bead 3 (verdict contract, ledger reads, slot layout)

## Bead 7: Launch, supervisor, recover and the panel workflow

Scope: R7, R12.5, R12.6, and the AC-8, AC-10 and AC-11 items that need a running launcher.

**Steps**
1. Add package `internal/panellaunch` with the supervisor internal mode in `supervisor_unix.go` per spec "R7 — `panel launch` and `panel recover`" R7.3: new process group, close-on-exec `run.lock`, `pgid` by rename, start pipe, no shell, scrubbed `MINDSPEC_` and git variables, child stdin per `prompt`, timeout, `exit.json`, SIGKILL to its group on every exit path.
2. Add `mindspec panel launch <slug> <slot> [--replace]` (R7.1, R7.2): refusals, slot lock then ledger lock, checkout through Bead 5, the orphan check (fill the Bead 5 hook), attempt `k`, prompt with nonce through Bead 6, the `started` event, then `start`.
3. Add outcomes, retry and substitution (R7.4–R7.7): nonce block or `verdict.json`, `quota_wall` from the stderr tail and configured literals only, `launch_failed`, `malformed` with one retry, `on_quota` substitution with the Bead 1 substitute model, the harness handoff print, exit 0 only for a submitted final.
4. Add `mindspec panel recover <slug> <slot> --reason R` (R7.8) with the digest and nonce checks. Add the Windows refusal `process runners need a Unix host` in `launch_windows.go` (R7.9).
5. Rewrite both `ms-panel.js` copies per R12.5: exact `ALLOWED_CLI`, fixed-flag `buildCommand`, roster from `panel verify --json`, the R12.1 table, a harness reviewer step that submits through `panel submit`. Remove `VALID_MIX_FAMILIES`, the substitution flag and the parse-retry ladder. Delete `plugins/mindspec/scripts/codex_verdict_extract.sh` (R12.6).
6. Add node-run workflow fixtures in `plugins/mindspec/workflow_test.go` with a stub harness API: one harness slot with a fake agent ends `final` with no Write on a verdict path; the quota handoff runs end to end.

**Verification**
- [ ] `go test ./internal/panellaunch/... ./cmd/mindspec/... -run 'TestPanelLaunch_|TestPanelRecover_|TestLedger_NoScratchPaths|TestPanelDispatch_IdenticalFinalReplacement'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelVerify_KilledLaunchLiveSupervisor|TestPanelVerify_KilledLaunchBothLocksFree|TestPanelVerify_ReplacementInFlight|TestSlotLock_StableAcrossPrune|TestPanelCleanup_RefusesHeldLock'` passes
- [ ] `go test ./plugins/mindspec/... -run 'TestMsPanelWorkflow_'` passes with `node` on `PATH`
- [ ] `GOOS=windows go build ./...` passes; `go test ./internal/panellaunch/... -run TestPanelLaunch_WindowsRefuses` passes on a Windows runner (build-tagged)
- [ ] `test ! -e plugins/mindspec/scripts/codex_verdict_extract.sh` and `grep -n -i -E '/tmp|codex|copilot|gemini' plugins/mindspec/workflows/ms-panel.js .claude/workflows/ms-panel.js` prints nothing
- [ ] `go test ./internal/setup/... -run 'TestClaudeSetup_InstallsWorkflowClaudeTargetOnly|TestSurfaceRefs_|TestProductSurface_NoInternalRefs'` stays green
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-10 in full, including the adapter end-to-end run, the supervisor fault cases, the lock tests, the Windows refusal, the identical-final replacement cases, the landed-round refusal and the quota handoff through the workflow fixture
- [ ] AC-8: the paused-launcher prune race (`TestSlotLock_StableAcrossPrune`) and the held-lock cleanup refusal (`TestPanelCleanup_RefusesHeldLock`)
- [ ] AC-11: the killed-launch items and the replacement-in-flight item
- [ ] AC-15: `ALLOWED_CLI` equals the R12.5 set; both `ms-panel.js` copies pass the family and `/tmp` grep; the harness workflow fixture ends `final` with no Write on a verdict path; the extract script is absent

**Depends on**
Bead 1 (runner argv, `on_quota`, substitute model), Bead 3 (record kinds, `panel.Submit`, slot lock), Bead 4 (verify status and gate for outcome tests), Bead 5 (checkout and the orphan-check hook), Bead 6 (process prompt render with nonce)

## Bead 8: Doctor checks

Scope: R11.1–R11.8, and the doctor legs of the R4.7 Error and Warn.

**Steps**
1. Add a launcher check per spec "R11 — Doctor checks" R11.1: for each process runner that a resolved roster of any gate uses, `argv[0]` is on `PATH`, else an Error naming the family.
2. Add the probe rule (R11.2): run `probe` only for a runner from an untracked `config.local.yaml` or under `doctor --probe`; 10-second limit; one Info line for a probe not run. Use the executor `ls-files` call from Bead 1 for the tracked check.
3. Add R11.3–R11.5: the tool suggestion as Info, the unrunnable-family Error, the model-only Warn, the load error as one Error with its recovery, and the tracked-local-file and non-ignored-root Warns.
4. Add R11.6 and R11.7: staged-rule markers in installed skills (no id, unknown id from Bead 2, past `remove-by`), the Info list of enforced rules, and slot directories of a missing, abandoned or later-round panel (recovery `mindspec panel checkout <slug> --prune`), found through the Bead 3 layout helpers.
5. Make `doctor --ci` skip R11.1, R11.2, the R11.3 suggestion and R11.7 (R11.8).

**Verification**
- [ ] `go test ./internal/doctor/... ./cmd/mindspec/... -run 'TestDoctor_Launchers|TestDoctor_ProbeNotRunFromTrackedConfig|TestDoctor_TrackedLocalProbe|TestDoctor_LoadErrorReported|TestDoctor_ConfigWarnings|TestDoctor_StagedMarkers|TestDoctor_StaleCheckouts|TestDoctor_CISkips'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestCreate_RefusesUnrunnableFamily|TestReviewer_ModelOnlyWarn'` passes with the doctor legs added
- [ ] `go test ./internal/doctor/...` and `go test ./cmd/mindspec/... -run 'TestDoctor'` stay green
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-14 in full
- [ ] AC-6: `doctor` gives the Error for a family with no runner and one Warn for a model-only entry

**Depends on**
Bead 1 (runners, local file, probe data, load errors), Bead 2 (rule ids for markers), Bead 3 (slot layout for the stale-checkout walk)

## Bead 9: Harness adapter skills, cross tests and documents

Scope: R10.6 (`ms-bead-fix` prompt), R12.1–R12.4, R12.7, R13.6, R13.7, and the AC-8 dispatch-then-submit items.

**Steps**
1. Rewrite both copies of `ms-panel-run` (80 lines or fewer) per spec "R12 — Skill and workflow slimming" R12.1, with one status-table row per non-final status and the R9.5 model pass-through. Rewrite both copies of `ms-panel-tally` (80 lines or fewer) per the R12.2 section table.
2. Edit both copies of `ms-bead-fix`, `ms-spec-final-review`, `ms-bead-cycle` and `ms-bead-impl` per R12.3 and R12.4: no `/tmp`, no tool family name, rule ids for conduct, `panel checkout --slot` and `--fixer`, a family-neutral `ms-bead-cycle` table. `ms-bead-fix` puts `mindspec rules --scope fixer` output in the fixer prompt (R10.6).
3. Add the replaced versions of every R12.3 skill to `internal/setup/historical_skills/` (R12.7). Update the fragment rows in `internal/setup/review_discipline_test.go` and any skill pins that the rewrite changes.
4. Add the Go-driven adapter fixture `TestPanelRun_SilentHarnessRedispatchOnce`. Add the AC-8 dispatch-then-submit tests and the Windows harness-slot lock lifecycle test (build-tagged).
5. Update `interfaces.md` § Panel Artifact Schema and `CHANGELOG.md` per R13.6, including the before-and-after example. Update `review-panels.md`, the panel paragraphs of `README.md`, and `plugins/mindspec/README.md` per R13.7.

**Verification**
- [ ] `wc -l plugins/mindspec/skills/ms-panel-run/SKILL.md .claude/skills/ms-panel-run/SKILL.md plugins/mindspec/skills/ms-panel-tally/SKILL.md .claude/skills/ms-panel-tally/SKILL.md` shows 80 or less for each
- [ ] `go test ./internal/setup/... ./plugins/mindspec/... -run 'TestPanelSkills_AdapterShape|TestPanelRun_StatusTable|TestSkills_NoFamilyNoTmp|TestBeadFix_FixerRules|TestSetup_RefreshesSpec130Skills|TestReviewDiscipline_|TestSurfaceRefs_|TestProductSurface_NoInternalRefs|TestMsBeadImplSkill_|TestLifecycleGateStep_'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelRun_SilentHarnessRedispatchOnce|TestPanelSubmit_SupersededAttemptRefused|TestPanelSubmit_SameAttemptDraftThenFinal|TestPanelSubmit_SameAttemptRerunRepairs|TestPanelSubmit_EndedAttemptRefused|TestPanelSubmit_LongFormSuperseded'` passes
- [ ] `go test ./cmd/mindspec/... -run TestSlotLock_WindowsLifecycle` passes on a Windows runner (build-tagged)
- [ ] `go test ./internal/validate/... ./cmd/mindspec/... -run 'TestADRAmendments_Spec131|TestUserDocs_NoRetiredPanelClaims'` passes
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-15: line limits; the R12.3 grep is clean for every skill copy; one status row per non-final status; exactly one redispatch for a silent harness agent; `mindspec setup claude` over an unmodified spec-130 install replaces every R12.3 skill
- [ ] AC-13: `ms-bead-fix` renders the fixer-scope rules into its prompt
- [ ] AC-8: the dispatch-then-submit items (superseded attempt, same-attempt draft then final, same-attempt rerun repair, ended attempt, long-form superseded)
- [ ] AC-10: the Windows harness-slot lock lifecycle item
- [ ] AC-16: `interfaces.md`, `CHANGELOG.md`, `review-panels.md` and both READMEs; `TestADRAmendments_Spec131` (all five ADR pins from Beads 1–4 and 6) passes

**Depends on**
Bead 2 (rule ids and `mindspec rules`), Bead 4 (submit CLI and `verify --json`), Bead 5 (`checkout --dispatch` and `--fixer`), Bead 6 (`panel prompt`)

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| AC-1 (R1) | Bead 3 verification: `TestVerdictV2_Accepts`, `_Refusals`, `_TransportLayer`, `_SchemaMatchesValidator` |
| AC-2 (R1.6) | Bead 4: `TestVerdictV2_FindingProjection` (projection code from Bead 3) |
| AC-3 (R2) | Bead 3: `TestPanelSubmit_Writes`, `_Replace`, `_CrashRepair`, `_Race`, `TestDispositionAppend_RefusesV2Kinds`; Bead 4: `TestPanelSubmit_Refusals`, `_ABA`, `_NoViaFlag`, `_LandedBindingRefused`, `TestPanelVerify_AnnotationsFromRecords` |
| AC-4 (R3) | Bead 4: `TestGateV2_*`, `TestGate_VerdictSchemaPinned`, `TestGate_PreV2RoundKeepsV1Rules`, `TestCreate_AlwaysV2` |
| AC-5 (R4.1–R4.5, R4.9) | Bead 1: `TestModelValue_Refusals`, `TestRunners_*`, `TestSubstitution_Retired`, `TestNoFamilyLiterals` |
| AC-6 (R4.6–R4.13) | Bead 1: `TestSlotModelResolution`, `TestReviewer_ModelOnlyIsHarness`, `TestSubstituteModel`, `TestLocalConfig_*`, `TestConfigResolve_*`, `TestRepoConfig_Resolves`, create and resolve legs of `TestCreate_RefusesUnrunnableFamily` and `TestReviewer_ModelOnlyWarn`; Bead 4: `TestRoster_Family`, `TestCreate_FirstV2RoundFamily`; Bead 8: doctor legs |
| AC-7 (R5) | Bead 1: `TestLenses_GateDefaults`, `_LensOnlyGateEntry`, `_UnknownRefused`; Bead 6: `TestBrief_AcceptanceRegion` |
| AC-8 (R6) | Bead 1: `TestIsolationRoot_Validate`, `TestBootstrap_ScratchIgnoredExisting`; Bead 5: `TestPanelCheckout_*`, `TestPanelDispatch_StaleFileRefused`, `TestBootstrap_CustomRootIgnored`, `TestWalks_SkipIsolationRoot`; Bead 7: `TestSlotLock_StableAcrossPrune`, `TestPanelCleanup_RefusesHeldLock`; Bead 9: `TestPanelSubmit_SupersededAttemptRefused`, `_SameAttemptDraftThenFinal`, `_SameAttemptRerunRepairs`, `_EndedAttemptRefused`, `_LongFormSuperseded` |
| AC-9 (R6.9) | Bead 4: `TestSlotRoot_SubmitFromFilePath`, `_SubmitFromClone`, `_Refusals`, `_Rules` |
| AC-10 (R7) | Bead 7: `TestPanelLaunch_*`, `TestPanelRecover_*`, `TestLedger_NoScratchPaths`, `TestPanelDispatch_IdenticalFinalReplacement`; Bead 9: `TestSlotLock_WindowsLifecycle` |
| AC-11 (R8) | Bead 4: `TestPanelVerify_SlotStatus`, `_DispatchedUnsubmitted`, `_HarnessTimeout`, `_Replaced`, `_ExitCodes`, `_V1Round`, `_JSONRoster`, `_Termsafe`; Bead 7: `_KilledLaunchLiveSupervisor`, `_KilledLaunchBothLocksFree`, `_ReplacementInFlight` |
| AC-12 (R9) | Bead 6: `TestBrief_QuotedACs`, `_Regions`, `_PathCheck`, `TestPanelPrompt_Render`, `_PriorRound`, `_PriorRoundV1` |
| AC-13 (R10) | Bead 2: `TestRuleTable_MinimumSet`, `TestRulesCmd`, `TestInstruct_RulesSection`, `TestStagedMarker_Parse`, `TestProductSurface_NoInternalRefs`; Bead 9: `TestBeadFix_FixerRules` |
| AC-14 (R11) | Bead 8: `TestDoctor_*` (eight named tests) |
| AC-15 (R12) | Bead 7: `TestMsPanelWorkflow_AllowedCLIExactSet`, `TestMsPanelWorkflow_HarnessSubmits`; Bead 9: `TestPanelSkills_AdapterShape`, `TestPanelRun_StatusTable`, `TestSkills_NoFamilyNoTmp`, `TestPanelRun_SilentHarnessRedispatchOnce`, `TestSetup_RefreshesSpec130Skills` |
| AC-16 (R13) | Beads 1, 2, 3, 4, 6: `TestADRAmendments_Spec131<ADR>` per ADR; Bead 9: `TestUserDocs_NoRetiredPanelClaims` and the full `-run TestADRAmendments_Spec131` |
| AC-17 (method) | Every bead: `go build ./... && go test ./... -short`, incl. `TestPanelLeafImports_StdlibPlusTermsafeOnly` and `TestEnforcementHasNoGitLeaks` |

provenance: default=model-protocol.md override=absent | drafting=default implementation=default orchestrator=default codex-effort=default codex-impl-effort=default spec=default plan=default bead=default final=default

Role: drafting = claude-opus-5-5.
