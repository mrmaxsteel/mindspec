---
status: Draft
spec_id: 130-panel-substrate-integrity
version: "1"
adr_citations:
  - ADR-0030
  - ADR-0035
  - ADR-0037
  - ADR-0041
  - ADR-0043
  - ADR-0044
  - ADR-0045
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/config/config.go
      - internal/config/config_test.go
      - internal/setup/review_discipline_test.go
      - plugins/mindspec/skills/ms-panel-run/SKILL.md
      - .claude/skills/ms-panel-run/SKILL.md
      - .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md
      - CHANGELOG.md
      - internal/validate/adr_spec130_sizes_test.go
  - id: 2
    depends_on: []
    key_file_paths:
      - internal/panel/panel.go
      - internal/panel/ledger.go
      - internal/panel/ledger_test.go
      - internal/panel/finding.go
      - internal/panel/finding_test.go
      - internal/panel/handedit.go
      - internal/panel/disposition.go
      - internal/panel/disposition_store.go
      - cmd/mindspec/panel_disposition.go
      - cmd/mindspec/panel_disposition_store.go
      - cmd/mindspec/panel_disposition_store_test.go
  - id: 3
    depends_on: []
    key_file_paths:
      - internal/panel/create.go
      - internal/panel/create_test.go
      - cmd/mindspec/panel.go
      - cmd/mindspec/panel_test.go
      - cmd/mindspec/root.go
      - internal/complete/panel_advisory.go
      - internal/complete/panel_gate_e2e_test.go
      - internal/guard/registries.go
  - id: 4
    depends_on: []
    key_file_paths:
      - internal/panel/subject.go
      - internal/panel/subject_test.go
      - internal/panel/gate.go
      - internal/panel/panel_decision_test.go
      - internal/gitutil/gitops.go
      - internal/executor/executor.go
      - internal/executor/mindspec_executor.go
      - internal/executor/mock.go
      - internal/complete/panel_advisory.go
      - internal/instruct/panelstate.go
      - cmd/mindspec/panel.go
      - internal/lint/ratchet_argv_table_test.go
      - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
      - internal/validate/adr_spec130_subject_test.go
  - id: 5
    depends_on: [1, 2, 3, 4]
    key_file_paths:
      - internal/panel/create.go
      - internal/panel/create_test.go
      - internal/panel/panel.go
      - internal/config/config.go
      - cmd/mindspec/panel.go
      - cmd/mindspec/panel_test.go
  - id: 6
    depends_on: [2, 3, 4]
    key_file_paths:
      - internal/panel/passrule.go
      - internal/panel/passrule_test.go
      - internal/panel/ruleline.go
      - internal/panel/gate.go
      - internal/panel/tally.go
      - internal/panel/tally_test.go
      - internal/panel/handedit.go
      - cmd/mindspec/panel.go
      - internal/instruct/panelstate.go
      - internal/complete/panel_advisory.go
      - .mindspec/domains/workflow/interfaces.md
      - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
      - .mindspec/adr/ADR-0044-panel-review-conduct.md
      - internal/validate/adr_spec130_passrule_test.go
  - id: 7
    depends_on: [5, 6]
    key_file_paths:
      - cmd/mindspec/panel_rebind.go
      - cmd/mindspec/panel_rebind_test.go
      - internal/config/config.go
      - internal/panel/gate.go
      - internal/complete/panel_advisory.go
      - internal/complete/panel_refuted_test.go
      - internal/lifecycle/landed.go
      - internal/lifecycle/landed_test.go
      - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
      - .mindspec/adr/ADR-0041-gate-before-mutate.md
      - internal/validate/adr_spec130_rebind_test.go
  - id: 8
    depends_on: [5, 6]
    key_file_paths:
      - cmd/mindspec/panel_refute.go
      - cmd/mindspec/panel_defer.go
      - cmd/mindspec/panel_abandon.go
      - cmd/mindspec/panel_verbs_test.go
      - internal/bead/deferral.go
      - internal/bead/deferral_test.go
      - plugins/mindspec/skills/ms-panel-tally/SKILL.md
      - plugins/mindspec/skills/ms-panel-run/SKILL.md
      - plugins/mindspec/skills/ms-bead-impl/SKILL.md
      - .claude/skills/ms-panel-tally/SKILL.md
      - .claude/skills/ms-panel-run/SKILL.md
      - .claude/skills/ms-bead-impl/SKILL.md
      - internal/setup/review_discipline_test.go
      - internal/setup/panel_skills_test.go
      - .mindspec/adr/ADR-0045-binary-owns-sequence-and-rules.md
      - .mindspec/specs/129-readiness-tracker-honesty/spec.md
      - internal/validate/adr_spec130_verbs_test.go
  - id: 9
    depends_on: [2, 4]
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
      - internal/executor/executor.go
      - internal/executor/mindspec_executor.go
      - internal/executor/mock.go
      - internal/complete/panel_ledger.go
      - internal/complete/panel_ledger_test.go
      - internal/complete/complete.go
      - internal/approve/impl.go
      - internal/approve/impl_test.go
      - .mindspec/domains/execution/interfaces.md
      - .mindspec/specs/129-readiness-tracker-honesty/spec.md
      - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
      - .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md
      - internal/validate/adr_spec130_ledger_test.go
  - id: 10
    depends_on: [5, 6, 9]
    key_file_paths:
      - internal/panel/disposition_store.go
      - internal/panel/disposition_query.go
      - internal/panel/disposition_query_test.go
      - cmd/mindspec/panel.go
      - cmd/mindspec/panel_test.go
      - internal/complete/panel_core3.go
      - internal/complete/panel_core3_test.go
      - internal/complete/complete.go
      - internal/approve/impl.go
      - internal/approve/impl_test.go
      - plugins/mindspec/skills/ms-panel-tally/SKILL.md
      - .claude/skills/ms-panel-tally/SKILL.md
      - .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md
      - internal/validate/adr_spec130_core3_test.go
  - id: 11
    depends_on: [7, 8, 10]
    key_file_paths:
      - internal/complete/panel_deferred.go
      - internal/complete/panel_deferred_test.go
      - internal/complete/complete.go
      - internal/approve/impl.go
      - internal/approve/impl_test.go
      - internal/bead/deferral.go
      - cmd/mindspec/panel_verbs_test.go
      - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
      - internal/validate/adr_spec130_deferral_test.go
  - id: 12
    depends_on: []
    key_file_paths:
      - internal/executor/mindspec_executor.go
      - internal/executor/merge_binding_test.go
      - internal/complete/complete.go
