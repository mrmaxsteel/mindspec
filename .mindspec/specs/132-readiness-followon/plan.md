---
status: Draft
spec_id: 132-readiness-followon
version: "1"
adr_citations:
  - ADR-0041
  - ADR-0036
  - ADR-0025
  - ADR-0023
  - ADR-0030
  - ADR-0035
  - ADR-0019
  - ADR-0006
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths: [internal/complete/complete.go, internal/complete/materialize.go,
      internal/complete/landing_tree_test.go, internal/validate/divergence.go,
      internal/validate/adr_domain_resolve.go, internal/validate/docsync.go,
      internal/validate/ownership_ref_test.go, internal/executor/executor.go,
      internal/executor/mindspec_executor.go, internal/executor/mock.go,
      internal/gitutil/neteffect.go, internal/gitutil/neteffect_test.go,
      .mindspec/adr/ADR-0036-ownership-discovery.md,
      .mindspec/specs/129-readiness-tracker-honesty/spec.md,
      .mindspec/domains/workflow/architecture.md, .mindspec/domains/execution/interfaces.md,
      internal/validate/adr_spec132_ownership_test.go]
  - id: 2
    depends_on: []
    key_file_paths: [internal/approve/impl.go, internal/approve/impl_preflight_test.go,
      internal/lifecycle/gitquery.go, internal/gitutil/gitops.go, internal/gitutil/gitops_test.go,
      internal/executor/executor.go, internal/executor/mindspec_executor.go,
      internal/executor/mock.go, internal/executor/finalize_hasremote_test.go,
      internal/executor/finalize_fault_test.go, internal/executor/finalize_worktree_only_test.go,
      internal/executor/executor_test.go, internal/executor/neteffect_probe_test.go,
      internal/executor/finalize_scoping_test.go, internal/executor/layout_guard_test.go,
      internal/executor/finalize_orphan_test.go, internal/executor/merge_binding_test.go,
      internal/executor/merge_conflict_test.go, internal/lint/ratchet_argv_table_test.go,
      internal/guard/registries.go, .mindspec/adr/ADR-0041-gate-before-mutate.md,
      .mindspec/domains/workflow/architecture.md, .mindspec/domains/execution/interfaces.md,
      .mindspec/domains/execution/architecture.md, internal/validate/adr_spec132_preflight_test.go]
  - id: 3
    depends_on: [2]
    key_file_paths: [internal/approve/impl.go, internal/approve/impl_rerun_test.go,
      internal/executor/mindspec_executor.go, internal/executor/executor.go,
      internal/executor/mock.go, internal/executor/finalize_tip_test.go, internal/gitutil/gitops.go,
      internal/gitutil/gitops_test.go, internal/lint/ratchet_argv_table_test.go,
      internal/guard/registries.go, .mindspec/adr/ADR-0041-gate-before-mutate.md,
      .mindspec/domains/workflow/architecture.md, .mindspec/domains/workflow/runbook.md,
      .mindspec/domains/execution/architecture.md, .mindspec/domains/execution/interfaces.md,
      internal/validate/adr_spec132_finalize_test.go]
  - id: 4
    depends_on: []
    key_file_paths: [internal/lifecycle/merge_state.go, internal/lifecycle/merge_state_test.go,
      internal/lifecycle/stale_open.go, internal/lifecycle/scan.go, internal/doctor/doctor.go,
      internal/doctor/doctor_test.go, internal/doctor/lifecycle_integrity.go,
      internal/doctor/lifecycle_integrity_test.go, cmd/mindspec/doctor.go,
      cmd/mindspec/doctor_info_test.go, .mindspec/domains/workflow/architecture.md,
      .mindspec/domains/workflow/interfaces.md]
  - id: 5
    depends_on: [1, 4]
    key_file_paths: [internal/complete/complete.go, internal/complete/merge_only.go, internal/complete/closing.go,
      internal/complete/merge_only_test.go, internal/complete/closing_test.go,
      internal/complete/ownership_conflict_test.go, internal/complete/panel_advisory.go,
      cmd/mindspec/complete.go, internal/executor/executor.go,
      internal/executor/mindspec_executor.go, internal/executor/mock.go,
      internal/guard/registries.go, internal/lint/ratchet_argv_table_test.go,
      .mindspec/adr/ADR-0041-gate-before-mutate.md, .mindspec/domains/workflow/architecture.md,
      .mindspec/domains/workflow/runbook.md, .mindspec/domains/workflow/interfaces.md,
      .mindspec/domains/execution/interfaces.md, project-docs/user/guides/merge-safety.md,
      internal/validate/adr_spec132_mergeonly_test.go]
  - id: 6
    depends_on: [4]
    key_file_paths: [internal/config/config.go, internal/config/config_test.go,
      internal/config/feedback_remote_test.go, cmd/mindspec/config.go, cmd/mindspec/config_test.go,
      cmd/mindspec/ceremony_guard_test.go, cmd/mindspec/verb_output_test.go,
      internal/harness/sandbox.go, internal/harness/sandbox_test.go,
      internal/harness/lifecycle_scenario_test.go, internal/doctor/config.go,
      internal/doctor/config_test.go, internal/doctor/models_commands_test.go,
      internal/lint/no_merge_strategy_writers_test.go, .mindspec/config.yaml,
      .mindspec/core/GIT-WORKFLOW.md, .mindspec/domains/core/interfaces.md,
      .mindspec/domains/workflow/interfaces.md, .mindspec/domains/execution/architecture.md,
      .mindspec/adr/ADR-0006.md, .mindspec/adr/ADR-0019.md,
      internal/validate/adr_spec132_mergestrategy_test.go]
  - id: 7
    depends_on: []
    key_file_paths: [internal/lifecycle/landed.go, internal/lifecycle/landed_reviewed_tree_test.go,
      internal/complete/reviewed_tree_reconcile_test.go,
      .mindspec/adr/ADR-0041-gate-before-mutate.md, .mindspec/domains/workflow/architecture.md,
      internal/validate/adr_spec132_reviewedtree_test.go]
  - id: 8
    depends_on: []
    key_file_paths: [internal/lifecycle/reattest.go, internal/lifecycle/reattest_ancestor_test.go,
      internal/lifecycle/fallback.go, internal/lifecycle/fallback_test.go,
      internal/lifecycle/scan.go, internal/gitutil/neteffect.go, internal/gitutil/neteffect_test.go,
      cmd/mindspec/reattest.go, cmd/mindspec/reattest_test.go, internal/doctor/ancestor.go,
      internal/doctor/lifecycle_integrity.go, internal/doctor/ancestor_test.go,
      internal/guard/registries.go, .mindspec/domains/workflow/architecture.md,
      .mindspec/domains/workflow/runbook.md, .mindspec/domains/execution/interfaces.md,
      project-docs/user/guides/merge-safety.md]
  - id: 9
    depends_on: []
    key_file_paths: [internal/phase/derive.go, internal/phase/lineage.go, internal/phase/lineage_test.go,
      internal/phase/resolve_spec_test.go, internal/approve/spec.go, internal/approve/spec_test.go,
      internal/next/beads.go, internal/next/readyset_test.go,
      internal/validate/readiness/readiness.go, internal/validate/readiness/readiness_test.go,
      internal/doctor/lineage.go, internal/doctor/lineage_test.go, cmd/mindspec/spec.go,
      cmd/mindspec/spec_create_lettered_test.go, cmd/mindspec/lettered_spec_test.go,
      internal/guard/registries.go, .mindspec/domains/core/interfaces.md,
      .mindspec/domains/core/architecture.md, .mindspec/domains/workflow/interfaces.md]
