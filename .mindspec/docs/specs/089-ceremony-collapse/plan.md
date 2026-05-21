---
adr_citations:
    - id: ADR-0034
approved_at: "2026-05-21T02:23:14Z"
approved_by: user
bead_ids:
    - mindspec-v4ec.1
    - mindspec-v4ec.2
    - mindspec-v4ec.3
spec_id: 089-ceremony-collapse
status: Approved
version: "1"
---
# Plan: 089-ceremony-collapse

## ADR Fitness

- **ADR-0034** (new, stub at
  `.mindspec/docs/adr/ADR-0034-ceremony-collapse.md`, `Status:
  Accepted`, `Domain(s): workflow`): the stub records the three
  decisions this spec implements — (1) auto-migrate on first
  lifecycle command with no operator opt-in, (2) keep ceremony
  children + keep `DerivePhaseFromChildren` as the cold-path
  back-stop, (3) `mindspec doctor --dry-run-migration` as the
  pre-mutation reporter. The `Domain(s): workflow` line satisfies
  the spec 087 plan-time cite-relevant gate
  (`checkADRCitations` + `checkADRCoverage` per ADR-0032) against
  this spec's single impacted domain `workflow`. Bead 3 step 5
  finalizes the ADR by replacing the stub `## Status` paragraph
  with text naming the merged implementation surfaces.
- **No other ADRs cited.** ADR-0030 and ADR-0032 are referenced
  in ADR-0034's `## Related` block for cross-linking only;
  neither carries `Domain(s): workflow`, so citing them in this
  plan's frontmatter would not contribute to the cite-relevant
  gate and would risk an `adr-cite-irrelevant` issue under
  ADR-0032. ADR-0034 alone covers the single impacted domain.

## Testing Strategy

This spec's failure mode is **silent ceremony drift**: pre-spec-080
lifecycle epics lack the authoritative `mindspec_phase` metadata
key, so phase derivation continues to traverse the legacy 7-bead
ceremony children indefinitely. The defense is two-layer: (a) a
one-shot `phase.EnsureMigrated` write on first lifecycle command
that converts the children-derived phase into the metadata-first
representation and stamps `mindspec_migrated_at`, and (b) a
regression test asserting the molecule-step scaffolding symbols
(`mol.pour`, `closeoutTargets`, `EnsureFullyBound`) stay absent
from `internal/` so a future commit cannot silently re-introduce
the old ceremony creation path.

**Synthesis-time audit result.** A grep across
`internal/` for the literal identifiers `mol.pour`,
`closeoutTargets`, and `EnsureFullyBound` returned ZERO matches.
The audit requirement (spec Requirement 7) therefore collapses to
the regression-protection test
`TestNoMoleculeScaffoldingSymbols`; no code deletion is required
in Bead 1. If the audit at implementation time finds residue, the
deletion lands in the same Bead 1 commit as the test.

**Bead ordering.** Bead 1 (audit + regression test) lands FIRST,
independent of Bead 2 and giving the suite a falsifiable lower
bound before any new code is added. Bead 2 (`EnsureMigrated`
helper + three call sites) depends only on existing
`bead.MergeMetadata`, `phase.FindEpicBySpecID`, and
`phase.DerivePhaseFromChildren` surfaces. Bead 3 (doctor flag +
ADR-0034 finalization) depends on Bead 2 being in the tree so the
reporter can share the same read sequence — the reporter does
NOT call `EnsureMigrated`; it duplicates `FindEpicBySpecID` +
`readStoredPhase` + `DerivePhaseFromChildren` and writes nothing.

Per HC-5 the per-commit gate is CI; locally each bead's
verification ends with `go build ./... && go test -short ./...`
passing. HC-3 (existing tests preserved, no skips relative to
`main`) is enforced per-bead: Bead 3 step 6 records a `go test
-v ./...` test-name + status diff vs `main` in its final commit
message. The legacy `TestDerivePhaseFromChildren` in
`internal/phase/derive_test.go` continues to pass unchanged
because `DerivePhaseFromChildren` is preserved verbatim per spec
Requirement 14.

