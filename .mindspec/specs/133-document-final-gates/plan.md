---
status: Draft
spec_id: 133-document-final-gates
version: "1"
adr_citations:
  - ADR-0037
  - ADR-0041
  - ADR-0043
  - ADR-0032
  - ADR-0040
  - ADR-0035
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths: [internal/config/config.go, .mindspec/config.yaml, internal/panel/subject.go, internal/panel/gate.go,
        internal/panel/create.go, cmd/mindspec/panel.go, internal/approve/lifecycle_gate.go,
        .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md]
  - id: 2
    depends_on: [1, 5, 6]
    key_file_paths: [internal/approve/spec.go, internal/approve/plan.go, .mindspec/adr/ADR-0041-gate-before-mutate.md]
  - id: 3
    depends_on: [1]
    key_file_paths: [internal/approve/impl.go, internal/executor/mindspec_executor.go]
  - id: 4
    depends_on: [1]
    key_file_paths: [internal/complete/panel_advisory.go, internal/complete/complete.go]
  - id: 5
    depends_on: []
    key_file_paths: [cmd/mindspec/panel_defer.go, internal/panel/ledger.go, internal/panel/gate.go, internal/panel/create.go,
        .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md]
  - id: 6
    depends_on: []
    key_file_paths: [internal/panel/verdict_v2.go, internal/panel/gate.go, cmd/mindspec/panel_schema.go]
  - id: 7
    depends_on: []
    key_file_paths: [cmd/mindspec/complete.go, internal/complete/complete.go, internal/validate/divergence.go,
        internal/validate/ownership_resolve.go, internal/validate/adr0032_amendment_test.go,
        .mindspec/adr/ADR-0032-adr-semantic-gates.md]
  - id: 8
    depends_on: [7]
    key_file_paths: [cmd/mindspec/complete.go, internal/complete/complete.go, internal/validate/docsync.go,
        internal/approve/impl.go, .mindspec/adr/ADR-0031-doc-sync-gate.md]
  - id: 9
    depends_on: [1]
    key_file_paths: [internal/setup/claude.go, internal/setup/historical_skills, internal/setup/lifecycle_gate_step_test.go,
        .claude/skills/ms-spec-approve/SKILL.md, plugins/mindspec/skills/ms-bead-cycle/SKILL.md,
        internal/instruct/instruct.go, internal/doctor/config.go, project-docs/user/guides/review-panels.md,
        CHANGELOG.md]
---
# Plan: 133-document-final-gates

This plan builds spec 133 R1–R12 in 9 beads. The longest dependency chain is 2 beads. Beads 1, 5, 6 and 7 start at once. Beads 2, 3, 4, 8 and 9 start when their dependencies merge.

## Resolutions

The spec has no to-plan deferrals. This section names the plan-time decisions.

- **Operator decisions.** OQ1–OQ8 are fixed. No bead reopens them. The post-131 defect fixes on the merged tree (deferral source binding, the ledger-only completeness check, subject-head resolution, the `impl approve` diff base) are current behavior. No bead plans them again.
- **Dogfood boundary.** See `## Testing Strategy`. Owner: every bead and every panel of this lane.
- **In-flight specs (R4.4).** Bead 2 owns the rule. A spec in Plan or Implement mode keeps its approvals. No verb runs a document gate for an approval that exists. Tests: `TestDocumentGate_InFlightSpecKeepsApproval` (AC16, first sentence) and `TestReApprove_EditedPlanRunsGateBeforeSupersede` (AC16, second and third sentences). Bead 9 adds the `doctor` Warn line for an in-flight spec whose epic has `panel_absent` (`TestPanelPresence_DoctorInFlightWarn`).
- **Bead count (9).** Each bead is one mechanism: gate core, document gate, final-review gate, bead presence, plan deferral, artifact path, content-gate report, doc declarations, guidance. Merging two of them puts two unrelated mechanisms in one diff and one panel. Splitting further adds edges. The chain stays at 2.
- **Shared helpers with one owner.** Bead 1 owns these. Beads 2, 3 and 9 call them and do not write a second copy.
  - `panel.DocumentSubjectDigest`: the one subject-digest function of R1.6 for `spec_approve`, `plan_approve` and `final_review` (own write sets out, frontmatter compared as parsed keys and values). `panel create`, `panel rebind`, `panel verify` and the gates call it.
  - `config.Enforcement.PanelPresence` (`warn` or `enforce`).
  - `internal/approve/lifecycle_gate.go`: `runLifecycleGate(root, specID, gate string, worktree string, out io.Writer)`. It returns an outcome (Allow, Warn with an audit record, or a refusal) and writes nothing. `recordLifecycleGateAudit(epicID, outcome)` writes `panel_gate_skipped`, `panel_abandoned` or `panel_absent` on the epic.
