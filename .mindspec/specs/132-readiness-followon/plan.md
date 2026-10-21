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
      internal/lint/ratchet_argv_table_test.go, .mindspec/adr/ADR-0036-ownership-discovery.md,
      .mindspec/specs/129-readiness-tracker-honesty/spec.md,
      .mindspec/specs/129-readiness-tracker-honesty/plan.md,
      .mindspec/domains/workflow/architecture.md, .mindspec/domains/execution/interfaces.md,
      internal/validate/adr_spec132_ownership_test.go, internal/validate/adr_divergence.go,
      internal/validate/snapshot_leg_test.go, internal/complete/complete_test.go,
      .mindspec/domains/workflow/interfaces.md,
      internal/lint/ratchet_inline_allowlist_test.go,
      internal/lint/ratchet_composition_allowlist_test.go]
  - id: 2
    depends_on: []
    key_file_paths: [internal/approve/impl.go, internal/approve/impl_preflight_test.go,
      internal/approve/impl_test.go, internal/approve/main_test.go,
      internal/approve/merge_preflight_ac5_test.go, internal/lifecycle/gitquery.go,
      internal/gitutil/gitops.go, internal/gitutil/gitops_test.go, internal/executor/executor.go,
      internal/executor/mindspec_executor.go, internal/executor/mock.go,
      internal/executor/finalize_hasremote_test.go, internal/executor/hasremote_callsite_test.go,
      internal/executor/finalize_fault_test.go, internal/executor/finalize_worktree_only_test.go,
      internal/executor/executor_test.go, internal/executor/neteffect_probe_test.go,
      internal/executor/finalize_scoping_test.go, internal/executor/layout_guard_test.go,
      internal/executor/finalize_orphan_test.go, internal/executor/merge_binding_test.go,
      internal/executor/merge_conflict_test.go, internal/lint/ratchet_argv_table_test.go,
      internal/guard/registries.go, internal/guard/registries_test.go,
      internal/lint/testdata/destructive_seed_manifest.txt,
      .mindspec/adr/ADR-0041-gate-before-mutate.md,
      .mindspec/domains/workflow/architecture.md, .mindspec/domains/execution/interfaces.md,
      .mindspec/domains/execution/architecture.md, internal/validate/adr_spec132_preflight_test.go,
      internal/lint/ratchet_inline_allowlist_test.go,
      internal/lint/ratchet_composition_allowlist_test.go]
  - id: 3
    depends_on: [2]
    key_file_paths: [internal/approve/impl.go, internal/approve/impl_rerun_test.go,
      internal/executor/mindspec_executor.go, internal/executor/executor.go,
      internal/executor/mock.go, internal/executor/finalize_tip_test.go, internal/gitutil/gitops.go,
      internal/gitutil/gitops_test.go, internal/lint/ratchet_argv_table_test.go,
      internal/guard/registries.go, internal/guard/registries_test.go,
      internal/lint/testdata/destructive_seed_manifest.txt,
      .mindspec/adr/ADR-0041-gate-before-mutate.md,
      .mindspec/domains/workflow/architecture.md, .mindspec/domains/workflow/runbook.md,
      .mindspec/domains/execution/architecture.md, .mindspec/domains/execution/interfaces.md,
      internal/validate/adr_spec132_finalize_test.go, internal/approve/impl_test.go,
      internal/approve/main_test.go, internal/executor/finalize_fault_test.go,
      internal/executor/finalize_orphan_test.go,
      internal/lint/ratchet_inline_allowlist_test.go,
      internal/lint/ratchet_composition_allowlist_test.go]
  - id: 4
    depends_on: []
    key_file_paths: [internal/lifecycle/merge_state.go, internal/lifecycle/merge_state_test.go,
      internal/lifecycle/stale_open.go, internal/lifecycle/scan.go, internal/gitutil/gitops.go,
      internal/gitutil/gitops_test.go, internal/lint/ratchet_argv_table_test.go,
      internal/guard/registries.go, internal/guard/registries_test.go,
      internal/lint/testdata/destructive_seed_manifest.txt,
      internal/doctor/doctor.go, internal/doctor/doctor_test.go,
      internal/doctor/lifecycle_integrity.go, internal/doctor/lifecycle_integrity_test.go,
      cmd/mindspec/doctor.go, cmd/mindspec/doctor_info_test.go,
      .mindspec/domains/workflow/architecture.md, .mindspec/domains/workflow/interfaces.md,
      .mindspec/domains/execution/interfaces.md, internal/doctor/skip_local_env_test.go,
      internal/lint/ratchet_inline_allowlist_test.go,
      internal/lint/ratchet_composition_allowlist_test.go]
  - id: 5
    depends_on: [1, 4]
    key_file_paths: [internal/complete/complete.go, internal/complete/merge_only.go, internal/complete/closing.go,
      internal/complete/merge_only_test.go, internal/complete/closing_test.go,
      internal/complete/ownership_conflict_test.go, internal/complete/panel_advisory.go,
      cmd/mindspec/complete.go, cmd/mindspec/merge_only_loop_test.go, internal/executor/executor.go,
      internal/executor/mindspec_executor.go, internal/executor/mock.go,
      internal/guard/registries.go, internal/guard/registries_test.go,
      internal/lint/testdata/destructive_seed_manifest.txt,
      internal/lint/destructive_guidance_test.go, internal/lint/ratchet_argv_table_test.go,
      .mindspec/adr/ADR-0041-gate-before-mutate.md, .mindspec/domains/workflow/architecture.md,
      .mindspec/domains/workflow/runbook.md, .mindspec/domains/workflow/interfaces.md,
      .mindspec/domains/execution/interfaces.md, project-docs/user/guides/merge-safety.md,
      internal/validate/adr_spec132_mergeonly_test.go, cmd/mindspec/ceremony_guard_test.go,
      cmd/mindspec/named_invocation_test.go,
      internal/lint/ratchet_inline_allowlist_test.go,
      internal/lint/ratchet_composition_allowlist_test.go]
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
    depends_on: [4]
    key_file_paths: [internal/lifecycle/landed.go, internal/lifecycle/landed_reviewed_tree_test.go,
      internal/complete/reviewed_tree_reconcile_test.go,
      .mindspec/adr/ADR-0041-gate-before-mutate.md, .mindspec/domains/workflow/architecture.md,
      internal/validate/adr_spec132_reviewedtree_test.go,
      internal/lint/ratchet_inline_allowlist_test.go,
      internal/lint/ratchet_composition_allowlist_test.go]
  - id: 8
    depends_on: []
    key_file_paths: [internal/lifecycle/reattest.go, internal/lifecycle/reattest_ancestor_test.go,
      internal/lifecycle/fallback.go, internal/lifecycle/fallback_test.go,
      internal/lifecycle/scan.go, internal/gitutil/neteffect.go, internal/gitutil/neteffect_test.go,
      internal/lint/ratchet_argv_table_test.go, cmd/mindspec/reattest.go,
      cmd/mindspec/reattest_test.go, internal/doctor/ancestor.go,
      internal/doctor/lifecycle_integrity.go, internal/doctor/ancestor_test.go,
      internal/guard/registries.go, internal/guard/registries_test.go,
      internal/lint/testdata/destructive_seed_manifest.txt,
      internal/lint/destructive_guidance_test.go,
      .mindspec/domains/workflow/architecture.md, .mindspec/domains/workflow/runbook.md,
      .mindspec/domains/execution/interfaces.md, project-docs/user/guides/merge-safety.md,
      internal/lifecycle/reattest_test.go,
      internal/lint/ratchet_inline_allowlist_test.go,
      internal/lint/ratchet_composition_allowlist_test.go]
  - id: 9
    depends_on: []
    key_file_paths: [internal/phase/derive.go, internal/phase/cache.go, internal/phase/cache_test.go,
      internal/phase/lineage.go, internal/phase/lineage_test.go,
      internal/phase/lineage_error_test.go, internal/phase/agreement_table_test.go,
      internal/phase/resolve_spec_test.go, internal/approve/spec.go, internal/approve/spec_test.go,
      internal/next/beads.go, internal/next/readyset_test.go,
      internal/validate/readiness/readiness.go, internal/validate/readiness/readiness_test.go,
      internal/doctor/doctor.go, internal/doctor/lineage.go, internal/doctor/lineage_test.go,
      cmd/mindspec/spec.go, cmd/mindspec/spec_create_lettered_test.go,
      cmd/mindspec/lettered_spec_test.go, cmd/mindspec/verb_output_test.go,
      internal/guard/registries.go, internal/guard/registries_test.go,
      internal/lint/testdata/destructive_seed_manifest.txt, .mindspec/adr/ADR-0023.md,
      .mindspec/domains/core/interfaces.md, .mindspec/domains/core/architecture.md,
      .mindspec/domains/workflow/interfaces.md, internal/validate/adr_spec132_lineage_test.go,
      internal/next/nearmiss.go, internal/next/nearmiss_test.go, internal/lifecycle/scan.go,
      .mindspec/domains/workflow/architecture.md,
      internal/approve/plan.go, cmd/mindspec/next_phase_advance_test.go,
      cmd/mindspec/next_positional_test.go, cmd/mindspec/next_ready_gate_test.go,
      cmd/mindspec/next_selection_test.go,
      internal/lint/ratchet_inline_allowlist_test.go,
      internal/lint/ratchet_composition_allowlist_test.go]
---
# Plan: 132-readiness-followon

## ADR Fitness