**New test additions across the three beads:**

- **Bead 1** (`internal/lint/scaffold_test.go` — new file):
  - `TestNoMoleculeScaffoldingSymbols` — walks `internal/` for
    `.go` files (skips the test file itself via a hard-coded
    `"scaffold_test.go"` suffix check, otherwise the test's own
    string literals would self-fail). For each file, reads bytes
    and `bytes.Contains` against the three banned literals. On
    any hit, `t.Errorf("scaffolding symbol %q reintroduced at
    %s:line %d", sym, path, lineNo)`. Self-contained; no
    fixtures required.

- **Bead 2** (`internal/phase/migrate_test.go` — new file, plus
  extensions in the three approve/complete test files):
  - `TestEnsureMigratedLegacyEpicWritesPhaseAndTimestamp` —
    stubs the bd seam via `phase.SetRunBDForTest` /
    `phase.SetListJSONForTest` (at `derive.go:27` and `:34`)
    representing a legacy epic (metadata lacking
    `mindspec_phase`) with seven children (one closed, six
    open → derives to `implement` per the `totalClosed > 0`
    branch at `derive.go:217`). Asserts the return is `(true,
    nil)` and the captured `MergeMetadata` invocation (via a
    new `SetMergeMetadataForTest` seam added in step 1) wrote
    both `mindspec_phase: "implement"` and a
    `mindspec_migrated_at` parseable by `time.Parse(
    time.RFC3339, …)` as UTC.
  - `TestEnsureMigratedIdempotent` — a second call on the same
    fixture (now with `mindspec_phase` present) returns
    `(false, nil)`; NO further `MergeMetadata` invocation
    recorded.
  - `TestEnsureMigratedNoEpicReturnsFalseNil` — stub where
    `FindEpicBySpecID` returns `""` (pre-approve-spec state).
    Returns `(false, nil)`; NO bd write.
  - `TestEnsureMigratedPropagatesWriteError` — stub where
    `MergeMetadata` errors. Returns `(false, err)` with the
    underlying error wrapped per spec Requirement 9.
  - `TestPlanApproveCallsEnsureMigrated`,
    `TestImplApproveCallsEnsureMigrated`,
    `TestCompleteCallsEnsureMigrated` — each fixture has a
    legacy 7-bead epic; calls the approve/complete entry point
    and asserts the captured `MergeMetadata` log shows the
    `mindspec_phase` + `mindspec_migrated_at` write happened
    BEFORE any phase-dependent bd call (for `complete`, before
    the `DerivePhaseFromChildren` call at `complete.go:404`).

- **Bead 3** (`internal/doctor/dry_run_migration_test.go` — new
  file, and one ADR-text assertion):
  - `TestDoctorDryRunMigrationReports` — fixture with two specs
    under a `t.TempDir()`-rooted `.mindspec/docs/specs/`: one
    legacy (no `mindspec_phase`, two children closed + three
    open → derives `implement`), one post-080 native (epic has
    `mindspec_phase: review`). Calls
    `doctor.RunWithOptions(root, doctor.Options{DryRunMigration:
    true})`. Asserts the resulting `Report` contains a `Check{
    Name: "would-migrate: spec=<legacy>", Status: Warn,
    Message: "epic=<id> phase=implement"}` for the legacy AND
    NO such entry for the native. Re-reads the legacy epic
    metadata after the run and confirms `mindspec_phase` and
    `mindspec_migrated_at` remain absent (no writes).
  - `TestDoctorDryRunMigrationExitsZeroWithLegacySpecs` — the
    presence of legacy specs does NOT cause `HasFailures()`
    true (the `would-migrate` checks are `Warn`, not `Error`
    or `Missing`). Per spec Requirement 11.
  - `TestDoctorDryRunMigrationSkipsExcludedTrees` — fixture
    with a stray `viz/`, `agentmind/`, or `bench/` path under
    the repo root (defensive). Asserts the walk does not
    descend into those trees per HC-4.
  - `TestADR0034FinalizedStatus` — asserts
    `ADR-0034-ceremony-collapse.md`'s `## Status` section no
    longer contains "Stub created during spec 089" after Bead
    3 step 5.