- **Shared files.** A shared file alone is not a dependency. Beads that run at once edit a shared file only at these seams. A bead that merges later rebases on the earlier merge and keeps its edit inside its seam.
  - `internal/panel/gate.go`: Bead 1 changes the message text for non-bead gates (R1.9). Bead 5 adds the `plan_deferrals` integrity check and discharge. Bead 6 adds the artifact path to the unresolved-finding message. `TestRawMergeFence_CallSiteShapeSentinel` stays green or is updated in the bead that changes a fence call site.
  - `internal/panel/create.go`: Bead 1 adds the `spec/<id>` target refusal. Bead 5 carries `plan_deferrals` into a new round.
  - `internal/approve/impl.go`: Bead 3 adds the final-review gate and the bead-worktree check. Bead 8 changes the branch-wide doc-sync gate and adds the doc-deferral check.
  - `internal/complete/complete.go`: Bead 4 changes the closed-bead panel paths. Beads 7 and 8 change the content-gate path; Bead 8 runs after Bead 7.
  - `.mindspec/domains/workflow/interfaces.md`: each bead adds or edits only the subsection of its own mechanism.
- **File lists.** `key_file_paths` and each per-bead file list are a starting map. They are not complete and they are not a gate. An implementer adds the files that its acceptance criteria need.
- **No internal references on the product surface (R11.8).** Every bead writes CLI messages, recovery lines, skill text, `instruct` text or user docs. Each bead's acceptance criteria state the rule for its own strings.
- **ADR amendments land with their citing beads (R12).** ADR-0037 in Bead 1. ADR-0041 in Bead 2 (it states the gate-before-first-write rule and the own write sets of all three approve verbs; Bead 3 cites it and does not edit it). ADR-0043 in Bead 5. ADR-0032 in Bead 7. ADR-0031 in Bead 8. Each amendment is one `## Amendment (Spec 133)` section. Each bead adds `internal/validate/adr_spec133_<adr>_test.go` with `TestADRAmendments_Spec133ADR<NNNN>`.
- **ADR pin constraint.** The spec-130 pins split each ADR on `## Amendment (Spec 130)` and need each bullet heading once. So a Spec 133 section must not contain `## Amendment (Spec 130)`, `## Amendment (Spec 131)` or `PRE-DRAFT`, and must not start a `- **` bullet with a heading that an earlier amendment uses. `TestADR0032Amendment_ThirdAmendmentAnchorsPresent` reads the last `## Amendment` section as the spec-122 one. Bead 7 changes it to select the section whose header cites spec 122, and keeps every anchor.

## ADR Fitness

- **ADR-0037 (panel gate contract).** Sound. The decision legs, hatches and trust boundary stay. The presence policy replaces §6 fail-open, and the gate covers documents and the final review. Amended (Bead 1).
- **ADR-0041 (gate before mutate).** Sound. The approve verbs gain a gate before their first write. The own write set keeps re-runs convergent. Amended (Bead 2).
- **ADR-0043 (disposition store).** Sound. A new record kind `deferred_to_plan` fits the append-only ledger. Amended (Bead 5).
- **ADR-0031 (doc-sync gate).** Sound. A no-doc-impact declaration and an owned doc deferral are narrower than the reason-only skew. Amended (Bead 8). It is not in `adr_citations`: its declared domains do not intersect the spec's impacted domains, and `validate plan` refuses that citation.
- **ADR-0032 (ADR semantic gates).** Sound. An unowned file becomes a Warn. Undeclared-domain, attribute and uncovered failures stay Errors. Amended (Bead 7).
- **ADR-0040 (layering ratchet).** Applied. The document-panel precondition moves from skill text to the binary (Beads 2 and 9).
- **ADR-0035 (agent error contract).** Applied. Each new refusal ends with one `recovery:` line. `go test ./internal/guard/... -run Recovery` covers the convention.

No ADR needs a superseding ADR.

## Testing Strategy