- **ADR-0041 (gate before mutate)**: stays fit. Amended. Beads 2, 3, 5 and 7 each add one subsection under a new `## Amendment (Spec 132)` section. Bead 2: the early guard copies, the push probe and the one has-remote decision. Bead 3: the state line, `mindspec_finalize_tip` and stage resume. Bead 5: `--merge-only` is a commit phase with no close, and the closing call publishes the close. Bead 7: the reviewed-tree rule changes amendment (ii); it confirms and never vetoes. Bead 7 also adds an inline back-pointer in the Spec 129 section (ii). Every phrase that `TestADR0041_Spec129Amendment` and `cmd/mindspec/adr0041_amendment_test.go` pin stays in place. Its snapshot-gate sentence "anchored at `T` too" stays unchanged; the ADR-0036 amendment (Bead 1) narrows it for ownership reads.
- **ADR-0036 (ownership discovery)**: stays fit. Amended by Bead 1: `complete` reads ownership at the landing tree. One ref feeds all six readers. The amendment names the ADR-0041 snapshot-gate clause it narrows ("every OWNERSHIP/content read anchored at `T` too"): the ownership read moves to the landing tree, and the diffed content stays at `T`. Bead 1 does not edit ADR-0041, so ADR-0041 gets no fifth parallel editor. `TestDocs_Spec132Amendments_Ownership` pins this text.
- **ADR-0025 (JSONL as build artifact)**: applied, no edit. Bead 5 closing step 4 commits the export on the spec branch before step 6 deletes the branch and worktree. The export shows the close before evidence goes.
- **ADR-0023 (lineage is a parent-child dependency)**: stays fit. Amended by Bead 9 with an `#### Amendment (spec 132-readiness-followon)` subsection, in the form of the spec 095 and spec 128 amendments. It states: only a parent-child edge to an epic is lineage, and the bead-title fallback is gone; epic-to-spec binding reads the stored `spec_id` key first, `spec_num` plus `spec_title` second, and the `[SPEC <id>]` epic title third, so a lettered ID resolves to its own spec. The `%03d` derivation stays the rule for the second source only.
- **ADR-0030 (executor boundary)**: applied, no edit. `internal/complete` and `internal/approve` do not import `internal/gitutil`. The landing tree (Bead 1), the early guards and the push probe (Bead 2) and the stage git work (Bead 3) go through Executor methods. `internal/lifecycle` calls `internal/gitutil` as today. `go test ./internal/lint/... -run TestEnforcementHasNoGitLeaks` stays green in every bead.
- **ADR-0035 (agent error contract)**: applied, no edit. Every new refusal ends with one or more `recovery:` lines. A recovery with a substituted operand is registered in `guard.OpaqueOperandRegistry` (`internal/guard/registries.go`). The R3.7 path-scoped restore line (`git -C <spec worktree> checkout <M> -- <paths>`) is in the floor family `FamilyGitCheckoutPathspecDiscard`. Per the Spec 127 amendment, clause 1(a), it is built only through `guard.NewDestructiveCommand` with an evaluated `guard.DestructionOutcome` (Bead 4). Its guidance copy in `merge-safety.md` gets a `KnownSitesExemptionList` entry (Bead 5).
- **ADR-0019, ADR-0006**: noted. Bead 6 adds a dated removal note at their `merge_strategy` guidance. The decisions stay.

No ADR needs a superseding ADR. Each pin file is `internal/validate/adr_spec132_<topic>_test.go`. Each pin function name has the form `TestDocs_Spec132Amendments_<Topic>`. Each pin asserts only the text of its own bead.

## Operator Rulings and Lane Constraints

- **Spec open questions are fixed.** `merge_strategy`: delete it and keep the has-remote rule (Bead 6 deletes; Bead 2 keeps the rule). #250 shape (a): `--merge-only` plus the git-state severity rule (Beads 4 and 5). Split: R8–R12 go to a follow-on spec; no bead implements them. Freshness warning: ruled (a) and implemented by the follow-on spec; no bead implements it.
- **Has-remote probe failure (lane ruling, R4.4a).** A failed probe refuses before any mutation and never selects local merge (Bead 2).
- **Parallel lane 131.** Both lanes touch `internal/config/**` and `internal/executor/**`. No bead rebases on or merges spec 131's unmerged branch. Spec 131 landing on `main` is not a prerequisite of any bead. No bead changes panel config loading, panel verbs or panel skills. Beads 5 and 7 read `panel.ToolArtifactPaths` and the existing panel gate only.
- **When `origin/main` comes in.** The spec branch takes `origin/main` one time: after all nine beads complete and before the final review. If spec 131 is on `main` then, that merge takes it in as a clause union. After that merge, the full suite runs on the merged spec branch: `go build ./... && go test ./... -short -timeout 20m`, plus `go test ./internal/config/... ./cmd/mindspec/... -run 'Config|Ceremony'`, `go test ./internal/lint/... -run TestNoMergeStrategyWriters` and `mindspec validate docs` (doc-sync). Before that point, each bead's checks run against the spec branch that the bead merges into.
- **R4 scheduling.** The R4 key removal is Bead 6. The lane starts Bead 6 after the other beads are in flight. This is a scheduling note, not an edge. Bead 6 has one real edge, to Bead 4: it uses the doctor Info status that Bead 4 adds.
- **R4 split.** R4.3, R4.4 and R4.4a (the one has-remote decision and its error-returning probe) are in Bead 2. The early guards (R2.1) and the push probe (R2.3) consume that one value. Thus Bead 2 needs no edge to Bead 6, and Bead 6 holds only the config key removal.
- **Docs ride with code.** Each bead updates the docs of every domain whose code it changes, in the same commit. No bead defers a doc to a later bead. No bead uses `--allow-doc-skew`.
- **Surface text.** Every bead that touches shipped surface text (CLI help, errors and output, `project-docs/user/guides/merge-safety.md`) keeps it free of internal records: no ADR number, spec number, R-id, bead ID, issue number or incident. Domain docs, ADRs and this plan may cite them.

## Testing Strategy

- **Dogfood boundary.** This spec changes `complete`, `impl approve`, `doctor` and reattest, which the lane itself uses. Every lifecycle command of this lane runs on the installed v0.17.0 binary (`~/.local/bin/mindspec`), never a spec-branch build. No bead installs a branch build. Tests exercise the branch code through `go test` only.
- **Unit tests** use fake `bd` on PATH and `executor.MockExecutor`. They set no real environment variables and start no model CLI.
- **Integration tests** use real temp git repos. `internal/complete` tests reuse `setupPanelGateRepo` (`internal/complete/panel_gate_e2e_test.go`) and `fakeBdOnPath` (`internal/complete/fault_injection_realgit_test.go`). Tests in other packages copy that pattern into a local helper. Remote tests use a `file://` bare remote with `GIT_CONFIG_GLOBAL=/dev/null`.
- **Cross-package tests.** `internal/approve` imports `internal/complete`, so a `package complete` test cannot call `approve.ApproveImpl`. A test that runs both `complete` and `impl approve` lives in `cmd/mindspec` (package `main`), which imports both.
- **Fault seams.** Finalize stages use `finalizeStepHookFn` (`internal/executor/mindspec_executor.go`). Bead 3 exports a test setter for it, so `internal/approve` tests drive the real `MindspecExecutor`. The has-remote probe uses `implHasRemoteFn`, which returns `(bool, error)` after Bead 2. Bead 2 adds a push-probe seam. Bead 5 adds a seam per closing step. Bead 8 adds a read seam for each `fallbackSafety` read.
- **Argv ratchet.** Each new git exec site adds one row to `internal/lint/ratchet_argv_table_test.go` (`go test ./internal/lint/... -run TestArgvIDOperandGated`). New sites: merge-tree with conflicted paths (Bead 1), `RemoteConfigured` and `PushProbe` (Bead 2), stage-resume reads (Bead 3), `DiffNameOnlyNoRenames` (Bead 4), the range revert check (Bead 8).
- **ID-render and composition ratchets.** A bead that adds, removes or moves an `fmt.*` render of a spec or bead ID, or a branch composition, routes it through `idrender` or edits the matching count. The counts are in `rawIDRenderAllowlist`, `inlineCompositionAllowlist` and `joinWithIDAllowlist` (`internal/lint/ratchet_inline_allowlist_test.go`) and `compositionCallAllowlist` (`internal/lint/ratchet_composition_allowlist_test.go`). Each bead that edits an allowlisted function lists both files: Beads 1, 2, 3, 4, 5, 7, 8 and 9.
- **Recovery operands and destructive guidance.** Every bead that adds a recovery line runs `go test ./internal/lint/... -run 'TestProductDiagnosticScan|TestDestructiveGuidanceSweep'` and `go test ./internal/guard/... -run TestKnownSitesExemptionList`. A guidance line with a floor-family command adds one `KnownSitesExemptionList` entry and raises `wantKnownSitesLiveCount` by one. That entry also adds a `[known_sites_exemption_list]` line (surface, exact text, count, family) to `internal/lint/testdata/destructive_seed_manifest.txt` and a `backgroundJustifiedWideningExemptions` row in `internal/lint/destructive_guidance_test.go`, and raises that list's count sentinel by one. Each new `guard.OpaqueOperandRegistry` entry adds an `[opaque_operand_registry]` line (file, func, detail) to the same manifest. Every bead that edits `internal/guard/registries.go` runs `go test ./internal/lint/... -run TestBootstrapManifest_`.
- **Doc-sync.** `checkInternalPackages` (`internal/validate/docsync.go`) maps each changed non-test source under `internal/` or `cmd/` to its domain through `OWNERSHIP.yaml`. It refuses unless a doc under `.mindspec/domains/<domain>/` changes in the same range. Domains: core (`internal/config`, `internal/phase`), execution (`internal/executor`, `internal/gitutil`, `internal/harness`), workflow (`cmd/`, `internal/complete`, `internal/approve`, `internal/lifecycle`, `internal/validate`, `internal/doctor`, `internal/next`, `internal/guard`). Each bead names its doc per domain. Bead 1 edits spec 129 `spec.md` and `plan.md` together with ADR-0036, so `validateSpecArtifactSync` passes.
- **Union merge.** Parallel beads add rows to the argv ratchet table, the operand registry and `KnownSitesExemptionList`, subsections to the ADR-0041 `## Amendment (Spec 132)` section, entries to the Executor interface and `MockExecutor`, and sections to the domain docs, `runbook.md` and `merge-safety.md`. Parallel beads also add lines to `destructive_seed_manifest.txt` and rows to `backgroundJustifiedWideningExemptions` (Beads 5 and 8), and edit `cmd/mindspec/verb_output_test.go` (Beads 6 and 9), `cmd/mindspec/ceremony_guard_test.go` (Beads 5 and 6), `internal/lifecycle/scan.go` (Beads 4, 8 and 9) and the two ratchet allowlist files. A merge conflict in these keeps every row, entry, line and section, and one `## Amendment (Spec 132)` heading. A conflict on `wantKnownSitesLiveCount` or on the widening-list count sentinel takes the base value plus each bead's added entries. A conflict on a ratchet allowlist count takes the base value plus each bead's delta. A bead that creates the amendment heading checks first whether it exists at the spec tip.
- **Integration with `origin/main`.** See § Operator Rulings and Lane Constraints, "When `origin/main` comes in". Per-bead checks run on the spec branch; the full suite runs again after that one merge.
- **Real verification.** A Verification line counts only when `go test <pkg> -list '<regex>'` prints each test that the bead names. `[no tests to run]` is a failure. A scanner test has a positive control: a planted violation turns it red.
- **Every bead** ends with `gofmt -l .` (empty output), `go build ./...`, `go test ./internal/lint/... -run 'TestEnforcementHasNoGitLeaks|TestArgvIDOperandGated|TestRawIDRenderForbidden|TestInlineBranchCompositionForbidden|TestJoinWithIDForbidden|TestWorkspaceCompositionCallSiteAllowlist'` and `go test ./... -short -timeout 20m`.
- **Opt-in, separate:** the live-agent harness suite for the finalize scenarios. It is not a bead gate.

