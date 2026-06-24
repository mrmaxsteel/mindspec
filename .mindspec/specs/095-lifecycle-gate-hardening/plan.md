---
adr_citations:
    - id: ADR-0035
      sections:
        - recovery-line contract (Bead 2 guard hint; Bead 1 gate-failure messages)
    - id: ADR-0023
      sections:
        - phase derivation rule refined (Bead 2)
    - id: ADR-0036
      sections:
        - OWNERSHIP loader read follows the diffed ref; absent-→claims-nothing preserved (Bead 1, amend)
approved_at: "2026-06-13T08:05:36Z"
approved_by: user
bead_ids:
    - mindspec-qzqm.1
    - mindspec-qzqm.2
spec_id: 095-lifecycle-gate-hardening
status: Approved
version: "2"
---
# Plan: 095-lifecycle-gate-hardening

> **Plan revision 2** — incorporates the 6-panel plan review (review/095-plangate/). Key
> corrections: (1) `impl approve` doc-sync is NOT already `main..specBranch` — it is
> working-tree-vs-base (`ValidateDocs(root, base, exec)` → `ValidateDocsRange(root, base, "",
> exec)`, `internal/approve/impl.go:219`, `internal/validate/docsync.go:39`); Bead 1 must
> ref-anchor BOTH the diff head AND the ownership read there (the spec's "already
> main..specBranch / out of scope" line is corrected here). (2) Domain ENUMERATION also reads
> ambient `root` (`docsync.go:356`, divergence `listDomainDirs(root)` fallback) — ref-anchor it
> too or a branch-only domain dir stays invisible. (3) The ref read must distinguish
> path-absent-at-ref (claims-nothing, ADR-0036) from an operational git failure (hard error),
> else a git glitch silently un-attributes a file and un-gates doc-drift. (4) The real
> `LoadOwnership(root)` seams are `attributeDomain` (ownership.go) + `checkUnclaimedSource`
> (docsync.go:196) + the `cmd/mindspec/validate.go:70` CLI caller — not just the gate entry
> functions. (5) Phase derivation must NOT derive `review` over an EMPTY lifecycle-child set.

## ADR Fitness

- **ADR-0023** (beads-based phase derivation): Bead 2 refines the `review` rule (lifecycle
  children only; empty lifecycle set → plan). **Decision: amend ADR-0023** (no new ADR).
- **ADR-0031** (doc-sync OWNERSHIP attribution): Bead 1 changes the tree the manifest +
  domain enumeration are read from (ambient working tree → diffed ref). **Decision: amend
  ADR-0031** — this is a fidelity refinement of the SAME attribution decision, not a new one;
  no new ADR (resolves spec DQ2). (Next free ADR ID is **ADR-0039** should any later step prove
  a genuinely distinct decision — but the default and expected outcome is amend-only.)
- **ADR-0036** (OWNERSHIP loader, no silent fallback): Bead 1 preserves absent-→claims-nothing
  under the ref read AND adds the operational-error-≠-absent distinction; cited, amended note.
- **ADR-0035** (recovery-line convention): Bead 2's guard hint follows it.

## Testing Strategy

Every gate-input change is proven RED-on-revert (FAILS if the read reverts to ambient `root` /
the derivation reverts to counting all children). Beyond validate-layer unit tests, the
load-bearing proofs are INTEGRATION tests at the real call sites: (a) `complete.Run` from the
MAIN ROOT against a synthetic bead branch that commits an OWNERSHIP claim for a file it changed
→ gates PASS with NO override AND no `mindspec_adr_override_*` / doc-skew metadata recorded
(the vvs9 AC's load-bearing half); (b) `ApproveImpl` whole-branch gate reads OWNERSHIP from the
spec-branch tip; (c) `ApproveImpl` proceeds to `review` with an open non-lifecycle bug child
AND emits the guard hint. Full `go test -short -race ./...` + **golangci-lint locally per bead**
(CI Lint parity: American spelling, no new gosec — the spec-094 Lint failure must not recur)
gate every bead.

**Bead dependency:** Bead 2 depends on Bead 1 — both edit `internal/approve/impl.go` (Bead 1
the doc-sync call + ownership ref; Bead 2 the phase-gate guard hint), so they are sequenced to
avoid a merge conflict on that file.

## Bead 1: Ref-anchor ALL gate tree-reads (vvs9) + impl-approve diff fix + aqey/perm regression-lock

Make the doc-sync + ADR-divergence gates resolve their OWNERSHIP attribution input — manifest
loading AND domain enumeration — from the SAME git ref they diff, at every call site, so an
OWNERSHIP claim (or new domain dir) committed on a branch satisfies its own gate with no
override. Correct `impl approve`'s doc-sync to diff `base..specBranch` (not working-tree). Lock
the already-landed aqey/perm diff-range anchoring with regression tests.

**Steps**
1. `internal/validate/ownership.go`: add `LoadOwnershipAtRef(exec, ref, domain)` that resolves
   the manifest via `Executor.FileAtRef(ref, ".mindspec/domains/<domain>/OWNERSHIP.yaml")`.
   It MUST classify outcomes: **path-absent-at-ref** → claims-nothing Ownership
   (Source()=="missing", ADR-0036), NO error; **operational git/executor failure** (bad ref,
   git error) → propagate as a hard error (never silently claims-nothing). Since
   `executor.FileAtRef` (`internal/executor/mindspec_executor.go:472`) returns a generic error
   for both, add explicit not-in-tree classification (e.g. a `git cat-file -e <ref>:<path>`
   probe, or parse the not-in-tree signal) at the executor boundary or in this loader. HC-5
   excluded-first-segment check + YAML parse run identically on the ref bytes. Define a
   ref-qualified `ManifestPath` form (e.g. `<ref>:<path>`) since it is no longer an absolute
   on-disk path. Keep on-disk `LoadOwnership` unchanged for doctor + `ownership populate`.
2. Add `listDomainDirsAtRef(exec, ref)` (ref-aware domain enumeration) mirroring
   `listDomainDirs(root)`, so branch-only domain dirs are discovered from the diffed ref.
3. Thread the ref through EVERY attribution seam in the gates (not just the entry functions):
   - `attributeDomain` (ownership.go ~130, the SHARED helper used by both gates) → use
     `LoadOwnershipAtRef`.
   - `checkInternalPackages` (docsync.go ~356) + the advisory `checkUnclaimedSource`
     (docsync.go ~196) → both their domain-enumeration AND ownership reads follow the ref (so
     the advisory WARN lane reads the SAME tree as the blocking lane — no within-run
     inconsistency).
   - `divergence.go` `listDomainDirs(root)` empty-impacted-domains fallback (~135) → ref-aware.
   `ValidateDocsRange(root, base, head, exec)` already carries `head`+`exec`;
   `CheckADRDivergence` already carries `exec`+`headRef`. Add an explicit OWNERSHIP-REF
   parameter where the ref differs from the diff head (it is INDEPENDENT of base/head — see
   step 4 for the per-caller value); do not assume ref == diff head.
4. Wire every call site with its correct ownership ref (enumerate ALL, do not miss one):
   - `complete.go` per-bead doc-sync (`:286`) + ADR-divergence (`:308`) → ownership ref =
     `beadHead`.
   - `approve/impl.go` whole-branch doc-sync (`:219`): change `ValidateDocs(root, base, exec)`
     to the explicit `base..specBranch` range form AND ownership ref = spec-branch tip;
     ADR-divergence (`:241`) → ownership ref = spec-branch tip. (Corrects the spec's wrong
     "already main..specBranch" claim — impl-approve doc-sync was working-tree-vs-base.)
   - `cmd/mindspec/validate.go:70` (the `mindspec validate docs` CLI, head=="") + the
     `ValidateDocs` wrapper → ownership ref = working-tree/HEAD (preserve current behavior).
   - Note: `source_globs` is still read from `root` (`docsync.go:74`); document this as
     intentionally working-tree (config is not a per-bead gate input) OR ref-anchor it for
     consistency — decide + state in the bead (default: leave on-disk, documented).
5. ADR: amend ADR-0031 (attribution reads the diffed ref; manifest + domain enumeration) +
   an ADR-0036 amend-note (operational-error ≠ absent). No new ADR.
6. Tests — ref-anchoring (RED-on-revert): claim ONLY at the ref (committed on branch, absent at
   root) → file attributed/owned, gate PASSES, no override; claim at root but ABSENT at ref →
   does NOT satisfy the gate; absent-at-ref → claims nothing; **operational git error → hard
   error, NOT silent claims-nothing**; excluded-first-segment at ref → still errors (HC-5);
   branch-only domain dir present only at the ref → discovered + evaluated from main root. Each
   FAILS if reverted to `os.ReadFile(root)` / `listDomainDirs(root)`.
7. Tests — END-TO-END (the load-bearing vvs9 proof): `complete.Run` from the MAIN ROOT against
   a synthetic bead branch that commits an `OWNERSHIP.yaml` claim for a file it changed →
   assert doc-sync + ADR-divergence PASS with NO `--override-adr`/`--allow-doc-skew` AND assert
   NO override / AllowDocSkew metadata is recorded on the bead. Plus an `ApproveImpl`
   whole-branch test: branch-tip-only claim passes; root-only-absent-at-ref claim does NOT
   spuriously pass.
8. Tests — aqey/perm regression-lock (Req 3): (a) main moves ahead of
   `merge-base(specBranch, beadHead)` with an unrelated file → per-bead range is `base..beadHead`,
   excludes the main-drift file (no false block); (b) invoked with `HEAD==specBranch` tip → range
   still `base..beadHead`, NOT empty (no vacuous pass). Both FAIL if reverted to ambient `HEAD`.

**Verification**
- [ ] `go build ./... && go test -race ./internal/validate/... ./internal/complete/... ./internal/approve/... ./cmd/mindspec/...` green
- [ ] E2E: synthetic claim-on-branch completes from MAIN ROOT, gates pass, ZERO override metadata recorded (test asserts both)
- [ ] Branch-only domain dir + manifest at the ref is discovered + evaluated from main root
- [ ] Operational git error on the ref read → hard error (NOT silent claims-nothing) — tested
- [ ] aqey/perm regression tests RED when diff-range anchoring reverts to ambient HEAD
- [ ] `impl approve` doc-sync diffs `base..specBranch` (RED-on-revert from both main root + spec worktree)
- [ ] golangci-lint clean (American spelling; no new gosec); `go test -short -race ./...` green

**Acceptance Criteria**
- [ ] A bead whose branch adds an `OWNERSHIP.yaml` claim for a file it also changed passes its
      own doc-sync + ADR-divergence gates at `mindspec complete` run FROM THE MAIN ROOT with NO
      `--override-adr`/`--allow-doc-skew`, and NO override/AllowDocSkew metadata is recorded.
- [ ] A claim present at `root` but ABSENT at the diffed ref does NOT satisfy the gate (the read
      follows the ref both directions); absent-at-ref → claims nothing (ADR-0036).
- [ ] An OPERATIONAL git/executor failure on the ref read is a HARD error, never silently
      treated as absent/claims-nothing (no silent un-gating of doc-drift).
- [ ] A branch-only domain dir + manifest present only at the diffed ref is discovered and
      evaluated from the main root.
- [ ] `impl approve`'s doc-sync diffs `base..specBranch` (both diff head + ownership ref at the
      spec-branch tip), not working-tree-vs-base.
- [ ] The aqey/perm diff-range fork-anchoring is regression-locked (tests RED when reverted to
      ambient HEAD); HC-5 excluded-first-segment still errors at the ref.

**Depends on**
None

## Bead 2: Phase derivation ignores non-lifecycle children, empty-set-safe (ry73) + impl-approve guard hint

Derive `review` when every LIFECYCLE child is closed even with open non-lifecycle follow-up
children — but NEVER over an empty lifecycle set — and emit an advisory guard hint at
`impl approve` naming any non-lifecycle child that would otherwise block.

**Steps**
1. `internal/phase/derive.go` `DerivePhaseFromChildren` (+ the cache-aware path): partition
   children into LIFECYCLE (`issue_type == "task"` — verified: `ApprovePlan` creates beads with
   `--type task`, `internal/approve/plan.go:341`) vs NON-LIFECYCLE (`bug` / any non-`task`,
   non-`epic`). Compute `review` over LIFECYCLE children only: **if there are ZERO lifecycle
   children, do NOT derive review off non-lifecycle children — fall back to `plan`** (treat the
   empty lifecycle set exactly like the no-children case; this closes the vacuous-review hazard
   where a bug filed as an epic child during plan mode would otherwise force `review` on an
   unimplemented spec). All lifecycle children closed (≥1 lifecycle child) → `review`. Any
   in_progress lifecycle child → `implement`. Some closed + some open lifecycle → `implement`.
2. Emit a structured guard hint (ADR-0035 convention) ON THE `impl approve` PATH
   (`internal/approve/impl.go`) — NOT only inside `DerivePhaseFromChildren` — when `review` is
   derived while a non-lifecycle child remains open: a stderr line naming the offending
   child(ren) + a recovery suggestion. The hint is advisory; it does NOT block. **The recovery
   line must NOT bare-recommend `bd update <id> --parent ""`** (that command is buggy per
   mindspec-bk5t — detach not reflected in `bd list --parent`); recommend re-filing as
   standalone backlog or leaving as-is, and note the detach caveat.
3. ADR: amend ADR-0023 with the refined `review` rule (lifecycle-children-only; empty set →
   plan) + the `issue_type==task` discriminator + its `plan.go:341` invariant.
4. Tests (RED-on-revert): all `task` children closed + open `bug` child → `review` (FAILS if
   reverted to counting all children); **ZERO `task` children + open `bug` child → `plan`, NOT
   review** (the vacuous-review guard); open `task` + closed `bug` → `implement`; in_progress
   `task` → `implement`; cover the cache-aware path identically.
5. Tests — INTEGRATION (the ry73 e2e guarantee, via `DerivePhaseDetail` + the spec-092 Req1
   derived-branch reconcile, NOT the metadata-first `DerivePhaseWithStatus`): (a) `ApproveImpl`
   with stored `mindspec_phase=="implement"` but child-derived `review` (a bug filed AFTER the
   last `complete`) → proceeds with NO manual `repair phase`, emits the guard hint; (b)
   `complete.Run` closing the LAST lifecycle bead while a bug child is already open →
   `advanceState` derives `review` and persists `mindspec_phase=="review"` (`complete.go:490`,
   `:495`).

**Verification**
- [ ] `go build ./... && go test -race ./internal/phase/... ./internal/approve/... ./internal/complete/...` green
- [ ] Epic (all lifecycle beads closed + open follow-up bug child) → `review`; `impl approve` proceeds, no manual detach
- [ ] Zero-lifecycle-children + open bug child → `plan` (NOT review) — tested
- [ ] Stored='implement'/derived='review' → impl approve proceeds via the 092 reconcile (no repair phase) — tested
- [ ] Guard hint names the offending child, does NOT recommend the buggy bare `--parent ""`; derivation tests RED-on-revert
- [ ] golangci-lint clean; `go test -short -race ./...` green

**Acceptance Criteria**
- [ ] An epic whose lifecycle (`task`) children are ALL closed but which carries an open
      non-lifecycle (`bug`) child derives `review`, and `mindspec impl approve` proceeds with NO
      manual detach + `repair phase`.
- [ ] An epic with ZERO lifecycle (`task`) children + an open `bug` child derives `plan`, NOT
      `review` (the vacuous-review hazard is closed; impl approve does not fire on an
      unimplemented spec).
- [ ] With a stored `mindspec_phase=="implement"` but child-derived `review` (a bug filed after
      the last `complete`), `impl approve` proceeds via the spec-092 derived-branch reconcile —
      no manual `repair phase` required.
- [ ] When `review` is derived while a non-lifecycle child is open, a structured guard hint
      names the child on the `impl approve` path and does NOT bare-recommend the bk5t-buggy
      `bd update <id> --parent ""`; the hint is advisory (does not block).

**Depends on**
Bead 1 (both edit `internal/approve/impl.go`)

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| Claim-on-branch passes own gate, no override + ZERO override metadata (vvs9) | Bead 1 steps 6,7 + verification |
| Read follows ref both directions; absent-at-ref claims nothing; operational-error≠absent; HC-5 | Bead 1 step 6 |
| Branch-only domain dir discovered from the ref | Bead 1 steps 2,6 |
| impl-approve doc-sync ref-anchored base..specBranch | Bead 1 steps 4,7 |
| Epic with open follow-up bug child reaches review (ry73) | Bead 2 steps 1,4,5 + verification |
| Empty lifecycle set → plan (vacuous-review guard) | Bead 2 steps 1,4 |
| Non-lifecycle child guard hint at impl approve (not buggy recovery) | Bead 2 steps 2,5 |
| aqey/perm diff-range anchoring regression-locked | Bead 1 step 8 |
| build/test/golangci-lint green | Both beads' verification |