## Provenance

Acceptance criteria → bead mapping:

| AC | Bead | Verification |
|----|------|--------------|
| `TestApproveSpecCreatesSingleBead` | Bead 1 | Existing behaviour confirmed (no code change at `internal/approve/spec.go:56-96`); test asserts exactly one bead exists for a freshly-approved spec and that bead carries `metadata.mindspec_phase == "plan"`. |
| `TestPhaseDerivationFromMetadataOnly` | Bead 2 | Verifies `DerivePhaseWithStatusWithCache` (`internal/phase/derive.go:120-130`) short-circuits on stored `mindspec_phase` without calling `queryChildren`; pre-existing behaviour, asserted by the new test as a guard. |
| `TestLegacyMigratesOnFirstCommand` | Bead 2 | `TestPlanApproveCallsEnsureMigrated` + `TestImplApproveCallsEnsureMigrated` + `TestCompleteCallsEnsureMigrated` together prove the call-site contract. |
| `TestDoctorDryRunMigrationReports` | Bead 3 | Direct test; verifies the dry-run reporter walks `.mindspec/docs/specs/`, identifies legacy epics, prints `would-migrate` lines, and writes nothing. |
| `TestMigratedEpicHasMigratedAtMarker` | Bead 2 | `TestEnsureMigratedLegacyEpicWritesPhaseAndTimestamp` asserts the RFC3339 UTC timestamp is parseable via `time.Parse(time.RFC3339, …)`. |
| `TestEnsureMigratedIdempotent` | Bead 2 | Direct test in `internal/phase/migrate_test.go`. |
| `TestEnsureMigratedNoEpicReturnsFalseNil` | Bead 2 | Direct test in `internal/phase/migrate_test.go`. |
| `TestNoMoleculeScaffoldingSymbols` | Bead 1 | Direct test in `internal/lint/scaffold_test.go`. |
| `TestDerivePhaseFromChildrenStillPasses` | Bead 2 | Existing `internal/phase/derive_test.go::TestDerivePhaseFromChildren` runs unchanged; preservation enforced by Bead 2 not touching `DerivePhaseFromChildren`'s body. |
| `TestCompleteUnchangedOnPostMigrationFixture` | Bead 2 | Existing `internal/complete/complete_test.go` fixtures continue to pass because `EnsureMigrated` writes the children-derived value (by construction). |
| ADR-0034 finalized at `Status: Accepted` with `Domain(s): workflow` | Bead 3 | Bead 3 step 5 replaces the "Stub created during spec 089" paragraph; `TestADR0034FinalizedStatus` enforces. |
| `go build ./... && go test -short ./...` green per commit | Beads 1–3 | Per-bead verification block; HC-5. |

---

## Bead 1 — Audit and regression-pin molecule scaffolding

**Title**: `[089] audit and regression-pin molecule-step scaffolding`
**Type**: `task`
**Parent epic**: spec 089 lifecycle epic
**Depends on**: none
**Estimated effort**: ~30 min

### Goal

Confirm via grep that the molecule-step scaffolding symbols
(`mol.pour`, `closeoutTargets`, `EnsureFullyBound`) are absent
from the `internal/` tree, and add a regression test that fails
if any future commit re-introduces them. Synthesis-time grep
already showed zero matches, so the expected outcome is "no code
deletion, one new test file."

### Files touched

- **NEW** `internal/lint/scaffold_test.go` — the regression test.
- (Conditional) any `internal/**/*.go` file that, at
  implementation time, surfaces a re-introduction of one of the
  three symbols — delete the dead code + its callers in the same
  commit. Not expected per synthesis-time grep.

