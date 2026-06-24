---
approved_at: "2026-05-21T02:09:28Z"
approved_by: user
status: Approved
---
# Spec 089-ceremony-collapse: Beads ceremony collapse — single-bead epic + legacy 7-bead auto-migrator

## Goal

`mindspec approve spec` creates exactly one bead (the lifecycle
epic) for a new spec; `phase.DerivePhase` reads only
`mindspec_phase` metadata for migrated epics; legacy 7-bead specs
auto-migrate on first lifecycle command after migration (no
`doctor --fix-ceremony` opt-in required).

This spec is **F5** of the converged transformation plan
(`/Users/Max/replit/mindspec-transformation-plan.md` lines ~165-195)
and is the last item in the F4 → F2 → F1 → F3 → F5 chain. F1 (087),
F2 (086), F3 (088), and F4 (085) have all landed. Per the plan,
"most of F5 is already done" — verification during synthesis
confirmed that `internal/approve/spec.go:56-90` already creates a
single lifecycle epic with `mindspec_phase: plan` in metadata, and
`internal/phase/derive.go:103-130` already treats stored metadata
as authoritative with a child-derived fallback. The remaining work
is the auto-migrator for legacy 7-bead specs, the
`--dry-run-migration` doctor flag, the `mindspec_migrated_at`
marker, and removal of any vestigial molecule-step scaffolding.

## Background

The historical molecule-step "7-bead" ceremony pre-dates ADR-0023
and ADR-0030. Spec 080 introduced `mindspec_phase` metadata on
lifecycle epics so phase derivation no longer requires walking
child beads. Per the F5 design block, the lifecycle epic is
**already** created as a single bead by `approve.SpecApprove`
(see `internal/approve/spec.go:56-90`, which calls `bd create
--type=epic --metadata '{"spec_num":...,"mindspec_phase":"plan"}'`
and is idempotent against pre-existing epics), and
`phase.DerivePhaseWithStatusWithCache`
(`internal/phase/derive.go:103-130`) reads stored metadata first,
running child-based derivation only as a consistency check. A
grep at synthesis time confirmed that `mol.pour`,
`closeoutTargets`, and `EnsureFullyBound` are no longer present in
the tree; these were already removed in earlier specs. Audit (see
Requirement 7) re-confirms.

What remains for F5 is:

1. Audit (one final grep + manual review) that no
   molecule-step scaffolding creation paths remain anywhere. If
   any vestigial references are found, delete them.
2. Convert the legacy `DerivePhaseFromChildren` fallback
   (`internal/phase/derive.go:141` and `:144`, currently used by
   `DerivePhaseWithStatusWithCache` when stored metadata is
   absent) into a **one-shot auto-migrator** that runs on first
   lifecycle command (`approve plan`, `approve impl`, `complete`):
   if the spec's epic lacks `mindspec_phase`, derive it from
   children once, write it via `bead.MergeMetadata`, stamp
   `mindspec_migrated_at` with the current RFC3339 timestamp, then
   proceed with the original command. Ceremony children are NOT
   deleted; they are ignored after migration.
3. Add `mindspec doctor --dry-run-migration` to report what *would*
   migrate without writing anything.

Per the plan: "Auto-migrate on first command; do not require
operator opt-in. Keep the dry-run affordance Claude was right to
want. Do not retain the children-derived fallback indefinitely — it
ends at the moment migration writes metadata." Legacy specs that
operators never touch continue working via the existing
metadata-or-children-derived dual read; a future major version may
remove the children-derived fallback after a deprecation window.

## Impacted Domains

- workflow

## Affected packages (per domain)

(Heading is at top level `##` — not `###` — to avoid the parser
bug discovered while drafting spec 088, which caused
`### Affected packages` nested under `## Impacted Domains` to be
silently mis-attributed.)

