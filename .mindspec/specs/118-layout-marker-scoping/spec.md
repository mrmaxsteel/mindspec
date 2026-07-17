---
approved_at: "2026-07-17T19:21:50Z"
approved_by: user
status: Approved
---

# Layout Marker Scoping

## Goal

Prevent flat-layout repositories from being permanently misclassified as mixed merely because an ordinary root `docs/` directory or a non-lifecycle `.mindspec/docs/` directory exists, while preserving detection of genuine canonical, legacy, and mixed lifecycle trees. Restore a clean `doctor` result after a completed `migrate layout` run, and keep git-ref merge-guard classification in exact semantic parity with filesystem classification for flat, canonical, and legacy markers, including each tier's `context-map.md` marker.

## Background

`internal/workspace/workspace.go` currently derives canonical and legacy markers from bare directory existence: `.mindspec/docs` marks canonical and `{root}/docs` marks legacy. Flat layout detection is narrower: it requires at least one lifecycle child named `specs`, `adr`, `domains`, or `core`, or the flat `context-map.md` file. Because `ClassifyLayout` treats a flat tree plus either broad marker as mixed, a valid flattened repository is wedged by a common, ordinary root `docs/` directory or by an empty/leftover/tracked `.mindspec/docs/` directory.

This wedge is operational, not cosmetic. In mixed state, the new-spec default in `SpecDir` does not take its flat-layout path and writes a new spec beneath `.mindspec/docs/specs/{slug}`, while reads prefer the flat tree. The result is split-brain placement in which newly created specs are orphaned from the active lifecycle. There is no configuration override. This failure occurred in a live downstream repository after a v0.11.0 flatten.

Separately, `internal/doctor/migration.go` retains `checkMigrationMetadata`, a check from the removed spec-036 classify pipeline. It becomes armed by `docs_archive/` or `.mindspec/lineage/manifest.json`; a legitimate layout migration writes the lineage manifest. The check then demands obsolete inventory, classification, extraction, plan, validation, and apply artifacts plus `plan.md` and `docs_archive`, reports them missing, and suggests the nonexistent `migrate apply` command. A completed layout migration can therefore leave `doctor` exiting 1, and `doctor --fix` cannot repair the result.

The merge guard has a second classification path. `internal/executor/layout_guard.go:31-40` lists only immediate children of `.mindspec` at a git ref and passes those names to `workspace.LayoutMarkersFromMindspecChildren`. Consequently, the mere child name `docs` marks canonical without inspecting `.mindspec/docs`; legacy is separately derived from bare root `docs` existence, and that probe is skipped whenever flat or canonical is already set. Both behaviors disagree with lifecycle-shaped filesystem classification and can either false-block an ordinary documentation tree or hide a genuine mixed tree. The guard is the only production caller of that helper and exists to block bead-to-spec and local spec-to-main regressions. Its semantics must change with the filesystem rule rather than drift from it.

## Impacted Domains

- **core**: `internal/workspace/**` — filesystem layout markers, artifact-tier resolution, and `SpecDir` selection.
- **execution**: `internal/executor/**` — git-ref layout classification and the bead-to-spec/local spec-to-main merge guard.
- **workflow**: `internal/doctor/**` migration-metadata health checks and the `internal/layout/**` mover/run-state metadata contract they validate.

These declarations match the current ownership globs exactly: `internal/workspace/**` is owned by `core`, `internal/executor/**` by `execution`, and both `internal/doctor/**` and `internal/layout/**` by `workflow`. No source file outside those owned paths is expected to change; the ADR-0039 documentation amendment identified below is additionally required.

## ADR Touchpoints

- [ADR-0039: Flat Layout v2](../../adr/ADR-0039-flat-layout-v2.md) defines the flat lifecycle layout whose marker resolution this specification corrects. As part of this specification's implementation, ADR-0039 **WILL BE AMENDED** to add `docs/context-map.md` to Decision §2's legacy tier and cite the executable `ContextMapPath`/`resolveArtifact` fallback contract. This is a documentation-consistency amendment only: it reconciles the Decision §2 text with existing resolver behavior and does not change that resolver behavior. The implementation MUST include this ADR amendment so the ADR-divergence gate sees the declared touchpoint.

## Requirements