**Steps**

1. Re-run the audit:
   `grep -rn "mol\.pour\|closeoutTargets\|EnsureFullyBound"
   internal/`. If the output is non-empty, list each hit, decide
   per call whether it is dead code (delete) or a needed
   reference inside an audit test (rename to avoid the literal
   match — the regression test scans literals).
2. Create `internal/lint/scaffold_test.go` with a single
   `TestNoMoleculeScaffoldingSymbols` test. Walk `internal/`
   for `.go` files (skip the test file itself via a hard-coded
   `"scaffold_test.go"` suffix check on the walked path). For
   each file, read bytes and `bytes.Contains` against the three
   banned literals. On any hit, `t.Errorf("scaffolding symbol
   %q reintroduced at %s", sym, path)`.
3. Verify locally:
   `go test -run TestNoMoleculeScaffoldingSymbols
   ./internal/lint/...` is green.

**Verification**

- [ ] `go build ./... && go test -short ./...` is green.
- [ ] `go test -run TestNoMoleculeScaffoldingSymbols
  ./internal/lint/...` passes against the current tree (the
  audit confirms zero hits).
- [ ] Manual smoke test: inject the literal `mol.pour` into any
  non-test file, re-run the test, see it fail, revert.

**Acceptance Criteria**

- [ ] `TestNoMoleculeScaffoldingSymbols` passes; the new file
  `internal/lint/scaffold_test.go` scans `internal/` for the
  three banned literals (`mol.pour`, `closeoutTargets`,
  `EnsureFullyBound`) and fails on any hit.
- [ ] `TestApproveSpecCreatesSingleBead` continues to pass
  (existing behaviour confirmed; no code change at
  `internal/approve/spec.go:56-96`).
- [ ] `go build ./... && go test -short ./...` exits 0 (per
  HC-5; no regressions vs `main`).

### Commit message

```
[089] regression-pin: assert molecule scaffolding symbols stay absent

Adds internal/lint/scaffold_test.go::TestNoMoleculeScaffoldingSymbols
which scans internal/ for the literal identifiers mol.pour,
closeoutTargets, and EnsureFullyBound. Synthesis-time grep confirmed
these symbols are already absent from the tree; this test prevents
regression.

Per spec 089 Requirement 7. AC: TestNoMoleculeScaffoldingSymbols.
```

---

## Bead 2 — `phase.EnsureMigrated` helper + three call sites

**Title**: `[089] phase.EnsureMigrated auto-migrator + approve/complete call sites`
**Type**: `task`
**Parent epic**: spec 089 lifecycle epic
**Depends on**: Bead 1 (so the scaffolding regression test is in
place before any phase-package code lands)
**Estimated effort**: ~3-4 h

### Goal

Add the exported `phase.EnsureMigrated(specID string) (migrated
bool, err error)` helper to `internal/phase/derive.go` and wire
it into `approve.PlanApprove`, `approve.ImplApprove`, and
`complete.Complete` as the first phase-touching call in each
flow.

### Files touched

- **MODIFIED** `internal/phase/derive.go` — append the new
  `EnsureMigrated` function (and a `mergeMetadataFn` indirection
  + `SetMergeMetadataForTest` helper for the test seam) after
  the existing `DerivePhaseFromChildren` block (around line
  222). Do NOT modify any existing exported surface.
- **MODIFIED** `internal/approve/plan.go` — insert
  `phase.EnsureMigrated(specID)` call immediately after spec ID
  validation, before existing plan-validation logic.
- **MODIFIED** `internal/approve/impl.go` — insert
  `phase.EnsureMigrated(specID)` call immediately after spec ID
  validation, before existing impl-validation logic.
- **MODIFIED** `internal/complete/complete.go` — insert
  `phase.EnsureMigrated(specID)` call immediately after spec ID
  resolution, before the existing
  `phase.DerivePhaseFromChildren` call at line 404.
- **NEW** `internal/phase/migrate_test.go` — the new helper's
  unit tests.
