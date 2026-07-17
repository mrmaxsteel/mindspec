---
adr_citations:
    - ADR-0039
approved_at: "2026-07-17T19:45:20Z"
approved_by: user
bead_ids:
    - mindspec-qqv1.1
    - mindspec-qqv1.2
    - mindspec-qqv1.3
spec_id: 118-layout-marker-scoping
status: Approved
version: "1"
---
# Plan: 118-layout-marker-scoping

## ADR Fitness

ADR-0039 remains the best architectural choice for this work. Its flat → canonical → legacy per-artifact resolver, whole-tree mixed-layout protection, and permanent multi-prefix git-ref posture are the contracts this plan preserves. The implementation corrects marker resolution so filesystem and git-ref classification match ADR-0039's own three-tier artifact footprint: each tier is marked only by an immediate lifecycle directory or that tier's regular `context-map.md` file.

ADR-0039 Decision §2 currently lists `docs/{specs,adr,domains,core}` for the legacy tier but omits the already-executable `docs/context-map.md` fallback. Bead 3 therefore amends Decision §2 to name `docs/context-map.md` and explicitly cite `ContextMapPath`/`resolveArtifact`. This is a documentation-consistency reconciliation with the existing resolver, not design divergence and not a resolver behavior change. ADR-0039 is not superseded, and no other ADR is superseded by this plan.

## Testing Strategy

Testing is organized around the acceptance matrix and uses table/subtest names that remain addressable with focused `go test -run` proofs.

- `internal/workspace`: table-driven filesystem classification tests create exact flat, canonical, legacy, mixed, nested-child, wrapper-as-file, lifecycle-child-as-file, and context-map-as-directory fixtures. The table exercises all four shared lifecycle names, immediate-child-only behavior, `IsDir`, regular-file context maps, `DetectLayout` errors, resolver behavior, and `SpecDir` recovery without changing its write-default branch.
- `internal/executor`: integration tests commit fixtures to real git refs and exercise the production `layoutAtRef`/local merge-guard path through `TreeDirsAtRef` plus a small type-aware ref seam backed by `git ls-tree <ref> -- <path>` (or `git cat-file -t`) that requires the entry type to be `blob`; `FileAtRef`/`git show` is not a type test because it also succeeds for trees. Tests distinguish git trees from blobs for every `context-map.md` tier, compare ref results with equivalent filesystem fixtures, and prove mixed-source → flat-target blocking.
- `internal/doctor` and `cmd/mindspec`: table-driven unit tests mutate current migration artifacts one at a time and assert finding status/health semantics; command-level tests assert exit code and rendered output, including absence of obsolete artifacts and `migrate apply` hints.
- Focused regressions follow RED-on-revert discipline: reverting either canonical or legacy detection to bare wrapper existence, restoring conditional marker derivation, accepting a tree as a context-map blob, or restoring the obsolete doctor contract must make its focused proof fail. Each AC below maps to a named `go test -run` command (or the ADR `rg` proof), and broad package tests supplement rather than replace those proofs.

The work is split into three beads because the filesystem predicate is the reusable semantic foundation, the executor has a real state dependency on that predicate, and doctor/ADR reconciliation can proceed independently. The only dependency edge is Bead 1 → Bead 2, so the longest serial chain is two beads while Bead 3 remains parallelizable.

## Bead 1: Scope filesystem layout markers to resolver-shaped artifacts

**Domain:** core (`internal/workspace/**`)

**Steps**