1. **Resolver-shaped canonical marker.** Filesystem layout detection MUST set the canonical marker when `.mindspec/docs` directly contains at least one lifecycle **directory** named `specs`, `adr`, `domains`, or `core`, OR when `.mindspec/docs/context-map.md` is a regular file. A named lifecycle child directory marks canonical even when that child is empty. Bare wrapper existence, an empty wrapper, unrelated children, a lifecycle name nested below a non-lifecycle child, and `.mindspec/docs` existing as a regular file MUST NOT set the marker or panic.
2. **Resolver-shaped legacy marker.** Filesystem layout detection MUST set the legacy marker when `{root}/docs` directly contains at least one lifecycle **directory** named `specs`, `adr`, `domains`, or `core`, OR when `{root}/docs/context-map.md` is a regular file. This context-map rule follows the actual `ContextMapPath`/`resolveArtifact` contract, whose documented and executable fallback includes `docs/context-map.md`; ADR-0039 Decision §2's currently asymmetrical legacy-tier text MUST be reconciled through the documentation-consistency amendment declared in ADR Touchpoints. A named lifecycle child directory marks legacy even when empty. Bare wrapper existence, an empty wrapper, unrelated documentation children, a lifecycle name nested below a non-lifecycle child, and root `docs` existing as a regular file MUST NOT set the marker or panic.
3. **Shared direct-child and file-type semantics.** Flat, canonical, and legacy detection MUST use one lifecycle-name vocabulary (`specs`, `adr`, `domains`, `core`) and immediate-child-only semantics. Lifecycle markers MUST test that the named child is a directory (`IsDir` or the git-tree equivalent), not merely that its path exists; `context-map.md` markers MUST test that the exact tier path is a file, not a directory. `.mindspec/context-map.md` marks only flat, `.mindspec/docs/context-map.md` marks canonical, and `docs/context-map.md` marks legacy. Therefore a flat marker coexisting with `.mindspec/docs/context-map.md` or `docs/context-map.md` is a genuine incomplete-flatten mixed tree, not the ordinary-`docs` wedge addressed by this specification.
4. **Classification preservation.** Narrowing marker detection MUST preserve genuine canonical and legacy classifications and MUST continue to classify as mixed any flat lifecycle tree that coexists with a lifecycle-shaped canonical or legacy tree.
5. **Flat write-path recovery without write-default changes.** Once a repository with a valid flat lifecycle tree and only non-lifecycle `docs` directories resolves as flat, the existing `SpecDir` new-spec behavior MUST select `.mindspec/specs/{slug}`. The write-default branching itself MUST NOT be changed.
6. **Doctor metadata rekey.** `checkMigrationMetadata` MUST remain active and MUST be rekeyed to the current `migrate layout` contract. Evidence of a current run—either `.mindspec/lineage/manifest.json` or a per-run `state.json`/`lineage.json` under `.mindspec/migrations/<run-id>/`—MUST arm the check, so deleting the global manifest from an otherwise completed run cannot disable validation. Doctor MUST require the global manifest to be parseable with a non-empty `run_id` and non-empty `entries`; MUST use that `run_id` to require `.mindspec/migrations/<run-id>/lineage.json` to be parseable under the current lineage schema with the same non-empty `run_id` (and non-empty entries); and MUST require `.mindspec/migrations/<run-id>/state.json` to be parseable with stage exactly `applied` before reporting a healthy completed run. Missing or malformed required current artifacts MUST remain Error/Missing findings and make doctor exit nonzero. A parseable state with a non-empty, non-`applied` stage MUST preserve the existing Warn/non-fatal, exit-0 severity because it can represent a legitimately in-progress run; it MUST NOT be reported as a healthy completed/applied run and MUST NOT be escalated to Error by this specification. The rekey MUST drop only the `docs_archive/` expectation, the seven obsolete artifacts `inventory.json`, `classification.json`, `extraction.json`, `plan.json`, `plan.md`, `validation.json`, and `apply.json`, and every `migrate apply` hint. A valid completed current run MUST not fail merely because those obsolete artifacts are absent.
7. **Doctor signal preservation.** The migration-metadata check MUST NOT be deleted or reduced to a no-op. Its current-contract manifest, per-run lineage, and applied-state validation, plus unrelated doctor checks and genuine errors, MUST retain Error/Missing severity and nonzero exit behavior where applicable.
8. **Full git-ref merge-guard parity.** `layoutAtRef` MUST derive every marker independently, regardless of whether another marker is already set. It MUST inspect direct tree children of both `.mindspec/docs` and root `docs` (for example with `TreeDirsAtRef`) and apply the same lifecycle-directory predicate as Requirements 1-3; bare `pathExistsAtRef(ref, "docs")` is insufficient. It MUST also use file-type-aware ref probes for `.mindspec/context-map.md`, `.mindspec/docs/context-map.md`, and `docs/context-map.md`. Thus an ordinary root `docs/` at a ref does not set Legacy, while root `docs/<lifecycle>/` coexisting with a flat marker remains represented as mixed. A mixed source ref merged onto a flat target MUST be treated as a layout regression and blocked rather than passing through the directional guard. The `docs`-child-name → Canonical shortcut in `LayoutMarkersFromMindspecChildren` is superseded: implementation MUST either re-signature that helper with the descended canonical/legacy evidence or stop relying on its canonical output. Filesystem and git-ref tests MUST demonstrate the same result for every equivalent fixture.
9. **Focused regression coverage.** Automated table-driven tests MUST cover every behavioral row in the acceptance resolve matrix, including the prior false-positive cases, all genuine-layout no-regression cases, the recovered `SpecDir` write path, current migration doctor behavior, and git-ref merge-guard parity. The ADR documentation row MUST have the runnable validation proof specified below.

