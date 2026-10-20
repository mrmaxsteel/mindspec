---
adr_citations:
    - ADR-0030
    - ADR-0035
    - ADR-0037
    - ADR-0041
    - ADR-0043
    - ADR-0044
    - ADR-0045
approved_at: "2026-10-20T01:31:10Z"
approved_by: user
bead_ids:
    - mindspec-inyk.1
    - mindspec-inyk.2
    - mindspec-inyk.3
    - mindspec-inyk.4
    - mindspec-inyk.5
    - mindspec-inyk.6
    - mindspec-inyk.7
    - mindspec-inyk.8
    - mindspec-inyk.9
    - mindspec-inyk.10
    - mindspec-inyk.11
    - mindspec-inyk.12
spec_id: 130-panel-substrate-integrity
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/config/config.go
        - internal/config/config_test.go
        - cmd/mindspec/config_test.go
        - internal/setup/review_discipline_test.go
        - plugins/mindspec/skills/ms-panel-run/SKILL.md
        - .claude/skills/ms-panel-run/SKILL.md
        - plugins/mindspec/skills/ms-panel-tally/SKILL.md
        - .claude/skills/ms-panel-tally/SKILL.md
        - .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md
        - .mindspec/domains/core/interfaces.md
        - CHANGELOG.md
        - internal/validate/adr_spec130_sizes_test.go
    - depends_on: []
      id: 2
      key_file_paths:
        - internal/panel/panel.go
        - internal/panel/ledger.go
        - internal/panel/ledger_test.go
        - internal/panel/finding.go
        - internal/panel/finding_test.go
        - internal/panel/handedit.go
        - internal/panel/disposition.go
        - internal/panel/disposition_store.go
        - internal/panel/disposition_query.go
        - internal/panel/disposition_query_test.go
        - cmd/mindspec/panel_disposition.go
        - cmd/mindspec/panel_disposition_store.go
        - cmd/mindspec/panel_disposition_store_test.go
        - .mindspec/domains/workflow/interfaces.md
    - depends_on: []
      id: 3
      key_file_paths:
        - internal/panel/create.go
        - internal/panel/create_test.go
        - cmd/mindspec/panel.go
        - cmd/mindspec/panel_test.go
        - cmd/mindspec/root.go
        - internal/complete/panel_advisory.go
        - internal/complete/panel_gate_e2e_test.go
        - internal/guard/registries.go
        - .mindspec/domains/workflow/interfaces.md
    - depends_on: []
      id: 4
      key_file_paths:
        - internal/panel/subject.go
        - internal/panel/subject_test.go
        - internal/panel/panel.go
        - internal/panel/create.go
        - internal/panel/gate.go
        - internal/panel/panel_decision_test.go
        - internal/gitutil/gitops.go
        - internal/executor/executor.go
        - internal/executor/mindspec_executor.go
        - internal/executor/mock.go
        - internal/complete/panel_advisory.go
        - internal/instruct/panelstate.go
        - cmd/mindspec/panel.go
        - cmd/mindspec/panel_subject_test.go
        - internal/lint/ratchet_argv_table_test.go
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
        - .mindspec/domains/workflow/interfaces.md
        - .mindspec/domains/execution/interfaces.md
        - internal/validate/adr_spec130_subject_test.go
    - depends_on:
        - 1
        - 2
        - 3
        - 4
      id: 5
      key_file_paths:
        - internal/panel/create.go
        - internal/panel/create_test.go
        - internal/panel/panel.go
        - internal/config/config.go
        - cmd/mindspec/panel.go
        - cmd/mindspec/panel_test.go
        - .mindspec/domains/workflow/interfaces.md
        - .mindspec/domains/core/interfaces.md
    - depends_on:
        - 1
        - 2
        - 3
        - 4
      id: 6
      key_file_paths:
        - internal/panel/passrule.go
        - internal/panel/passrule_test.go
        - internal/panel/ruleline.go
        - internal/panel/gate.go
        - internal/panel/tally.go
        - internal/panel/tally_test.go
        - internal/panel/handedit.go
        - internal/panel/panel_decision_test.go
        - internal/panel/refutation_test.go
        - internal/panel/hostile_fields_test.go
        - internal/panel/votedecision_test.go
        - cmd/mindspec/panel.go
        - cmd/mindspec/panel_ruleline_test.go
        - internal/instruct/panelstate.go
        - internal/instruct/panelstate_test.go
        - internal/instruct/panelstate_hostile_test.go
        - internal/complete/panel_advisory.go
        - .mindspec/domains/workflow/interfaces.md
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
        - .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md
        - .mindspec/adr/ADR-0044-panel-review-conduct.md
        - internal/validate/adr_spec130_passrule_test.go
    - depends_on:
        - 5
        - 6
      id: 7
      key_file_paths:
        - cmd/mindspec/panel_rebind.go
        - cmd/mindspec/panel_rebind_test.go
        - internal/panel/gate.go
        - internal/complete/panel_advisory.go
        - internal/complete/panel_refuted_test.go
        - internal/lifecycle/landed.go
        - internal/lifecycle/landed_test.go
        - internal/guard/registries.go
        - .mindspec/domains/workflow/interfaces.md
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
        - .mindspec/adr/ADR-0041-gate-before-mutate.md
        - internal/validate/adr_spec130_rebind_test.go
    - depends_on:
        - 5
        - 6
      id: 8
      key_file_paths:
        - cmd/mindspec/panel_refute.go
        - cmd/mindspec/panel_abandon.go
        - cmd/mindspec/panel_verbs_test.go
        - internal/lint/ratchet_argv_table_test.go
        - internal/guard/registries.go
        - plugins/mindspec/skills/ms-panel-tally/SKILL.md
        - plugins/mindspec/skills/ms-panel-run/SKILL.md
        - plugins/mindspec/skills/ms-bead-impl/SKILL.md
        - .claude/skills/ms-panel-tally/SKILL.md
        - .claude/skills/ms-panel-run/SKILL.md
        - .claude/skills/ms-bead-impl/SKILL.md
        - internal/setup/review_discipline_test.go
        - internal/setup/readiness_skill_pin_test.go
        - internal/setup/panel_skills_test.go
        - .mindspec/domains/workflow/interfaces.md
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
        - .mindspec/adr/ADR-0045-binary-owns-sequence-and-rules.md
        - .mindspec/specs/129-readiness-tracker-honesty/spec.md
        - internal/validate/adr_spec130_verbs_test.go
    - depends_on:
        - 1
        - 2
        - 4
      id: 9
      key_file_paths:
        - internal/gitutil/gitignore.go
        - internal/gitutil/reviews_ignore_test.go
        - internal/bootstrap/bootstrap.go
        - internal/bootstrap/gitignore_test.go
        - .gitignore
        - internal/panel/artifacts.go
        - internal/panel/artifacts_test.go
        - internal/panel/gate.go
        - internal/next/guard.go
        - internal/next/guard_test.go
        - internal/complete/materialize.go
        - internal/executor/executor.go
        - internal/executor/mindspec_executor.go
        - internal/executor/mock.go
        - internal/complete/panel_ledger.go
        - internal/complete/panel_ledger_test.go
        - internal/complete/complete.go
        - internal/approve/impl.go
        - internal/approve/impl_test.go
        - internal/guard/registries.go
        - .mindspec/domains/execution/interfaces.md
        - .mindspec/domains/workflow/interfaces.md
        - .mindspec/specs/129-readiness-tracker-honesty/spec.md
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
        - .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md
        - internal/validate/adr_spec130_ledger_test.go
    - depends_on:
        - 5
        - 6
        - 9
        - 12
      id: 10
      key_file_paths:
        - internal/panel/disposition_store.go
        - internal/panel/disposition_query.go
        - internal/panel/disposition_query_test.go
        - cmd/mindspec/panel.go
        - cmd/mindspec/panel_test.go
        - internal/complete/panel_core3.go
        - internal/complete/panel_core3_test.go
        - internal/complete/panel_advisory.go
        - internal/complete/complete.go
        - internal/approve/impl.go
        - internal/approve/impl_test.go
        - internal/guard/registries.go
        - .mindspec/domains/workflow/interfaces.md
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
        - .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md
        - internal/validate/adr_spec130_core3_test.go
    - depends_on:
        - 7
        - 8
        - 10
      id: 11
      key_file_paths:
        - internal/bead/deferral.go
        - internal/bead/deferral_test.go
        - cmd/mindspec/panel_defer.go
        - cmd/mindspec/panel_verbs_test.go
        - internal/lint/ratchet_argv_table_test.go
        - internal/complete/panel_deferred.go
        - internal/complete/panel_deferred_test.go
        - internal/complete/complete.go
        - internal/approve/impl.go
        - internal/approve/impl_test.go
        - internal/panel/passrule.go
        - internal/panel/passrule_test.go
        - internal/panel/gate.go
        - internal/guard/registries.go
        - plugins/mindspec/skills/ms-panel-tally/SKILL.md
        - plugins/mindspec/skills/ms-panel-run/SKILL.md
        - .claude/skills/ms-panel-tally/SKILL.md
        - .claude/skills/ms-panel-run/SKILL.md
        - internal/setup/panel_skills_test.go
        - .mindspec/domains/workflow/interfaces.md
        - .mindspec/domains/execution/interfaces.md
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
        - .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md
        - internal/validate/adr_spec130_deferral_test.go
    - depends_on:
        - 4
      id: 12
      key_file_paths:
        - internal/executor/executor.go
        - internal/executor/mindspec_executor.go
        - internal/executor/mock.go
        - internal/executor/merge_binding_test.go
        - internal/complete/complete.go
        - internal/complete/panel_advisory.go
        - internal/complete/panel_landed_review_test.go
        - internal/complete/panel_advisory_test.go
        - internal/complete/panel_advisory_hostile_test.go
        - internal/complete/panel_adhoc_isolation_test.go
        - internal/complete/panel_gate_layout_test.go
        - internal/complete/panel_gate_untracked_test.go
        - internal/complete/panel_gate_e2e_test.go
        - .mindspec/domains/execution/interfaces.md
        - .mindspec/domains/workflow/interfaces.md
        - .mindspec/domains/workflow/architecture.md
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
        - internal/validate/adr_spec130_keys_test.go