1. Replace the private flat-only lifecycle-name list with one shared lifecycle-name predicate for `specs`, `adr`, `domains`, and `core`, usable by filesystem classification and later by git-ref classification.
2. Refactor filesystem marker probing so each tier independently recognizes only an immediate lifecycle child that is a directory, or its exact `context-map.md` path when that path is a regular file. Treat absent wrappers, empty wrappers, unrelated children, nested lifecycle names, wrapper regular files, lifecycle-name regular files, and context-map directories as non-markers without panics.
3. Preserve `ClassifyLayout`/`DetectLayout` behavior for genuine flat, canonical, legacy, and mixed trees, including mixed-layout errors for flat plus canonical/legacy context-map markers and the existing artifact read precedence.
4. Prove that corrected classification restores `SpecDir(root, newSlug)` to `.mindspec/specs/{slug}` in an otherwise-flat repository with ordinary documentation wrappers; do not alter the existing write-default branch.
5. Replace or update only the `internal/workspace` filesystem fossil rows that encode bare-wrapper or name-without-file-type marker behavior, and add the complete table-driven filesystem resolve matrix with all four lifecycle names represented.

**Verification**

- [ ] **B1-V1 (AC-1–AC-6, AC-13, AC-15, AC-17–AC-21):** `go test ./internal/workspace/... -run 'TestLayoutMarkerResolveMatrix'` passes the table-driven classification matrix, including applicable `DetectLayout` error assertions.
- [ ] **B1-V2 (AC-8):** `go test ./internal/workspace/... -run 'TestSpecDir_UnwedgedFlatNewSpec'` proves a new slug resolves under `.mindspec/specs/` without a write-default logic change.
- [ ] **B1-V3 (AC-14):** `go test ./internal/workspace/... -run 'TestContextMapPath_ThreeTierFallback'` proves the legacy regular-file marker and `ContextMapPath` fallback resolve `docs/context-map.md`.
- [ ] **B1-V4 (AC-13, AC-14, AC-19, AC-20):** `go test ./internal/workspace/... -run 'TestLayoutMarkerResolveMatrix/(canonical_context_map_file|legacy_context_map_file|.*context_map_directory|.*lifecycle_name_regular_file)'` passes and matches at least one subtest in every listed class.
- [ ] **B1-V5 (package regression):** `go test ./internal/workspace/...` passes, including updated fossil expectations.

**Acceptance Criteria**

- Filesystem canonical/legacy markers are set only by an immediate lifecycle directory (`specs`/`adr`/`domains`/`core`, `IsDir`) or the tier's regular `context-map.md` file; bare wrappers, empty wrappers, unrelated children, nested lifecycle names, wrapper-as-file, lifecycle-name-as-file, and context-map-as-directory set no marker and never panic (spec AC-1, AC-2, AC-3, AC-4, AC-13, AC-14, AC-17, AC-18, AC-19, AC-20).
- Genuine flat, canonical, legacy, and mixed classifications are preserved, including flat + canonical/legacy lifecycle-or-context-map coexistence resolving `mixed` with the `DetectLayout` error (spec AC-5, AC-6, AC-15, AC-21).
- An un-wedged flat repository resolves a new spec slug to `.mindspec/specs/{slug}` without changing the `SpecDir` write-default branch (spec AC-8).

**Depends on**

None.

## Bead 2: Bring real-git-ref merge guards into full marker parity

**Domain:** execution (`internal/executor/**`)

Bead 2's edit to the core-owned `internal/workspace/workspace.go` helper is gate-legal: `ValidateDivergence` attributes changed files against the spec's declared impacted domains `{core, execution, workflow}`, all covered by cited ADR-0039, rather than against this bead's single domain, so the cross-file edit cannot by itself trigger `adr-divergence-unowned`.

**Steps**