---
# Plan: 132-readiness-followon

## ADR Fitness

- **ADR-0041 (gate before mutate)**: stays fit. Amended. Beads 2, 3, 5 and 7 each add one subsection under a new `## Amendment (Spec 132)` section. Bead 2: the early guard copies, the push probe and the one has-remote decision. Bead 3: the state line, `mindspec_finalize_tip` and stage resume. Bead 5: `--merge-only` is a commit phase with no close, and the closing call publishes the close. Bead 7: the reviewed-tree rule in amendment (ii) is a confirming datum and never vetoes. The pinned text of the Spec 125, Spec 129 and Spec 130 sections does not change (`TestADR0041_Spec129Amendment`, `cmd/mindspec/adr0041_amendment_test.go`).
- **ADR-0036 (ownership discovery)**: stays fit. Amended by Bead 1: `complete` reads ownership at the landing tree. One ref feeds all six readers.
- **ADR-0025 (JSONL as build artifact)**: applied, no edit. Bead 5 closing step 4 commits the export on the spec branch before step 6 deletes the branch and worktree. The export shows the close before evidence goes.
- **ADR-0023 (lineage is a parent-child dependency)**: applied, no edit. Bead 9 removes the bead-title fallback. Only a parent-child edge to an epic is lineage.
- **ADR-0030 (executor boundary)**: applied, no edit. `internal/complete` and `internal/approve` do not import `internal/gitutil`. The landing tree (Bead 1), the push probe (Bead 2) and the stage git work (Bead 3) go through Executor methods. The has-remote probe stays behind the gitutil seam through the `lifecycle` wrapper and `implHasRemoteFn`. `go test ./internal/lint/... -run TestEnforcementHasNoGitLeaks` stays green in every bead.
- **ADR-0035 (agent error contract)**: applied, no edit. Every new refusal ends with one or more `recovery:` lines. A bead that adds a recovery with a substituted operand registers it in `guard.OpaqueOperandRegistry` (`internal/guard/registries.go`).
- **ADR-0019, ADR-0006**: noted. Bead 6 adds a dated removal note at their `merge_strategy` guidance. The decisions stay.

No ADR needs a superseding ADR. Each pin file is `internal/validate/adr_spec132_<topic>_test.go`. Each pin function name starts with `TestDocs_Spec132Amendments`, so `go test ./internal/validate/... -run TestDocs_Spec132Amendments` runs all pins (AC-26). Each pin asserts only the text of its own bead.

## Operator Rulings and Lane Constraints

- **OQ-1..OQ-4 are fixed.** OQ-1: Bead 6 deletes `merge_strategy`; Bead 2 keeps the has-remote rule. OQ-2 (a): Beads 4 and 5 build `--merge-only` and the git-state severity rule. OQ-3: R8–R12 are out of scope; no bead implements them. OQ-4: Bead 2 makes a failed has-remote probe refuse before any mutation. It never selects local merge.
- **Parallel lane 131.** Both lanes touch `internal/config/**` and `internal/executor/**`. No bead rebases on or merges spec 131's unmerged branch. No bead changes panel config loading, panel verbs or panel skills. Beads 5 and 7 read `panel.ToolArtifactPaths` and the existing panel gate only. If spec 131 reaches `main` first, the after-all-beads merge of `origin/main` takes it in as a clause union.
- **R4 scheduling.** The R4 key removal is Bead 6. The lane starts Bead 6 after the other beads are in flight. This is a scheduling note, not an edge. Bead 6 has one real edge, to Bead 4: it uses the doctor Info status that Bead 4 adds.
- **R4 split.** R4.3, R4.4 and R4.4a (the one has-remote decision and its error-returning probe) are in Bead 2. The early guards (R2.1) and the push probe (R2.3) consume that one value. Thus Bead 2 needs no edge to Bead 6, and Bead 6 holds only the config key removal.
- **Docs ride with code.** Each bead updates the docs of every domain whose code it changes, in the same commit. No bead defers a doc to a later bead. No bead uses `--allow-doc-skew`.
- **Surface text.** Every bead that touches shipped surface text (CLI help, errors and output, `project-docs/user/guides/merge-safety.md`) keeps it free of internal records: no ADR number, spec number, R-id, bead ID, issue number or incident. Domain docs, ADRs and this plan may cite them.

## Testing Strategy