- **MODIFIED** `internal/approve/plan_test.go`,
  `internal/approve/impl_test.go`,
  `internal/complete/complete_test.go` — add the three
  `TestXCallsEnsureMigrated` extensions using existing seams
  (`SetPlanMergeMetadataForTest` at `plan.go:39`,
  `implMergeMetadataFn` at `impl.go:26`, plus the new
  `phase.SetMergeMetadataForTest`).

**Steps**

1. **Helper definition.** In `internal/phase/derive.go`, add a
   package-level `mergeMetadataFn = bead.MergeMetadata`
   indirection + `SetMergeMetadataForTest` helper following the
   existing `SetRunBDForTest` pattern at line 27. Append at
   the end of file (after the existing `DerivePhaseFromChildren`
   function around line 222):

   ```go
   // EnsureMigrated runs one-shot legacy-to-metadata migration for
   // the spec's lifecycle epic. Returns (migrated bool, err error).
   //
   // Per ADR-0034: if the spec's epic lacks mindspec_phase metadata
   // (legacy pre-080 7-bead layout), derive the phase from existing
   // ceremony children once, write mindspec_phase + mindspec_migrated_at,
   // and return (true, nil). If the metadata is already present, or no
   // epic exists yet (pre-approve-spec), return (false, nil).
   //
   // Idempotent: a second call on a migrated epic returns (false, nil).
   func EnsureMigrated(specID string) (bool, error) {
       epicID, err := FindEpicBySpecID(specID)
       if err != nil || epicID == "" {
           return false, nil // nothing to migrate
       }
       c := NewCache()
       if storedPhase := readStoredPhaseWithCache(c, epicID); storedPhase != "" {
           return false, nil // already migrated or post-080 native
       }
       children, _ := c.GetChildren(epicID)
       derived := DerivePhaseFromChildren(children)
       if err := mergeMetadataFn(epicID, map[string]interface{}{
           "mindspec_phase":       derived,
           "mindspec_migrated_at": time.Now().UTC().Format(time.RFC3339),
       }); err != nil {
           return false, fmt.Errorf("ensure-migrated %s: %w", specID, err)
       }
       fmt.Fprintf(os.Stderr, "event=lifecycle.migrated spec=%s epic=%s phase=%s\n",
           specID, epicID, derived)
       return true, nil
   }
   ```

   Add `"time"` to the existing import block. The `bead.MergeMetadata`
   reference uses the existing `internal/bead` import on line 11.

2. **Call site — `internal/approve/plan.go::PlanApprove`.**
   After the existing `validate.SpecID(specID)` check near the
   top of `PlanApprove`, insert:

   ```go
   if _, err := phase.EnsureMigrated(specID); err != nil {
       return nil, err
   }
   ```

   The error is surfaced to the caller — a migration write
   failure fails the command, per spec Requirement 9.

3. **Call site — `internal/approve/impl.go::ImplApprove`.** Same
   insertion immediately after `validate.SpecID(specID)` near
   the top of `ImplApprove`.

4. **Call site — `internal/complete/complete.go::Complete`.**
   Insert immediately after the spec ID resolution block (in
   `Complete` itself, before any call that reaches
   `advanceState(root, specID)` at line 393 → which calls
   `phase.DerivePhaseFromChildren` at line 404). The call must
   precede any phase-dependent read.

5. **Tests in `internal/phase/migrate_test.go`.** Use the new
   `phase.SetMergeMetadataForTest` seam plus the existing
   `phase.SetRunBDForTest` / `phase.SetListJSONForTest` seams to
   stub bd. The four `TestEnsureMigrated*` tests in the Testing
   Strategy block above all sit in this file.

6. **Tests in approve/complete packages.** Each test:
   1. Stubs `FindEpicBySpecID` (via the bd seam) to return a
      legacy epic ID, stubs the child query to return the
      7-bead layout, stubs the relevant `MergeMetadata` seam.
   2. Calls the approve/complete entry point.
   3. Asserts the captured `MergeMetadata` invocations show
      `mindspec_phase` + `mindspec_migrated_at` written BEFORE
      any plan/impl/complete-specific bd call.