---
# Plan: 130-panel-substrate-integrity

## ADR Fitness

- **ADR-0030 (executor boundary)**: stays fit. `internal/complete` and `internal/approve` get `ls-tree` and `check-ignore` through two new Executor methods (Beads 4 and 9). They never import `gitutil`. `TestEnforcementHasNoGitLeaks` stays green.
- **ADR-0035 (agent error contract)**: stays fit. Every new refusal uses `guard.NewFailure` with one `recovery:` line. New recovery templates go into `internal/guard/registries.go`.
- **ADR-0037 (panel gate contract)**: amended. Bead 6 appends the `## Amendment (Spec 130)` section (§1 fields, §3 pass rule, §8 hand-edit scope). Other beads add inline back-pointers in their own paragraphs: Bead 4 (§4 subject tree, the Decision Details cap), Bead 7 (§4 rebind), Bead 8 (§1 verbs are the only writers), Bead 9 (Spec-116 `artifactPaths`), Bead 11 (§7, Spec-115 reach, §8 carve-out).
- **ADR-0041 (gate before mutate)**: amended. Bead 7 adds a back-pointer at datum (c) to R3.5. All verbs check before they write.
- **ADR-0043 (disposition store)**: amended. Bead 10 appends the `## Amendment (Spec 130)` section. Bead 1 adds the inline back-pointer at the Spec-126 bullet and keeps the `## Amendment (Spec 126)` heading. Bead 8 adds a back-pointer at "ONE canonical Go verb". Bead 9 adds the "tracked again" note.
- **ADR-0044 (review conduct)**: stays fit. Bead 6 adds the two R13.4 back-pointers.
- **ADR-0045 (binary owns rules)**: amended. Bead 8 adds the R13.5 status note. Bead 8 moves the refute and abandon procedures out of skills into verbs, so it can cite this ADR.

Each bead that edits an ADR adds its own pin file `internal/validate/adr_spec130_<topic>_test.go`. Each pin function name starts with `TestADRAmendments_Spec130`. Thus `-run TestADRAmendments_Spec130` runs all pins (AC-26).

## Testing Strategy

- **Unit tests** in `internal/panel` use fixture panel dirs and a fake `GateIO`. `internal/panel` stays a leaf: no git, no `os/exec` (`TestPanelLeafImports_StdlibPlusTermsafeOnly`).
- **Integration tests** in `cmd/mindspec`, `internal/complete` and `internal/approve` use real temp git repos. Reuse `setupPanelGateRepo` (`internal/complete/panel_gate_e2e_test.go`) and `fakeBdOnPath` (`internal/complete/fault_injection_realgit_test.go`). The `defer` checks use a fake `bd` on PATH.
- **Hatch seams**: tests set `panelSkipEnvFn` (`internal/complete/panel_advisory.go`). They do not set real environment variables.
- **Argv ratchet**: each new exec site adds one row to `internal/lint/ratchet_argv_table_test.go` (`TestArgvIDOperandGated`). When two parallel beads add rows, merge them as a union.
- **Every bead** ends with `gofmt -l .` (empty output) and `go build ./... && go test ./... -short` (AC-27).
- **Dogfood boundary**: this spec changes the panel gate the lane itself uses; bead panels run on the installed v0.16.0 binary, not a spec-branch build; no branch build is installed during the run.

## Bead 1: Per-gate default panel sizes

**Steps**
1. In `internal/config/config.go`, resolve per-gate default sizes when a gate has no reviewer list and `panel.reviewers` is absent: `spec_approve` 6, `plan_approve` 6, `bead` 4, `final_review` 6, `adhoc` 6. Fill families round-robin: claude, codex, claude, and so on (R2.1). An explicit `panel.reviewers` applies to every gate without its own list (OQ-1).
2. Let a config with no `panel.reviewers` and `approve_threshold: 3` load. Let a config with only `panel.gates.bead.approve_threshold: 5` load.
3. Change the single-reviewer error text in `validateOrchestration` and `validateGates`. The text says "cannot pass" and not "always-pass" (R1.9).
4. Update the spec-112 pins: the `DefaultConfig` 3+3 assertions in `internal/config/config_test.go` (`TestLoad_GatesAbsentByteIdentical109` and its siblings).
5. Update the spec-126 pins in `internal/setup/review_discipline_test.go`: the "shipped default: 6 reviewers" and "n-1" fragment rows, and `TestReviewDiscipline_AC1PanelSizeConsistency`. Keep `TestReviewDiscipline_ADR0043Amendment`.
6. Replace the sentence "`DefaultConfig` panel VALUES are unchanged" in both `ms-panel-run` copies.
7. In ADR-0043, add an inline back-pointer at the bullet "The shipped default is 6 / `n-1`, unconditionally". Keep the `## Amendment (Spec 126)` heading.
8. Add a `CHANGELOG.md` `## [Unreleased]` line that names the 6/6/4/6 change.
9. Add `internal/validate/adr_spec130_sizes_test.go` (`TestADRAmendments_Spec130_Sizes`). It pins the ADR-0043 back-pointer and the CHANGELOG line.