- **Dogfood boundary.** This spec changes `complete`, `impl approve`, `doctor` and reattest, which the lane itself uses. Every lifecycle command of this lane runs on the installed v0.17.0 binary (`~/.local/bin/mindspec`), never a spec-branch build. No bead installs a branch build. Tests exercise the branch code through `go test` only.
- **Unit tests** use fake `bd` on PATH and `executor.MockExecutor`. They set no real environment variables and start no model CLI.
- **Integration tests** use real temp git repos. `internal/complete` tests reuse `setupPanelGateRepo` (`internal/complete/panel_gate_e2e_test.go`) and `fakeBdOnPath` (`internal/complete/fault_injection_realgit_test.go`). Tests in other packages copy that pattern into a local helper. Remote tests use a `file://` bare remote with `GIT_CONFIG_GLOBAL=/dev/null`.
- **Fault seams.** Finalize stages use `finalizeStepHookFn` (`internal/executor/mindspec_executor.go`). The has-remote probe uses `implHasRemoteFn`. Bead 2 adds a push-probe seam. Bead 5 adds a seam per closing step. Bead 8 adds a read seam for each `fallbackSafety` read.
- **Argv ratchet.** Each new git exec site adds one row to `internal/lint/ratchet_argv_table_test.go` (`go test ./internal/lint/... -run TestArgvIDOperandGated`).
- **Recovery operands.** `go test ./internal/lint/... -run TestProductDiagnosticScan_UnprovableCommandOperandsAreGoverned` passes in every bead that adds a recovery line.
- **Doc-sync.** `checkInternalPackages` (`internal/validate/docsync.go`) maps each changed non-test source under `internal/` or `cmd/` to its domain through `OWNERSHIP.yaml`. It refuses unless a doc under `.mindspec/domains/<domain>/` changes in the same range. Domains: core (`internal/config`, `internal/phase`), execution (`internal/executor`, `internal/gitutil`, `internal/harness`), workflow (`cmd/`, `internal/complete`, `internal/approve`, `internal/lifecycle`, `internal/validate`, `internal/doctor`, `internal/next`, `internal/guard`). Each bead names its doc per domain. Bead 1 edits spec 129 `spec.md` together with ADR-0036, so `validateSpecArtifactSync` passes.
- **Union merge.** Parallel beads add rows to the argv ratchet table and the operand registry, subsections to the ADR-0041 `## Amendment (Spec 132)` section, entries to the Executor interface and `MockExecutor`, and sections to the workflow and execution domain docs, `runbook.md` and `merge-safety.md`. A merge conflict in these keeps every row, entry and section, and one `## Amendment (Spec 132)` heading. A bead that creates that heading checks first whether it exists at the spec tip.
- **Real verification.** A Verification line counts only when `go test <pkg> -list '<regex>'` prints each test that the bead names. `[no tests to run]` is a failure.
- **Every bead** ends with `gofmt -l .` (empty output), `go build ./...`, `go test ./internal/lint/... -run 'TestEnforcementHasNoGitLeaks|TestArgvIDOperandGated'` and `go test ./... -short -timeout 20m`.
- **Opt-in, separate:** the live-agent harness suite for the finalize scenarios. It is not a bead gate.

## Bead 1: Ownership at the landing tree

**Steps**
1. Add Executor method `LandingTree(workdir, base, specTip, beadRef string) (tree string, conflicted []string, err error)`. It runs `git merge-tree --write-tree --merge-base=<B>` through the net-effect primitive (`runMergeTreeWriteTree`, `internal/gitutil/neteffect.go`). It returns the output tree also when paths conflict. For the materialize leg, wrap tree T in a dangling commit (no ref update) when merge-tree needs a commit. Add it to `MockExecutor`.
2. In `runCommittedRangeGate` (`internal/complete/complete.go`), compute B as `git merge-base <spec tip> <bead HEAD>` and pass the landing tree as `ownerRef` to `ValidateDocsRange` and `CheckADRDivergence`. Do the same in the materialize leg (`internal/complete/materialize.go`) for `ValidateDocsSnapshot` and `CheckADRDivergenceSnapshot` (R1.1).
3. Make all six readers use that one ref: Impacted-Domains normalization and domain enumeration and path attribution (`internal/validate/divergence.go`), the ADR domain-resolving store (`adr_domain_resolve.go`), `checkInternalPackages` and `checkUnclaimedSource` (`docsync.go`). Remove any reader that reads a second ref (R1.2, R1.3).
4. When paths conflict and `OWNERSHIP.yaml` is not conflicted, read it from the merge-tree output tree. `MergeInto` then runs and keeps the conflict as today (R1.4, first half). Bead 5 owns the refusal for a conflicted `OWNERSHIP.yaml`.
5. On `--resolve-merge` re-entry, use `git write-tree` of the spec worktree index as the landing tree. Keep today's unresolved-conflict refusal while unmerged entries remain (R1.5). Bead 5 owns the hand-committed-merge case (R3.13).
6. Remove any ADR number or bead ID left in `materialize.go` refusal text (AC-21).
7. Docs: append `## Amendment (Spec 132)` to ADR-0036 (landing-tree ownership). Add dated status notes to spec 129 `spec.md` per R13.2. Update `.mindspec/domains/workflow/architecture.md` (ownership ref) and `.mindspec/domains/execution/interfaces.md` (`LandingTree`). Add `internal/validate/adr_spec132_ownership_test.go` (`TestDocs_Spec132Amendments_Ownership`): it pins the ADR-0036 section and each spec 129 note.

**Verification**
- [ ] `go test ./internal/complete/... -run 'TestCommittedRangeGate_SpecTipClaimOwns|TestMaterialize_SpecTipClaimOwns|TestCommittedRangeGate_RevokedAtSpecTipRefuses|TestCommittedRangeGate_SourceConflictPassesThenResolves|TestMaterializeText_NoHistoricalCitations'` passes
- [ ] `go test ./internal/validate/... -run 'TestCheckUnclaimedSource_LandingTree|TestDocs_Spec132Amendments_Ownership'` passes
- [ ] `go test ./internal/gitutil/... ./internal/executor/...` passes
- [ ] `mindspec validate spec 129-readiness-tracker-honesty` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-1 (R1.1–R1.3): `TestCommittedRangeGate_SpecTipClaimOwns`, `TestMaterialize_SpecTipClaimOwns`, `TestCommittedRangeGate_RevokedAtSpecTipRefuses`, `TestCheckUnclaimedSource_LandingTree` pass.
- [ ] AC-2 (R1.4 source-conflict leg, R1.5): `TestCommittedRangeGate_SourceConflictPassesThenResolves` passes.
- [ ] AC-21 (materialize part): `TestMaterializeText_NoHistoricalCitations` passes.
- [ ] AC-26 (R13.1 ADR-0036, R13.2): the pin passes and `mindspec validate spec 129-readiness-tracker-honesty` passes.

**Depends on**
None.

## Bead 2: `impl approve` preflight and one has-remote decision

