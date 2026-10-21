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
      - internal/config/family_literals_test.go
      - internal/executor/executor.go
      - internal/executor/mindspec_executor.go
      - internal/executor/mock.go
      - cmd/mindspec/config.go
      - internal/bootstrap/bootstrap.go
      - internal/redact/redact.go
      - internal/config/config_test.go
      - cmd/mindspec/config_test.go
      - cmd/mindspec/ceremony_guard_test.go
      - cmd/mindspec/panel_create_round_test.go
      - internal/setup/review_discipline_test.go
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
      - internal/setup/review_discipline_test.go
      - internal/setup/product_surface_test.go
      - internal/redact/redact.go
      - .mindspec/adr/ADR-0044-panel-review-conduct.md
      - .mindspec/adr/ADR-0045-binary-owns-sequence-and-rules.md
  - id: 3
    depends_on: []
    key_file_paths:
      - internal/panel/panel.go
      - internal/panel/verdict_v2.go
      - internal/panel/submit.go
      - internal/panel/ledger.go
      - internal/panel/slotlayout.go
      - internal/panel/slot_lock_logic.go
      - internal/panel/slot_lock_unix.go
      - internal/panel/slot_lock_windows.go
      - internal/panel/paneltest
      - internal/panel/create_round_test.go
      - internal/panel/handedit_test.go
      - internal/panel/ledger_test.go
      - internal/panel/passrule_test.go
      - internal/workspace/slotroot.go
      - cmd/mindspec/root.go
      - cmd/mindspec/panel_landed.go
      - cmd/mindspec/panel_schema.go
      - cmd/mindspec/panel_disposition_store.go
      - internal/redact/redact.go
      - .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md
  - id: 4
    depends_on: [1, 2, 3]
    key_file_paths:
      - internal/panel/panel.go
      - internal/panel/gate.go
      - internal/panel/tally.go
      - internal/panel/slotstatus.go
      - internal/panel/create.go
      - cmd/mindspec/panel.go
      - cmd/mindspec/panel_submit.go
      - cmd/mindspec/panel_refute.go
      - cmd/mindspec/panel_rebind.go
      - cmd/mindspec/panel_defer.go
      - cmd/mindspec/panel_abandon.go
      - cmd/mindspec/panel_disposition_store.go
      - cmd/mindspec/panel_disposition_query.go
      - cmd/mindspec/rules.go
      - cmd/mindspec/panel_create_integrity_test.go
      - cmd/mindspec/panel_defer_test.go
      - cmd/mindspec/panel_rebind_test.go
      - cmd/mindspec/panel_subject_test.go
      - cmd/mindspec/panel_test.go
      - cmd/mindspec/panel_verbs_test.go
      - internal/complete/panel_deferred_test.go
      - internal/complete/panel_gate_e2e_test.go
      - internal/complete/panel_resume_test.go
      - internal/panel/subject_test.go
      - internal/panel/create_round_test.go
      - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
  - id: 5
    depends_on: [1, 3]
    key_file_paths:
      - cmd/mindspec/panel_checkout.go
      - cmd/mindspec/panel.go
      - cmd/mindspec/panel_abandon.go
      - cmd/mindspec/git_boundary_test.go
      - internal/executor/executor.go
      - internal/executor/mindspec_executor.go
      - internal/executor/mock.go
      - internal/panel/panel.go
      - cmd/mindspec/migrate.go
      - internal/bootstrap/bootstrap.go
      - internal/doctor/ownership.go
      - internal/doctor/links.go
  - id: 6
    depends_on: [1, 2, 3]
    key_file_paths:
      - internal/panel/brief.go
      - internal/panel/prompt.go
      - internal/panel/create.go
      - internal/panel/create_test.go
      - cmd/mindspec/panel.go
      - cmd/mindspec/panel_prompt.go
      - .mindspec/adr/ADR-0045-binary-owns-sequence-and-rules.md
  - id: 7
    depends_on: [1, 3, 4, 5, 6]
    key_file_paths:
      - internal/panellaunch/launch.go
      - internal/panellaunch/supervisor_unix.go
      - internal/panellaunch/launch_windows.go
      - internal/panellaunch/boundary_test.go
      - cmd/mindspec/panel_launch.go
      - cmd/mindspec/panel_recover.go
      - cmd/mindspec/panel_checkout.go
      - plugins/mindspec/workflows/ms-panel.js
      - .claude/workflows/ms-panel.js
      - plugins/mindspec/workflow_test.go
      - plugins/mindspec/scripts/codex_verdict_extract.sh
      - internal/setup/surface_refs_test.go
      - internal/setup/historical_workflows
      - internal/setup/upgrade_refresh_workflow_test.go
  - id: 8
    depends_on: [1, 2, 3]
    key_file_paths:
      - internal/doctor/panel_runners.go
      - internal/doctor/staged_markers.go
      - internal/doctor/stale_checkouts.go
      - internal/doctor/panel_runners_test.go
      - internal/doctor/doctor.go
      - internal/doctor/config.go
      - internal/doctor/config_test.go
      - internal/doctor/models_commands_test.go
      - cmd/mindspec/doctor.go
  - id: 9
    depends_on: [2, 4, 5, 6]
    key_file_paths:
      - plugins/mindspec/skills/ms-panel-run/SKILL.md
      - plugins/mindspec/skills/ms-panel-tally/SKILL.md
      - plugins/mindspec/skills/ms-bead-fix/SKILL.md
      - plugins/mindspec/skills/ms-bead-cycle/SKILL.md
      - plugins/mindspec/skills/ms-spec-final-review/SKILL.md
      - plugins/mindspec/skills/ms-bead-impl/SKILL.md
      - .claude/skills/ms-panel-run/SKILL.md
      - .claude/skills/ms-panel-tally/SKILL.md
      - .claude/skills/ms-bead-fix/SKILL.md
      - .claude/skills/ms-bead-cycle/SKILL.md
      - .claude/skills/ms-spec-final-review/SKILL.md
      - .claude/skills/ms-bead-impl/SKILL.md
      - internal/setup/historical_skills
      - internal/setup/upgrade_refresh_spec131_test.go
      - internal/setup/review_discipline_test.go
      - internal/setup/adhoc_panel_skill_test.go
      - internal/setup/panel_tally_skill_bead_test.go
      - internal/setup/panel_skills_test.go
      - internal/setup/skills_test.go
      - internal/lint/destructive_guidance_test.go
      - internal/setup/readiness_skill_pin_test.go
      - cmd/mindspec/bead_ready_test.go
      - internal/setup/upgrade_refresh_pre126_test.go
      - cmd/mindspec/slot_lock_windows_test.go
      - .mindspec/domains/workflow/interfaces.md
      - CHANGELOG.md
      - project-docs/user/guides/review-panels.md
      - README.md
      - plugins/mindspec/README.md
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
- **ADR amendments with their citing beads (R13).** ADR-0040 in Bead 1. ADR-0044 in Bead 2. ADR-0045 in Bead 2 (rule table, `instruct`, marker form) and Bead 6 (BRIEF and slot prompt templates), under one `## Amendment (Spec 131)` heading. ADR-0043 in Bead 3. ADR-0037 in Bead 4. Each bead adds its own pin test file `internal/validate/adr_spec131_<adr>_test.go` with a function named `TestADRAmendments_Spec131<ADR>`.
- **ADR pin constraint.** The spec-130 pins (`TestADRAmendments_Spec130_*`) read each ADR to end of file, and the deferral pin needs a unique heading prefix. So a Spec 131 amendment must not repeat a Spec 130 bullet heading (a line that starts `- **<heading>` as used under the Spec 130 amendment) and must not contain the literal `## Amendment (Spec 130)`. Each bead that amends an ADR runs the spec-130 pins after its edit.
- **Root discovery (choice: move the helper to wave 1).** Bead 3 owns the one root helper, so no new edge is needed and the chain stays at 3. Bead 3 adds `workspace.FindSlotRoot(cwd string, isolationRoot func(mainRoot string) (string, error)) (SlotRoot, bool, error)` and the cmd-layer `findPanelRoot(isolationRoot func(mainRoot string) (string, error)) (string, error)` in `cmd/mindspec/root.go`. With no slot directory above the cwd, `findPanelRoot` returns what `findRoot` returns. Bead 1 adds `config.IsolationRootFor(mainRoot string) (string, error)`; every caller passes it. Bead 4 changes `resolvePanelVerb`, every existing panel RunE, the `panel disposition query` glob and `mindspec rules` to call `findPanelRoot` (the file list is in the `cmd/mindspec/panel.go` seam below). Beads 5, 6 and 7 call it in `panel checkout`, `panel prompt`, `panel launch` and `panel recover`. Bead 7 (after 4, 5 and 6) adds `TestSlotRoot_EveryPanelVerb`.
- **Shared helpers with one owner.** Later beads code against these signatures and do not write a second copy.
  - Executor `LsFiles` (tracked check): Bead 1. No bead adds a path-at-SHA call or an ignore call: the existing `FileAtRefOrAbsent` (`present` is false for a path absent at the SHA) and `CheckIgnore` do that work. Bead 5 adds the clone calls and uses `CheckIgnore`. Bead 6 uses `FileAtRefOrAbsent`.
  - `panel.LatestSubmissions(dir string, round int) (map[string]SubmissionRecord, error)` (the latest `verdict` record per slot in that round only; a record of another round never counts): Bead 3. Current-round callers pass round N: Bead 4 (gate, slot status, verify), Bead 5 (the `--dispatch` final-slot refusal) and Bead 7. Bead 6 passes N-1 for prior-round items.
  - `landedBindingFor(readMeta func(beadID string) (map[string]interface{}, error), beadID, slug string, round int) (bool, error)` in `cmd/mindspec/panel_landed.go` (true when the bead's landed-review metadata binds this panel round): Bead 3. It reads bd metadata the same way as `landedBindingMetadataFn` in `internal/lifecycle`; production passes `bead.GetMetadata`, tests pass a fake. An empty `beadID` (ad hoc panel) gives false. Beads 4, 5 and 7 call it.
  - Slot lock API (`AcquireSlotLock`, `TrySlotLock`, `ProbeSlotLock`) and the slot-layout paths: Bead 3.
  - `internal/panel/paneltest`: Bead 3 (v2 panel fixture, ledger seeder for `round`, `verdict`, `dispatched`, `started`, `launch_failed`, `substitution` and attempt directories, verdict builder).
- **Redaction token table (choice: wave 1 only).** `TestRedactEnum_NoCobraDrift` runs under `-short` and needs each new cobra verb word in `internal/redact/redact.go`. Extra tokens do not fail it. Bead 1 adds `resolve` (`SubcommandTokens`). Bead 2 adds `rules` (`CommandTokens`). Bead 3 adds `schema`, and also `submit`, `checkout`, `prompt`, `launch` and `recover` for the verbs of Beads 4, 5, 6 and 7 (`SubcommandTokens`). Each edit only adds map entries. The wave-1 merge order is 1, then 2, then 3; each later bead rebases on the earlier merge. No wave-2 or wave-3 bead edits this file. The same order 1, 2, 3 applies to every file that wave-1 beads share, for example `internal/setup/review_discipline_test.go` (Beads 1 and 2).
- **Shared files in wave 2.** Beads in one wave edit one file only at the seams below. The merge order in wave 2 is 4, then 5, then 6, then 8. A later bead rebases on the earlier merge and keeps its edit inside its seam.
  - `internal/panel/create.go`: Bead 4 adds the `verdict_schema` and `family` writes and the first-v2 carry in `Create`. Bead 6 adds one `BriefRegions` field to `CreateInput` and one render call in `renderCreateBriefHeader`.
  - `cmd/mindspec/panel.go`: Bead 4 changes the root lookup of every RunE in this file and adds the submit wiring. Its Step 5 changes the root lookup in these files and no others: `panel.go`, `panel_refute.go` (`resolvePanelVerb`), `panel_rebind.go`, `panel_disposition_store.go`, `panel_disposition_query.go`, `panel_defer.go`, `panel_abandon.go` and `rules.go`. Bead 5 adds one call `cleanupPriorRound(...)` before `panel.Create`; it takes the slot locks in sorted order, then the ledger lock. Bead 6 fills `CreateInput.BriefRegions`.
  - `internal/panel/panel.go`: Bead 4 edits the pinned-field list. Bead 5 edits `Scan` (skip the isolation root).
  - `cmd/mindspec/panel_abandon.go`: Bead 4 changes the root lookup. Bead 5 adds the cleanup call.
  - Executor files: Bead 1 (wave 1) and Bead 5 (wave 2) do not run at once, because Bead 5 depends on Bead 1.
- **Attempt binding (choice: move the check to Bead 3).** The R2.9 check runs in `panel.Submit` under the ledger lock, so a launch between the check and the append cannot pass a superseded attempt. No new edge: Bead 4 only wires the CLI.
- **Deleted-name sweep.** For each name that a bead deletes or renames, `git grep -l -F -e <name> 38bc9cde -- '*_test.go'` gave these test files. Each hit is in that bead's key_file_paths. Hits on the English words "substitution" and "substitutes" in other test files are not the config field and need no edit. A second sweep at 38a13240 covers (a) in-package callers of each changed function and (b) each addition that can break a build or a test: a new struct field (unkeyed literals), a new parameter, a new interface method. Its hits are in the addition table below and in key_file_paths. No hit adds a dependency edge.

| Bead | Deleted or renamed | Test files that change |
|------|--------------------|------------------------|
| 1 | `knownModels`, `KnownModels()`, the known-model warning | `cmd/mindspec/config_test.go` (warning text) |
| 1 | `defaultLenses` | `internal/config/config_test.go` |
| 1 | `panel.substitution`, `claude_sub_on_quota`, `ClaudeSubOnQuota`, `.Substitution` | `internal/config/config_test.go`, `cmd/mindspec/config_test.go`, `cmd/mindspec/ceremony_guard_test.go` |
| 1 | the two-name family literal in `resolveGateReviewers` | `internal/config/config_test.go` (`TestPanelGateDefaults_RoundRobinFamilies`), `internal/setup/review_discipline_test.go` (`TestReviewDiscipline_AC1PanelSizeConsistency`), `cmd/mindspec/panel_create_round_test.go` (`TestPanelCreate_ReviewersN` defaults case): each expects `harness` slots |
| 2 | marker `target=next-action` form, `readinessStagedRule` | `internal/setup/readiness_skill_pin_test.go`; `internal/validate/adr_spec130_verbs_test.go` reads ADR-0045 text that stays, so no edit (stay-green row) |
| 3 | none (it adds `RosterSlot.Family`; see the addition table) | `internal/panel/create_round_test.go`, `handedit_test.go`, `ledger_test.go`, `passrule_test.go` |
| 4 | v1 round from `create` (a hand-written verdict file no longer counts) | `cmd/mindspec/panel_create_integrity_test.go`, `panel_defer_test.go`, `panel_rebind_test.go`, `panel_subject_test.go`, `panel_test.go`, `panel_verbs_test.go`, `internal/complete/panel_deferred_test.go`, `panel_gate_e2e_test.go`, `panel_resume_test.go` (found by grep for a create plus a verdict write; the implementer checks each); in-package `Create`: `internal/panel/subject_test.go` (`TestGate_SubjectTreeStaleness`). `internal/panel/create_round_test.go` (`TestCreate_ArchivesBriefOnBump` stays on a v2 round and now expects 0 `Verdicts` and 1 `Unsubmitted` entry). No edit, re-checked against the `Tally` filter: `create_test.go` and `create_integrity_test.go` (no tally after a verdict file), `internal/complete/fault_injection_realgit_test.go` (false hit on `noopWorktreeOps.Create`; its panel has no `round` event, so v1 rules apply) |
| 5 | none | none |
| 6 | none | none |
| 7 | `VALID_MIX_FAMILIES`, the substitution flag, the parse-retry ladder, `codex_verdict_extract.sh` | `internal/setup/surface_refs_test.go`, `plugins/mindspec/workflow_test.go`, `internal/setup/upgrade_refresh_workflow_test.go` |
| 8 | none (two config loads become one) | `internal/doctor/config_test.go` (`checkSourceGlobs`), `internal/doctor/models_commands_test.go` (`checkModels`) |
| 9 | skill text (see Bead 9 Step 4) | the pin files named in Bead 9 Step 4 |

| Bead | Addition or changed caller | Test files that change |
|------|----------------------------|------------------------|
| 1 | `LsFiles` on `executor.Executor`; `ReviewerSlot.Family` | none: only `MindspecExecutor` and `MockExecutor` implement the interface (both in key paths), and test decorators embed it; every `ReviewerSlot` literal is keyed |
| 2 | `instruct` marker output | none: no unkeyed literal of a changed type |
| 3 | `RosterSlot.Family` | the four unkeyed-literal files in the Bead 3 row; every other literal is keyed |
| 4 | v2 rules for a round from in-package `Create`; `verdict_schema` in the pinned-field list | `internal/panel/subject_test.go`; the field-matcher tables (`gate_integrity_test.go`, `handedit_test.go`) list only the fields they edit, so no edit |
| 5 | clone calls on `executor.Executor`; the isolation-root skip | none: same implementers as Bead 1; `Scan(roots ...string)` keeps its signature (8 test files call it); the doctor and migrate walks keep theirs |
| 6 | `CreateInput.BriefRegions`; a regions parameter on `renderCreateBriefHeader` | `internal/panel/create_test.go` (its direct call passes empty regions); every `CreateInput` literal is keyed |
| 7 | workflow rewrite | none beyond the deleted-name row; `historical_workflows/ms-panel.presurface.js` is a frozen snapshot |
| 8 | `Options.Probe`; one config load in `checkModels` and `checkSourceGlobs` | the two files in the Bead 8 row; no unkeyed `Options` literal |
| 9 | skill text | none beyond Bead 9 Step 4 |

- **Adapter redispatch.** The executable adapter is `ms-panel.js` (Bead 7). So Bead 7 owns `TestPanelRun_SilentHarnessRedispatchOnce`. Bead 9 writes the same rule into the skill table and pins it with `TestPanelRun_StatusTable`.
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
- **ADR-0030 (executor boundary).** Applied. Clone, remote, `ls-files`, `check-ignore` and `cat-file` calls go through `internal/executor`. `internal/panel` stays a leaf: it may import only `internal/termsafe` and `internal/idvalidate` (`TestPanelLeafImports_StdlibPlusTermsafeOnly`). So config resolution, rule lookup and git calls happen in the cmd layer, which passes values or callbacks into `internal/panel`. Process supervision lives in a new package `internal/panellaunch`. `TestEnforcementHasNoGitLeaks` scans only `internal/{validate,approve,complete,state,phase}`. It does not guard `cmd/mindspec`, `internal/workspace` or `internal/panellaunch`. So Bead 5 adds `TestPanelCheckout_GitOnlyViaExecutor` and Bead 7 adds `TestPanelLaunch_NoGitCalls` (see those beads).
- **ADR-0035 (agent error contract).** Applied. Every new refusal ends with one `recovery:` line. No divergence is proposed.

## Testing Strategy

- **Dogfood boundary.** This spec changes the panel verbs, the gate, the panel skills and the `/ms-panel` workflow that this lane uses. Every panel of this lane runs on the installed v0.17.0 binary `~/.local/bin/mindspec`. No panel runs on a spec-branch build. Nobody installs a branch build. Lane panels use the main checkout's installed skills and `/ms-panel` workflow, never the spec branch's rewritten copies, until the spec merges. The R4.13 `panel.runners` block is safe for the released binary, because its config load ignores unknown keys.
- **No real model CLI.** Tests use temp repos (`t.TempDir()` with `git init`) and local fake runner scripts only. No test starts a real `codex`, `claude` or other model CLI. The R4.12 suggestion tests put a fake executable on a temp `PATH`.
- **Presence rule.** A wrong bead must not pass because a test is absent. A `go test -run` pattern that matches no test passes. So each "owned tests" item below means: `go test <pkgs> -run '^(<names>)$' -v -count=1` exits 0 and prints one top-level `--- PASS: <Name>` line for each listed name. A missing line fails the item. Every `-run` pattern in this plan is anchored. A build-tagged Windows test is checked by `GOOS=windows go vet ./...` and by a `grep` for its `func` line in a file with `//go:build windows`.
- **Layers.**
  - Unit: `internal/config`, `internal/rules`, `internal/panel` (pure; fake resolvers and callbacks), `internal/workspace`.
  - Integration: `cmd/mindspec` tests drive the verbs in temp repos. The Bead 3 `paneltest` seeder writes ledger records and attempt directories where a test needs a state that a later bead's verb makes.
  - Process: `internal/panellaunch` tests run fake runners under the supervisor. Unix-only tests use `_unix_test.go` or a `!windows` build tag. Windows tests use a `windows` build tag.
  - Workflow: Bead 7 runs `ms-panel.js` under `node` from Go with a stub harness API. The test fails, and does not skip, when `node` is absent.
- **Windows gap.** CI has no Windows job, and `.github/` is outside the spec's Impacted Domains. So the R7.9 stale-file algorithm is OS-neutral code (`decideStaleLock` in `slot_lock_logic.go`) with injected pid-liveness and file operations, and Bead 3 tests it on Unix. Bead 3 `TestSlotLock_WindowsUsesDecisionLogic` parses `slot_lock_windows.go` with `go/parser` on any host and fails unless that file calls `decideStaleLock`, so the Windows binding cannot bypass the tested logic. `GOOS=windows go vet ./...` runs in Beads 3, 7 and 9, so the Windows code and the build-tagged tests compile. Nothing in this plan executes `TestSlotLock_WindowsLifecycle` or `TestPanelLaunch_WindowsRefuses`. That execution is a named gap; it needs a Windows host after the spec merges.
- **Shipped-surface rule.** Every bead that adds or changes surface text (skills, workflows, setup-written files, templates, rule table, data files, CLI help, errors and output, user manuals) cites no ADR-NNNN, spec number, R-id or §, bead ID or issue number. `TestSurfaceRefs_NoInternalReferences` (exists; Go string literals) and `TestProductSurface_NoInternalRefs` (Bead 2; embedded data files and rule fields) enforce it. Each later bead keeps both green.
- **Gates for every bead.** `go build ./...`, `go test ./... -short`, `go vet ./...` and `gofmt -l .` (empty output). CI runs `-short` and the race tests.

## Bead 1: Panel config: runners, lenses, local file, isolation root

Scope: R4.1–R4.7 (config side), R4.9–R4.13, R5, R6.1, and the R4.10 and R6.2 `.gitignore` lines from `mindspec init`.

**Steps**
1. Add `panel.runners` to `internal/config` per spec "R4 — Runners and slot resolution" R4.1–R4.4: types, argv placeholder check, merge by family over the default `{family: harness, kind: harness}`, default `timeout_minutes` 60 for a process runner. Make zero-config fill go round-robin over the merged runner order. Delete the two-name literal in `resolveGateReviewers`. Add `TestNoFamilyLiterals` per R4.5. Its scanner takes a root directory. The test runs it on the repository, and on a `t.TempDir()` fixture with a planted `"codex"` literal in a non-test `.go` file, and asserts one hit there (the same form as `TestSurfaceRefs_NegativeEachClass`).
2. Add family and model resolution to slot expansion: each slot gets `Family`; model order per R4.6; a `SubstituteModel(toFamily)` helper; model-safe refusal from every source. A model-only entry gets family `harness` (R4.7). A family with no runner refuses with the recovery `add a runner for the family to panel.runners`. `create` and `config resolve` both get this refusal through slot expansion.
3. Add `.mindspec/config.local.yaml` (R4.10): only `panel.runners` and `models`, a 65536-byte limit, strict keys, merge after `config.yaml`. Retire `panel.substitution` (R4.9) with the `on_quota` recovery. Delete `knownModels`, `KnownModels()` and the `config show` known-model warning (R4.5). Update the tests that these deletes reach: in `internal/config/config_test.go`, make the `panel.substitution` fixtures load-error cases and make `TestPanelGateDefaults_RoundRobinFamilies` expect `harness` slots; in `cmd/mindspec/config_test.go`, remove the `substitution` key and known-model advisory assertions of `TestConfigShow_GatesSubstitutesAndModelAdvisory` and `TestConfigShowGate_ResolvedJSON`; in `cmd/mindspec/ceremony_guard_test.go`, remove the three `panel.substitution*` keys from the `TestCeremonyNonInflation_ConfigKeys` want set and add `panel.runners`, `panel.lenses` and `panel.isolation_root`; make the zero-config defaults case of `TestPanelCreate_ReviewersN` (`panel_create_round_test.go`) and the family counts of `TestReviewDiscipline_AC1PanelSizeConsistency` (`review_discipline_test.go`) expect `harness`.
4. Add R5: `panel.lenses`, `panel.gates.<g>.lenses|acceptance`, the R5.4 and R5.5 default tables as data, lens-name validation, and a lens-only gate entry. Delete `defaultLenses`; rewrite its coverage loop and the slot-lens check in `internal/config/config_test.go` against the default lens table. Add `panel.isolation_root` with the R6.1 lexical refusals in load, and `config.IsolationRootFor(mainRoot)`.
5. Add one executor call in `executor.go`, `mindspec_executor.go` and `mock.go`: `LsFiles` (is a path tracked). Add no path-at-SHA call; the existing `FileAtRefOrAbsent` does it. Use `LsFiles` for the tracked-root refusal at the cmd-layer config load.
6. Add `mindspec config resolve --gate G [--json]` with per-value source and the R4.7 model-only Warn. Add `internal/config/review_tools.yaml` and the R4.12 suggestion line. Make `mindspec init` add `config.local.yaml` and the effective isolation root to `.gitignore`. Add `resolve` to `SubcommandTokens` in `internal/redact/redact.go`.
7. Add the R4.13 `panel.runners` block to this repository's `.mindspec/config.yaml`. Append `## Amendment (Spec 131)` to ADR-0040 per R13.5, and add `TestADRAmendments_Spec131ADR0040`.

**Verification**
- [ ] Owned tests (presence rule): `go test ./internal/config/... ./internal/executor/... ./internal/bootstrap/... ./internal/validate/... ./cmd/mindspec/... -run '^(TestModelValue_Refusals|TestRunners_Validate|TestRunners_TimeoutDefault|TestRunners_ZeroConfigHarness|TestRunners_MergeByFamily|TestSubstitution_Retired|TestNoFamilyLiterals|TestSlotModelResolution|TestReviewer_ModelOnlyIsHarness|TestReviewer_ModelOnlyWarn|TestCreate_RefusesUnrunnableFamily|TestSubstituteModel|TestLocalConfig_Merge|TestLocalConfig_Refusals|TestConfigResolve_Provenance|TestConfigResolve_ToolSuggestion|TestRepoConfig_Resolves|TestLenses_GateDefaults|TestLenses_LensOnlyGateEntry|TestLenses_UnknownRefused|TestIsolationRoot_Validate|TestBootstrap_ScratchIgnoredExisting|TestExecutor_LsFiles|TestADRAmendments_Spec131ADR0040)$' -v -count=1` prints a `--- PASS:` line for each of the 24 names. `TestCreate_RefusesUnrunnableFamily` and `TestReviewer_ModelOnlyWarn` are in `cmd/mindspec` (the `create` and `config resolve` legs)
- [ ] `TestNoFamilyLiterals -v` output shows its planted-fixture subtest passing (the scanner reports the planted `"codex"`)
- [ ] Stay green: `go test ./internal/config/...`, `go test ./cmd/mindspec/... -run '^(TestADR0040|TestRedactEnum_|TestCeremonyNonInflation_ConfigKeys|TestConfigShow_|TestPanelCreate_ReviewersN)'`, `go test ./internal/setup/... -run '^TestReviewDiscipline_AC1PanelSizeConsistency$'`, `go test ./internal/next/... -run '^TestGateReadiness'`, `go test ./internal/validate/... -run '^TestADRAmendments_Spec130_'` and `go test ./internal/setup/... -run '^(TestRunClaude_|TestSurfaceRefs_)'`
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-5 in full: every listed load refusal; the 60-minute default; zero-config `bead` gives 4 `harness` slots; the merge-by-family order; `TestNoFamilyLiterals` passes on `G1-codex` and reports the planted `"codex"`; `KnownModels` does not exist
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
2. Add `mindspec rules [--scope S] [--json]` and `mindspec rules show <id>` in `cmd/mindspec/rules.go`. An unknown id or scope refuses with a `recovery:` line. Add `rules` to `CommandTokens` in `internal/redact/redact.go`. Bead 4 changes its root lookup to `findPanelRoot`.
3. Make `instruct` render a `## Rules` section: orchestrator-scope rules in every mode, plus implementer-scope rules in implement mode (R10.6).
4. Change the `ms-bead-impl` staged-rule marker in both copies to `<!-- staged-rule: id=sequence.bead-impl-prompt; remove-by=YYYY-MM-DD -->` (R10.5). Add a marker parser. Update `readinessStagedRule` in `internal/setup/readiness_skill_pin_test.go` and the matching rows in `internal/setup/review_discipline_test.go`.
5. Add `TestProductSurface_NoInternalRefs` (R10.7) in `internal/setup/product_surface_test.go`. It parses every non-test `.go` file under `internal/` with `go/parser`, collects every `//go:embed` directive, and scans each file that a directive names, plus every rule field. The pattern set is `ADR-[0-9]`, `[Ss]pec [0-9]{3}`, `\bR[0-9]+\.[0-9]`, `#[0-9]{3}` and the bead-id pattern. It exempts only `internal/setup/historical_skills/` and `internal/setup/historical_workflows/`, which hold frozen earlier bodies used as digest oracles and never written as new content; the exemption is a named list in the test. A negative subtest plants `R7.3` in a temp data file, runs the same scanner and asserts a hit. Go string literals stay covered by `TestSurfaceRefs_NoInternalReferences`.
6. Append `## Amendment (Spec 131)` to ADR-0044 per R13.2. Start the ADR-0045 amendment with the step-1, step-5 and marker-form items (R13.1). Obey the ADR pin constraint in `## Lane resolutions`. Add `TestADRAmendments_Spec131ADR0044` and `TestADRAmendments_Spec131ADR0045Rules`.

**Verification**
- [ ] Owned tests (presence rule): `go test ./internal/rules/... ./cmd/mindspec/... ./internal/instruct/... ./internal/setup/... ./internal/validate/... -run '^(TestRuleTable_MinimumSet|TestStagedMarker_Parse|TestRulesCmd|TestInstruct_RulesSection|TestProductSurface_NoInternalRefs|TestADRAmendments_Spec131ADR0044|TestADRAmendments_Spec131ADR0045Rules)$' -v -count=1` prints a `--- PASS:` line for each of the 7 names
- [ ] `TestProductSurface_NoInternalRefs -v` output shows the planted-`R7.3` subtest passing
- [ ] Stay green after the ADR edits: `go test ./internal/validate/... -run '^TestADRAmendments_Spec130_'` (incl. the ADR-0045 marker text in `TestADRAmendments_Spec130_Verbs`, `adr_spec130_verbs_test.go`, which needs no edit)
- [ ] Stay green: `go test ./internal/setup/... -run '^(TestMsBeadImplSkill_|TestReviewDiscipline_|TestSurfaceRefs_|TestLifecycleGateStep_)'` and `go test ./cmd/mindspec/... -run '^TestRedactEnum_'`
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-13: the table holds every R10.2 id; `rules --scope reviewer` lists exactly the reviewer rules; `rules show nope` refuses; `instruct` lists `lifecycle.no-probe-complete` in spec and implement mode and `review.derive` only in implement mode; the `ms-bead-impl` marker parses with `id=sequence.bead-impl-prompt`; the internal-reference scan is clean on every embedded data file and goes red on the planted fixture
- [ ] AC-16: ADR-0044 carries the R13.2 amendment; ADR-0045 carries the R13.1 rule-table items

**Depends on**
None

## Bead 3: Verdict v2, ledger kinds, submit core, slot layout, lock and root helper

Scope: R1, R2.4–R2.6 and R2.8 (core), the `verdict`, `launch` and `substitution` record kinds, the slot-directory layout, the slot lock (Terms, R7.3 lock tests, R7.9 lock-file rules), the R6.9 root helper, and the shared helpers in `## Lane resolutions`.

**Steps**
1. Add the v2 verdict per spec "R1 — Verdict schema v2": one Go definition that gives the schema layer and the JSON Schema, plus the transport and relation layer. Each error names its JSON pointer. Add the R1.6 projection to `ccr[i]` and the slot hash. Add `mindspec panel schema`. Add `schema`, `submit`, `checkout`, `prompt`, `launch` and `recover` to `SubcommandTokens` in `internal/redact/redact.go` (see `## Lane resolutions`).
2. Add the three record kinds to the ledger (`IsVerbRecord`, `validateLedgerMap`, keys per the R2.5 and R7.6 tables). Add `Family` to `RosterSlot` in `panel.go`, and change the unkeyed `RosterSlot` literals in `create_round_test.go` (`createRoster`), `handedit_test.go`, `ledger_test.go` and `passrule_test.go` to keyed fields (a legacy fixture keeps an empty `Family`). Make `panel disposition append` refuse the three kinds; `validate|check|query` accept them.
3. Add `panel.Submit` in the leaf package. It takes the target (with the attempt `j` parsed from an `attempt-<j>/` file path, if any), the bytes and a resolver callback that returns the effective model, `runner_kind` and `runner_source`. Under the ledger lock it runs the pure R2.3 checks, then the R2.9 attempt binding (it reads the slot's latest `dispatched` or `started` attempt and its outcome under the same lock and refuses `attempt <j> was superseded by <k>` or an ended attempt), then the R2.5 same-bytes repair or the append with `seq` and the temp-file rename, then the `submitted` outcome for an open `dispatched` attempt. Add a crash-injection hook and a lock-held hook for tests. Add `panel.LatestSubmissions(dir, round)`; it reads only the `verdict` records of that round.
4. Add slot-layout path helpers (`<isolation_root>/<spec or adhoc>/<slug>/r<N>/<slot>/`, `slot.json`, `tree/`, `attempt-<k>/`, `.locks/…/<slot>.lock`) and the attempt-number rule of R7.2. Add `internal/panel/paneltest` (fixture, seeder, verdict builder, and a v1-round helper: a `round` event with no `verdict_schema`, for the Bead 4 test migration).
5. Add the slot lock API. Unix: `flock`, no file deletion. Windows: put the R7.9 decision logic in `decideStaleLock` in `slot_lock_logic.go` (no build tag), with injected pid-liveness and file operations (exclusive create, read, rename, link, remove): a dead pid is stale and free; an empty or unparseable file is held; the stale-file rename, re-read and delete-only-if-same-pid; the relink put-back; the link-fails case deletes the moved file only when its pid is dead, else refuses and names both files; no path deletes a file whose pid is live. `slot_lock_windows.go` binds the real OS calls and calls `decideStaleLock`; `TestSlotLock_WindowsUsesDecisionLogic` checks that call with `go/parser`. Add the non-waiting probe and the 1-second bounded retry (R7.3). Follow `disposition_lock_unix.go` and `disposition_lock_windows.go`.
6. Add `workspace.FindSlotRoot` in `internal/workspace/slotroot.go` per R6.9: the nearest ancestor with `slot.json` and `tree/`, the exact-path check against the isolation-root callback, and the `slot.json` field check. Add `findPanelRoot` in `cmd/mindspec/root.go` (signature in `## Lane resolutions`). Add `landedBindingFor` in `cmd/mindspec/panel_landed.go` (signature in `## Lane resolutions`; it takes the bd metadata reader, not the executor).
7. Append `## Amendment (Spec 131)` to ADR-0043 per R13.4, obeying the ADR pin constraint. Add `TestADRAmendments_Spec131ADR0043`.

**Verification**
- [ ] Owned tests (presence rule): `go test ./internal/panel/... ./internal/workspace/... ./internal/validate/... ./cmd/mindspec/... -run '^(TestVerdictV2_Accepts|TestVerdictV2_Refusals|TestVerdictV2_TransportLayer|TestVerdictV2_SchemaMatchesValidator|TestPanelSubmit_Writes|TestPanelSubmit_Replace|TestPanelSubmit_CrashRepair|TestPanelSubmit_Race|TestPanelSubmit_AttemptBindingUnderLock|TestLatestSubmissions|TestSlotLock_StaleLogic|TestSlotLock_WindowsUsesDecisionLogic|TestSlotLock_UnixProbeNoDelete|TestFindSlotRoot|TestFindPanelRoot_NoSlotFallsBack|TestLandedBindingFor|TestDispositionAppend_RefusesV2Kinds|TestPanelSchema|TestADRAmendments_Spec131ADR0043)$' -v -count=1` prints a `--- PASS:` line for each of the 19 names
- [ ] `TestLatestSubmissions` uses one ledger with round-1 items X and round-2 items Y. Round 1 gives X and round 2 gives Y, also after a round-2 draft then final. A round with no submission gives no entry for a slot, even when an earlier round has one. `TestSlotLock_WindowsUsesDecisionLogic` has a negative subtest: a temp file with no `decideStaleLock` call gives a hit
- [ ] `go test ./internal/panel/... -run '^TestPanelSubmit_Race$' -race -v -count=1` exits 0 and prints `--- PASS: TestPanelSubmit_Race`
- [ ] `TestPanelSubmit_AttemptBindingUnderLock` is a `package panel` test. The lock-held hook is an unexported package variable; the test file has an unexported raw-append helper that takes no lock. The test does not import `paneltest` (import cycle). The test is deterministic: the hook fires inside the lock-acquire wait path, when the first try-lock fails because the test holds the ledger lock; the hook only signals and does not block. The test holds the ledger lock, starts a submit from `attempt-1/`, waits for the signal, appends `launch_failed` for attempt 1 and `dispatched` for attempt 2 with the raw helper (the held flock would block `AppendRecords`, so the test cannot deadlock), then releases the lock. The submit refuses with `attempt 1 was superseded by 2`, and the ledger holds no `submitted` or `verdict` record after the seeded ones. An attempt check placed before the lock is held, including one between the hook and the lock, reads the unseeded ledger and makes this test fail
- [ ] `TestSlotLock_StaleLogic` runs on Unix with fakes and has one subtest each for: dead pid freed; empty file held; unparseable file held; live pid never deleted; rename re-read finds a new pid and relinks; relink race leaves one holder; link fails and the moved file's pid is live, so it refuses and names both files
- [ ] Stay green: `go test ./internal/panel/... -run '^(TestPanelLeafImports_StdlibPlusTermsafeOnly|TestLedger|TestScan_)'`, `go test ./internal/validate/... -run '^TestADRAmendments_Spec130_'` and `go test ./cmd/mindspec/... -run '^TestRedactEnum_'`
- [ ] `GOOS=windows go vet ./...` passes
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-1 in full, including the 1 MiB padding case, the 4096-byte `tail` case and schema-versus-validator agreement
- [ ] AC-2 (code only): the R1.6 projection gives `ccr[i]` and the slot hash; Bead 4 owns the test, because the refute leg needs the v2 gate
- [ ] AC-3: submit writes the file and one `verdict` record with `seq: 1`; a same-bytes rerun adds no record; final APPROVE then final REQUEST_CHANGES both stay in the ledger; a crash after the append repairs on rerun; two concurrent submits leave the file that matches the latest record; `panel disposition append` refuses each new kind
- [ ] AC-10 (logic only): the R7.9 stale-file rules hold under `TestSlotLock_StaleLogic`
- [ ] AC-16: ADR-0043 carries the R13.4 amendment

**Depends on**
None

## Bead 4: Gate, verify, submit CLI and root wiring

Scope: R2.1–R2.3, R2.7, R2.9, R3, R4.6–R4.8 (create side), R6.9 wiring, R8.

**Steps**
1. Make `create` write `verdict_schema: 2` to `panel.json` and every new `round` event, and add it to the pinned-field list in `panel.go` (R3.1). Write `family` on each roster entry. Add the first-v2-round family carry and its refusal (R4.8). Stay inside the `create.go` seam in `## Lane resolutions`.
2. Change the gate per spec "R3 — The gate on a v2 round": v2-ness from the latest `round` event only; a slot is in only by R3.3; R3.5 discharge binding on the latest submission of the current round (`panel.LatestSubmissions(dir, N)`), in the gate filter of `tally.go` (`Tally`, which `ResolveGateFacts` calls); R3.6 rounds with no `verdict_schema` keep spec 130's rules. On a v2 round, `Tally` itself leaves a verdict file with no matching latest submission (R3.4) out of `Verdicts`, `Approves`, `Rejects` and `HardBlocks`, and lists its file name in a new field `Result.Unsubmitted []string`. So every `Tally` caller (the gate, `complete`, impl-approve, the coverage manifest, `instruct` and the verb re-tallies) sees the same counts. `verify` reads `Unsubmitted` to show `unsubmitted`. A v1 round keeps spec 130's behaviour.
3. Add `mindspec panel submit` (long and short form) in `cmd/mindspec/panel_submit.go`. It resolves config, the effective runner and the effective model, and passes them to `panel.Submit`. Add the landed-binding refusal (`landedBindingFor`), the process-slot refusal and the status print (R2.7). For R2.9 it parses the file path and passes the attempt to `panel.Submit`, which does the binding under the ledger lock (Bead 3); this step only wires the CLI to it. Add no `--via` flag.
4. Add R8 slot status to `panel verify`: the closed status set, the R8.2 rule order, R8.3 annotations, `--json` with `handoff_pending`, the R8.6 exit codes, and `termsafe` for every reviewer string. `tally` prints one line per `replaced` slot. Verify writes nothing.
5. Change the root lookup to `findPanelRoot(config.IsolationRootFor)` (R6.9) in the files that the `cmd/mindspec/panel.go` seam lists: every RunE in `panel.go`, `resolvePanelVerb` in `panel_refute.go`, `panel_rebind.go`, `panel_disposition_store.go`, `panel_defer.go`, `panel_abandon.go` and `rules.go`, and `resolveDispositionQueryGlob` in `panel_disposition_query.go` (the default glob and the `--spec` glob resolve under that root; an explicit `--dir` stays as given). Add a real `git clone --shared` test helper for AC-9 (test code may run git). Move each existing test that creates a round and then writes a verdict file by hand (the Bead 4 rows of the deleted-name sweep) to a submit through `panel.Submit` or the `paneltest` seeder, or to a v1 round from the `paneltest` v1-round helper. A `package panel` test cannot import `paneltest` (import cycle); it calls `Submit` directly or writes a v1 `round` event in the package.
6. Append `## Amendment (Spec 131)` to ADR-0037 per R13.3, obeying the ADR pin constraint. Add `TestADRAmendments_Spec131ADR0037`.

**Verification**
- [ ] Owned tests (presence rule): `go test ./internal/panel/... ./cmd/mindspec/... ./internal/validate/... -run '^(TestVerdictV2_FindingProjection|TestGateV2_SubmittedOnly|TestTally_DirectCallerIgnoresUnsubmitted|TestGateV2_DigestBinding|TestGateV2_DraftAndVoidMissing|TestGateV2_ReplacementAndDischarge|TestGateV2_DischargePositional|TestGate_VerdictSchemaPinned|TestGate_PreV2RoundKeepsV1Rules|TestCreate_AlwaysV2|TestRoster_Family|TestCreate_FirstV2RoundFamily|TestPanelSubmit_Refusals|TestPanelSubmit_ABA|TestPanelSubmit_NoViaFlag|TestPanelSubmit_LandedBindingRefused|TestPanelSubmit_SupersededAttemptRefused|TestPanelSubmit_SameAttemptDraftThenFinal|TestPanelSubmit_SameAttemptRerunRepairs|TestPanelSubmit_EndedAttemptRefused|TestPanelSubmit_LongFormSuperseded|TestPanelVerify_AnnotationsFromRecords|TestPanelVerify_SlotStatus|TestPanelVerify_DispatchedUnsubmitted|TestPanelVerify_HarnessTimeout|TestPanelVerify_Replaced|TestPanelVerify_ExitCodes|TestPanelVerify_V1Round|TestPanelVerify_JSONRoster|TestPanelVerify_Termsafe|TestSlotRoot_SubmitFromFilePath|TestSlotRoot_SubmitFromClone|TestSlotRoot_Refusals|TestSlotRoot_Rules|TestSlotRoot_DispositionQuery|TestADRAmendments_Spec131ADR0037)$' -v -count=1` prints a `--- PASS:` line for each of the 36 names. `TestTally_DirectCallerIgnoresUnsubmitted` calls `panel.Tally` directly, not through the gate, on a v2 round with a hand-written APPROVE file for every slot and no submissions: `Complete()` is false, `PassRule(true)` is false and `Unsubmitted` lists every file. `TestSlotRoot_DispositionQuery`: from a slot `tree/sub` cwd, the default and `--spec` queries read the main root's ledgers, and `--dir` reads the given directory
- [ ] The five attempt-binding tests seed `dispatched` events, `attempt-<k>/` directories and a `launch_failed` outcome with the Bead 3 `paneltest` seeder; no test in this bead needs `panel checkout`
- [ ] Stay green: `go test ./internal/complete/...`, `go test ./cmd/mindspec/... -run '^(TestPanel|TestRebind|TestDefer)'`, `go test ./internal/complete/... -run '^(TestReadinessIsolation_|TestPanelGate_AdHocPanelNeverGates)'`, `go test ./internal/panel/...` and `go test ./internal/validate/... -run '^TestADRAmendments_Spec130_'`
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-3: each R2.3 refusal leaves the panel directory and ledger byte-identical; the v1-round refusal names `--round`; A, B, A gives `seq` 1, 2, 3 and the gate counts A; a CLI submit on a process slot refuses; `--via` is an unknown flag; a wrong `model` refuses; the landed-binding refusal names `--round`; the local-file annotation items
- [ ] AC-2: `TestVerdictV2_FindingProjection` — findings `ccr[0]` and `ccr[1]` carry the sha256 of `a` and `b`; `panel refute --finding ccr[1]` discharges `ccr[1]` only
- [ ] AC-4 in full; the R3.6 pin is owned here (`TestGate_PreV2RoundKeepsV1Rules`, `TestGate_VerdictSchemaPinned`, `TestCreate_AlwaysV2`)
- [ ] AC-6: `create` writes `family`; the first-v2-round carry and its `--reviewers` refusal; earlier round records stay byte-identical
- [ ] AC-8: the attempt-binding items (superseded attempt, same-attempt draft then final, same-attempt rerun repair, ended attempt, long-form superseded) on seeded ledgers
- [ ] AC-9 in full
- [ ] AC-11: every item that a seeded ledger can reach (slot status per R8.1, `dispatched` with a hand-written file, harness timeout, `replaced` annotations and the tally line, exit codes, v1 round, `--json`, `termsafe`, verify writes nothing)
- [ ] AC-16: ADR-0037 carries the R13.3 amendment

**Depends on**
Bead 1 (slot family, effective runner and model, `IsolationRootFor`), Bead 2 (`cmd/mindspec/rules.go`), Bead 3 (verdict, ledger kinds, `panel.Submit`, `LatestSubmissions`, `findPanelRoot`, `landedBindingFor`, `paneltest`)

## Bead 5: Slot checkout, dispatch and cleanup

Scope: R6.2–R6.8.

**Steps**
1. Add executor calls for slot clones in `executor.go`, `mindspec_executor.go` and `mock.go`: `clone --shared --no-checkout`, detached checkout and `remote remove origin`. Use the existing `CheckIgnore` for the ignore refusal; do not add a second one. Each runs with `-c core.fsmonitor=false -c core.hooksPath=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, and the `git rev-parse --local-env-vars` names removed (R6.4).
2. Add `mindspec panel checkout <slug> (--slot S | --all)` per spec "R6 — Slot clones and scratch" R6.2–R6.4. It finds the root with `findPanelRoot`. Add the symlink and ignore refusals, `slot.json` with the main root, reuse at the reviewed SHA, and the `lstat`, `commondir`, `core.worktree`, `core.bare` and `alternates` refusals with the `--prune --slot` recovery. Plain checkout writes no ledger record.
3. Add `--dispatch [--replace]` (R6.5): exclusive `mkdir` of `attempt-<k>/`, `launch_failed` for earlier open `dispatched` attempts, then one `dispatched` event. Refuse a process slot, a final non-`VOID` slot (from `panel.LatestSubmissions(dir, N)`) without `--replace`, and a landed binding (`landedBindingFor`) before any append. Add `--fixer` (R6.6).
4. Add `--prune [--slot S]`, and `cleanupPriorRound` for `create --round N+1` (one call in `panel.go` before `panel.Create`) and `abandon` (R6.7): slot locks in sorted order before the ledger lock, the orphan check hook (a no-op until Bead 7 fills it), no symlink follow, and no slot-lock file deletion on Unix.
5. Make the registration lookup (`panel.Scan` in `panel.go`; it keeps its `Scan(roots ...string)` signature and wraps a new `ScanSkipping(skip []string, roots ...string)` ; only `cmd/mindspec/panel.go` (`panel tally`) switches to it. The other `panel.Scan` callers (`cmd/mindspec/config.go`, `internal/approve/impl.go`, `internal/complete/panel_ledger.go`, `internal/instruct/panelstate.go`) keep `Scan`, because `Scan` globs only `<root>/review/*/panel.json` and cannot reach a slot directory), the `doctor` ownership and links walks, and the `migrate` walks skip the resolved isolation root (R6.8). Make `mindspec init` add a custom isolation root to `.gitignore`.
6. Add `TestPanelCheckout_GitOnlyViaExecutor` in `cmd/mindspec/git_boundary_test.go`. It parses `panel_checkout.go`, `panel_submit.go`, `panel_prompt.go`, `panel_launch.go`, `panel_recover.go` (each when present) and `internal/workspace/slotroot.go` with `go/parser`. It fails on an import of `os/exec` or `internal/gitutil`, and on a string literal `"git"`. A negative subtest runs the same check on a temp file that imports `os/exec` and asserts a hit.

**Verification**
- [ ] Owned tests (presence rule): `go test ./cmd/mindspec/... ./internal/bootstrap/... ./internal/panel/... ./internal/doctor/... -run '^(TestPanelCheckout_Creates|TestPanelCheckout_GitStaysInSlot|TestPanelCheckout_NoOrigin|TestPanelCheckout_Refusals|TestPanelCheckout_Prune|TestPanelCheckout_HarnessDispatched|TestPanelCheckout_DispatchFlag|TestPanelCheckout_PlainWritesNoLedger|TestPanelCheckout_GitRedirectRefused|TestPanelCheckout_TreeLstat|TestPanelCheckout_NoFsmonitor|TestPanelCheckout_GitEnvScrubbed|TestPanelCheckout_PruneKeepsLockFile|TestPanelCheckout_GitOnlyViaExecutor|TestPanelCleanup_HeldLockNoChange|TestBootstrap_CustomRootIgnored|TestWalks_SkipIsolationRoot)$' -v -count=1` prints a `--- PASS:` line for each of the 17 names
- [ ] `TestPanelCheckout_PlainWritesNoLedger` seeds a final submission with the Bead 3 `paneltest` seeder; after a plain checkout the ledger is byte-identical and the latest `verdict` record of the slot from `panel.LatestSubmissions(dir, N)` still has status `final`. It does not use the Bead 4 slot status
- [ ] `TestPanelCleanup_HeldLockNoChange`: the test process holds one slot lock through the Bead 3 lock API; `checkout --prune`, `create --round 2` and `abandon` each refuse, name the held slot, and leave every file under the panel and isolation directories and the ledger byte-identical
- [ ] `TestPanelCheckout_PruneKeepsLockFile` (Unix): after `--prune --slot S`, the slot lock file still exists at its path
- [ ] Stay green: `go test ./internal/executor/...`, `go test ./internal/doctor/... -run '^(TestOwnership|TestLinks)'` and `go test ./cmd/mindspec/... -run '^(TestMigrate|TestPanelAbandon|TestPanelCreate)'`
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-8: `checkout --all` on 4 slots, idempotent rerun, no `origin`; `git status` and `git stash` in a slot tree leave the main git directory unchanged; the other-SHA, git-redirect, `lstat`, fsmonitor and `GIT_DIR` items; `--prune --slot` keeps the lock file; `create --round 2` and `abandon` cleanup, and their refusal on a held lock; the `--dispatch` event and refusals; plain checkout after a final writes nothing; the symlinked and non-ignored root refusals; the custom-root `init` item; the walk items
- [ ] AC-17 support: new cmd-layer panel files run git only through `internal/executor`

**Depends on**
Bead 1 (`panel.isolation_root`, `IsolationRootFor`, runner kind), Bead 3 (slot layout, slot lock API, `launch` record kind, `LatestSubmissions`, `findPanelRoot`, `landedBindingFor`)

## Bead 6: BRIEF regions and slot prompt

Scope: R9.1–R9.4. R9.5 is adapter behaviour; Bead 9 writes it into the skills.

**Steps**
1. Add the four BRIEF regions per spec "R9 — BRIEF and slot prompt" R9.1: acceptance (from Bead 1 config), quoted spec ACs at the reviewed SHA (fenced, `termsafe`), conduct (reviewer-scope rules for the gate, from Bead 2, passed in by the cmd layer), and the verdict contract. Add one `BriefRegions` field to `CreateInput` and one render call in `renderCreateBriefHeader` (it gains a regions parameter; the direct call in `create_test.go` passes empty regions); fill the field in `cmd/mindspec/panel.go`. Stay inside the seams in `## Lane resolutions`.
2. Add the R9.3 check: binary-authored text names no documentation path and no internal reference; every repository path it names exists at the reviewed SHA (the existing executor `FileAtRefOrAbsent`, `present` true), else `create` refuses. Paths in the quotation are not checked.
3. Add `mindspec panel prompt <slug> --slot S` (R9.4). It finds the root with `findPanelRoot`. It prints the BRIEF, slot id, effective family, model, lens and focus, the absolute slot directory and tree, the output instruction for the effective runner, and the scratch instruction. It writes nothing. For a harness runner, the output instruction names the attempt of the latest `dispatched` event, as R9.4 states. This bead adds no refusal that R9.4 does not state.
4. Add the process output instruction as a render function that takes the nonce. Bead 7 calls it with a new nonce for `attempt-<k>/prompt.md`.
5. Add round-2 prior items: the slot's latest-submission items of round N-1 (`panel.LatestSubmissions(dir, N-1)`) on a v2 prior round; the parsed `<slot>-round-<N-1>.json` items on a v1 prior round; else `no prior-round items`.
6. Extend the ADR-0045 `## Amendment (Spec 131)` section with the step-3 template items (R13.1), obeying the ADR pin constraint. Add `TestADRAmendments_Spec131ADR0045Templates`.

**Verification**
- [ ] Owned tests (presence rule): `go test ./internal/panel/... ./cmd/mindspec/... ./internal/validate/... -run '^(TestBrief_QuotedACs|TestBrief_Regions|TestBrief_PathCheck|TestBrief_AcceptanceRegion|TestPanelPrompt_Render|TestPanelPrompt_PriorRound|TestPanelPrompt_PriorRoundV1|TestADRAmendments_Spec131ADR0045Templates)$' -v -count=1` prints a `--- PASS:` line for each of the 8 names
- [ ] `TestPanelPrompt_PriorRound` uses one ledger with round-1 items X and round-2 items Y. After a round-2 draft and a round-2 final submission, the round-2 prompt still renders X and does not render Y
- [ ] Stay green: `go test ./internal/panel/... -run '^(TestCreate_|TestPanelLeafImports_StdlibPlusTermsafeOnly)'`, `go test ./internal/validate/... -run '^(TestADRAmendments_Spec131ADR0045Rules|TestADRAmendments_Spec130_)'`
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-12 in full
- [ ] AC-7: the BRIEF of each gate holds its R5.5 question (`TestBrief_AcceptanceRegion`)
- [ ] AC-16: ADR-0045 carries the R13.1 template items

**Depends on**
Bead 1 (lenses and acceptance data, `IsolationRootFor`), Bead 2 (rule table), Bead 3 (verdict contract, `LatestSubmissions`, slot layout, `findPanelRoot`)

## Bead 7: Launch, supervisor, recover and the panel workflow

Scope: R7, R12.5, R12.6, the adapter-redispatch item of AC-15, and the AC-8, AC-10 and AC-11 items that need a running launcher.

**Steps**
1. Add package `internal/panellaunch` with the supervisor internal mode in `supervisor_unix.go` per spec "R7 — `panel launch` and `panel recover`" R7.3: new process group, close-on-exec `run.lock`, `pgid` by rename, start pipe, no shell, scrubbed `MINDSPEC_` and git variables, child stdin per `prompt`, timeout, `exit.json`, SIGKILL to its group on every exit path.
2. Add `mindspec panel launch <slug> <slot> [--replace]` (R7.1, R7.2). It finds the root with `findPanelRoot`. Add the refusals (incl. the landed round, through `landedBindingFor`), slot lock then ledger lock, checkout through Bead 5, the orphan check (fill the Bead 5 hook in `panel_checkout.go`), attempt `k`, prompt with nonce through Bead 6, the `started` event, then `start`.
3. Add outcomes, retry and substitution (R7.4–R7.7): nonce block or `verdict.json`, `quota_wall` from the stderr tail and configured literals only, `launch_failed`, `malformed` with one retry, `on_quota` substitution with the Bead 1 substitute model, the harness handoff print, exit 0 only for a submitted final.
4. Add `mindspec panel recover <slug> <slot> --reason R` (R7.8) with the digest and nonce checks; it finds the root with `findPanelRoot`. Add the Windows refusal `process runners need a Unix host` in `launch_windows.go` (R7.9).
5. Rewrite both `ms-panel.js` copies per R12.5: exact `ALLOWED_CLI`, fixed-flag `buildCommand`, roster from `panel verify --json`, the R12.1 table, a harness reviewer step that submits through `panel submit`, and one redispatch (R12.1 harness redispatch) when a harness agent returns with no submission and no file. Remove `VALID_MIX_FAMILIES`, the substitution flag and the parse-retry ladder. Delete `plugins/mindspec/scripts/codex_verdict_extract.sh` (R12.6). In `internal/setup/surface_refs_test.go`, remove the `walk("file:", "plugins/mindspec/scripts")` call and the `"file:plugins/mindspec/scripts/codex_verdict_extract.sh"` row of `TestSurfaceRefs_ScansEverySurfaceKind`, because the directory is then empty. Snapshot the v0.17.0 workflow before the rewrite lands: `git show v0.17.0:plugins/mindspec/workflows/ms-panel.js > internal/setup/historical_workflows/ms-panel.pre131.js` (sha256 `fa8d6e59a9b0893aa4f11f2685316177c1ffd6ddba84e2e49a59d768b54b6bec`). Add `TestUpgradeRefreshWorkflow_V0170Refreshes` in `upgrade_refresh_workflow_test.go`, in the form of `TestUpgradeRefreshWorkflow_DigestOracle` and `_UnmodifiedOldBodyRefreshes`: it pins that digest, and an installed copy with that body refreshes to the new body.
6. Add node-run workflow fixtures in `plugins/mindspec/workflow_test.go` with a stub harness API. `TestMsPanelWorkflow_HarnessSubmits`: one harness slot with a fake agent ends `final` with no Write on a verdict path. `TestPanelRun_SilentHarnessRedispatchOnce` drives `ms-panel.js` under `node` against a temp panel; the stub agent returns with no submission and no file; the test asserts exactly one more `dispatched` event in the ledger, then a reported slot, and goes red on zero or two redispatches.
7. Add `TestSlotRoot_EveryPanelVerb`: from a real slot `tree/sub` cwd, it walks `panelCmd.Commands()` and asserts that each verb resolves the main root. It drives `panel disposition query` with its default glob. It holds an explicit list of root-free verbs (`panel schema`, `panel disposition validate`, and the parent groups `panel` and `panel disposition`). It fails on any other verb that it does not drive, so a new verb cannot skip root discovery. Add `TestPanelLaunch_NoGitCalls` in `internal/panellaunch/boundary_test.go`: it parses the package's non-test files and fails on an import of `internal/gitutil` and on a string literal `"git"` passed to `exec.Command` or `exec.CommandContext`.

**Verification**
- [ ] Owned tests (presence rule): `go test ./internal/panellaunch/... ./cmd/mindspec/... ./plugins/mindspec/... ./internal/setup/... -run '^(TestPanelLaunch_VoidRetry|TestPanelLaunch_KillsOrphan|TestPanelLaunch_SupervisorTimeoutAfterLaunchDeath|TestPanelLaunch_FaultBeforePgid|TestPanelLaunch_LaunchDiesBeforeStart|TestPanelLaunch_LedgerPgidOnly|TestPanelLaunch_NoEventNoPgidRefuses|TestPanelLaunch_OrphanOutcomeDigests|TestPanelLaunch_VerifyLoopNoRefusal|TestPanelLaunch_SupervisorTermKillsGroup|TestPanelLaunch_ChildStdinNull|TestPanelLaunch_SetsidDescendant|TestPanelLaunch_NoSignalStalePgid|TestPanelLaunch_LocksNotInherited|TestPanelLaunch_RecoverAfterFailedReplacement|TestPanelLaunch_IdenticalFinalReplacement|TestPanelDispatch_IdenticalFinalReplacement|TestPanelLaunch_LandedRoundRefused|TestPanelLaunch_QuotaHandoffE2E|TestPanelLaunch_AdapterE2E|TestPanelLaunch_Outcomes|TestPanelLaunch_Channels|TestPanelLaunch_Retry|TestPanelLaunch_Substitution|TestPanelLaunch_DraftAndVoid|TestPanelLaunch_IdenticalResubmit|TestPanelLaunch_Env|TestPanelLaunch_Refusals|TestPanelLaunch_NoGitCalls|TestPanelRecover_Stdout|TestPanelRecover_File|TestLedger_NoScratchPaths|TestPanelVerify_KilledLaunchLiveSupervisor|TestPanelVerify_KilledLaunchBothLocksFree|TestPanelVerify_ReplacementInFlight|TestSlotLock_StableAcrossPrune|TestPanelCleanup_RefusesHeldLock|TestSlotRoot_EveryPanelVerb|TestMsPanelWorkflow_AllowedCLIExactSet|TestMsPanelWorkflow_HarnessSubmits|TestPanelRun_SilentHarnessRedispatchOnce|TestUpgradeRefreshWorkflow_V0170Refreshes)$' -v -count=1` prints a `--- PASS:` line for each of the 42 names, with `node` on `PATH`
- [ ] `TestPanelLaunch_WindowsRefuses` exists: `grep -l 'func TestPanelLaunch_WindowsRefuses(' internal/panellaunch/*_test.go` names one file, and that file has `//go:build windows`; `GOOS=windows go vet ./...` passes
- [ ] `test ! -e plugins/mindspec/scripts/codex_verdict_extract.sh`, `grep -n 'plugins/mindspec/scripts' internal/setup/surface_refs_test.go` prints nothing, and `grep -n -i -E '/tmp|codex|copilot|gemini' plugins/mindspec/workflows/ms-panel.js .claude/workflows/ms-panel.js` prints nothing
- [ ] Stay green: `go test ./internal/setup/... -run '^(TestClaudeSetup_InstallsWorkflowClaudeTargetOnly|TestSurfaceRefs_|TestProductSurface_NoInternalRefs|TestUpgradeRefreshWorkflow_)'` and `go test ./cmd/mindspec/... -run '^TestPanelCheckout_GitOnlyViaExecutor$'`
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-10 in full except the Windows execution (named gap): the adapter end-to-end run, the supervisor fault cases, the lock tests, the identical-final replacement cases, the landed-round refusal and the quota handoff through the workflow fixture; the Windows refusal compiles
- [ ] AC-8: the paused-launcher prune race (`TestSlotLock_StableAcrossPrune`) and the launcher-held and fake-runner cleanup refusal (`TestPanelCleanup_RefusesHeldLock`)
- [ ] AC-9 support: every panel verb, incl. `checkout`, `prompt`, `launch` and `recover`, resolves the main root from a slot tree
- [ ] AC-11: the killed-launch items and the replacement-in-flight item
- [ ] AC-15: `ALLOWED_CLI` equals the R12.5 set; both `ms-panel.js` copies pass the family and `/tmp` grep; the harness workflow fixture ends `final` with no Write on a verdict path; exactly one redispatch for a silent harness agent; the extract script is absent; an unmodified v0.17.0 workflow install refreshes

**Depends on**
Bead 1 (runner argv, `on_quota`, substitute model), Bead 3 (record kinds, `panel.Submit`, slot lock, `findPanelRoot`, `landedBindingFor`), Bead 4 (verify status, gate and submit for outcome tests), Bead 5 (checkout and the orphan-check hook), Bead 6 (process prompt render with nonce, `panel prompt`)

## Bead 8: Doctor checks

Scope: R11.1–R11.8, and the doctor legs of the R4.7 Error and Warn.

**Steps**
1. Add a launcher check per spec "R11 — Doctor checks" R11.1: for each process runner that a resolved roster of any gate uses, `argv[0]` is on `PATH`, else an Error naming the family.
2. Add the probe rule (R11.2): run `probe` only for a runner from an untracked `config.local.yaml` or under `doctor --probe`; 10-second limit; one Info line for a probe not run. Add the `--probe` flag in `cmd/mindspec/doctor.go` and a `Probe` field to `Options` in `internal/doctor/doctor.go`, read by `RunWithOptions`. Use the Bead 1 executor `LsFiles` call for the tracked check.
3. Add R11.3–R11.5: the tool suggestion as Info, the unrunnable-family Error, the model-only Warn, the load error as one Error with its recovery (one config load shared by `checkModels` and `checkSourceGlobs` in `internal/doctor/config.go`, so one bad file gives one Error; update their callers in `internal/doctor/models_commands_test.go` and `internal/doctor/config_test.go` to the shared load), and the tracked-local-file and non-ignored-root Warns.
4. Add R11.6 and R11.7: staged-rule markers in installed skills (no id, unknown id from Bead 2, past `remove-by`), the Info list of enforced rules, and slot directories of a missing, abandoned or later-round panel (recovery `mindspec panel checkout <slug> --prune`), found through the Bead 3 layout helpers.
5. Make `doctor --ci` skip R11.1, R11.2, the R11.3 suggestion and R11.7 (R11.8).

**Verification**
- [ ] Owned tests (presence rule): `go test ./internal/doctor/... ./cmd/mindspec/... -run '^(TestDoctor_Launchers|TestDoctor_ProbeNotRunFromTrackedConfig|TestDoctor_TrackedLocalProbe|TestDoctor_LoadErrorReported|TestDoctor_ConfigWarnings|TestDoctor_StagedMarkers|TestDoctor_StaleCheckouts|TestDoctor_CISkips|TestDoctor_UnrunnableFamily|TestDoctor_ModelOnlyWarn)$' -v -count=1` prints a `--- PASS:` line for each of the 10 names. `TestDoctor_UnrunnableFamily` (one Error naming the family) and `TestDoctor_ModelOnlyWarn` (one Warn) are top-level functions in `internal/doctor/panel_runners_test.go`
- [ ] Stay green: `go test ./internal/doctor/...` and `go test ./cmd/mindspec/... -run '^TestDoctor'`
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-14 in full
- [ ] AC-6: `doctor` gives the Error for a family with no runner and one Warn for a model-only entry

**Depends on**
Bead 1 (runners, local file, probe data, load errors, `LsFiles`), Bead 2 (rule ids for markers), Bead 3 (slot layout for the stale-checkout walk)

## Bead 9: Harness adapter skills, cross tests and documents

Scope: R10.6 (`ms-bead-fix` prompt), R12.1–R12.4, R12.7, R13.6, R13.7, the Windows lifecycle item of AC-10, and one end-to-end dispatch-then-submit leg.

**Steps**
1. Before any skill edit, snapshot each R12.3 skill from the v0.17.0 tag, never from the spec branch: `git show v0.17.0:plugins/mindspec/skills/<name>/SKILL.md > internal/setup/historical_skills/<name>.pre131.md` for `ms-panel-run`, `ms-panel-tally`, `ms-bead-fix`, `ms-bead-cycle`, `ms-spec-final-review` and `ms-bead-impl`. (The `ms-bead-impl` body on the spec branch already has the Bead 2 marker; that body is not a snapshot.)
2. Add `TestSetup_RefreshesSpec130Skills` in `internal/setup/upgrade_refresh_spec131_test.go`, in the pinned-digest form of `upgrade_refresh_presurface_test.go`: it pins the sha256 of each v0.17.0 body (computed from `git show v0.17.0:...` when the bead runs, written as literals), checks each snapshot file matches its pin, and checks that `mindspec setup claude` over an install with those bodies writes the new bodies, and keeps a user-modified copy.
3. Rewrite both copies of `ms-panel-run` (80 lines or fewer) per spec "R12 — Skill and workflow slimming" R12.1, with one status-table row per non-final status, the harness redispatch row and the R9.5 model pass-through. Rewrite both copies of `ms-panel-tally` (80 lines or fewer) per the R12.2 section table.
4. Edit both copies of `ms-bead-fix`, `ms-spec-final-review`, `ms-bead-cycle` and `ms-bead-impl` per R12.3 and R12.4: no `/tmp`, no tool family name, rule ids for conduct, `panel checkout --slot` and `--fixer`, a family-neutral `ms-bead-cycle` table. `ms-bead-fix` puts `mindspec rules --scope fixer` output in the fixer prompt (R10.6). Update the fragment rows in `internal/setup/review_discipline_test.go`. The other pins that the rewrite reaches, each checked by grep: the rewrite keeps the `adhoc_panel_skill_test.go` invocation `mindspec panel create <panel-slug> --gate adhoc --target <ref>` and location `<repo>/.mindspec/reviews/<panel-slug>/`; keeps the `--bead <bead-id>` stale-verdict line and its copy parity (`panel_tally_skill_bead_test.go`); keeps every `want` and `forbidden` fragment and the reference patterns of `TestPanelSkills_NoHandEditInstructions` (`panel_skills_test.go`), except the `ms-panel-run` row ``roster ids `R1..Rn` `` / `as verdict file prefixes`, which is retired (the binary names verdict files) and replaced by a want of `mindspec panel submit`; keeps the skill names and approve order of `skills_test.go`; keeps both `ms-bead-cycle` fragments of `internal/lint/destructive_guidance_test.go` (``Never merge a bead branch with raw `git merge bead/<id>` `` and `stopped between bd-close and the actual git merge`); keeps the `ms-bead-cycle` fragments of `TestMsBeadCycleSkill_NotReadyRoutingShipped` (`No panel round is consumed`, ``Never routed to `/ms-bead-fix` ``) and `TestMsBeadCycleSkill_BoundedLoopContractShipped` (`--allow-not-ready`, `never interchangeable`) in `internal/setup/readiness_skill_pin_test.go`; keeps the plugin and `.claude` byte parity of `ms-bead-cycle`, the `--allow-not-ready=<ids> --evidence <path>` form and the absence of a bare ``--allow-not-ready` `` in `TestReadyCheck_CycleSkillCopiesMatch` (`cmd/mindspec/bead_ready_test.go`); keeps `internal/setup/upgrade_refresh_pre126_test.go` with no edit, because its `TestUpgradeRefreshPre126_` oracles read frozen snapshots and compare them to the current canonical body. No pin of these three files is retired.
5. Add `TestSlotLock_WindowsLifecycle` in `cmd/mindspec/slot_lock_windows_test.go` (`//go:build windows`) per the AC-10 Windows item: 20 cycles of `checkout --dispatch`, verify probes, `submit`, a redispatch and `--prune`. Add `TestPanelSubmit_DispatchThenSubmitE2E` (Unix): `checkout --dispatch` on a real slot, then a short-form submit from `attempt-1/`, then `--dispatch --replace` and a refused submit from `attempt-1/`. Add `TestPanelDispatch_StaleFileRefused` (Unix) per the AC-8 leg: after `--dispatch --replace`, a submit with no new `attempt-<k>/verdict.json` refuses, and `panel verify` shows the slot `running`. It needs the Bead 4 submit, attempt binding and slot status, and the Bead 5 dispatch.
6. Update `interfaces.md` § Panel Artifact Schema and `CHANGELOG.md` per R13.6, including the before-and-after example. Update `review-panels.md`, the panel paragraphs of `README.md`, and `plugins/mindspec/README.md` per R13.7.

**Verification**
- [ ] `wc -l plugins/mindspec/skills/ms-panel-run/SKILL.md .claude/skills/ms-panel-run/SKILL.md plugins/mindspec/skills/ms-panel-tally/SKILL.md .claude/skills/ms-panel-tally/SKILL.md` shows 80 or less for each
- [ ] For each of the six names, `git show v0.17.0:plugins/mindspec/skills/<name>/SKILL.md | cmp - internal/setup/historical_skills/<name>.pre131.md` exits 0
- [ ] Owned tests (presence rule): `go test ./internal/setup/... ./plugins/mindspec/... ./cmd/mindspec/... ./internal/validate/... -run '^(TestPanelSkills_AdapterShape|TestPanelRun_StatusTable|TestSkills_NoFamilyNoTmp|TestBeadFix_FixerRules|TestSetup_RefreshesSpec130Skills|TestPanelSubmit_DispatchThenSubmitE2E|TestPanelDispatch_StaleFileRefused|TestUserDocs_NoRetiredPanelClaims)$' -v -count=1` prints a `--- PASS:` line for each of the 8 names
- [ ] All ADR pins: `go test ./internal/validate/... -run '^(TestADRAmendments_Spec131ADR0040|TestADRAmendments_Spec131ADR0044|TestADRAmendments_Spec131ADR0045Rules|TestADRAmendments_Spec131ADR0045Templates|TestADRAmendments_Spec131ADR0043|TestADRAmendments_Spec131ADR0037)$' -v -count=1` prints a `--- PASS:` line for each of the 6 names
- [ ] `TestSlotLock_WindowsLifecycle` exists in a `//go:build windows` file (`grep -l 'func TestSlotLock_WindowsLifecycle(' cmd/mindspec/*_test.go`), and `GOOS=windows go vet ./...` passes
- [ ] Stay green: `go test ./internal/setup/... -run '^(TestReviewDiscipline_|TestSurfaceRefs_|TestProductSurface_NoInternalRefs|TestMsBeadImplSkill_|TestLifecycleGateStep_|TestUpgradeRefreshPresurface_|TestMsPanelRunSkill_|TestMsPanelTallySkill_|TestPanelSkills_|TestSkillInventory_|TestSkillSurface_|TestInstallSkills_|TestMsBeadCycleSkill_NotReadyRoutingShipped|TestMsBeadCycleSkill_BoundedLoopContractShipped|TestUpgradeRefreshPre126_)'`, `go test ./cmd/mindspec/... -run '^TestReadyCheck_CycleSkillCopiesMatch$'` and `go test ./internal/lint/... -run '^TestDestructiveGuidanceSweep_'`
- [ ] `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- [ ] AC-15: line limits; the R12.3 grep is clean for every skill copy; one status row per non-final status; `mindspec setup claude` over an unmodified v0.17.0 install replaces every R12.3 skill
- [ ] AC-13: `ms-bead-fix` renders the fixer-scope rules into its prompt
- [ ] AC-8: one end-to-end dispatch-then-submit leg through `checkout --dispatch`; the stale-file refusal (`TestPanelDispatch_StaleFileRefused`)
- [ ] AC-10: the Windows harness-slot lock lifecycle test compiles (execution is the named gap)
- [ ] AC-16: `interfaces.md`, `CHANGELOG.md`, `review-panels.md` and both READMEs; all six ADR pin functions from Beads 1, 2, 3, 4 and 6 pass

**Depends on**
Bead 2 (rule ids and `mindspec rules`), Bead 4 (submit CLI and `verify --json`), Bead 5 (`checkout --dispatch` and `--fixer`), Bead 6 (`panel prompt`)

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| AC-1 (R1) | Bead 3: `TestVerdictV2_Accepts`, `_Refusals`, `_TransportLayer`, `_SchemaMatchesValidator` |
| AC-2 (R1.6) | Bead 4: `TestVerdictV2_FindingProjection` (projection code from Bead 3) |
| AC-3 (R2) | Bead 3: `TestPanelSubmit_Writes`, `_Replace`, `_CrashRepair`, `_Race`, `_AttemptBindingUnderLock`, `TestDispositionAppend_RefusesV2Kinds`; Bead 4: `TestPanelSubmit_Refusals`, `_ABA`, `_NoViaFlag`, `_LandedBindingRefused`, `TestPanelVerify_AnnotationsFromRecords` |
| AC-4 (R3) | Bead 4: `TestGateV2_SubmittedOnly`, `_DigestBinding`, `_DraftAndVoidMissing`, `_ReplacementAndDischarge`, `_DischargePositional`, `TestGate_VerdictSchemaPinned`, `TestGate_PreV2RoundKeepsV1Rules`, `TestCreate_AlwaysV2` |
| AC-5 (R4.1–R4.5, R4.9) | Bead 1: `TestModelValue_Refusals`, `TestRunners_Validate`, `_TimeoutDefault`, `_ZeroConfigHarness`, `_MergeByFamily`, `TestSubstitution_Retired`, `TestNoFamilyLiterals` |
| AC-6 (R4.6–R4.13) | Bead 1: `TestSlotModelResolution`, `TestReviewer_ModelOnlyIsHarness`, `TestSubstituteModel`, `TestLocalConfig_Merge`, `_Refusals`, `TestConfigResolve_Provenance`, `_ToolSuggestion`, `TestRepoConfig_Resolves`, `TestCreate_RefusesUnrunnableFamily` and `TestReviewer_ModelOnlyWarn` (create and resolve legs, `cmd/mindspec`); Bead 4: `TestRoster_Family`, `TestCreate_FirstV2RoundFamily`; Bead 8: `TestDoctor_UnrunnableFamily`, `TestDoctor_ModelOnlyWarn` |
| AC-7 (R5) | Bead 1: `TestLenses_GateDefaults`, `_LensOnlyGateEntry`, `_UnknownRefused`; Bead 6: `TestBrief_AcceptanceRegion` |
| AC-8 (R6) | Bead 1: `TestIsolationRoot_Validate`, `TestBootstrap_ScratchIgnoredExisting`; Bead 4: `TestPanelSubmit_SupersededAttemptRefused`, `_SameAttemptDraftThenFinal`, `_SameAttemptRerunRepairs`, `_EndedAttemptRefused`, `_LongFormSuperseded`; Bead 5: the twelve `TestPanelCheckout_*` names of AC-8, `TestBootstrap_CustomRootIgnored`, `TestWalks_SkipIsolationRoot`, plus `TestPanelCleanup_HeldLockNoChange` and `TestPanelCheckout_PruneKeepsLockFile`; Bead 7: `TestSlotLock_StableAcrossPrune`, `TestPanelCleanup_RefusesHeldLock`; Bead 9: `TestPanelSubmit_DispatchThenSubmitE2E`, `TestPanelDispatch_StaleFileRefused` |
| AC-9 (R6.9) | Bead 3: `TestFindSlotRoot`; Bead 4: `TestSlotRoot_SubmitFromFilePath`, `_SubmitFromClone`, `_Refusals`, `_Rules`, `_DispositionQuery`; Bead 7: `TestSlotRoot_EveryPanelVerb` |
| AC-10 (R7) | Bead 3: `TestSlotLock_StaleLogic`, `TestSlotLock_WindowsUsesDecisionLogic`; Bead 7: the 28 `TestPanelLaunch_*` names of AC-10 (`_WindowsRefuses` compiled only), `TestPanelRecover_Stdout`, `_File`, `TestLedger_NoScratchPaths`, `TestPanelDispatch_IdenticalFinalReplacement`; Bead 9: `TestSlotLock_WindowsLifecycle` (compiled only; Windows execution is the named gap) |
| AC-11 (R8) | Bead 4: `TestTally_DirectCallerIgnoresUnsubmitted`, `TestPanelVerify_SlotStatus`, `_DispatchedUnsubmitted`, `_HarnessTimeout`, `_Replaced`, `_ExitCodes`, `_V1Round`, `_JSONRoster`, `_Termsafe`; Bead 7: `_KilledLaunchLiveSupervisor`, `_KilledLaunchBothLocksFree`, `_ReplacementInFlight` |
| AC-12 (R9) | Bead 6: `TestBrief_QuotedACs`, `_Regions`, `_PathCheck`, `TestPanelPrompt_Render`, `_PriorRound`, `_PriorRoundV1` |
| AC-13 (R10) | Bead 2: `TestRuleTable_MinimumSet`, `TestRulesCmd`, `TestInstruct_RulesSection`, `TestStagedMarker_Parse`, `TestProductSurface_NoInternalRefs`; Bead 9: `TestBeadFix_FixerRules` |
| AC-14 (R11) | Bead 8: the eight `TestDoctor_*` names of AC-14 |
| AC-15 (R12) | Bead 7: `TestMsPanelWorkflow_AllowedCLIExactSet`, `TestMsPanelWorkflow_HarnessSubmits`, `TestPanelRun_SilentHarnessRedispatchOnce`; Bead 9: `TestPanelSkills_AdapterShape`, `TestPanelRun_StatusTable`, `TestSkills_NoFamilyNoTmp`, `TestSetup_RefreshesSpec130Skills` |
| AC-16 (R13) | Beads 1, 2, 3, 4, 6: `TestADRAmendments_Spec131<ADR>` per ADR; Bead 9: `TestUserDocs_NoRetiredPanelClaims` and the six-name ADR pin run |
| AC-17 (method) | Every bead: `go build ./... && go test ./... -short`, incl. `TestPanelLeafImports_StdlibPlusTermsafeOnly` and `TestEnforcementHasNoGitLeaks`; Bead 5: `TestPanelCheckout_GitOnlyViaExecutor`; Bead 7: `TestPanelLaunch_NoGitCalls` |

provenance: default=model-protocol.md override=absent | drafting=default implementation=default orchestrator=default codex-effort=default codex-impl-effort=default spec=default plan=default bead=default final=default

Role: drafting = claude-opus-5-5.