1. Refactor `layoutAtRef` to derive flat, canonical, and legacy independently. Descend direct directory children of both `.mindspec/docs` and root `docs` with `TreeDirsAtRef`, and reuse Bead 1's lifecycle-name predicate instead of treating wrapper existence or the `.mindspec` child name `docs` as a marker.
2. Probe `.mindspec/context-map.md`, `.mindspec/docs/context-map.md`, and `docs/context-map.md` with a type-aware ref operation: use `git ls-tree <ref> -- <path>` and require the returned entry `type == blob` (or equivalently use `git cat-file -t`). Do not use `FileAtRef`/`git show`, which exits successfully for a tree as well as a blob. Add the small executor git-layer seam needed for this result (for example, `BlobExistsAtRef` or an entry-type helper), so trees at those paths are non-markers without classification errors; AC-23 pins this implementation.
3. Remove the `!Flat && !Canonical` legacy gating so every ref marker is observable simultaneously. Own the complete supersession of `LayoutMarkersFromMindspecChildren` in this bead: revise or retire the helper code in `internal/workspace/workspace.go` (if retained, re-signature it around descended evidence rather than a bare `docs` child), update its own `internal/workspace` fossil row `{"canonical docs child", []string{"docs"}, LayoutMarkers{Canonical: true}}`, and update the executor integration assertions in `TestLayoutAtRef_ClassifiesRealBranches` plus all other bare-wrapper/bare-`docs` fossil rows. Land the helper, both fossil-test locations, and the `layoutAtRef` rewrite atomically so the one bead commit stays green.
4. Extend `mergeLayoutRegression` and the applicable local merge seams so a mixed source onto a flat target is blocked alongside canonical/legacy regressions, while preserving migration-direction, same-layout, recovery, and documented fail-open behavior outside this change.
5. Expand real-repository tests to commit equivalent filesystem/ref fixtures for unrelated wrappers, lifecycle directories, all three context-map blobs, and all three context-map trees; assert parity and reach the local guard for every blocked case.

**Verification**

- [ ] **B2-V1 (AC-9, AC-11, AC-16, AC-23):** `go test ./internal/executor/... -run 'TestLayoutAtRef_ClassifiesRealBranches'` passes real-git-ref parity rows for unrelated wrappers, three context-map blobs, and three context-map trees, proving the type-aware probe requires `type == blob` rather than treating successful `FileAtRef`/`git show` output as a file test.
- [ ] **B2-V2 (AC-10, AC-12, AC-22):** `go test ./internal/executor/... -run 'TestGuardMergeLayout_(CanonicalMixedSource|LegacyMixedSource|LegacyContextMapMixedSource)'` reaches the applicable local merge guard and blocks each mixed source onto a flat target.
- [ ] **B2-V3 (AC-10, AC-12, AC-22):** `go test ./internal/executor/... -run 'TestMergeLayoutRegression_Matrix|TestGuardMergeLayout_Directional'` proves mixed → flat is a regression while the preserved directional rows still pass.
- [ ] **B2-V4 (filesystem/ref equivalence):** `go test ./internal/executor/... -run 'TestLayoutAtRef_FilesystemParity'` compares every equivalent fixture through real refs and filesystem classification, including blob-versus-tree context-map variants through the type-aware ref seam.
- [ ] **B2-V5 (package regression):** `go test ./internal/executor/...` passes, including all updated superseded fossil rows.

**Acceptance Criteria**

- At real git refs, `layoutAtRef` derives every marker independently (no `!Flat && !Canonical` gating) by descending both `.mindspec/docs` and root `docs` with the shared lifecycle predicate, so an ordinary wrapper does not set Canonical/Legacy and does not false-block a flat ref (spec AC-9, AC-11).
- Context-map markers at a ref use a type-aware `blob` probe: the three `context-map.md` paths classify `flat`/`canonical`/`legacy` as blobs and set no marker when committed as trees, matching filesystem classification (spec AC-16, AC-23).
- A flat source coexisting with a canonical or legacy lifecycle directory or `docs/context-map.md` at a ref is `mixed` and is blocked by the local merge guard (spec AC-10, AC-12, AC-22).

**Depends on**

Bead 1. The git-ref resolver reuses Bead 1's exported/shared lifecycle-name predicate, so this is a real state dependency rather than ordering convenience.

## Bead 3: Rekey doctor migration health and reconcile ADR-0039

**Domain:** workflow (`internal/doctor/**`, command-level doctor tests in `cmd/mindspec/**`, and `.mindspec/adr/ADR-0039-flat-layout-v2.md`)

**Steps**