**Steps**
1. Add `gitutil.RemoteConfigured() (bool, error)`. Expose it through a `lifecycle` wrapper and make `implHasRemoteFn` return `(bool, error)`. `gitutil.HasRemote()` and its other callers do not change (R4.4a).
2. In `ApproveImpl` (`internal/approve/impl.go`), decide has-remote one time in preflight, after the EnsureMigrated exemption and before the `--supersede-adr` placeholder write, the phase-reconcile write and MUTATION 1/3. A probe error refuses: `cannot tell whether a git remote is configured: <err>`, then `recovery: git remote -v`. An error never selects local merge (R4.3, R4.4a).
3. Add a `hasRemote bool` parameter to `FinalizeEpic` in the Executor interface, `MindspecExecutor` and `MockExecutor`. Remove the `gitutil.HasRemote()` call in `FinalizeEpic`. `result.MergeStrategy` follows the passed value. Update every `FinalizeEpic` call site, test files included. Update the two-probe comments on `HasRemote` (`internal/lifecycle/gitquery.go`) and on `implHasRemoteFn` and the preflight gate (R4.4).
4. Run an early copy of the merge-layout guard and of the work-destruction preflight for the decided path, before the phase-reconcile write, the placeholder write and MUTATION 1/3. Keep the executor guards in front of every `MergeInto` and `MergeBranch` call (R2.1, R2.2).
5. Add Executor method `PushProbe(branch string) error`. It runs `git push --dry-run --no-verify` to the remote and refspec of `PushBranch`, with `GIT_TERMINAL_PROMPT=0`. When has-remote is true, `ApproveImpl` calls it before MUTATION 1/3. Only a failed probe refuses. Add a credential-helper hint only for an authentication error on an HTTP(S) remote (R2.3–R2.5).
6. Docs: under ADR-0041 `## Amendment (Spec 132)`, add the subsection for early guards, the push probe and the one has-remote decision. Update `.mindspec/domains/workflow/architecture.md` (`impl approve` preflight order), `.mindspec/domains/execution/interfaces.md` (`FinalizeEpic` parameter, `PushProbe`, `RemoteConfigured`) and `.mindspec/domains/execution/architecture.md` (finalize no longer probes). Add `internal/validate/adr_spec132_preflight_test.go` (`TestDocs_Spec132Amendments_Preflight`).

**Verification**
- [ ] `go test ./internal/approve/... -run 'TestApproveImpl_EarlyLayoutGuardRefuses|TestApproveImpl_EarlyDestructionRefuses|TestApproveImpl_EarlyGuardBeforePhaseReconcile|TestApproveImpl_PushProbeAuthFailureNamesHelper|TestApproveImpl_PushProbeNonFastForwardNoHelperHint|TestApproveImpl_PushProbeFileRemoteNoHelperPasses|TestApproveImpl_OneHasRemoteDecision|TestApproveImpl_HasRemoteProbeErrorRefuses'` passes
- [ ] `go test ./internal/executor/... -run 'TestFinalizeEpic_ExecutorGuardAfterAutoMerge|TestPushProbe_ArgvMatchesPushBranch|TestFinalizeEpic_WithRemotePushes|TestFinalizeEpic_NoRemoteMergesLocally|TestFinalizeEpic_PassedFalseWithRemoteMergesLocally|TestFinalizeEpic_PassedTrueNoRemoteTakesPushLeg'` passes
- [ ] `go test ./internal/gitutil/... ./internal/lifecycle/... -run 'RemoteConfigured|HasRemote'` passes
- [ ] `go test ./internal/validate/... -run TestDocs_Spec132Amendments_Preflight` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-3 (R2.1): a layout-guard failure and a destruction failure each exit non-zero with the epic open and the phase unchanged, also from a stale-phase fixture.
- [ ] AC-4 (R2.2): `TestFinalizeEpic_ExecutorGuardAfterAutoMerge` passes.
- [ ] AC-5 (R2.3–R2.5): the four push-probe tests pass.
- [ ] AC-15 (R4.3, R4.4, R4.4a): the six tests pass. The probe runs exactly one time. An injected probe error with `--supersede-adr` leaves the epic open, the phase unchanged and no placeholder ADR.
- [ ] AC-21 (R4.4a part): `TestApproveImpl_HasRemoteProbeErrorRefuses` asserts that the refusal matches none of `ADR-[0-9]`, `q9ea`, `[Ss]pec [0-9]{3}`.

**Depends on**
None.

## Bead 3: `impl approve` state line, finalize tip and re-run

**Steps**
1. After a failure past MUTATION 1/3, read the epic status and print one state line before the inner error: "epic closed" or "epic still open", plus the failed finalize stage. Keep the inner recovery lines last. Add `recovery: mindspec impl approve <spec>` only when the inner error has none (R2.6–R2.8).
2. In `FinalizeEpic`, write epic metadata `mindspec_finalize_tip` after the export commit and all bead auto-merges, immediately before the spec-to-main merge or the spec-branch push. Every run overwrites it. Keep the pre-finalize tip record separate (R2.9).
3. Make each stage converge on re-run, on both paths, per the R2.10 table: idempotent close; skip merged beads; push no-op; reuse `chore/finalize-<id>` and push it with `--force-with-lease` on the SHA that `git ls-remote` reads just before the push; skip a direct merge when the spec tip is an ancestor of main; finish remaining cleanup. Open no PR (R2.10).
4. While the spec branch exists, a re-run runs every pre-close gate again on the current spec tip (R2.11).
5. Add the "already landed" re-run per R2.12. Use the has-remote value from Bead 2 to pick the target: main (direct), the remote spec branch (push), or remote main when the host deleted the spec branch. Fetch the target into a temporary ref. A fetch failure, a missing object, or a squash or rebase merge refuses with today's branch-missing recovery. Cleanup runs the destruction guard again and deletes only bead branches whose tips are ancestors of the target.
6. Docs: under ADR-0041 `## Amendment (Spec 132)`, add the subsection for the state line, `mindspec_finalize_tip` and stage resume. Update `.mindspec/domains/workflow/architecture.md` and `runbook.md` (re-run recovery), and `.mindspec/domains/execution/architecture.md` and `interfaces.md` (finalize tip, stage resume). Add `internal/validate/adr_spec132_finalize_test.go` (`TestDocs_Spec132Amendments_Finalize`).

**Verification**
- [ ] `go test ./internal/approve/... -run 'TestApproveImpl_PostCloseConflictKeepsInnerRecovery|TestApproveImpl_StateLineReadsEpicStatus|TestApproveImpl_PostCloseBareErrorGetsRerun|TestApproveImpl_RerunConvergesFromEachStage|TestApproveImpl_RerunReRunsGates|TestApproveImpl_AfterDeletionAlreadyLanded|TestApproveImpl_RemoteSpecBranchDeletedUsesMain|TestApproveImpl_TargetFetchFailureRefuses|TestApproveImpl_ForgedFinalizeTipRefuses'` passes
- [ ] `go test ./internal/executor/... -run TestFinalizeEpic_FinalizeTipIsPublishedTip` passes
- [ ] `go test ./internal/validate/... -run TestDocs_Spec132Amendments_Finalize` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-6 (R2.7, R2.8): the three state-line tests pass.
- [ ] AC-7 (R2.9, R2.10): `TestApproveImpl_RerunConvergesFromEachStage` covers every `finalizeStepHook` stage, `orphan_finalize` included, on both paths, with one close, exit 0 and at most one push per remote ref per converged state. `TestFinalizeEpic_FinalizeTipIsPublishedTip` passes.
- [ ] AC-8 (R2.11, R2.12): the five re-run tests pass. A forged `mindspec_finalize_tip` refuses and the bead branch survives.