**Verification**

- [ ] `go build ./... && go test -short ./...` is green.
- [ ] `go test -run "Migrate|Migrated" ./internal/phase/...
  ./internal/approve/... ./internal/complete/...` is green.
- [ ] Existing `TestDerivePhaseFromChildren` passes unchanged.
- [ ] AST boundary lint (`internal/lint/boundary_test.go`)
  stays green — the helper adds no `os/exec` or git imports;
  it uses only `bead.MergeMetadata` + the existing `phase`
  package internals. Per HC-6.

**Acceptance Criteria**

- [ ] `TestPhaseDerivationFromMetadataOnly` passes; verifies
  `DerivePhaseWithStatusWithCache` short-circuits on stored
  `mindspec_phase` without calling `queryChildren`.
- [ ] `TestLegacyMigratesOnFirstCommand` proven via
  `TestPlanApproveCallsEnsureMigrated`,
  `TestImplApproveCallsEnsureMigrated`, and
  `TestCompleteCallsEnsureMigrated` — each asserts the
  `MergeMetadata` write of `mindspec_phase` +
  `mindspec_migrated_at` happens before any phase-dependent
  bd call.
- [ ] `TestMigratedEpicHasMigratedAtMarker` passes
  (`TestEnsureMigratedLegacyEpicWritesPhaseAndTimestamp`
  asserts the RFC3339 UTC timestamp is parseable).
- [ ] `TestEnsureMigratedIdempotent` passes; second call on a
  migrated fixture returns `(false, nil)` with NO further
  `MergeMetadata` invocation.
- [ ] `TestEnsureMigratedNoEpicReturnsFalseNil` passes; when
  `FindEpicBySpecID` returns `""` the helper returns
  `(false, nil)` with NO bd write.
- [ ] `TestDerivePhaseFromChildrenStillPasses`: the existing
  `internal/phase/derive_test.go::TestDerivePhaseFromChildren`
  runs unchanged (preservation enforced by Bead 2 not touching
  `DerivePhaseFromChildren`'s body, per spec Requirement 14).
- [ ] `TestCompleteUnchangedOnPostMigrationFixture`: existing
  `internal/complete/complete_test.go` fixtures continue to
  pass because `EnsureMigrated` writes the children-derived
  value by construction.
- [ ] `go build ./... && go test -short ./...` exits 0 per
  HC-5; AST boundary lint stays green per HC-6.

### Commit message

```
[089] add phase.EnsureMigrated auto-migrator + wire approve/complete

Introduces phase.EnsureMigrated(specID) which converts legacy 7-bead
specs to the metadata-first representation on first lifecycle command:
derives mindspec_phase from existing children once, writes it plus
mindspec_migrated_at (RFC3339 UTC), then returns. Idempotent.

Wires three call sites: approve.PlanApprove, approve.ImplApprove,
complete.Complete. Each calls EnsureMigrated immediately after spec ID
resolution; migration errors fail the command.

Per ADR-0034 and spec 089 Requirements 8, 9, 12.
AC: TestLegacyMigratesOnFirstCommand, TestMigratedEpicHasMigratedAtMarker,
TestEnsureMigratedIdempotent, TestEnsureMigratedNoEpicReturnsFalseNil.
```

---

## Bead 3 — `mindspec doctor --dry-run-migration` + ADR-0034 finalization

**Title**: `[089] doctor --dry-run-migration reporter + ADR-0034 status finalization`
**Type**: `task`
**Parent epic**: spec 089 lifecycle epic
**Depends on**: Bead 2 (the dry-run reporter shares the
`readStoredPhase` + `DerivePhaseFromChildren` read sequence with
the migrator; Bead 2 establishes the call-site invariants the
reporter parallels)
**Estimated effort**: ~2-3 h

### Goal

Add the `--dry-run-migration` flag to `mindspec doctor`,
implement the reporter that walks `.mindspec/docs/specs/`, and
finalize ADR-0034's `Status` block to remove the stub language.

### Files touched

- **MODIFIED** `cmd/mindspec/doctor.go` — register the new
  `--dry-run-migration` bool flag (parallel to the existing
  `--fix` and `--force` flags at line 75-76); plumb into
  `doctor.Options.DryRunMigration`.
- **MODIFIED** `internal/doctor/doctor.go` — extend the
  `Options` struct with `DryRunMigration bool`; in
  `RunWithOptions`, when set, call the new reporter from
  `internal/doctor/migration.go` and skip the other
  fix-oriented checks.
- **MODIFIED** `internal/doctor/migration.go` — add
  `checkDryRunMigration(r *Report, root string)` that walks
  `.mindspec/docs/specs/`, identifies legacy epics lacking
  `mindspec_phase`, and appends one `Check{Name: "would-migrate:
  spec=<id>", Status: Warn, Message: "epic=<id>
  phase=<derived>"}` per legacy spec. Writes NOTHING.
- **NEW** `internal/doctor/dry_run_migration_test.go` — the four
  reporter tests from the Testing Strategy block.
- **MODIFIED** `.mindspec/docs/adr/ADR-0034-ceremony-collapse.md`
  — replace the `## Status` block paragraph "Stub created during
  spec 089-ceremony-collapse drafting. Finalized in spec 089
  Bead 3 alongside the auto-migrator implementation." (lines
  14-16) with the finalized text naming the merged
  implementation surfaces.