- **`internal/approve/spec.go`** (domain: `workflow`) — already
  creates the single lifecycle epic at lines 56-90 with
  `mindspec_phase: plan` in initial metadata. This spec touches
  this file only to confirm by test
  (`TestApproveSpecCreatesSingleBead`) that exactly one bead is
  created per `mindspec approve spec`. No code change is required
  unless the audit at implementation time finds residue.
- **`internal/approve/impl.go`** (domain: `workflow`) — entry
  point for `approve impl`. Adds a call to the new
  `phase.EnsureMigrated(specID)` helper at the top of the
  approval flow, before any phase-dependent logic, so a legacy
  7-bead epic is migrated exactly once.
- **`internal/approve/plan.go`** (domain: `workflow`) — entry
  point for `approve plan`. Same `phase.EnsureMigrated(specID)`
  call at the top of the approval flow as `impl.go`.
- **`internal/complete/complete.go`** (domain: `workflow`) —
  entry point for `mindspec complete`. Same
  `phase.EnsureMigrated(specID)` call at the top of the complete
  flow, before the existing `phase.DerivePhaseFromChildren` call
  at line 404 (which becomes the trusted post-migration metadata
  read).
- **`internal/phase/derive.go`** (domain: `workflow`) — adds the
  new exported helper
  `EnsureMigrated(specID string) (migrated bool, err error)`. It
  resolves the spec's epic via the existing
  `FindEpicBySpecID(specID)`, checks for `mindspec_phase` in
  metadata via the existing `readStoredPhaseWithCache` path, and
  if absent: calls `DerivePhaseFromChildren` once, writes the
  derived phase plus `mindspec_migrated_at: <RFC3339 now>` via
  `bead.MergeMetadata`, and returns `migrated=true`. If
  `mindspec_phase` is already present (i.e., the epic was created
  post-080), returns `migrated=false, nil`. The existing
  `DerivePhaseFromChildren` and `deriveFromChildrenOrStatusWithCache`
  remain in place as the back-stop fallback for unmigrated legacy
  epics that no one has touched (see Scope: Out).
- **`internal/doctor/migration.go`** (domain: `workflow`) — adds
  the `--dry-run-migration` flag handling. The flag, when set,
  walks all specs under `.mindspec/specs/`, locates each
  spec's epic via `phase.FindEpicBySpecID`, checks whether the
  epic has `mindspec_phase` metadata, and prints a line per legacy
  spec stating what would be written (the derived phase value)
  without writing anything. The existing
  `checkMigrationMetadata(r *Report, root string)` function (at
  line 25) is the natural integration point.
- **`cmd/mindspec/`** (domain: `execution`, READ-ONLY for this
  spec) — the existing `mindspec doctor` subcommand wires the
  new `--dry-run-migration` boolean flag through to
  `internal/doctor/`. Pure flag plumbing; no behaviour change
  in the `execution` domain, so `execution` is NOT listed in
  `## Impacted Domains`.

## ADR Touchpoints

- [ADR-0034-ceremony-collapse.md](../../adr/ADR-0034-ceremony-collapse.md)
  (**new**, drafted as part of this spec): records the decision
  that lifecycle epics are single-bead (the epic itself), that
  `mindspec_phase` metadata is the authoritative phase source,
  that legacy 7-bead specs auto-migrate on first lifecycle command
  (no operator opt-in), that migrated epics are stamped with
  `mindspec_migrated_at` (RFC3339), that ceremony children are
  retained (not deleted) for back-compat, and that the
  children-derived fallback (`DerivePhaseFromChildren`) is kept
  as a last-resort back-stop for unmigrated legacy epics until a
  future major version removes it. **`Domain(s): workflow`** is
  set so the spec 087 plan-time cite-relevant gate
  (`checkADRCitations` + `checkADRCoverage`) passes against this
  spec's single impacted domain (`workflow`).