1. Rekey `checkMigrationMetadata` activation to current-run evidence: the global `.mindspec/lineage/manifest.json` or any per-run `state.json`/`lineage.json` under `.mindspec/migrations/<run-id>/`, so removing the global manifest does not silence validation of an otherwise-present run.
2. Validate the global manifest as parseable with non-empty `run_id` and entries; use its run ID to require parseable, non-empty per-run `lineage.json` under the current lineage schema with the same `run_id`, plus parseable `state.json` whose healthy completed stage is exactly `applied`.
3. Preserve Error/Missing findings and nonzero command exit for missing, malformed, empty, or identity-mismatched required current artifacts. Preserve Warn/non-fatal exit-0 behavior for a parseable non-empty non-`applied` stage, and ensure that case is not described as healthy/completed/applied.
4. Remove only the `docs_archive/` expectation, the seven obsolete artifact checks (`inventory.json`, `classification.json`, `extraction.json`, `plan.json`, `plan.md`, `validation.json`, `apply.json`), and all `migrate apply` hints. Keep unrelated doctor checks and genuine errors intact.
5. Add table-driven `internal/doctor` mutation tests and `cmd/mindspec` command-level exit/output tests for completed, malformed, missing, mismatched, and in-progress fixtures. Explicitly update the obsolete-contract test `TestCheckMigrationMetadata_MissingLineageManifest` (and any sibling fossil tests pinning the deleted classify-pipeline artifacts) to the rekeyed current-run contract so the package remains green.
6. Amend ADR-0039 Decision §2's legacy-tier bullet, on that bullet's line, to include `docs/context-map.md` after `docs/{specs,adr,domains,core}`. Add the exact sentence `This documentation-consistency reconciliation records the existing ContextMapPath/resolveArtifact fallback; it makes no resolver behavior change and does not supersede ADR-0039.`

**Verification**

- [ ] **B3-V1 (AC-7):** `go test ./internal/doctor/... -run 'TestCheckMigrationMetadata_CurrentCompletedRun'` proves the current three-artifact fixture is healthy without `docs_archive/`, obsolete artifacts, or obsolete hints.
- [ ] **B3-V2 (AC-7b):** `go test ./internal/doctor/... -run 'TestCheckMigrationMetadata_CurrentContractMutations'` passes table rows for malformed manifest/state/lineage, lineage run-ID mismatch, and removal of each required artifact, asserting Error/Missing findings.
- [ ] **B3-V3 (AC-24):** `go test ./internal/doctor/... -run 'TestCheckMigrationMetadata_CurrentContractMutations/non_applied_finalize_warns'` asserts Warn/non-fatal status and no healthy/completed/applied claim.
- [ ] **B3-V4 (AC-7, AC-7b, AC-24 command behavior):** `go test ./cmd/mindspec/... -run 'TestDoctorMigrationMetadata_CurrentContract'` proves exit codes and rendered output, including no obsolete-artifact finding or `migrate apply` hint.
- [ ] **B3-V5 (AC-25):** `rg -n '^[[:space:]]*3\. \*\*legacy\*\*.*docs/\{specs,adr,domains,core\}.*docs/context-map\.md' .mindspec/adr/ADR-0039-flat-layout-v2.md && rg -nF 'This documentation-consistency reconciliation records the existing ContextMapPath/resolveArtifact fallback; it makes no resolver behavior change and does not supersede ADR-0039.' .mindspec/adr/ADR-0039-flat-layout-v2.md` matches both the amended Decision §2 legacy-tier bullet and its consistency/no-supersession declaration. This proof MUST fail against the current, unamended ADR and pass only after the Bead 3 amendment.
- [ ] **B3-V6 (package regression):** `go test ./internal/doctor/... ./cmd/mindspec/...` passes with unrelated doctor behavior intact.

**Acceptance Criteria**