**Steps**

1. **Options struct extension.** In `internal/doctor/doctor.go`,
   add `DryRunMigration bool` to the `Options` struct (around
   line 62) with a comment block explaining: "when true,
   doctor skips its repair checks and runs only the
   `checkDryRunMigration` reporter, which walks all specs and
   reports which would migrate on their next lifecycle command.
   Writes nothing."
2. **Reporter.** In `internal/doctor/migration.go`, add
   `checkDryRunMigration(r *Report, root string)`. The reporter
   uses `filepath.Glob(filepath.Join(root, ".mindspec",
   "docs", "specs", "*"))` to enumerate spec directories,
   parses each directory basename as a spec ID, and for each
   spec ID:
   1. Calls `phase.FindEpicBySpecID(specID)`. Skip on empty.
   2. Resolves the epic via a single `bd show` and reads
      metadata; if `mindspec_phase` is present, skip.
   3. Queries children via `phase.NewCache().GetChildren`,
      computes `phase.DerivePhaseFromChildren(children)`.
   4. Appends `Check{Name: fmt.Sprintf("would-migrate:
      spec=%s", specID), Status: Warn, Message:
      fmt.Sprintf("epic=%s phase=%s", epicID, derived)}` to
      the report.
   Excluded-tree guard runs against each resolved path with
   `strings.HasPrefix` checks for `viz/`, `agentmind/`,
   `bench/` (defensive; specs are not under those trees per
   HC-4 but the guard makes the contract explicit).