**Verification**
- [ ] `go test ./internal/config/... -run 'TestPanelGateDefaults_|TestConfigValidate_SingleReviewerText|TestLoad_GatesAbsentByteIdentical109'` passes
- [ ] `go test ./internal/setup/... -run TestReviewDiscipline_` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Sizes` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-7: `TestPanelGateDefaults_PerGateSizes`, `TestPanelGateDefaults_RoundRobinFamilies`, `TestPanelGateDefaults_ExplicitGlobalWins`, `TestPanelGateDefaults_NoGateAndGateThreshold` pass (R2.1).
- AC-6 (config part): `TestConfigValidate_SingleReviewerText` passes (R1.9).
- AC-26 (part): R13.2 Spec-126 back-pointer and the CHANGELOG line are pinned.

**Depends on**
None.

## Bead 2: Panel data model and ledger store

**Steps**
1. In `internal/panel/panel.go`, add the new `panel.json` fields at the end of the `Panel` struct: `roster`, `deferrals`, `rebinds`, `telemetry`, `abandon_history`. Extend `Refutation` with `finding`, `finding_sha256`, `ground`, `ac` and `by`. Bead 4 owns `subject_digest`.
2. Add `internal/panel/ledger.go`. It defines the five R5.1 record kinds: `round`, `refutation`, `deferral`, `rebind`, `abandon`. Each record holds `spec` and `panel`; an adhoc record holds `spec: "adhoc"`.
3. Key the store by panel directory. Append a batch of records under one hold of `dispositions.lock`. A same-key record that differs only in `created_at` adds nothing. A same-key record that differs in another field refuses.
4. Add one snapshot writer for `panel.json` (write a temp file, then rename). Only verbs call it.
5. Add `internal/panel/finding.go`. A finding is `ccr[i]` of the latest-round verdict. Its hash is the sha256 of its text. Replace `/tmp/` and `/Users/` paths with `<path>` before storage. A REQUEST_CHANGES with an empty list has one finding `ccr[0]`, its rationale. With an empty rationale too, the slot is INVALID (OQ-4).
6. Add the legacy-marker read (spec Terms) and the R5.3 matcher in `internal/panel/handedit.go`. The matcher compares each verb-written field against the ledger events. It returns the first unbacked field. Bead 6 wires it into the gate.
7. `panel disposition validate` accepts the five kinds and rejects an unknown kind. `panel disposition append` refuses the five kinds, and refuses a manifest on a non-legacy panel.

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestLedger_RecordKeys|TestFinding_'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestDispositionValidate_LedgerKinds|TestDispositionAppend_RefusesVerbKinds'` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-13 (store part): `TestLedger_RecordKeys`, `TestDispositionValidate_LedgerKinds`, `TestDispositionAppend_RefusesVerbKinds` pass (R5.1, R5.2).
- OQ-4 finding model is in place for AC-17 (R7.4).

**Depends on**
None.

## Bead 3: Create refusals and registration resolution