## Scope

### In Scope

- Narrow canonical and legacy filesystem markers to direct lifecycle children or their resolver-supported `context-map.md` file.
- Preserve flat, canonical, legacy, and true-mixed classification behavior under the narrowed rules.
- Rekey the doctor migration-metadata check to the current layout-migration manifest, per-run lineage, and applied state.
- Update the git-ref merge guard to inspect both wrapper trees and all three context-map paths with filesystem-equivalent semantics.
- Add focused table-driven unit/integration tests for the complete resolve matrix.
- Update ADR-0039 Decision §2 to document the existing legacy `docs/context-map.md` resolver fallback and cite `ContextMapPath`/`resolveArtifact`, without changing resolver behavior.
- Update ALL fossil-behavior table rows that pin the superseded bare-`docs`-child or bare-wrapper classification, including but not limited to the existing `LayoutMarkersFromMindspecChildren` row `{"canonical docs child", []string{"docs"}, LayoutMarkers{Canonical: true}}`; updating only that named row is insufficient.

### Out of Scope

- **FOLLOWUP-C — Published-State Abort Safety:** arm `State.Published` and refuse rollback/abort once the migration reaches the `applied` stage.
- **FOLLOWUP-D — Mixed-Source Mover Hardening:** make the mover fail loudly on mixed input and define handling for non-plan children under `.mindspec/docs`.
- **FOLLOWUP-A3 — Versioned Schema Layout Marker:** introduce `.mindspec/schema.json` as a versioned layout footprint.
- Changing the `SpecDir` write-default decision logic.
- Changing layout-migration run-state or lineage production beyond making the doctor consumer validate the artifacts already written by the mover.

## Non-Goals

- Removing support for canonical or legacy layouts.
- Treating every directory named `docs` as a MindSpec lifecycle tree.
- Relaxing detection of genuinely mixed lifecycle layouts.
- Changing flat → canonical → legacy artifact read precedence. In particular, this specification intentionally narrows whole-tree markers without changing `DocsDir`'s existing canonical-wrapper precedence over legacy reads.
- Removing the actual resolver's legacy `docs/context-map.md` fallback merely because ADR-0039's current legacy-tier bullet omits that file. The decision here follows `ContextMapPath`/`resolveArtifact`; the ADR text is reconciled via the documentation-consistency amendment declared above.
- Eliminating the narrow residual ambiguity in which an ordinary DDD-style root `docs/` tree that itself contains a regular `docs/context-map.md` classifies as `legacy`. This is an accepted trade-off of following the executable resolver fallback; an ordinary `docs/` tree without that exact file remains non-lifecycle.
- Redesigning migration, rollback, publishing, or artifact production.
- Introducing a new configured layout override or schema/version marker in this change.
- Repairing or moving already split-brain specs; this change prevents new misplacement by restoring correct classification.

## Acceptance Criteria

The following resolve matrix is mandatory. Each behavioral row MUST be represented by an automated test whose setup creates exactly the stated relevant tree shape and whose assertion checks the stated result. The ADR documentation row MUST be covered by the runnable validation proof stated below.