## Bead 1: Ownership at the landing tree

**Steps**
1. Add `gitutil.MergeTreeLanding(workdir, base, ours, theirs string) (tree string, conflicted []string, err error)` in `internal/gitutil/neteffect.go`. It runs `git merge-tree --write-tree --name-only -z --merge-base=<B>` and returns the output tree and the conflicted paths, also when paths conflict. Add its argv ratchet row. Wrap it in Executor method `LandingTree(workdir, base, specTip, beadRef string)` and in `MockExecutor`. For the materialize leg, wrap tree T in a commit that no ref names when merge-tree needs a commit.
2. In `runCommittedRangeGate` (`internal/complete/complete.go`), compute B as `git merge-base <spec tip> <bead HEAD>` and pass the landing tree as `ownerRef` to `ValidateDocsRange` and `CheckADRDivergence`. Do the same in the materialize leg (`internal/complete/materialize.go`) for `ValidateDocsSnapshot` and `CheckADRDivergenceSnapshot`; add an `ownerRef` parameter to `CheckADRDivergenceSnapshot` (`adr_divergence.go`), which has none, and stop passing the tree OID as `ownerRef` in `ValidateDocsSnapshot` (R1.1). Change the ownership-ref assertions to the landing tree in `TestValidateDocsSnapshot_AttributionReadsTreeOID` and `TestValidateDivergenceSnapshot_UsesChangedFilesTreeAndForcedOwnerRef` (`snapshot_leg_test.go`) and in `TestPerBeadGatesAnchorOwnershipRefAndRangeToBeadHead` (`complete_test.go`); the diff range stays `<fork>..<bead HEAD>`.
3. Make all six readers use that one ref: Impacted-Domains normalization, domain enumeration and path attribution (`internal/validate/divergence.go`), the ADR domain-resolving store (`adr_domain_resolve.go`), `checkInternalPackages` and `checkUnclaimedSource` (`docsync.go`). Remove any reader that reads a second ref (R1.2, R1.3).
4. When paths conflict and `OWNERSHIP.yaml` is not in the conflicted list, read it from the merge-tree output tree; `MergeInto` then runs and keeps the conflict (R1.4). When `OWNERSHIP.yaml` is in the conflicted list, the pre-merge gate does not read it: return the conflict to the caller. Bead 5 decides per path: path 7 refuses; `--merge-only` and path 5 run `MergeInto`, which keeps the conflict, and the gate runs at re-entry (step 5).
5. On `--resolve-merge` re-entry, use `git write-tree` of the spec worktree index as the landing tree. Keep today's unresolved-conflict refusal while unmerged entries remain (R1.5). Bead 5 owns the hand-committed-merge case (R3.13).
6. Remove any ADR number or bead ID left in `materialize.go` refusal text (AC-21).
7. Docs: append `## Amendment (Spec 132)` to ADR-0036 (landing-tree ownership). It states that it narrows the ADR-0041 snapshot-gate clause "every OWNERSHIP/content read anchored at `T` too": the ownership read uses the landing tree, and the diffed content stays at `T`. In `.mindspec/domains/workflow/interfaces.md`, update the `validate.ValidateDocsSnapshot(...)` row (new `ownerRef`) and the `validate.CheckADRDivergenceSnapshot` / `ValidateDivergenceSnapshot` row, which says "`ownerRef` is FORCED to the same `treeOID`". Add the R13.2 dated status notes to spec 129 `spec.md`. In spec 129 `plan.md`, set `-timeout=20m` on every `go test ./... -short` suite command. Update `.mindspec/domains/workflow/architecture.md` (ownership ref) and `.mindspec/domains/execution/interfaces.md` (`MergeTreeLanding`, `LandingTree`). Add `internal/validate/adr_spec132_ownership_test.go` (`TestDocs_Spec132Amendments_Ownership`). It pins the ADR-0036 section, its sentence that narrows the ADR-0041 clause, and each spec 129 note. It also asserts that spec 129 `plan.md` has no `go test ./... -short` command without `-timeout=20m`.

**Verification**
- [ ] `go test ./internal/complete/... -run 'TestCommittedRangeGate_SpecTipClaimOwns|TestMaterialize_SpecTipClaimOwns|TestCommittedRangeGate_RevokedAtSpecTipRefuses|TestCommittedRangeGate_SourceConflictPassesThenResolves|TestMaterializeText_NoHistoricalCitations'` passes
- [ ] `go test ./internal/validate/... -run 'TestCheckUnclaimedSource_LandingTree|TestDocs_Spec132Amendments_Ownership|TestValidateDocsSnapshot_|TestValidateDivergenceSnapshot_'` and `go test ./internal/complete/... -run TestPerBeadGatesAnchorOwnershipRefAndRangeToBeadHead` pass
- [ ] `go test ./internal/gitutil/... -run 'TestMergeTreeLanding_CleanReturnsTree|TestMergeTreeLanding_ConflictListsPaths'` passes
- [ ] `go test ./internal/executor/...` passes
- [ ] `mindspec validate spec 129-readiness-tracker-honesty` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-1 (R1.1–R1.3): `TestCommittedRangeGate_SpecTipClaimOwns`, `TestMaterialize_SpecTipClaimOwns`, `TestCommittedRangeGate_RevokedAtSpecTipRefuses`, `TestCheckUnclaimedSource_LandingTree` pass.
- [ ] AC-2 (R1.4 source-conflict leg, R1.5): `TestCommittedRangeGate_SourceConflictPassesThenResolves` passes.
- [ ] AC-21 (materialize part): `TestMaterializeText_NoHistoricalCitations` passes.
- [ ] AC-26 (R13.1 ADR-0036, R13.2): the pin passes for the ADR section, the spec 129 notes and the spec 129 plan timeout.

**Depends on**
None.

## Bead 2: `impl approve` preflight and one has-remote decision