- **Dogfood boundary.** The lane that builds this spec runs every lifecycle and panel verb on the installed v0.18.0 binary (`~/.local/bin/mindspec`). It never runs a spec-branch build for a lifecycle or panel verb. Its panels never use `artifact_path`, because the released binary refuses the field. The released binary does not run the new gates. The `enforcement.panel_presence` key that Bead 1 writes is ignored by the released config loader (by inspection: `config.Load` uses a non-strict `yaml.Unmarshal`); Bead 1 checks this with `~/.local/bin/mindspec config show`.
- **Doc-sync on the released binary.** The released `complete` refuses a change to `internal/**` with no domain-doc change. Each bead updates `.mindspec/domains/workflow/interfaces.md` (Bead 1 also `.mindspec/domains/core/interfaces.md`) for its mechanism. Bead 9 edits `CHANGELOG.md`, which no manifest claims; the released binary refuses it with `adr-divergence-unowned`, so Bead 9 completes with `--override-adr` and a reason.
- **No real model runs.** Tests use temp git repos, scratch bead stores (the existing `bd` test seams such as `SetSpecRunBDForTest`) and local fake runners. No test starts a real codex, claude or other model CLI.
- **Shared fixtures.** Bead 1 adds a test fixture builder for a spec with a registration at `spec/<id>` for a named gate (pass, each failing leg, abandoned, SHA target). Beads 2, 3 and 9 use it. Bead 6 uses the existing `internal/panel/paneltest` builders.
- **Unit tests** cover the digest, config validation, verdict schema, ledger kinds and message text. **Integration tests** drive `ApproveSpec`, `ApprovePlan`, `ApproveImpl` and `complete.Run` in temp repos, with fault injection through the existing fault seams.
- **Gate:** each bead runs its named tests and the tests of the files it touches. The final review runs `make test` (AC34).

## Bead 1: Lifecycle gate core

R1.2, R1.3 (target refusal), R1.5 (presence decision), R1.6 (digest), R1.8, R1.9, R4.1, R4.6, R12 for ADR-0037.

**Steps**
1. Add `enforcement.panel_presence` to `config.Enforcement`, default `warn`. `config.Load` refuses another value and names the key. Set `enforcement.panel_presence: enforce` in `.mindspec/config.yaml`.
2. Add `panel.DocumentSubjectDigest`. It leaves out the own write sets of R1.6 and R2.5 and compares frontmatter as parsed keys and values. Use it in `panel create`, `panel rebind`, `panel verify` and the gate for `spec_approve`, `plan_approve` and `final_review`.
3. Make `panel create` refuse a target other than `spec/<id>` for those three gates.
4. Add `runLifecycleGate` and `recordLifecycleGateAudit` in `internal/approve/lifecycle_gate.go`. The gate resolves the head from the recorded target, refuses another target and names it, runs every decision leg and the coverage check on each live registration, runs the dirt check in the spec worktree outside the own write set, applies the hatches and the presence policy, and returns the outcome.
5. Remove the bead merge fence and the bead fix-skill text from non-bead gate messages. Each refusal names the gate, panel and leg and ends with one `recovery:` line.
6. Add the `## Amendment (Spec 133)` section to ADR-0037 and its pin test. Update the domain docs.

**Verification**
- [ ] `go test ./internal/config/... -run PanelPresence` passes.
- [ ] `go test ./internal/panel/... -run 'DocumentSubject|OwnWriteSet'` passes.
- [ ] `go test ./cmd/mindspec/... -run 'PanelCreateTarget|PanelVerify|PanelRebind'` passes.
- [ ] `go test ./internal/approve/... -run LifecycleGate` passes.
- [ ] `go test ./internal/validate/... -run 'ADRAmendments|ADR0037'` passes.
- [ ] `go test ./internal/panel/... -run RawMergeFence` passes.
- [ ] `~/.local/bin/mindspec config show` exits 0 in the bead worktree.

**Acceptance Criteria**
- [ ] `panel_presence: maybe` makes `config.Load` fail with a message that names `enforcement.panel_presence`. A loader that falls back to `warn` fails the test (AC15).
- [ ] A test loads `.mindspec/config.yaml` with the config loader and reads `enforce` (AC18). A test that greps the file instead fails the review.
- [ ] `panel create --target <sha>` exits non-zero for each of the three gates. A refusal for only one gate fails (AC3).
- [ ] `DocumentSubjectDigest` is equal for a spec before and after a frontmatter rewrite with a changed key order, for a draft and an approved `## Approval` section, and for an absent section. It differs after one extra line under `## Approval` or a body edit. A digest over raw blob ids fails these tests (AC3, AC5).
- [ ] `runLifecycleGate` returns a refusal for each failing leg under `warn` and under `enforce`. A policy that turns a failing leg into a Warn fails the test (R1.2).
- [ ] With no registration, the refusal ends with `recovery: mindspec panel create <slug> --spec <id> --gate <gate> --target spec/<id>`. No non-bead gate message contains the `RawMergeFence` text (R1.8, R1.9).
- [ ] No string this bead adds to a CLI message or recovery line cites an ADR number, spec number, requirement id, bead id or issue number. A test asserts this on each new message.
- [ ] `TestADRAmendments_Spec133ADR0037` finds the presence policy. The spec-130 and spec-131 ADR-0037 pins stay green: `TestADRAmendments_Spec130_Core3`, `_Keys`, `_Deferral`, `_PassRule`, `_Ledger`, `_Subject`, `_Rebind`, `_Verbs` and `TestADRAmendments_Spec131ADR0037`.

