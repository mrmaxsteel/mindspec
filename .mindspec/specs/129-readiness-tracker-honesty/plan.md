---
status: Draft
spec_id: 129-readiness-tracker-honesty
version: "1"
adr_citations:
  - ADR-0023
  - ADR-0025
  - ADR-0030
  - ADR-0035
  - ADR-0037
  - ADR-0039
  - ADR-0040
  - ADR-0041
  - ADR-0042
  - ADR-0045
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/bead/bdcli.go
      - internal/bead/bdcli_test.go
      - internal/bead/hygiene.go
      - internal/next/beads.go
      - internal/next/beads_test.go
      - internal/lifecycle/orphans.go
      - internal/lifecycle/stale_open.go
      - internal/complete/complete.go
      - internal/approve/adopt_lattice.go
      - internal/approve/plan_test.go
      - internal/approve/spec_test.go
      - internal/doctor/beads.go
      - internal/lint/rowcap_test.go
  - id: 2
    depends_on: []
    key_file_paths:
      - internal/gitutil/gitops.go
      - internal/gitutil/reachable_merges_test.go
      - internal/lifecycle/landed.go
      - internal/lifecycle/landed_test.go
      - internal/lifecycle/fallback.go
      - internal/lifecycle/fallback_test.go
      - internal/lifecycle/reattest.go
      - internal/lifecycle/reattest_test.go
      - cmd/mindspec/reattest.go
      - internal/lint/ratchet_argv_table_test.go
      - .mindspec/adr/ADR-0041-gate-before-mutate.md
      - internal/validate/adr0041_spec129_amendment_test.go
  - id: 3
    depends_on: []
    key_file_paths:
      - internal/bead/planbead.go
      - internal/bead/planbead_test.go
      - internal/validate/readiness/readiness.go
      - internal/validate/readiness/report.go
      - internal/validate/readiness/records.go
      - internal/validate/readiness/records_test.go
      - internal/validate/readiness/mf2_forms_test.go
      - internal/validate/readiness/readiness_test.go
  - id: 4
    depends_on: [2, 3]
    key_file_paths:
      - internal/validate/readiness/readiness.go
      - internal/validate/readiness/mf3.go
      - internal/validate/readiness/fixtures.go
      - internal/validate/readiness/readiness_test.go
      - internal/validate/readiness/mf3_test.go
  - id: 5
    depends_on: [1, 3]
    key_file_paths:
      - internal/next/beads.go
      - internal/next/select.go
      - internal/next/nearmiss.go
      - internal/next/planread.go
      - internal/next/nearmiss_test.go
      - internal/next/beads_test.go
      - internal/gitutil/fileprobe.go
      - internal/gitutil/fileprobe_test.go
      - cmd/mindspec/next.go
      - cmd/mindspec/next_test.go
      - internal/instruct/templates/implement.md
  - id: 6
    depends_on: [3]
    key_file_paths:
      - internal/bead/readiness.go
      - internal/bead/override_marker.go
      - internal/bead/override_marker_test.go
      - internal/validate/readiness/override.go
      - internal/validate/readiness/override_test.go
      - cmd/mindspec/bead_ready.go
      - cmd/mindspec/bead_ready_test.go
      - plugins/mindspec/skills/ms-bead-impl/SKILL.md
      - plugins/mindspec/skills/ms-bead-cycle/SKILL.md
      - .claude/skills/ms-bead-impl/SKILL.md
      - internal/setup/readiness_skill_pin_test.go
      - internal/setup/skills.go
  - id: 7
    depends_on: [4, 5, 6]
    key_file_paths:
      - internal/next/ready_gate.go
      - internal/next/ready_gate_test.go
      - internal/next/evidence.go
      - cmd/mindspec/next.go
      - cmd/mindspec/next_test.go
      - cmd/mindspec/bead_clarify.go
      - .mindspec/adr/ADR-0041-gate-before-mutate.md
      - internal/validate/adr0041_spec129_amendment_test.go
      - .mindspec/core/USAGE.md
      - .mindspec/core/WORKFLOW-STATE-MACHINE.md
      - .mindspec/domains/workflow/architecture.md
      - .mindspec/domains/workflow/interfaces.md
      - .mindspec/domains/workflow/runbook.md
      - .mindspec/domains/execution/interfaces.md
  - id: 8
    depends_on: []
    key_file_paths:
      - internal/gitutil/gitignore.go
      - internal/gitutil/gitignore_test.go
      - internal/gitutil/reviews_ignore_test.go
      - internal/bootstrap/bootstrap.go
      - internal/bootstrap/gitignore_test.go
      - internal/complete/panel_gate_untracked_test.go
      - .gitignore
---
# Plan: 129-readiness-tracker-honesty

*Citation scope: every code anchor below names a symbol, file or template. Each was checked against `spec/129-readiness-tracker-honesty` at `7bae2664`, by inspection (read, not executed) unless a line says "run". No line numbers are cited.*

This plan decomposes spec 129 R1–R8, as amended on 2026-10-19 (F2 retired; R8 added), into **eight beads**. The longest dependency chain is **three beads**. R7 (ADR-0045 and its ADR-0040/ADR-0044 back-pointers) already landed with the spec commit `7bae2664` (checked: `.mindspec/adr/ADR-0045-binary-owns-sequence-and-rules.md` carries `- **Status**: Accepted`; ADR-0040 and ADR-0044 each carry `> **Amended by [ADR-0045](…)`). No bead re-authors it; Bead 6 re-verifies it under AC-18.

## Plan-level choices

### Decomposition and chain length

| Bead | Title (short) | Requirements | depends_on | Longest chain ending here |
|---|---|---|---|---|
| 1 | bd row-cap waist + linkage pins | R2, R3 | — | 1 |
| 2 | Landed-merge truth in `internal/lifecycle` | R4.2 (F1, F3, F4, F5; advisory registrations), R4.3, ADR-0041 amendment parts (i)–(ii) | — | 1 |
| 3 | Readiness foundation: plan-bead helpers, MF-2 forms, failure records | R1.1 helpers, R5, R6.2 records (MF-1/2/4), R1.8 lineage return | — | 1 |
| 4 | MF-3 membership, message arms, aggregation | R4.1, R4.4, R4.5 | 2, 3 | 2 |
| 5 | Spec-scoped selection honesty | R1.1–R1.7 | 1, 3 | 2 |
| 6 | Validated override marker + `ready-check` read-back + skill move | R6.3, R6.4, R6.5 (skills) | 3 | 2 |
| 7 | Claim guard + scoped `--allow-not-ready` + doc sweep | R1.8, R6.1, R6.2 (writer), R6.5 (docs), ADR-0041 amendment part (iii) | 4, 5, 6 | 3 |
| 8 | Review artifacts out of git by default | R8.1–R8.3 | — | 1 |

Chains (counted by beads on the path): 2→4→7, 3→4→7, 1→5→7, 3→5→7, 3→6→7, each 3; Bead 8 stands alone, 1. Nothing is longer.

**Why eight beads, not 3–5.** `mindspec instruct` prefers 3–5 beads and asks for a justification above 6. The spec itself (OQ-1) estimates 9–12 beads. The dispatch brief bounds this plan to 5–8 with a chain of at most 3. Eight is the smallest count that keeps each bead under one review centre:
- Bead 2 (lifecycle) and Bead 4 (readiness) cannot merge: Bead 2 alone carries most of AC-11's real-git fixtures.
- Bead 6 (reader) and Bead 7 (writer) split so the chain stays at 3. R6's two halves meet only through the marker schema (Bead 6) and the digest function (Bead 3).
- R1.8 rides in Bead 7, not Bead 5. Both R1.8 and R6.1 change `next.GateReadiness`'s signature, so one bead changes it once.
- R8 (Bead 8) shares no file and no review centre with R1–R6: it is init and `.gitignore` text plus untracked-file tests. Folding it into Bead 1 would put a `core`-owned file and the `workflow` bootstrap into the row-cap bead's review.

**R6 relief valve.** Not fired. R6 is two beads (6, 7). If the plan gate still judges 129 over-compressed, Beads 6 and 7 split out cleanly as a further plain-numbered spec. Bead 7's R1.8 part would then move to Bead 5, and Bead 5 would gain `ready_gate.go`.

### Shared files and serialisation

These files are edited by more than one bead. Every pair below is ordered by a dependency edge, so no two parallel beads edit the same file.

| File | Beads (in order) | Ordered by |
|---|---|---|
| `internal/next/beads.go` | 1 → 5 | 5 depends on 1 |
| `internal/validate/readiness/readiness.go` | 3 → 4 | 4 depends on 3 |
| `cmd/mindspec/next.go` | 5 → 7 | 7 depends on 5 |
| `.mindspec/adr/ADR-0041-gate-before-mutate.md`, `internal/validate/adr0041_spec129_amendment_test.go` | 2 → 7 | 7 depends on 4, which depends on 2 |
| `internal/gitutil/` | 2 edits `gitops.go`; 5 adds the new file `fileprobe.go`; 8 edits `gitignore.go` and adds `reviews_ignore_test.go` | disjoint files |
| `internal/complete/` | 1 edits `complete.go`; 8 adds `panel_gate_untracked_test.go` | disjoint files |
| `internal/bead/` | 1 edits `bdcli.go`/`hygiene.go`; 3 adds `planbead.go`; 6 edits `readiness.go` and adds `override_marker.go` | disjoint files |

Beads 1, 2, 3 and 8 are mutually parallel. Beads 4, 5 and 6 are mutually parallel. Bead 6 adds new files in `internal/validate/readiness/` (`override.go`) and does not edit `readiness.go`, so it never conflicts with Bead 4.

### OQ-3 (can the panel leg confirm after spec 128 Bead 6?) — decision

The spec asks the plan stage to run the fixture. **It was not run at plan time (named gap 1).** By inspection, spec Background D.5 already records the reproduced shape: `mindspec complete` lands commits that no panel reviewed (materialization or `.beads/issues.jsonl` sync), so the merge's second parent differs from the registered `reviewed_head_sha`. The spec says OQ-3 "re-opens R4.3's wording, never its rule", so no bead's code depends on the answer.