**Steps**
1. In `internal/panel/create.go`, refuse `create --round N` when round N has verdict files, and refuse a round below the recorded round. Name `--round <N+1>` and `panel rebind` in the recovery. Leave `panel.json` byte-identical. A round with no verdict files is re-created (R1.1).
2. Refuse an unparseable `panel.json` with no write (R1.3).
3. For `--gate bead`, derive `bead_id` from `--target bead/<id>`. Refuse `--target main`, a `--bead` that differs from the target, and `--bead` on a non-bead gate. Write no file on a refusal (R1.6).
4. In `cmd/mindspec/panel.go`, make `findPanelRegistration` refuse an ambiguous slug and list each match. Accept `--spec` to choose one. Let the checkout copy win over a `.worktrees/` copy of the same spec. Refuse `create` of a slug that another spec owns, and refuse adhoc and non-adhoc slug collisions (R1.7).
5. Make `tallyExitActionNonBead` print no `--spec` for an adhoc panel and the recorded spec id for a spec panel (R1.8).
6. Set `SilenceUsage` on every `panel` subcommand (R1.9).
7. Change `TestCreate_RepanelOfAbandonedPanelRevivesIt` to use round 2.
8. Register the new recovery templates in `internal/guard/registries.go`.

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestCreate_UnparseablePanelJSONRefuses|TestCreate_RepanelOfAbandonedPanelRevivesIt'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelCreate_RefusesSameRoundWithVerdicts|TestPanelCreate_RefusesOlderRound|TestPanelCreate_SameRoundNoVerdictsAllowed|TestPanelCreate_BeadIDDerivedFromTarget|TestPanelCreate_BeadTargetRefusals|TestFindPanelRegistration_|TestPanelCreate_SlugTakenByOtherSpec|TestPanelCreate_AdhocSlugCollision|TestTallyExitActionNonBead_|TestPanelSubcommands_SilenceUsage'` passes
- [ ] `go test ./internal/complete/... -run TestComplete_FindsDerivedBeadPanel` passes
- [ ] `go test ./internal/guard/... -run TestOpaqueOperandRegistry_` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-1 (R1.1), AC-5 (R1.7), AC-6 (R1.8, R1.9 SilenceUsage) pass.
- AC-2 (R1.3 part): `TestCreate_UnparseablePanelJSONRefuses` passes.
- AC-4 (R1.6 part): `bead_id` derivation, the refusals and `TestComplete_FindsDerivedBeadPanel` pass. Bead 5 adds the 4-slot assertion.

**Depends on**
None.

## Bead 4: Subject-tree staleness

**Steps**
1. Add `internal/gitutil` primitive `LsTree(workdir, rev string)`. Use `RejectOptionLike` on `rev`. Add one ratchet row.
2. Add `LsTree` to the Executor interface, `MindspecExecutor` and `mock.go`.
3. Add `internal/panel/subject.go`: `Subject(gate, specDir)` returns the R3.1 path set for each gate and both layouts. The digest is the sha256 of the filtered `ls-tree` listing. It excludes panel roots only. An error or an empty listing fails closed.
4. Add `subject_digest` to `Panel`, directly after `ReviewedHeadSHA`. `create` records it and refuses on an `ls-tree` error or an empty listing.
5. Add gate leg 6 and `GateIO.SubjectTree`. On a SHA mismatch with a recorded digest, compare digests: equal is fresh; different is stale. Equal SHAs make no `SubjectTree` call. A legacy panel keeps today's message byte-for-byte (R3.3).
6. Wire the same `GateIO` into the four consumers: `complete` (`internal/complete/panel_advisory.go`), `panel verify`, `panel tally` (`cmd/mindspec/panel.go`) and `instruct` (`internal/instruct/panelstate.go`) (R3.4).
7. In ADR-0037 §4, add the subject-tree back-pointer. In the Decision Details cap, write: "three on a SHA mismatch". Also state that the `complete` preflight legs (Core 3, the deferral re-check and the `check-ignore` call) are outside the cap. This assigns round-7 note (a).
8. Add `internal/validate/adr_spec130_subject_test.go` (`TestADRAmendments_Spec130_Subject`).

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestSubject_PerGate|TestSubjectDigest_|TestGate_SubjectTreeStaleness|TestGate_LegacyStaleMessageUnchanged|TestGate_EqualSHANoSubjectCall|TestPanelLeafImports_StdlibPlusTermsafeOnly'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelConsumers_SubjectTreeParity|TestSubjectDigest_FailsClosed'` passes
- [ ] `go test ./internal/lint/... -run TestArgvIDOperandGated` passes
- [ ] `go test ./internal/complete/... ./internal/approve/... -run TestEnforcementHasNoGitLeaks` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Subject` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-10 (R3.1, R3.2) and AC-11 (R3.3, R3.4) pass.
- AC-26 (part): R13.1 §4 and the cap text are pinned.

**Depends on**
None.

## Bead 5: Create writes the round

**Steps**
1. On a round bump, carry forward refutations, deferrals and rebinds whose ledger events back them. Drop unbacked entries and name each one (R1.2, R5.3 recovery). On revival of an abandoned panel, append one `abandon_history` entry.
2. Before a bump, archive `BRIEF.md` as `BRIEF-round-<N>.md`. Never overwrite an archive. Tally counts only `.json` slots (R1.4).
3. Render the prior-round region in the new BRIEF: `consolidated-round-<N>.md` with `termsafe.Escape`, or `none recorded` (R1.5).
4. Add `--reviewers N`. Refuse N below 2. Fill N slots from the resolved family list, round-robin (R2.2, R2.3).
5. Write the roster with ids `R1..Rn` from `PanelGateReviewerSlots`. List each slot in the BRIEF as `slot model lens`. A later round carries the roster forward unchanged (R2.4, OQ-2).
6. Append the `round` event in the same operation as the `panel.json` write. Set `telemetry: "required"` on new panels (R11.1).
7. Write no `approve_threshold` (R7.5).
8. Extend `TestPanelCreate_BeadIDDerivedFromTarget` to assert 4 slots.

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestCreate_CarriesRefutationsAcrossRoundBump|TestCreate_RevivalAppendsAbandonHistory|TestCreate_ArchivesBriefOnBump|TestCreate_NeverOverwritesArchive|TestCreate_PriorRoundRegion|TestCreate_RosterCarriedForward|TestCreate_BriefListsRoster|TestCreate_DropsUnbackedEntries'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelCreate_ReviewersFloor|TestPanelCreate_ReviewersN|TestPanelCreate_NoApproveThreshold|TestPanelCreate_BeadIDDerivedFromTarget'` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-2 (R1.2), AC-3 (R1.4, R1.5) pass.
- AC-4 (R2.1 part): 4 slots.
- AC-8 (create part), AC-9 (roster and BRIEF part), AC-14 (`TestCreate_DropsUnbackedEntries`), AC-18 (`TestPanelCreate_NoApproveThreshold`) pass.

**Depends on**
Bead 1, Bead 2, Bead 3, Bead 4.

## Bead 6: Pass rule, roster tally and hand-edit gate