3. **Run wiring.** In `RunWithOptions` (line 72), before the
   existing `checkDocs(r, root)` call at line 74, branch on
   `opts.DryRunMigration`: if true, call
   `checkDryRunMigration(r, root)` and return early. The
   dry-run path does not run any other check (per spec
   Requirement 11: "Exit code is 0 if all walks succeed
   regardless of how many legacy specs exist").
4. **CLI flag.** In `cmd/mindspec/doctor.go`, add
   `doctorCmd.Flags().Bool("dry-run-migration", false, "Report
   which specs would migrate on their next lifecycle command
   without writing any state")` (parallel to the existing
   `Bool("fix", …)` registration at line 75). Plumb the value
   into `doctor.Options.DryRunMigration` (parallel to the
   existing `Force: force` plumbing at line 36 — the new line
   reads `dryRun, _ := cmd.Flags().GetBool("dry-run-migration")`
   then `doctor.RunWithOptions(root, doctor.Options{Force:
   force, DryRunMigration: dryRun})`).
5. **ADR-0034 finalization.** In
   `.mindspec/docs/adr/ADR-0034-ceremony-collapse.md`, replace
   the current `## Status` block paragraph (lines 13-16) with:

   ```markdown
   ## Status

   Accepted. Implemented in spec 089-ceremony-collapse:
   - `phase.EnsureMigrated` at `internal/phase/derive.go` (Bead 2)
   - Call sites in `internal/approve/plan.go`,
     `internal/approve/impl.go`, `internal/complete/complete.go`
     (Bead 2)
   - `mindspec doctor --dry-run-migration` reporter in
     `internal/doctor/migration.go` (Bead 3)
   - Regression guard `TestNoMoleculeScaffoldingSymbols` in
     `internal/lint/scaffold_test.go` (Bead 1)
   ```

   No other ADR-0034 section is edited.
6. **Tests.** Create `internal/doctor/dry_run_migration_test.go`
   with the four tests from the Testing Strategy block. Stub
   `phase.FindEpicBySpecID` and the bd children query via the
   existing `phase.SetRunBDForTest` /
   `phase.SetListJSONForTest` seams.

**Verification**

- [ ] `go build ./... && go test -short ./...` is green.
- [ ] `mindspec doctor --dry-run-migration` against a hand-built
  test repo with one legacy + one native spec prints exactly
  one `would-migrate: spec=…` line and exits 0 (smoke test
  noted in commit message; not automated beyond the
  `TestDoctorDryRunMigration*` cases).
- [ ] `go test -v ./...` shows zero new skips or excludes
  relative to `main` (HC-3). Bead 3's commit message records
  the diff.

**Acceptance Criteria**

- [ ] `TestDoctorDryRunMigrationReports` passes; reporter
  walks `.mindspec/docs/specs/`, identifies legacy epics, emits
  a `would-migrate: spec=<id>` Warn check per legacy spec, and
  writes nothing.
- [ ] `TestDoctorDryRunMigrationExitsZeroWithLegacySpecs`
  passes; presence of legacy specs does NOT cause
  `HasFailures()` true (Warn checks only). Per spec
  Requirement 11.
- [ ] `TestDoctorDryRunMigrationSkipsExcludedTrees` passes;
  the walk does not descend into `viz/`, `agentmind/`, or
  `bench/` per HC-4.
- [ ] `TestADR0034FinalizedStatus` passes;
  `ADR-0034-ceremony-collapse.md`'s `## Status` section no
  longer contains "Stub created during spec 089" after Bead 3
  step 5.
- [ ] ADR-0034 finalized at `Status: Accepted` with
  `Domain(s): workflow`; the `## Status` paragraph names the
  merged implementation surfaces (helper, call sites,
  reporter, regression guard).
- [ ] `go build ./... && go test -short ./...` exits 0 per
  HC-5; `go test -v ./...` shows zero new skips/excludes vs
  `main` per HC-3 (diff recorded in commit message).

### Commit message

```
[089] doctor --dry-run-migration reporter + finalize ADR-0034

Adds the --dry-run-migration flag to mindspec doctor. When set, the
doctor walks .mindspec/docs/specs/, identifies legacy 7-bead specs
whose lifecycle epic lacks mindspec_phase metadata, and reports each
as a would-migrate Warn check. Writes nothing.

Also replaces ADR-0034's Status block stub language with the
finalized text naming the implementation surfaces landed in beads 1-3.

Per ADR-0034 decision 3 and spec 089 Requirements 11, 15.
AC: TestDoctorDryRunMigrationReports,
TestDoctorDryRunMigrationExitsZeroWithLegacySpecs,
TestDoctorDryRunMigrationSkipsExcludedTrees,
TestADR0034FinalizedStatus.
```
