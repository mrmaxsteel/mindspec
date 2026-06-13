---
approved_at: "2026-06-13T07:32:32Z"
approved_by: user
status: Approved
---
# Spec 095-lifecycle-gate-hardening: Lifecycle gate fork-anchoring + phase derivation hardening

## Goal

Eliminate the two recurring lifecycle-mechanics friction classes that forced manual
workarounds throughout specs 091–094: (1) doc-sync / ADR-divergence gates that falsely
block a `complete`/`impl approve` because they read OWNERSHIP attribution from the ambient
working tree instead of the diffed ref (forcing a `--override-adr` on every legitimately
on-branch ownership claim — **mindspec-vvs9**); and (2) a phase derivation that counts a
filed follow-up bug as a blocking epic child, so the spec can never reach `review` and
`impl approve` fails until the operator manually detaches the child and runs `repair phase`
(**mindspec-ry73**). After this spec, an OWNERSHIP claim committed on a bead/spec branch
satisfies its own gate with no override, and filing a follow-up bug never strands a spec
short of `review`.

## Background

A cluster of "gate tree-anchoring" bugs was discovered across specs 091–094:

- **mindspec-aqey / mindspec-perm (ALREADY FIXED — regression-lock only):** the per-bead
  gate diff range used the ambient `HEAD`, measuring main-side drift from the repo root
  (false blocks) or an empty range from the spec worktree (vacuous passes). This is already
  resolved: `internal/complete/complete.go` anchors the range to
  `merge-base(specBranch, beadHead)`..`beadHead` (see the comment at complete.go:267-285,
  which names aqey/perm). This spec adds explicit regression tests so the fix cannot silently
  regress; it does NOT re-implement it.

- **mindspec-vvs9 (LIVE):** the diff range is now ref-anchored, but OWNERSHIP *attribution*
  still is not. `internal/validate/ownership.go::LoadOwnership(root, domain)` does
  `os.ReadFile(filepath.Join(root, ".mindspec/docs/domains/<domain>/OWNERSHIP.yaml"))` — it
  reads the manifest from the ambient working tree at `root`. When `complete`/`impl approve`
  runs from the main checkout, `root` is main, so the gate evaluates a changed file against
  *main's* OWNERSHIP, which lacks any claim the branch added. The branch literally cannot
  satisfy its own gate; every spec 094 `complete` + `impl approve` required a truthful
  `--override-adr "<vvs9>"`. The fix: read the OWNERSHIP manifests from the SAME ref the gate
  diffs (`beadHead` at `complete`, `specBranch` at `impl approve`), via the executor's
  `git show <ref>:<path>`, not from the ambient working tree.

- **mindspec-ry73 (LIVE):** `internal/phase/derive.go::DerivePhaseFromChildren` returns
  `review` only when EVERY child is closed. Filing a P3 follow-up bug as a child of the spec
  epic (the natural thing to do) leaves one open child, so the epic derives `implement` even
  with every lifecycle bead closed, and `impl approve` fails `expected review mode`. Spec 094
  hit this with cdk8.5; specs 091/092 only dodged it by keeping follow-ups as standalone
  backlog. The fix: phase derivation must not let a NON-LIFECYCLE follow-up child block
  `review`, and must emit a clear guard hint when one would.

These are exactly the lifecycle frictions the spec-094 self-improvement loop is designed to
surface; this spec pays down the highest-cost ones.

## Impacted Domains

- **workflow**: `internal/validate` (doc-sync + ADR-divergence ownership attribution gates),
  `internal/complete` + `internal/approve` (thread the diffed ref into the gates) — Bead 1.
- **execution**: `internal/executor` (the `FileAtRefOrAbsent` / `TreeDirsAtRef` ref-read methods
  the gates resolve OWNERSHIP manifests through) — Bead 1.
- **core**: `internal/phase` (lifecycle phase derivation) — Bead 2.

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md): beads-based lifecycle phase derivation — Req 2 refines
  the `review` derivation rule for non-lifecycle children.