**Steps**
1. Add `gitutil.RemoteConfigured() (bool, error)`: a failing `git remote` returns `(false, err)`. Expose it through a `lifecycle` wrapper and make `implHasRemoteFn` return `(bool, error)`. Migrate the bool-only stubs in `internal/approve/main_test.go` and `merge_preflight_ac5_test.go`. `gitutil.HasRemote()` and its other callers do not change (R4.4a).
2. In `ApproveImpl` (`internal/approve/impl.go`), decide has-remote one time, after the EnsureMigrated exemption and before the branch-existence switch (`implSpecBranchExistsFn`, `implBranchMissingRefusal`). Thus the decision also precedes the `--supersede-adr` placeholder write, the phase-reconcile write and MUTATION 1/3, and Bead 3's "already landed" path reuses it. A probe error refuses: `cannot tell whether a git remote is configured: <err>`, then `recovery: git remote -v`. An error never selects local merge (R4.3, R4.4a).
3. Append a `hasRemote bool` parameter to `FinalizeEpic` in the Executor interface, `MindspecExecutor` and `MockExecutor`. Remove the `gitutil.HasRemote()` call in `FinalizeEpic`; `result.MergeStrategy` follows the passed value. Update every call site and the argument-count assertions (`len(calls[0].Args) != 6` in `impl_test.go` and `merge_preflight_ac5_test.go`) to 7. Update the two-probe comments on `HasRemote` (`internal/lifecycle/gitquery.go`) and on `implHasRemoteFn` (R4.4).
4. Add Executor method `PreflightFinalizeMerges(epicID, specID, specBranch string, lifecycleAllowSet []string, overrideReason string, hasRemote bool) error`. It enumerates the merges that `FinalizeEpic` will make, with the same helper. Push path: every pending bead-to-spec auto-merge. Direct path: those, plus spec-to-main. For each merge it calls `guardMergeLayout` and `preflightMergeDestruction` with the same (branch, target, allow-set, override) arguments as `FinalizeEpic`, and it mutates nothing. `ApproveImpl` calls it in preflight, in place of today's direct-only `implWorkDestructionPreflightFn` call. The executor guards stay in front of every `MergeInto` and `MergeBranch` call (R2.1, R2.2). Delete the `implWorkDestructionPreflightFn` seam, its stub in `main_test.go` and `TestImplWorkDestructionPreflightFn_DeclaredDefaultIsRealImplementation`. In `TestOpaqueOperandRegistry_MergePreflightRerunCallers` (`internal/guard/registries_test.go`), remove the `impl.go` case: the `mindspec_executor.go` case already covers the `preflightMergeDestruction` call in `PreflightFinalizeMerges`. Add `PreflightFinalizeMergesErr` and a call record to `MockExecutor`. Rewrite the `TestApproveImpl_AC5_*` tests to set that error and read that record: `_StaleSpecBranchRefusesBeforeMutation` and `_StillAppliesWithNoRemoteConfigured` assert a refusal before any mutation with `hasRemote` false; `_SkippedWhenRemoteConfigured` becomes `_PushPathPreflightsBeadMergesOnly` and asserts one call with `hasRemote` true. `TestApproveImpl_AC7_AllowNetDeletionOverrideCompletes` asserts that the override reason reaches the mock. In `rawIDRenderAllowlist`, the count for `internal/approve/impl.go ApproveImpl fmt.Sprintf(...specID...)` goes from 8 to 7, plus or minus any new site.
5. Add Executor method `PushProbe(branch string) error`. It runs `git push --dry-run --no-verify` to the remote and refspec of `PushBranch`, with `GIT_TERMINAL_PROMPT=0`. When has-remote is true, `ApproveImpl` calls it before MUTATION 1/3. Only a failed probe refuses. Add a credential-helper hint only for an authentication error on an HTTP(S) remote (R2.3–R2.5).
6. Add `internal/executor/hasremote_callsite_test.go` (`TestExecutor_HasRemoteOnlyInSpecBranchBase`). It parses the non-test sources of `internal/executor` and fails on any `gitutil.HasRemote` or `gitutil.RemoteConfigured` call outside `specBranchBase`. Its positive control is a planted call in a parsed fixture.
7. Docs: under ADR-0041 `## Amendment (Spec 132)`, add the subsection for early guards, the push probe and the one has-remote decision. Update `.mindspec/domains/workflow/architecture.md` (`impl approve` preflight order), `.mindspec/domains/execution/interfaces.md` (`FinalizeEpic` parameter, `PreflightFinalizeMerges`, `PushProbe`, `RemoteConfigured`) and `.mindspec/domains/execution/architecture.md` (finalize no longer probes). Add `internal/validate/adr_spec132_preflight_test.go` (`TestDocs_Spec132Amendments_Preflight`).

**Verification**
- [ ] `go test ./internal/approve/... -run 'TestApproveImpl_EarlyLayoutGuardRefuses|TestApproveImpl_EarlyDestructionRefuses|TestApproveImpl_EarlyGuardBeforePhaseReconcile|TestApproveImpl_PushProbeAuthFailureNamesHelper|TestApproveImpl_PushProbeNonFastForwardNoHelperHint|TestApproveImpl_PushProbeFileRemoteNoHelperPasses|TestApproveImpl_OneHasRemoteDecision|TestApproveImpl_HasRemoteProbeErrorRefuses|TestApproveImpl_AC5|TestApproveImpl_AC7|TestApproveImpl_FinalizeEpicReceivesIntersectedAllowSet'` passes
- [ ] `TestApproveImpl_EarlyLayoutGuardRefuses` and `TestApproveImpl_EarlyDestructionRefuses` each have a push-path subtest (has-remote true, a bead-to-spec merge fails the guard) and a direct-path subtest; each asserts zero mutations: no close, no phase write, no placeholder ADR, no `FinalizeEpic` call
- [ ] `TestApproveImpl_OneHasRemoteDecision` counts `implHasRemoteFn` calls as exactly one on the push path, the direct path and the spec-branch-missing refusal path
- [ ] `go test ./internal/executor/... -run 'TestFinalizeEpic_ExecutorGuardAfterAutoMerge|TestPushProbe_ArgvMatchesPushBranch|TestPreflightFinalizeMerges_MatchesFinalizeMerges|TestFinalizeEpic_WithRemotePushes|TestFinalizeEpic_NoRemoteMergesLocally|TestFinalizeEpic_PassedFalseWithRemoteMergesLocally|TestFinalizeEpic_PassedTrueNoRemoteTakesPushLeg|TestExecutor_HasRemoteOnlyInSpecBranchBase'` passes
- [ ] `go test ./internal/gitutil/... -run 'TestRemoteConfigured_WithRemote|TestRemoteConfigured_NoRemote|TestRemoteConfigured_GitErrorReturnsError'` passes, and `-list 'TestRemoteConfigured_'` prints those three tests. The error case runs outside a git repository and asserts `(false, non-nil)`.
- [ ] `go test ./internal/validate/... -run TestDocs_Spec132Amendments_Preflight` and `go test ./internal/lint/... -run TestRawIDRenderForbidden` pass
- [ ] `go test ./internal/lint/... -run TestBootstrapManifest_` and `go test ./internal/guard/...` pass
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-3 (R2.1): a layout-guard failure and a destruction failure each exit non-zero with the epic open and the phase unchanged, on the push path and on the direct path, also from a stale-phase fixture.
- [ ] AC-4 (R2.2): `TestFinalizeEpic_ExecutorGuardAfterAutoMerge` passes.
- [ ] AC-5 (R2.3–R2.5): the three push-probe approve tests and `TestPushProbe_ArgvMatchesPushBranch` pass.
- [ ] AC-15 (R4.3, R4.4, R4.4a): the four `TestFinalizeEpic_` remote tests, `TestApproveImpl_OneHasRemoteDecision` and `TestApproveImpl_HasRemoteProbeErrorRefuses` pass. An injected probe error with `--supersede-adr` leaves the epic open, the phase unchanged and no placeholder ADR. A real `git remote` failure returns an error (`TestRemoteConfigured_GitErrorReturnsError`).
- [ ] AC-21 (R4.4a part): `TestApproveImpl_HasRemoteProbeErrorRefuses` asserts that the refusal matches none of `ADR-[0-9]`, `q9ea`, `[Ss]pec [0-9]{3}`.

**Depends on**
None.

## Bead 3: `impl approve` state line, finalize tip and re-run

**Steps**
1. After a failure past MUTATION 1/3, read the epic status and print one state line before the inner error: "epic closed" or "epic still open", plus the failed finalize stage. Keep the inner recovery lines last. Add `recovery: mindspec impl approve <spec>` only when the inner error has none (R2.6–R2.8).
2. In `FinalizeEpic`, write epic metadata `mindspec_finalize_tip` after the export commit and all bead auto-merges, immediately before the spec-to-main merge or the spec-branch push. Every run overwrites it. Keep the pre-finalize tip record separate (R2.9).
3. Make each stage converge on re-run, on both paths, per the R2.10 table: idempotent close; skip merged beads; push no-op; reuse `chore/finalize-<id>` and push it with `--force-with-lease` on the SHA that `git ls-remote` reads just before the push; skip a direct merge when the spec tip is an ancestor of main; finish remaining cleanup. Open no PR (R2.10). Update `TestFinalizeFault_*` (`finalize_fault_test.go`, for example the "spec branch spec/077-test does not exist" refusal) and `finalize_orphan_test.go` where a stage now converges.
4. While the spec branch exists, a re-run runs every pre-close gate again on the current spec tip (R2.11).
5. Add the "already landed" re-run per R2.12, in the branch-missing case of the branch-existence switch. It reuses the one has-remote value that Bead 2 decides before that switch, with no second probe, to pick the target: main (direct), the remote spec branch (push), or remote main when the host deleted the spec branch. Fetch the target into a temporary ref. A fetch failure, a missing object, or a squash or rebase merge refuses with today's branch-missing recovery. Cleanup runs the destruction guard again and deletes only bead branches whose tips are ancestors of the target. With no landed evidence, `TestApproveImpl_MissingSpecBranchRefusesBeforeMergeBase` (`impl_test.go`) keeps the `mindspec impl adopt` refusal and no `FinalizeEpic` call; update its `implHasRemoteFn` stub (`main_test.go`).
6. Export a test setter `executor.SetFinalizeStepHookForTest(fn func(stage string) error) (restore func())` in `mindspec_executor.go`. `TestApproveImpl_RerunConvergesFromEachStage` uses it with the real `MindspecExecutor`, real git and a `file://` remote, not `MockExecutor.FinalizeEpicErr`.
7. Docs: under ADR-0041 `## Amendment (Spec 132)`, add the subsection for the state line, `mindspec_finalize_tip` and stage resume. Update `.mindspec/domains/workflow/architecture.md` and `runbook.md` (re-run recovery), and `.mindspec/domains/execution/architecture.md` and `interfaces.md` (finalize tip, stage resume, the test setter). Add `internal/validate/adr_spec132_finalize_test.go` (`TestDocs_Spec132Amendments_Finalize`).