---
# Plan: 130-panel-substrate-integrity

## ADR Fitness

- **ADR-0030 (executor boundary)**: stays fit. `internal/complete` and `internal/approve` get `ls-tree` and `check-ignore` through two new Executor methods (Beads 4 and 9). They never import `gitutil`. `go test ./internal/lint/... -run TestEnforcementHasNoGitLeaks` stays green.
- **ADR-0035 (agent error contract)**: stays fit. Every new refusal uses `guard.NewFailure` with one `recovery:` line. Register each recovery with an unprovable operand in `guard.OpaqueOperandRegistry` (`internal/guard/registries.go`).
- **ADR-0037 (panel gate contract)**: amended. Bead 4 appends the `## Amendment (Spec 130)` section. Beads 6, 7, 8, 9, 10, 11 and 12 add bullets to it for the code that each one ships. See "ADR landing" below.
- **ADR-0041 (gate before mutate)**: amended. Bead 7 appends the section and a back-pointer at datum (c) to R3.5. All verbs check before they write.
- **ADR-0043 (disposition store)**: amended. Bead 1 appends the `## Amendment (Spec 130)` section, adds the inline back-pointer at the Spec-126 bullet, and keeps the `## Amendment (Spec 126)` heading. Beads 6, 9, 10 and 11 add bullets.
- **ADR-0044 (review conduct)**: amended. Bead 6 appends the section and the two R13.4 back-pointers.
- **ADR-0045 (binary owns rules)**: amended. Bead 8 appends the section and the R13.5 status note. Bead 8 moves the refute and abandon procedures out of skills into verbs, so it can cite this ADR.

Each bead that edits an ADR adds its own pin file `internal/validate/adr_spec130_<topic>_test.go`. Each pin asserts only the text of its own bead. Each pin function name starts with `TestADRAmendments_Spec130`. Thus `-run TestADRAmendments_Spec130` runs all pins (AC-26).

## Testing Strategy

- **Unit tests** in `internal/panel` use fixture panel dirs and a fake `GateIO`. `internal/panel` stays a leaf: no git, no `os/exec` (`TestPanelLeafImports_StdlibPlusTermsafeOnly`).
- **Integration tests** in `internal/complete` use real temp git repos, `setupPanelGateRepo` (`internal/complete/panel_gate_e2e_test.go`) and `fakeBdOnPath` (`internal/complete/fault_injection_realgit_test.go`). These helpers are package-private. Tests in `internal/approve` and `cmd/mindspec` copy their pattern into a local helper. The `defer` checks use a fake `bd` on PATH.
- **Hatch seams**: `complete` tests set `panelSkipEnvFn` (`internal/complete/panel_advisory.go`). `internal/approve` has no seam today (`runOrphanObligationGate`: "Hatches ... are never consulted here"). Bead 10 adds `implPanelSkipEnvFn` in `internal/approve/impl.go`, and Bead 11 uses it. Tests do not set real environment variables.
- **Argv ratchet**: each new exec site adds one row to `internal/lint/ratchet_argv_table_test.go` (`TestArgvIDOperandGated`).
- **Recovery operands**: a bead that adds a recovery line with a substituted command operand registers it in `guard.OpaqueOperandRegistry` (`internal/guard/registries.go`). `go test ./internal/lint/... -run TestProductDiagnosticScan_UnprovableCommandOperandsAreGoverned` (`internal/lint/destructive_guidance_test.go`) fails on an unregistered one.
- **Union merge**: parallel beads can add rows to the ratchet table and bullets to the same ADR `## Amendment (Spec 130)` section. Merge them as a union. Keep every row and every bullet.
- **Real verification**: a Verification line counts only when `go test <pkg> -list '<regex>'` prints each test that the bead names. `[no tests to run]` is a failure.
- **Boundary tests**: `TestEnforcementHasNoGitLeaks` is in `internal/lint/boundary_test.go`. Run it with `go test ./internal/lint/... -run TestEnforcementHasNoGitLeaks`. `TestPanelLeafImports_StdlibPlusTermsafeOnly` runs in `./internal/panel/...`.
- **Doc-sync**: `complete` runs the doc-sync gate. `runCommittedRangeGate` calls `validate.ValidateDocsRange` (`internal/complete/complete.go`), and the snapshot path calls `validate.ValidateDocsSnapshot` (`internal/complete/materialize.go`). In `internal/validate/docsync.go`, `isSourceFile` counts non-test `.go` files under `internal/` or `cmd/`. `checkInternalPackages` maps each `internal/` file to its domain through `OWNERSHIP.yaml`. It fails with "internal sources in domain %q changed ... but no doc updates under" unless a doc under `.mindspec/domains/<domain>/` changes in the same range. `cmd/**` is in the workflow domain too (`.mindspec/domains/workflow/OWNERSHIP.yaml`), so a `cmd/` source change needs a workflow doc change. Thus each bead updates the `interfaces.md` of each domain whose `internal/` or `cmd/` code it changes: `core` (`internal/config`), `execution` (`internal/executor`, `internal/gitutil`, `internal/bead`) and `workflow` (`cmd/`, `internal/panel`, `internal/complete`, `internal/approve`, `internal/next`, `internal/instruct`, `internal/lifecycle`, `internal/bootstrap`, `internal/guard`). No bead uses `--allow-doc-skew`. `validateSpecArtifactSync` accepts a spec 129 `spec.md` change only with an ADR or sibling change; Beads 8 and 9 edit an ADR in the same range.
- **Every bead** ends with `gofmt -l .` (empty output) and `go build ./... && go test ./... -short` (AC-27).
- **Dogfood boundary**: this spec changes the panel gate that the lane itself uses. Bead panels run on the installed v0.16.0 binary, not a spec-branch build. No branch build is installed during the run.

## Bead 1: Per-gate default panel sizes

**Steps**
1. In `internal/config/config.go`, remove the `panel.reviewers` list from `DefaultConfig`. Remove the `loadUncached` backfill and the `globalReviewers` fallback to `DefaultConfig().Panel.Reviewers` (R2.1).
2. When a gate resolves no reviewer list, use the per-gate default size: `spec_approve` 6, `plan_approve` 6, `bead` 4, `final_review` 6, `adhoc` 6. Fill families round-robin: claude, codex, claude, and so on. An explicit `panel.reviewers` applies to every gate without its own list (OQ-1).
3. Make `PanelExpectedReviewers` resolve through the per-gate defaults. A `create` without `--gate` uses the `adhoc` size (6). Make `PanelGateAdvisoryDefault` use the size of the recorded gate.
4. Apply the sum-below-2 check and the `approve_threshold` range check only to an explicit global list. Do not range-check `panel.gates.<g>.approve_threshold`. A config with no `panel.reviewers` and `approve_threshold: 3` loads. A config with only `panel.gates.bead.approve_threshold: 5` loads.
5. Change the single-reviewer error text in `validateOrchestration` and `validateGates`. The text says "cannot pass" and not "always-pass" (R1.9).
6. Update the spec-112 pins: the `DefaultConfig` 3+3 assertions in `internal/config/config_test.go` (`TestLoad_GatesAbsentByteIdentical109` and its siblings), and in `cmd/mindspec/config_test.go` `TestConfigShow_EmitsPanelModelsLoop` ("family: claude", "count: 3" on `renderConfig(DefaultConfig())`) and `TestConfigShow_GatesSubstitutesAndModelAdvisory` ("expected_reviewers: 6").
7. Change the size text in the skills. Change only the size; Bead 8 replaces the threshold text.
   - `plugins/.../ms-panel-run/SKILL.md`: the description "(shipped default: 3 Claude Agents + 3 Codex CLI sessions)", the sentence "shipped default: 6 reviewers — 3 `claude` + 3 `codex`", the sentence "`DefaultConfig` panel VALUES are unchanged", and the sentence "`DefaultConfig` panel VALUES ship unchanged — a zero-config install keeps its sizes".
   - `.claude/.../ms-panel-run/SKILL.md`: the description "launch 6 reviewers (3 Claude Agents + 3 Codex CLI sessions)" and "(6 reviewers, N−1 threshold)".
   - `plugins/.../ms-panel-tally/SKILL.md`: "shipped default: 6 reviewers".
   - `.claude/.../ms-panel-tally/SKILL.md`: "5/6 for the default panel".