- [ADR-0031](../../adr/ADR-0031.md): doc-sync OWNERSHIP attribution — Req 1 changes the tree
  the manifest is read from (ambient working tree → diffed ref).
- [ADR-0036](../../adr/ADR-0036.md): OWNERSHIP loader (spec 091 Req 13, no silent fallback) —
  Req 1 preserves the absent-manifest-claims-nothing semantics under a ref read.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md): agent error/recovery contract
  (Accepted; Domain(s): workflow, execution, core — covers BOTH impacted domains). Req 2's
  phase-gate guard hint follows its recovery-line convention; the ADR-divergence/doc-sync gate
  failures Req 1 governs are emitted via this contract.
- Decision (resolved at planning, was a deferred question): the ref-anchored gate-input read is
  a fidelity refinement of ADR-0031's attribution decision — **amend ADR-0031**, no new ADR.

## Requirements

1. **(vvs9) Gate OWNERSHIP attribution reads the diffed ref, not the ambient working tree.**
   The doc-sync gate (`ValidateDocsRange`) and the ADR-divergence gate (`CheckADRDivergence`)
   MUST resolve every per-domain `OWNERSHIP.yaml` from the same git ref they diff — `beadHead`
   for the per-bead gates in `complete`, the spec branch tip for the whole-branch gate in
   `impl approve` — via the executor (`git show <ref>:<manifest-path>`), NOT via
   `os.ReadFile` on the ambient `root`. An OWNERSHIP claim committed on the branch under test
   MUST satisfy its own gate with no override. The absent-manifest → claims-nothing semantics
   (ADR-0036 / spec 091 Req 13) and the HC-5 excluded-first-segment schema rejection MUST be
   preserved when the manifest is read from a ref. A manifest absent at the ref (never
   committed) is treated identically to absent on disk (claims nothing).

2. **(ry73) Phase derivation does not let a non-lifecycle follow-up child block `review`.**
   `DerivePhaseFromChildren` MUST derive `review` when every LIFECYCLE child is closed even
   if non-lifecycle follow-up children (e.g. bugs filed post-implementation) remain open. When
   a non-lifecycle open child is the only thing preventing `review`, `impl approve`'s phase
   gate MUST NOT block; instead it emits a structured guard hint (ADR-0035 recovery-line
   convention) naming the offending child(ren) so the operator can re-file or detach if they
   disagree. The exact discriminator between "lifecycle" and "non-lifecycle" child is a design
   question for planning (see Design Questions), but the observable contract is: filing a
   follow-up bug as an epic child never strands the spec short of `review`.

3. **(aqey/perm) Regression-lock the already-landed diff-range fork-anchoring.** Add explicit
   regression tests proving the per-bead gate range is anchored to
   `merge-base(specBranch, beadHead)`..`beadHead` and is invariant to (a) main moving ahead of
   the fork point (no false block from unrelated main drift) and (b) being invoked from the
   spec worktree (no vacuous empty-range pass). This requirement adds tests only; it MUST NOT
   alter the existing anchoring behavior.

## Scope

### In Scope
- `internal/validate/ownership.go` — a ref-aware manifest read (executor-backed) alongside the
  existing on-disk `LoadOwnership`.
- `internal/validate/docsync.go`, `internal/validate/divergence.go` — consume the ref-aware
  ownership read; thread the ref + executor through.
- `internal/complete/complete.go`, `internal/approve/impl.go` — pass the diffed ref
  (`beadHead` / spec branch tip) to the gates for the ownership read.
- `internal/phase/derive.go` — non-lifecycle-child-aware `review` derivation + guard hint.
- Regression tests for aqey/perm anchoring.