**Depends on**
None

## Bead 2: Document gate in spec and plan approve

R1.1, R1.4, R1.7, R4.2–R4.4 for documents, R6.4, R12 for ADR-0041.

**Steps**
1. Call `runLifecycleGate` at the start of `ApproveSpec` (`spec_approve`) and `ApprovePlan` (`plan_approve`), before every write the spec lists in R1.1, on each run.
2. After the epic is found or created, write the audit records, `panel_absent` and the deferral-id list with `recordLifecycleGateAudit`. When `spec approve` cannot create the epic and a record is due, print the record in a warning, exit non-zero after the commits, and print the re-run recovery line.
3. In `ApproveSpec`, record the id of each `deferred_to_plan` record of the spec on the epic.
4. In `ApprovePlan`, before the first write, read the epic list and every `deferred_to_plan` record in the tracked ledger of every `spec_approve` registration. Refuse unless `## Resolutions` names each id. List each missing id.
5. Keep R4.4: no verb runs a document gate for an approval that exists. `plan approve` on an edited plan runs its gate before the supersede close.
6. Add the `## Amendment (Spec 133)` section to ADR-0041 and its pin test. Update the domain docs.

**Verification**
- [ ] `go test ./internal/approve/... -run 'DocumentGate|ReApprove|Resolutions'` passes.
- [ ] `go test ./internal/approve/... -run 'Fault|SpecApprove|PlanApprove'` passes.
- [ ] `go test ./internal/validate/... -run 'ADR0041|Spec132Amendments|Spec130_Rebind|ADRAmendments_Spec133'` passes.
- [ ] `go test ./cmd/mindspec/... -run 'ADR0041|Reattest|Section4Finalize'` passes.
- [ ] `go test ./internal/gitutil/... -run ADR0041` passes.

**Acceptance Criteria**
- [ ] Under `enforce` with no panel, `spec approve` exits non-zero, names `spec_approve` and ends with the filled recovery line. HEAD, `spec.md`, the absent `plan.md` and the absent epic are unchanged. A gate that runs after the frontmatter write fails this test (AC1).
- [ ] Each AC2 leg case refuses under `warn` and under `enforce`, names the leg, and has no merge fence. A passing panel gives exit 0 (AC2).
- [ ] The AC3 commit cases, the SHA-target case and the hand-edited target case give the AC3 results through `spec approve` (AC3).
- [ ] An uncommitted body edit of `spec.md` refuses, names `spec.md`, and makes no commit (AC4).
- [ ] Every AC5 fault and re-run case gives the AC5 result, including the epic-creation failure with each due record. Hand-written approval fields with no passing panel refuse (AC5).
- [ ] Every AC6 case for `plan approve` gives the AC6 result. A refusal creates no bead (AC6).
- [ ] The AC7 hatch cases give the AC7 results. No message contains the skip variable name (AC7).
- [ ] Under `warn`, the AC1 fixture exits 0, prints one `warning:` line that names the gate and records `panel_absent`. With no `panel_presence` key, the same holds (AC15).
- [ ] `TestDocumentGate_InFlightSpecKeepsApproval` and `TestReApprove_EditedPlanRunsGateBeforeSupersede` give the AC16 results. An edited-plan refusal closes no bead and creates no bead (AC16).
- [ ] Every AC22 case gives the AC22 result. An id that occurs only outside `## Resolutions` is refused (AC22).
- [ ] The end-to-end `spec approve` cases of AC20 (artifact path named, then `--to-plan` passes) and AC21 (`spec approve` passes after `--to-plan`; a hand-added `plan_deferrals` entry refuses on the integrity leg) pass.
- [ ] No string this bead adds to a CLI message or recovery line cites an ADR number, spec number, requirement id, bead id or issue number.
- [ ] `TestADRAmendments_Spec133ADR0041` finds the own write set. These pins stay green: `TestADR0041_Spec129Amendment`, `TestDocs_Spec132Amendments_*`, `TestADRAmendments_Spec130_Rebind`, `TestADR0041FourthVerbClause_*`, `TestADR0041OQ1a_*`, `TestADR0041ReattestAmendment_Anchored`, `TestADR0041_Section4FinalizeCarrierAnchors`, `TestADR0041_Section2ConvergenceCompletenessAnchors`.

**Depends on**
Bead 1 (gate core), Bead 5 (`deferred_to_plan` records), Bead 6 (artifact block, for the AC20 case)

