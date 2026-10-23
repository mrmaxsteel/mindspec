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
    depends_on: [1]
    key_file_paths: [internal/panel/verdict_v2.go, internal/panel/gate.go, cmd/mindspec/panel_schema.go]
  - id: 7
    depends_on: [1, 4]
    key_file_paths: [cmd/mindspec/complete.go, internal/complete/complete.go, internal/validate/divergence.go,
        internal/validate/ownership_resolve.go, internal/validate/adr0032_amendment_test.go,
        .mindspec/adr/ADR-0032-adr-semantic-gates.md]
  - id: 8
    depends_on: [3, 7]
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

This plan builds spec 133 R1–R12 in 9 beads. The longest dependency chain is 4 beads: Bead 1, Bead 4, Bead 7, Bead 8. Beads 1 and 5 start at once. When Bead 1 merges, Beads 3, 4, 6 and 9 start at once. Each other bead starts when its dependencies merge.

## Resolutions

The spec has no to-plan deferrals. This section names the plan-time decisions.

- **Operator decisions.** OQ1–OQ8 are fixed. No bead reopens them. The post-131 defect fixes on the merged tree (deferral source binding, the ledger-only completeness check, subject-head resolution, the `impl approve` diff base) are current behavior. No bead plans them again.
- **Dogfood boundary.** See `## Testing Strategy`. Owner: every bead and every panel of this lane.
- **In-flight specs (R4.4).** Bead 2 owns the rule. This change does not check approvals that exist: a spec in Plan or Implement mode keeps its spec and plan approvals, and no other verb runs the `spec_approve` or `plan_approve` gate for them again. Each new run of `spec approve` or `plan approve` is a new approval and runs its gate (R1.1). This includes a re-run of `spec approve` on a spec whose epic exists (the replay path, `backfillExistingEpicPhase` in `internal/approve/spec.go`), and `plan approve` on an edited plan whose old beads are all open: the gate runs before the supersede close and before the new beads. A re-run after a fault is not a new approval of changed content: R1.6 applies. Tests: `TestS133DocGate_InFlightSpecKeepsApproval` (AC16, first sentence), `TestS133DocGate_ReApproveEditedPlanRunsGateBeforeSupersede` (AC16, second and third sentences) and `TestS133DocGate_SpecApproveReplayRunsGate`. Bead 9 adds the `doctor` Warn line for an in-flight spec whose epic has `panel_absent` (`TestS133DoctorPresence_InFlightWarn`).
- **Bead count (9).** Each bead is one mechanism: gate core, document gate, final-review gate, bead presence, plan deferral, artifact path, content-gate report, doc declarations, guidance. Merging two of them puts two unrelated mechanisms in one diff and one panel.
- **Dependencies for the `enforce` half.** An AC that names no policy holds under `warn` and under `enforce`. Each fixture has a passing panel for each gate other than the one under test (the spec's AC preamble). So a bead needs the presence code of another verb only when one of its cases runs that verb with no registration. Bead 7 depends on Bead 4: its `--check` case under `enforce` with no bead panel can fail only when the presence refusal of Bead 4 exists. Bead 8 depends on Bead 3 (the final-review presence of `impl approve`) and on Bead 7. Beads 3 and 6 do not depend on Bead 4: each `complete` case of Bead 3 (AC11 (b) and (c)) and of Bead 6 (AC20) has a bead registration, and Bead 4 changes only the result with no registration. Bead 5 has no edge: Bead 2 re-runs its `spec approve` and `panel defer` cases under `enforce`. No other edge is needed. Each bead re-runs its cases under both policies. The `## Policy sweep` section names the bead that verifies each such AC under `enforce`.
- **Validation Proofs commands.** The `-run` patterns in the spec's `## Validation Proofs` (`DocumentGate|FinalReviewGate|Resolutions|ReApprove`, `PanelDeferToPlan|PanelCreateTarget`, `PanelPresence`, `BeadCycleCheck` and others) match none of this plan's `TestS133<Mechanism>_` names. `go test -run` exits 0 when no test matches, so those commands would prove nothing. The commands in `## Provenance` and in each bead's Verification replace them. The spec text stays as approved.
- **Shared helpers with one owner.** Bead 1 owns these. Beads 2, 3 and 9 call them and do not write a second copy.
  - `panel.DocumentSubjectDigest`: the one subject-digest function of R1.6 for `spec_approve`, `plan_approve` and `final_review` (own write sets out, frontmatter compared as parsed keys and values). `panel create`, `panel rebind`, `panel verify` and the gates call it.
  - `config.Enforcement.PanelPresence` (`warn` or `enforce`).
  - `internal/approve/lifecycle_gate.go`: `runLifecycleGate(root, specID, gate string, worktree string, out io.Writer)`. It returns an outcome (Allow, Warn with an audit record, or a refusal) and writes nothing.
  - `recordLifecycleGateAudit(epicID string, outcome Outcome, deferralIDs []string)`. It is the one writer of the R1.7 epic records: `panel_gate_skipped`, `panel_abandoned`, `panel_absent` and the R6.4 deferral-id list. A nil list writes no deferral key. Bead 2 passes the ids from `spec approve` and nil from `plan approve`. Bead 3 passes nil. The epic-absent warning and the re-run path of R1.7 print and re-write the same set.
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
- **Test names.** Each new test of this spec has a name that starts with `TestS133<Mechanism>_`, with the mechanism prefix that its bead's Verification names. No test on the base tree matches `S133` (checked with `go test <pkg> -list` in each package). `go test -run` exits 0 with "no tests to run" when no test matches. So each bead's new tests must appear in `go test -v` output for each Verification line that names a `TestS133` prefix. A run that lists none of them fails the bead. A Verification line without a `TestS133` prefix is a regression run of tests that exist on the base tree.
- **Both policies.** Each test for an AC that names no presence policy runs its case under `warn` and under `enforce` (a table over the two values). A test that sets only one value fails the review.
- **Fixture panels.** Each fixture has a passing panel for each gate other than the one under test (the spec's AC preamble), under both policies. A fixture has no panel for a gate only when its AC names the missing panel, or when the verb does not run that gate (`complete --check` skips the bead panel gate).
- **No real model runs.** Tests use temp git repos, scratch bead stores (the existing `bd` test seams such as `SetSpecRunBDForTest`) and local fake runners. No test starts a real codex, claude or other model CLI.
- **Shared fixtures.** Bead 1 adds a test fixture builder for a spec with a registration at `spec/<id>` for a named gate (pass, each failing leg, abandoned, SHA target). Beads 2, 3 and 9 use it. Bead 6 uses the existing `internal/panel/paneltest` builders.
- **Unit tests** cover the digest, config validation, verdict schema, ledger kinds and message text. **Integration tests** drive `ApproveSpec`, `ApprovePlan`, `ApproveImpl` and `complete.Run` in temp repos, with fault injection through the existing fault seams.
- **Gate:** each bead runs its named tests and the tests of the files it touches. The final review runs `make test` (AC34).

## Bead 1: Lifecycle gate core

R1.2, R1.3 (target refusal), R1.5 (presence decision), R1.6 (digest), R1.7 (audit writer), R1.8, R1.9, R4.1, R4.6, R12 for ADR-0037.

**Steps**
1. Add `enforcement.panel_presence` to `config.Enforcement`, default `warn`. `config.Load` refuses another value and names the key. Set `enforcement.panel_presence: enforce` in `.mindspec/config.yaml`.
2. Add `panel.DocumentSubjectDigest`. It leaves out the own write sets of R1.6 and R2.5 and compares frontmatter as parsed keys and values. Use it in `panel create`, `panel rebind`, `panel verify` and the gate for `spec_approve`, `plan_approve` and `final_review`.
3. Make `panel create` refuse a target other than `spec/<id>` for those three gates.
4. Add `runLifecycleGate` and `recordLifecycleGateAudit` in `internal/approve/lifecycle_gate.go`. The gate resolves the head from the recorded target, refuses another target and names it, runs every decision leg and the coverage check on each live registration, runs the dirt check in the spec worktree outside the own write set, applies the hatches and the presence policy, and returns the outcome. The audit writer has the contract in `## Resolutions`.
5. Remove the bead merge fence and the bead fix-skill text from non-bead gate messages. Each refusal names the gate, panel and leg and ends with one `recovery:` line.
6. Add the `## Amendment (Spec 133)` section to ADR-0037 and its pin test. Update the domain docs.

**Verification**
- [ ] `go test ./internal/config/... -run '^TestS133Config_' -v` passes and lists the new tests.
- [ ] `go test ./internal/panel/... -run '^TestS133(Digest|GateText)_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS133PanelDigest_|^TestS133PanelTarget_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run 'PanelVerify|PanelRebind'` passes (regression).
- [ ] `go test ./internal/approve/... -run '^TestS133LifecycleGate_' -v` passes and lists the new tests.
- [ ] `go test ./internal/validate/... -run '^TestADRAmendments_Spec133ADR0037$|ADRAmendments|ADR0037' -v` passes and lists `TestADRAmendments_Spec133ADR0037`.
- [ ] `go test ./internal/panel/... -run RawMergeFence` passes (regression).
- [ ] `~/.local/bin/mindspec config show` exits 0 in the bead worktree.

**Acceptance Criteria**
- [ ] `panel_presence: maybe` makes `config.Load` fail with a message that names `enforcement.panel_presence`. A loader that falls back to `warn` fails the test (AC15).
- [ ] A test loads `.mindspec/config.yaml` with the config loader and reads `enforce` (AC18). A test that greps the file instead fails the review.
- [ ] `panel create --target <sha>` exits non-zero for each of the three gates. A refusal for only one gate fails (AC3).
- [ ] `DocumentSubjectDigest` is equal for a spec before and after a frontmatter rewrite with a changed key order, for a draft and an approved `## Approval` section, and for an absent section. It differs after one extra line under `## Approval` or a body edit. A digest over raw blob ids fails these tests (AC3, AC5).
- [ ] `DocumentSubjectDigest` differs after an edit to a frontmatter field outside the own write set: a new field in `spec.md`, and a `work_chunks` `depends_on` edit and an `adr_citations` edit in `plan.md`. The `spec_approve` digest differs when `plan.md` is not byte-equal to the scaffold. A digest that strips the whole frontmatter block, or that leaves `plan.md` out of the `spec_approve` subject, fails these tests (R1.6).
- [ ] For each of `spec_approve`, `plan_approve` and `final_review`: `panel create`, then `panel rebind`, then a frontmatter rewrite with a changed key order, then `panel verify` passes. The digest that `panel create` and `panel rebind` record equals `DocumentSubjectDigest` of the subject. A `panel rebind` that records the old digest form fails the verify step. A `panel verify` that compares the old digest form fails the test (R1.6, AC3).
- [ ] `runLifecycleGate` returns a refusal for each failing leg under `warn` and under `enforce`. A policy that turns a failing leg into a Warn fails the test (R1.2).
- [ ] `recordLifecycleGateAudit` writes each of `panel_gate_skipped`, `panel_abandoned` and `panel_absent` for its outcome, and the deferral-id list when one is given. A nil list writes no deferral key (R1.7, R6.4).
- [ ] With no registration, the refusal ends with `recovery: mindspec panel create <slug> --spec <id> --gate <gate> --target spec/<id>`. No non-bead gate message contains the `RawMergeFence` text or the bead fix-skill text (`/ms-bead-fix`) of the unresolved-finding message. A test checks each leg's message for `spec_approve`, `plan_approve` and `final_review` (R1.8, R1.9).
- [ ] No string this bead adds to a CLI message or recovery line cites an ADR number, spec number, requirement id, bead id or issue number. A test asserts this on each new message.
- [ ] `TestADRAmendments_Spec133ADR0037` finds the presence policy. The spec-130 and spec-131 ADR-0037 pins stay green: `TestADRAmendments_Spec130_Core3`, `_Keys`, `_Deferral`, `_PassRule`, `_Ledger`, `_Subject`, `_Rebind`, `_Verbs` and `TestADRAmendments_Spec131ADR0037`.

**Depends on**
None

## Bead 2: Document gate in spec and plan approve

R1.1, R1.4, R1.7, R4.2–R4.4 for documents, R6.4, R12 for ADR-0041.

**Steps**
1. Call `runLifecycleGate` at the start of `ApproveSpec` (`spec_approve`) and `ApprovePlan` (`plan_approve`), before every write the spec lists in R1.1, on each run. This includes the replay path for an epic that exists (`backfillExistingEpicPhase`).
2. After the epic is found or created, call `recordLifecycleGateAudit` with the outcome. `ApproveSpec` passes the id of each `deferred_to_plan` record of the spec; `ApprovePlan` passes nil. When `spec approve` cannot create the epic and a record is due, print the record in a warning, exit non-zero after the commits, and print the re-run recovery line.
3. In `ApprovePlan`, before the first write, read the epic list and every `deferred_to_plan` record in the tracked ledger of every `spec_approve` registration. Refuse unless `## Resolutions` names each id. List each missing id.
4. Apply R4.4 in full. This change does not check approvals that exist, and no other verb runs a document gate for them. Each new run of `spec approve` or `plan approve` runs its gate, the replay of `spec approve` on a spec whose epic exists included. `plan approve` on an edited plan runs its gate before the supersede close.
5. Add the `## Amendment (Spec 133)` section to ADR-0041 and its pin test. Update the domain docs.

**Verification**
- [ ] `go test ./internal/approve/... -run '^TestS133(DocGate|Resolutions)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/approve/... -run 'Fault|SpecApprove|PlanApprove'` passes (regression).
- [ ] `go test ./internal/validate/... -run '^TestADRAmendments_Spec133ADR0041$|ADR0041|Spec132Amendments|Spec130_Rebind' -v` passes and lists `TestADRAmendments_Spec133ADR0041`.
- [ ] `go test ./cmd/mindspec/... -run 'ADR0041|Reattest|Section4Finalize'` passes (regression).
- [ ] `go test ./internal/gitutil/... -run ADR0041` passes (regression).

**Acceptance Criteria**
- [ ] Under `enforce` with no panel, `spec approve` exits non-zero, names `spec_approve` and ends with the filled recovery line. HEAD, `spec.md`, the absent `plan.md` and the absent epic are unchanged. A gate that runs after the frontmatter write fails this test (AC1).
- [ ] `TestS133DocGate_SpecApproveReplayRunsGate`: under `enforce`, a re-run of `spec approve` on a Plan-mode spec whose epic exists, with no `spec_approve` panel, exits non-zero and names `spec_approve`. A verb that skips the gate when the epic exists fails this test (R1.1, R4.4).
- [ ] Each AC2 leg case refuses under `warn` and under `enforce`, names the leg, and has no merge fence. A passing panel gives exit 0 (AC2).
- [ ] Under `warn` and under `enforce`, the AC3 commit cases, the SHA-target case and the hand-edited target case give the AC3 results through `spec approve`. After `spec approve` rewrites the frontmatter with a changed key order, `panel verify` and the gate both still pass (AC3).
- [ ] Under `warn` and under `enforce`, an uncommitted body edit of `spec.md` refuses, names `spec.md`, and makes no commit (AC4).
- [ ] Every AC5 fault and re-run case gives the AC5 result, including the epic-creation failure with each due record and the deferral-id list. Hand-written approval fields with no passing panel refuse (AC5).
- [ ] Every AC6 case for `plan approve` gives the AC6 result, under each policy that the case names, and under both when it names none. A refusal creates no bead (AC6).
- [ ] The AC7 hatch cases give the AC7 results. No message contains the skip variable name (AC7).
- [ ] Under `warn`, the AC1 fixture exits 0, prints one `warning:` line that names the gate and records `panel_absent`. With no `panel_presence` key, the same holds (AC15).
- [ ] `TestS133DocGate_InFlightSpecKeepsApproval` and `TestS133DocGate_ReApproveEditedPlanRunsGateBeforeSupersede` give the AC16 results. An edited-plan refusal closes no bead and creates no bead (AC16).
- [ ] Every AC22 case gives the AC22 result under `warn` and under `enforce`. An id that occurs only outside `## Resolutions` is refused (AC22).
- [ ] Under `warn` and under `enforce`: the end-to-end `spec approve` cases of AC20 (artifact path named, then `--to-plan` passes) and AC21 (`spec approve` passes after `--to-plan`; a hand-added `plan_deferrals` entry refuses on the integrity leg) pass. The `panel defer` cases of AC21 give the AC21 results with `enforcement.panel_presence: enforce` in the fixture config (AC20, AC21).
- [ ] No string this bead adds to a CLI message or recovery line cites an ADR number, spec number, requirement id, bead id or issue number.
- [ ] `TestADRAmendments_Spec133ADR0041` finds the own write set. These pins stay green: `TestADR0041_Spec129Amendment`, `TestDocs_Spec132Amendments_*`, `TestADRAmendments_Spec130_Rebind`, `TestADR0041FourthVerbClause_*`, `TestADR0041OQ1a_*`, `TestADR0041ReattestAmendment_Anchored`, `TestADR0041_Section4FinalizeCarrierAnchors`, `TestADR0041_Section2ConvergenceCompletenessAnchors`.

**Depends on**
Bead 1 (gate core and audit writer), Bead 5 (`deferred_to_plan` records), Bead 6 (artifact block, for the AC20 case)

## Bead 3: Final-review gate in impl approve

R2.1, R2.4–R2.8, R4.2 and R4.3 for `final_review`.

**Steps**
1. Call `runLifecycleGate` for `final_review` in `ApproveImpl` before the placeholder ADR, the phase-reconcile write, the first epic mutation and each `FinalizeEpic` commit or merge. Write the audit records with `recordLifecycleGateAudit` (deferral ids nil).
2. Make the dirt check skip excluded paths. Keep `implLedgerPreflight` unchanged.
3. Treat the `--supersede-adr` placeholder, byte-equal to its text, as the own write set.
4. Before the first write, read each plan bead worktree that the `FinalizeEpic` worktree list names. Refuse on any change that `git status` reports. Name the bead and each path. Print the recovery lines of R2.6 by path class.
5. Keep the gate a pre-close gate on re-run. With the spec branch gone, let the "already landed" checks decide. With the spec worktree gone, the epic closed and the phase done, compare the subject at the spec tip with the subject at `mindspec_finalize_tip`.
6. Update the domain docs.

**Verification**
- [ ] `go test ./internal/approve/... -run '^TestS133FinalGate_' -v` passes and lists the new tests.
- [ ] `go test ./internal/approve/... -run 'ImplFault|ImplPreflight|Orphan|WorktreeEnumeration'` passes (regression).
- [ ] `go test ./internal/executor/... -run 'Finalize|MergedFinalize'` passes (regression).
- [ ] `go test ./internal/lint/... -run ProductDiagnosticScan` passes (regression).

**Acceptance Criteria**
- [ ] Every AC8 case gives the AC8 result. No placeholder ADR exists after a refused run with `--supersede-adr` (AC8).
- [ ] Under `warn`, the AC8 fixture (2 plan beads, no `final_review` registration) exits 0, prints one `warning:` line that names `final_review` and records `panel_absent` on the epic (AC15).
- [ ] With 1 plan bead, `warn` gives exit 0, one `warning:` line and `panel_absent` on the epic; `enforce` refuses before any write. A gate that skips one-bead specs fails (AC9, AC15).
- [ ] Every AC10 case gives the AC10 result. The flat-layout ledger case and the `.beads/issues.jsonl` case run under `warn` and under `enforce`. A dirt check that names the ledger or `.beads/issues.jsonl` fails (AC10).
- [ ] Under `warn` and under `enforce`, every AC11 case (a)–(d) gives the AC11 result, the `mindspec complete <bead>` recovery steps included. Each merged plan bead in these fixtures has a passing bead panel, so `complete <bead>` gives the same result before and after Bead 4. A check that runs inside `FinalizeEpic` after the spec-artifact commit fails case (b). A recovery that prints `panel create` for an export-only path fails case (c) (AC11).
- [ ] Every AC12 fault, re-run and no-worktree case gives the AC12 result. The cases that name no policy run under both (AC12).
- [ ] A committed edit to the placeholder ADR beyond the placeholder text makes the `final_review` gate refuse with a stale-subject message. An own write set that leaves out the placeholder path whatever its content fails this test (R2.5).
- [ ] No string this bead adds to a CLI message or recovery line cites an ADR number, spec number, requirement id, bead id or issue number.
- [ ] The ADR-0041 pins listed in Bead 2 stay green after this bead (it does not edit the ADR).

**Depends on**
Bead 1 (gate core)

## Bead 4: Bead panel presence in complete

R3.1, R3.3, R4.2 and R4.3 for the bead gate.

**Steps**
1. Replace the "no registration → Allow" result of `panelGate` with the presence policy.
2. Under `warn`, print one `warning:` line with the gate and recovery command, and record `panel_absent` on the bead.
3. Under `enforce`, refuse on each `complete` path that runs the panel gate today, with the bead state that R3.1 gives for each path. Print the two closed-bead recovery lines. Do not put the refusal on a path that does not run the panel gate today.
4. Keep the path-4 cleanup guard a refusal under both policies. Keep the branch-less reconcile path, the busy-worktree refusal and the refutation-obligation branch unchanged.
5. Update the domain docs.

**Verification**
- [ ] `go test ./internal/complete/... -run '^TestS133BeadPresence_' -v` passes and lists the new tests.
- [ ] `go test ./internal/complete/... -run 'PanelGate|ClosedUnmerged|MergeOnly|Cleanup'` passes (regression).

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
- [ ] `go test ./internal/panel/... -run '^TestS133DeferToPlan_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS133DeferToPlan_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run PanelDefer` passes (regression).
- [ ] `go test ./internal/approve/... -run DeferralPreflight` passes (regression).
- [ ] `go test ./internal/validate/... -run '^TestADRAmendments_Spec133ADR0043$|ADR0043|Spec130_(Core3|Deferral|PassRule|Ledger|Sizes)' -v` passes and lists `TestADRAmendments_Spec133ADR0043`.
- [ ] `go test ./internal/setup/... -run ReviewDiscipline` passes (regression).

**Acceptance Criteria**
- [ ] `--to-plan` appends one record and one disposition row per covered finding. `panel disposition check` passes after it. A record with a `bead` field fails the test (AC21).
- [ ] `--to-plan` on a `bead` panel and `--to-plan --bead` each exit non-zero. `panel defer` without `--to-plan` on a `spec_approve` panel exits non-zero with today's message (AC21).
- [ ] A hand-added `plan_deferrals` entry makes the `spec_approve` gate decision refuse on the integrity leg. A new round carries the backed entries (AC21).
- [ ] A `deferred_to_plan` record whose finding hash differs from the hash of the current finding text does not discharge that finding for the `spec_approve` gate: the unresolved-finding leg refuses. A discharge that matches on slot, round and finding id only fails this test (R6.3).
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
- [ ] `go test ./internal/panel/... -run '^TestS133ArtifactPath_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS133ArtifactPath_' -v` passes and lists the new tests.
- [ ] `go test ./internal/complete/... -run '^TestS133ArtifactPath_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run 'PanelSchema|PanelSubmit'` passes (regression).

**Acceptance Criteria**
- [ ] `panel submit` refuses `../x`, `/x` and a control byte, and names the JSON pointer. `panel schema` output contains `artifact_path` (AC19).
- [ ] Under `warn` and under `enforce`, every `complete` case of AC20 gives the AC20 result. Each case has a bead registration with the finding-bearing round. A check that reads the rebound head instead of the round tree fails the rebind case (AC20).
- [ ] A verdict with no `artifact_path` gives the same decision as before (R5.4).
- [ ] No string this bead adds to a CLI message or recovery line cites an ADR number, spec number, requirement id, bead id or issue number.

**Depends on**
Bead 1 (`PanelPresence` config field)

## Bead 7: complete --check and the full content-gate report

R7.1–R7.6, R7.7 (refused flags), R10, R12 for ADR-0032.

**Steps**
1. Add `--check` to `complete`. Accept it only on an open bead with a branch and no pending merge-only run. Refuse `--merge-only`, `--resolve-merge`, `--override-adr`, `--supersede-adr` and `--allow-net-deletion`.
2. Run doc-sync, the ADR gates and the work-destruction preflight on the landing tree that plain `complete` uses. Fork `--check` before the panel gate and its presence refusal. Skip the panel gate and the coverage check. Write nothing.
3. Make plain `complete` run every content gate before it refuses, and name every failing gate. Share one function between `--check` and plain `complete`.
4. Make `validateDivergence` give a Warn `adr-divergence-unowned` that names the file and the domains root. Keep the undeclared-domain, attribute and uncovered cases as Errors.
5. Re-seat the existing guards whose red signal depends on an unowned file being an Error (list below).
6. Add the `## Amendment (Spec 133)` section to ADR-0032 and its pin test. Change `TestADR0032Amendment_ThirdAmendmentAnchorsPresent` to select the spec-122 section by its header. Update the domain docs.

**Guards to re-seat.** The list comes from the substring grep `grep -rn 'adr-divergence\|unowned' --include='*_test.go' internal cmd` on the base tree. Each name is the enclosing test function (a hit in a doc comment belongs to the function below it). A hit whose red signal does not need an unowned Error (an uncovered, attribute, proposed or load case, a load count, a direct ownership read, or a message sample) is not listed.
- Guards of another mechanism. Each must use a fixture where the wrong read makes the file owned by a declared domain outside the spec's Impacted Domains, so the failure stays an Error (R10.2): `TestCommittedRangeGate_RevokedAtSpecTipRefuses` and `TestCommittedRangeGate_SourceConflictPassesThenResolves` (`internal/complete/landing_tree_test.go`), `TestCompleteRun_RefAnchoredOwnershipClaimPassesFromMainRoot` (`internal/complete/refanchor_e2e_test.go`, its RED-on-revert of the ownership read), `TestComplete_OwnershipResolutionRevokesClaimRefuses` (`internal/complete/ownership_conflict_test.go`), `TestBead6Leg_i_UntrackedFileGatesBeforeCommit` (the gate measures untracked files before any commit) and `TestBead6Leg_xxiv_ForgedMarkerAndTrailerNeverResumePastGates` (a forged marker cannot resume past the gates) (`internal/complete/bead6_fixture_legs_test.go`), `TestApproveImpl_WholeBranchOwnershipFromRef` and `TestApproveImpl_ScenarioImplApproveCoverageTriple` (`internal/approve/impl_test.go`).
- Guards whose wrong read makes the file unowned by every domain, so no re-seat keeps an Error. Each also asserts that no `adr-divergence-unowned` issue exists at any severity, not only `HasFailures()`: `TestValidateDivergence_BranchOnlyDomainDirFromRef` (`ownership_ref_test.go`, its RED-on-revert of the ref-anchored enumeration) and `TestValidateDivergence_ProjectDocsNotUnowned` (`layout_classifier_test.go`).
- Guards that read the unowned finding from the `ValidateDivergence` findings slice: `TestProcessArtifactsFiltered` and `TestRootOperatorDocsNotUnowned` (`divergence_test.go`). They stay as they are. The findings-slice AC below keeps them falsifiable.
- Tests of the unowned case itself. Each asserts the Warn (same name, the file, no failure) and adds an R10.2 case that stays an Error: `TestUnownedFileRejected`, `TestValidateDivergenceGenuinelyUnownedStillFails`, `TestValidateDivergence_ForwardOnlyGrandfathersApprovedMidFlight` (`plan_test.go`), `TestValidateDivergenceUnownedSplitNamesRealOwner`, `TestValidateDivergenceUnownedHintLayoutAware`, `TestValidateDivergenceUnownedIndeterminateOnBrokenSibling`, `TestValidateDivergenceUnownedHintRefConsistentRoot` (`divergence_test.go`).
- Tests that pin the message text or level. Each is updated only where the text or level of its own case changes (an R10.2 case keeps the Error): `TestValidateDiagnosticsByteIdenticalAfterCaching` (`divergence_caching_test.go`), `TestComplete_HandMergeImpactedDomainsExitCloses` (`internal/complete/merge_only_test.go`), the `divergenceFoldedFixture` golden fixture (`internal/redact/golden_corpus_test.go`).
- Tests that assert the absence of the finding stay as they are: `TestValidateDivergenceFilePathImpactedDomainResolves`, `Test147EndToEndZeroErrors`.

**Verification**
- [ ] `go test ./internal/complete/... -run '^TestS133CompleteCheck_' -v` passes and lists the new tests.
- [ ] `go test ./cmd/mindspec/... -run '^TestS133CompleteCheck_' -v` passes and lists the new tests.
- [ ] `go test ./internal/validate/... -run '^TestS133UnownedWarn_|^TestADRAmendments_Spec133ADR0032$' -v` passes and lists the new tests.
- [ ] `go test ./internal/validate/... -run 'Unowned|Divergence|ADR0032'` passes (regression and re-seated guards).
- [ ] `go test ./internal/complete/... -run 'LandingTree|Materializ|FaultInjection|CommittedRangeGate|RefAnchored|OwnershipResolution|HandMerge'` passes (regression and re-seated guards).
- [ ] `go test ./internal/complete/... -run 'Bead6Leg_(i|xxiv)_|OverrideUnblocks|ADRDivergenceFailure_RepairFirstLadder' -v` passes and lists the four tests (re-seated guards and uncovered-Error regression).
- [ ] `go test ./internal/validate/... -run 'BranchOnlyDomainDirFromRef|ProjectDocsNotUnowned|ProcessArtifactsFiltered|RootOperatorDocsNotUnowned' -v` passes and lists the four tests (re-seated guards).
- [ ] `go test ./internal/approve/... -run 'WholeBranchOwnershipFromRef|ScenarioImplApproveCoverageTriple'` passes (re-seated guards).
- [ ] `go test ./internal/redact/... -run Golden` passes (regression).

**Acceptance Criteria**
- [ ] On a fixture with a doc-sync and an ADR failure, `--check` exits 1 and names both. `git status --porcelain`, `git rev-parse HEAD`, the bead status and the bead metadata are byte-equal before and after. On a closed bead, `--check` exits non-zero and writes nothing (AC23).
- [ ] Under `enforce`, `complete <bead> --check` on an open bead with no panel exits 0 on a passing tree and reports only content gates: no panel-gate line and no `panel_absent` record. A presence refusal before the `--check` fork fails this test (R7.3, AC23).
- [ ] Each AC23, AC24, AC25 (refused part) and AC29 case runs under `warn` and under `enforce` and gives the same result under both. Each plain `complete` fixture (the AC24 comparisons and the AC29 completion cases) has a passing bead panel. Only the `--check` cases (AC23, the `--check` half of AC24, AC25) have no bead panel.
- [ ] Control: under `enforce`, on the AC24 pass tree with no bead panel, plain `complete` exits non-zero and names the bead panel gate, and `--check` exits 0. A plain `complete` that acts as `warn` under `enforce` fails this test.
- [ ] For the five AC24 trees, the failing set of `--check` equals the refusal set of plain `complete`. A plain `complete` that returns on the first failure fails the "all three" tree (AC24).
- [ ] `--check` with each of the five refused flags exits non-zero (AC25, refused part).
- [ ] Every AC29 case gives the AC29 result. A Warn for an undeclared-domain file fails (AC29).
- [ ] The unowned Warn still returns its finding (Kind `unowned`, the path) in the `ValidateDivergence` findings slice. A Warn that drops the finding fails a `TestS133UnownedWarn_` case (R10.1).
- [ ] Each guard in "Guards to re-seat" passes. For each guard of another mechanism, a revert of the code it guards still makes the test fail with an Error (the bead report names the revert and the red assertion for each). Inverting a guard's expectation to accept the Warn fails the review.
- [ ] No string this bead adds to a CLI message, flag help or recovery line cites an ADR number, spec number, requirement id, bead id or issue number.
- [ ] `TestADRAmendments_Spec133ADR0032` finds the unowned Warn. `TestADR0032Amendment_ThirdAmendmentAnchorsPresent` keeps every anchor and passes. The tests in `adr_domain_resolve_test.go` stay green.

**Depends on**
Bead 1 (`PanelPresence` config field), Bead 4 (bead-gate presence: the `--check` case with no bead panel under `enforce` and the control case need its refusal)

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
- [ ] `go test ./internal/complete/... -run '^TestS133(NoDocImpact|DocOwner)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/approve/... -run '^TestS133(NoDocImpact|DocOwner)_' -v` passes and lists the new tests.
- [ ] `go test ./internal/approve/... -run DocSync` passes (regression).
- [ ] `go test ./internal/validate/... -run '^TestADRAmendments_Spec133ADR0031$|DocSync|Docs' -v` passes and lists `TestADRAmendments_Spec133ADR0031`.

**Acceptance Criteria**
- [ ] `--check --allow-doc-skew "<reason>" --doc-owner <open bead>` exits 0 on a doc-sync-only failure and writes no record. With a closed owner it exits non-zero (AC25).
- [ ] `--check` with `--no-doc-impact` values, and `--check` with `--allow-doc-skew --doc-owner`, each print every domain that the declaration or the deferral covers. Output that does not name a covered domain fails the test (R7.7).
- [ ] Every AC26 case gives the AC26 result. An exemption for a source commit made directly on `spec/<id>` fails (AC26).
- [ ] Every AC27 case gives the AC27 result. A recovery that drops `--doc-owner` fails (AC27).
- [ ] `impl approve --allow-doc-skew "<reason>"` with an open deferral exits 0 and records the deferral id and reason on the epic (AC28).
- [ ] Each AC25 (accepted part) and AC26–AC28 case runs under `warn` and under `enforce`, with passing bead and final-review panels, and gives the same result under both.
- [ ] No string this bead adds to a CLI message, flag help or recovery line cites an ADR number, spec number, requirement id, bead id or issue number.
- [ ] `TestADRAmendments_Spec133ADR0031` finds `--doc-owner`.

**Depends on**
Bead 3 (final-review presence, for the `impl approve` cases under `enforce`), Bead 7 (`--check` mode and the shared content-gate function)

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
- [ ] `go test ./internal/setup/... -run '^TestS133Guidance_' -v` passes and lists the new tests.
- [ ] `go test ./internal/setup/... -run 'Refresh|SurfaceRef|LifecycleGateStep|ProductSurface'` passes (regression and updated tests).
- [ ] `go test ./internal/instruct/... -run '^TestS133InstructGate_' -v` passes and lists the new tests.
- [ ] `go test ./internal/doctor/... -run '^TestS133DoctorPresence_' -v` passes and lists the new tests.
- [ ] `git grep -n 'guidance, not a mechanized' -- internal/setup/claude.go .claude/skills .agents/skills plugins` exits 1.
- [ ] `go test ./internal/lint/... -run DestructiveGuidance` passes (regression).

**Acceptance Criteria**
- [ ] The `git grep` above exits 1. An install over each pre-change copy writes the new text. An install over a user-changed copy keeps it (AC30).
- [ ] The `ms-impl-approve` and `ms-spec-final-review` text states the final-review refusal for every spec. In `ms-bead-cycle`, the `--check` step comes before the bead-panel step; a test that only checks presence of `--check` fails the review. The approve skills name the four change kinds. `ms-panel-run` gives `--target spec/<id>` for the three gates (AC31).
- [ ] `instruct` in Spec mode and in Plan mode prints the panel line with that mode's gate and the change-kinds line, under `warn` and under `enforce` (AC32).
- [ ] `doctor` prints the Info line under `warn` and one Warn line for a Plan-mode fixture whose epic has `panel_absent`, under `warn` and under `enforce`. It prints no Info line under `enforce` (AC17).
- [ ] No text this bead adds to a skill, `instruct`, `doctor`, a template or a user doc cites an ADR number, spec number, requirement id, bead id or issue number. `TestSurfaceRefs_NoInternalReferences` and `TestProductSurface_NoInternalRefs` pass on the new text (AC30).

**Depends on**
Bead 1 (`PanelPresence` config field for `doctor`)

## Policy sweep

An AC that names no presence policy holds under `warn` and under `enforce`. This table names the bead that verifies each such AC, or each such part, under `enforce`. ACs that name the policy of each case (AC1, AC2, AC5, AC7, AC8, AC9, AC13, AC15, AC16, AC22) are not listed. AC10 names `enforce` only for its first case. AC33 and AC34 check static text and the full suite.

| AC (part with no policy) | Bead that verifies it under `enforce` |
|---|---|
| AC3, AC4 | Bead 2 |
| AC6 (the cases that name no policy) | Bead 2 |
| AC10 (the flat-layout ledger case and the `.beads/issues.jsonl` case) | Bead 3 |
| AC11, AC12 (the cases that name no policy) | Bead 3 |
| AC17 (the Warn line) | Bead 9 |
| AC18 | Bead 1 (the config file, policy-free) |
| AC19 | Bead 6 |
| AC20 (`complete` cases) | Bead 6 |
| AC20 (`spec approve` cases), AC21 | Bead 2 |
| AC23, AC24, AC25 (refused part), AC29 | Bead 7 |
| AC25 (accepted part), AC26, AC27, AC28 | Bead 8 |
| AC30, AC31, AC32 | Bead 9 |

## Provenance

Each `-run` pattern runs in the package its bead's Verification names. A `TestS133` pattern must list its new tests in `go test -v` output.

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| AC1, AC4, AC7 | Bead 2: `go test ./internal/approve/... -run '^TestS133DocGate_'` |
| AC2 | Bead 1: `-run '^TestS133LifecycleGate_'`; Bead 2: `-run '^TestS133DocGate_'` |
| AC3 | Bead 1: `-run '^TestS133PanelTarget_|^TestS133PanelDigest_'`, `-run '^TestS133Digest_'`; Bead 2: `-run '^TestS133DocGate_'` |
| AC5, AC6 | Bead 1: `-run '^TestS133Digest_'` (AC5); Bead 2: `-run '^TestS133DocGate_'` |
| AC8–AC12 | Bead 3: `go test ./internal/approve/... -run '^TestS133FinalGate_'` |
| AC13 | Bead 4: `go test ./internal/complete/... -run '^TestS133BeadPresence_'` |
| AC15 | Bead 1: `-run '^TestS133Config_'`; the `warn` cases in Beads 2, 3 and 4 (`^TestS133DocGate_`, `^TestS133FinalGate_`, `^TestS133BeadPresence_`) |
| AC16 | Bead 2: `TestS133DocGate_InFlightSpecKeepsApproval`, `TestS133DocGate_ReApproveEditedPlanRunsGateBeforeSupersede` |
| AC17 | Bead 9: `go test ./internal/doctor/... -run '^TestS133DoctorPresence_'` |
| AC18 | Bead 1: `go test ./internal/config/... -run '^TestS133Config_'` |
| AC19 | Bead 6: `go test ./cmd/mindspec/... -run '^TestS133ArtifactPath_'` and `./internal/panel/...` |
| AC20 | Bead 6: `go test ./internal/complete/... -run '^TestS133ArtifactPath_'` (complete cases); Bead 2: `-run '^TestS133DocGate_'` (spec approve cases) |
| AC21 | Bead 5: `-run '^TestS133DeferToPlan_'` (panel and cmd); Bead 2: `-run '^TestS133DocGate_'` (spec approve cases) |
| AC22 | Bead 2: `go test ./internal/approve/... -run '^TestS133Resolutions_'` |
| AC23, AC24 | Bead 7: `-run '^TestS133CompleteCheck_'` (complete and cmd) |
| AC25 | Bead 7: `-run '^TestS133CompleteCheck_'` (refused flags); Bead 8: `go test ./internal/complete/... -run '^TestS133DocOwner_'` (accepted flags) |
| AC26, AC27 | Bead 8: `go test ./internal/complete/... -run '^TestS133(NoDocImpact|DocOwner)_'` |
| AC28 | Bead 8: `go test ./internal/approve/... -run '^TestS133DocOwner_'` |
| AC29 | Bead 7: `go test ./internal/validate/... -run '^TestS133UnownedWarn_'` |
| AC30 | Bead 9: `-run '^TestS133Guidance_'`, `-run 'Refresh|SurfaceRef|ProductSurface'` and the `git grep` exit 1 |
| AC31 | Bead 9: `go test ./internal/setup/... -run '^TestS133Guidance_'`, `-run LifecycleGateStep` |
| AC32 | Bead 9: `go test ./internal/instruct/... -run '^TestS133InstructGate_'` |
| AC33 | Beads 1, 2, 5, 7, 8: `go test ./internal/validate/... -run '^TestADRAmendments_Spec133ADR'` |
| AC34 | Final review: `make test` on the spec branch |