**Verification**
- [ ] `go test ./internal/approve/... -run 'TestApproveImpl_PostCloseConflictKeepsInnerRecovery|TestApproveImpl_StateLineReadsEpicStatus|TestApproveImpl_PostCloseBareErrorGetsRerun|TestApproveImpl_RerunConvergesFromEachStage|TestApproveImpl_RerunReRunsGates|TestApproveImpl_AfterDeletionAlreadyLanded|TestApproveImpl_RemoteSpecBranchDeletedUsesMain|TestApproveImpl_TargetFetchFailureRefuses|TestApproveImpl_ForgedFinalizeTipRefuses'` passes
- [ ] `TestApproveImpl_AfterDeletionAlreadyLanded` and `TestApproveImpl_RemoteSpecBranchDeletedUsesMain` assert exactly one `implHasRemoteFn` call
- [ ] `go test ./internal/executor/... -run 'TestFinalizeEpic_FinalizeTipIsPublishedTip|TestFinalizeFault_|TestFinalizeEpic_Orphan|TestFinalizeOrphanedSpecBranch_'` and `go test ./internal/approve/... -run 'TestApproveImpl_MissingSpecBranchRefusesBeforeMergeBase|TestApproveImpl_BranchExistsProbeErrorRefusesDistinctly'` pass
- [ ] `go test ./internal/validate/... -run TestDocs_Spec132Amendments_Finalize` passes
- [ ] `go test ./internal/lint/... -run TestBootstrapManifest_` and `go test ./internal/guard/...` pass
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-6 (R2.7, R2.8): the three state-line tests pass.
- [ ] AC-7 (R2.9, R2.10): `TestApproveImpl_RerunConvergesFromEachStage` covers every `finalizeStepHookFn` stage, `orphan_finalize` included, on both paths, with one close, exit 0 and at most one push per remote ref per converged state. `TestFinalizeEpic_FinalizeTipIsPublishedTip` passes.
- [ ] AC-8 (R2.11, R2.12): the five re-run tests pass. A forged `mindspec_finalize_tip` refuses and the bead branch survives.

**Depends on**
Bead 2. It needs the `FinalizeEpic` has-remote parameter and the one has-remote value, decided before the branch-existence switch, to pick the R2.12 target.

## Bead 4: Merge-state rules and doctor severity

**Steps**
1. Add `gitutil.DiffNameOnlyNoRenames(workdir, base, head string) ([]string, error)` in `internal/gitutil/gitops.go`. It runs `git diff --no-renames --name-only -z <base> <head>` and splits on NUL. Add its argv ratchet row. Beads 4 and 7 use it.
2. Add `internal/lifecycle/merge_state.go`. It finds the bead's naming merges on the spec branch's first-parent chain and decides a valid merge M (spec Terms). It runs the R3.5 presence check over `DiffNameOnlyNoRenames(M^1, M)`, minus tool-artifact paths. A path is reverted when its content at the spec tip equals M^1, or when M added it and it is absent at the spec tip. A later naming merge for another bead that changed the path exempts it. The valid-merge test and the presence check are separate functions, so a caller can use one without the other.
3. Add the R3.7 restore-recovery builder: each later commit that changed a reverted path, newest first, with full SHA, escaped subject and M paths; `git -C <spec worktree> revert <commit>`, with `-m 1` for a merge; the path-scoped checkout choice; then the R3.6 lines from step 2 on. The builder takes a `guard.DestructionOutcome` from its caller and builds the checkout line only through `guard.NewDestructiveCommand(cmd, outcome)`. The outcome comes from the spec-worktree user-changes check (R3.4): no user changes gives `DestructionClean`; a read error gives `DestructionEvidenceError`. When that check finds user changes or gives an error, the builder leaves out the checkout choice and keeps the revert lines. Register substituted operands in `guard.OpaqueOperandRegistry`.
4. Add doctor status `Info` (`internal/doctor/doctor.go`). It does not count in `HasFailures` and does not change the exit code. `Fix` takes no action on Info. `cmd/mindspec/doctor.go` renders it as `[INFO]` (R3.11).
5. Make the stale-open finding (`internal/lifecycle/stale_open.go`, `internal/doctor/lifecycle_integrity.go`) Info with no FixFunc and recovery `mindspec complete <bead>` when a valid merge exists. Without one it stays Error, so `TestRunWithOptions_SkipLocalEnv_StillFailsOnRealDivergence` (`skip_local_env_test.go`), whose fixture has no valid merge, still sees Error and `HasFailures`. The merge-only marker does not change severity (R3.11).
6. Report, at Error, an open bead whose naming merge is fully or partly reverted, with the R3.7 recovery (the doctor evaluates the user-changes outcome read-only). Report no stale-open finding for that bead, give it no FixFunc, and give no finding for a closed bead (R3.12).
7. Docs: update `.mindspec/domains/workflow/architecture.md` (git-state severity rule), `interfaces.md` (doctor Info status) and `.mindspec/domains/execution/interfaces.md` (`DiffNameOnlyNoRenames`).

**Verification**
- [ ] `go test ./internal/doctor/... -run 'TestDoctor_StaleOpenSeverityFromGitState|TestDoctor_RevertedMergeOnlyOneFinding|TestDoctor_FixSkipsRevertedMergeOnly|TestDoctor_ClosedRevertedNoFinding|TestDoctor_FixSkipsStaleOpenInfo|TestRunWithOptions_SkipLocalEnv_StillFailsOnRealDivergence'` passes
- [ ] `go test ./cmd/mindspec/... -run TestDoctorStatusTag_Info` passes
- [ ] `go test ./internal/lifecycle/... -run 'TestMergeState_LaterMergeExemptsPath|TestMergeState_LaterEditNotRevert|TestMergeState_RenameInMergeRestoredOldPathIsReverted|TestMergeState_RestoreCheckoutFromConstructor|TestMergeState_RestoreOmitsCheckoutOnUserChanges'` passes. In the rename fixture, M renames `a` to `b`, and a later commit restores `a` with its M^1 content. The presence check reports `a` reverted. The test fails when the path list comes from `gitutil.DiffNameOnly` (rename detection on), because that list has only `b`.
- [ ] `go test ./internal/gitutil/... -run 'TestDiffNameOnlyNoRenames_RenameListsBothPaths|TestDiffNameOnlyNoRenames_NULSeparated'` passes
- [ ] `go test ./internal/lint/... -run 'TestProductDiagnosticScan|TestDestructiveGuidanceSweep'` passes
- [ ] `go test ./internal/lint/... -run TestBootstrapManifest_` and `go test ./internal/guard/...` pass
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-13 (R3.11, R3.12): the six AC-13 tests pass. A stale-open bead with a valid merge gives Info, and `doctor --fix` leaves it open.
- [ ] The R3.5 presence check and the valid-merge rule exist as separate lifecycle functions that Bead 5 calls. `TestMergeState_RestoreCheckoutFromConstructor` fails when the checkout line is built by string concatenation outside `guard.NewDestructiveCommand`.

**Depends on**
None.

## Bead 5: `complete --merge-only` and the closing path