## Bead 3: Final-review gate in impl approve

R2.1, R2.4–R2.8, R4.2 and R4.3 for `final_review`.

**Steps**
1. Call `runLifecycleGate` for `final_review` in `ApproveImpl` before the placeholder ADR, the phase-reconcile write, the first epic mutation and each `FinalizeEpic` commit or merge. Write the audit records with `recordLifecycleGateAudit`.
2. Make the dirt check skip excluded paths. Keep `implLedgerPreflight` unchanged.
3. Treat the `--supersede-adr` placeholder, byte-equal to its text, as the own write set.
4. Before the first write, read each plan bead worktree that the `FinalizeEpic` worktree list names. Refuse on any change that `git status` reports. Name the bead and each path. Print the recovery lines of R2.6 by path class.
5. Keep the gate a pre-close gate on re-run. With the spec branch gone, let the "already landed" checks decide. With the spec worktree gone, the epic closed and the phase done, compare the subject at the spec tip with the subject at `mindspec_finalize_tip`.
6. Update the domain docs.

**Verification**
- [ ] `go test ./internal/approve/... -run FinalReviewGate` passes.
- [ ] `go test ./internal/approve/... -run 'ImplFault|ImplPreflight|Orphan|WorktreeEnumeration'` passes.
- [ ] `go test ./internal/executor/... -run 'Finalize|MergedFinalize'` passes.
- [ ] `go test ./internal/lint/... -run ProductDiagnosticScan` passes.

**Acceptance Criteria**
- [ ] Every AC8 case gives the AC8 result. No placeholder ADR exists after a refused run with `--supersede-adr` (AC8).
- [ ] With 1 plan bead, `warn` gives exit 0, one `warning:` line and `panel_absent` on the epic; `enforce` refuses before any write. A gate that skips one-bead specs fails (AC9, AC15).
- [ ] Every AC10 case gives the AC10 result. A dirt check that names the ledger or `.beads/issues.jsonl` fails (AC10).
- [ ] Every AC11 case (a)–(d) gives the AC11 result. A check that runs inside `FinalizeEpic` after the spec-artifact commit fails case (b). A recovery that prints `panel create` for an export-only path fails case (c) (AC11).
- [ ] Every AC12 fault, re-run and no-worktree case gives the AC12 result (AC12).
- [ ] No string this bead adds to a CLI message or recovery line cites an ADR number, spec number, requirement id, bead id or issue number.
- [ ] The ADR-0041 pins listed in Bead 2 stay green after this bead (it does not edit the ADR).

**Depends on**
Bead 1 (gate core)

## Bead 4: Bead panel presence in complete

R3.1, R3.3, R4.2 and R4.3 for the bead gate.

**Steps**
1. Replace the "no registration → Allow" result of `panelGate` with the presence policy.
2. Under `warn`, print one `warning:` line with the gate and recovery command, and record `panel_absent` on the bead.
3. Under `enforce`, refuse on each `complete` path that runs the panel gate today, with the bead state that R3.1 gives for each path. Print the two closed-bead recovery lines.
4. Keep the path-4 cleanup guard a refusal under both policies. Keep the branch-less reconcile path, the busy-worktree refusal and the refutation-obligation branch unchanged.
5. Update the domain docs.

**Verification**
- [ ] `go test ./internal/complete/... -run Presence` passes.
- [ ] `go test ./internal/complete/... -run 'PanelGate|ClosedUnmerged|MergeOnly|Cleanup'` passes.

**Acceptance Criteria**
- [ ] Every AC13 case gives the AC13 result. A refusal that closes the open bead, or one that moves the spec tip on path 4, fails (AC13).
- [ ] Under `warn`, the AC13 missing-registration cases exit 0 with one `warning:` line and `panel_absent` on the bead. The path-4 cleanup guard fixture exits non-zero, prints no such warning and records no `panel_absent` (AC15).
- [ ] No string this bead adds to a CLI message or recovery line cites an ADR number, spec number, requirement id, bead id or issue number.

**Depends on**
Bead 1 (`PanelPresence` config field)

## Bead 5: Spec findings deferred to the plan

R6.1–R6.3, R6.5, R12 for ADR-0043.

**Steps**
1. Add `--to-plan` to `panel defer`. Refuse it on gates other than `spec_approve` and with `--bead`. With `--finding` omitted, cover each finding of the slot.
2. In one ledger write, append one `deferred_to_plan` record and one `confirmed-deferred` disposition row (evidence `plan:<deferral id>`) for each covered finding. Make the ledger validator accept the kind.
3. Add `plan_deferrals` to `panel.json`. Make the integrity leg refuse an entry with no backing record. Carry the backed entries into a new round.
4. Make a backed entry discharge its finding for `spec_approve` with the refutation hash rule. Keep `implPanelDeferralPreflight` blind to `plan_deferrals`.
5. Add the `## Amendment (Spec 133)` section to ADR-0043 and its pin test. Update the domain docs.