- `checkMigrationMetadata` is rekeyed to the current `migrate layout` contract: a completed run (global manifest + per-run `state.json` stage `applied` + per-run `lineage.json`) is healthy and doctor exits 0 with no `docs_archive/`, no obsolete-artifact finding, and no `migrate apply` hint (spec AC-7).
- Malformed, missing, empty, or identity-mismatched current artifacts still yield Error/Missing findings and nonzero exit, and a parseable non-`applied` (in-progress) stage remains a Warn/non-fatal, exit-0 finding that is never reported as healthy/completed (spec AC-7b, AC-24).
- ADR-0039 Decision §2 is amended to include legacy `docs/context-map.md` and cite the `ContextMapPath`/`resolveArtifact` fallback as a documentation-consistency reconciliation with no behavior change and no supersession (spec AC-25).

**Depends on**

None. This bead is independent of Beads 1 and 2 and can proceed in parallel.

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| AC-1 | Bead 1, B1-V1 — ordinary root `docs/` plus flat lifecycle resolves flat |
| AC-2 | Bead 1, B1-V1 — empty/leftover canonical wrapper plus flat lifecycle resolves flat |
| AC-3 | Bead 1, B1-V1 — empty direct legacy lifecycle directory resolves legacy |
| AC-4 | Bead 1, B1-V1 — empty direct canonical lifecycle directory resolves canonical |
| AC-5 | Bead 1, B1-V1 — flat plus canonical lifecycle directory resolves mixed and errors |
| AC-6 | Bead 1, B1-V1 — flat plus legacy lifecycle directory resolves mixed and errors |
| AC-7 | Bead 3, B3-V1 and B3-V4 — completed current run is healthy and command exits 0 without obsolete output |
| AC-7b | Bead 3, B3-V2 and B3-V4 — every malformed, mismatched, and missing current artifact fails with Error/Missing and nonzero command exit |
| AC-8 | Bead 1, B1-V2 — un-wedged new-spec path is `.mindspec/specs/{slug}` |
| AC-9 | Bead 2, B2-V1 and B2-V4 — unrelated canonical wrapper at a real ref does not mark canonical |
| AC-10 | Bead 2, B2-V2 and B2-V3 — real-ref flat plus canonical lifecycle source is mixed and locally blocked |
| AC-11 | Bead 2, B2-V1 and B2-V4 — ordinary legacy wrapper at a real ref does not mark legacy or false-block flat |
| AC-12 | Bead 2, B2-V2 and B2-V3 — independently derived legacy lifecycle marker makes real-ref source mixed and locally blocked |
| AC-13 | Bead 1, B1-V1 and B1-V4 — canonical context-map regular file resolves canonical |
| AC-14 | Bead 1, B1-V3 and B1-V4 — legacy context-map regular file resolves legacy and is selected by `ContextMapPath` |
| AC-15 | Bead 1, B1-V1 — flat plus canonical context map resolves mixed and errors |
| AC-16 | Bead 2, B2-V1 and B2-V4 — three real-ref context-map blobs classify flat/canonical/legacy with filesystem parity |
| AC-17 | Bead 1, B1-V1 — nested lifecycle names do not mark canonical or legacy |
| AC-18 | Bead 1, B1-V1 — wrapper regular files do not mark or panic |
| AC-19 | Bead 1, B1-V1 and B1-V4 — context-map directories do not mark or panic |
| AC-20 | Bead 1, B1-V1 and B1-V4 — lifecycle-name regular files do not mark or panic at any tier |
| AC-21 | Bead 1, B1-V1 — flat plus legacy context-map regular file resolves mixed and errors |
| AC-22 | Bead 2, B2-V2 and B2-V3 — independently derived legacy context-map marker makes real-ref source mixed and locally blocked |
| AC-23 | Bead 2, B2-V1 — context-map trees at all three real-ref paths do not mark and do not error |
| AC-24 | Bead 3, B3-V3 and B3-V4 — `finalize` remains Warn/non-fatal, exit 0, and not healthy/completed/applied |
| AC-25 | Bead 3, B3-V5 — the Decision §2 legacy-tier bullet itself names both `docs/{specs,adr,domains,core}` and the newly added `docs/context-map.md`, and the exact amendment sentence pins `ContextMapPath`/`resolveArtifact`, documentation consistency, no behavior change, and no supersession; the proof is RED before the amendment and GREEN afterward |