**Steps**
1. Add `--merge-only` to `mindspec complete` (`cmd/mindspec/complete.go`, `CompleteOpts`). Implement R3.0 path selection in the stated order (paths 1–7), with the marker check on paths 2, 4 and 6 and recovery `bd update <bead> --unset-metadata mindspec_merge_only_sha`. Add `--merge-only` to the complete flag set in `TestCeremonyNonInflation_HelpFlags` (`ceremony_guard_test.go`) and a `mindspec complete --merge-only` row to `namedInvocations` (`named_invocation_test.go`).
2. Merge-only path (`internal/complete/merge_only.go`): run every gate, materialize, write `mindspec_merge_only=pending`, merge, print the merge SHA and both parents, then replace the pending key with `mindspec_merge_only_sha=<merge>`. Keep the bead open with no binding, ledger commit or cleanup. A conflict recovery is `mindspec complete <bead> --merge-only --resolve-merge`. A repeated run with a valid merge for the tip writes the marker only. Refuse a second naming merge (R3.1–R3.4, R3.10).
3. Use Bead 1's conflicted-path list. When it has `OWNERSHIP.yaml`: plain `complete` on an open bead (path 7) refuses before the close with `recovery: mindspec complete <bead> --merge-only`. `--merge-only` and path 5 do not run the pre-merge ownership read and do not refuse: `MergeInto` keeps the conflict, and the gate runs on `--resolve-merge` re-entry against the `write-tree` of the resolved index (R1.4, R1.5).
4. Open closing path (path 6, `internal/complete/closing.go`): refuse on a busy spec worktree, a moved tip, user changes, an invalid merge, or a failed presence check (Bead 4). Closed resume (path 4): keep naming-merge identity, the tip and user-changes checks, the full panel gate at the bead tip and the R3.13 gates, but do not call the presence check (R3.0 path 4). Both paths run the doc-sync, ADR and ownership gates on M (base M^1, head M, `ownerRef` M's tree), then steps 1–6 of R3.8, each a no-op when its done signal holds. Step 4 is a path-scoped commit of the export and panel ledger in the spec worktree (R3.4, R3.8, R3.13).
5. Implement the path-4 cleanup guard for a layout that is not flat, the refusal texts of R3.6 and R3.7 (Bead 4 builder, with the outcome from this path's user-changes check), the step 1–4 failure recovery that repeats `--allow-doc-skew` or `--override-adr` with its reason, and the R3.13 exits (R3.6, R3.7, R3.9, R3.13).
6. Put the AC-10 loop tests in `cmd/mindspec/merge_only_loop_test.go`, because `internal/approve` imports `internal/complete`. They call `complete.Run`, `next`, `doctor` and `approve.ApproveImpl` in one real temp repo with fake `bd` and a local helper.
7. Docs: under ADR-0041 `## Amendment (Spec 132)`, add the subsection for `--merge-only` as a commit phase with no close, and for the closing call. Update `.mindspec/domains/workflow/architecture.md`, `interfaces.md` (`--merge-only`, metadata keys) and `runbook.md`, and `project-docs/user/guides/merge-safety.md`: `--merge-only`, the one-naming-merge rule, "do not revert a `--merge-only` merge by hand" and the restore-then-close recovery (R13.3). Each `merge-safety.md` line with the path-scoped checkout gets a `KnownSitesExemptionList` entry, and `wantKnownSitesLiveCount` rises by the same number. If the bead adds an Executor method, update `.mindspec/domains/execution/interfaces.md`. Add `internal/validate/adr_spec132_mergeonly_test.go` (`TestDocs_Spec132Amendments_MergeOnly`).

**Verification**
- [ ] `go test ./internal/complete/... -run 'TestComplete_OwnershipConflictRefusesBeforeClose|TestComplete_OwnershipConflictMergeOnlyLoop|TestComplete_OwnershipResolutionRevokesClaimRefuses|TestComplete_ClosedNoMergeOwnershipConflictDefersToReentry'` passes
- [ ] `go test ./internal/complete/... -run 'TestComplete_ClosedBeadMovedTipRefuses|TestComplete_ClosedBeadUserDirtRefuses|TestComplete_ClosedMergedForeignMergeInProgressRefuses|TestComplete_ClosingPathGatesMergeRange|TestComplete_ClosedHandMergeStalePanelRefuses|TestComplete_ClosedHandMergeNoPanelFailsOpen|TestComplete_ClosedHandMergeForgedReviewedKeyRefuses|TestComplete_ClosedNonFlatAbsentWorktreeRefuses|TestComplete_ClosedNonFlatAbsentWorktreeGateOffAllows|TestComplete_ClosedOwnMergeInProgressAborts|TestComplete_MergeOnlyBranchlessRefuses|TestComplete_PendingKeyPlainCompleteStaysOpen|TestComplete_MergeOnlyKeepsBeadOpen|TestComplete_MergeOnlyHandCommitStaysOpen|TestComplete_PlainConflictResolveFinishes|TestComplete_ClosedUnmergedRerunMerges|TestComplete_HandMergeTakesClosingPath|TestComplete_ForeignMarkerRefuses|TestComplete_BranchlessReconcileUnchanged|TestComplete_ClosedBeadResumesAllSteps|TestComplete_MergeOnlyConflictStaysMergeOnly'` passes
- [ ] `go test ./internal/complete/... -run TestComplete_ClosedBeadLaterRevertResumesCleanup` passes
- [ ] `go test ./internal/complete/... -run 'TestComplete_BeadAdvancedAfterMergeOnlyRefuses|TestComplete_UserDirtAfterMergeOnlyRefuses|TestComplete_RevertedMergeOnlyRefuses|TestComplete_PartlyRevertedMergeOnlyRefuses|TestComplete_PresenceCheckLaterMergeExemptsPath|TestComplete_PresenceCheckLaterEditNotRevert|TestComplete_SecondMergeOnlyRefuses|TestComplete_MergeOnlyRefusalEscapes'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestComplete_MergeOnlyFollowUpLoop|TestComplete_MergeOnlyRestoreLoop|TestCeremonyNonInflation_HelpFlags|TestNamedInvocations_'` passes
- [ ] `go test ./internal/complete/... -run 'TestComplete_AfterMergeOnlyPublishesClose|TestComplete_ClosingPathSpecWorktreeBusyRefuses|TestComplete_AfterMergeOnlyExportFailureConverges|TestComplete_ClosedBeadFinishesCleanup|TestComplete_OverrideRunRecoveryRepeatsFlag|TestComplete_RepeatedMergeOnlyNoNewMerge'` passes
- [ ] `go test ./internal/lint/... -run 'TestProductDiagnosticScan|TestDestructiveGuidanceSweep'` and `go test ./internal/guard/... -run TestKnownSitesExemptionList` pass
- [ ] `go test ./internal/validate/... -run TestDocs_Spec132Amendments_MergeOnly` passes
- [ ] `go test ./internal/lint/... -run TestBootstrapManifest_` and `go test ./internal/guard/...` pass
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-2 (R1.4 ownership-conflict leg, R3.13): the three ownership-conflict tests pass. The bead stays open on the refusal, and the amend exit passes. `TestComplete_ClosedNoMergeOwnershipConflictDefersToReentry` shows that path 5 does not refuse on a conflicted `OWNERSHIP.yaml`.
- [ ] AC-9 (R3.0–R3.3, R3.13): the 21 AC-9 tests pass.
- [ ] AC-10 (R3.4–R3.7): the 10 AC-10 tests pass. The follow-up and restore loops end with `doctor` clean for the bead, dependents passing MF-3, and `impl approve` exit 0.
- [ ] AC-11 (R3.8, R3.9): the five AC-11 tests pass.
- [ ] AC-12 (R3.10): `TestComplete_RepeatedMergeOnlyNoNewMerge` passes.
- [ ] Closed resume skips the presence check: `TestComplete_ClosedBeadLaterRevertResumesCleanup` (a closed bead, cleanup interrupted at step 6, then a later commit reverts an M path) exits 0 and finishes steps 2–6.

**Depends on**
Bead 1 (the landing tree and its conflicted-path list for the R1.4 decisions) and Bead 4 (the valid-merge, presence-check and restore-recovery functions).

## Bead 6: Remove `merge_strategy`

**Steps**
1. Delete `config.MergeStrategy`, its `auto` default, its defaulting and its mention in the defaulting comment (`internal/config/config.go`). Delete the `config show` row (`cmd/mindspec/config.go`). Keep `FinalizeResult.MergeStrategy` and its readers (R4.1).
2. Update the field assertions in `internal/config/config_test.go` and `internal/config/feedback_remote_test.go`. Use `worktree_root` with two valid relative values in `TestLoadCaches_DiskMutationIgnored`, and a retained project-config field in the feedback-remote scope test. Remove `merge_strategy` from `TestCeremonyNonInflation_ConfigKeys` (R4.1).
3. Remove the key from `internal/harness/sandbox.go`, `internal/harness/lifecycle_scenario_test.go`, `cmd/mindspec/verb_output_test.go`, `internal/doctor/config_test.go` and `internal/doctor/models_commands_test.go`. Keep it only in named legacy-key fixtures. Remove `merge_strategy: pr` from `.mindspec/config.yaml`. Keep the `scenario_finalize_pr` push leg (R4.2, R4.7).
4. Add a doctor check in `internal/doctor/config.go`: one Info (Bead 4 status) when `.mindspec/config.yaml` has a top-level `merge_strategy` key, by line regex like the existing key checks. A commented or indented key gives nothing (R4.5, R4.6).
5. Add `internal/lint/no_merge_strategy_writers_test.go` (`TestNoMergeStrategyWriters`). It scans Go sources, test fixtures and `.mindspec/config.yaml`. Its exemptions are named test functions or fixture constants, not files. Its positive control, `TestNoMergeStrategyWriters_PlantedWriterFails`, runs the scanner over a temp tree with one planted writer in a Go source, one in a fixture and one in `config.yaml`, and asserts three findings.
6. Docs: update the config example in `.mindspec/core/GIT-WORKFLOW.md`, the Config description in `.mindspec/domains/core/interfaces.md`, `.mindspec/domains/workflow/interfaces.md` (`config show`, the doctor Info) and `.mindspec/domains/execution/architecture.md` (sandbox config). Add dated removal notes at the `merge_strategy` guidance in ADR-0006 and ADR-0019. Add `internal/validate/adr_spec132_mergestrategy_test.go` (`TestDocs_Spec132Amendments_MergeStrategy`).

**Verification**
- [ ] `go test ./internal/config/... -run 'TestConfigLoad_LegacyMergeStrategyIgnored|TestLoadCaches_DiskMutationIgnored'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestConfigShow_NoMergeStrategyRow|TestCeremonyNonInflation_ConfigKeys'` passes
- [ ] `go test ./internal/harness/... -run TestHarnessSandbox_NoMergeStrategy` passes
- [ ] `go test ./internal/doctor/... -run 'TestDoctor_LegacyMergeStrategyInfo|TestDoctor_LegacyMergeStrategyCommentedOrNested'` passes
- [ ] `go test ./internal/lint/... -run 'TestNoMergeStrategyWriters|TestNoMergeStrategyWriters_PlantedWriterFails'` passes
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
2. In `FindLandedMerge`, confirm a candidate with second parent D when D is in R, or when some r in R is an ancestor of D and `gitutil.DiffNameOnlyNoRenames(r, D)` (Bead 4) lists no path or only paths in `panel.ToolArtifactPaths` for the spec directory of the bead's own epic (R5.2).
3. Keep tip and binding contradictions and the revert check in force after the datum. Do not evaluate the datum on the reachable-fallback path (R5.3, R5.4).
4. Name the matching rule in the provenance string: `exact` or `reviewed-tree` (R5.5). Keep the `FindLandedMerge` counts in both ratchet allowlist files equal to the renders in the function. The existing non-corroborating tests use an unrelated SHA, not an ancestor, so they stay green.
5. Docs: under ADR-0041 `## Amendment (Spec 132)`, add the subsection that changes amendment (ii): the reviewed-tree rule confirms and never vetoes. In the Spec 129 section (ii), add the inline back-pointer `See [Amendment (Spec 132)](#amendment-spec-132)` next to "The set alone never confirms an ancestor or a fallback." and next to "the panel leg is normally non-corroborating", as the Spec 130 back-pointer in that paragraph does. Keep every pinned phrase. Update `.mindspec/domains/workflow/architecture.md`. Add `internal/validate/adr_spec132_reviewedtree_test.go` (`TestDocs_Spec132Amendments_ReviewedTree`); it pins the new subsection and both back-pointers.

**Verification**
- [ ] `go test ./internal/lifecycle/... -run 'TestFindLandedMerge_ReviewedTreeEmptyWrapperConfirms|TestFindLandedMerge_ReviewedTreeSourceDiffNoConfirm|TestFindLandedMerge_ReviewedTreeRenameNoConfirm|TestFindLandedMerge_ReviewedTreeOtherSpecDirNoConfirm|TestFindLandedMerge_UnregisteredKeyNoConfirm|TestReattestFallback_IgnoresPanelDatum'` passes
- [ ] `go test ./internal/complete/... -run TestFindLandedMerge_ReviewedTreeConfirms` passes (the `complete` reconcile path on a branch-less bead; it fails on the old code)
- [ ] `go test ./internal/validate/... -run 'TestDocs_Spec132Amendments_ReviewedTree|TestADR0041_Spec129Amendment'` and `go test ./cmd/mindspec/... -run ADR0041` pass
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-16 (R5.1–R5.5): the seven AC-16 tests pass, with provenance `reviewed-tree` on the confirming case.
- [ ] AC-26 (R13.1, ADR-0041 (ii)): the new subsection and both back-pointers are pinned, and the Spec 129 pins stay green.

**Depends on**
Bead 4 (`gitutil.DiffNameOnlyNoRenames`).

## Bead 8: Ancestor without a naming merge

**Steps**
1. Add `gitutil.RangeRevertedAt(workdir, base, head, target string) (bool, error)` in `internal/gitutil/neteffect.go`. It reports whether the target reverts any change in base..head. Add its argv ratchet row (R6.5).
2. In `internal/lifecycle/reattest.go`, evaluate R6 only after the primary scan and the reachable fallback find no candidate. Apply R6.2–R6.4: absent branch, non-ancestor tip, empty range, unknown base B, and "not P's own work" when P's tip equals another bead's naming-merge second parent. A git read error refuses as indeterminate (R6.1–R6.4).
3. Check B..P-tip at the spec tip with `ContentSubsumedOutcome` and `RangeRevertedAt`. Content present and not reverted: state `ancestor-without-naming-merge` with the person-must-verify sentence and one `mindspec next <dependent> --allow-not-ready=MF-3 --evidence <file>` line per open dependent in the epic, or `recovery: mindspec doctor` with zero dependents. Otherwise keep `no-owned-exact-merge` with the history line and no "present" claim (R6.5–R6.8, R6.10).
4. Add a read seam to each of the three `fallbackSafety` reads (`internal/lifecycle/fallback.go`). Each read error refuses (R6.11).
5. Add the R6 finding to the lifecycle scan (`internal/lifecycle/scan.go`). Render it in a new `internal/doctor/ancestor.go` at Warn, naming P, with the R6.8 recovery lines (R6.9). `ancestor.go` reads only the scan finding and does not call `ReattestLandedMerge` or `reattestBindingFn` (`TestReattestNotWiredThroughDoctor`).
6. Rewrite the `no-owned-exact-merge` text, its render in `cmd/mindspec/reattest.go` and the reattest `Short` text with no ADR number or bead ID. The no-owned refusal says "attested restore" and that a person must verify the merge first. Recovery lines are commands with no parentheses. Change the "human attested-restore" assertions in `internal/lifecycle/reattest_test.go` (`TestReattest_RefusesTrulyBare`) and `cmd/mindspec/reattest_test.go` (`TestReattestRefusalRendering`, its no-owned-exact-merge leg) to the new text. Keep the ADR-0041 §2(ii) code comments in both `reattest.go` files (`TestADR0041ReattestAmendment_Anchored`).
7. Docs: update `.mindspec/domains/workflow/architecture.md` and `runbook.md`, `.mindspec/domains/execution/interfaces.md` (`RangeRevertedAt`), and `project-docs/user/guides/merge-safety.md` with the attested-restore exit and the R6 states (R13.3). A guidance line with a floor-family command gets a `KnownSitesExemptionList` entry.

**Verification**
- [ ] `go test ./internal/lifecycle/... -run 'TestReattest_Ancestor|TestFallbackSafety_ForwardContentErrorRefuses|TestFallbackSafety_ReverseContentErrorRefuses|TestFallbackSafety_BranchTipErrorRefuses'` passes
- [ ] `go test ./internal/lifecycle/... -list 'TestReattest_Ancestor'` prints these 13 tests: `TestReattest_AncestorWithoutNamingMerge` (AC-17); `TestReattest_AncestorZeroCommitsNoClaim`, `TestReattest_AncestorFirstParentRestoreNoClaim`, `TestReattest_AncestorDeletedBranch`, `TestReattest_AncestorNotInSpecTip`, `TestReattest_AncestorInsideOtherRangeChecksContent`, `TestReattest_AncestorOtherBeadTipNotOwn`, `TestReattest_AncestorEarlierCommitReverted`, `TestReattest_AncestorRevertedHistoryOnly`, `TestReattest_AncestorNoBaseUnknown`, `TestReattest_AncestorReadErrorIndeterminate` (AC-18); `TestReattest_AncestorRecoveryPerDependent`, `TestReattest_AncestorZeroDependents` (AC-19)
- [ ] `go test ./internal/doctor/... -run TestDoctor_AncestorNamesPrerequisite` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestReattestText_NoHistoricalCitations|TestReattestText_OperandsRender|TestReattestNotWiredThroughDoctor|TestADR0041ReattestAmendment_Anchored|TestReattestRefusalRendering'` and `go test ./internal/lifecycle/... -run TestReattest_RefusesTrulyBare` pass
- [ ] `go test ./internal/gitutil/... -run 'TestRangeRevertedAt_DetectsRevertOfRangeChange|TestRangeRevertedAt_LaterEditNotRevert|TestRangeRevertedAt_BaseExcludesEarlierCommits'` passes
- [ ] `go test ./internal/lint/... -run 'TestProductDiagnosticScan|TestDestructiveGuidanceSweep'` and `go test ./internal/guard/... -run TestKnownSitesExemptionList` pass
- [ ] `go test ./internal/lint/... -run TestBootstrapManifest_` and `go test ./internal/guard/...` pass
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
1. In `internal/phase`, add `FindEpicLineageWithCache`. It returns the epic, the spec ID and the lineage provenance. Lineage is the one `parent-child` dependency on an epic. A `blocks` edge is not lineage. Two parent-child epics refuse with `recovery: bd dep remove <bead> <epic>`. Remove the bead-title fallback. Keep the signatures of `FindEpicForBead` and `FindEpicForBeadWithCache`; they call the new function (R7.1–R7.3). Existing fixtures whose epic dependency has no `dependency_type` gain `"dependency_type":"parent-child"`: `internal/phase/lineage_error_test.go`, `wireEpicFixture` in `internal/phase/agreement_table_test.go` (so `TestAgreementTable_Rows` keeps its claim rows) and `TestVerbOutput_Next` in `cmd/mindspec/verb_output_test.go`. The `TestBuildReadySet_` fixtures in `internal/next/readyset_test.go` and the fake-`bd` rows in `cmd/mindspec/next_phase_advance_test.go`, `next_positional_test.go`, `next_ready_gate_test.go` and `next_selection_test.go` also get a parent-child edge (`dependency_type` or the fake `bd` `parent` field) in place of title-only lineage. `TestFindEpicForBead_DependentEpicLookupErrorPropagates` stays: a failed parent-epic lookup still propagates and is not `ErrNoEpicLineage`. Delete `TestFindEpicForBead_EpicListErrorPropagates`, which tests the removed title-fallback list; `TestFindEpicForBead_NoTitleFallback` replaces it and asserts no lineage and no epic-list call.
2. Add one full-ID resolver for epic to spec, with three sources in order: `spec_id`, then `spec_num` plus `spec_title`, then the epic's `[SPEC <id>]` title as a full spec ID. Apply the R7.6 rules: a valid `spec_id` decides; an invalid one refuses; `spec_num` 0 or an empty title is absent; a disagreement or no source refuses with `recovery: bd update <epic> --set-metadata spec_id=<id>` (R7.6).
3. Route `(*Cache).FindEpicBySpecID` (`internal/phase/cache.go`) through that resolver. It scans every epic that `AllEpics` returns, closed epics included, before it memoizes. Two epics on one spec refuse in a verb that resolves that spec and name both; the refusal is memoized as an error, never as the first match (R7.6).
4. `spec approve` writes the full spec ID to epic metadata `spec_id` (`internal/approve/spec.go`) (R7.5). `next` uses the returned lineage provenance and runs no second `bd show` per plan row (`internal/next/beads.go`) (R7.4). Near-miss classes (`classifyNearMiss`, `internal/next/nearmiss.go`): a foreign parent-child epic stays N2c, inspection-only, with no `bd update` (`TestNearMissN2a_NeverReparentRecovery` stays unchanged). Remove the N2b arm: title-number lineage no longer exists, so N2b has no source. A parentless title-tagged plan-form bead is N1. The N1 text becomes "it has no lineage (no parent-child edge to an epic)", and its recovery stays `bd update <bead> --parent <epic>`, as spec R7.8 states for a title-only bead. Rename `TestNearMissN2b_SameNumberReparentCorrectsLineage` and `TestNearMissN2b_OtherNumberInspectsFirst` to `TestNearMissN1_TitleOnlySameNumber` and `TestNearMissN1_TitleOnlyOtherNumber`. Keep their parentless title-tagged rows and assert N1. Keep the fake-`bd` `bd update --parent` then `BuildReadySet` resolve check. Update the `want` text in `TestNearMissN1_RecoveryYieldsClaimable` and the class maps in `readyset_test.go` (`epic-129.7` goes from N2b to N1). Update the near-miss class list in `.mindspec/domains/workflow/architecture.md`. `ScanIntegrityFindings` (`internal/lifecycle/scan.go`) and `resolveTargetEpic` (`internal/approve/plan.go`) use the full-ID resolver in place of `SpecIDFromMetadata` (AC-24).
5. `beadIndex` (`internal/validate/readiness/readiness.go`) accepts a bead ID only when it equals `<epicID>.<N>` (R7.7). `spec create` refuses a new lettered ID such as `129b-x` (`cmd/mindspec/spec.go`); existing lettered directories stay valid (R7.9).
6. Add `internal/doctor/lineage.go`: a Warn for each open bead that resolves only through the removed title rule, with `recovery: bd update <bead> --parent <epic>`, and a report for two epics on one spec that continues (R7.6, R7.8). `RunWithOptions` (`internal/doctor/doctor.go`) calls the new `checkLineage(r, root)` after `checkLifecycleIntegrity(r, root)`.
7. Docs: add `#### Amendment (spec 132-readiness-followon)` to ADR-0023 (see § ADR Fitness). Update `.mindspec/domains/core/interfaces.md` and `architecture.md` (lineage API, spec-ID resolution, cache) and `.mindspec/domains/workflow/interfaces.md` (`spec_id` metadata, doctor lineage check, `spec create` refusal). Add `internal/validate/adr_spec132_lineage_test.go` (`TestDocs_Spec132Amendments_Lineage`).

**Verification**
- [ ] `go test ./internal/phase/... -run 'TestFindEpicForBead_BlocksDepIgnored|TestFindEpicForBead_TwoParentsRefuse|TestFindEpicForBead_NoTitleFallback|TestResolveSpec_|TestCacheFindEpicBySpecID_SpecIDOnlyLettered|TestCacheFindEpicBySpecID_DuplicateWithClosedEpicRefuses'` passes
- [ ] `go test ./internal/next/... -run 'TestBuildReadySet_OneShowPerPlanRow|TestBuildReadySet_ScopedLetteredSpecResolves|TestBuildReadySet_ScopedDuplicateEpicsRefuse'` passes (the `--spec` path through `rs.Cache.FindEpicBySpecID`)
- [ ] `go test ./internal/validate/readiness/... -run TestBeadIndex_RequiresEpicPrefix` passes
- [ ] `go test ./internal/doctor/... -run 'TestDoctor_TitleOnlyLineage|TestRunWithOptions_LineageFindingsReachReport'` passes. The second test runs `RunWithOptions` with fake `bd` and asserts that the title-only Warn and the two-epic report are in the returned report, and that a check after `checkLineage` (`checkGit`) also ran.
- [ ] `go test ./internal/phase/... -run 'TestFindEpicForBead_|TestAgreementTable_Rows'` and `go test ./cmd/mindspec/... -run 'TestVerbOutput_Next|TestAdvancePhaseOnClaim_|TestNext_|TestNextSelection_'` pass
- [ ] `go test ./internal/next/... -run 'TestNearMissN1_|TestNearMissN2a_|TestNearMissN4_|TestExclusionNotice_|TestBuildReadySet_'`, `go test ./internal/lifecycle/... -run TestScanIntegrityFindings_` and `go test ./internal/approve/... -run 'TestResolveTargetEpic_|TestPlanApprove_'` pass
- [ ] `go test ./cmd/mindspec/... -run 'TestSpecCreate_RefusesLetteredID|TestLifecycle_LetteredSpecResolves'` passes
- [ ] `go test ./internal/approve/... -run TestSpecApprove_WritesSpecID` passes
- [ ] `go test ./internal/validate/... -run TestDocs_Spec132Amendments_Lineage` passes
- [ ] `go test ./internal/lint/... -run TestBootstrapManifest_` and `go test ./internal/guard/...` pass
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short -timeout 20m` passes

**Acceptance Criteria**
- [ ] AC-22 (R7.1–R7.3): the three `TestFindEpicForBead_` tests pass.
- [ ] AC-23 (R7.4): `TestBuildReadySet_OneShowPerPlanRow` passes.
- [ ] AC-24 (R7.5, R7.6): `TestResolveSpec_PlainAndLetteredBothPresent`, `TestLifecycle_LetteredSpecResolves`, `TestResolveSpec_LegacyPairRepairResolves`, `TestResolveSpec_TitleWithoutIDRefuses`, `TestResolveSpec_SpecNumZeroAbsent`, `TestResolveSpec_InvalidSpecIDRefuses`, `TestResolveSpec_TwoEpicsScopedRefusal` pass. The cached scoped `next` path resolves a `spec_id`-only lettered epic and refuses duplicates, a closed duplicate included.
- [ ] AC-25 (R7.7–R7.9): `TestBeadIndex_RequiresEpicPrefix`, `TestDoctor_TitleOnlyLineage`, `TestSpecCreate_RefusesLetteredID` pass, and `idvalidate` accepts `008b-human-gates`.
- [ ] AC-26 (ADR-0023): the amendment is pinned.

**Depends on**
None.

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| AC-1 (R1.1–R1.3) | Bead 1 verification: `go test ./internal/complete/...` and `./internal/validate/...` landing-tree tests |
| AC-2 (R1.4, R1.5, R3.13) | Bead 1 verification (`TestCommittedRangeGate_SourceConflictPassesThenResolves`); Bead 5 verification (ownership-conflict tests, path-5 deferral) |
| AC-3 (R2.1) | Bead 2 verification: `TestApproveImpl_Early*`, push-path and direct-path subtests |
| AC-4 (R2.2) | Bead 2 verification: `TestFinalizeEpic_ExecutorGuardAfterAutoMerge` |
| AC-5 (R2.3–R2.5) | Bead 2 verification: push-probe tests and `TestPushProbe_ArgvMatchesPushBranch` |
| AC-6 (R2.7, R2.8) | Bead 3 verification: state-line tests |
| AC-7 (R2.9, R2.10) | Bead 3 verification: `TestApproveImpl_RerunConvergesFromEachStage` on the real executor, `TestFinalizeEpic_FinalizeTipIsPublishedTip` |
| AC-8 (R2.11, R2.12) | Bead 3 verification: five re-run tests |
| AC-9 (R3.0–R3.3, R3.13) | Bead 5 verification: 21 AC-9 tests |
| AC-10 (R3.4–R3.7) | Bead 5 verification: eight `internal/complete` tests and two `cmd/mindspec` loop tests; Bead 4 verification (`TestMergeState_`) |
| AC-11 (R3.8, R3.9) | Bead 5 verification: five AC-11 tests |
| AC-12 (R3.10) | Bead 5 verification: `TestComplete_RepeatedMergeOnlyNoNewMerge` |
| AC-13 (R3.11, R3.12) | Bead 4 verification: five doctor tests and `TestDoctorStatusTag_Info` |
| AC-14 (R4.1, R4.2, R4.5, R4.6) | Bead 6 verification: six AC-14 tests |
| AC-15 (R4.3, R4.4, R4.4a) | Bead 2 verification: four `TestFinalizeEpic_` remote tests, `TestApproveImpl_OneHasRemoteDecision`, `TestApproveImpl_HasRemoteProbeErrorRefuses`, `TestRemoteConfigured_GitErrorReturnsError` |
| AC-16 (R5.1–R5.5) | Bead 7 verification: seven AC-16 tests |
| AC-17 (R6.1–R6.6) | Bead 8 verification: `TestReattest_AncestorWithoutNamingMerge` |
| AC-18 (R6.2–R6.5, R6.7) | Bead 8 verification: ten AC-18 tests |
| AC-19 (R6.8, R6.9) | Bead 8 verification: two reattest tests and `TestDoctor_AncestorNamesPrerequisite` |
| AC-20 (R6.11) | Bead 8 verification: three `TestFallbackSafety_` tests |
| AC-21 (internal refs) | Bead 8 (`TestReattestText_*`), Bead 1 (`TestMaterializeText_NoHistoricalCitations`), Bead 2 (R4.4a text in `TestApproveImpl_HasRemoteProbeErrorRefuses`), Bead 6 (R4.6 text in `TestDoctor_LegacyMergeStrategyInfo`) |
| AC-22 (R7.1–R7.3) | Bead 9 verification: three `TestFindEpicForBead_` tests |
| AC-23 (R7.4) | Bead 9 verification: `TestBuildReadySet_OneShowPerPlanRow` |
| AC-24 (R7.5, R7.6) | Bead 9 verification: seven resolution tests and the cached scoped-`next` tests |
| AC-25 (R7.7–R7.9) | Bead 9 verification: `TestBeadIndex_RequiresEpicPrefix`, `TestDoctor_TitleOnlyLineage`, `TestSpecCreate_RefusesLetteredID` |
| AC-26 (R13) | AC-26 names `TestDocs_Spec132Amendments`; the plan delivers it as the prefix set. `go test ./internal/validate/... -run '^TestDocs_Spec132Amendments'` passes, and `-list '^TestDocs_Spec132Amendments'` prints these seven: `_Ownership` (Bead 1), `_Preflight` (Bead 2), `_Finalize` (Bead 3), `_MergeOnly` (Bead 5), `_MergeStrategy` (Bead 6), `_ReviewedTree` (Bead 7), `_Lineage` (Bead 9). Bead 1 also runs `mindspec validate spec 129-readiness-tracker-honesty`; R13.3 doc text is in Beads 5 and 8 |
| Validation proofs | Every bead: `gofmt -l .`, `go build ./...`, `go test ./... -short -timeout 20m`; after the `origin/main` merge: the full suite again; `golangci-lint run` at final review |