**Steps**
1. Add `internal/panel/passrule.go`. A panel passes only when it is complete, has no veto, is fresh, has every finding discharged, and is intact (R7.1). The APPROVE count is not an input. Other gate legs stay unchanged.
2. A discharge is pinned to its round and its `finding_sha256`. An item-level discharge clears one finding. A slot-level discharge clears the slot until the verdict file changes (R7.2-R7.4). A refuted REJECT, `hard_block` or INVALID still blocks.
3. Treat a recorded `approve_threshold` as legacy. Print the legacy line. It does not change the decision (R7.5).
4. Count roster slots only. An off-roster file counts nothing; an off-roster REJECT or INVALID blocks and names the file. A legacy panel with no roster tallies as today (R2.5).
5. Make a sub-floor `expected_reviewers` a registration error that names `--reviewers 2` and no `/ms-bead-fix` (R2.2).
6. Wire the R5.3 matcher. On a non-legacy panel, a field change with no backing event blocks with `changed outside the panel verbs`. A legacy panel is exempt. A carried round-1 entry does not block (R5.3).
7. Add `internal/panel/ruleline.go`, one render of the rule line with the INVALID count. Use it in `tally`, `verify` and `instruct`. Remove every `threshold is %d` and `threshold %d` format (R8).
8. Update `.mindspec/domains/workflow/interfaces.md` § Panel Artifact Schema with the new fields.
9. Append `## Amendment (Spec 130)` to ADR-0037 (§1 fields, §3 pass rule and the superseded Spec-114 sentences, §8 hand-edit scope). Add the two R13.4 back-pointers to ADR-0044. Add `internal/validate/adr_spec130_passrule_test.go` (`TestADRAmendments_Spec130_PassRule`).

**Verification**
- [ ] `go test ./internal/panel/... -run 'TestPassRule_|TestApproveThreshold_LegacyNotice|TestTally_OffRoster|TestTally_LegacyNoRosterUnchanged|TestGate_SubFloorRegistrationError|TestGate_HandEditDetected|TestGate_HandChangedEntryPayload|TestGate_OneMarkerBlocks|TestGate_LegacyHandEditExempt|TestGate_CarriedBaseline'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestRuleLine_SharedRender|TestRuleLine_INVALIDCount'` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_PassRule` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-17 (R7.1-R7.3), AC-18 (R7.4, R7.5 tally part), AC-19 (R8) pass.
- AC-8 (gate part), AC-9 (tally part), AC-14 (gate tests) pass.
- AC-26 (part): R13.1 §1/§3/§8 and R13.4 are pinned.

**Depends on**
Bead 2, Bead 3, Bead 4.

## Bead 7: Panel rebind

**Steps**
1. Add `cmd/mindspec/panel_rebind.go`: `panel rebind <slug> --reason <text>`. It refuses an empty reason, a panel that does not pass, a reviewed SHA that is not an ancestor of HEAD, and a diff path outside `panel.rebind_allowlist` (default `OWNERSHIP.yaml`) (R4.1-R4.3).
2. Always refuse a diff that contains `.mindspec/config.yaml`, even with a widened allowlist. Refuse an empty allowlist. Add the config key in `internal/config/config.go`.
3. On success, append a `rebinds[]` entry and a `rebind` event in one operation. Set the new reviewed SHA and digest. Do not bump the round (R4.4).
4. `complete` writes `panel_rebound_entries`, through the same stage as `panel_refuted` (R4.5).
5. In `internal/lifecycle/landed.go`, add each R5.3-matched `rebinds[].from_sha` to the `reviewedHeadSHAForBead` set. Use exact equality (R3.5, OQ-5).
6. Add the ADR-0037 §4 rebind back-pointer and the ADR-0041 datum (c) back-pointer. Add `internal/validate/adr_spec130_rebind_test.go` (`TestADRAmendments_Spec130_Rebind`).

**Verification**
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelRebind_OwnershipOnly|TestPanelRebind_Refusals'` passes
- [ ] `go test ./internal/complete/... -run TestComplete_CopiesRebinds` passes
- [ ] `go test ./internal/lifecycle/... -run TestReviewedHeadSHASet_IncludesRebindFrom` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Rebind` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-12 (R4.1-R4.5, R3.5) passes, except `TestToolArtifactPaths_SingleHome` (Bead 9).
- AC-26 (part): R13.1 §4 rebind and R13.3 are pinned.

**Depends on**
Bead 5, Bead 6.

## Bead 8: Refute, defer and abandon verbs

**Steps**
1. Add `cmd/mindspec/panel_refute.go`. It takes `--slot`, `--round`, `--finding`, `--ground`, `--reason`, `--evidence`, `--ac` and `--by`. It writes the refutation entry, the `refutation` event and an `audited-refuted` row (reviewer equals `--slot`) in one lock hold. A rerun adds nothing (R9.1, R9.2).
2. Refute refuses: an APPROVE, REJECT, `hard_block` or INVALID slot; no latest-round verdict file; a stale round; an unknown finding; an empty `--by`; `--evidence` that resolves to nothing; `out_of_acceptance_scope` without a valid `--ac`. It stores redacted text (`<path>`).
3. Add `internal/bead/deferral.go`. It reads a bead's status, parent, `discovered-from` edge and description through bd.
4. Add `cmd/mindspec/panel_defer.go`. Defer refuses a closed bead, a child of the epic, a bead with no `discovered-from` edge, and a description without the finding text. It shares the refute slot and round refusals. It refuses on `spec_approve` and adhoc panels, with a recovery that names `refute`. It writes the deferral, the `deferral` event and a `confirmed-deferred` row (R9.3).
5. Add `cmd/mindspec/panel_abandon.go`. It requires `--reason` and `--by`. It sets `abandoned` and `abandon_reason` and appends the `abandon` event. It writes no manifest (R9.4).
6. Update both copies of `ms-panel-tally` and `ms-panel-run`. Remove every hand-edit instruction. Name the three verbs. Name the roster ids as the verdict prefix in `ms-panel-run` (R5.4, OQ-2). Add `internal/setup/panel_skills_test.go` (`TestPanelSkills_NoHandEditInstructions`).
7. Add the ADR-0045 status note. Update the `ms-bead-impl` marker text "the spec-130 rule table" in both copies and its pin `TestReviewDiscipline_AC10StagedRuleMarkerOnly`. Add the spec 129 R7.2 status line (R13.5).
8. Add the ADR-0037 §1 "verbs are the only writers" back-pointer and the ADR-0043 back-pointer at "ONE canonical Go verb". Add `internal/validate/adr_spec130_verbs_test.go` (`TestADRAmendments_Spec130_Verbs`).

**Verification**
- [ ] `go test ./cmd/mindspec/... -run 'TestPanelRefute_|TestPanelDefer_|TestPanelAbandon_Verb'` passes
- [ ] `go test ./internal/bead/... -run TestDeferral_` passes
- [ ] `go test ./internal/setup/... -run 'TestPanelSkills_NoHandEditInstructions|TestReviewDiscipline_AC10StagedRuleMarkerOnly'` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Verbs` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-15 (R5.4), AC-20 (R9.1, R9.2), AC-21 (R9.1, R9.3) pass.
- AC-22 (R9.4 part): `TestPanelAbandon_Verb` passes.
- AC-26 (part): R13.5, the spec 129 R7.2 line and the R13.1/R13.2 verb back-pointers are pinned.