**Depends on**
Bead 2. It needs the `FinalizeEpic` has-remote parameter and the one has-remote value to pick the R2.12 target.

## Bead 4: Merge-state rules and doctor severity

**Steps**
1. Add `internal/lifecycle/merge_state.go`. It finds the bead's naming merges on the spec branch's first-parent chain, decides a valid merge M (spec Terms), and runs the R3.5 presence check: paths M changes against M^1, minus tool-artifact paths; reverted when equal to M^1 content at the spec tip or added by M and absent; exempt when a later naming merge for another bead changed the path.
2. Add the R3.7 restore-recovery builder: each later commit that changed a reverted path, newest first, with full SHA, escaped subject and M paths; `git -C <spec worktree> revert <commit>` with `-m 1` for a merge; the path-scoped `checkout <M> -- <paths>` choice; then the R3.6 lines from step 2 on. Bead 5 reuses it.
3. Add doctor status `Info` (`internal/doctor/doctor.go`). It does not count in `HasFailures` and does not change the exit code. `Fix` takes no action on Info. `cmd/mindspec/doctor.go` renders it as `[INFO]` (R3.11).
4. Make the stale-open finding (`internal/lifecycle/stale_open.go`, `internal/doctor/lifecycle_integrity.go`) Info with no FixFunc and recovery `mindspec complete <bead>` when a valid merge exists. Without one it stays Error. The merge-only marker does not change severity (R3.11).
5. Report, at Error, an open bead whose naming merge is fully or partly reverted, with the R3.7 recovery. Report no stale-open finding for that bead, give it no FixFunc, and give no finding for a closed bead (R3.12).
6. Docs: update `.mindspec/domains/workflow/architecture.md` (git-state severity rule) and `interfaces.md` (doctor Info status).

**Verification**
- [ ] `go test ./internal/doctor/... -run 'TestDoctor_StaleOpenSeverityFromGitState|TestDoctor_RevertedMergeOnlyOneFinding|TestDoctor_FixSkipsRevertedMergeOnly|TestDoctor_ClosedRevertedNoFinding|TestDoctor_FixSkipsStaleOpenInfo'` passes
- [ ] `go test ./cmd/mindspec/... -run TestDoctorStatusTag_Info` passes
- [ ] `go test ./internal/lifecycle/... -run 'TestMergeState_'` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-13 (R3.11, R3.12): the six AC-13 tests pass. A stale-open bead with a valid merge gives Info, and `doctor --fix` leaves it open.
- [ ] The R3.5 presence check and the valid-merge rule exist as one lifecycle API that Bead 5 calls. `TestMergeState_LaterMergeExemptsPath` and `TestMergeState_LaterEditNotRevert` pass.

**Depends on**
None.

## Bead 5: `complete --merge-only` and the closing path