Disposition:
- Bead 2 writes ADR-0041 amendment part (ii) with D.5's wording: on the normal path the panel leg is non-corroborating, and AC-12 (iv)/(v) is the normal path.
- Before Bead 2's panel, the orchestrator runs the OQ-3 fixture as a Validation Proof: panel reviews `B`; `mindspec complete` lands; compare the merge's second parent with `B` on a clean worktree. If they are ever equal, the orchestrator amends one sentence of part (ii) to name AC-12 (i) as the live proof. No code changes either way.

### OQ-10 (which plan.md does readiness read?) — decision: keep the notice

`readiness.EvaluateReadiness` keeps reading the invoking checkout's `plan.md`. Selection (Bead 5) reads the committed `spec/<S>` blob and prints R1.1's divergence notice when the two differ.

Rationale:
- Moving readiness to the blob would route a git read into `internal/validate/readiness`. ADR-0030 allows that only through `internal/lifecycle`. It would also change the input of the AC-12 isolation test, `internal/complete/readiness_isolation_test.go`. That is a larger contract change than R1 needs.
- The divergence notice is already spec-mandated, and `GateReadiness` refuses fail-closed when the checkout disagrees (an MF-1 failure or the "reading plan.md" error), so the boundary stays honest.

Re-open condition (from the spec): a shipped flow is found that edits `plan.md` in a bead worktree before committing it.

### R2.4 (bd version floor 1.0.4) — decision

Planning could not verify the three facts at bd 1.0.4: `-n 0` unlimited, `ready --json` rows carry `parent` when set, and they carry `metadata` when set. No bd 1.0.4 binary was available, and none was run (named gap 2). The spec measured them on bd 1.1.0.

Bead 1 therefore takes the spec's "otherwise" branch, unless its implementer can show all three at 1.0.4 from the bd 1.0.4 source or changelog, with the evidence cited in the doc comment:
- `doctor`'s `bdVersionFloor` (`internal/doctor/beads.go`, `const bdVersionFloor = "1.0.4"`) is raised to `"1.1.0"`.
- The doc comment cites spec 129 Background B and C as the measured evidence.

Raising the floor is the fail-closed direction: an older bd gets a doctor warning instead of a silent short read.

### Fixture realisability notes

- **AC-1 bead X (conflicting epic dependency).** The spec asks planning to confirm on real bd that a `parent-child` E row can also carry a dependency on epic E2 that `bd show` returns first. This was **not checked on real bd (named gap 3)**. Both shapes classify as N2c under R1.2, so Bead 5's fake-bd fixture runs both legs:
  - X with parent E plus a first-returned epic dependency on E2;
  - X′ with `parent: E2`.

  The first leg's comment records "realisability on real bd unverified (plan gap 3)". AC-1's expected outcome (X is N2c, never a member) is asserted on both.
- **`mindspec panel create`'s default round.** Checked: `panelCreateCmd.Flags().Int("round", 1, "Panel round (default 1; bump on re-panel)")` in `cmd/mindspec/panel.go`. No check depends on it (R4.2). AC-11 (xv) uses round 1 for the new slug, matching this default.

## Resolutions of spec round-7 notes (mindspec-haw6)

The 2026-10-19 spec amendment retired F2, which changes three of the four carrier items.

### O2/O3 — short all-hex and ref-like `reviewed_head_sha` values → **moot**

The finding was that class (b) skips values such as `abc12` or `bead/x` that can name unlanded commits, which was fail-OPEN on F2. With F2 retired, no registration refuses anything, so a skip cannot open a gate. The values R4.3 admits as set members are unchanged: only a `wellFormedGitObjectID` (7–64 hex) is a member, and the set confirms only on exact string equality with the second parent, so `abc12` and `bead/x` can never confirm. Bead 2 keeps the classifier only as R4.3's argv hygiene.

### O2 — scan the spec worktree's `reviews/` when run elsewhere → **moot**

The cross-worktree scan existed to make F2 stricter outside the spec worktree. Registrations are now advisory, and AC-11 (x-w) pins that both checkouts write the same binding and tuple. Bead 2 scans `workspace.FindRoot(cwd)` and its spec dir and names both roots in the advisory output.

### O1 — `git cat-file -e` exits 128 for a garbage-collected object and for a git failure → **typed probe not needed**

The typed probe `gitutil.CommitObjectState` existed to separate F2's "absent, so unlanded, refuse" from F5's "read error, refuse". After the amendment no check probes a reviewed SHA: F1 and F4 work on scan-found merges, F3 compares a branch tip by equality, and R4.3's set is string equality. The only remaining git operand is the advisory ancestry probe (`merge-base --is-ancestor`); exit 1 prints "outside", and any other outcome, including a missing object, prints `(not determinable locally)` and refuses nothing. Bead 2 does not add `CommitObjectState`.

### F2 — AC-1's fifth fixture is listed above the fourth → **precision-only**

An ordering note only, unrelated to the retired check of the same label. Bead 5 implements and names them in numeric order:
- fourth: stale checkout, divergence notice (`TestNext_PlanDivergenceNotice`);
- fifth: blob absent, checkout used, probe failure refuses (`TestNext_PlanBlobAbsentUsesCheckout`, `TestNext_PlanProbeFailureRefuses`).

### R8.4 — review retention after finalize → **needs operator decision**

Spec OQ-11 lists the options: (A) accept the loss; (B) keep `dispositions.jsonl` tracked with a `**`-plus-negation pattern; (C) finalize copies `reviews/` into the main checkout before removing the worktree. Bead 8 implements R8.1–R8.3, which hold under every option, and writes no retention code.

## ADR Fitness

- **ADR-0023** — APPLIED (Bead 5): the R1.4 completion signal is the child-derived phase from bead statuses (`phase.DerivePhaseDetail`), never the emptiness of a ready query.
- **ADR-0025** — RESPECTED (Beads 6, 7): the R6 marker is bd metadata and IS exported into the tracked `.beads/issues.jsonl`. It stores the evidence file's path, sha256 and byte count, never its text.
- **ADR-0030** — APPLIED (all beads):
  - `internal/validate/readiness` imports neither `os/exec` nor `internal/gitutil`. Bead 4's membership reads go through `internal/bead`, and its landed-merge read through `internal/lifecycle`.
  - Bead 2's reachable-merge primitive and Bead 5's plan-blob probe live in `internal/gitutil`.
  - Bead 1's waist lives in `internal/bead`.
  - `TestBoundary*` in `internal/lint/boundary_test.go` stays green.
- **ADR-0035** — APPLIED: every new refusal ends with a `recovery:` line. New emitted commands are registered wherever the `internal/guard` recovery-convention lint requires.
- **ADR-0037** — RESPECTED: `internal/panel` is untouched. R4.3's classifier and R6.3 validate agent-writable data by shape only and claim no authentication (§8). Bead 8 changes only whether registrations are tracked; every gate keeps reading them from the filesystem.
- **ADR-0039** — RESPECTED (Bead 8): §4's co-located `reviews/` location and the gate scan are unchanged. Bead 8 ignores that location in `.gitignore` (a `core`-owned file) and through `mindspec init` (`internal/bootstrap`, `workflow`), and moves nothing.
- **ADR-0040** — APPLIED (Beads 5, 6): R1.4's rule sentence lands at L3 (`internal/instruct/templates/implement.md`). R6's coverage rule ratchets from L4 (`ms-bead-impl` prose) to L1 (`bead ready-check`), and the L4 copy is deleted.
- **ADR-0041** — AMENDED by a new appended § "Amendment (Spec 129)", following the Spec-125 precedent:
  - parts (i) (reachable-landing fallback is a finder, not an identifier) and (ii) (non-equal panel datum is non-corroborating) land with Bead 2;
  - part (iii) (scoped, fingerprint-bound fourth-verb override) lands with Bead 7;
  - each part comes with its text-pin test (`internal/validate/adr0041_spec129_amendment_test.go`).