**Depends on**
Bead 5, Bead 6.

## Bead 9: Tracked ledger and the tool-artifact home

**Steps**
1. Add `internal/panel/artifacts.go` with `ToolArtifactPaths` (`.beads/issues.jsonl` and the panel ledger pattern). Make `internal/panel/gate.go` and `internal/next/guard.go` use it. Remove the local copies (R4.6). Add `TestToolArtifactPaths_SingleHome`.
2. Add the two negation lines to `gitutil.ReviewIgnoreEntries` after the spec-129 entries (R6.1).
3. In `internal/bootstrap/bootstrap.go`, write the negation lines by exact-line presence. Do not use the check-ignore re-append. A second run leaves `.gitignore` byte-identical (R6.2).
4. Add the two lines to this repo's `.gitignore`. Change its R8.4 comment to name option (B) (R6.3). Add the spec 129 R8.4 status line (R6.4).
5. Add `CheckIgnore` to the Executor interface, `MindspecExecutor` and `mock.go`.
6. Add `internal/complete/panel_ledger.go`. Before the bead merge, check each ledger path with `check-ignore` in the spec worktree. Refuse before any side effect, with the R6.6 recovery. After the merge, commit each matched ledger in one `chore(panel): ledger` commit with `exec.CommitPaths`. A legacy panel with no ledger makes no commit and prints no line.
7. In `internal/approve/impl.go`, add a preflight leg. It runs the same `check-ignore` check. Then it refuses an uncommitted or untracked spec ledger, with the `git -C <spec-worktree>` recovery.
8. Hatches: the two hatches do not skip the R6.6 refusal at `impl approve` (spec text). At `complete`, the hatches also do not skip the `check-ignore` refusal or the ledger commit. Reason: the ledger is a tool artifact, not a gate input, and an ignored ledger breaks the later `impl approve` leg. This assigns round-7 note (b).
9. Update `.mindspec/domains/execution/interfaces.md` § Gitignore Ensure. Add the ADR-0043 "tracked again" note and the ADR-0037 Spec-116 `artifactPaths` back-pointer. Add `internal/validate/adr_spec130_ledger_test.go` (`TestADRAmendments_Spec130_Ledger`). It also pins the spec 129 R8.4 line.

**Verification**
- [ ] `go test ./internal/panel/... -run TestToolArtifactPaths_SingleHome` passes
- [ ] `go test ./internal/bootstrap/... -run 'TestBootstrap_ReviewsLedgerTracked|TestBootstrap_ReviewsIgnored'` passes
- [ ] `go test ./internal/next/... -run TestNextGuard_LedgerIsArtifact` passes
- [ ] `go test ./internal/complete/... -run 'TestComplete_CommitsLedger|TestComplete_NoLedgerNoCommit|TestLedger_IgnoredPathRefuses|TestEnforcementHasNoGitLeaks'` passes
- [ ] `go test ./internal/approve/... -run 'TestImplApprove_UncommittedLedgerRefuses|TestEnforcementHasNoGitLeaks'` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Ledger` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-16 (R6) passes.
- AC-12 (R4.6 part): `TestToolArtifactPaths_SingleHome` passes.
- AC-26 (part): R13.1 Spec-116, R13.2 "tracked again" and the spec 129 R8.4 line are pinned.

**Depends on**
Bead 2, Bead 4.

## Bead 10: Core 3 coverage enforcement

**Steps**
1. Make `tally` write the coverage manifest. It lists roster slots only. It appends a new manifest when the verdict set changed since the last one. It writes one on an Allow, and on a stale non-bead registration with covering rows (R11.2). Remove the manifest step from both `ms-panel-tally` copies.
2. Make `ComputeQ4` and every manifest reader use only the last manifest of each panel (R11.3).
3. Add `internal/complete/panel_core3.go`. After the gate Allow and before materialization, check each required registration that `panel.ForBead` matches. Block on a REQUEST_CHANGES slot with no covering row, a missing store, no manifest, a manifest for another round, a manifest that differs from the verdict files, or a malformed record (R11.4). A legacy registration passes with an advisory. A Warn registration skips the check with one advisory line (R11.6).
4. In `internal/approve/impl.go`, add a preflight leg on every required registration of the spec, final review included. It runs no gate decision. Before it skips an abandoned or legacy registration, it runs the legacy-marker check and the R5.3 check. A hand-set `abandoned: true` with no `abandon` event blocks. It names the panel and slot (R11.5).
5. Both legs honour `MINDSPEC_SKIP_PANEL` and `enforcement.panel_gate: false` (OQ-3).
6. Append `## Amendment (Spec 130)` to ADR-0043 with the R13.2 items. Add `internal/validate/adr_spec130_core3_test.go` (`TestADRAmendments_Spec130_Core3`).