8. Update the spec-126 pins in `internal/setup/review_discipline_test.go`: the "shipped default: 6 reviewers" and "n-1" fragment rows for the `ms-panel-run` and `ms-panel-tally` surfaces, and `TestReviewDiscipline_AC1PanelSizeConsistency`. Keep `TestReviewDiscipline_ADR0043Amendment`. The text "a zero-config install keeps its sizes" is the locator of the AC-10 allowlist entry (`surface: "ms-panel-run"`, `matched: "Spec 126"`). Keep that locator and the Spec 126 citation, or update the entry. Add an entry for each new `Spec 130` token in that skill.
9. In ADR-0043, append `## Amendment (Spec 130)` with the first bullet: the default becomes 6/6/4/6 per gate (R2.1); this reverses spec 112 R2. Add an inline back-pointer at the bullet "The shipped default is 6 / `n-1`, unconditionally". Keep the `## Amendment (Spec 126)` heading.
10. Add a `CHANGELOG.md` `## [Unreleased]` line that names the 6/6/4/6 change.
11. Update `.mindspec/domains/core/interfaces.md` § Config with the per-gate defaults.
12. Add `internal/validate/adr_spec130_sizes_test.go` (`TestADRAmendments_Spec130_Sizes`). It pins the section heading, the size bullet, the back-pointer and the CHANGELOG line.

**Verification**
- [ ] `go test ./internal/config/... -run 'TestPanelGateDefaults_|TestConfigValidate_SingleReviewerText|TestLoad_GatesAbsentByteIdentical109'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestConfigShow_EmitsPanelModelsLoop|TestConfigShow_GatesSubstitutesAndModelAdvisory'` passes
- [ ] `go test ./internal/setup/... -run TestReviewDiscipline_` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Sizes` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-7: `TestPanelGateDefaults_PerGateSizes`, `TestPanelGateDefaults_RoundRobinFamilies`, `TestPanelGateDefaults_ExplicitGlobalWins`, `TestPanelGateDefaults_NoGateAndGateThreshold`, `TestPanelGateDefaults_AdhocSize` pass (R2.1).
- AC-6 (config part): `TestConfigValidate_SingleReviewerText` passes (R1.9).
- AC-26 (part): the R13.2 Spec-126 items and the CHANGELOG line are pinned.

**Depends on**
None.

## Bead 2: Panel data model and ledger store

**Steps**
1. In `internal/panel/panel.go`, add the new `panel.json` fields at the end of the `Panel` struct: `roster`, `deferrals`, `rebinds`, `telemetry`, `abandon_history`. Extend `Refutation` with `finding`, `finding_sha256`, `ground`, `ac` and `by`. Bead 4 owns `subject_digest`.
2. Add `internal/panel/ledger.go`. It defines the five R5.1 record kinds: `round`, `refutation`, `deferral`, `rebind`, `abandon`. Each record holds `spec` and `panel`. An adhoc record holds `spec: "adhoc"`.
3. Key the store by panel directory. Append a batch of records under one hold of `dispositions.lock`. A same-key record that differs only in `created_at` adds nothing. A same-key record that differs in another field refuses.
4. Add one snapshot writer for `panel.json` (write a temp file, then rename). Only verbs call it.
5. Add `internal/panel/finding.go`. A finding is `ccr[i]` of the latest-round verdict. Its hash is the sha256 of its text. Replace `/tmp/` and `/Users/` paths with `<path>` before storage. A REQUEST_CHANGES with an empty list has one finding `ccr[0]`, its rationale. With an empty rationale too, the slot is INVALID (OQ-4).
6. Add the legacy-marker read (spec Terms) and the R5.3 matcher in `internal/panel/handedit.go`. The matcher compares each verb-written field against the ledger events. It returns the first unbacked field. Bead 6 wires it into the gate.
7. `panel disposition validate`, `check` (`CheckCompleteness`) and `query` (`LoadStore`, the record switch in `loadDispositionFile`) accept the five kinds. `validate` rejects an unknown kind. `panel disposition append` refuses the five kinds, and refuses a manifest on a non-legacy panel (R5.1).
8. Update `.mindspec/domains/workflow/interfaces.md` § Panel Artifact Schema with the new fields and record kinds.

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestLedger_RecordKeys|TestFinding_|TestDispositionQuery_LedgerKinds'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestDispositionValidate_LedgerKinds|TestDispositionCheck_LedgerKinds|TestDispositionAppend_RefusesVerbKinds'` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-13 (store part): `TestLedger_RecordKeys`, `TestDispositionValidate_LedgerKinds`, `TestDispositionCheck_LedgerKinds`, `TestDispositionQuery_LedgerKinds`, `TestDispositionAppend_RefusesVerbKinds` pass (R5.1, R5.2).
- The OQ-4 finding model is in place for AC-17 (R7.4).

**Depends on**
None.

## Bead 3: Create refusals and registration resolution