**Verification**
- [ ] `go test ./internal/panel/... -run DeferToPlan` passes.
- [ ] `go test ./cmd/mindspec/... -run 'PanelDeferToPlan|PanelDefer'` passes.
- [ ] `go test ./internal/approve/... -run DeferralPreflight` passes.
- [ ] `go test ./internal/validate/... -run 'ADR0043|Spec130_(Core3|Deferral|PassRule|Ledger|Sizes)|ADRAmendments_Spec133'` passes.
- [ ] `go test ./internal/setup/... -run ReviewDiscipline` passes.

**Acceptance Criteria**
- [ ] `--to-plan` appends one record and one disposition row per covered finding. `panel disposition check` passes after it. A record with a `bead` field fails the test (AC21).
- [ ] `--to-plan` on a `bead` panel and `--to-plan --bead` each exit non-zero. `panel defer` without `--to-plan` on a `spec_approve` panel exits non-zero with today's message (AC21).
- [ ] A hand-added `plan_deferrals` entry makes the `spec_approve` gate decision refuse on the integrity leg. A new round carries the backed entries (AC21).
- [ ] `ApproveImpl` does not refuse on a `plan_deferrals` entry (AC21).
- [ ] No string this bead adds to a CLI message, flag help or recovery line cites an ADR number, spec number, requirement id, bead id or issue number.
- [ ] `TestADRAmendments_Spec133ADR0043` finds `deferred_to_plan`. `TestADRAmendments_Spec131ADR0043`, the spec-130 ADR-0043 pins and `TestReviewDiscipline_*` stay green.

**Depends on**
None

## Bead 6: Artifact path hard block

R5.1–R5.4.

**Steps**
1. Add optional `artifact_path` to the v2 verdict item. Refuse an absolute path, a `..` segment and a control byte, with the JSON pointer. Add the field to the schema that `panel schema` prints.
2. Give the gate a reader that checks a path at the `reviewed_head_sha` of the round's `round` ledger record.
3. Name the slot, item and path in the refusal while an item's path is absent from the round tree. Discharge it by refutation, deferral or a new round at a tree with the path. Do not let `panel rebind` discharge it.
4. Update the domain docs.

**Verification**
- [ ] `go test ./internal/panel/... -run ArtifactPath` passes.
- [ ] `go test ./cmd/mindspec/... -run 'PanelSchema|PanelSubmit'` passes.
- [ ] `go test ./internal/complete/... -run ArtifactPath` passes.

**Acceptance Criteria**
- [ ] `panel submit` refuses `../x`, `/x` and a control byte, and names the JSON pointer. `panel schema` output contains `artifact_path` (AC19).
- [ ] Every `complete` case of AC20 gives the AC20 result. A check that reads the rebound head instead of the round tree fails the rebind case (AC20).
- [ ] A verdict with no `artifact_path` gives the same decision as before (R5.4).
- [ ] No string this bead adds to a CLI message or recovery line cites an ADR number, spec number, requirement id, bead id or issue number.

**Depends on**
None

## Bead 7: complete --check and the full content-gate report

R7.1–R7.6, R7.7 (refused flags), R10, R12 for ADR-0032.

**Steps**
1. Add `--check` to `complete`. Accept it only on an open bead with a branch and no pending merge-only run. Refuse `--merge-only`, `--resolve-merge`, `--override-adr`, `--supersede-adr` and `--allow-net-deletion`.
2. Run doc-sync, the ADR gates and the work-destruction preflight on the landing tree that plain `complete` uses. Skip the panel gate and the coverage check. Write nothing.
3. Make plain `complete` run every content gate before it refuses, and name every failing gate. Share one function between `--check` and plain `complete`.
4. Make `validateDivergence` give a Warn `adr-divergence-unowned` that names the file and the domains root. Keep the undeclared-domain, attribute and uncovered cases as Errors.
5. Add the `## Amendment (Spec 133)` section to ADR-0032 and its pin test. Change `TestADR0032Amendment_ThirdAmendmentAnchorsPresent` to select the spec-122 section by its header. Update the domain docs.

**Verification**
- [ ] `go test ./internal/complete/... -run CompleteCheck` passes.
- [ ] `go test ./cmd/mindspec/... -run CompleteCheck` passes.
- [ ] `go test ./internal/validate/... -run 'Unowned|Divergence|ADR0032|ADRAmendments_Spec133'` passes.
- [ ] `go test ./internal/complete/... -run 'LandingTree|Materializ|FaultInjection'` passes.