**Verification**
- [ ] `go test ./cmd/mindspec/... -run 'TestTally_AppendsManifestOnChange|TestTally_WritesManifestOnAllow|TestTally_ManifestWhenOnlyStale|TestPanelAbandon_KeepsManifest'` passes
- [ ] `go test ./internal/panel/... -run TestComputeQ4_LastManifestOnly` passes
- [ ] `go test ./internal/complete/... -run 'TestComplete_Core3'` passes
- [ ] `go test ./internal/approve/... -run 'TestImplApprove_Core3Leg|TestImplApprove_AbandonedSkipIntegrity'` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130_Core3` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-24 (R11) passes.
- AC-22 (part): `TestPanelAbandon_KeepsManifest` passes.
- AC-26 (part): R13.2 is pinned.

**Depends on**
Bead 5, Bead 6, Bead 9.

## Bead 11: Deferral audit and gate-time re-check

**Steps**
1. Add `internal/complete/panel_deferred.go`. `complete` writes `panel_deferred` with one entry per deferral, through the pending-marker protocol. Each entry holds `finding_text` and `finding_sha256` (R10.1).
2. At `complete` and in the `impl approve` preflight, re-check each deferral bead through `internal/bead/deferral.go`. Block on re-parent under the epic, deletion, a removed `discovered-from` edge, removed finding text, or a bd error. Name the slot, the finding and the bead (R10.2).
3. A closed bead blocks. At `complete` and for a `final_review` panel at `impl approve`, the recovery names `bd reopen` and `panel refute`. For a bead panel at `impl approve`, it names `bd reopen` only. A slot-level deferral's refute line has no `--finding`.
4. A refutation of the same finding and text supersedes the deferral. Skip deferrals on abandoned panels. Compare redacted text, so a raw `/tmp/x` in the bead description passes.
5. Both legs honour the two hatches (OQ-3).
6. Add `TestLedger_RecordKinds`: each verb appends its R5.1 record with `spec` and `panel`.
7. In ADR-0037, add the §7 back-pointer (abandon verb, Core 3 skip rules), the Spec-115 reach note and the §8 carve-out extension (`panel_deferred`, ledger, R12 keys). Add `internal/validate/adr_spec130_deferral_test.go` (`TestADRAmendments_Spec130_Deferral`).

**Verification**
- [ ] `go test ./internal/complete/... -run 'TestComplete_PanelDeferredAudit|TestGateTime_'` passes
- [ ] `go test ./internal/approve/... -run TestGateTime_` passes
- [ ] `go test ./cmd/mindspec/... -run TestLedger_RecordKinds` passes
- [ ] `go test ./internal/validate/... -run TestADRAmendments_Spec130` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-23 (R10) passes.
- AC-13 (verb part): `TestLedger_RecordKinds` passes.
- AC-26: all `TestADRAmendments_Spec130` pins pass.

**Depends on**
Bead 7, Bead 8, Bead 10.

## Bead 12: Landed reviewed-SHA keys

**Steps**
1. Do not change the `CompleteBead` signature. Add an optional interface `LandedReviewSetter` with `SetLandedReview(sha, panel string)` on `MindspecExecutor`.
2. When `complete`'s gate decision is Allow, `complete` calls it before `exec.CompleteBead`. The value of `panel` is `<slug>@r<N>`.
3. `ensureLandedBinding` writes `mindspec_landed_reviewed_sha` and `mindspec_landed_review_panel` in the same `mergeBindingFn` call as the binding, on the `CompleteBead` path only. A Warn writes neither key. `FinalizeEpic` keeps the keys unchanged (R12).

**Verification**
- [ ] `go test ./internal/executor/... -run 'TestEnsureLandedBinding_'` passes
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-25 (R12) passes.

**Depends on**
None.

## Carrier-brief resolutions

- **Ordering**: R6 (Bead 9) depends on R3 (Bead 4). R1.1 (Bead 3) lands before R4 (Bead 7, through Bead 5).
- **ADR landing**: each amendment lands with the bead that ships its code and is pinned by its own `TestADRAmendments_Spec130_*` file. One bead per ADR appends the EOF section (ADR-0037: Bead 6; ADR-0043: Bead 10). Other beads add inline back-pointers in other paragraphs. The ADR-0043 `## Amendment (Spec 126)` heading stays. Note: no test greps the text "6 / `n-1`, unconditionally"; only the heading is pinned. Bead 1 keeps the bullet and adds a back-pointer. Bead 4 amends the two-subprocess cap.
- **Bead count and chain**: 12 beads, split by mechanism. Longest chain: 4 beads (for example Bead 4 → Bead 5 → Bead 10 → Bead 11). Beads 1, 2, 3, 4 and 12 start in parallel.
- **Dogfood**: bead panels run on the installed v0.16.0 binary. No branch build is installed during the run.
- **OQ-1**: Bead 1. **OQ-2**: Bead 5 (roster ids) and Bead 8 (skill text). **OQ-3**: Beads 10 and 11. **OQ-4**: Bead 2. **OQ-5**: Bead 7.
- **Round-7 notes**: (a) Bead 4 names the `check-ignore` call in the cap text. (b) Bead 9: the hatches do not skip the `check-ignore` refusal at `complete`.