**Steps**
1. In `internal/panel/create.go`, refuse `create --round N` when round N has verdict files, and refuse a round below LatestRound (from the verdict files). Name `--round <N+1>` and `panel rebind` in the recovery. Leave `panel.json` byte-identical. A round with no verdict files is re-created (R1.1).
2. Refuse an unparseable `panel.json` with no write (R1.3).
3. For `--gate bead`, derive `bead_id` from `--target bead/<id>`. Refuse `--target main`, a `--bead` that differs from the target, and `--bead` on a non-bead gate. Write no file on a refusal (R1.6).
4. In `cmd/mindspec/panel.go`, make `findPanelRegistration` refuse an ambiguous slug and list each match. Accept `--spec` to choose one. Let the checkout copy win over a `.worktrees/` copy of the same spec. `tally` and `verify` print the resolved directory and round. Refuse `create` of a slug that another spec owns, and refuse adhoc and non-adhoc slug collisions (R1.7).
5. Make `tallyExitActionNonBead` print no `--spec` for an adhoc panel and the recorded spec id for a spec panel (R1.8).
6. Set `SilenceUsage` on every `panel` subcommand (R1.9).
7. Change `TestCreate_RepanelOfAbandonedPanelRevivesIt` to use round 2.
8. Register the new recovery templates in `internal/guard/registries.go`.
9. Update `.mindspec/domains/workflow/interfaces.md` § Panel CLI Verb Tree with the refusals and `--spec`.

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestCreate_UnparseablePanelJSONRefuses|TestCreate_RepanelOfAbandonedPanelRevivesIt'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelCreate_RefusesSameRoundWithVerdicts|TestPanelCreate_RefusesOlderRound|TestPanelCreate_SameRoundNoVerdictsAllowed|TestPanelCreate_BeadIDDerivedFromTarget|TestPanelCreate_BeadTargetRefusals|TestFindPanelRegistration_|TestPanelCreate_SlugTakenByOtherSpec|TestPanelCreate_AdhocSlugCollision|TestTallyExitActionNonBead_|TestPanelSubcommands_SilenceUsage'` passes
- [ ] `go test ./internal/complete/... -run TestComplete_FindsDerivedBeadPanel` passes
- [ ] `go test ./internal/guard/... -run TestOpaqueOperandRegistry_` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-1 (R1.1), AC-6 (R1.8, R1.9 SilenceUsage) pass.
- AC-5 (R1.7): the `TestFindPanelRegistration_` tests pass, `TestFindPanelRegistration_PrintsResolvedDir` included.
- AC-2 (R1.3 part): `TestCreate_UnparseablePanelJSONRefuses` passes.
- AC-4 (R1.6 part): `bead_id` derivation, the refusals and `TestComplete_FindsDerivedBeadPanel` pass. Bead 5 adds the 4-slot assertion.

**Depends on**
None.

## Bead 4: Subject-tree staleness

**Steps**
1. Add the `internal/gitutil` primitive `LsTree(workdir, rev string)`. Use `RejectOptionLike` on `rev`. Add one ratchet row.
2. Add `LsTree` to the Executor interface, `MindspecExecutor` and `mock.go`.
3. Add `internal/panel/subject.go`. `Subject(gate, specDir)` returns the R3.1 roots for each gate and both layouts. The digest is the sha256 of the filtered `ls-tree` listing. Every subject excludes exactly `<specs>/*/reviews/**`, `.mindspec/reviews/**`, `review/**` and `.beads/issues.jsonl`. No other `reviews` directory is excluded. An error or an empty listing fails closed.
4. Add `subject_digest` to `Panel` in `internal/panel/panel.go`, directly after `ReviewedHeadSHA`. `create` gets the digest through `CreateInput` (`internal/panel/create.go`) and records it. `create` refuses on an `ls-tree` error or an empty listing.
5. Add gate leg 6 and `GateIO.SubjectTree`. On a SHA mismatch with a recorded digest, compare digests: equal is fresh; different is stale. Equal SHAs make no `SubjectTree` call. A legacy panel keeps today's message byte-for-byte (R3.3).
6. Wire the same `GateIO` into the four consumers: `complete` (`internal/complete/panel_advisory.go`), `panel verify`, `panel tally` (`cmd/mindspec/panel.go`) and `instruct` (`internal/instruct/panelstate.go`) (R3.4).
7. Update `.mindspec/domains/execution/interfaces.md` § Executor Interface (`LsTree`) and `.mindspec/domains/workflow/interfaces.md` § Panel Artifact Schema (`subject_digest`).
8. Append `## Amendment (Spec 130)` to ADR-0037 with two bullets: §4 subject-tree staleness (R3), and the Decision Details cap becomes "three on a SHA mismatch" (R3.3). Add `internal/validate/adr_spec130_subject_test.go` (`TestADRAmendments_Spec130_Subject`).

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestSubject_PerGate|TestSubjectDigest_|TestGate_SubjectTreeStaleness|TestGate_LegacyStaleMessageUnchanged|TestGate_EqualSHANoSubjectCall|TestPanelLeafImports_StdlibPlusTermsafeOnly'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelConsumers_SubjectTreeParity|TestSubjectDigest_FailsClosed'` passes
- [ ] `go test ./internal/lint/... -run 'TestArgvIDOperandGated|TestEnforcementHasNoGitLeaks'` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Subject` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-10 (R3.1, R3.2): `TestSubject_PerGate`, `TestSubjectDigest_ExcludesPanelRootsOnly` and `TestSubjectDigest_FailsClosed` pass. `TestSubjectDigest_ExcludesPanelRootsOnly` has a case where a commit that changes only `.beads/issues.jsonl` gives an equal digest.
- AC-11 (R3.3, R3.4) passes.
- AC-26 (part): the R13.1 §4 subject item and the cap item are pinned.

**Depends on**
None.

## Bead 5: Create writes the round

**Steps**
1. On a round bump, carry entries forward as R1.2 says: "On a non-legacy panel, it carries only the entries that R5.3 matches, copies the roster from the latest `round` event, and names each entry that it drops."
2. On a legacy panel (no ledger events), carry every refutation, deferral and rebind unchanged. Drop none. R1.2 says: "On a legacy panel, the first `create` that writes a `round` event records each carried entry in that event's `carried` field (R5.3 baseline)." This is the baseline of R5.3: "A carried entry matches its baseline in the `carried` field of the first `round` event".
3. On revival of an abandoned panel, append one `abandon_history` entry and clear `abandoned`.
4. Before a bump, archive `BRIEF.md` as `BRIEF-round-<N>.md`. Never overwrite an archive. Tally counts only `.json` slots (R1.4).
5. Render the prior-round region in the new BRIEF: `consolidated-round-<N>.md` with `termsafe.Escape`, or `none recorded` (R1.5).
6. Add `--reviewers N`. Refuse N below 2. Fill N slots from the resolved family list, round-robin (R2.2, R2.3).
7. Write the roster with ids `R1..Rn` from `PanelGateReviewerSlots`. List each slot in the BRIEF as `slot model lens`. A later round carries the roster forward unchanged unless `--reviewers` is given (R2.4, OQ-2).
8. Append the `round` event in the same operation as the `panel.json` write. Set `telemetry: "required"` on new panels (R11.1).
9. Write no `approve_threshold` (R7.5).
10. Extend `TestPanelCreate_BeadIDDerivedFromTarget` to assert 4 slots.
11. Update `.mindspec/domains/workflow/interfaces.md` § Panel CLI Verb Tree (`--reviewers`, roster) and, if `internal/config/config.go` changes, `.mindspec/domains/core/interfaces.md` § Config.

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestCreate_CarriesRefutationsAcrossRoundBump|TestCreate_LegacyBumpRecordsCarried|TestCreate_RevivalAppendsAbandonHistory|TestCreate_ArchivesBriefOnBump|TestCreate_NeverOverwritesArchive|TestCreate_PriorRoundRegion|TestCreate_RosterCarriedForward|TestCreate_ReviewersOverridesCarriedRoster|TestCreate_BriefListsRoster|TestCreate_DropsUnbackedEntries'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelCreate_ReviewersFloor|TestPanelCreate_ReviewersN|TestPanelCreate_NoApproveThreshold|TestPanelCreate_BeadIDDerivedFromTarget'` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-2 (R1.2): `TestCreate_CarriesRefutationsAcrossRoundBump` has a legacy fixture and a non-legacy fixture. Each keeps both entries.
- AC-14 (create part): `TestCreate_LegacyBumpRecordsCarried` passes. A legacy panel with two hand refutations, bumped to round 2, keeps both, and the new `round` event's `carried` field holds both. `TestCreate_DropsUnbackedEntries` passes.
- AC-3 (R1.4, R1.5) passes.
- AC-4 (R2.1 part): 4 slots.
- AC-8 (create part), AC-9 (roster and BRIEF part; `TestCreate_ReviewersOverridesCarriedRoster`: a round-2 `create --reviewers 3` writes a 3-slot roster), AC-18 (`TestPanelCreate_NoApproveThreshold`) pass.

**Depends on**
Bead 1, Bead 2, Bead 3, Bead 4.

## Bead 6: Pass rule, roster tally and hand-edit gate

**Steps**
1. Add `internal/panel/passrule.go`. A panel passes only when it is complete, has no veto, is fresh, has every finding discharged, and is intact (R7.1). The APPROVE count is not an input. Other gate legs stay unchanged. Rework every test fixture that expects a sub-threshold Block (leg 10), in `hostile_fields_test.go` and `refutation_test.go`, so that it blocks on an R7 term.
2. A discharge is pinned to its round and its `finding_sha256`. An item-level discharge clears one finding. A slot-level discharge clears the slot until the verdict file changes (R7.2-R7.4). A refuted REJECT, `hard_block` or INVALID still blocks. The pass rule counts no deferral as a discharge, on any panel, legacy and carried entries included. Bead 11 turns the deferral discharge on. `TestPassRule_DeferralInactive`: a complete, fresh legacy panel with a same-round, text-pinned, hand-added deferral stays unresolved.
3. Treat a recorded `approve_threshold` as legacy. Print the legacy line. It does not change the decision (R7.5).
4. Count roster slots only. An off-roster file counts nothing. An off-roster REJECT or INVALID blocks and names the file. A legacy panel with no roster tallies as today (R2.5).
5. Make a sub-floor `expected_reviewers` a registration error that names `--reviewers 2` and no `/ms-bead-fix` (R2.2).
6. Wire the R5.3 matcher. The check runs before the abandoned leg (3), so a hand-set `abandoned: true` blocks and does not Warn-skip. On a non-legacy panel, a field change with no backing event blocks with `changed outside the panel verbs`. A legacy panel is exempt. A carried entry that matches its `carried` baseline does not block (R5.3).
7. Add `internal/panel/ruleline.go`, one render of the rule line with the INVALID count. Use it in `tally`, `verify` and `instruct` (R8).
8. Remove every `threshold is %d` and `threshold %d` format in `cmd/mindspec`, `internal/panel`, `internal/instruct` and `internal/complete` (AC-19). Do not change other packages (for example `internal/validate/plan.go` "exceeds threshold %d").
9. Update `.mindspec/domains/workflow/interfaces.md` § Panel Artifact Schema with the pass rule and the roster tally.
10. Add bullets to the ADR-0037 `## Amendment (Spec 130)` section: §1 new fields; §3 the pass rule (R7) replaces the N−1 threshold, supersedes the Spec-114 sentences "the approve threshold remains a NECESSARY floor" and "refutation cannot buy a sub-threshold panel past the gate", and retires the Spec-109 `approve_threshold` extension; §7 a Block message and the R8 rule line may name the refute and defer verbs; §8 hand-edit detection (R5.3) detects accidental desync only, is anti-footgun and not anti-adversary, and adds no signing.
11. Add bullets to the ADR-0043 `## Amendment (Spec 130)` section: the new ledger record kinds (R5.1), and the gate reads `round`, `rebind` and `abandon` events (R5.3). Add back-pointers at "No lifecycle verb reads the" and at the Related line "consumes none of them".
12. Append `## Amendment (Spec 130)` to ADR-0044 with the two R13.4 back-pointers (at "The threshold is a" and at "evidence-refuted via the audited refutation procedure").
13. Add `internal/validate/adr_spec130_passrule_test.go` (`TestADRAmendments_Spec130_PassRule`). It pins the items of steps 10-12, the Spec-109 retirement included.

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestPassRule_|TestApproveThreshold_LegacyNotice|TestTally_OffRosterSlotCountsNothing|TestTally_OffRosterRejectBlocks|TestTally_OffRosterInvalidBlocks|TestTally_LegacyNoRosterUnchanged|TestGate_SubFloorRegistrationError|TestGate_HandEditDetected|TestGate_HandChangedEntryPayload|TestGate_OneMarkerBlocks|TestGate_LegacyHandEditExempt|TestGate_CarriedBaseline'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestRuleLine_SharedRender|TestRuleLine_INVALIDCount'` passes
- [ ] `grep -rn -e 'threshold is %d' -e 'threshold %d' cmd/mindspec internal/panel internal/instruct internal/complete --include='*.go' | grep -v _test` prints nothing
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_PassRule` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-17 (R7.1-R7.3), AC-18 (R7.4, R7.5 tally part), AC-19 (R8, the tests and the grep) pass. `TestPassRule_DeferralInactive` passes.
- AC-8 (gate part), AC-9 (tally part), AC-14 (gate tests, a hand-set `abandoned: true` included) pass.
- AC-26 (part): the R13.1 §1/§3/§7/§8 items, the R13.2 ledger-read items and R13.4 are pinned.

**Depends on**
Bead 1, Bead 2, Bead 3, Bead 4.

## Bead 7: Panel rebind

**Steps**
1. Add `cmd/mindspec/panel_rebind.go`: `panel rebind <slug> --reason <text>`. It refuses an empty reason, a panel that fails the parse, abandon, round or R5.3 check, a panel that does not pass R7 with staleness ignored, a reviewed SHA that is not an ancestor of HEAD, and a diff that contains `.mindspec/config.yaml` (R4.1, R4.2).
2. Accept every other changed path. Do not add a config key or a path match. Record the diff paths, minus the R3.1 exclusions, in the entry's `paths` (R4.3).
3. When the recorded SHA equals the tip, print `already fresh` and write nothing (R4.4).
4. Otherwise, append a `rebinds[]` entry and a `rebind` event in one operation. Set the new reviewed SHA and digest. Do not bump the round (R4.4).
5. `complete` writes `panel_rebound_entries`, through the same stage as `panel_refuted` (R4.5).
6. In `internal/lifecycle/landed.go`, add each R5.3-matched `rebinds[].from_sha` to the `reviewedHeadSHAForBead` set. Use exact equality (R3.5, OQ-5).
7. Update `.mindspec/domains/workflow/interfaces.md` § Panel CLI Verb Tree (`rebind`).
8. Add the ADR-0037 §4 rebind bullet. Append `## Amendment (Spec 130)` to ADR-0041 with the datum (c) back-pointer to R3.5. Add `internal/validate/adr_spec130_rebind_test.go` (`TestADRAmendments_Spec130_Rebind`).

**Verification**
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelRebind_OwnershipOnly|TestPanelRebind_AnyPath|TestPanelRebind_Refusals|TestPanelRebind_AlreadyFresh'` passes
- [ ] `go test ./internal/complete/... -run TestComplete_CopiesRebinds` passes
- [ ] `go test ./internal/lifecycle/... -run TestReviewedHeadSHASet_IncludesRebindFrom` passes
- [ ] `go test ./internal/lint/... -run TestProductDiagnosticScan_UnprovableCommandOperandsAreGoverned` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Rebind` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-12 (R4.1-R4.5, R3.5) passes, except `TestToolArtifactPaths_SingleHome` (Bead 9). `TestPanelRebind_OwnershipOnly` uses a diff that changes only a domain `OWNERSHIP.yaml`; the panel is stale before the rebind and passes after it. `TestPanelRebind_AnyPath` uses a diff that changes a `.go` file; the rebind succeeds and the entry's `paths` holds that file. `TestPanelRebind_Refusals` has one case for each R4.1 and R4.2 refusal.
- AC-26 (part): the R13.1 §4 rebind item and R13.3 are pinned.

**Depends on**
Bead 5, Bead 6.

## Bead 8: Refute and abandon verbs

**Steps**
1. Add `cmd/mindspec/panel_refute.go`. It takes `--slot`, `--round`, `--finding`, `--ground`, `--reason`, `--evidence`, `--ac` and `--by`. It writes the refutation entry, the `refutation` event and an `audited-refuted` row (reviewer equals `--slot`) in one lock hold. A rerun adds nothing (R9.1, R9.2).
2. Refute refuses: an APPROVE, REJECT, `hard_block` or INVALID slot; no latest-round verdict file; a stale round; an unknown finding; an empty `--by`; `--evidence` that resolves to nothing; `out_of_acceptance_scope` without a valid `--ac`. It stores redacted text (`<path>`). If the `--evidence` check adds a git exec site, add one ratchet row.
3. Add `cmd/mindspec/panel_abandon.go`. It requires `--reason` and `--by`. It sets `abandoned` and `abandon_reason` and appends the `abandon` event. It writes no manifest (R9.4).
4. Update both copies of `ms-panel-tally` and `ms-panel-run`:
   - Remove every hand-edit instruction (the AC-15 grep set).
   - Name `mindspec panel refute` and `mindspec panel abandon`. Bead 11 adds `mindspec panel defer`.
   - Remove the manifest step: neither `ms-panel-tally` copy contains `disposition-manifest.json`. The skills name `tally` as the manifest writer (R5.4). Bead 10 makes `tally` write it.
   - Replace the N−1 threshold text with the R7 pass rule.
   - Name the roster ids as the verdict prefix in `ms-panel-run` (R5.4, OQ-2).
5. Add `internal/setup/panel_skills_test.go` (`TestPanelSkills_NoHandEditInstructions`). It asserts the AC-15 grep set is absent, `refute` and `abandon` are named, the roster ids are named, and `disposition-manifest.json` is absent. Update the "n-1" rows in `internal/setup/review_discipline_test.go` that the threshold text change removes.
6. Update the `ms-bead-impl` marker text "the spec-130 rule table" in both copies (R13.5). Update its pins: the constant `readinessStagedRule` in `internal/setup/readiness_skill_pin_test.go`, and in `TestReviewDiscipline_AC10StagedRuleMarkerOnly` the allowlist entry `matched: "spec-130"` and the token loop.
7. Add the spec 129 R7.2 status line (R13.5).
8. Update `.mindspec/domains/workflow/interfaces.md` § Panel CLI Verb Tree (`refute`, `abandon`).
9. Add the ADR-0037 §7 bullet: abandon is a verb (R9.4); abandonment stays the audited exit. Append `## Amendment (Spec 130)` to ADR-0045 with the R13.5 status note. Add `internal/validate/adr_spec130_verbs_test.go` (`TestADRAmendments_Spec130_Verbs`). It also pins the spec 129 R7.2 line.

**Verification**
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelRefute_|TestPanelAbandon_Verb'` passes
- [ ] `go test ./internal/setup/... -run 'TestPanelSkills_NoHandEditInstructions|TestReviewDiscipline_|TestMsBeadImplSkill_'` passes
- [ ] `go test ./internal/lint/... -run 'TestArgvIDOperandGated|TestProductDiagnosticScan_UnprovableCommandOperandsAreGoverned'` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Verbs` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-20 (R9.1, R9.2) passes.
- AC-15 (R5.4, refute and abandon part): `TestPanelSkills_NoHandEditInstructions` passes.
- AC-22 (R9.4 part): `TestPanelAbandon_Verb` passes.
- AC-26 (part): R13.5, the spec 129 R7.2 line and the R13.1 §7 abandon item are pinned.

**Depends on**
Bead 5, Bead 6.

## Bead 9: Tracked ledger and the tool-artifact home

**Steps**
1. Add `internal/panel/artifacts.go` with the single home `ToolArtifactPaths`: the exact path `.beads/issues.jsonl` and the pattern `<specs>/*/reviews/*/dispositions.jsonl` for the resolved `<specs>`, with a matcher that uses `path.Match` (R4.6).
2. Make the three consumers use it, and remove their local copies:
   - `internal/panel/gate.go` (`artifactPaths`, `isArtifactPath`) uses the matcher.
   - `internal/next/guard.go` (`artifactPaths`) uses the matcher.
   - `internal/complete/materialize.go` (`materializedArtifactPaths`): `nonArtifactPaths` uses the matcher in place of its exact-key map. `exec.SnapshotWorktreeTree` gets both entries as exclude pathspecs; a git pathspec wildcard matches the pattern.
3. Add `TestToolArtifactPaths_SingleHome`. It asserts that the three consumer files hold no local list and use the home. Add `TestMaterialize_LedgerIsArtifact`: a dirty ledger is not user dirt at materialization.
4. Add the two negation lines to `gitutil.ReviewIgnoreEntries` after the spec-129 entries (R6.1).
5. In `internal/bootstrap/bootstrap.go`, write the negation lines by exact-line presence. Do not use the check-ignore re-append. A second run leaves `.gitignore` byte-identical (R6.2).
6. Add the two lines to this repo's `.gitignore`. Change its R8.4 comment to name option (B) (R6.3). Add the spec 129 R8.4 status line (R6.4).
7. Add `CheckIgnore` to the Executor interface, `MindspecExecutor` and `mock.go`. It calls the existing `gitutil.CheckIgnore`.
8. Add `internal/complete/panel_ledger.go`. Before the bead merge, check each ledger path with `check-ignore` in the spec worktree. Refuse before any side effect, with the R6.6 recovery. After the merge, commit each matched ledger in one `chore(panel): ledger` commit with `exec.CommitPaths`, in the spec worktree, as the `check-ignore` check. This step applies only on the flat layout. If the commit fails, print the `impl approve` recovery line. A legacy panel with no ledger makes no commit and prints no line.
9. In `internal/approve/impl.go`, add a preflight leg. It runs the same `check-ignore` check, with the same refusal. Then it refuses an uncommitted or untracked spec ledger, with the `git -C <spec-worktree>` recovery.
10. Hatches: the two hatches do not skip the R6.6 refusal at `impl approve` (spec text). At `complete`, the hatches also do not skip the `check-ignore` refusal or the ledger commit. The ledger is a tool artifact, not a gate input, and an ignored ledger breaks the later `impl approve` leg.
11. Update `.mindspec/domains/execution/interfaces.md` § Gitignore Ensure and § Executor Interface (`CheckIgnore`), and `.mindspec/domains/workflow/interfaces.md` (the ledger commit and the tool-artifact home).
12. Add ADR-0037 bullets: the Spec-116 `artifactPaths` back-pointer and bullet (R4.6 single home); §8 the Spec-114 carve-out extends to the tracked ledger (R6); the Spec-115 reach note for the R6.6 leg, a third new `impl approve` leg with no hatch; the `check-ignore` call is a `complete` preflight leg outside the cap. Add the ADR-0043 bullet and back-pointer: `dispositions.jsonl` is tracked again (R6). Add `internal/validate/adr_spec130_ledger_test.go` (`TestADRAmendments_Spec130_Ledger`). It also pins the spec 129 R8.4 line.

**Verification**
- [ ] `go test ./internal/panel/... -run TestToolArtifactPaths_SingleHome` passes
- [ ] `go test ./internal/bootstrap/... -run 'TestBootstrap_ReviewsLedgerTracked|TestBootstrap_ReviewsIgnored'` passes
- [ ] `go test ./internal/next/... -run TestNextGuard_LedgerIsArtifact` passes
- [ ] `go test ./internal/complete/... -run 'TestComplete_CommitsLedger|TestComplete_NoLedgerNoCommit|TestLedger_IgnoredPathRefuses|TestMaterialize_LedgerIsArtifact'` passes
- [ ] `go test ./internal/approve/... -run 'TestImplApprove_UncommittedLedgerRefuses|TestImplApprove_IgnoredLedgerRefuses'` passes
- [ ] `go test ./internal/lint/... -run 'TestEnforcementHasNoGitLeaks|TestProductDiagnosticScan_UnprovableCommandOperandsAreGoverned'` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Ledger` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-16 (R6) passes. `TestImplApprove_IgnoredLedgerRefuses` asserts that `impl approve` refuses an ignored untracked ledger, and that its recovery names `mindspec init` and `git -C <spec-worktree>`.
- AC-12 (R4.6 part): `TestToolArtifactPaths_SingleHome` passes for all three consumers.
- AC-26 (part): the R13.1 Spec-116, §8 ledger and R6.6 reach items, the R13.2 "tracked again" item and the spec 129 R8.4 line are pinned.

**Depends on**
Bead 1, Bead 2, Bead 4.

## Bead 10: Core 3 coverage enforcement

**Steps**
1. Make `tally` write the coverage manifest. It lists roster slots only. It appends a new manifest unless the last manifest of that round equals the derivation (R11.2). It writes one on an Allow, and on a stale non-bead registration with covering rows (R11.2).
2. Make `ComputeQ4` and every manifest reader use only the last manifest of each panel (R11.3).
3. Add `internal/complete/panel_core3.go`. Read the per-registration decisions that `panelGate` returns (Bead 12). Before materialization, check each required registration whose returned decision is Allow. Block on a REQUEST_CHANGES slot with no covering row, a missing store, no manifest, a manifest for another round, a manifest that differs from the verdict files, or a malformed record (R11.4). A legacy registration passes with an advisory. A Warn registration (abandoned, missing ref, git error) skips the check with one advisory line (R11.6). Do not derive Warn from the `abandoned` field.
4. In `internal/approve/impl.go`, add a preflight leg on every required registration of the spec, final review included. It runs no gate decision. Before it skips an abandoned or legacy registration, it runs the legacy-marker check and the R5.3 check. A hand-set `abandoned: true` with no `abandon` event blocks. It names the panel and slot (R11.5).
5. Add the approve-side hatch seam `implPanelSkipEnvFn` in `internal/approve/impl.go`. Both legs honour `MINDSPEC_SKIP_PANEL` and `enforcement.panel_gate: false` (OQ-3). `complete` uses `panelSkipEnvFn`.
6. Tests seed refutation, deferral and abandon events with the Bead 2 batch append, not with the verbs.
7. Update `.mindspec/domains/workflow/interfaces.md` § Panel Artifact Schema (manifest rules) and § Lifecycle verb state honesty (the Core 3 legs).
8. Add ADR-0037 bullets: §7 the Core 3 skip rules (R11.5, R11.6); the Spec-115 reach note for the R11.5 leg, which blocks and honours the two hatches; the orphan/obligation refusal keeps its no-hatch rule; the Core 3 leg is a `complete` preflight leg outside the cap. Add ADR-0043 bullets: the binary writes the coverage manifest (R11.2), roster slots only, the `created_at` key; this supersedes "one per terminal panel", "appended ONCE at terminal state" and "Every terminal panel writes its manifest regardless of finding count"; abandonment keeps earlier manifests; every manifest reader uses the last manifest (R11.3); Consequences: `complete` and `impl approve` gate on completeness (R11). Add `internal/validate/adr_spec130_core3_test.go` (`TestADRAmendments_Spec130_Core3`).

**Verification**
- [ ] `go test ./cmd/mindspec/... -run 'TestTally_AppendsManifestOnChange|TestTally_AppendsManifestPerRound|TestTally_WritesManifestOnAllow|TestTally_ManifestWhenOnlyStale'` passes
- [ ] `go test ./internal/panel/... -run TestComputeQ4_LastManifestOnly` passes
- [ ] `go test ./internal/complete/... -run 'TestComplete_Core3'` passes
- [ ] `go test ./internal/approve/... -run 'TestImplApprove_Core3Leg|TestImplApprove_AbandonedSkipIntegrity|TestImplApprove_Core3Hatches'` passes
- [ ] `go test ./internal/lint/... -run TestProductDiagnosticScan_UnprovableCommandOperandsAreGoverned` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Core3` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-24 (R11) passes. `TestComplete_Core3WarnSkips` has an abandoned case and a missing-ref case, each with no manifest. `TestTally_AppendsManifestPerRound`: a round-2 derivation equal to the round-1 manifest appends a round-2 manifest.
- AC-26 (part): the R13.1 §7 Core 3 and R11.5 reach items (the orphan/obligation no-hatch rule included) and the R13.2 manifest items are pinned.

**Depends on**
Bead 5, Bead 6, Bead 9, Bead 12.

## Bead 11: Defer verb, deferral audit and gate-time re-check

**Steps**
1. Add `internal/bead/deferral.go`. It reads a bead's status, parent chain, `discovered-from` edge and description through bd. Add one ratchet row for each new bd call.
2. Add `cmd/mindspec/panel_defer.go`. Defer refuses: a closed or missing bead; the epic, or a child or descendant of it; a bead with no `discovered-from` edge to the epic or the panel's bead; a description without the finding text (R9.3 replacement). It shares the refute slot and round refusals. It refuses on `spec_approve` and adhoc panels, with a recovery that names `refute`. It writes the deferral, the `deferral` event and a `confirmed-deferred` row (R9.3).
3. Add `internal/complete/panel_deferred.go`. `complete` writes `panel_deferred` with one entry per effective deferral, taken from each Allow decision that `panelGate` returns, as `AppliedRefutations` is today (`panel.Decision`, `internal/panel/gate.go`). It writes the audit through the pending-marker protocol. Each entry holds `slot`, `round`, `finding`, `finding_text`, `finding_sha256`, `bead` and `by` (R10.1).
4. Re-check each effective deferral through `internal/bead/deferral.go`, at `complete` and in the `impl approve` preflight. Block on re-parent under the epic, deletion, a removed `discovered-from` edge, removed finding text, a closed bead, or a bd error. Name the slot, the finding and the bead (R10.2).
5. Turn the deferral discharge on in `internal/panel/passrule.go` (spec Terms: effective deferral; R7.3, R7.4), together with `panel defer`, the R10.1 audit and both re-checks. Replace `TestPassRule_DeferralInactive` with `TestPassRule_DeferralActive`.
6. Source of the deferrals (R10.2):
   - At `complete`: each registration that `panel.ForBead` matches.
   - At `impl approve`, a non-bead registration: its `panel.json` and ledger.
   - At `impl approve`, a bead panel: its bead's `panel_deferred` audit. A `panel refute` on the bead panel after `complete` does not change this re-check.
7. Recovery for a closed bead: at `complete` and for a non-bead registration at `impl approve`, name `bd reopen <B>` or `mindspec panel refute`. The refute line carries the deferral's `--finding`, and has no `--finding` for a slot-level deferral. For a bead panel at `impl approve`, name `bd reopen <B>` only.
8. A refutation of the same finding and text supersedes the deferral. Skip deferrals on abandoned registrations; at `impl approve`, the R11.5 integrity check runs before this skip. Compare redacted text, so a raw `/tmp/x` in the bead description passes.
9. Both legs honour the two hatches through `panelSkipEnvFn` and `implPanelSkipEnvFn` (OQ-3).
10. Update both copies of `ms-panel-tally` and `ms-panel-run` to name `mindspec panel defer`. Extend `TestPanelSkills_NoHandEditInstructions` to require it.
11. Add `TestLedger_RecordKinds`: each verb appends its R5.1 record with `spec` and `panel`.
12. Add `TestPanelAbandon_KeepsManifest`: after `tally` writes a manifest, `panel abandon` keeps it, and `ComputeQ4` still counts its slots.
13. Update `.mindspec/domains/workflow/interfaces.md` § Panel CLI Verb Tree (`defer`, the re-check) and `.mindspec/domains/execution/interfaces.md` (`internal/bead` deferral reads).
14. Add ADR-0037 bullets: §1 the verbs are the only writers; §8 the Spec-114 carve-out extends to the `panel_deferred` audit (R10.1); the Spec-115 reach note for the R10.2 leg, which blocks and honours the two hatches; the R10.2 re-check is a `complete` preflight leg outside the cap. Add the ADR-0043 bullet: this supersedes "Writes go through ONE canonical Go verb", because `refute`, `defer` and `tally` also write the store. Add `internal/validate/adr_spec130_deferral_test.go` (`TestADRAmendments_Spec130_Deferral`).

**Verification**
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelDefer_|TestLedger_RecordKinds|TestPanelAbandon_KeepsManifest'` passes
- [ ] `go test ./internal/bead/... -run TestDeferral_` passes
- [ ] `go test ./internal/complete/... -run 'TestComplete_PanelDeferredAudit|TestGateTime_'` passes
- [ ] `go test ./internal/approve/... -run TestGateTime_` passes
- [ ] `go test ./internal/panel/... -run TestPassRule_DeferralActive` passes
- [ ] `go test ./internal/setup/... -run TestPanelSkills_NoHandEditInstructions` passes
- [ ] `go test ./internal/lint/... -run 'TestArgvIDOperandGated|TestEnforcementHasNoGitLeaks|TestProductDiagnosticScan_UnprovableCommandOperandsAreGoverned'` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-21 (R9.1, R9.3): `TestPanelDefer_BDChecks`, `TestPanelDefer_SharedRefusals`, `TestPanelDefer_NoEpicGates`, `TestPanelDefer_Writes` pass. `TestPanelDefer_BDChecks` has a descendant (grandchild) case.
- AC-23 (R10) passes. `TestGateTime_OpenBeadDeferralResolves`: a deferral to an open bead resolves its finding and `complete` Allows. `TestGateTime_DeferThenCloseBlocks` writes a deferral to an open bead through the `panel defer` write path, then closes the bead or removes its `discovered-from` edge. `complete` blocks before materialization.
- AC-17 (deferral part): `TestPassRule_DeferralActive` passes.
- AC-15 (defer part): `TestPanelSkills_NoHandEditInstructions` requires `mindspec panel defer`.
- AC-22 (part): `TestPanelAbandon_KeepsManifest` passes.
- AC-13 (verb part): `TestLedger_RecordKinds` passes.
- AC-26: every `TestADRAmendments_Spec130` pin merged so far passes.

**Depends on**
Bead 7, Bead 8, Bead 10.

## Bead 12: Landed reviewed-SHA keys

**Steps**
1. Do not change the `CompleteBead` signature. Add an optional interface `LandedReviewSetter` with `SetLandedReview(sha, panel string)` in `internal/executor/executor.go`. Implement it on `MindspecExecutor` and on the mock executor (`internal/executor/mock.go`).
2. Make `panelGate` (`internal/complete/panel_advisory.go`) return the decision (Allow or Warn) of every matched registration, in scan order, each with its registration. The first Allow is the landed-key source. Today it returns `(matched, nil)` for an Allow, for a Warn and for `panel_gate: false`. Update every `panelGate` caller. When a returned decision is Allow, `complete` calls `SetLandedReview` once, from the first Allow, before `exec.CompleteBead`. The value of `panel` is `<slug>@r<N>`. A Warn (abandoned panel, hatch, missing ref), `panel_gate: false` or no panel makes no call.
3. `ensureLandedBinding` writes `mindspec_landed_reviewed_sha` and `mindspec_landed_review_panel` in the same `mergeBindingFn` call as the binding, on the `CompleteBead` path only. With no setter call, it writes neither key. `FinalizeEpic` keeps the keys unchanged (R12).
4. Update `.mindspec/domains/execution/interfaces.md` § Executor Interface (`LandedReviewSetter`, the two keys) and `.mindspec/domains/workflow/interfaces.md` (the `complete` call). In `.mindspec/domains/workflow/architecture.md`, replace the description of the `panelGate` nil fail-open return with the new return (the decision of each matched registration).
5. Add the ADR-0037 §8 bullet: the Spec-114 carve-out extends to the R12 keys. Add `internal/validate/adr_spec130_keys_test.go` (`TestADRAmendments_Spec130_Keys`).

**Verification**
- [ ] `go test ./internal/executor/... -run 'TestEnsureLandedBinding_'` passes
- [ ] `go test ./internal/complete/... -run TestComplete_LandedReviewOnAllowOnly` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Keys` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-25 (R12) passes. `TestComplete_LandedReviewOnAllowOnly` gets its Allow from `setupPanelGateRepo` and records calls through an executor wrapper that it passes to `Run`. It asserts one `SetLandedReview(<reviewed_head_sha>, <slug>@r<N>)` call before `CompleteBead` on an Allow. It asserts no call with no panel, with an abandoned panel, with `panelSkipEnvFn` true, with `panel_gate: false`, and with a missing reviewed ref.
- AC-26 (part): the R13.1 §8 R12-keys item is pinned.

**Depends on**
Bead 4.

## Carrier-brief resolutions

- **Ordering**: R6 (Bead 9) depends on R3 (Bead 4). R1.1 (Bead 3) lands before R4 (Bead 7, through Bead 5). R9.3 `defer` lands with R10.1 and R10.2 (Bead 11). No merged state has a deferral discharge without the gate-time re-check. Before Bead 11, the R7 pass rule counts no deferral as a discharge, on any panel, legacy and carried entries included. Bead 11 turns it on together with `panel defer`, the R10.1 audit and both re-checks. Core 3 (Bead 10) and the R10.1 audit (Bead 11) read the gate decisions that Bead 12 returns.
- **ADR landing**: each ADR text change lands in the bead that ships its code, or in a bead that depends on that bead. The first bead that touches an ADR appends `## Amendment (Spec 130)`; later beads add bullets to it. ADR-0037: Bead 4 appends; Beads 6, 7, 8, 9, 10, 11 and 12 depend on Bead 4 and add bullets. ADR-0043: Bead 1 appends; Beads 6, 9, 10 and 11 depend on Bead 1 and add bullets. ADR-0041 (Bead 7), ADR-0044 (Bead 6) and ADR-0045 (Bead 8) have one bead each. Each bead pins only its own text. The ADR-0043 `## Amendment (Spec 126)` heading stays (`TestReviewDiscipline_ADR0043Amendment`).
- **Outside the cap**: the three `complete` preflight legs land with their code: the `check-ignore` call (Bead 9), Core 3 (Bead 10) and the R10.2 re-check (Bead 11). Each bead adds its own cap bullet.
- **Hatches at `complete`**: the hatches do not skip the `check-ignore` refusal or the ledger commit (Bead 9).
- **Bead count and chain**: 12 beads, split by mechanism. Longest chain: 4 beads (for example Bead 4 → Bead 5 → Bead 7 → Bead 11, Bead 4 → Bead 9 → Bead 10 → Bead 11, or Bead 4 → Bead 12 → Bead 10 → Bead 11). Beads 1, 2, 3 and 4 start in parallel. Bead 12 starts after Bead 4. Bead 12 is not a leaf: Bead 10 depends on it.
- **Dogfood**: bead panels run on the installed v0.16.0 binary. No branch build is installed during the run.
- **OQ-1**: Bead 1. **OQ-2**: Bead 5 (roster ids) and Bead 8 (skill text). **OQ-3**: Beads 10 and 11. **OQ-4**: Bead 2. **OQ-5**: Bead 7.

## Provenance

In the Verification column, `\|` is table escaping for the regex `|`.

| AC | Bead | Verification |
|----|------|--------------|
| AC-1 | 3 | `go test ./cmd/mindspec/... -run 'TestPanelCreate_RefusesSameRoundWithVerdicts\|TestPanelCreate_RefusesOlderRound\|TestPanelCreate_SameRoundNoVerdictsAllowed'`; `go test ./internal/panel/... -run TestCreate_RepanelOfAbandonedPanelRevivesIt` |
| AC-2 | 3, 5 | `go test ./internal/panel/... -run 'TestCreate_UnparseablePanelJSONRefuses\|TestCreate_CarriesRefutationsAcrossRoundBump\|TestCreate_RevivalAppendsAbandonHistory'` |
| AC-3 | 5 | `go test ./internal/panel/... -run 'TestCreate_ArchivesBriefOnBump\|TestCreate_NeverOverwritesArchive\|TestCreate_PriorRoundRegion'` |
| AC-4 | 3, 5 | `go test ./cmd/mindspec/... -run 'TestPanelCreate_BeadIDDerivedFromTarget\|TestPanelCreate_BeadTargetRefusals'`; `go test ./internal/complete/... -run TestComplete_FindsDerivedBeadPanel` |
| AC-5 | 3 | `go test ./cmd/mindspec/... -run 'TestFindPanelRegistration_\|TestPanelCreate_SlugTakenByOtherSpec\|TestPanelCreate_AdhocSlugCollision'` |
| AC-6 | 1, 3 | `go test ./cmd/mindspec/... -run 'TestTallyExitActionNonBead_\|TestPanelSubcommands_SilenceUsage'`; `go test ./internal/config/... -run TestConfigValidate_SingleReviewerText` |
| AC-7 | 1 | `go test ./internal/config/... -run TestPanelGateDefaults_` |
| AC-8 | 5, 6 | `go test ./cmd/mindspec/... -run 'TestPanelCreate_ReviewersFloor\|TestPanelCreate_ReviewersN'`; `go test ./internal/panel/... -run TestGate_SubFloorRegistrationError` |
| AC-9 | 5, 6 | `go test ./internal/panel/... -run 'TestCreate_RosterCarriedForward\|TestCreate_ReviewersOverridesCarriedRoster\|TestCreate_BriefListsRoster\|TestTally_OffRosterSlotCountsNothing\|TestTally_OffRosterRejectBlocks\|TestTally_OffRosterInvalidBlocks\|TestTally_LegacyNoRosterUnchanged'` |
| AC-10 | 4 | `go test ./internal/panel/... ./cmd/mindspec/... -run 'TestSubject_PerGate\|TestSubjectDigest_'` |
| AC-11 | 4 | `go test ./internal/panel/... -run 'TestGate_SubjectTreeStaleness\|TestGate_LegacyStaleMessageUnchanged\|TestGate_EqualSHANoSubjectCall'`; `go test ./cmd/mindspec/... -run TestPanelConsumers_SubjectTreeParity` |
| AC-12 | 7, 9 | `go test ./cmd/mindspec/... -run TestPanelRebind_`; `go test ./internal/complete/... -run TestComplete_CopiesRebinds`; `go test ./internal/lifecycle/... -run TestReviewedHeadSHASet_IncludesRebindFrom`; `go test ./internal/panel/... -run TestToolArtifactPaths_SingleHome` |
| AC-13 | 2, 11 | `go test ./internal/panel/... -run 'TestLedger_RecordKeys\|TestDispositionQuery_LedgerKinds'`; `go test ./cmd/mindspec/... -run 'TestLedger_RecordKinds\|TestDispositionValidate_LedgerKinds\|TestDispositionCheck_LedgerKinds\|TestDispositionAppend_RefusesVerbKinds'` |
| AC-14 | 5, 6 | `go test ./internal/panel/... -run 'TestGate_HandEditDetected\|TestGate_HandChangedEntryPayload\|TestGate_OneMarkerBlocks\|TestGate_LegacyHandEditExempt\|TestGate_CarriedBaseline\|TestCreate_DropsUnbackedEntries\|TestCreate_LegacyBumpRecordsCarried'` |
| AC-15 | 8, 11 | `go test ./internal/setup/... -run TestPanelSkills_NoHandEditInstructions` |
| AC-16 | 9 | `go test ./internal/bootstrap/... -run 'TestBootstrap_ReviewsLedgerTracked\|TestBootstrap_ReviewsIgnored'`; `go test ./internal/next/... -run TestNextGuard_LedgerIsArtifact`; `go test ./internal/complete/... -run 'TestComplete_CommitsLedger\|TestComplete_NoLedgerNoCommit\|TestLedger_IgnoredPathRefuses'`; `go test ./internal/approve/... -run 'TestImplApprove_UncommittedLedgerRefuses\|TestImplApprove_IgnoredLedgerRefuses'` |
| AC-17 | 6, 11 | `go test ./internal/panel/... -run 'TestPassRule_EachTermBlocks\|TestPassRule_ZeroApprovePasses\|TestPassRule_NeverDischarged\|TestPassRule_EmptyRCInvalid\|TestPassRule_RoundPinned\|TestPassRule_FindingTextPinned\|TestPassRule_DeferralActive'` |
| AC-18 | 5, 6 | `go test ./internal/panel/... -run 'TestPassRule_ItemLevelDischarge\|TestPassRule_SlotLevelVoidedByChange\|TestApproveThreshold_LegacyNotice'`; `go test ./cmd/mindspec/... -run TestPanelCreate_NoApproveThreshold` |
| AC-19 | 6 | `go test ./cmd/mindspec/... -run 'TestRuleLine_SharedRender\|TestRuleLine_INVALIDCount'`; the AC-19 grep prints nothing |
| AC-20 | 8 | `go test ./cmd/mindspec/... -run TestPanelRefute_` |
| AC-21 | 11 | `go test ./cmd/mindspec/... -run TestPanelDefer_`; `go test ./internal/bead/... -run TestDeferral_` |
| AC-22 | 8, 11 | `go test ./cmd/mindspec/... -run 'TestPanelAbandon_Verb\|TestPanelAbandon_KeepsManifest'` |
| AC-23 | 11 | `go test ./internal/complete/... ./internal/approve/... -run 'TestComplete_PanelDeferredAudit\|TestGateTime_'` |
| AC-24 | 10 | `go test ./cmd/mindspec/... -run 'TestTally_AppendsManifestOnChange\|TestTally_AppendsManifestPerRound\|TestTally_WritesManifestOnAllow\|TestTally_ManifestWhenOnlyStale'`; `go test ./internal/panel/... -run TestComputeQ4_LastManifestOnly`; `go test ./internal/complete/... -run TestComplete_Core3`; `go test ./internal/approve/... -run 'TestImplApprove_Core3Leg\|TestImplApprove_AbandonedSkipIntegrity\|TestImplApprove_Core3Hatches'` |
| AC-25 | 12 | `go test ./internal/executor/... -run TestEnsureLandedBinding_`; `go test ./internal/complete/... -run TestComplete_LandedReviewOnAllowOnly` |
| AC-26 | 1, 4, 6, 7, 8, 9, 10, 11, 12 | `go test ./internal/validate/... -run TestADRAmendments_Spec130` |
| AC-27 | all | `gofmt -l .`; `go build ./... && go test ./... -short`; the boundary tests run in their own packages: `go test ./internal/lint/... -run 'TestEnforcementHasNoGitLeaks\|TestProductDiagnosticScan_UnprovableCommandOperandsAreGoverned'` and `go test ./internal/panel/... -run TestPanelLeafImports_StdlibPlusTermsafeOnly` |