- [ ] **AC-1** — Flat lifecycle directory under `.mindspec` plus an ordinary root `docs/` with no direct lifecycle directory and no `context-map.md` file → Layout resolves to `flat`. _(Filesystem legacy wedge fix; RED on revert)_
- [ ] **AC-2** — Flat lifecycle directory under `.mindspec` plus empty/leftover `.mindspec/docs/` with no direct lifecycle directory and no `context-map.md` file → Layout resolves to `flat`. _(Filesystem canonical wedge fix; RED on revert)_
- [ ] **AC-3** — No flat marker; root `docs/` directly contains an empty lifecycle directory → Layout resolves to `legacy`. _(Genuine legacy and empty-child no-regression)_
- [ ] **AC-4** — No flat marker; `.mindspec/docs/` directly contains an empty lifecycle directory → Layout resolves to `canonical`. _(Genuine canonical and empty-child no-regression)_
- [ ] **AC-5** — Flat lifecycle directory and `.mindspec/docs/specs/` both exist → Layout resolves to `mixed` and `DetectLayout` returns the applicable layout error. _(True-mixed no-regression)_
- [ ] **AC-6** — Flat lifecycle directory and root `docs/specs/` both exist → Layout resolves to `mixed` and `DetectLayout` returns the applicable layout error. _(Flat-plus-legacy no-regression)_
- [ ] **AC-7** — Doctor runs on a completed-migration fixture containing a parseable, non-empty `.mindspec/lineage/manifest.json` whose `run_id` matches a parseable, non-empty `.mindspec/migrations/<run-id>/lineage.json`, plus a parseable `.mindspec/migrations/<run-id>/state.json` with stage `applied`; the fixture omits `docs_archive/` and all seven obsolete artifacts → Doctor exits `0`; current artifacts are reported healthy; no obsolete-artifact finding or `migrate apply` hint is emitted. _(Doctor rekey; RED on revert)_
- [ ] **AC-7b** — For the AC-7 completed-migration fixture, table-driven mutations independently make the global manifest malformed, make `state.json` unparseable, make the per-run `lineage.json` malformed, mismatch the per-run `lineage.json` `run_id` against the global manifest, and remove each required current artifact (global manifest, per-run lineage, or state) → Every mutation emits an Error or Missing finding and doctor exits nonzero. _(Proves current validation, including lineage schema and identity validation, remains active rather than becoming a no-op)_
- [ ] **AC-8** — A new spec slug is resolved in the AC-1 un-wedged flat repository → `SpecDir` returns `.mindspec/specs/{slug}`. _(Split-brain prevention through classification recovery)_
- [ ] **AC-9** — At a real git ref, `.mindspec/docs/` contains only an unrelated tracked file (so the wrapper is representable in Git), with no direct lifecycle directory and no canonical context-map file → Ref markers do not set Canonical, matching the equivalent filesystem fixture. _(Git-ref canonical parity; RED on revert)_
- [ ] **AC-10** — At a real git ref, a flat lifecycle directory and `.mindspec/docs/specs/` both exist → Ref classification is `mixed` and the applicable local merge is blocked. _(Canonical mixed-tree guard safety)_
- [ ] **AC-11** — At a real git ref, an ordinary root `docs/` contains only an unrelated tracked file, with no direct lifecycle directory and no legacy context-map file → Ref markers do not set Legacy; a flat ref remains `flat` and is not false-blocked. _(Git-ref legacy wedge fix; RED on revert)_
- [ ] **AC-12** — At a real git ref, a flat lifecycle directory and root `docs/specs/` both exist → Ref classification is `mixed` and the applicable local merge is blocked. _(Legacy mixed-tree guard safety despite Flat already being set)_
- [ ] **AC-13** — No other marker exists and only `.mindspec/docs/context-map.md` is a regular file → Layout resolves to `canonical`. _(ADR-0039 canonical context-map tier)_
- [ ] **AC-14** — No other marker exists and only root `docs/context-map.md` is a regular file → Layout resolves to `legacy` and `ContextMapPath` resolves that file. _(Actual legacy resolver contract)_
- [ ] **AC-15** — A flat marker coexists with `.mindspec/docs/context-map.md` → Layout resolves to `mixed` and the applicable layout error remains observable. _(Incomplete flatten is not mistaken for the wedge)_
- [ ] **AC-16** — Equivalent real-git-ref fixtures contain, in turn, only `.mindspec/context-map.md`, only `.mindspec/docs/context-map.md`, and only `docs/context-map.md` → Ref classification is respectively `flat`, `canonical`, and `legacy`, matching filesystem classification. _(File-marker git-ref parity)_
- [ ] **AC-17** — In separate variants, `.mindspec/docs/sub/specs/` and root `docs/sub/specs/` exist but the applicable wrapper has no direct lifecycle child and no context-map file → The wrapper sets no canonical/legacy marker. _(Direct-child-only guard against recursive walks in both tiers)_
- [ ] **AC-18** — Root `docs` and, separately, `.mindspec/docs` exist as regular files rather than directories → Neither fixture sets a canonical/legacy marker and neither panics. _(File-vs-directory safety)_
- [ ] **AC-19** — In separate variants, `.mindspec/context-map.md`, `.mindspec/docs/context-map.md`, and `docs/context-map.md` exist as directories rather than regular files → The respective flat/canonical/legacy marker is not set and detection does not panic. _(Context-map file-type enforcement)_
- [ ] **AC-20** — In separate variants for flat, canonical, and legacy tiers, an immediate lifecycle name such as `specs` exists as a regular file rather than a directory → The respective marker is not set and detection does not panic. _(Lifecycle `IsDir` enforcement)_
- [ ] **AC-21** — A flat marker coexists with root `docs/context-map.md` as a regular file → Layout resolves to `mixed` and `DetectLayout` returns the applicable layout error. _(Legacy-tier incomplete flatten mirrors AC-15 and cannot be hidden by Flat)_
- [ ] **AC-22** — At a real git ref, a flat lifecycle directory and root `docs/context-map.md` both exist → Ref classification is `mixed` and the applicable local merge is blocked. _(Independent legacy context-map ref derivation despite Flat already being set)_
- [ ] **AC-23** — At real git refs in separate variants, `.mindspec/context-map.md`, `.mindspec/docs/context-map.md`, and `docs/context-map.md` are each committed as a directory/tree rather than a regular file/blob → The respective flat/canonical/legacy marker is not set at the ref, and ref classification does not error. _(Forces file-type-aware git-ref probes rather than bare path existence)_
- [ ] **AC-24** — For an otherwise valid AC-7 current-run fixture, `state.json` is parseable with a non-empty stage `finalize` rather than `applied` → Doctor emits a Warn/non-fatal finding for the stage, exits `0`, does not report the run healthy/completed/applied, and does not emit an Error for that stage. _(Pins existing in-progress stage severity without expanding into FOLLOWUP-C/D)_
- [ ] **AC-25** — ADR-0039 Decision §2 is amended during implementation to include legacy `docs/context-map.md` and explicitly cite the existing `ContextMapPath`/`resolveArtifact` resolver fallback → The ADR amendment is present and describes a documentation-consistency reconciliation with no resolver behavior change. _(Prevents an undeclared ADR-divergence gate surprise)_