- **ADR-0042** — APPLIED:
  - IDs render via `idrender.Bead`/`idrender.Spec`;
  - bd stderr (Bead 1's truncation error) and the evidence path (Beads 6, 7) render via `termsafe.Escape`;
  - recovery lines carry only idrender-gated IDs, closed-set signal IDs and literal placeholders (`<path-to-justification>`);
  - Bead 2 classifies every `reviewed_head_sha` before any git call, so only class (c) becomes a git operand, and every advisory line is termsafe-escaped.
- **ADR-0045** — CITED BY BEAD 6 ONLY, under its Status fence: Bead 6 moves the override-coverage rule out of the `ms-bead-impl` skill into `mindspec bead ready-check`, which is the one move ADR-0045 lets spec 129 cite. No other bead cites it as coverage.

No ADR is superseded. No new ADR is needed beyond ADR-0045, which has already landed.

## Testing Strategy

- **Hermetic by default.** bd is faked through the existing seams (`runBDFn`, `listJSONFn`, `fetchBeadRecordFn`, `findEpicForBeadFn`, `readiness.FakeBDStore` in `fixtures.go`) or a fake `bd` executable on `PATH` inside `t.TempDir()`. No test calls `t.Skip` when real bd is absent: spec 124's bd-less rule, which CI enforces.
- **The fake bd honours row caps.** For AC-1 and AC-7 it applies bd 1.1.0's measured default cap: 100 rows for `ready` unless `-n 0`/`--limit 0` is in argv. Its `--parent` is edge-only. Ready rows carry `parent`/`metadata` only when set (omit-when-empty).
- **Real git where git is the subject.** Beads 2 and 4 extend `initLandedRepo`/`mergeBead` (`internal/lifecycle/landed_test.go`) and `initGitRepo`/`mergeDependency` (`internal/validate/readiness/fixtures.go`). Fixture repos set author/committer identity through env or `-c` flags, so git-identity-less CI passes. No fixture reads `~/.gitconfig`.
- **Argument-recording seams** pin every bd and git argv that a requirement names (`-n 0`, no `--parent` on the scoped ready read, SHA classes never reaching git).
- **Red-on-revert.** Each AC names its legs that fail today. Each implementer runs those legs against the pre-change code once (for example with `git stash`-free scratch copies, or by reasoning from the named red behaviour) and records the result in the commit body as "red-on-revert: run" or "red-on-revert: by inspection".
- **Lint and format.** `go test ./internal/lint/...` covers the ADR-0030 boundary, the argv ratchet `TestArgvIDOperandGated` (every new git or bd exec site is classified in `ratchet_argv_table_test.go`) and Bead 1's `TestBDRowCapWaist`. `gofmt -l .` must print nothing; doc comments stay plain ASCII.
- **Package scope.** `internal/harness` and `internal/instruct` tests run with `-short` only. The whole suite gate is `go build ./... && go test ./... -short` (AC-19).

## Bead 1: bd row-cap waist, row-cap lint and linkage pins

**Steps**
1. In `internal/bead/bdcli.go`, make `ListJSON(args ...string)` the `list` waist and add `ReadyJSON(args ...string) ([]byte, error)` as the `ready` waist (R2.2):
   - both append `-n 0` unless the caller already passed exactly `"-n", "0"` or `"--limit", "0"`, which is tolerated and never duplicated;
   - any other `-n`/`--limit` value (including `--limit=5` and `-n5` spellings) returns an error before bd is spawned;
   - `ReadyJSON` captures stderr separately on success (a dedicated exec path inside `internal/bead`; `tracedOutput`'s `cmd.Output()` discards it). If stderr contains bd's `Showing <N> of <M>` notice, it returns `bd truncated the ready set (<termsafe.Escape(notice)>) although mindspec requested no cap` with an ADR-0035 `recovery:` line (for example `recovery: bd ready -n 0 --json` to inspect).
2. Route every R2.1 site through the waist:
   - `next.QueryReady` and `next.QueryReadyForEpic` (`internal/next/beads.go`) use `bead.ReadyJSON`;
   - `next.ResolveActiveBead`'s `listJSONFn("--parent", epicID, "--status=in_progress")` gets the waist's `-n 0`;
   - `lifecycle`'s `listClosedBeadsFn` (`orphans.go`) and `listOpenBeadsFn` (`stale_open.go`);
   - `bead/hygiene.go`'s two `ListJSON("--status=open")` calls (`AuditWorkset`, `FixHygiene`);
   - `complete.go`'s post-close `runBDFn("ready", "--parent", epicID, "--json")` moves to the `ReadyJSON` waist through a seam;
   - `approve/adopt_lattice.go`'s `adoptListBDFn("list", …, "-n", "0", …)` migrates to `bead.ListJSON`.

   Keep each site's existing func-var seam so tests can record argv.
3. Add `internal/lint/rowcap_test.go` with `TestBDRowCapWaist` (R2.3):
   - It walks non-test Go sources and flags a call whose callee is `bead.RunBD`, `bead.RunBDCombined`, `exec.Command` with first argument `"bd"`, or a package-level func var initialised to one of those, when its first bd argument is the literal `"ready"` or `"list"`.
   - It exempts `internal/bead` and any call carrying a `// rowcap-allowlisted: <reason>` comment on the call line or the line above.
   - Add that marker to `internal/doctor/beads.go`'s schema-drift probe (rows unread).
   - A self-test feeds an injected non-test source from `internal/lint/testdata/` containing an unmarked `bead.RunBD("list", …)` and asserts red. It also asserts green for the doctor marker, `internal/redact/redact.go`'s `"list"` map key and `internal/harness`'s `sandbox.runBD(...)` method calls.
4. Decide R2.4 per the plan-level choice: raise `bdVersionFloor` to `"1.1.0"` with a doc comment citing spec 129 Background B and C, unless all three facts are shown at 1.0.4 from bd's 1.0.4 source or changelog (then keep `1.0.4` and cite that evidence). Update any doctor test that pins the floor string.
5. Add the R3.2 pins:
   - `TestPlanApprove_BeadsCarrySpecID` in `internal/approve/plan_test.go`: every `planRunBDFn`-recorded `bd create` argv for a bead carries `--metadata` JSON whose `spec_id` equals the spec ID and passes `idvalidate.SpecID`;
   - `TestSpecApprove_EpicHasNoSpecID` in `internal/approve/spec_test.go`: `specRunBDFn`'s epic argv carries no `spec_id`.

   No production change.
6. Add the AC-7 seam tests and the end-to-end cap test: `TestQueryReady_Uncapped`, `TestResolveActiveBead_Uncapped`, `TestOrphans_ListUncapped`, `TestStaleOpen_ListUncapped`, `TestHygiene_ListUncapped`, `TestComplete_PostCloseReady_Uncapped`, `TestAdoptLattice_ListViaWaist`, `TestListJSON_ToleratesExplicitNZero`, `TestListJSON_RejectsCap`, `TestReadyJSON_TruncationNoticeIsError`, and `TestNext_101ReadyBeads_AllVisible`. The last test drives `next --emit-only` against the capping fake with 101 ready rows: 101 items listed, and `--pick 101` emits the last.

**Verification**
- [ ] `go test ./internal/bead/... -run 'TestListJSON_|TestReadyJSON_|TestHygiene_ListUncapped'` passes
- [ ] `go test ./internal/next/... -run 'TestQueryReady_Uncapped|TestResolveActiveBead_Uncapped|TestNext_101ReadyBeads_AllVisible'` passes (place the 101-row test in `cmd/mindspec` instead if `--emit-only` is only reachable there; then run it with `go test ./cmd/mindspec/... -run TestNext_101ReadyBeads_AllVisible`)
- [ ] `go test ./internal/lifecycle/... -run 'TestOrphans_ListUncapped|TestStaleOpen_ListUncapped'` and `go test ./internal/complete/... -run TestComplete_PostCloseReady_Uncapped` pass
- [ ] `go test ./internal/approve/... -run 'TestAdoptLattice_ListViaWaist|TestPlanApprove_BeadsCarrySpecID|TestSpecApprove_EpicHasNoSpecID'` passes
- [ ] `go test ./internal/lint/...` passes, including `TestBDRowCapWaist` and its injected-file self-test
- [ ] `go test ./internal/phase/... ./internal/doctor/...` passes unchanged except any floor-string pin, which proves `phase.Cache`'s explicit `-n 0` is tolerated
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-7: every R2.1 site's recorded argv carries exactly one `-n 0`; `-n 5` errors with no bd spawn; with 101 ready rows the emit-only list has 101 items (red-on-revert: 100 today).
- AC-8: `TestBDRowCapWaist` is red on the injected unmarked `bead.RunBD("list", …)` and green on the doctor marker, the redact map key and harness `sandbox.runBD`. A stderr `Showing 100 of 106 ready issues…` at exit 0 makes `ReadyJSON` return the escaped truncation error with a `recovery:` line.
- AC-9: both linkage pins pass with no production change.
- R2.4: the floor decision is made in code with its evidence comment.

**Depends on**
None.

## Bead 2: Landed-merge truth in internal/lifecycle (reachable fallback, panel-SHA set, audited reattest)

**Steps**
1. In `internal/gitutil/gitops.go`, beside `FirstParentMerges`, add one primitive:
   - **`ReachableMerges(workdir, ref string) ([]MergeCommit, error)`**: `git log --merges --topo-order` over `ref`, newest first. Each entry carries the SHA, both parents and the subject, in the same shape `FirstParentMerges` returns. Use `RejectOptionLike` on `ref`.

   Update the `FirstParentMerges` doc contract to name its first-parent-only scope and point to `ReachableMerges`. Classify the new exec site in `internal/lint/ratchet_argv_table_test.go`. No `CommitObjectState` probe is added (round-7 resolutions, O1).
2. Add `internal/lifecycle/fallback.go`:
   - **The `reviewed_head_sha` value classifier** (R4.3's argv hygiene). One ordered table:
     - (a) empty → skip;
     - (d) option-like or revision-expression → malformed, non-corroborating;
     - (c) `wellFormedGitObjectID` → set member;
     - (b) everything else → placeholder, skip.

     Pinned by `TestReviewedSHAClass_Table`. Classification happens before any git call, and no class refuses.
   - **The fallback subject matcher.** It admits only two-parent merges whose subject names `bead/<id>` as SOURCE in one of these forms:
     - `Merge bead/<id>`;
     - `Merge branch 'bead/<id>' into <spec-branch>`;
     - `Merge bead/<id> into <spec-branch>`.

     The last two are admitted only when `<target>` equals this spec branch. It excludes a merge INTO `bead/<id>`, any `into <other>` form and unrecognised formats. `parseMergeSubjectBeadBranch`'s G2-2 any-token fallback is not used here.
   - **The checks F1, F3, F4 and F5** (labels kept; F2 is retired), each returning a typed refusal:
     - F1: exactly one admitted candidate, else the new `ReattestStateMultipleFallbackCandidates` (`"multiple-fallback-candidates"`).
     - F3: the Spec-125 surviving-tip contradiction.
     - F4: the Spec-125 `R5(d)` net-effect/revert test anchored on the single candidate at the current tip.
     - F5: any read error is an error.
   - **The advisory registration report** (reattest only, never read-back). Over every registration `panel.ForBead` returns in `workspace.FindRoot(cwd)` and its spec dir: for each class-(c) SHA, `IsAncestor` against the tip; exit 1 → `advisory: registration <dir> (round <r>) reviewed <sha>, which is outside <spec-branch>'s tip history`; any other non-zero outcome → the same line with `(not determinable locally)`. A class-(d) value → `advisory: registration <dir> carries a malformed reviewed_head_sha; ignored`. Then once `advisory: panel roots scanned: <root>, <spec-dir>`. Every value is termsafe-escaped. It returns lines and a count, never an error that refuses.
3. In `internal/lifecycle/landed.go`, make three changes:
   - **R4.3.** Replace `reviewedHeadSHAForBead`'s first-non-empty return with a set of class-(c) SHAs across all registrations. In `FindLandedMerge`, the set CONFIRMS iff it contains the candidate's exact second parent; otherwise it is non-corroborating and never pre-empts the tip (b) or binding (d) legs. A (b) or (d) exact contradiction still contradicts.
   - **R4.2.** Add `FindLandedMergeReachable(root, specBranch, beadID string) (*LandedMerge, error)`. It runs the unchanged first-parent scan first, and runs the `ReachableMerges` fallback only when that scan nominates zero owned two-parent candidates. On the fallback it identifies a merge ONLY through the read-back:
     - a VALID audit (`op == reattestOp`, `at` RFC-3339, non-empty `actor`, `scanned_branch` equal to this spec branch, both SHA keys well-formed);
     - audit tuple == current binding pair == the single scan-derived candidate;
     - F1, F3, F4 and F5 pass now; no registration is read.

     Otherwise it returns the new `*LandedMergeOffFirstParent`, which wraps `ErrLandedMergeNotFound` (never `ErrLandedMergeNoCandidate`) and names the bead, the spec branch, each admitted candidate's merge SHA and second parent, and `mindspec reattest <id>`.
   - **Unchanged.** `FindLandedMerge` itself, and therefore `MergedUnclosed`, `orphan_hints.go`, `planFindLandedMergeFn`, `adoptFindLandedMergeFn` and the executor paths, keep the first-parent scan only. Update the `ErrLandedMergeNoCandidate` and `SubsumptionConflict` doc contracts per R4.2/R4.4.
4. In `internal/lifecycle/reattest.go`:
   - Nominate through the same first-parent-then-fallback generator.
   - On a fallback candidate, run every Spec-125 check, then F1, F3, F4 and F5. Print `candidate <sha> reached <spec-branch>'s tip off the first-parent chain; its subject names no target` and the advisory lines before writing. No advisory changes the exit code or the writes.
   - Write the binding plus `mindspec_landed_reattest_merge_sha`, `mindspec_landed_reattest_second_parent` and `mindspec_landed_reattest_scanned_branch` in ONE `reattestBindingFn` (`bead.MergeMetadata`) call. On a fallback, the `mindspec_landed_reattest_corroboration` value is `off-first-parent fallback; F1, F3, F4, F5 passed; advisory registrations outside tip: <n>; panel roots scanned: <roots>`.
   - Convergent no-op ONLY when the stored audit is VALID and its tuple equals both the binding and the candidate; otherwise re-run F1, F3, F4 and F5 and write a VALID record.
   - Datum (c) uses the R4.3 set: non-corroborating values are recorded in the corroboration string and never refuse. `ReattestStatePanelContradiction` is no longer produced by a non-matching set.
   - A refusal writes nothing.
   - Surface the new states and lines in `cmd/mindspec/reattest.go`.
5. Append § "Amendment (Spec 129)" to `.mindspec/adr/ADR-0041-gate-before-mutate.md` with parts (i) and (ii), covering every AC-18 element they own:
   - the fallback-scoped callers and the finder-only rule;
   - F1, F3, F4 and F5 (all git-derived), and F2's retirement with registrations as advisory output only;
   - the audited tuple, its validity contract and the validity-qualified no-op, and the fallback-provenance corroboration string;
   - q9ea's exclusion from the fallback exits;
   - the metadata-forge residual with its bounded new reach, and the read-back conditions;
   - the undirected-subject target gap;
   - the non-corroborating panel datum and its reach into the CLOSE paths, using the OQ-3 wording chosen above;
   - binding veto versus audited repair.

   Cross-reference it from item (c). Add `internal/validate/adr0041_spec129_amendment_test.go` pinning those phrases (part (iii) is added by Bead 7).
6. Write the lifecycle tests named in AC-11 and in AC-12's lifecycle legs, reusing and extending `initLandedRepo`/`mergeBead`/`mergeBeadDefaultSubject`. Write panel registrations as separate dirs with explicit `round`.
   - **Rewrite to the non-corroborating outcome:** `TestFindLandedMerge_ReviewedHeadSHAContradicts`, `TestReattest_PanelShaContradictionRefuses` and the two `…HostilePanelSHAEscapedInRefusal` tests. Keep the escaping assertion where a SHA is still rendered.
   - **Must stay green unchanged:** `TestReattest_ContradictoryBindingOverwrittenAudited`, `TestReattest_StaleOlderRemergeBindingRewrittenToNewest` and `TestReattest_HappyPathFleetState`'s `Wrote=false` leg.
   - **AC-21's lifecycle leg:** `TestReviewedHeadSHASet_UntrackedIgnored` writes the registration under a `.gitignore`d `reviews/` dir, never `git add`s it, and asserts the set contains its SHA.

**Verification**
- [ ] `go test ./internal/gitutil/... -run TestReachableMerges` passes
- [ ] `go test ./internal/lifecycle/ -run 'TestFindLandedMergeReachable|TestReattest_Fallback|TestReadBack_|TestMergedUnclosed_IgnoresFallback|TestFindLandedMerge_PanelSet|TestReattest_PanelSet|TestMergedUnclosed_PanelSet|TestReattest_RepairsContradictoryBindingOnFallback|TestReviewedSHAClass_Table|TestReviewedHeadSHASet_UntrackedIgnored'` passes (the spec's Validation Proof command, widened)
- [ ] `go test ./internal/lifecycle/... ./cmd/mindspec/... -run 'TestReattest'` passes, with `TestReattest_HappyPathFleetState`, `TestReattest_ContradictoryBindingOverwrittenAudited` and `TestReattest_StaleOlderRemergeBindingRewrittenToNewest` unchanged
- [ ] `go test ./internal/validate/... -run TestADR0041_Spec129Amendment` passes
- [ ] `go test ./internal/lint/...` passes (the argv ratchet classifies the new exec site)
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-11 lifecycle legs:
  - acceptance legs i, i-r, i-m, i-a, i-c, ii, iii, iii-neg, iv, v, vi;
  - refusal legs ix and x;
  - advisory legs vii, vii-a, ix-f, x-m, x-e, x-p, x-g, x-w;
  - read-back legs xi–xvi.

  Each passes as `TestFindLandedMergeReachable_*`, `TestReattest_Fallback*`, `TestReadBack_*` and `TestMergedUnclosed_IgnoresFallback`. Every advisory leg asserts the same exit code and writes as the fixture without the registration. A garbage-collected SHA prints `(not determinable locally)`. A refusal writes nothing (seam on `reattestBindingFn`).
- AC-12 lifecycle legs i, ii, iii, vii and viii pass (`TestFindLandedMerge_PanelSet*`, `TestReattest_PanelSetNonCorroborating`, `TestMergedUnclosed_PanelSet`, `TestReattest_RepairsContradictoryBindingOnFallback`). Leg v's reattest half passes with a corroboration string recording the panel as non-corroborating.
- AC-18 (ADR-0041 parts i–ii): the amendment text pin passes.
- AC-21 (lifecycle leg): `TestReviewedHeadSHASet_UntrackedIgnored` passes.

**Depends on**
None.

## Bead 3: Readiness foundation (plan-bead helpers, MF-2 form symmetry, failure records)

**Steps**
1. Add `internal/bead/planbead.go` with `PlanBeadIndex(epicID, id string) (n int, ok bool)`:
   - `ok` iff `id` is exactly `epicID + "." + <N>`, where N is one decimal segment with N ≥ 1 and no leading zero, and there are no further dots.
   - Table-test it in `planbead_test.go`, including `<E>.0`, `<E>.01`, `<E>.1.2`, `<other>.1` and an `<E>`-prefix collision such as `<E>x.1`.
2. In `internal/validate/readiness/readiness.go`, add `PlanMapsBead(planContent string, n int) (mapped bool, why string)`:
   - N is mapped iff `extractBeadSectionRaw(planContent, n)` finds the Nth `## Bead ` heading by POSITIONAL count, counting exactly as it does today (every line beginning `## Bead `, including inside a fenced block), AND the parsed `work_chunks` carries id N;
   - `why` is a stable phrase such as `plan.md declares 7 ## Bead sections`, `no work_chunks id 8` or `no ## Bead section 10`.

   Refactor `evaluateMF1` to call it, keeping its remaining content checks (concrete AC entry, `key_file_paths`) and existing Detail texts. Add `TestPlanMapsBead_AndPositional`: headings 1–7 plus 12, chunk ids 1–8 plus 10, and a fenced `## Bead ` line, asserting the same count `extractBeadSectionRaw` gives.
3. Implement R5 with one shared form table used by both `harvestClaims`/`extractLineClaims` (bead side) and `extractSpecTokens` (spec side). The table covers:
   - appended sub-letter (`R9a`, `AC-2b`);
   - parenthetical sub-letter (`R9(a)`, `AC-2(b)`);
   - Roman-enumerator parenthetical degrading to base (`R5(ii)`);
   - bare base;
   - hyphenless `AC9`, normalised to `AC-9` and harvested as a claim.

   Dotted sub-numbers keep resolving at base on both sides; a comment on the table says so (OQ-6). Add `TestMF2_FormSymmetry` (every form through both sides; each harvested key is producible by the indexer from the same text). `TestReadiness_MF2AllClasses` gains the six R5.2 legs.
4. Add `Failures []string` to `readiness.Signal` in `report.go`, and add `internal/validate/readiness/records.go`:
   - **Record grammars** (R6.2), raw and pre-render, with no free-form error text, timestamps or bd stderr:
     - MF-1: one stable key per existing `evaluateMF1` failure branch (for example `section-missing:<n>`, `ac-entry-missing:<n>`, `chunk-missing:<n>`, `key-file-paths-empty:<n>`, `key-file-paths-placeholder:<n>`, `key-file-paths-blank:<n>`);
     - MF-2: `dangling:<normalised-token>`;
     - MF-4: `<kind>:<source>:<section>:<k>:<sha256 of the normalised line>`, where kind is `token`|`checklist`, source is `description`|`plan`, section is the plan heading path or `-`, and k is the 1-based occurrence index among identical tuples;
     - MF-3, as constructors only (Bead 4 emits them): `<dep-id>:<class>[:<candidate-sha>]`, with the class constants `indeterminate`, `not-closed`, `no-candidate`, `off-first-parent`, `no-evidence`, `contradiction` and `reverted`, plus `IsIndeterminateRecord(rec string) bool`.
   - **`FailureRecordDigests(s Signal) []string`**: the sorted lowercase-hex sha256 of each record. It is the ONE function both writer and reader use.
5. Make `evaluateMF1`, `evaluateMF2` and `evaluateMF4` aggregate ALL failures into sorted, unique `Failures`, instead of returning on the first.
   - `Detail` stays human text and may list every failure.
   - `Recovery` keeps spec 124 R1's one line per signal.
   - `findBlockingToken`/`findBlockingChecklistItem` become all-occurrence scanners feeding MF-4 records.
6. Give `readiness.Report` the lineage `EvaluateReadiness` already resolves: `EpicID` and `SpecID` fields filled from `findEpicForBeadFn`. This is the R1.8 groundwork Bead 7 consumes; no caller changes here. Add `TestFailureRecordDigests_Deterministic` (two runs on an unchanged fixture are byte-identical for every signal) and `TestFailureRecordDigests_WriterReaderAgree` (an MF-4 line containing a control byte gives equal digests from the raw records on both paths).

**Verification**
- [ ] `go test ./internal/bead/... -run TestPlanBeadIndex` passes
- [ ] `go test ./internal/validate/readiness/ -run 'TestPlanMapsBead_AndPositional|TestMF2_FormSymmetry|TestReadiness_MF2AllClasses|TestFailureRecordDigests_|TestReadiness_MF1|TestReadiness_MF4'` passes
- [ ] `go test ./internal/complete/... -run Isolation` passes (readiness still never reads the override marker)
- [ ] `go test ./internal/lint/...` passes (no new `os/exec`/`gitutil` import in `internal/validate/readiness`)
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-14: `TestMF2_FormSymmetry` passes over the full table; `TestReadiness_MF2AllClasses`' six R5.2 legs pass; legs 1, 4 and 6 are recorded red-on-revert.
- AC-1 (helper part): `TestPlanMapsBead_AndPositional` passes, including the positional heading-12 mapping and the fenced-heading count.
- AC-16 (digest part): `TestFailureRecordDigests_Deterministic` and `TestFailureRecordDigests_WriterReaderAgree` pass.
- `bead.PlanBeadIndex`, `readiness.PlanMapsBead`, `Signal.Failures`, `FailureRecordDigests`, the MF-3 record constructors and `Report.EpicID`/`SpecID` exist with doc comments, for Beads 4–7 (R1.1, R6.2).

**Depends on**
None.

## Bead 4: MF-3 membership, message arms and aggregated failures

**Steps**
1. Extend `fetchBeadRecordReal` and `dependencyEdge` to keep each `blocks` dependency's `metadata.spec_id`, which the bead's own `bd show --json` already returns. Add a seam `fetchDependencyParentFn` that runs one `bd show <D> --json` through `internal/bead` and returns D's `parent`. Never read the dependency entry's own `parent` key: it is null even when D has a parent (Background C).
2. Implement R4.1 membership in `evaluateMF3`, in a new `internal/validate/readiness/mf3.go`. A closed `blocks` dependency D is held to the landed-merge test iff any of these holds:
   - (i) `bead.PlanBeadIndex(report.EpicID, D)`;
   - (ii) its `spec_id` equals S;
   - (iii) its parent read equals E, run only when (i) and (ii) are both negative.

   Outcomes:
   - All negative from successful reads → PASS with `dependency <D>: closed; not a bead of spec <S> — landed-merge test not applicable`.
   - Any read or parse error → an `indeterminate` record with `could not determine whether <D> belongs to spec <S>: <escaped error>` and `recovery: bd show <D> --json`.
   - Held only by (ii) or (iii): add the "linked to spec <S> by <metadata.spec_id | parent edge> but does not have the plan-bead form" hint plus the scoped override line.
   - Plan-form but unmapped (`PlanMapsBead` false): add the follow-up hint.
   - A non-closed dependency fails as `not-closed`, as today.
3. Switch `findLandedMergeFn` to `lifecycle.FindLandedMergeReachable`. Discriminate errors in R4.4's order:
   - `*LandedMergeNoEvidence` → unchanged, recovery `mindspec reattest <D>`;
   - `*LandedMergeOffFirstParent` → arm 1b's sentence, with `Recovery` = `mindspec reattest <D>`. The second exit, `recovery (if reattest refuses): mindspec next <this bead> --allow-not-ready=MF-3 --evidence <path-to-justification>`, goes on a Detail line. No q9ea line;
   - `errors.Is(err, lifecycle.ErrLandedMergeNoCandidate)` → `no two-parent merge reachable from <spec-branch>'s tip lands bead/<D> (first-parent scan, then the reachable-landing fallback)` plus today's two-possibility wording;
   - a corroboration contradiction → `a merge landing bead/<D> exists (<sha>) but <datum> contradicts its second parent <sha>`;
   - a revert shape → `merge <sha> landing bead/<D> was reverted or cleanly removed after landing`.

   Each arm ends with its own inspection-first recovery. None recommends `mindspec complete` for a bead whose merge exists. Distinguish contradiction from revert by typed errors or sentinels exported from `internal/lifecycle`, not by string matching. If Bead 2 did not export them, add `ErrLandedMergeContradiction`/`ErrLandedMergeReverted` wrappers in `landed.go` that keep the `ErrLandedMergeNotFound` chain.
4. Aggregate MF-3 across EVERY closed member dependency, one landed-merge evaluation each:
   - `Failures` holds one record per failing dependency, built with Bead 3's constructors;
   - `Detail` lists every failing dependency with its class and its own recovery text;
   - `Recovery` is the highest-precedence record's line, in the order `indeterminate` > `not-closed` > `no-candidate` > `off-first-parent` > `no-evidence` > `contradiction` > `reverted`, and by dependency ID within a class.
5. Extend `registerLandedPanel` in `fixtures.go` with a slug parameter and update every caller. Add the MF-3 tests:
   - `TestReadiness_MF3Variants` legs a–g;
   - `TestReadiness_MF3_OffFirstParentFailsWithExits` (AC-11 leg i end to end: arm 1b, then PASS after a real `lifecycle.ReattestLandedMerge` on the same fixture);
   - `TestReadiness_MF3_CompleteWrapperShape` (AC-12 leg iv);
   - AC-12 leg v's MF-3 half;
   - `TestReadiness_MF3_LocalAheadOfRemotePasses` (leg vi, labelled green today);
   - `TestReadiness_MF3ErrorArmDiscrimination`'s four new legs;
   - `TestReadiness_MF3AggregatedPrecedence`.
6. Make no change to `next`, `cmd` or the override marker in this bead.

**Verification**
- [ ] `go test ./internal/validate/readiness/ -run 'TestReadiness_MF3|TestMF2_FormSymmetry'` passes (the spec's Validation Proof command)
- [ ] `go test ./internal/validate/readiness/... ./internal/next/... ./cmd/mindspec/...` passes (callers of `registerLandedPanel` and `EvaluateReadiness` stay green)
- [ ] `go test ./internal/complete/... -run Isolation` passes
- [ ] `go test ./internal/lint/...` passes (no `os/exec`/`gitutil` import added to `internal/validate/readiness`)
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-10: legs a–g pass; a, b and f are recorded red-on-revert; leg g is a FAIL "could not determine whether…", never PASS.
- AC-11 (MF-3 legs): leg i's MF-3 FAILS with arm 1b, never the zero-candidate sentence, and PASSES after reattest via read-back.
- AC-12 (MF-3 legs iv, v, vi): iv passes via the binding; v reports `*LandedMergeNoEvidence` naming the candidate, then PASSES after reattest; vi passes.
- AC-13: the four arm legs and the aggregated-precedence test pass; a two-class failure shows both records and exactly one `recovery:` line (`mindspec complete A`).
- R4.5 (#232 shape) is closed by AC-12 leg iv/v as above, with no `origin/` ref read.

**Depends on**
Bead 2 (`FindLandedMergeReachable`, `*LandedMergeOffFirstParent`, the R4.3 set) and Bead 3 (`PlanBeadIndex`, `PlanMapsBead`, `Report.EpicID`/`SpecID`, the MF-3 record constructors).

## Bead 5: Spec-scoped selection honesty in mindspec next

**Steps**
1. Add `internal/gitutil/fileprobe.go` with `FileAtRefTyped(workdir, ref, path string) ([]byte, error)` and the sentinels `ErrRefAbsent` and `ErrPathNotAtRef`:
   - `git rev-parse --verify --quiet refs/heads/<ref>`: exit 1 with no output → `ErrRefAbsent`;
   - then `git ls-tree <ref> -- <path>`: empty output → `ErrPathNotAtRef`; non-zero exit → an error;
   - then the blob read.

   Any other failure is a plain error. Classify the exec sites in the argv ratchet. Add `internal/next/planread.go`:
   - it reads `plan.md` for spec S LAZILY: from `spec/<S>` via the probe, falling back to `workspace.SpecDir(root, S)/plan.md` only on `ErrRefAbsent`/`ErrPathNotAtRef`;
   - absent in both places → "no plan" (every plan-form row becomes N4 `no plan.md exists yet for <S>`);
   - any other error refuses with `could not read plan.md for <S>`;
   - when both exist and differ, print once `plan.md in this checkout differs from spec/<S>; mapping used: spec/<S>`.
2. Rewrite `next.BuildReadySet`'s scoped branch over ONE `bead.ReadyJSON` global read (no `--parent`, no title filter):
   - a row is a **member** iff it is not an epic, is open, `bead.PlanBeadIndex(E, id)` holds, `phase.FindEpicForBead(id)` returns E (via a `findEpicForBeadFn` seam, with `phase.Cache` for the epic list), and `readiness.PlanMapsBead` maps N;
   - members keep bd's order;
   - a lineage read error refuses with `could not resolve the lineage of <id>`.

   Keep `scopeResolutionRefusal` and `emptyAfterFilterError` as the mechanisms, now over this predicate (settled fact 1).
3. Add `internal/next/nearmiss.go`, replacing `exclusionReasonForName`. It classifies every non-member, non-epic row linked to S by plan-form ID, row `parent` E, title (`containsSpecID`, now only this link test) or `metadata.spec_id` == S.
   - **Order:** N2a, N2c, N4 (any unmapped N, whatever the title lineage), N2b, N1, N3. Each class uses R1.2's exact message and recovery lines.
   - **N2b** splits on whether E2's spec number equals S's: same → `bd update <id> --parent <E>` first; different → `bd show <id> --json` first, then `recovery (only if <id> belongs to <S>): bd update <id> --parent <E>`.
   - **N4 with no lineage** appends `; it also has no lineage, so readiness cannot evaluate it at all`.
   - **Blocking:** N1, N2a, N2b, N2c, and an N3 whose row `parent` is E and whose type is lifecycle (`phase.IsLifecycleIssueType`). Everything else is informational.
   - **Rendering:** every ID via `idrender.Bead`; `ReadySet.ExclusionNotice` names IDs, never a bare count (R1.5).
4. In `cmd/mindspec/next.go`, on the scoped claim path, consult the selector BEFORE the empty-set branch (R1.3):
   - empty members plus positional → `no ready plan bead for spec <S>; positional <id> cannot be claimed from an empty ready set`;
   - `--pick N` → `--pick <N> is out of range: 0 items`;
   - both end with `recovery: mindspec next --spec <S>`.

   Positional handling:
   - a near-miss positional refuses with its R1.2 sentence on empty AND non-empty sets, except an N4 with lineage E, which passes to `GateReadiness`;
   - an N4 positional with no lineage refuses at selection with `bead <id> has no lineage, so readiness cannot evaluate it; no readiness override applies` and `recovery: bd show <id> --json`, never calling `GateReadiness` (seam);
   - a globally ready but unlinked positional → `bead <id> is bd-ready but is not linked to spec <S>`, plus `(parent: <P>)` when the row carries one;
   - a positional absent from the ready set keeps "not found or not ready".

   `--emit-only`'s `FetchBeadByID` bypass and its disclosure line are unchanged.
5. Replace the empty-member, no-selector output (R1.4). Read E's child list (uncapped, all statuses, as `phase.Cache` lists it) and print, in order:
   - the lifecycle-children tally `closed / in_progress / open`;
   - `stored phase: <p>; child-derived phase: <q>` from `phase.DerivePhaseDetail`;
   - the verbatim sentence `an empty ready set is not evidence the spec is complete; the derived phase above is`;
   - one arm keyed on the tally:
     - all closed → `recovery: mindspec impl approve <S>`;
     - in-progress → today's `ResolveActiveBead`/`EnsureWorktree` recovery, after these lines;
     - open and none in progress → `<n> open lifecycle bead(s), none selectable (any that are bd-ready but unmapped or unlinked are named above)` plus `recovery: bd blocked --parent <E>`;
     - zero children → `recovery: mindspec plan approve <S>`.

   Add `recovery: mindspec repair phase <S>` when the child-derived phase is ahead of the stored phase. Exit codes:
   - 0 with no blocking near-miss;
   - non-zero with one or more blocking near-misses;
   - non-zero with the indeterminate form and no tally when the child read fails.

   Fix the ambiguity menu to print `mindspec next --spec=<spec-id>` (R1.6). Add the sentence once to `internal/instruct/templates/implement.md`, pinned by `TestImplementTemplate_EmptinessRule` in `internal/instruct`.
6. Rewrite the spec-128 tests named in R1.7 over the new predicate:
   - `TestBuildReadySet_ScopedFiltersByTitle_ExclusionAccounted`;
   - `TestBuildReadySet_EmptyAfterFilter_LoudError`;
   - `TestNextSelection_{ClaimPath,EmitOnlyPath}_PartialExclusionNoticePrinted` gain the ID assertion.

   Add the AC-1 to AC-5 tests. The AC-1 fixture includes both X legs (plan gap 3) and the fourth and fifth fixtures in numeric order. Keep Bead 1's `TestNext_101ReadyBeads_AllVisible` green.

**Verification**
- [ ] `go test ./internal/gitutil/... -run TestFileAtRefTyped` passes (ref absent, path absent, present, and a seam-injected git failure)
- [ ] `go test ./internal/next/... -run 'TestBuildReadySet_|TestNextSelection_|TestNearMiss|TestExclusionNotice_NamesIDs'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestNext_ScopedSelection_|TestNext_EmitOnlyReadsPlanFromSpecBranch|TestNext_PlanDivergenceNotice|TestNext_PlanBlobAbsentUsesCheckout|TestNext_PlanProbeFailureRefuses|TestNext_LazyPlanReadForNoLineageRow|TestNext_NoPlanYetAllN4|TestNext_SelectorOnEmptySet_Refuses|TestNext_NearMissPositional_Refuses|TestNext_UnlinkedReadyPositional_Refuses|TestNext_N4NoLineagePositionalRefuses|TestNext_EmptyScoped_|TestNextMenu_PrintsSpecIDForm|TestNext_101ReadyBeads_AllVisible'` passes (move any test that lives in `internal/next` there; the names are binding, the package is not)
- [ ] `go test ./internal/instruct/... -short -run TestImplementTemplate_EmptinessRule` passes
- [ ] `go test ./internal/lint/...` passes; `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-1: on the edge-only, capping fake:
  - members are exactly {A, G, G′} in bd order;
  - blocking near-misses are D (N1), P (N2a), Q (N2b same-number), Q2 (N2b other-number), X and X′ (N2c) and R (N3);
  - informational near-misses are O, H, L (N4) and F, M, T (N3);
  - the epic row is in neither list;
  - `--emit-only --pick 2` emits G;
  - no `bd ready --parent` argv is recorded.

  The second to fifth fixtures and the lazy-trigger and fenced-heading fixtures behave as AC-1 states. The red-on-revert legs are recorded.
- AC-2: the four near-miss recovery tests pass hermetically. After the fake applies `bd update <D> --parent <E>`, D is a member and `readiness.EvaluateReadiness` reaches its signals.
- AC-3 (all legs except the scoped-override claim of an N4 positional, which is Bead 7's): selector refusals end with `^recovery: mindspec next --spec `, and the no-lineage N4 positional never reaches `GateReadiness` (byte-identical audit).
- AC-4: fixtures i, ii, iii-a, iii-b, iv, v and vi, plus the failing-child-read leg, print the specified lines and exit codes. Fixture iv records zero plan reads.
- AC-5: `TestExclusionNotice_NamesIDs` and `TestNextMenu_PrintsSpecIDForm` pass, and the R1.7 rewritten tests pass.

**Depends on**
Bead 1 (`bead.ReadyJSON`, uncapped `ResolveActiveBead`) and Bead 3 (`bead.PlanBeadIndex`, `readiness.PlanMapsBead`).

## Bead 6: Validated override marker, ready-check read-back, and the skill rule move

**Steps**
1. In `internal/bead/readiness.go`, beside `MetaKeyReadinessOverride`, add the schema-v2 field-name constants. Add `internal/bead/override_marker.go` with `ParseOverrideMarker(raw any) (OverrideMarker, MarkerValidity)`, classifying VALID, LEGACY or MALFORMED exactly per R6.3:
   - **VALID:** `schema` 2; `signals` a non-empty, duplicate-free subset of `MF-1`…`MF-4`; one `fingerprints` entry per signal, each a non-empty, sorted, duplicate-free list of 64-hex digests; `overridden` RFC-3339; `evidence.sha256` 64-hex; `evidence.bytes` in [1, 65536]; `evidence.path` non-empty.
   - **LEGACY:** no `schema` key; only `signals` and `overridden`.
   - **MALFORMED:** anything else.

   Add a reader `ReadOverrideMarker(beadID string)` over `GetMetadata` that returns an error on a metadata read failure, never "no marker". Also add the writer-side builder `NewOverrideMarkerV2(signals, fingerprints, evidence, now)` that Bead 7 calls, so writer and validator share one schema definition. Test it in `override_marker_test.go` (`TestOverrideMarker_Validity`, covering every MALFORMED case AC-17 lists).
2. Add `internal/validate/readiness/override.go` with `ApplyOverride(report *Report, marker OverrideMarker, validity MarkerValidity) CoverageResult`. The marker is DATA passed in; `EvaluateReadiness` never reads it.
   - A failing signal X is covered iff the marker is VALID, X is in `signals`, every digest in `FailureRecordDigests` of X's current `Failures` is in `fingerprints[X]`, and no current record is indeterminate (`IsIndeterminateRecord`).
   - Growth uncovers; shrinkage stays covered.
   - Rendering:
     - covered → `MF-x: OVERRIDDEN (recorded <ts>; evidence <sha256-prefix> <termsafe-escaped path>)`;
     - recorded but passing → `MF-x: PASS (override recorded but no longer needed)`;
     - LEGACY → `override marker is legacy (no evidence) — not honoured`;
     - MALFORMED → `override marker is malformed — not honoured`.
3. In `cmd/mindspec/bead_ready.go`, read the marker through a seam after `readiness.EvaluateReadiness` and apply coverage.
   - Exit 0 iff every signal passes or every failing signal is covered. Otherwise exit non-zero, with `uncovered: <ids>`, then `recovery: mindspec release <bead-id>` and `recovery: mindspec next <bead-id> --allow-not-ready=<all currently failing ids> --evidence <path-to-justification>` on the LEGACY, MALFORMED and uncovered arms. The re-claim line names ALL currently failing signals, covered or not.
   - A metadata read error refuses non-zero.
   - Add the seam `readyCheckEvaluateFn` so the AC-17 tests drive MF-3 and MF-4 failure sets (including an `indeterminate` MF-3 record built with Bead 3's constructors) without a real tracker.
4. Move the rule (R6.5, ADR-0045). In `plugins/mindspec/skills/ms-bead-impl/SKILL.md`, replace the override-coverage paragraph (the "FAIL, WITH the durable override marker" bullet, which compares `mindspec_readiness_override` `signals`) with ONE line: run `mindspec bead ready-check <id>`; proceed on exit 0; otherwise stop and surface its report. Immediately before it, put the marker:

   `<!-- staged-rule: target=next-action (ADR-0045 step 4, spec 133); remove-by=2027-06-30 (provisional; the spec-130 rule table may revise it) -->`

   In `plugins/mindspec/skills/ms-bead-cycle/SKILL.md`, replace the bare `--allow-not-ready` wording with the scoped form `--allow-not-ready=<ids> --evidence <path>`. Use `grep -n 'allow-not-ready' plugins/mindspec/skills/*/SKILL.md` (run unpiped) to find every site.
5. Follow the skill-refresh protocol, as earlier specs did (`previouslyShippedSkills` in `internal/setup/skills.go` and the snapshots under `internal/setup/historical_skills/`):
   - add a byte-exact snapshot of each replaced plugin `SKILL.md` text so existing installs refresh;
   - edit no existing snapshot;
   - refresh `.claude/skills/ms-bead-impl/SKILL.md` from the plugin source, not by hand.

   Rewrite `TestMsBeadImplSkill_OverrideCoverageCheckShipped` and `TestMsBeadImplSkill_OverrideCoverageMutationProbe` in `internal/setup/readiness_skill_pin_test.go` to pin the new line and the staged-rule marker, and to assert the absence of the `mindspec_readiness_override` subset comparison.
6. Re-verify R7 for AC-18 (no edit): run `mindspec adr list` and confirm ADR-0045 is listed; confirm ADR-0045's `Status: Accepted`, its Status fence, Amends ADR-0040 §1 and ADR-0044, step 1 → spec 130 and Decision 5's transition clause; confirm the ADR-0040 and ADR-0044 back-pointers. Record the outputs in the commit body.

**Verification**
- [ ] `go test ./internal/bead/... -run TestOverrideMarker_Validity` passes
- [ ] `go test ./internal/validate/readiness/ -run TestApplyOverride` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestReadyCheck_HonoursValidMarkerOnly|TestReadyCheck_LegacyRecoveryReleasesThenReclaims|TestReadyCheck_MixedCoverageRecoveryNamesAllFailing'` passes
- [ ] `go test ./internal/setup/... -run 'TestMsBeadImplSkill_OverrideCoverage'` passes, and the skill-refresh tests in `internal/setup` stay green
- [ ] `go test ./internal/complete/... -run Isolation` passes
- [ ] `mindspec adr list` (run) lists ADR-0045
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-17 (coverage, render and exit legs):
  - a VALID matching marker → exit 0 with `MF-3: OVERRIDDEN (…)`;
  - MF-2 also failing → non-zero naming MF-2;
  - changed failing dependency → MF-3 uncovered;
  - now passing → the "no longer needed" note;
  - an `indeterminate` record → never covered;
  - LEGACY → the legacy note plus the two-line release/re-claim recovery;
  - mixed coverage → `uncovered: MF-3` with a re-claim naming `MF-2,MF-3`;
  - each MALFORMED case → the malformed note;
  - a metadata read error → non-zero.

  The first case is recorded red-on-revert.
- AC-18 (skill and ADR-0045 parts): `ms-bead-impl/SKILL.md` has no `mindspec_readiness_override` subset comparison, and has the `ready-check` exit-code line immediately preceded by the staged-rule marker. The rewritten pins pass. ADR-0045 and its back-pointers are verified present.
- R6.5's move is complete: the coverage rule lives in `bead ready-check` (L1), and the L4 copy is deleted.

**Depends on**
Bead 3 (`Signal.Failures`, `FailureRecordDigests`, `IsIndeterminateRecord`, the MF-3 record constructors).

## Bead 7: Claim guard, scoped allow-not-ready with evidence, and the flag-wording sweep

**Steps**
1. Change `next.GateReadiness` to `GateReadiness(root, beadID string, opts GateOptions) (*GateResult, error)`, where `GateOptions` carries the scope spec (from `--spec`, or `resolve.ResolveTarget` on the unscoped path), the named signals, and whether the flag was given. Before any mutation, after `evaluateReadinessFn`, check R1.8 in order, using `report.EpicID`/`SpecID` (Bead 3) as E_T and T:
   - (a) `bead.PlanBeadIndex(E_T, id)` must hold, else `bead <id> is not a plan bead of spec <T> (no <E_T>.<N> ID); mindspec next claims only plan beads` with `recovery: bd show <id> --json`;
   - (b) a scope spec S ≠ T refuses with `bead <id> resolves to spec <T> by lineage but is being claimed for spec <S>; nothing was claimed`;
   - (d) `spec/<T>` must resolve through `gitutil.RevParseRef` (called from `internal/next` or `cmd`, never `internal/validate`), else `spec branch spec/<T> for bead <id> does not exist (finalized and deleted, or never created); nothing was claimed` with `recovery: mindspec state show`.

   `GateResult` returns T as the dispatch spec.
2. In `cmd/mindspec/next.go`, apply (c): pass T to `next.EnsureWorktree` (and so to `DispatchBead`) and to recording, never an empty spec, so the bead branch is cut from `spec/<T>` and never from `HEAD`. Correct `advancePhaseOnClaim`'s comment about non-lifecycle claims: a bug bead with no epic lineage is refused by `EvaluateReadiness`'s lineage error.
3. Re-register `--allow-not-ready` as a String flag with a `NoOptDefVal` sentinel, and add `--evidence`. Parse and refuse before positional resolution:
   - the bare form, and the space-separated `--allow-not-ready MF-3` that cobra parses as bare plus positional: `MF-3` is never looked up as a bead;
   - any ID outside `MF-1`…`MF-4`;
   - `all`.

   In `GateReadiness`, implement R6.1:
   - **Proceed** iff the failing set is a subset of the named set.
   - **Named but passing:** print `MF-x named but passing — not recorded`.
   - **Nothing failing:** a normal claim, with no marker and no `--evidence` required.
   - **A non-named failing signal:** the NOT-READY report plus `recovery: mindspec next <bead-id> --allow-not-ready=<all currently failing ids> --evidence <path-to-justification>`. The bead ID is idrender-gated, the IDs are closed-set and the placeholder is literal.
   - **A named failing signal with an indeterminate record:** `<MF-x> cannot be overridden: <record> could not be evaluated; fix the read first`.
4. Add `internal/next/evidence.go`, read before any mutation:
   - `--evidence` must be a readable regular file of 1 byte to 64 KiB;
   - a missing `--evidence` when at least one signal would be recorded refuses;
   - the result carries the path, sha256 and byte count, never the text.

   `RecordReadinessOverride` REPLACES the marker with Bead 6's `NewOverrideMarkerV2`. Its fields:
   - `signals` = failing ∩ named;
   - `fingerprints[X]` = `readiness.FailureRecordDigests` of X;
   - `overridden` = RFC-3339;
   - `evidence` = the path, sha256 and bytes.

   Keep the marker-before-claim, fail-closed and rollback-on-claim-failure contract (`RollbackReadinessOverride`) unchanged.
5. Sweep the bare and all-or-nothing wording to the scoped form:
   - `internal/next/ready_gate.go`'s NOT-READY recovery string;
   - `cmd/mindspec/next.go`'s record-failure recovery and flag help;
   - `cmd/mindspec/bead_clarify.go`;
   - `.mindspec/core/USAGE.md` (two places) and `.mindspec/core/WORKFLOW-STATE-MACHINE.md`;
   - `.mindspec/domains/workflow/architecture.md`, `interfaces.md` and `runbook.md`;
   - `.mindspec/domains/execution/interfaces.md` (the schema-v2 marker row, recommending a repository-relative evidence path).

   Use `grep -rn 'allow-not-ready' .mindspec/core .mindspec/domains cmd internal/next` (run unpiped) to find every site. Leave `internal/bead/clarify.go` and `internal/bead/bdcli.go` doc comments alone (spec Impacted Domains). Append ADR-0041 amendment part (iii) and extend `internal/validate/adr0041_spec129_amendment_test.go` to pin it.
6. Add the tests:
   - AC-6: `TestGateReadiness_LineageSpecMismatchRefuses`, `TestNext_UnscopedDispatchesIntoLineageSpec`, `TestGateReadiness_NonPlanBeadRefuses`, `TestGateReadiness_MissingSpecBranchRefuses`, `TestNext_NoLineageBugRefuses`;
   - AC-15: `TestGateReadiness_ScopedOverride`, `TestNext_BareAllowNotReady_Refuses`, `TestNext_AllowNotReadySpaceSeparated_Refuses`, `TestNext_ScopedFlagNothingFailing_ClaimsWithoutMarker`, `TestGateReadiness_IndeterminateNotOverridable`;
   - AC-16: the `TestOverride_*` legs and `TestAllowNotReady_EvidenceTooLargeRefuses`;
   - AC-3: `TestNext_N4PositionalOverridesMF1`;
   - AC-17: the end-to-end re-claim legs (`release`, then a fresh MF-2,MF-3 re-claim that REPLACES the marker; an explicit MF-3-only re-claim refuses without mutation; the MF-3/MF-4 and grown-digest variants).

   Every refusal leg asserts a byte-identical bd, branch and worktree audit (spec 124 AC-4 shape) and zero `mergeMetadataFn` calls.

**Verification**
- [ ] `go test ./internal/next/... -run 'TestGateReadiness_|TestRecordReadinessOverride|TestOverride_|TestAllowNotReady_'` passes
- [ ] `go test ./cmd/mindspec/... -run 'TestNext_UnscopedDispatchesIntoLineageSpec|TestNext_NoLineageBugRefuses|TestNext_BareAllowNotReady_Refuses|TestNext_AllowNotReadySpaceSeparated_Refuses|TestNext_ScopedFlagNothingFailing_ClaimsWithoutMarker|TestNext_N4PositionalOverridesMF1|TestReadyCheck_'` passes (the names are binding; place each in the package that reaches its code)
- [ ] `go test ./internal/validate/... -run TestADR0041_Spec129Amendment` passes with parts (i)–(iii)
- [ ] `go test ./internal/complete/... -run Isolation` passes; `go test ./internal/lint/...` passes (the argv ratchet and the guard recovery-convention lint cover the new recovery lines)
- [ ] `mindspec validate spec 129-readiness-tracker-honesty` (run) reports 0 errors
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes, with `internal/harness` and `internal/instruct` under `-short`

**Acceptance Criteria**
- AC-6: legs a–f pass, each refusing before any mutation with a byte-identical audit. The unscoped claim with no resolved spec dispatches into T, cut from `spec/<T>`. The missing spec branch refuses with `ClaimBead` never called. Red-on-revert is recorded.
- AC-15: all eight legs pass. The MF-2+MF-3 refusal's last `recovery:` line contains `--allow-not-ready=MF-2,MF-3 --evidence <path-to-justification>` and no operator path.
- AC-16: all ten legs pass across Beads 3 and 7. The growth and MF-4-multiplicity legs are recorded red-on-revert against the single-fingerprint design.
- AC-3 (scoped-override leg): `mindspec next <O> --spec S --allow-not-ready=MF-1 --evidence <f>` claims O and records `signals: [MF-1]`.
- AC-17 (re-claim legs) pass. AC-18 (ADR-0041 part iii) passes. AC-19: `go build ./... && go test ./... -short` and `go test ./internal/lint/...` are green.

**Depends on**
Bead 4 (MF-3 records, including `indeterminate`), Bead 5 (`cmd/mindspec/next.go` selection rewrite) and Bead 6 (marker schema v2 builder, validator, `ready-check` coverage).

## Bead 8: Review artifacts out of git by default

**Steps**
1. In `internal/gitutil/gitignore.go`, beside `RuntimeIgnoreEntries`, add `ReviewIgnoreEntries = []string{".mindspec/specs/*/reviews/", ".mindspec/reviews/"}` with a doc comment: panel reviews are an optional local helper (spec 129 R8); the list is separate from `RuntimeIgnoreEntries` because doctor's runtime-file check reports tracked runtime entries, and this repo tracks review files that R8 leaves tracked. Add `TestReviewIgnoreEntries_NotRuntimeEntries` to `gitignore_test.go`: the two lists share no entry.
2. In `internal/bootstrap/bootstrap.go`, after `bootstrap.Run`'s existing `gitutil.EnsureGitignoreEntries(root, gitutil.RuntimeIgnoreEntries...)` call, call it again with `gitutil.ReviewIgnoreEntries...`, honouring `dryRun` exactly as the runtime call does. Leave `starterGitignore` unchanged. Add to `internal/bootstrap/gitignore_test.go`:
   - `TestBootstrap_ReviewsIgnoredGreenfield`: a fresh temp repo gets both lines;
   - `TestBootstrap_ReviewsIgnoredExistingGitignore`: a pre-existing `.gitignore` keeps its lines and gains both;
   - `TestBootstrap_ReviewsIgnoredIdempotent`: a second `Run` leaves `.gitignore` byte-identical;
   - in each, `git check-ignore -q` reports `.mindspec/specs/001-x/reviews/a/panel.json` and `.mindspec/reviews/a/panel.json` ignored, and `src/reviews/x.go` not ignored.
3. Add `internal/gitutil/reviews_ignore_test.go` (real git in `t.TempDir()`, identity via `-c` flags or env, never `~/.gitconfig`), with the entries written to `.gitignore` and an untracked `panel.json` under a spec review dir:
   - `TestCommitAll_SkipsIgnoredReviews`: after `gitutil.CommitAll`, `git ls-files` does not list it and `git status --porcelain` is empty;
   - `TestSnapshotWorktreeTree_SkipsIgnoredReviews`: the tree from `gitutil.SnapshotWorktreeTree` does not contain it.
4. Add `internal/complete/panel_gate_untracked_test.go` with `TestPanelGate_UntrackedIgnoredRegistration`: the same registration, tracked in one fixture and untracked-ignored in the other, gives the same `panelGate` decision through `panelGateRoots`. Reuse the fixtures in `panel_gate_layout_test.go`.
5. Add the two lines to this repository's `.gitignore`, next to the existing `**/reviews/*/dispositions.lock` entry, with a comment citing spec 129 R8 and stating that already-tracked review files stay tracked. Do not run `git rm --cached` on anything. Correct that block's comment that `dispositions.jsonl` "IS version-controlled" to say it is version-controlled only where already tracked, pending R8.4.
6. No commit-path code changes (spec R8.2). No retention code (R8.4 needs operator decision).

**Verification**
- [ ] `go test ./internal/gitutil/... -run 'TestReviewIgnoreEntries_NotRuntimeEntries|TestCommitAll_SkipsIgnoredReviews|TestSnapshotWorktreeTree_SkipsIgnoredReviews'` passes
- [ ] `go test ./internal/bootstrap/... -run 'TestBootstrap_ReviewsIgnored'` passes
- [ ] `go test ./internal/complete/... -run TestPanelGate_UntrackedIgnoredRegistration` passes
- [ ] `go test ./internal/doctor/... ./internal/setup/...` passes unchanged (no new tracked-runtime finding; setup's list is untouched)
- [ ] `git check-ignore -v .mindspec/specs/129-readiness-tracker-honesty/reviews/x/panel.json` (run in the bead worktree) names the new `.gitignore` line
- [ ] `gofmt -l .` prints nothing; `go build ./... && go test ./... -short` passes

**Acceptance Criteria**
- AC-20: init adds both entries on a greenfield and an existing `.gitignore`, idempotently; the anchored patterns ignore both review locations and not `src/reviews/`; the two lists are disjoint; this repo's `.gitignore` carries both lines.
- AC-21 (commit and gate legs): `CommitAll` and `SnapshotWorktreeTree` leave the untracked review out, `git status --porcelain` is empty, and the panel gate decides the same on an untracked-ignored registration.

**Depends on**
None.

## Provenance

Enumerated with `grep -nE '^- \[ \] \*\*AC-[0-9]+' .mindspec/specs/129-readiness-tracker-honesty/spec.md`, which printed 21 lines (AC-1 … AC-21), and `grep -cE` with the same pattern, which printed `21`.

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| AC-1 (R1.1, R1.2 selection fixtures) | Bead 5 verification (`TestBuildReadySet_PlanBeadPredicate`, `TestNext_ScopedSelection_*`, `TestNext_EmitOnlyReadsPlanFromSpecBranch`, `TestNext_PlanDivergenceNotice`, `TestNext_PlanBlobAbsentUsesCheckout`, `TestNext_PlanProbeFailureRefuses`, `TestNext_LazyPlanReadForNoLineageRow`, `TestNext_NoPlanYetAllN4`); Bead 3 verification (`TestPlanMapsBead_AndPositional`) |
| AC-2 (R1.2 recoveries are real) | Bead 5 verification (`TestNearMissN1_RecoveryYieldsClaimable`, `TestNearMissN2b_SameNumberReparentCorrectsLineage`, `TestNearMissN2b_OtherNumberInspectsFirst`, `TestNearMissN4_NoClaimableRecovery`); the real-bd reporter-shape run is a spec Validation Proof run by the orchestrator |
| AC-3 (R1.3 selector refusals) | Bead 5 verification (`TestNext_SelectorOnEmptySet_Refuses`, `TestNext_NearMissPositional_Refuses`, `TestNext_UnlinkedReadyPositional_Refuses`, `TestNext_N4NoLineagePositionalRefuses`); Bead 7 verification (`TestNext_N4PositionalOverridesMF1`) |
| AC-4 (R1.4 derived-state emptiness) | Bead 5 verification (`TestNext_EmptyScoped_DerivedState`, `TestNext_EmptyScoped_InformationalNearMissExitsZero`, `TestImplementTemplate_EmptinessRule`) |
| AC-5 (R1.5–R1.7) | Bead 5 verification (`TestExclusionNotice_NamesIDs`, `TestNextMenu_PrintsSpecIDForm`, rewritten spec-128 tests) |
| AC-6 (R1.8 claim guard and dispatch spec) | Bead 7 verification (`TestGateReadiness_LineageSpecMismatchRefuses`, `TestNext_UnscopedDispatchesIntoLineageSpec`, `TestGateReadiness_NonPlanBeadRefuses`, `TestGateReadiness_MissingSpecBranchRefuses`, `TestNext_NoLineageBugRefuses`) |
| AC-7 (R2.1–R2.2 `-n 0` everywhere) | Bead 1 verification (seven site tests, two waist tests, `TestNext_101ReadyBeads_AllVisible`); Bead 5 keeps the 101-row test green |
| AC-8 (R2.3 lint and truncation error) | Bead 1 verification (`TestBDRowCapWaist` with self-test, `TestReadyJSON_TruncationNoticeIsError`) |
| AC-9 (R3.2 linkage pins) | Bead 1 verification (`TestPlanApprove_BeadsCarrySpecID`, `TestSpecApprove_EpicHasNoSpecID`) |
| AC-10 (R4.1 membership) | Bead 4 verification (`TestReadiness_MF3Variants` legs a–g) |
| AC-11 (R4.2 reachable fallback, F1/F3/F4/F5, advisory registrations, audited reattest, read-back) | Bead 2 verification (`TestFindLandedMergeReachable_*`, `TestReattest_Fallback*`, `TestReadBack_*`, `TestMergedUnclosed_IgnoresFallback`); Bead 4 verification (`TestReadiness_MF3_OffFirstParentFailsWithExits`) |
| AC-12 (R4.3 panel-SHA set, R4.5 #232 shape) | Bead 2 verification (`TestFindLandedMerge_PanelSet*`, `TestReattest_PanelSetNonCorroborating`, `TestMergedUnclosed_PanelSet`, `TestReattest_RepairsContradictoryBindingOnFallback`); Bead 4 verification (`TestReadiness_MF3_CompleteWrapperShape`, `TestReadiness_MF3_LocalAheadOfRemotePasses`, leg v's MF-3 half) |
| AC-13 (R4.4 message arms and precedence) | Bead 4 verification (`TestReadiness_MF3ErrorArmDiscrimination`, `TestReadiness_MF3AggregatedPrecedence`) |
| AC-14 (R5 form symmetry) | Bead 3 verification (`TestMF2_FormSymmetry`, `TestReadiness_MF2AllClasses`) |
| AC-15 (R6.1 scoped flag) | Bead 7 verification (`TestGateReadiness_ScopedOverride`, `TestNext_BareAllowNotReady_Refuses`, `TestNext_AllowNotReadySpaceSeparated_Refuses`, `TestNext_ScopedFlagNothingFailing_ClaimsWithoutMarker`, `TestGateReadiness_IndeterminateNotOverridable`) |
| AC-16 (R6.2 record digests, no widening) | Bead 3 verification (`TestFailureRecordDigests_Deterministic`, `TestFailureRecordDigests_WriterReaderAgree`); Bead 7 verification (`TestOverride_*`, `TestAllowNotReady_EvidenceTooLargeRefuses`) |
| AC-17 (R6.3, R6.4 validated read-back) | Bead 6 verification (`TestReadyCheck_HonoursValidMarkerOnly`, `TestOverrideMarker_Validity`, `TestReadyCheck_LegacyRecoveryReleasesThenReclaims`, `TestReadyCheck_MixedCoverageRecoveryNamesAllFailing`); Bead 7 verification (end-to-end release and re-claim legs) |
| AC-18 (R6.5, R7, ADR-0041 amendment) | Bead 6 verification (skill line and staged-rule marker pins, `mindspec adr list` showing ADR-0045, back-pointers); Bead 2 verification (ADR-0041 parts i–ii text pin); Bead 7 verification (ADR-0041 part iii text pin, `mindspec validate spec 129-readiness-tracker-honesty`) |
| AC-19 (method) | Every bead's final verification line (`gofmt -l .`; `go build ./... && go test ./... -short`; `go test ./internal/lint/...`); Bead 7 runs the whole-suite gate last |
| AC-20 (R8.1 init ignores reviews) | Bead 8 verification (`TestBootstrap_ReviewsIgnored{Greenfield,ExistingGitignore,Idempotent}`, `TestReviewIgnoreEntries_NotRuntimeEntries`, the repo `.gitignore` check-ignore line) |
| AC-21 (R8.2, R8.3 no auto-commit; gates read untracked) | Bead 8 verification (`TestCommitAll_SkipsIgnoredReviews`, `TestSnapshotWorktreeTree_SkipsIgnoredReviews`, `TestPanelGate_UntrackedIgnoredRegistration`); Bead 2 verification (`TestReviewedHeadSHASet_UntrackedIgnored`) |