**Acceptance Criteria**
- [ ] On a fixture with a doc-sync and an ADR failure, `--check` exits 1 and names both. `git status --porcelain`, `git rev-parse HEAD`, the bead status and the bead metadata are byte-equal before and after. On a closed bead, `--check` exits non-zero and writes nothing (AC23).
- [ ] For the five AC24 trees, the failing set of `--check` equals the refusal set of plain `complete`. A plain `complete` that returns on the first failure fails the "all three" tree (AC24).
- [ ] `--check` with each of the five refused flags exits non-zero (AC25, refused part).
- [ ] Every AC29 case gives the AC29 result. A Warn for an undeclared-domain file fails (AC29).
- [ ] No string this bead adds to a CLI message, flag help or recovery line cites an ADR number, spec number, requirement id, bead id or issue number.
- [ ] `TestADRAmendments_Spec133ADR0032` finds the unowned Warn. `TestADR0032Amendment_ThirdAmendmentAnchorsPresent` keeps every anchor and passes. The tests in `adr_domain_resolve_test.go` stay green.

**Depends on**
None

## Bead 8: No-doc-impact declaration and doc-deferral owner

R7.7 (accepted flags), R8.1–R8.4, R9.1–R9.7, R12 for ADR-0031.

**Steps**
1. Add repeatable `--no-doc-impact "<domain>: <reason>"` to `complete`. Refuse an empty reason and an untouched domain. Record domain, reason, actor and time on the bead.
2. Add `--doc-owner <bead>` and require it with `--allow-doc-skew` on `complete`. Accept only an open bead in the same epic that is not the bead itself. Record the deferral on both beads. Let a new owner move it.
3. Make `complete` of an owner refuse unless its landed change updates the docs of each deferred domain.
4. In `impl approve`, accept a domain whose source changes all came through the landed merges of declaring beads. List and refuse each open deferral. Let `impl approve --allow-doc-skew` override both and record each open deferral id on the epic. Keep the old skew record under today's check.
5. Repeat each flag value in every printed recovery that repeats the flags of the failed run. Let `--check` validate the flags, report their domains and record nothing.
6. Add the `## Amendment (Spec 133)` section to ADR-0031 and its pin test. Update the domain docs.

**Verification**
- [ ] `go test ./internal/complete/... -run 'NoDocImpact|DocOwner|CompleteCheck'` passes.
- [ ] `go test ./internal/approve/... -run 'NoDocImpact|DocOwner|DocSync'` passes.
- [ ] `go test ./internal/validate/... -run 'DocSync|Docs|ADRAmendments_Spec133'` passes.

**Acceptance Criteria**
- [ ] `--check --allow-doc-skew "<reason>" --doc-owner <open bead>` exits 0 on a doc-sync-only failure and writes no record. With a closed owner it exits non-zero (AC25).
- [ ] Every AC26 case gives the AC26 result. An exemption for a source commit made directly on `spec/<id>` fails (AC26).
- [ ] Every AC27 case gives the AC27 result. A recovery that drops `--doc-owner` fails (AC27).
- [ ] `impl approve --allow-doc-skew "<reason>"` with an open deferral exits 0 and records the deferral id and reason on the epic (AC28).
- [ ] No string this bead adds to a CLI message, flag help or recovery line cites an ADR number, spec number, requirement id, bead id or issue number.
- [ ] `TestADRAmendments_Spec133ADR0031` finds `--doc-owner`.

**Depends on**
Bead 7 (`--check` mode and the shared content-gate function)

## Bead 9: Skills, instruct and doctor guidance

R4.5, R11.1–R11.8.

**Steps**
1. Rewrite `ms-spec-approve` and `ms-plan-approve` in `internal/setup` and in the `.claude/skills`, `.agents/skills` and `plugins/mindspec/skills` copies: run the panel with `--target spec/<id>`, then the verb; the verb refuses without a passing panel. Remove the "guidance, not a mechanized preflight" sentence and the text it qualifies. Name the four change kinds that call for more than one family and `config resolve --gate <gate>`.
2. State the final-review refusal in `ms-impl-approve` and `ms-spec-final-review`. Add the `mindspec complete <bead> --check` step to `ms-bead-cycle` before the bead panel. State the `--target spec/<id>` rule and the new-round recovery in `ms-panel-run`.
3. Add the current text of each changed skill as a historical snapshot. Do not change existing snapshots.
4. Add the two `instruct` lines in Spec and Plan mode.
5. Add the `doctor` Info line under `warn` and one Warn line for each in-flight spec whose epic has `panel_absent`.
6. Update `TestLifecycleGateStep_*` for the new text. Update `project-docs/user/guides/review-panels.md` and `CHANGELOG.md`.