## Provenance

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
| AC-9 | 5, 6 | `go test ./internal/panel/... -run 'TestCreate_RosterCarriedForward\|TestCreate_BriefListsRoster\|TestTally_OffRoster\|TestTally_LegacyNoRosterUnchanged'` |
| AC-10 | 4 | `go test ./internal/panel/... ./cmd/mindspec/... -run 'TestSubject_PerGate\|TestSubjectDigest_'` |
| AC-11 | 4 | `go test ./internal/panel/... -run 'TestGate_SubjectTreeStaleness\|TestGate_LegacyStaleMessageUnchanged\|TestGate_EqualSHANoSubjectCall'`; `go test ./cmd/mindspec/... -run TestPanelConsumers_SubjectTreeParity` |
| AC-12 | 7, 9 | `go test ./cmd/mindspec/... -run TestPanelRebind_`; `go test ./internal/complete/... -run TestComplete_CopiesRebinds`; `go test ./internal/lifecycle/... -run TestReviewedHeadSHASet_IncludesRebindFrom`; `go test ./internal/panel/... -run TestToolArtifactPaths_SingleHome` |
| AC-13 | 2, 11 | `go test ./internal/panel/... -run TestLedger_RecordKeys`; `go test ./cmd/mindspec/... -run 'TestLedger_RecordKinds\|TestDispositionValidate_LedgerKinds\|TestDispositionAppend_RefusesVerbKinds'` |
| AC-14 | 5, 6 | `go test ./internal/panel/... -run 'TestGate_HandEditDetected\|TestGate_HandChangedEntryPayload\|TestGate_OneMarkerBlocks\|TestGate_LegacyHandEditExempt\|TestGate_CarriedBaseline\|TestCreate_DropsUnbackedEntries'` |
| AC-15 | 8 | `go test ./internal/setup/... -run TestPanelSkills_NoHandEditInstructions` |
| AC-16 | 9 | `go test ./internal/bootstrap/... -run 'TestBootstrap_ReviewsLedgerTracked\|TestBootstrap_ReviewsIgnored'`; `go test ./internal/next/... -run TestNextGuard_LedgerIsArtifact`; `go test ./internal/complete/... -run 'TestComplete_CommitsLedger\|TestComplete_NoLedgerNoCommit\|TestLedger_IgnoredPathRefuses'`; `go test ./internal/approve/... -run TestImplApprove_UncommittedLedgerRefuses` |
| AC-17 | 6 | `go test ./internal/panel/... -run 'TestPassRule_EachTermBlocks\|TestPassRule_ZeroApprovePasses\|TestPassRule_NeverDischarged\|TestPassRule_EmptyRCInvalid\|TestPassRule_RoundPinned\|TestPassRule_FindingTextPinned'` |
| AC-18 | 5, 6 | `go test ./internal/panel/... -run 'TestPassRule_ItemLevelDischarge\|TestPassRule_SlotLevelVoidedByChange\|TestApproveThreshold_LegacyNotice'`; `go test ./cmd/mindspec/... -run TestPanelCreate_NoApproveThreshold` |
| AC-19 | 6 | `go test ./cmd/mindspec/... -run 'TestRuleLine_SharedRender\|TestRuleLine_INVALIDCount'` |
| AC-20 | 8 | `go test ./cmd/mindspec/... -run TestPanelRefute_` |
| AC-21 | 8 | `go test ./cmd/mindspec/... -run TestPanelDefer_` |
| AC-22 | 8, 10 | `go test ./cmd/mindspec/... -run 'TestPanelAbandon_Verb\|TestPanelAbandon_KeepsManifest'` |
| AC-23 | 11 | `go test ./internal/complete/... ./internal/approve/... -run 'TestComplete_PanelDeferredAudit\|TestGateTime_'` |
| AC-24 | 10 | `go test ./cmd/mindspec/... -run 'TestTally_AppendsManifestOnChange\|TestTally_WritesManifestOnAllow\|TestTally_ManifestWhenOnlyStale'`; `go test ./internal/panel/... -run TestComputeQ4_LastManifestOnly`; `go test ./internal/complete/... -run TestComplete_Core3`; `go test ./internal/approve/... -run 'TestImplApprove_Core3Leg\|TestImplApprove_AbandonedSkipIntegrity'` |
| AC-25 | 12 | `go test ./internal/executor/... -run TestEnsureLandedBinding_` |
| AC-26 | 1, 4, 6, 7, 8, 9, 10, 11 | `go test ./internal/validate/... -run TestADRAmendments_Spec130` |
| AC-27 | all | `gofmt -l .`; `go build ./... && go test ./... -short` (includes `TestPanelLeafImports_StdlibPlusTermsafeOnly` and `TestEnforcementHasNoGitLeaks`) |