Additional falsifiable constraints:

- The filesystem classification tests for AC-1 through AC-6 and AC-13 through AC-21 MUST be table-driven. AC-7b and AC-24 MUST be table-driven doctor mutations. Across the layout table all four lifecycle names MUST be exercised, including at least one empty lifecycle directory, so a predicate that recognizes only `specs`, accepts regular files, or requires directory contents cannot pass.
- AC-9 through AC-12, AC-16, AC-22, and AC-23 MUST exercise the production merge-guard path or its git-tree marker resolver with real git-ref tree fixtures; a test of filesystem detection alone does not satisfy them. The blocked assertions in AC-10, AC-12, and AC-22 MUST reach the applicable local merge guard, not merely assert `ClassifyLayout` in isolation.
- Tests MUST fail if canonical or legacy detection is reverted to bare directory existence.

## Validation Proofs

The implementation review MUST record successful output from concrete commands equivalent to:

```bash
go test ./internal/workspace/... -run 'Test.*(Layout|SpecDir|Marker|Resolve)'
go test ./internal/doctor/... -run 'Test.*(Migration|Doctor|Layout)'
go test ./internal/executor/... -run 'Test(GuardMergeLayout_Directional|LayoutAtRef_ClassifiesRealBranches|.*(GuardMergeLayout|LayoutAtRef).*)'
go test ./internal/workspace/... ./internal/doctor/... ./internal/executor/...
rg -n 'docs/context-map\.md|ContextMapPath|resolveArtifact' .mindspec/adr/ADR-0039-flat-layout-v2.md
```

The named focused tests MUST map their subtest names or review evidence to every behavioral row from AC-1 through AC-24, including AC-7b; the `rg` proof MUST demonstrate AC-25. The executor expression intentionally matches the existing `TestGuardMergeLayout_Directional` and `TestLayoutAtRef_ClassifiesRealBranches` tests as well as new tests carrying either stem. If the repository's final test names differ from these regular expressions, review evidence MUST include the exact runnable `go test {package} -run {test}` commands that execute every behavioral matrix row. Passing a broad package command, or a focused `-run` expression that matches zero tests, is insufficient.

## Open Questions

None. Merge-guard drift is resolved by Requirement 8: the git-ref path independently derives all three markers from direct wrapper children and tier-specific context-map files. The legacy-context-map question is resolved against the actual resolver: `ContextMapPath` documents `.mindspec/context-map.md` → `.mindspec/docs/context-map.md` → `docs/context-map.md`, and `resolveArtifact` supplies that legacy fallback, so `docs/context-map.md` marks legacy here; ADR-0039's currently omitted legacy file is reconciled through the implementation-time amendment declared in ADR Touchpoints and AC-25.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-07-17
- **Notes**: Approved via mindspec approve spec