**Verification**
- [ ] `go test ./internal/setup/... -run 'Refresh|SurfaceRef|BeadCycleCheck|LifecycleGateStep|ProductSurface'` passes.
- [ ] `go test ./internal/instruct/... -run PanelGateLine` passes.
- [ ] `go test ./internal/doctor/... -run PanelPresence` passes.
- [ ] `git grep -n 'guidance, not a mechanized' -- internal/setup/claude.go .claude/skills .agents/skills plugins` exits 1.
- [ ] `go test ./internal/lint/... -run DestructiveGuidance` passes.

**Acceptance Criteria**
- [ ] The `git grep` above exits 1. An install over each pre-change copy writes the new text. An install over a user-changed copy keeps it (AC30).
- [ ] The `ms-impl-approve` and `ms-spec-final-review` text states the final-review refusal for every spec. In `ms-bead-cycle`, the `--check` step comes before the bead-panel step; a test that only checks presence of `--check` fails the review. The approve skills name the four change kinds. `ms-panel-run` gives `--target spec/<id>` for the three gates (AC31).
- [ ] `instruct` in Spec mode and in Plan mode prints the panel line with that mode's gate and the change-kinds line (AC32).
- [ ] `doctor` prints the Info line under `warn` and one Warn line for a Plan-mode fixture whose epic has `panel_absent`. It prints no Info line under `enforce` (AC17).
- [ ] No text this bead adds to a skill, `instruct`, `doctor`, a template or a user doc cites an ADR number, spec number, requirement id, bead id or issue number. `TestSurfaceRefs_NoInternalReferences` and `TestProductSurface_NoInternalRefs` pass on the new text (AC30).

**Depends on**
Bead 1 (`PanelPresence` config field for `doctor`)

## Provenance

Each `-run` pattern runs in the package its bead's Verification names.

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| AC1, AC4, AC7 | Bead 2: `go test ./internal/approve/... -run DocumentGate` |
| AC2 | Bead 1: `-run LifecycleGate`; Bead 2: `-run DocumentGate` |
| AC3 | Bead 1: `-run PanelCreateTarget`, `-run DocumentSubject`; Bead 2: `-run DocumentGate` |
| AC5, AC6 | Bead 1: `-run OwnWriteSet` (AC5); Bead 2: `-run 'DocumentGate|ReApprove'` |
| AC8–AC12 | Bead 3: `go test ./internal/approve/... -run FinalReviewGate` |
| AC13 | Bead 4: `go test ./internal/complete/... -run Presence` |
| AC15 | Bead 1: `-run PanelPresence`; the `warn` cases in Beads 2, 3 and 4 |
| AC16 | Bead 2: `TestDocumentGate_InFlightSpecKeepsApproval`, `TestReApprove_EditedPlanRunsGateBeforeSupersede` |
| AC17 | Bead 9: `go test ./internal/doctor/... -run PanelPresence` |
| AC18 | Bead 1: `go test ./internal/config/... -run PanelPresence` |
| AC19 | Bead 6: `go test ./internal/panel/... -run ArtifactPath` |
| AC20 | Bead 6: `-run ArtifactPath` (complete cases); Bead 2: `-run DocumentGate` (spec approve cases) |
| AC21 | Bead 5: `-run 'DeferToPlan|PanelDeferToPlan'`; Bead 2: `-run DocumentGate` (spec approve cases) |
| AC22 | Bead 2: `go test ./internal/approve/... -run Resolutions` |
| AC23, AC24 | Bead 7: `-run CompleteCheck` |
| AC25 | Bead 7: `-run CompleteCheck` (refused flags); Bead 8: `-run 'DocOwner|CompleteCheck'` (accepted flags) |
| AC26, AC27 | Bead 8: `go test ./internal/complete/... -run 'NoDocImpact|DocOwner'` |
| AC28 | Bead 8: `go test ./internal/approve/... -run DocOwner` |
| AC29 | Bead 7: `go test ./internal/validate/... -run Unowned` |
| AC30 | Bead 9: `-run 'Refresh|SurfaceRef|ProductSurface'` and the `git grep` exit 1 |
| AC31 | Bead 9: `-run 'BeadCycleCheck|LifecycleGateStep'` |
| AC32 | Bead 9: `go test ./internal/instruct/... -run PanelGateLine` |
| AC33 | Beads 1, 2, 5, 7, 8: `go test ./internal/validate/... -run ADRAmendments_Spec133` |
| AC34 | Final review: `make test` on the spec branch |