- ADR-0030 (executor boundary) and ADR-0032 (ADR semantic gates)
  are intentionally NOT cited here. ADR-0030's `Domain(s)` field
  is `validation lifecycle` and ADR-0032's is
  `validation adr lifecycle`; **neither includes `workflow`**, so
  citing them would not contribute to the F1 cite-relevant gate
  for this spec's `workflow` domain. ADR-0034 (new) is the only
  ADR required to satisfy that gate.

## Requirements

### Hard Constraints (from converged plan)

1. **HC-1 F5 lands after F1 (087).** F1 (087) and F3 (088) have
   merged; F5 is last in the F4 → F2 → F1 → F3 → F5 chain. No
   direct code dependency between F5 and F1, but the chain
   ordering is the cross-cutting decision per the plan.
2. **HC-2 Solo-developer UX preserved.** Auto-migration runs on
   first lifecycle command with no operator opt-in, no prompt, no
   interactive confirmation. The migration is silent on success
   (a structured log line is emitted to stderr per
   `internal/log` conventions, but no operator action is
   required). `--dry-run-migration` is the only opt-in surface and
   it never writes.
3. **HC-3 Existing test suite preserved.** No test is skipped,
   excluded, or marked `t.Skip` relative to `main`. The existing
   `TestDerivePhaseFromChildren` (`internal/phase/derive_test.go`)
   continues to pass unchanged because `DerivePhaseFromChildren`
   is preserved as the migration's derivation step and as the
   back-stop fallback. The existing `complete_test.go` cases
   continue to pass because the `phase.DerivePhaseFromChildren`
   call at `complete.go:404` is preserved (it now runs against
   post-migration metadata-trusted state, but its inputs and
   outputs are unchanged on the seven-bead test fixtures).
4. **HC-4 `viz/agentmind/bench` excluded.** F5 does not read,
   reference, or include any file under those trees. The doctor
   `--dry-run-migration` walk explicitly skips any spec whose path
   starts with `viz/`, `agentmind/`, or `bench/`. (Specs are under
   `.mindspec/specs/`, none of which is excluded, but the
   walk is defensive.)
5. **HC-5 Every commit `go build ./... && go test -short ./...`
   green.** Each of: (a) the audit/no-op commit (if no
   scaffolding remains, this commit is just a test asserting the
   absence), (b) the `phase.EnsureMigrated` helper introduction
   commit, (c) the three caller-site commits (`approve/plan.go`,
   `approve/impl.go`, `complete/complete.go`), (d) the
   `--dry-run-migration` doctor commit, (e) the ADR-0034 drafting
   commit.
6. **HC-6 AST boundary lint stays green.** The new
   `phase.EnsureMigrated` helper uses only the existing
   `bead.MergeMetadata` and `phase.FindEpicBySpecID` surfaces; it
   adds no new git/process call sites and respects the spec 085
   executor boundary contract. `internal/lint/boundary_test.go`
   stays green.

### Spec-specific (from F5 design)

7. **Audit and remove vestigial molecule-step scaffolding.** Per
   the plan: "grep for `mol.pour` and similar. `closeoutTargets` /
   `EnsureFullyBound` may already be vestigial — confirm and
   delete." Synthesis-time grep confirmed these symbols are
   already absent from the working tree (no matches under
   `--include="*.go"`). This requirement is satisfied by:
   (a) a test `TestNoMoleculeScaffoldingSymbols` that fails if any
   future commit re-introduces the literal identifiers `mol.pour`,
   `closeoutTargets`, or `EnsureFullyBound` anywhere under
   `internal/`; (b) if any such symbol is found at implementation
   time, deletion plus its callers in the same commit.