**Steps**
1. Add `--merge-only` to `mindspec complete` (`cmd/mindspec/complete.go`, `CompleteOpts`). Implement R3.0 path selection in the stated order (paths 1–7), with the marker check on paths 2, 4 and 6 and recovery `bd update <bead> --unset-metadata mindspec_merge_only_sha`.
2. Merge-only path (`internal/complete/merge_only.go`): run every gate, materialize, write `mindspec_merge_only=pending`, merge, print the merge SHA and both parents, then replace the pending key with `mindspec_merge_only_sha=<merge>`. Keep the bead open with no binding, ledger commit or cleanup. A conflict recovery is `mindspec complete <bead> --merge-only --resolve-merge`. A repeated run with a valid merge for the tip writes the marker only. Refuse a second naming merge (R3.1–R3.4, R3.10).
3. When `OWNERSHIP.yaml` conflicts in the Bead 1 landing tree, plain `complete` on an open bead (path 7) refuses before the close with `recovery: mindspec complete <bead> --merge-only`. `--merge-only` and path 5 do not refuse here (R1.4).
4. Closing path (`internal/complete/closing.go`, paths 4 and 6): refuse on a busy spec worktree, a moved tip, user changes, an invalid merge or a failed presence check (Bead 4 API). Run the panel gate at the bead tip and the doc-sync, ADR and ownership gates on M (base M^1, head M, `ownerRef` M's tree). Then run steps 1–6 of R3.8, each a no-op when its done signal holds. Step 4 is a path-scoped commit of the export and panel ledger in the spec worktree (R3.4, R3.8, R3.13).
5. Implement the path-4 cleanup guard for a layout that is not flat, the refusal texts of R3.6 and R3.7 (Bead 4 builder), the step 1–4 failure recovery that repeats `--allow-doc-skew` or `--override-adr` with its reason, and the R3.13 exits (R3.6, R3.7, R3.9, R3.13).
6. Docs: under ADR-0041 `## Amendment (Spec 132)`, add the subsection for `--merge-only` as a commit phase with no close and the closing call. Update `.mindspec/domains/workflow/architecture.md`, `interfaces.md` (`--merge-only`, metadata keys) and `runbook.md`, and `project-docs/user/guides/merge-safety.md`: `--merge-only`, the one-naming-merge rule, "do not revert a `--merge-only` merge by hand" and the restore-then-close recovery (R13.3). If the bead adds an Executor method, update `.mindspec/domains/execution/interfaces.md`. Add `internal/validate/adr_spec132_mergeonly_test.go` (`TestDocs_Spec132Amendments_MergeOnly`).

**Verification**
- [ ] `go test ./internal/complete/... -run 'TestComplete_OwnershipConflictRefusesBeforeClose|TestComplete_OwnershipConflictMergeOnlyLoop|TestComplete_OwnershipResolutionRevokesClaimRefuses'` passes
- [ ] `go test ./internal/complete/... -run 'TestComplete_ClosedBeadMovedTipRefuses|TestComplete_ClosedBeadUserDirtRefuses|TestComplete_ClosedMergedForeignMergeInProgressRefuses|TestComplete_ClosingPathGatesMergeRange|TestComplete_ClosedHandMergeStalePanelRefuses|TestComplete_ClosedHandMergeNoPanelFailsOpen|TestComplete_ClosedHandMergeForgedReviewedKeyRefuses|TestComplete_ClosedNonFlatAbsentWorktreeRefuses|TestComplete_ClosedNonFlatAbsentWorktreeGateOffAllows|TestComplete_ClosedOwnMergeInProgressAborts|TestComplete_MergeOnlyBranchlessRefuses|TestComplete_PendingKeyPlainCompleteStaysOpen|TestComplete_MergeOnlyKeepsBeadOpen|TestComplete_MergeOnlyHandCommitStaysOpen|TestComplete_PlainConflictResolveFinishes|TestComplete_ClosedUnmergedRerunMerges|TestComplete_HandMergeTakesClosingPath|TestComplete_ForeignMarkerRefuses|TestComplete_BranchlessReconcileUnchanged|TestComplete_ClosedBeadResumesAllSteps|TestComplete_MergeOnlyConflictStaysMergeOnly'` passes
- [ ] `go test ./internal/complete/... -run 'TestComplete_BeadAdvancedAfterMergeOnlyRefuses|TestComplete_UserDirtAfterMergeOnlyRefuses|TestComplete_RevertedMergeOnlyRefuses|TestComplete_PartlyRevertedMergeOnlyRefuses|TestComplete_PresenceCheckLaterMergeExemptsPath|TestComplete_PresenceCheckLaterEditNotRevert|TestComplete_SecondMergeOnlyRefuses|TestComplete_MergeOnlyRefusalEscapes|TestComplete_MergeOnlyFollowUpLoop|TestComplete_MergeOnlyRestoreLoop'` passes
- [ ] `go test ./internal/complete/... -run 'TestComplete_AfterMergeOnlyPublishesClose|TestComplete_ClosingPathSpecWorktreeBusyRefuses|TestComplete_AfterMergeOnlyExportFailureConverges|TestComplete_ClosedBeadFinishesCleanup|TestComplete_OverrideRunRecoveryRepeatsFlag|TestComplete_RepeatedMergeOnlyNoNewMerge'` passes
- [ ] `go test ./internal/validate/... -run TestDocs_Spec132Amendments_MergeOnly` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-2 (R1.4 ownership-conflict leg, R3.13): the three ownership-conflict tests pass. The bead stays open on the refusal, and the amend exit passes.
- [ ] AC-9 (R3.0–R3.3, R3.13): the 21 AC-9 tests pass.
- [ ] AC-10 (R3.4–R3.7): the 10 AC-10 tests pass. The follow-up and restore loops end with `doctor` clean for the bead, dependents passing MF-3, and `impl approve` exit 0.
- [ ] AC-11 (R3.8, R3.9): the five AC-11 tests pass.
- [ ] AC-12 (R3.10): `TestComplete_RepeatedMergeOnlyNoNewMerge` passes.

**Depends on**
Bead 1 (the landing tree and its conflicted-path list for the R1.4 refusal) and Bead 4 (the valid-merge, presence-check and restore-recovery API).

## Bead 6: Remove `merge_strategy`

**Steps**
1. Delete `config.MergeStrategy`, its `auto` default, its defaulting and its mention in the defaulting comment (`internal/config/config.go`). Delete the `config show` row (`cmd/mindspec/config.go`). Keep `FinalizeResult.MergeStrategy` and its readers (R4.1).
2. Update the field assertions in `internal/config/config_test.go` and `internal/config/feedback_remote_test.go`. Use `worktree_root` with two valid relative values in `TestLoadCaches_DiskMutationIgnored`, and a retained project-config field in the feedback-remote scope test. Remove `merge_strategy` from `TestCeremonyNonInflation_ConfigKeys` (R4.1).
3. Remove the key from `internal/harness/sandbox.go`, `internal/harness/lifecycle_scenario_test.go`, `cmd/mindspec/verb_output_test.go`, `internal/doctor/config_test.go` and `internal/doctor/models_commands_test.go`. Keep it only in named legacy-key fixtures. Remove `merge_strategy: pr` from `.mindspec/config.yaml`. Keep the `scenario_finalize_pr` push leg (R4.2, R4.7).
4. Add a doctor check in `internal/doctor/config.go`: one Info (Bead 4 status) when `.mindspec/config.yaml` has a top-level `merge_strategy` key, by line regex like the existing key checks. A commented or indented key gives nothing (R4.5, R4.6).
5. Add `internal/lint/no_merge_strategy_writers_test.go` (`TestNoMergeStrategyWriters`). It scans Go sources, test fixtures and `.mindspec/config.yaml`. Its exemptions are named test functions or fixture constants, not files.
6. Docs: update the config example in `.mindspec/core/GIT-WORKFLOW.md`, the Config description in `.mindspec/domains/core/interfaces.md`, `.mindspec/domains/workflow/interfaces.md` (`config show`, the doctor Info) and `.mindspec/domains/execution/architecture.md` (sandbox config). Add dated removal notes at the `merge_strategy` guidance in ADR-0006 and ADR-0019. Add `internal/validate/adr_spec132_mergestrategy_test.go` (`TestDocs_Spec132Amendments_MergeStrategy`).

**Verification**
- [ ] `go test ./internal/config/... -run 'TestConfigLoad_LegacyMergeStrategyIgnored|TestLoadCaches_DiskMutationIgnored'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestConfigShow_NoMergeStrategyRow|TestCeremonyNonInflation_ConfigKeys'` passes
- [ ] `go test ./internal/harness/... -run TestHarnessSandbox_NoMergeStrategy` passes
- [ ] `go test ./internal/doctor/... -run 'TestDoctor_LegacyMergeStrategyInfo|TestDoctor_LegacyMergeStrategyCommentedOrNested'` passes
- [ ] `go test ./internal/lint/... -run TestNoMergeStrategyWriters` passes
- [ ] `go test ./internal/validate/... -run TestDocs_Spec132Amendments_MergeStrategy` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-14 (R4.1, R4.2, R4.5, R4.6): the six AC-14 tests pass. The Info does not make doctor exit non-zero.
- [ ] AC-21 (R4.6 part): `TestDoctor_LegacyMergeStrategyInfo` asserts that the Info text matches none of `ADR-[0-9]`, `q9ea`, `[Ss]pec [0-9]{3}`.
- [ ] AC-26 (R4.2 notes): the ADR-0006 and ADR-0019 notes are pinned.

**Depends on**
Bead 4 (the doctor Info status). Lane note: start after the other beads are in flight.

## Bead 7: Reviewed-tree datum

**Steps**
1. In `internal/lifecycle/landed.go`, build the reviewed set R from the panel registrations (`reviewedHeadSHAForBead`). Count `mindspec_landed_reviewed_sha` only when it is in R. Otherwise the provenance calls it unverified (R5.1).
2. In `FindLandedMerge`, confirm a candidate with second parent D when D is in R, or when some r in R is an ancestor of D and `git diff --no-renames --name-only -z r D` lists no path or only paths in `panel.ToolArtifactPaths` for the spec directory of the bead's own epic (R5.2).
3. Keep tip and binding contradictions and the revert check in force after the datum. Do not evaluate the datum on the reachable-fallback path (R5.3, R5.4).
4. Name the matching rule in the provenance string: `exact` or `reviewed-tree` (R5.5).
5. Docs: under ADR-0041 `## Amendment (Spec 132)`, add the subsection that changes amendment (ii): the reviewed-tree rule confirms and never vetoes. Update `.mindspec/domains/workflow/architecture.md`. Add `internal/validate/adr_spec132_reviewedtree_test.go` (`TestDocs_Spec132Amendments_ReviewedTree`).

**Verification**
- [ ] `go test ./internal/lifecycle/... -run 'TestFindLandedMerge_ReviewedTreeEmptyWrapperConfirms|TestFindLandedMerge_ReviewedTreeSourceDiffNoConfirm|TestFindLandedMerge_ReviewedTreeRenameNoConfirm|TestFindLandedMerge_ReviewedTreeOtherSpecDirNoConfirm|TestFindLandedMerge_UnregisteredKeyNoConfirm|TestReattestFallback_IgnoresPanelDatum'` passes
- [ ] `go test ./internal/complete/... -run TestFindLandedMerge_ReviewedTreeConfirms` passes (the `complete` reconcile path on a branch-less bead; it fails on the old code)
- [ ] `go test ./internal/validate/... -run TestDocs_Spec132Amendments_ReviewedTree` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-16 (R5.1–R5.5): the seven AC-16 tests pass, with provenance `reviewed-tree` on the confirming case.

**Depends on**
None.

## Bead 8: Ancestor without a naming merge

**Steps**
1. Add a base-taking revert check in `internal/gitutil/neteffect.go`: given base, head and target, it reports whether the target reverts any change in base..head (R6.5).
2. In `internal/lifecycle/reattest.go`, evaluate R6 only after the primary scan and the reachable fallback find no candidate. Apply R6.2–R6.4: absent branch, non-ancestor tip, empty range, unknown base B, and "not P's own work" when P's tip equals another bead's naming-merge second parent. A git read error refuses as indeterminate (R6.1–R6.4).
3. Check B..P-tip at the spec tip with `ContentSubsumedOutcome` and the new revert check. Content present and not reverted: state `ancestor-without-naming-merge` with the person-must-verify sentence and one `mindspec next <dependent> --allow-not-ready=MF-3 --evidence <file>` line per open dependent in the epic, or `recovery: mindspec doctor` with zero dependents. Otherwise keep `no-owned-exact-merge` with the history line and no "present" claim (R6.5–R6.8, R6.10).
4. Add a read seam to each of the three `fallbackSafety` reads (`internal/lifecycle/fallback.go`). Each read error refuses (R6.11).
5. Add the R6 finding to the lifecycle scan (`internal/lifecycle/scan.go`). Render it in a new `internal/doctor/ancestor.go` at Warn, naming P, with the R6.8 recovery lines (R6.9).
6. Rewrite the `no-owned-exact-merge` text, its render in `cmd/mindspec/reattest.go` and the reattest `Short` text with no ADR number or bead ID. The no-owned refusal says "attested restore" and that a person must verify the merge first. Recovery lines are commands with no parentheses.
7. Docs: update `.mindspec/domains/workflow/architecture.md` and `runbook.md`, `.mindspec/domains/execution/interfaces.md` (the revert check), and `project-docs/user/guides/merge-safety.md` with the attested-restore exit and the R6 states (R13.3).

**Verification**
- [ ] `go test ./internal/lifecycle/... -run 'TestReattest_Ancestor|TestFallbackSafety_ForwardContentErrorRefuses|TestFallbackSafety_ReverseContentErrorRefuses|TestFallbackSafety_BranchTipErrorRefuses'` passes, and `-list 'TestReattest_Ancestor'` prints the 14 AC-17, AC-18 and AC-19 reattest tests
- [ ] `go test ./internal/doctor/... -run TestDoctor_AncestorNamesPrerequisite` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestReattestText_NoHistoricalCitations|TestReattestText_OperandsRender'` passes
- [ ] `go test ./internal/gitutil/... -run Revert` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-17 (R6.1–R6.6): `TestReattest_AncestorWithoutNamingMerge` passes.
- [ ] AC-18 (R6.2–R6.5, R6.7): the ten AC-18 tests pass.
- [ ] AC-19 (R6.8, R6.9): `TestReattest_AncestorRecoveryPerDependent`, `TestReattest_AncestorZeroDependents`, `TestDoctor_AncestorNamesPrerequisite` pass.
- [ ] AC-20 (R6.11): the three `TestFallbackSafety_` tests pass.
- [ ] AC-21 (reattest part): `TestReattestText_NoHistoricalCitations` and `TestReattestText_OperandsRender` pass with fixture bead `mindspec-ab12`.

**Depends on**
None.

## Bead 9: One lineage and full spec IDs

**Steps**
1. In `internal/phase`, add `FindEpicLineageWithCache`. It returns the epic, the spec ID and the lineage provenance. Lineage is the one `parent-child` dependency on an epic. A `blocks` edge is not lineage. Two parent-child epics refuse with `recovery: bd dep remove <bead> <epic>`. Remove the bead-title fallback. Keep the signatures of `FindEpicForBead` and `FindEpicForBeadWithCache`; they call the new function (R7.1–R7.3).
2. Resolve epic to spec from three sources in order: `spec_id`, then `spec_num` plus `spec_title`, then the epic's `[SPEC <id>]` title as a full spec ID. Apply the R7.6 rules: a valid `spec_id` decides; an invalid one refuses; `spec_num` 0 or an empty title is absent; a disagreement or no source refuses with `recovery: bd update <epic> --set-metadata spec_id=<id>`; two epics on one spec refuse in a verb that resolves that spec and name both (R7.6).
3. `spec approve` writes the full spec ID to epic metadata `spec_id` (`internal/approve/spec.go`) (R7.5).
4. `next` uses the returned provenance and runs no second `bd show` per plan row (`internal/next/beads.go`) (R7.4).
5. `beadIndex` (`internal/validate/readiness/readiness.go`) accepts a bead ID only when it equals `<epicID>.<N>` (R7.7). `spec create` refuses a new lettered ID such as `129b-x` (`cmd/mindspec/spec.go`); existing lettered directories stay valid (R7.9).
6. Add `internal/doctor/lineage.go`: a Warn for each open bead that resolves only through the removed title rule, with `recovery: bd update <bead> --parent <epic>`, and a report for two epics on one spec that continues (R7.6, R7.8).
7. Docs: update `.mindspec/domains/core/interfaces.md` and `architecture.md` (lineage API, spec-ID resolution) and `.mindspec/domains/workflow/interfaces.md` (`spec_id` metadata, doctor lineage check, `spec create` refusal).

**Verification**
- [ ] `go test ./internal/phase/... -run 'TestFindEpicForBead_BlocksDepIgnored|TestFindEpicForBead_TwoParentsRefuse|TestFindEpicForBead_NoTitleFallback|TestResolveSpec_'` passes
- [ ] `go test ./internal/next/... -run TestBuildReadySet_OneShowPerPlanRow` passes
- [ ] `go test ./internal/validate/readiness/... -run TestBeadIndex_RequiresEpicPrefix` passes
- [ ] `go test ./internal/doctor/... -run TestDoctor_TitleOnlyLineage` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestSpecCreate_RefusesLetteredID|TestLifecycle_LetteredSpecResolves'` passes
- [ ] `go test ./internal/approve/... -run TestSpecApprove_WritesSpecID` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-22 (R7.1–R7.3): the three `TestFindEpicForBead_` tests pass.
- [ ] AC-23 (R7.4): `TestBuildReadySet_OneShowPerPlanRow` passes.
- [ ] AC-24 (R7.5, R7.6): `TestResolveSpec_PlainAndLetteredBothPresent`, `TestLifecycle_LetteredSpecResolves`, `TestResolveSpec_LegacyPairRepairResolves`, `TestResolveSpec_TitleWithoutIDRefuses`, `TestResolveSpec_SpecNumZeroAbsent`, `TestResolveSpec_InvalidSpecIDRefuses`, `TestResolveSpec_TwoEpicsScopedRefusal` pass.
- [ ] AC-25 (R7.7–R7.9): `TestBeadIndex_RequiresEpicPrefix`, `TestDoctor_TitleOnlyLineage`, `TestSpecCreate_RefusesLetteredID` pass, and `idvalidate` accepts `008b-human-gates`.

**Depends on**
None.

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| AC-1 (R1.1–R1.3) | Bead 1 verification: `go test ./internal/complete/...` and `./internal/validate/...` landing-tree tests |
| AC-2 (R1.4, R1.5, R3.13) | Bead 1 verification (`TestCommittedRangeGate_SourceConflictPassesThenResolves`); Bead 5 verification (three ownership-conflict tests) |
| AC-3 (R2.1) | Bead 2 verification: `TestApproveImpl_Early*` |
| AC-4 (R2.2) | Bead 2 verification: `TestFinalizeEpic_ExecutorGuardAfterAutoMerge` |
| AC-5 (R2.3–R2.5) | Bead 2 verification: push-probe tests and `TestPushProbe_ArgvMatchesPushBranch` |
| AC-6 (R2.7, R2.8) | Bead 3 verification: state-line tests |
| AC-7 (R2.9, R2.10) | Bead 3 verification: `TestApproveImpl_RerunConvergesFromEachStage`, `TestFinalizeEpic_FinalizeTipIsPublishedTip` |
| AC-8 (R2.11, R2.12) | Bead 3 verification: five re-run tests |
| AC-9 (R3.0–R3.3, R3.13) | Bead 5 verification: 21 AC-9 tests |
| AC-10 (R3.4–R3.7) | Bead 5 verification: 10 AC-10 tests; Bead 4 verification (`TestMergeState_`) |
| AC-11 (R3.8, R3.9) | Bead 5 verification: five AC-11 tests |
| AC-12 (R3.10) | Bead 5 verification: `TestComplete_RepeatedMergeOnlyNoNewMerge` |
| AC-13 (R3.11, R3.12) | Bead 4 verification: five doctor tests and `TestDoctorStatusTag_Info` |
| AC-14 (R4.1, R4.2, R4.5, R4.6) | Bead 6 verification: six AC-14 tests |
| AC-15 (R4.3, R4.4, R4.4a) | Bead 2 verification: four `TestFinalizeEpic_` remote tests, `TestApproveImpl_OneHasRemoteDecision`, `TestApproveImpl_HasRemoteProbeErrorRefuses` |
| AC-16 (R5.1–R5.5) | Bead 7 verification: seven AC-16 tests |
| AC-17 (R6.1–R6.6) | Bead 8 verification: `TestReattest_AncestorWithoutNamingMerge` |
| AC-18 (R6.2–R6.5, R6.7) | Bead 8 verification: ten AC-18 tests |
| AC-19 (R6.8, R6.9) | Bead 8 verification: two reattest tests and `TestDoctor_AncestorNamesPrerequisite` |
| AC-20 (R6.11) | Bead 8 verification: three `TestFallbackSafety_` tests |
| AC-21 (internal refs) | Bead 8 (`TestReattestText_*`), Bead 1 (`TestMaterializeText_NoHistoricalCitations`), Bead 2 (R4.4a text in `TestApproveImpl_HasRemoteProbeErrorRefuses`), Bead 6 (R4.6 text in `TestDoctor_LegacyMergeStrategyInfo`) |
| AC-22 (R7.1–R7.3) | Bead 9 verification: three `TestFindEpicForBead_` tests |
| AC-23 (R7.4) | Bead 9 verification: `TestBuildReadySet_OneShowPerPlanRow` |
| AC-24 (R7.5, R7.6) | Bead 9 verification: seven resolution tests |
| AC-25 (R7.7–R7.9) | Bead 9 verification: `TestBeadIndex_RequiresEpicPrefix`, `TestDoctor_TitleOnlyLineage`, `TestSpecCreate_RefusesLetteredID` |
| AC-26 (R13) | `go test ./internal/validate/... -run TestDocs_Spec132Amendments` over the pins of Beads 1, 2, 3, 5, 6 and 7; Bead 1 `mindspec validate spec 129-readiness-tracker-honesty`; R13.3 doc text in Beads 5 and 8 |
| Validation proofs | Every bead: `gofmt -l .`, `go build ./...`, `go test ./... -short -timeout 20m`; `golangci-lint run` at final review |
