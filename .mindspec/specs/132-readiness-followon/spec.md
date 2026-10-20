---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 132-readiness-followon: Readiness Followon

*Citation scope: code citations were verified against `main` at `d15787fd` on 2026-10-20 in the spec-132 worktree. Line numbers are approximate; the named symbol is authoritative.*

## Goal

This spec closes the items that spec 129 deferred, plus field defects found while 129 and 130 ran. After this spec:

- `complete` reads domain ownership from the spec branch, not only from the bead tree.
- `impl approve` refuses in preflight when it cannot finish. It never leaves a closed epic with an unmerged spec and no recovery line.
- An operator can merge a bead, verify the merge, and then close the bead.
- `merge_strategy` controls how `impl approve` lands a spec.
- Reattest uses the reviewed-SHA keys that spec 130 writes. It names the case where work arrived without a naming merge.
- Bead lineage comes from the parent dependency only. Spec titles no longer decide lineage.
- Lifecycle output supports parallel agents. Verbs that name a spec print guidance for that spec.
- Readiness and commit diagnostics follow the error contract.

## Background

Provenance: spec 129 deferred G (#234), H (#244), #250, I (`merge_strategy`) and phase-lineage unification (129 spec.md, "Deferred to the follow-on spec"). Spec 130 Non-Goals moved #233's reviewed-tree check, #251 and e07y item 1 here. The rest comes from spec 129 and 130 panel follow-up beads and from field reports.

Defects, each verified on `d15787fd`:

- **#234 (ownership ref).** `runCommittedRangeGate` (`internal/complete/complete.go:1653`) passes `ownerRef = beadHead` to `ValidateDocsRange` and `CheckADRDivergence`. `materialize.go:429,435` passes the bead tree. `checkUnclaimedSource` (`internal/validate/docsync.go:228`) uses the same ref. An `OWNERSHIP.yaml` claim made on the spec branch after the bead forked is invisible, so `adr-divergence-unowned` blocks owned paths.
- **#244 (half-done impl approve).** `ApproveImpl` (`internal/approve/impl.go`) closes the epic (MUTATION 1/3, :618) and writes `mindspec_phase=done` (2/3) before `FinalizeEpic` (3/3, :676). `FinalizeEpic` runs `guardMergeLayout`, `preflightMergeDestruction` and `PushBranch` (`internal/executor/mindspec_executor.go:822,831`). A failure there returns `finalizing epic: %w` with no recovery line. The epic is closed and the spec is not merged. Nobody has run a re-run test.
- **#250 (no verification window).** `complete` merges, closes the bead and removes the branch and worktree in one call. An operator cannot inspect the merge before the evidence is gone. A hand merge followed by `complete` works, but `doctor` reports the gap as a stale-open Error (`internal/doctor/lifecycle_integrity.go:43`).
- **`merge_strategy` is dead.** `config.MergeStrategy` (`internal/config/config.go:20`, default `auto`) is read only by `config show`. `FinalizeEpic` decides from `gitutil.HasRemote()` alone. This repo sets `merge_strategy: pr`.
- **#233 (reviewed tree).** Spec 130 R12 writes `mindspec_landed_reviewed_sha` and `mindspec_landed_review_panel`. Nothing reads them. Since spec 129, datum (c) never corroborates on the normal path, because `complete` lands commits after the panel SHA (129 plan.md OQ-3).
- **#251 (ancestor without naming merge).** Reattest nominates ownership only from a merge subject. When bead P's work reached the spec tip inside a later bead's branch, reattest returns `no-owned-exact-merge` (`internal/lifecycle/reattest.go:191`). The text says the work is absent. `doctor` reports the downstream bead, not P (`internal/lifecycle/scan.go:173`).
- **Internal refs in refusals.** The `no-owned-exact-merge` text and its CLI render (`cmd/mindspec/reattest.go:286,289`) print `ADR-0035 mindspec-q9ea`.
- **Title lineage (wtjr).** `FindEpicForBeadWithCache` (`internal/phase/derive.go:939`) falls back to a `[NNN]` title parse with `Sscanf %d`. `parseSpecID` (`internal/approve/spec.go:423`) uses `Atoi`, so spec `129b` gets `spec_num 0`, and its beads resolve to spec 129. `beadIndexRe` (`internal/validate/readiness/readiness.go`) accepts any trailing `.N`. `next` runs a second `bd show` per foreign-lineage plan row (fdfg.6) because `internal/phase` returns no provenance.
- **Serial-only advice.** `FormatResult` (`complete.go:1510`) prints "Do NOT run `mindspec next` or claim another bead". The instruct templates (`implement.md:49,94`, `plan.md:157`, the lifecycle table) and `plan_cmd.go:28,78` say the same. An orchestrator that runs parallel agents gets wrong advice.
- **Freshness warning noise.** `next --force` prints `warning: bypassing session freshness gate` on every claim (`cmd/mindspec/next.go:156`).
- **Ambiguous tail.** `spec create` (`cmd/mindspec/spec.go:79`), `spec approve` (`:226`), `plan approve` (`plan_cmd.go:81`) and `next` (`next.go:420,425`) call `emitInstruct(root)` with no spec. With two active specs, the tail prints "cannot determine mode" and `recovery: mindspec doctor`.
- **dvw3.** `next` runs `checkUnmergedBeads` only when `--spec` is set before `BuildReadySet` (`next.go:215`). A claim whose spec comes from the gate skips it.
- **m0ww.1.** `advanceState` (`complete.go:1562`) ignores a JSON decode failure of the ready list and returns implement mode with no next bead.
- **Readiness diagnostics (qdoh, fdfg.4, 4hdt).** `GateReadiness` reads plan.md and spec.md before the R1.8 lineage and scope checks. `next --help` shows the `__bare_allow_not_ready__` sentinel (`next.go:824`). MF-3 recoveries sit inside escaped Detail text. Some `recovery:` lines are prose. The menu path has no legacy-marker recovery. The loud-empty refusal has no recovery line. `ApplyOverride` (`override.go:23`) indexes rendered lines by signal position, so a multi-line Detail breaks it.
- **CommitPaths (0a84, 5x3j).** `gitutil.CommitPaths` (`internal/gitutil/gitops.go:1825`) stages path by path. A later failure leaves earlier paths staged. Errors embed raw git stderr. Spec 129 R8.2 and plan step 6 describe a narrower contract than the code.
- **Spec 129 text (ejdg).** 129 AC-11 contradicts R4.2. AC-1 quotes a diagnostic that the code does not emit. A plan verification command has no timeout.

Already fixed: 3gvd (`.cache/` is in `.gitignore:87`; no file under `.cache/` is tracked on main).

## Impacted Domains

- **workflow**:
  - `internal/complete/**`: ownership ref (R1), `--merge-only` (R3), parallel advice (R8), `advanceState` (R12).
  - `internal/approve/**`: preflight hoist and resume (R2), strategy preflight (R4), lettered-ID refusal (R7).
  - `internal/lifecycle/**`: reviewed-tree datum (R5), ancestor state and scan (R6).
  - `internal/validate/**`: two-ref ownership (R1), readiness order, render and override (R7, R10).
  - `internal/doctor/**`: merge-only severity (R3), ancestor finding (R6), title-lineage finding (R7).
  - `internal/next/**`, `internal/instruct/**`, `cmd/mindspec/**`: tail target (R9), advice (R8), freshness (R8), dvw3 (R12), help sentinel (R10), refusal text (R6).
- **execution**: `internal/executor/**` (strategy, preflight methods, push probe: R2, R4); `internal/gitutil/**` (CommitPaths: R11).
- **core**: `internal/phase/**` (lineage provenance: R7); `internal/state/**` (warned flag: R8); `internal/config/**` (strategy values: R4).

Documentation surfaces: ADR amendments (R13); `.mindspec/domains/workflow/{architecture,runbook}.md`; `project-docs/user/guides/merge-safety.md`; spec 129 status lines (R13).

## ADR Touchpoints

- [ADR-0041](../../adr/ADR-0041-gate-before-mutate.md) — AMENDED: `impl approve` preflight covers merge layout, destruction and push auth (R2). `complete --merge-only` is a commit phase with no close (R3). The Spec-129 amendment (ii) changes: datum (c) may corroborate through the reviewed tree (R5). It still never vetoes.
- [ADR-0036](../../adr/ADR-0036-ownership-discovery.md) — AMENDED: a path is owned when a manifest claims it at the bead tree or at the spec-branch tip (R1).
- [ADR-0023](../../adr/ADR-0023.md) — APPLIED: lineage is a bd parent dependency (R7).
- [ADR-0030](../../adr/ADR-0030-executor-boundary.md) — APPLIED: strategy and preflight git work stays in the executor (R2, R4).
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED: every new refusal ends with a command `recovery:` line (R2, R6, R10).

Terms:

- *Spec tip*: the commit at `spec/<id>` when the verb runs.
- *Naming merge*: a first-parent merge on the spec branch whose subject names `bead/<id>`.
- *Tool-artifact paths*: the set that `panel.ToolArtifactPaths` returns.

## Requirements

### R1 — Ownership from the spec tip (#234)

1. Both `complete` ownership legs (committed range and materialize) read manifests at two refs: the bead tree and the spec tip.
2. A path is owned when either ref claims it for the bead's domain.
3. `checkUnclaimedSource` uses the same two-ref rule.
4. A claim present only at the spec tip is owned. A claim absent at both refs still blocks with `adr-divergence-unowned`.

### R2 — Atomic `impl approve` (#244)

1. `ApproveImpl` runs the merge-layout guard and the destruction preflight for the chosen strategy before MUTATION 1/3.
2. On the push strategy, `ApproveImpl` runs `git push --dry-run` for the spec branch before MUTATION 1/3.
3. A preflight failure exits non-zero and leaves the epic open and the phase unchanged.
4. When `GIT_CONFIG_GLOBAL=/dev/null` or no credential helper is set, the push refusal names the credential helper.
5. A `FinalizeEpic` failure after MUTATION 1/3 returns a guard failure. Its text says the epic is closed and the spec is not merged. Its recovery is `mindspec impl approve <spec>`.
6. A re-run of `impl approve` after such a failure finishes the merge without a second close and without an error.

### R3 — Merge, then verify, then close (#250)

1. `mindspec complete <bead> --merge-only` runs every gate that `complete` runs, materializes, and merges the bead into the spec branch.
2. It does not close the bead, delete the branch or remove the worktree.
3. It writes `mindspec_merge_only_sha=<merge sha>` to bead metadata and prints the merge SHA and both parents.
4. A later plain `complete <bead>` finds the recorded merge, does not merge again, and closes and cleans up.
5. If the spec tip no longer contains the recorded merge, plain `complete` refuses and names `mindspec complete <bead> --merge-only`.
6. `doctor` reports a stale-open bead with a valid marker at Info, with recovery `mindspec complete <bead>`. A bead without a marker stays at Error.

### R4 — `merge_strategy` decides the landing

1. `merge_strategy` accepts `auto`, `pr` and `direct`. Config load refuses any other value.
2. `auto` keeps today's rule: push when a remote exists, else merge locally.
3. `pr` requires a remote. With no remote, `impl approve` refuses in preflight.
4. `direct` merges the spec branch into main locally and does not push, even when a remote exists.
5. `impl approve` prints the strategy it used and where that value came from.

### R5 — Reviewed-tree datum (#233)

1. Reattest reads R from `mindspec_landed_reviewed_sha`, else from the panel registrations.
2. Datum (c) corroborates when R is an ancestor of the merge's second parent D and `git diff --name-only R D` lists only tool-artifact paths.
3. Datum (c) still corroborates when D is in the reviewed-SHA set.
4. Datum (c) never vetoes. A failed datum (c) leaves the result to the other data.
5. The corroboration provenance string names which rule matched.

### R6 — Ancestor without a naming merge (#251)

1. Reattest adds state `ancestor-without-naming-merge`. It applies when no naming merge exists and the `bead/<P>` tip is an ancestor of the spec tip.
2. Its text says the work is present on the spec branch, that no merge names the bead, and that a re-merge cannot help.
3. Its recovery is the audited override for each dependent: `mindspec next <dependent> --allow-not-ready=MF-3 --evidence <file>`.
4. `no-owned-exact-merge` keeps its outcome. Its text and recovery say "attested restore" and print no ADR number or bead ID.
5. `doctor` reports P itself when P has no landed binding and matches R6.1. The finding is a Warn with the R6.3 recovery.
6. Reattest gets no operator nomination.
7. Error-injection tests cover the three `fallbackSafety` reads (forward content, reverse content, branch tip). Each read error refuses.

### R7 — One lineage (wtjr, fdfg.6)

1. `FindEpicForBeadWithCache` resolves lineage from the parent dependency only. The title fallback goes.
2. `internal/phase` returns the lineage provenance with the epic. `next` uses it and runs no second `bd show` per plan row.
3. `beadIndex` accepts a bead ID only when it equals `<epicID>.<N>`.
4. `doctor` lists each plan bead whose epic resolves by title but not by parent. Its recovery is `bd update <bead> --parent <epic>`.
5. `spec create` refuses a lettered spec ID such as `129b-x`. Existing lettered spec directories stay valid.

### R8 — Parallel-safe advice

1. `complete`, `plan approve` and the instruct templates say: start the next bead in a fresh context or a fresh agent with `mindspec next`.
2. No lifecycle output prints "Do NOT run `mindspec next`" or "claim another bead".
3. `next --force` prints the freshness-bypass warning once per session. The session file records that the warning was printed.

### R9 — Spec-named tail

1. `spec create`, `spec approve`, `plan approve` and `next` (after a claim) render the instruct tail for the spec that they acted on.
2. This rendering uses the same resolution as `mindspec instruct --spec <id>`.
3. With two active specs, none of these verbs prints "cannot determine mode".

### R10 — Readiness diagnostics

1. `GateReadiness` runs the plan-bead and scope checks before it reads plan.md or spec.md.
2. `next --help` does not show the bare-override sentinel.
3. `Render` keeps one line per signal. Each MF-3 dependency recovery and extra exit prints as its own `recovery:` line after the signal line, each escaped.
4. `ApplyOverride` addresses lines by signal ID, not by line index.
5. Every readiness and `next` `recovery:` line is a runnable command with no prose and no parenthetical.
6. The menu path prints the legacy-marker release recovery, as the in-progress path does.
7. The loud-empty refusal ends with `recovery: mindspec next --spec <id>`.

### R11 — CommitPaths contract (0a84, 5x3j)

1. `CommitPaths` stages tracked paths with `add -u` and untracked, non-ignored paths with `add`. It never force-adds an ignored path.
2. On any staging failure, `CommitPaths` restores the index to its state before the call.
3. `CommitPaths` errors pass git stderr through `termsafe`.

### R12 — Small `complete` and `next` fixes (m0ww.1, dvw3)

1. When `advanceState` cannot decode the ready list, `complete` takes the indeterminate post-close path. It does not report implement mode with no bead.
2. `next` runs `checkUnmergedBeads` for the spec that the gate learns, before the claim.

### R13 — Text amendments

1. ADR-0041 and ADR-0036 get a "Spec 132 amendment" section with the R1, R2, R3 and R5 changes.
2. Spec 129 gets dated status notes: R8.2 and plan step 6 follow R11.1; AC-11 says advisories never change the exit code, the binding or the audited tuple; AC-1 quotes the emitted diagnostic; the plan verification command sets `-timeout=20m`; the "Deferred to the follow-on spec" list names spec 132.
3. `merge-safety.md` and the workflow runbook name the attested-restore exit, the ancestor state and `--merge-only`.

## Scope

### In Scope
- The packages and documents in Impacted Domains.

### Out of Scope
- The fix branch `fix/resolve-merge-hook-next-bead-line`: `--resolve-merge` and the pre-commit hook, the empty "Next bead ready:" line, and the worktree redirect after `complete`. Spec 132 builds R8 and R9 on that branch. It must reach main before R8 starts.

## Non-Goals

- Operator nomination of landed ownership (#251 option 3).
- Test-only hygiene with no behaviour change: e07y items 2–9, fdfg items 1, 2, 3 and 5, m0ww items 2 and 3, fl6d, 79os, 1x70. One chore bead holds them.
- Renaming or migrating existing lettered spec directories.
- A verification hook between merge and close (#250 option 3).

## Acceptance Criteria

- [ ] **AC-1 (R1.1–R1.4)**: A spec-tip `OWNERSHIP.yaml` claim made after the bead forks lets `complete` pass in both legs. With no claim at either ref, `complete` blocks with `adr-divergence-unowned`. Tests: `TestCommittedRangeGate_SpecTipClaimOwns`, `TestMaterialize_SpecTipClaimOwns`, `TestCheckUnclaimedSource_TwoRefs`, `TestCommittedRangeGate_NoClaimStillBlocks`.
- [ ] **AC-2 (R2.1–R2.3)**: A layout-guard failure, a destruction-preflight failure and a failing push dry-run each exit non-zero with the epic open and the phase unchanged. Tests: `TestApproveImpl_PreflightLayoutRefusesBeforeClose`, `TestApproveImpl_PreflightDestructionRefusesBeforeClose`, `TestApproveImpl_PushProbeRefusesBeforeClose`.
- [ ] **AC-3 (R2.4)**: With `GIT_CONFIG_GLOBAL=/dev/null`, the push refusal contains "credential helper". Test: `TestApproveImpl_PushProbeNamesCredentialHelper`.
- [ ] **AC-4 (R2.5, R2.6)**: An injected `FinalizeEpic` failure prints "epic closed", "spec not merged" and `recovery: mindspec impl approve <spec>`. A second run merges the spec, closes nothing again and exits 0. Test: `TestApproveImpl_FinalizeFailureThenRerunConverges` (runs both calls).
- [ ] **AC-5 (R3.1–R3.3)**: `complete --merge-only` leaves the bead open, the branch and the worktree present, writes the marker, and prints the merge SHA and both parents. Test: `TestComplete_MergeOnlyKeepsBeadOpen`.
- [ ] **AC-6 (R3.4, R3.5)**: A plain `complete` after `--merge-only` makes no new merge commit, closes the bead and removes the branch and worktree. When the spec tip lacks the recorded merge, plain `complete` refuses and names `--merge-only`. Tests: `TestComplete_AfterMergeOnlyConverges`, `TestComplete_MergeOnlyMarkerMissingFromTipRefuses`.
- [ ] **AC-7 (R3.6)**: `doctor` reports a marked stale-open bead at Info and an unmarked one at Error. Test: `TestDoctor_StaleOpenMergeOnlyIsInfo`.
- [ ] **AC-8 (R4.1–R4.5)**: `merge_strategy: bogus` fails config load. `pr` with no remote refuses before the epic close. `direct` with a remote merges locally and makes no push call. `auto` keeps both current paths. Output names the strategy and its source. Tests: `TestConfig_MergeStrategyRejectsUnknown`, `TestApproveImpl_PRStrategyNoRemoteRefuses`, `TestFinalizeEpic_DirectStrategyWithRemoteDoesNotPush`, `TestFinalizeEpic_AutoStrategyUnchanged`.
- [ ] **AC-9 (R5.1–R5.3, R5.5)**: R is an ancestor of D and R..D touches only tool-artifact paths, so datum (c) corroborates and the provenance names the reviewed-tree rule. R..D also touches a source file, so datum (c) does not corroborate. Tests: `TestReattest_ReviewedTreeCorroborates`, `TestReattest_ReviewedTreeSourceDiffNoCorroboration`, `TestReattest_ReviewedSHAFromRegistrationWhenKeyAbsent`.
- [ ] **AC-10 (R5.4)**: A datum (c) mismatch with the other data agreeing still binds. Test: `TestReattest_ReviewedTreeNeverVetoes`.
- [ ] **AC-11 (R6.1–R6.3)**: Bead P's tip is an ancestor of the spec tip with no naming merge, so reattest returns `ancestor-without-naming-merge`. The text says the work is present and a re-merge cannot help. The recovery names the MF-3 override. Test: `TestReattest_AncestorWithoutNamingMerge`.
- [ ] **AC-12 (R6.4)**: The `no-owned-exact-merge` refusal and its CLI render contain "attested restore" and match neither `ADR-[0-9]` nor `mindspec-[a-z0-9]{4}`. Test: `TestReattestRefusalText_NoInternalRefs`.
- [ ] **AC-13 (R6.5)**: `doctor` names P, not its dependent, at Warn with the override recovery. Test: `TestDoctor_AncestorWithoutNamingMergeNamesPrerequisite`.
- [ ] **AC-14 (R6.7)**: An injected error on each of the three `fallbackSafety` reads refuses. Tests: `TestFallbackSafety_ForwardContentErrorRefuses`, `TestFallbackSafety_ReverseContentErrorRefuses`, `TestFallbackSafety_BranchTipErrorRefuses`.
- [ ] **AC-15 (R7.1, R7.2)**: A bead with a `[129]` title and no parent dependency has no lineage. A foreign-lineage plan row costs one `bd show`. Tests: `TestFindEpicForBead_NoTitleFallback`, `TestBuildReadySet_OneShowPerPlanRow`.
- [ ] **AC-16 (R7.3)**: `beadIndex` rejects `mindspec-x.3` for epic `mindspec-y` and accepts `mindspec-y.3`. Test: `TestBeadIndex_RequiresEpicPrefix`.
- [ ] **AC-17 (R7.4)**: `doctor` lists a title-only bead with recovery `bd update <bead> --parent <epic>`. Test: `TestDoctor_TitleOnlyLineage`.
- [ ] **AC-18 (R7.5)**: `spec create 129b-x` refuses. `idvalidate` still accepts `008b-human-gates`. Tests: `TestSpecCreate_RefusesLetteredID`, `TestSpecID_LegacyLetteredDirValid`.
- [ ] **AC-19 (R8.1, R8.2)**: No template, `complete` output or `plan approve` output contains "Do NOT run `mindspec next`" or "claim another bead". Each names a fresh context or fresh agent. Test: `TestLifecycleAdvice_ParallelSafe` (scans all rendered surfaces).
- [ ] **AC-20 (R8.3)**: Two `next --force` claims in one session print the bypass warning once. A new session prints it again. Test: `TestNext_ForceWarningOncePerSession`.
- [ ] **AC-21 (R9.1–R9.3)**: With two active specs, each of the four verbs prints the tail for its spec and no "cannot determine mode". Test: `TestSpecNamedVerbs_TailTargetsSpec` (one leg per verb).
- [ ] **AC-22 (R10.1)**: A missing spec dir for a foreign-lineage bead refuses with the lineage reason, not a read error. Test: `TestGateReadiness_LineageBeforeFileIO`.
- [ ] **AC-23 (R10.2)**: `next --help` does not contain `bare_allow_not_ready`. Test: `TestNextHelp_HidesSentinel`.
- [ ] **AC-24 (R10.3, R10.4)**: A multi-line MF-3 plus an overridden passing MF-4 renders each signal once, keeps every MF-3 recovery as its own `recovery:` line, and marks MF-4 once. Test: `TestApplyOverride_MultiLineMF3WithOverriddenMF4`.
- [ ] **AC-25 (R10.5–R10.7)**: Every `recovery:` line from readiness and `next` starts with `mindspec ` or `bd ` or `git `. The menu path prints the legacy-marker recovery. The loud-empty refusal prints its recovery. Tests: `TestReadinessRecoveries_AreCommands`, `TestNextMenu_LegacyMarkerRecovery`, `TestHandleEmptyScoped_HasRecovery`.
- [ ] **AC-26 (R11.1–R11.3)**: A failing third path leaves the index identical to its pre-call state. An ignored path is not added. A control byte in git stderr is escaped. An `ls-files --error-unmatch` exit other than 1 is an error, not "untracked". `bootstrap --dry-run` skips the review-ignore ensure. Tests: `TestCommitPaths_AllOrNothing`, `TestCommitPaths_NeverForceAddsIgnored`, `TestCommitPaths_StderrEscaped`, `TestStageExplicitPaths_ExitCodeDiscrimination`, `TestBootstrapDryRun_SkipsReviewEnsure`.
- [ ] **AC-27 (R12.1, R12.2)**: A non-JSON ready list gives the indeterminate post-close output. An unscoped claim whose spec has an orphaned closed bead refuses. Tests: `TestAdvanceState_DecodeFailureIndeterminate`, `TestNext_UnscopedClaimChecksUnmergedBeads`.
- [ ] **AC-28 (R13)**: The ADR and spec-129 notes exist. `mindspec validate spec 129-readiness-tracker-honesty` passes. Test: `TestDocs_Spec132Amendments` (greps each note).

## Validation Proofs

- `go test ./...`: PASS.
- `golangci-lint run`: no new findings.
- `mindspec validate spec 132-readiness-followon`: PASS.
- A scratch repo with a remote and `GIT_CONFIG_GLOBAL=/dev/null`: `impl approve` refuses before the epic close and the epic stays open.
- A scratch repo: `complete --merge-only`, then `git log -1 --format=%P spec/<id>`, then `complete`: one merge commit, bead closed.

## Open Questions

- [ ] **`merge_strategy`: honour or remove?** This spec honours it (R4). The alternative is to delete the key and its `config show` row, since `HasRemote` already gives the right answer for most repos. Recommended: honour.
- [ ] **Freshness warning: once per session or a dispatch flag?** This spec prints it once per session (R8.3). The alternative is a `next --dispatched` flag that an orchestrator sets to skip the gate and the warning. Recommended: once per session.
- [ ] **#250 shape.** This spec adds `--merge-only` (R3.1–R3.5) and the Info severity (R3.6). The smaller alternative keeps only R3.6 and documents a hand merge followed by `complete` as supported. Recommended: both, as written.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