8. **`phase.EnsureMigrated(specID string) (migrated bool, err
   error)` helper.** New exported function in
   `internal/phase/derive.go`. Algorithm:
   1. Resolve epic via `phase.FindEpicBySpecID(specID)`. If empty
      (no epic exists yet — e.g., `approve spec` is in flight),
      return `(false, nil)` — there is nothing to migrate.
   2. Read epic's stored phase via the existing
      `readStoredPhaseWithCache` helper. If present (non-empty),
      return `(false, nil)` — already migrated (or post-080
      native).
   3. Query children via the existing cache-aware children fetch
      (`queryChildrenWithCache`). Call
      `DerivePhaseFromChildren(children)` to compute the derived
      phase string.
   4. Write the derived phase plus the migration timestamp via a
      single `bead.MergeMetadata(epicID, map[string]interface{}{
      "mindspec_phase": derived, "mindspec_migrated_at":
      time.Now().UTC().Format(time.RFC3339)})` call. Return
      `(true, nil)` on success, or `(false, err)` on write
      failure.
   5. The helper is idempotent: calling it twice on the same
      already-migrated epic returns `(false, nil)` the second
      time (because step 2's stored-phase read short-circuits).
9. **First-command auto-migration call sites.** The following
   three entry points each call `phase.EnsureMigrated(specID)` at
   the top of their flow, before any phase-dependent logic:
   - `internal/approve/plan.go::PlanApprove(specID, ...)` —
     immediately after spec ID validation, before the existing
     plan-validation steps.
   - `internal/approve/impl.go::ImplApprove(specID, ...)` —
     immediately after spec ID validation, before the existing
     impl-validation steps.
   - `internal/complete/complete.go::Complete(...)` — immediately
     after spec ID resolution, before the existing
     `phase.DerivePhaseFromChildren(children)` call at line 404.
   Errors from `EnsureMigrated` are surfaced to the caller (the
   command fails; the operator sees the error). A successful
   migration emits a structured log line to stderr in the existing
   `internal/log` format (`event=lifecycle.migrated spec=<id>
   epic=<id> phase=<derived>`).
10. **`approve spec` does NOT call `EnsureMigrated`.** `approve
    spec` is the *creator* of the lifecycle epic; it cannot
    migrate an epic that does not yet exist. The auto-migration is
    a no-op when called on a newly-created epic (because the new
    epic has `mindspec_phase: plan` from creation per spec 080), so
    no call is needed and no test regression occurs. `approve
    spec`'s only behaviour change in this spec is the addition of
    the `TestApproveSpecCreatesSingleBead` assertion confirming
    that exactly one bead exists for the spec after the call.
11. **`mindspec doctor --dry-run-migration` flag.** New boolean
    flag on `mindspec doctor`. When set, the doctor:
    - Walks `.mindspec/specs/*/` (top-level entries only).
    - For each spec ID found, calls `phase.FindEpicBySpecID(specID)`.
    - For each epic, checks for `mindspec_phase` in metadata.
    - For each epic *lacking* `mindspec_phase`, reads children via
      `queryChildren` (no cache needed here; doctor is cold), calls
      `DerivePhaseFromChildren(children)` to compute the derived
      phase, and prints one line per legacy spec:
      `would-migrate: spec=<id> epic=<id> phase=<derived>`.
    - Writes NOTHING. The metadata is not touched; no
      `mindspec_migrated_at` is stamped. The flag is a pure
      reporter.
    - Exit code is `0` if all walks succeed (regardless of how
      many legacy specs exist), non-zero only on walk error.
12. **`mindspec_migrated_at` metadata field.** RFC3339 UTC
    timestamp written by `EnsureMigrated` alongside
    `mindspec_phase`. The field's presence is the durable marker
    that a migration ran; its absence (combined with the presence
    of `mindspec_phase`) indicates a post-080 native epic.
    Operators can grep epic metadata for `mindspec_migrated_at` to
    list specs that were migrated from the legacy 7-bead layout.
13. **Ceremony children are NOT deleted.** Legacy 7-bead specs
    retain their child beads after migration. Post-migration, the
    children are ignored by `DerivePhase` (because
    `readStoredPhaseWithCache` short-circuits with the now-present
    `mindspec_phase`). Operators may close, ignore, or manually
    delete the children at their discretion; mindspec does not
    touch them. This preserves the back-compat property in the
    plan's risk note: "Legacy specs that operators do not touch
    never trigger migration. Acceptable — they continue working
    via the metadata-or-children-derived dual read."
14. **`DerivePhaseFromChildren` fallback retained.** The
    children-derived fallback at `internal/phase/derive.go:141`
    and `:144` is preserved as the last-resort back-stop for
    legacy epics that have never had a lifecycle command run
    against them (so `EnsureMigrated` has never fired). Removal of
    this fallback is a future-major-version concern, explicitly
    out of scope (see Scope: Out).
15. **ADR-0034 drafted with `Domain(s): workflow`.** ADR-0034
    must list `workflow` in its `Domain(s)` field so the spec 087
    plan-time cite-relevant gate passes against this spec's single
    impacted domain. The ADR records the decisions listed in the
    `## ADR Touchpoints` block above.

## Scope

### In Scope

- New helper `phase.EnsureMigrated(specID string) (migrated bool,
  err error)` per Requirement 8.
- Call sites at `approve plan`, `approve impl`, `complete` per
  Requirement 9.
- `mindspec doctor --dry-run-migration` flag per Requirement 11.
- `mindspec_migrated_at` RFC3339 UTC metadata marker per
  Requirement 12.
- Audit of vestigial molecule scaffolding (grep + delete if
  found) per Requirement 7 — synthesis-time grep confirmed
  these symbols are already absent, so this collapses to a
  regression-protection test (`TestNoMoleculeScaffoldingSymbols`)
  unless the audit at implementation time finds residue.
- ADR-0034 drafted with `Domain(s): workflow` per Requirement 15.
- Preservation of `DerivePhaseFromChildren` as the back-stop
  fallback per Requirement 14.
- Preservation of all existing tests (the seven-bead fixtures in
  `complete_test.go` and `derive_test.go` continue to pass
  unchanged) per HC-3.

### Out of Scope

- **Deleting ceremony children.** Legacy 7-bead specs retain
  their child beads indefinitely for back-compat. Mindspec does
  not delete them; operators may, manually, at their discretion.
- **Removing the children-derived fallback.**
  `DerivePhaseFromChildren` and the
  `deriveFromChildrenOrStatusWithCache` path remain as the
  last-resort back-stop for untouched legacy epics. Removal is a
  future-major-version concern.
- **Forced bulk migration.** There is no `mindspec migrate-all`
  command. Migration is strictly first-touch; the
  `--dry-run-migration` flag reports the migration set but never
  writes.
- **Migration rollback.** Once `mindspec_phase` and
  `mindspec_migrated_at` are written, there is no
  unmigrate-this-epic command. Operators may manually clear the
  metadata via `bd update --metadata` if needed; this is not a
  supported flow.
- **Restructuring `phase.DerivePhase`'s public signature.** The
  existing exported surfaces (`DerivePhase`, `DerivePhaseWithCache`,
  `DerivePhaseWithStatus`, `DerivePhaseWithStatusWithCache`,
  `DerivePhaseFromChildren`) are all preserved verbatim.
  `EnsureMigrated` is purely additive.
- **A new ADR for retaining the children-derived fallback.**
  ADR-0034 covers this; no separate ADR is drafted.
- **`viz/agentmind/bench` inclusion under any flag.** HC-4 is
  absolute.

## Acceptance Criteria

- [ ] `TestApproveSpecCreatesSingleBead` passes: running
  `approve.SpecApprove(<new-spec-id>, ...)` against a fixture with
  no pre-existing epic results in exactly one bead existing for
  that spec (a `bd list --type=epic` filtered by the spec's
  metadata returns one row), and that bead has
  `metadata.mindspec_phase == "plan"`.
- [ ] `TestPhaseDerivationFromMetadataOnly` passes: an epic
  fixture with `mindspec_phase: plan` set in metadata and **zero**
  child beads has `phase.DerivePhase(epicID)` return `"plan"`
  without inspecting children (asserted via a test seam that
  fails the test if `queryChildren` is called).
- [ ] `TestLegacyMigratesOnFirstCommand` passes: a legacy 7-bead
  spec fixture (epic exists, lacks `mindspec_phase`, has seven
  child beads in the historical layout) has its epic metadata
  updated to include both `mindspec_phase` and
  `mindspec_migrated_at` after the first call to
  `approve.PlanApprove(<specID>, ...)`. A subsequent call to
  `phase.DerivePhase(epicID)` ignores the seven children (asserted
  via the no-`queryChildren`-call seam) and returns the migrated
  phase value.
- [ ] `TestDoctorDryRunMigrationReports` passes: running
  `doctor.Run(opts{DryRunMigration: true})` against a fixture
  containing one legacy spec and one post-080 native spec produces
  output containing the line `would-migrate: spec=<legacy-spec-id>
  epic=<legacy-epic-id> phase=<derived>` and does NOT contain a
  `would-migrate` line for the native spec. The legacy epic's
  metadata is **unchanged** after the run (asserted by re-reading
  metadata and confirming `mindspec_phase` is still absent and
  `mindspec_migrated_at` is still absent).
- [ ] `TestMigratedEpicHasMigratedAtMarker` passes: after the
  first call to `phase.EnsureMigrated(<legacy-spec-id>)` against a
  legacy fixture, the epic's metadata contains
  `mindspec_migrated_at` as an RFC3339 UTC string parseable by
  `time.Parse(time.RFC3339, ...)`.
- [ ] `TestEnsureMigratedIdempotent` passes: calling
  `phase.EnsureMigrated(<specID>)` twice in a row against the
  same legacy fixture returns `(true, nil)` the first time and
  `(false, nil)` the second time. The `mindspec_migrated_at`
  value is the timestamp from the first call (i.e., not
  overwritten on the second call).
- [ ] `TestEnsureMigratedNoEpicReturnsFalseNil` passes: calling
  `phase.EnsureMigrated(<spec-id-with-no-epic>)` returns
  `(false, nil)` — `approve spec` is the creator and
  `EnsureMigrated` is a no-op pre-creation.
- [ ] `TestNoMoleculeScaffoldingSymbols` passes: a `grep` across
  `internal/` for the literal identifiers `mol.pour`,
  `closeoutTargets`, and `EnsureFullyBound` returns zero matches.
  Synthesis-time grep confirmed these symbols are already absent;
  this test prevents regression.
- [ ] `TestDerivePhaseFromChildrenStillPasses` (the existing
  `TestDerivePhaseFromChildren` in
  `internal/phase/derive_test.go`) continues to pass unchanged
  because `DerivePhaseFromChildren` is preserved as the
  migration's derivation step and as the back-stop fallback.
- [ ] `TestCompleteUnchangedOnPostMigrationFixture` passes: the
  existing `complete_test.go` fixtures (one closed + one open
  child → `implement`, etc.) continue to pass after a prior
  `EnsureMigrated` call writes `mindspec_phase`. The complete
  flow's output is unchanged because the post-migration metadata
  agrees with the children-derived value (by construction —
  `EnsureMigrated` writes the children-derived value into
  metadata).
- [ ] `ADR-0034-ceremony-collapse.md` exists under
  `.mindspec/adr/`, has `Status: Accepted`, lists
  `Domain(s): workflow` so the spec 087 plan-time cite-relevant
  gate passes, and records: single-bead-epic decision,
  `mindspec_phase` as authoritative source, auto-migration on
  first lifecycle command (no opt-in), `mindspec_migrated_at`
  marker, ceremony-children retention, `DerivePhaseFromChildren`
  fallback retention.
- [ ] All existing tests still pass; AST boundary lint from
  spec 085 (`internal/lint/boundary_test.go`) stays green.
- [ ] `go build ./... && go test -short ./...` is green on every
  commit of the F5 branch.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-05-21
- **Notes**: Approved via mindspec approve spec