### Out of Scope
- The per-bead diff RANGE computation (already fork-anchored — Req 3 only pins it with tests).
- (CORRECTION per plan review: `impl approve`'s doc-sync is NOT already `main..specBranch` —
  it is working-tree-vs-base (`ValidateDocs(root, base, exec)`). Ref-anchoring BOTH its diff
  head and its ownership read to the spec-branch tip IS in scope for Bead 1; only the
  ADR-divergence committed-range refs, which are already `base..specBranchTip`, stay out.)

## Non-Goals

- **mindspec-bk5t** (the `bd update --parent ""` reverse-index bug) — that is in the external
  `bd`/beads tool, not the mindspec codebase; not addressed here.
- The self-improvement loop remote-push (**mindspec-sot1** follow-on) and free-text Scrub
  completeness (**mindspec-gne7**) — unrelated.
- Re-implementing the aqey/perm diff-range anchoring — already landed; this spec only locks it.

## Acceptance Criteria

- [ ] A bead whose branch adds an `OWNERSHIP.yaml` claim for a file it also changes passes its
      own doc-sync + ADR-divergence gates at `mindspec complete` run FROM THE MAIN ROOT with
      NO `--override-adr` / `--allow-doc-skew` (the vvs9 scenario that forced overrides on
      every spec-094 complete).
- [ ] An OWNERSHIP manifest that exists on disk at `root` but is ABSENT at the diffed ref does
      NOT spuriously satisfy the gate (the read follows the ref, both directions).
- [ ] Absent-at-ref manifest → domain claims nothing (ADR-0036 preserved); an excluded
      first-segment entry at the ref still errors (HC-5 preserved).
- [ ] A spec epic whose lifecycle beads are all closed but which carries an open follow-up bug
      child derives `review` and `mindspec impl approve` proceeds with NO manual detach +
      `repair phase` (the ry73 / cdk8.5 scenario).
- [ ] When a non-lifecycle open child would otherwise block, a structured guard hint names it
      and the recovery — and the gate still proceeds.
- [ ] Regression tests pin the aqey/perm diff-range anchoring (main-drift → no false block;
      spec-worktree invocation → no vacuous pass) and FAIL if the anchoring is reverted to
      ambient HEAD.
- [ ] `go build ./...` + `go test -short -race ./...` green; golangci-lint (the CI Lint job)
      clean (American spelling; no new gosec).

## Validation Proofs

- `go test ./internal/validate/...`: ref-anchored ownership read — claim-on-branch passes,
  claim-only-on-disk-not-at-ref does not; absent-at-ref claims nothing; HC-5 still errors.
- `go test ./internal/phase/...`: an epic with all lifecycle children closed + an open
  follow-up bug child derives `review`; the guard hint names the offending child.
- `go test ./internal/complete/... ./internal/approve/...`: per-bead + whole-branch gates read
  ownership from the diffed ref; aqey/perm anchoring regression tests RED-on-revert.
- End-to-end: a synthetic bead that claims a new path on its branch completes from main root
  with zero override metadata recorded.

## Design Questions (for the panel)

None blocking approval. Refined at planning / by the implementation panel:

- The exact "lifecycle vs non-lifecycle child" discriminator for Req 2: by `issue_type`
  (lifecycle beads are `task` created by `plan approve`; follow-ups are typically `bug`), by a
  creation-phase marker, by the parent-link relation (`discovered-from` vs a plan child), or by
  resolving it at the `impl approve` gate rather than in `DerivePhaseFromChildren`. Draft
  position: treat `issue_type == bug` (or any non-`task` child) as non-lifecycle for the
  `review` derivation, and emit the guard hint; confirm no legitimate lifecycle bead is ever a
  `bug`.
- Whether the ref-anchored ownership read warrants a new ADR or is covered by amending
  ADR-0031.
- Whether `impl approve`'s whole-branch ownership read should anchor to the spec branch tip vs
  `main..specBranch` head (consistency with the per-bead `beadHead` anchoring).

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-06-13
- **Notes**: Approved via mindspec approve spec