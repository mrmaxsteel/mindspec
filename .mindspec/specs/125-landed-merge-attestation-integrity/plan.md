---
adr_citations:
    - ADR-0041
    - ADR-0035
    - ADR-0025
    - ADR-0023
approved_at: "2026-07-24T09:10:45Z"
approved_by: user
bead_ids:
    - mindspec-xhd5.1
    - mindspec-xhd5.2
    - mindspec-xhd5.3
    - mindspec-xhd5.4
spec_id: 125-landed-merge-attestation-integrity
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/executor/mindspec_executor.go
        - internal/executor/merge_binding_test.go
        - internal/executor/merge_conflict_test.go
        - internal/gitutil/gitops.go
        - internal/gitutil/gitops_test.go
    - depends_on: []
      id: 2
      key_file_paths:
        - internal/gitutil/neteffect.go
        - internal/gitutil/neteffect_test.go
        - internal/lifecycle/landed.go
        - internal/lifecycle/landed_test.go
    - depends_on:
        - 1
        - 2
      id: 3
      key_file_paths:
        - internal/lifecycle/landed.go
        - internal/lifecycle/landed_test.go
        - internal/executor/landed_e2e_test.go
    - depends_on:
        - 3
      id: 4
      key_file_paths:
        - internal/lifecycle/reattest.go
        - internal/lifecycle/reattest_test.go
        - cmd/mindspec/reattest.go
        - cmd/mindspec/reattest_test.go
        - cmd/mindspec/root.go
        - cmd/mindspec/help_golden_test.go
        - .mindspec/adr/ADR-0041-gate-before-mutate.md
---
# Plan: 125-landed-merge-attestation-integrity

Four beads implement the attestation-integrity spec. The decomposition
follows the spec's own sketch ("a natural decomposition is three
beads") with **one deliberate re-cut, stated**: the sketch's bead 1
(R1+R2+R5) is SPLIT into an executor bead (Bead 1: persistence, loud
miss, recovery message — R1/R2 plus R5's executor site) and a lifecycle
bead (Bead 3: `FindLandedMerge`'s ownership/exact-match identity — R5's
read site), because they edit DISJOINT packages
(`internal/executor` vs `internal/lifecycle`), each is independently
PR-sized with its own RED-today fixtures, and the split buys a real
parallel wave: Bead 1 (executor) and Bead 2 (R3, gitutil) share no
files and run concurrently, which the sketch's monolithic bead 1 could
not. The spec's sequencing note ("beads 1 and 3 both touch the binding
write/read seams and the second-parent identity primitive, so the plan
sequences 3 after 1") is honored as the `1→3` edge below.

**Dependency graph (acyclic), waves, and the shared-file seam.**
Edges: `1→3`, `2→3`, `3→4`. Waves: W1 = {1, 2} (parallel),
W2 = {3}, W3 = {4}. Longest serial chain: 3 (`1→3→4` and `2→3→4`), at
the heuristic ceiling — justified because each link is genuine
produced-then-consumed state, not file adjacency:

- **Bead 3 depends on Bead 1**: (i) Bead 3's R6(a) e2e — the AC-1b/AC-2
  MF-3 contract, a real-git run through production `CompleteBead` in
  the real topology followed by `lifecycle.FindLandedMerge` identifying
  the branch-DELETED bead — consumes the binding persistence Bead 1
  lands (without it the e2e's read half has no write half to continue
  from); (ii) Bead 3's identity rewrite consumes the ONE shared
  exact-second-parent scan primitive (`gitutil.ExactSecondParentMerges`,
  Bead 1 Step 1) so the executor and lifecycle sites identify by the
  IDENTICAL rule rather than two drifting reimplementations; (iii) the
  AC-1b recovery-line assertion reads the `-m` fix Bead 1 makes.
- **Bead 3 depends on Bead 2**: R5's same-second-parent re-merge rule
  REUSES Requirement 3's discrimination VERBATIM, anchored on the
  OLDEST merge M₁ (AC-2e) — it invokes the corrected
  CleanDivergence sub-classification (`landedRevertShapeFn`) Bead 2
  lands; a Bead 3 built on today's primitive would MIS-refuse the
  AC-2e masked-revert assertions' evolved siblings. This edge also
  serializes the only co-edited file (below).
- **Bead 4 depends on Bead 3**: R4's derivation IS R5's rule — the
  re-attest scan reuses Bead 3's subject-ownership parser and
  exact-match/oldest-anchor helpers (the spec: "the WRITE requires a
  git-corroborated EXACT second-parent match under the Requirement 5
  rule"), and AC-8's descendant-decoy/circularity legs are only
  meaningful against Bead 3's landed fail-closed identity.

**Shared-file seam resolution (the 117 false-independence lesson),
stated explicitly** — `key_file_paths` is the true EDIT set per bead:
the only non-test source file edited by more than one bead is
`internal/lifecycle/landed.go` — Bead 2 (the `:365-382` CleanDivergence
revert-leg consumption + the seam block `:87-106`) and Bead 3 (the
`:249-388` identity loop, the ownership parser, the exact-match/
oldest-anchor rewrite). Under the `2→3` edge they are STRICTLY
SEQUENTIAL, so no two beads edit it concurrently. Bead 1's edit set is
executor + `gitutil/gitops.go`; Bead 2's gitutil edit is
`neteffect.go` only — W1 co-edits no file. Bead 3 adds the NEW
cross-package e2e file `internal/executor/landed_e2e_test.go`
(external `executor_test` package importing `internal/lifecycle` —
test-only, so the executor-must-not-import-lifecycle PRODUCTION
boundary stands untouched); it is a new file, so no collision with
Bead 1's executor test files. **This e2e drives the PRODUCTION seam
defaults through a stateful fake-`bd`-on-PATH (below), NOT unexported
in-package seam stubs** — from an external `_test` package the
unexported `mergeBindingFn`/`landedBindingMetadataFn` seams are
unreachable, so a stub-based e2e could neither build nor run
(F3-1/G1-F1). Bead 4 creates new files plus
`cmd/mindspec/root.go` (one `AddCommand` line), `help_golden_test.go`
(new verb in help output), and finalizes the ADR-0041 amendment —
none co-edited with any other bead.

**Plan-level choices the spec delegates (its "Recorded as plan-level
choices" paragraph), resolved:**

- **Re-attest verb: a top-level `mindspec reattest <bead-id>`.** A new
  cobra command registered in `cmd/mindspec/root.go` (the
  `completeCmd`/`doctorCmd` pattern, `root.go:216-225`), one bead per
  invocation — deliberately NO `--all`/fleet flag, so the
  mass-mutation vector across merged-bead history stays a scripted
  sequence of explicit per-bead invocations, and NO doctor surfacing
  that writes (AC-7 asserts the write is not produced by an implicit
  `doctor` run; doctor is untouched by this spec). No bypass flag of
  any kind exists on the surface (AC-8's flag-surface assertion). The
  spec branch is derived from the bead's epic linkage (bead → parent
  epic → `spec/<spec-id>`), with an explicit `--spec-branch` operand
  admitted ONLY as scoping input for beads whose linkage is
  underivable — it names WHERE to scan, never substitutes for
  corroboration.
- **R5 corroboration mechanism: a shared gitutil scan primitive +
  subject demoted to ownership nominator; the recovery fix does
  BOTH admissible halves.** Bead 1 adds
  `gitutil.ExactSecondParentMerges(root, branch, tip)` — a thin,
  newest-first filter over the existing `FirstParentMerges`
  (`gitops.go:539`) selecting exactly-two-parent merges whose SECOND
  parent EQUALS `tip` — consumed by BOTH identity sites
  (`locateLandedMergeByIdentity` in Bead 1, `FindLandedMerge` in
  Bead 3), so the exact-match rule has one home. `MergeInto`
  (`gitops.go:154`) keeps writing the exact `Merge <beadBranch>`
  subject (it remains the ownership nominator); and
  `beadToSpecConflictFailure`'s recovery line (`:1623`) gains
  `-m "Merge <beadBranch>"` so an operator following it verbatim ALSO
  produces the exact subject — belt (subject-independent
  identification) and suspenders (identifiable recovery subject),
  both of which AC-1b admits. `directMergeConflictFailure`'s
  `-m`-less line (`:1634`) is left unchanged per the spec's Out of
  Scope (the spec→main subject is never identity-scanned).
- **R3 discrimination mechanism: a SIBLING primitive
  `gitutil.RevertShape`, consulted only on the CleanDivergence
  outcome — the reverse "un-apply" no-op test.**
  `RevertShape(workdir, mergeSHA, target)` computes the three-way
  `merge-tree(base = M, ours = target-tip, theirs = M^1)` — i.e. "does
  UN-applying M's change from the tip do anything?" — and reports
  revert-shape iff the un-apply is CLEAN and its result tree EQUALS
  the tip's tree (the tip already carries NONE of M's introduced
  content at any of its sites: exactly what `git revert M` leaves, and
  also the stated clean-full-removal residual, BOTH of which the spec
  REFUSES). Any other outcome — the un-apply CHANGES the tip (M's
  content is present, wholly or PARTIALLY, the real 8nhe.2
  partial-supersession shape) or CONFLICTS (the tip built on M's
  region) — is EVOLVED → identified. `landed.go`'s revert leg becomes:
  `SubsumptionCleanDivergence && RevertShape` → refuse;
  `SubsumptionCleanDivergence && !RevertShape` → identify (the AC-5
  fix). `ContentSubsumedOutcome`, its trichotomy, and the
  `ContentSubsumed`/`NetEffectLanded` boolean projections are
  BYTE-IDENTICAL (grep-confirmable — the AC-6/Conflict-arm guard); the
  Conflict arm is not routed through the new primitive at all. The
  ≥2-parent requirement is enforced at the primitive (`M^1`/`M^2`
  resolution fails on a non-merge; callers already exclude octopus
  candidates — AC-6 pins the exclusion). Pure
  relocate-with-zero-evolution classifies with the residual floor
  (refuses) — consistent with AC-5's fixture parenthetical, which
  explicitly EXCLUDES "a pure delete/relocate" as "the stated
  indistinguishable residual"; the mechanism may be refined at
  implementation only within AC-5/AC-6's observable contracts.
- **R2 classification mechanism: membership and locate share the scan
  but classification is STRUCTURAL.** `ensureLandedBinding`'s
  nothing-to-bind test is "the bead tip is the second parent of NO
  first-parent merge on `spec`" — computed DIRECTLY from the gitutil
  scan, never from own-commit-count/merge-base (the spec's proven-
  insufficient classifier). The locate goes behind a NEW package seam
  (`locateLandedMergeFn`, default pinned by an anti-drift pointer test
  — the `netEffectLandedFn`/AC-17 pattern at
  `neteffect_probe_test.go:191`) so AC-3/AC-4b can FORCE a locate miss
  while the unseamed membership test still sees the merge — the shape
  that goes loud under the structural classifier and silently passes
  under a count classifier.
- **Bead-anonymous-subject reconciliation (R1 residual vs R5
  ownership wording), stated for the plan gate — REVISED by the codex
  final-review G-1 BLOCKING fix.** R5 trusts a cached binding "ONLY
  when it resolves to a real merge that BOTH exact-matches AND whose
  subject names THIS bead". R1's residual notes that a merge with a
  wholly-custom subject naming NO bead has no surviving subject-side
  ownership signal. The ORIGINAL plan proposed a second BINDING-SHA
  candidate-generation entry point admitting such an anonymous merge
  directly on its binding; **that path is REMOVED as a forgery hole
  (G-1).** Git-corroborating a binding proves only that the merge is
  REAL with a given exact second parent — NOT that the merge is THIS
  bead's — so admitting an anonymous merge on the binding alone makes
  the agent-writable binding an independent OWNERSHIP authority: a
  METADATA-forge (EASIER than a commit-forge, so BELOW the documented
  git-history threat boundary) on a never-landed bead X, pointing at any
  real anonymous merge, would make `FindLandedMerge(X)` positively
  identify a merge that isn't X's — an unsafe false-positive.
  Therefore the AUTOMATIC path has exactly ONE candidate-generation
  entry point: the SUBJECT-SCAN (the merge's subject NAMES this bead,
  either subject form, full branch-name equality). The binding stays an
  ownership CACHE **only** for a candidate whose subject already names
  THIS bead (this-bead only — no longer this-bead-OR-no-bead); it is
  DISCARDED when the subject names a DIFFERENT bead, including a `bead/…`
  token in an unrecognized format (G2-2). Because mindspec's OWN merges
  ALWAYS name the bead (both `MergeInto`'s `Merge <branch>` and git's
  conflict-recovery default `Merge branch 'bead/X' into …` carry the
  branch name), the anonymous-subject case arises only from a
  hand-crafted operator merge with a wholly-custom message — rare, and
  recovered ONLY through the EXPLICIT, operator-vouched, AUDITED
  `mindspec reattest` (Bead 4), never a binding-alone automatic
  identification. So R1's residual reconciles as FAIL-CLOSED on the
  automatic path (safe refusal → the audited explicit surface), not a
  binding-alone read. Bead 3 pins the anonymous-subject-REFUSES
  direction AND the forged-binding-at-a-real-anonymous-merge exploit
  (RED against the removed impl), and keeps the different-bead-format
  reject green.
- **Audit-record keys (constrained by AC-7's auditability
  assertion):** flat keys in the existing bd metadata map, written in
  the SAME `MergeMetadata` call as the binding —
  `mindspec_landed_reattest_actor` (acting identity/authority:
  `os/user` + host, plus the invoking operation's argv0),
  `mindspec_landed_reattest_at` (RFC3339 UTC),
  `mindspec_landed_reattest_op` (the invoking operation),
  `mindspec_landed_reattest_corroboration` (which admissible datum,
  (a)–(d), and its SHA), `mindspec_landed_reattest_prior_merge_sha` /
  `mindspec_landed_reattest_prior_second_parent` (before-values; empty
  string when previously absent), and
  `mindspec_landed_reattest_scanned_branch` (the spec branch the scan
  actually ran against — F2-2, so a mis-scoped `--spec-branch` is
  reconstructable by inspection). Exact names may be adjusted at
  implementation only within AC-7/AC-9's inspectability assertions.
- **No new redact/registry token.** The re-attest surface introduces
  no secret-bearing config or token (it reads git and writes bd
  metadata SHAs), so the spec-110 core-domain precedent is NOT
  activated: `core` stays untouched, exactly the conditional polarity
  the spec's Impacted Domains anticipated.

**Model tiering (standing protocol):** Sonnet implements by default,
escalate to Fable. **Beads 2 and 3 are the delicate ones and should go
to (or escalate early to) Fable**: Bead 2's discrimination must fix
ONLY the CleanDivergence arm while keeping the Conflict arm and every
boolean projection byte-identical, and its AC-5 fixture must assert its
own shape-precondition (today's primitive returns CleanDivergence) so
it cannot drift into the already-green conflict shape; Bead 3 is the
ownership/topology root-of-trust rewrite with five adversarial AC
fixtures each RED against a specific tempting-but-wrong implementation.
Beads 1 and 4 are Sonnet-suitable.

**Dogfood note**: every bead touches only execution-owned
(`internal/executor`, `internal/gitutil`, `internal/bead`) and
workflow-owned (`internal/lifecycle`, `cmd/**`) paths plus process
artifacts, under a spec whose Impacted Domains and cited ADRs cover
them — each bead's own `mindspec complete` MUST pass the divergence
gate with ZERO `--override-adr`. NOTE the recursion hazard this spec
uniquely carries: these beads change the very `complete` path that
completes them — Bead 1's own completion is the first live run of its
persistence fix, and the orchestrator should verify the binding on
Bead 1's own bead ID (`bd show <id> --json`) as an extra live proof.
**Not self-gated (plan-gate G3-B2, refuted):** 125's OWN beads are not
readiness-gated — MF-3 is spec 124's gate, and 125's beads are
completed by the INSTALLED (still-buggy) `mindspec` binary regardless
of bead order, so ALL of 125's beads lack landed-bindings during 125's
own development. That is EXPECTED and is NOT a decomposition problem:
125 fixes FUTURE completes after it ships and is installed; spec 124's
rebase-onto-125 + a later `mindspec reattest` pass recover the interim
bindings. No order-dependent breakage exists among 125's own beads (W1
= {1, 2} share no files; the `2→3→4` chain is produced-then-consumed
state, not a completion-time dependency).

**Delivery housekeeping (orchestrator close-out, not a bead)**: file
the named follow-up bead for the `SubsumptionConflict`-hides-revert
false-POSITIVE residual (a content-presence discriminator without the
§2(i) deadlock — the spec's Non-Goal); close `mindspec-8nhe.2`'s
false-reject trail against AC-5; note that ACTUALLY running
`mindspec reattest` across the 755/757 historical beads is operator
work the spec explicitly excludes from this delivery.

## ADR Fitness

- **ADR-0041 (Gate-Before-Mutate) — AMENDED by this spec (narrowly),
  otherwise APPLIED; the only ADR change.** Requirements 1–3 and 5 are
  pure applications of §2(i)/(ii) (the spec's Touchpoints argue each
  needs NO amendment: the loud-miss fix is what §2(ii) already
  mandates; second-parent corroboration tightens TOWARD "never subject
  text alone"; the evolved-content fix removes a §2(i)-forbidden
  permanent refusal). Requirement 4 needs the one narrow §2(ii)
  touchpoint: re-attestation admissibility derives from
  corroborated-identity discipline, not write time, with the
  STANDALONE datum enumeration (a)–(e), the audited
  detectable-by-inspection (not tamper-proof) record, and the honest
  recovery scope. Per the 122/123 precedent the full amendment text is
  **PRE-DRAFTED at plan time** — it sits in this worktree's
  `.mindspec/adr/ADR-0041-gate-before-mutate.md` now as
  `## Amendment (Spec 125): Re-attested landed-bindings under §2(ii)`
  under an explicit `PRE-DRAFT` marker comment — and is FINALIZED by
  Bead 4 (marker removed; wording adjusted only where the concrete
  implementation forces it), so the amendment is reviewable at
  plan-approve and lands with the re-attest surface (AC-11's
  amendment-RED-until-landed pin).
- **ADR-0035 (Agent Error Contract) — unchanged, applied.** The new
  loud bind-failure refusal (Bead 1) and every re-attest refusal
  (Bead 4) route through `internal/guard` with a named, copy-pastable
  recovery (re-run `mindspec complete <id>`; `mindspec reattest <id>`;
  the q9ea attested-restore exit for the genuinely-bare case); no
  refusal ends without a forward exit.
- **ADR-0025 (JSONL as Build Artifact) — unchanged, respected.** All
  binding and audit writes go through the existing `internal/bead`
  helpers (`MergeMetadata`/`GetMetadata`, `bdcli.go:322`/`:368`) behind
  the existing seam family; no direct JSONL edits, no raw
  `bd update --metadata` surface.
- **ADR-0023 (Beads/Dolt as Single State Authority) — unchanged,
  respected.** The binding remains advisory corroboration metadata:
  `reattest` writes identity evidence and audit values only — it never
  opens, closes, or reclassifies a bead, and no consumer gains
  lifecycle authority from it.

No ADR is superseded; no divergence requiring a human stop. The
executor/lifecycle import boundary (executor MUST NOT import
`internal/lifecycle`) is preserved: the one cross-package artifact is a
TEST-ONLY external-package file (`landed_e2e_test.go`).

## Testing Strategy

- **Real-git temp-repo fixtures, existing patterns.** Executor beads
  reuse the `merge_binding_test.go`/`merge_conflict_test.go` real-git
  fixture shapes (production `CompleteBead`/`FinalizeEpic` against a
  scratch root, hermetic in-memory metadata store via
  `mergeBindingFn`/`mergeBindingReadFn` overrides); lifecycle beads
  reuse `initLandedRepo`/`mergeBead`/`commitResolvedMerge`
  (`landed_test.go`), stubbing `landedBindingMetadataFn` for binding
  fixtures. "Real topology" per the spec: merge performed in the spec
  worktree, locate/bind driven from `g.Root`, bead branch + worktree
  deleted immediately after.
- **The pinned conflict-recovery miss shape is a first-class
  fixture** (AC-1/AC-1b/R6a): a 2nd bead under a spec hits a REAL
  add/add conflict on a tracked `.beads/issues.jsonl` path (a plain
  committed file in the fixture — no bd process needed), the printed
  recovery `git merge --no-ff` runs WITHOUT `-m` (producing git's
  default `Merge branch 'bead/x' into 'spec/…'` subject), then the
  recovery re-run of `complete` finds the branch already an ancestor.
  RED-today is UNCONDITIONAL on this shape (F1-1).
- **RED discipline, tagged honestly in-test.** RED-today set (fail on
  the spec-init SHA, fail again on fix-revert): AC-1, AC-1b, AC-1c,
  AC-2, AC-4b, AC-5, AC-7. **AC-3 is RED via the REAL-MISS shape or the
  fix-revert leg, NOT the spec-init SHA** (its seam-forced miss is a
  mechanism this spec adds; F1 MINOR). RED-against-the-wrong-impl set
  (pass only against the conforming mechanism; each names its target
  in-test): AC-2b (naive newest-first ancestor-consistent scan), AC-2c
  (newest-ancestor on no-exact-match), AC-2d (topology-only cache
  trust), AC-2e (newest-anchored content check), AC-2f
  (prefix/substring ownership), AC-4b (count/merge-base classifier —
  doubly red), AC-8 (decoy/descendant/circular legs). Guards (must not
  change): AC-4, AC-6, AC-10, and AC-5's different-region sub-case
  (a green anti-false-ID guard, NOT a RED-today assertion — F1 MINOR).
  Anti-drift: AC-11.
- **CI-hermetic, bd-less, no-skip-gating (spec 119 convention).**
  Everything above runs in the `go test -short ./...` bd-less lane:
  bd-touching legs go through the injectable seams
  (`mergeBindingFn`/`mergeBindingReadFn`/`landedBindingMetadataFn` and
  Bead 4's `reattestBindingFn` family) backed hermetically. **The one
  exception is Bead 3's cross-package `landed_e2e_test.go`** — an
  external `_test` package cannot reach the unexported seams, so it
  drives the PRODUCTION seam defaults against a stateful
  fake-`bd`-on-PATH (the `internal/approve` PATH-scoped scratch-`bin/`
  pattern), which exercises the real `bead.MergeMetadata`/`GetMetadata`
  write+read end-to-end. NO new test may skip-when-bd-missing; any test
  genuinely requiring a real (or fake) `bd` FATALs rather than skips
  when its declared environment lacks it. Seam DEFAULTS are pinned to the real `bead.MergeMetadata`/
  `bead.GetMetadata` symbols by pointer-equality anti-drift tests (the
  `netEffectLandedFn` AC-17 pattern, `neteffect_probe_test.go:191`) so
  the gate cannot go hollow (AC-11(i)); the new `locateLandedMergeFn`,
  `landedRevertShapeFn`, and `reattestBindingFn` seams get the same
  pin, AND the PRE-EXISTING but currently-UNPINNED lifecycle READ seam
  `landedBindingMetadataFn` (default `bead.GetMetadata`,
  `landed.go:96`) gains a pointer pin here so the read gate cannot go
  hollow either (F3-2).
- **Byte-identical guard technique.** AC-6's Conflict-arm/projection
  preservation is asserted as BEHAVIOR (the existing
  `neteffect_test.go` trichotomy + projection contracts and the
  spec-121 `landed_test.go:501`/`:526` reapply fixtures stay green,
  plus a new guard that a `SubsumptionConflict` outcome still
  falls through to identify), NOT a literal line-span — the
  CleanDivergence lines Bead 2 DOES change are adjacent.
- **Integration gates (every bead).** `go build ./...`,
  `go test -short ./...` (no new red), `go vet ./...`, `gofmt -l`
  clean, `golangci-lint run ./...`,
  `mindspec validate spec 125-landed-merge-attestation-integrity`, and
  a zero-`--override-adr` `mindspec complete` (dogfood note above).
  Review evidence maps every AC-1..AC-11 (incl. AC-1b, AC-1c,
  AC-2b–AC-2f, AC-4b) to exact `go test <package> -run <test>`
  commands per the spec's Validation Proofs; the local bd-on-PATH e2e
  (`bd show <bead> --json | jq -e
  '.metadata.mindspec_landed_merge_sha'`) is recorded at Bead 3 and at
  the spec's own beads as they complete.

## Bead 1: Ground-truth binding persistence + loud fail-closed miss (executor)

R1 + R2 in full, plus R5's executor site and the R5 recovery-message
fix. After every real bead→spec merge — INCLUDING the pinned
conflict-recovery/default-subject shape — `complete` and
`FinalizeEpic` durably persist the landed-binding BEFORE cleanup; a
genuine miss is LOUD, cleanup-suppressing, and recoverable; a true
bd_close orphan stays quiet on structural (first-parent-membership)
evidence.

**Steps**
1. `internal/gitutil/gitops.go`: add `ExactSecondParentMerges(root,
   branch, tip string) ([]MergeCommit, error)` — a newest-first filter
   over the existing `FirstParentMerges` (`:539`) selecting merges with
   EXACTLY two parents whose second parent EQUALS `tip`. This is the
   ONE exact-match identity primitive both sites consume (Bead 3 reuses
   it — the anti-drift rationale). Pin with a `gitops_test.go` fixture
   (incl. an octopus merge being excluded and a >1-result
   newest-first order).
2. `internal/executor/mindspec_executor.go`: rewrite
   `locateLandedMergeByIdentity` (`:1468`) onto WRITE-path ground
   truth: rev-parse `beadBranch`'s tip (the branch exists at bind
   time), scan `ExactSecondParentMerges(g.Root, specBranch, tip)`, and
   return the NEWEST exact match's `(mergeSHA, secondParent)`. The
   exact-subject gate (`"Merge "+beadBranch`) is REMOVED — identity is
   corroborated by second parent, so the default conflict-recovery
   subject can no longer defeat it (R1's "persist regardless of the
   merge's subject FORMAT"; no subject parsing on the write path). Put
   the locate behind a package seam `locateLandedMergeFn` (default =
   the real function; pointer-pinned per AC-11(i)'s pattern) so
   AC-3/AC-4b can force a miss.
3. `ensureLandedBinding` (`:1514`): DELETE the silent-nil miss swallow
   (`:1516-1519`). On a locate miss, classify by FIRST-PARENT
   MEMBERSHIP computed DIRECTLY from the gitutil scan (not through the
   seam): the bead tip is the second parent of NO first-parent merge
   on `specBranch` → the positive nothing-to-bind quiet path (no
   binding, no warning); the tip IS such a second parent (it did
   merge) but the locate missed or the write failed → return a
   `guard.NewFailure` naming the bead, the branch, the state, and the
   recovery (re-run `mindspec complete <id>`), which the existing
   producer legs (`CompleteBead :448-453`, `FinalizeEpic :672`/`:726`)
   already propagate BEFORE any cleanup — branch/worktree survive as
   the corroborating datum and re-invocation converges (§2(ii)).
   Explicitly NO own-commit-count / merge-base classifier anywhere on
   this path (R2's falsifier). **Idempotent-skip tightening (plan-gate
   G2-1):** the existing "already bound → convergent no-op" skip
   (`:1534-1538`) keys ONLY on the stored `mindspec_landed_merge_sha`
   matching the located merge — so a binding with the correct merge-SHA
   but an EMPTY or WRONG `mindspec_landed_second_parent` survives as a
   latent bad binding. Tighten the skip to require BOTH the stored
   merge-SHA AND the stored second-parent to AGREE with the located
   merge's `(mergeSHA, secondParent)`; any disagreement (incl. an empty
   stored second-parent) falls through to the OVERWRITE below, which is
   already fail-closed. This preserves the spec-121 G3-1 discipline and
   closes its second-parent gap.
4. `beadToSpecConflictFailure` (`:1600`): change the printed recovery
   line (`:1623`) to `git merge --no-ff -m "Merge <beadBranch>"
   <beadBranch>` so a verbatim-following operator produces an
   identifiable exact subject too (AC-1b's message half).
   `directMergeConflictFailure` (`:1634`) unchanged (Out of Scope).
5. Tests (executor package, hermetic store, real git): **AC-1** — the
   full conflict-shape fixture (2nd bead, add/add on
   `.beads/issues.jsonl`, `-m`-less recovery merge landing the default
   subject, recovery re-run of production `CompleteBead`): exit
   success; store holds `mindspec_landed_merge_sha` +
   `mindspec_landed_second_parent` equal to the rev-parse-verified
   merge and second parent; branch + worktree cleaned. **AC-1c** —
   `FinalizeEpic`'s auto-merge AND already-ancestor legs each persist
   the binding before cleanup; the already-ancestor sub-case's
   original merge carries the DEFAULT recovery subject (F1-3's
   unconditionally-RED shape). **AC-3** — merged-with-own-commits bead,
   locate forced to miss: non-zero `guard.NewFailure`
   naming bead/branch/recovery; branch and worktree SURVIVE; a
   subsequent successful run converges. Its RED evidence is the
   REAL-MISS shape or the fix-revert leg (NOT a spec-init-SHA claim —
   the seam-forced miss is a mechanism this bead ADDS, so it cannot be
   red on a tree that lacks the seam; F1 MINOR). **AC-4** — true
   bd_close orphan (tip second parent of NO merge): quiet completion,
   no binding, no refusal, no warning noise; pinned to the membership
   discriminator. **AC-4b** — merged-then-ancestor bead (byte-identical
   to the orphan under `rev-list`/`merge-base`), locate forced to miss:
   LOUD refusal — a count classifier would silently pass, so the test
   names and fails that impl. **Idempotent-skip second-parent guard
   (G2-1):** a bead carrying an existing binding with the CORRECT
   merge-SHA but an EMPTY (and, a sibling sub-case, a WRONG)
   `mindspec_landed_second_parent` re-completes → the binding is
   RE-WRITTEN (not skipped) so the store ends with both keys agreeing
   with the located merge; RED against the merge-SHA-only skip. Anti-
   drift pointer pins: `mergeBindingFn == bead.MergeMetadata`,
   `mergeBindingReadFn == bead.GetMetadata`, `locateLandedMergeFn ==`
   the real locate (AC-11(i)'s executor half, cited by Bead 4).

**Verification**
- [ ] `go test -short ./internal/executor/... ./internal/gitutil/...` passes; final test names per AC recorded in review evidence
- [ ] AC-1/AC-1c subtests RED on the spec-init SHA (silent-nil + exact-subject miss) and red again on fix-revert; AC-3/AC-4b red today; AC-4b additionally demonstrated red against a count/merge-base classifier (deviation-target named in-test)
- [ ] AC-4 quiet-path guard green today and after; refusal messages route through `guard.NewFailure` with the named re-run recovery (ADR-0035)
- [ ] `rg -n '"Merge "\+beadBranch|errNoLandedMergeIdentified' internal/executor/mindspec_executor.go` shows the exact-subject gate and silent-nil swallow retired; the conflict-recovery line prints `-m "Merge <beadBranch>"` (string-asserted)
- [ ] Seam-default pointer pins green; `go build ./... && go test -short ./... && golangci-lint run ./...` clean; `mindspec validate spec 125-…` passes; bead completes with zero `--override-adr` — and the orchestrator verifies THIS bead's own binding via `bd show <id> --json` (first live proof)

**Acceptance Criteria**
- [ ] AC-1 — binding persisted by real `CompleteBead` through the pinned conflict-recovery shape (RED today)
- [ ] AC-1c — both `FinalizeEpic` producer legs persist, incl. the default-subject already-ancestor sub-case (RED today)
- [ ] AC-3 — genuine miss is loud, fail-closed, cleanup-suppressing, convergent (RED today)
- [ ] AC-4 — true orphan positively classified quiet on first-parent membership (guard)
- [ ] AC-4b — merged-with-own-commits forced miss is LOUD; red against the count-classifier re-hiding impl (RED today AND against the wrong fix)

**Depends on**
None (W1 root; parallel with Bead 2 — zero shared files).
(Human-readable narration only — bd edges are wired exclusively from
`work_chunks[].depends_on`.)

## Bead 2: Revert-vs-evolved discrimination — `RevertShape` sub-classification of CleanDivergence

R3 in full, confined to the `SubsumptionCleanDivergence`-on-EVOLVED
false-negative (the 8nhe.2 bug). Content evolved by later honest work
identifies; genuinely backed-out content still refuses; the Conflict
arm, the trichotomy, and every boolean projection are byte-identically
preserved.

**Steps**
1. `internal/gitutil/neteffect.go`: add the SIBLING primitive
   `RevertShape(workdir, mergeSHA, target string) (bool, error)` — the
   reverse un-apply three-way `merge-tree(base = M, ours = target,
   theirs = M^1)`: revert-shape (true) iff the un-apply is CLEAN and
   its result tree EQUALS target's tree (the tip carries none of M's
   introduced content at any of its sites — the `git revert M` shape
   AND the clean-full-removal residual, both refusing per spec);
   false when the un-apply CHANGES the tip (content present, wholly or
   partially) or CONFLICTS (tip built on M's region — evolved).
   Fail on a <2-parent `mergeSHA` (no `M^1` guessing; octopus/non-bead
   candidates are excluded by callers and rejected here). **Infra-error
   discipline (plan-gate O2-1):** on ANY git/tree-resolution failure
   `RevertShape` PROPAGATES the error (returns `(false, err)` with the
   error non-nil) — it MUST NEVER map an infra failure to a definite
   classification, and the `landed.go` caller must treat a non-nil
   error as a fail-closed infra refusal (the existing `subErr`
   handling at `:376-378`), never as "identify". Doc comment words the
   predicate in the primitive's own terms (the spec's round-3
   prose-direction note) and names the deliberate false-negative
   floor.
2. `ContentSubsumedOutcome`, `ContentSubsumed`, `NetEffectLanded`
   (`neteffect.go:217-258`): UNTOUCHED — grep-confirmably
   byte-identical (AC-6; the spec's Conflict-arm scope pin).
3. `internal/lifecycle/landed.go`: new seam `landedRevertShapeFn =
   gitutil.RevertShape` beside `landedContentSubsumedFn` (`:105`);
   the revert leg (`:379-382`) becomes: on
   `SubsumptionCleanDivergence`, consult
   `landedRevertShapeFn(root, m.SHA, specBranch)` — revert-shape →
   the existing reverted-after-landing refusal (message updated to
   state "content no longer present" honestly covering both
   revert and clean-removal); NOT revert-shape → fall through to
   identify (the 8nhe.2 fix). `SubsumptionConflict` and
   `SubsumptionLanded` arms untouched.
4. AC-5 fixtures (`landed_test.go`): the 8nhe.2 PARTIAL-supersession
   shape — merge M lands content across two surfaces; later
   first-parent commits remove/supersede ONE surface while M's other
   content remains at the tip (removed-and-replaced / M's region
   reworked — explicitly NOT a pure delete/relocate). The fixture
   ASSERTS ITS OWN SHAPE-PRECONDITION: `ContentSubsumedOutcome(base =
   M^1, ref = M, target = tip)` == `SubsumptionCleanDivergence` on the
   fixture (so it cannot drift into the already-green conflict shape),
   then asserts `FindLandedMerge` identifies M (the RED-today core).
   SEPARATE sub-case (a GREEN anti-false-ID guard, NOT a RED-today
   assertion — F1 MINOR): later work on a DIFFERENT region of a file M
   also touched → M still identified — this shape is already a
   `SubsumptionConflict`/`SubsumptionLanded` today (identified), so it
   guards against a per-file-path REGRESSION, it does not fail on the
   spec-init SHA.
5. AC-6 guards: `git revert -m 1 M`, no later rework → refused with
   the revert-naming message; a pure full clean removal of M's paths →
   REFUSES (residual floor asserted as deliberate, in-test); spec-121
   revert-then-reapply + cherry-pick-reapply fixtures
   (`landed_test.go:501`, `:526`) stay green; F2-2r Probes 1+2
   (`:712`, `:760`) stay green; octopus/non-bead candidate excluded
   (never run through the discrimination); `neteffect_test.go`
   trichotomy + projection contracts unchanged; NEW guard: a
   `SubsumptionConflict` outcome still falls through to identify
   (behavior-scoped, not line-span). Anti-drift pointer pin:
   `landedRevertShapeFn == gitutil.RevertShape`.

**Verification**
- [ ] `go test -short ./internal/gitutil/... ./internal/lifecycle/...` passes; final names per AC in review evidence
- [ ] AC-5 subtests RED on the spec-init SHA (refused as "reverted after landing") and red on revert; the shape-precondition assertion is itself part of the test
- [ ] AC-6 guard set green today AND after: reapply fixtures, F2-2r probes, trichotomy/projection contracts, Conflict-arm fall-through, residual-floor refusal, octopus exclusion
- [ ] `git diff --stat` (review evidence) shows `ContentSubsumedOutcome`/`ContentSubsumed`/`NetEffectLanded` bodies untouched; `landedRevertShapeFn` pointer pin green
- [ ] `go build ./... && go test -short ./... && golangci-lint run ./...` clean; `mindspec validate spec 125-…` passes; bead completes with zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-5 — evolved-content-PRESENT CleanDivergence shape identifies, at changed-content granularity, with the shape-precondition asserted (RED today); different-region sub-case identifies (GREEN regression guard, not RED-today)
- [ ] AC-6 — true revert refuses; residual floor refuses (asserted deliberate); reapply/boolean/Conflict-arm contracts byte-identically preserved; octopus excluded (guard)

**Depends on**
None (W1; parallel with Bead 1 — Bead 1 edits `gitops.go`, this bead
edits `neteffect.go` + the `landed.go` revert leg). (bd edges wired
from `work_chunks[].depends_on`.)

## Bead 3: Exact-second-parent OWNERSHIP identity in `FindLandedMerge` + the MF-3 e2e

R5 in full at the read site, plus R6(a)/(b)/(e). Identification moves
off the exact-subject gate onto the two-source root-of-trust model:
subject-parsed bead-branch name NOMINATES ownership (both subject
forms, full-name equality), topology proves landed-ness (exact
second-parent match against a real two-parent merge), the
binding/panel-sha is a git-corroborated CACHE — fail-closed on every
ambiguity, oldest-anchored on same-second-parent re-merges.

**Steps**
1. `internal/lifecycle/landed.go`: add the ownership parser
   `parseMergeSubjectBeadBranch(subject) (branch string, present bool)`
   with a CONSERVATIVE three-state contract (plan-gate G2-2): it
   returns (i) `(branch, true)` when a bead-branch-name token is
   present in ANY recognizable position — `Merge bead/<id>`, git's
   default `Merge branch 'bead/<id>' into '…'`, or the bare
   `Merge branch 'bead/<id>'` form; (ii) `("", false)` ONLY when
   genuinely NO `bead/…` token appears in any recognizable position
   (the true anonymous-subject case). It MUST NOT collapse "a `bead/…`
   token is present but in a shape I don't fully parse" into the
   no-bead bucket — if a `bead/…`-shaped token is detected at all, it
   is treated as PRESENT-and-named (nominating THAT bead), so a
   different-bead subject in an unrecognized format REJECTS rather than
   passing the names-no-bead exception (AC-2d-style misattribution
   guard, new sub-test below). Ownership comparison is FULL
   branch-name EQUALITY against `workspace.BeadBranch(beadID)` — never
   `HasPrefix`/`Contains` (AC-2f). The parsed token is COMPARED DATA
   only, never a git operand, and is `termsafe.Escape`-d in any
   refusal message (the G2-1 discipline already at `:331`).
2. Rewrite the `FindLandedMerge` identity loop (`:257-345`):
   - Candidate generation, ONE admissible entry point (REVISED by the
     codex final-review G-1 BLOCKING fix): the SUBJECT-SCAN path —
     two-parent first-parent merges on `spec` whose subject NAMES THIS
     bead (ownership, either subject form), via the parser over the
     existing `firstParentMergesFn` scan. The originally-planned
     BINDING-SHA anonymous-subject entry point (G1-F2) is **REMOVED**:
     git-corroborating a binding proves only that the merge is REAL
     with a given exact second parent — NOT that it is THIS bead's — so
     a binding-alone entry point makes the agent-writable binding an
     independent OWNERSHIP authority, and a METADATA-forge (below the
     git-history threat boundary) on a never-landed bead pointing at any
     real anonymous merge would be identified (an unsafe false-positive).
     A merge whose subject names NO bead is therefore NOT auto-
     identifiable and FAILS CLOSED; its recovery is the EXPLICIT audited
     `mindspec reattest` (Bead 4). Bead 1's `ExactSecondParentMerges` is
     reused for topology corroboration.
   - Corroborating data are EXACT-EQUALITY only: surviving branch tip
     == second parent; `reviewed_head_sha` == second parent; binding
     resolving to a REAL exact merge. The current ancestor-TOLERANT
     confirmation legs (`:285-294`, `:301-307`, `:323-333`) are
     REMOVED — an ancestor-only-consistent candidate is never a
     positive identification (AC-2b/AC-2c).
   - The binding is an ownership CACHE over an already-subject-OWNED
     candidate ONLY (this-bead only — no longer this-bead-OR-no-bead):
     it confirms a candidate whose subject already names THIS bead. A
     binding pointing at a merge whose subject names a DIFFERENT bead
     (parser `present == true`, different name — including the
     unrecognized-format case per Step 1's conservative rule) never
     even generates a candidate (AC-2d); one pointing at no real exact
     merge is discarded (forgery, AC-2c's second half); one pointing at
     a real ANONYMOUS merge cannot generate a candidate either (G-1
     fail-closed).
   - Preserve Bead 2's just-landed CleanDivergence/Conflict split
     VERBATIM (plan-gate O1-2): the loop rewrite touches candidate
     GENERATION and corroboration, NOT the revert-leg
     `SubsumptionConflict → identified` fall-through — AC-6 guards
     that arm's behavior across this rewrite.
   - No exact-and-owned match ⟹ REFUSE: `*LandedMergeNoEvidence`
     naming the best candidate when one exists (preserving the
     `landed_test.go:213` no-datum contract — AC-10), plain
     `ErrLandedMergeNotFound` when none.
   - Same-second-parent exact matches (same owner's re-merges):
     report the NEWEST as `*LandedMerge.SHA`; anchor the R3 check
     VERBATIM on the OLDEST merge M₁ —
     `landedContentSubsumedFn(root, M₁^1, M₁, specBranch)` +
     Bead 2's `landedRevertShapeFn` on M₁ — never the newest's base
     (AC-2e; single-merge case reduces to R3 exactly).
3. Misattribution suite (`landed_test.go`): **AC-2b** — X then
   descendant Y, both default subjects: `FindLandedMerge(X)` returns
   M_X never M_Y; after reverting only X's content, X REFUSES (revert
   leg reads M_X). **AC-2c** — X's branch deleted, no binding, an
   ANCESTOR `reviewed_head_sha`, M_Y present, NO exact match for X:
   REFUSES, never newest-ancestor; plus the forged-binding
   (no-real-merge) discard. **AC-2d** — X carries a binding whose
   second-parent equals Z's real landed tip: discarded on OWNERSHIP,
   refuse. **AC-2e** — M₁ landed, reverted, empty re-merge M₂ of the
   same second parent: names M₂ but classifies REVERTED via the
   M₁-anchored check; red against a newest-anchored impl for EITHER
   parameter. **AC-2f** — `mindspec-8nhe.1` vs `mindspec-8nhe.12`
   full-name equality, collision fails SAFE. **AC-10** — the
   no-datum `*LandedMergeNoEvidence` guard stays green.
   **Anonymous-subject REFUSE direction (codex final-review G-1
   BLOCKING fix — was ACCEPT):** a merge whose subject names NO bead (a
   wholly-custom subject), EVEN with a valid-looking complete-time
   binding pointing at that real exact merge, is NOT auto-identified —
   the automatic path FAILS CLOSED (the binding cannot supply ownership
   independently). Pinned by `TestFindLandedMerge_AnonymousSubject
   BindingRefuses` AND the exploit
   `TestFindLandedMerge_ForgedBindingAtRealAnonymousMergeRefuses` (a
   forged binding on a NEVER-landed bead pointing at some OTHER work's
   real anonymous merge → refuse; RED against the removed binding-SHA
   impl, which identifies the unrelated merge). **Different-bead-
   unrecognized-format REJECT (plan-gate G2-2):** a merge whose subject
   references `bead/Z` in a format the parser does not fully recognize
   nominates `bead/Z` (present-but-not-this-bead), so it never generates
   a candidate for X even with a corroborating binding — refuse.
4. NEW `internal/executor/landed_e2e_test.go` (external
   `executor_test` package; imports `internal/lifecycle` TEST-ONLY —
   the production import boundary stands): **AC-1b** — the full R6(a)
   chain: 2nd-bead add/add conflict on `.beads/issues.jsonl` →
   `-m`-less recovery merge landing the DEFAULT subject → recovery
   re-run of production `CompleteBead` → binding persisted (Bead 1) →
   branch deleted → `lifecycle.FindLandedMerge` POSITIVELY identifies
   (this bead); plus the string assertion that
   `beadToSpecConflictFailure`'s printed recovery line carries `-m`.
   **AC-2** — continuing AC-1's end state, `FindLandedMerge` returns
   the positive `*LandedMerge` for the same commit — the exact
   contract spec 124's MF-3 consumes.
   **e2e MECHANISM (plan-gate F3-1/G1-F1):** because this test lives in
   an external `_test` package, the unexported binding seams
   (`mergeBindingFn`, `mergeBindingReadFn`, `landedBindingMetadataFn`)
   are UNREACHABLE from it — it therefore drives the PRODUCTION seam
   DEFAULTS end-to-end against a stateful **fake-`bd`-on-PATH** (the
   existing `internal/approve` PATH-scoped scratch-`bin/` pattern,
   `plan_fault_test.go:401-419` — a small `bd` shim that persists
   metadata to a scratch file so `MergeMetadata`'s write and
   `GetMetadata`'s read see the SAME store). This is STRONGER than seam
   stubbing (it exercises the real `bead.MergeMetadata`/`GetMetadata`
   code paths) and stays CI-hermetic without a real `bd` — and it does
   NOT trip R6's bd-less no-skip falsifier: the fake bd IS its declared
   environment, so it FATALs (never skips) if the shim is absent, per
   the spec-119 convention.
5. Consumers untouched: `internal/complete`'s merged-unclosed
   reconcile and the doctor stale-OPEN cross-check keep their
   error-classification contracts (`errors.Is/As` via the preserved
   `Unwrap`); no message edits outside `landed.go`.

**Verification**
- [ ] `go test -short ./internal/lifecycle/... ./internal/executor/...` passes; final names per AC in review evidence
- [ ] AC-1b/AC-2 RED on the spec-init SHA (default subject never matches; no binding exists); each AC-2b–2f subtest demonstrates red against its NAMED wrong impl (naive ancestor scan / topology-only cache / newest anchor / prefix ownership)
- [ ] AC-10 (`TestFindLandedMerge_BranchDeletedNoBindingNoEvidence` contract) green before and after; `MergedUnclosed`/stale-OPEN classification tests unchanged
- [ ] Local bd-on-PATH proof recorded: real `mindspec complete` of a scratch bead then `bd show <bead> --json | jq -e '.metadata.mindspec_landed_merge_sha'` (Validation Proofs)
- [ ] `go build ./... && go test -short ./... && golangci-lint run ./...` clean; `mindspec validate spec 125-…` passes; bead completes with zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-1b — the pinned conflict-recovery miss shape ends identified for the branch-deleted bead; recovery line identifiable (RED today)
- [ ] AC-2 — branch-deleted identification, the MF-3 contract (RED today)
- [ ] AC-2b — exact-match-exists misattribution guard (red vs naive scan)
- [ ] AC-2c — fail-closed on no-exact-match ambiguity + forged-binding discard (red vs newest-ancestor)
- [ ] AC-2d — ownership check discards another-bead's-real-merge cache (red vs topology-only)
- [ ] AC-2e — oldest-anchored content check on same-second-parent re-merges (red vs newest-anchored)
- [ ] AC-2f — full-name-equality ownership, prefix collision fails safe (red vs prefix impl)
- [ ] AC-10 — no-datum refusal preserved (guard)

**Depends on**
Beads 1 and 2 (consumes Bead 1's persistence + shared exact-match
primitive + `-m` fix for the e2e; consumes Bead 2's discrimination for
the M₁-anchored check; serializes the `landed.go` seam).
(bd edges wired from `work_chunks[].depends_on`.)

## Bead 4: `mindspec reattest` — git-corroborated re-attest verb + ADR-0041 §2(ii) amendment

R4 in full plus the amendment finalization and AC-11. An explicit,
operator-invoked, fail-closed recovery for already-merged bindingless
beads: derives the binding from independent git topology under the
R5 rule, recovers exactly the corroborable subset, refuses the rest to
the audited q9ea exit, and leaves an inspectable audit record.

**Steps**
1. `internal/lifecycle/reattest.go`: `ReattestLandedMerge(root,
   specBranch, beadID, actor string)` — the derivation engine, reusing
   Bead 3's parser and the shared exact-match/oldest-anchor helpers:
   - NOMINATE ownership by SUBJECT: two-parent first-parent merges on
     `spec` whose subject names THIS bead (the deliberately
     more-permissive EXPLICIT-path datum — it lives ONLY here and MUST
     NOT leak into `FindLandedMerge`, which Bead 3's AC-10 pins).
     Ownership for a bare bead is subject-NOMINATED, NOT independent
     (plan-gate G3-B1, aligned with the amendment): the subject names
     which bead, topology proves landed-ness — datum (a) is not
     claimed as independent-of-subject ownership proof. This is the
     documented, operator-vouched trust in the subject-to-name mapping
     spec 121 already relied on, inside the threat boundary.
   - CORROBORATE LANDED-NESS per the amendment's standalone data: (a)
     the git-derived exact scan itself (branch-deleted happy path — a
     REAL exact second-parent merge exists; topology, not the subject,
     is the corroboration, so it is non-circular); (b) a surviving
     bead-branch tip the exact match must EQUAL; (c) a registered panel
     `reviewed_head_sha` equal to an exact match's second parent;
     (d) an existing binding git-corroborated per R5. When multiple
     data are present they must AGREE.
   - FAIL CLOSED: owned candidates with DIFFERENT second parents →
     ambiguity → refuse; a subject-matching decoy whose second parent
     contradicts a surviving tip/panel datum → refuse; an
     operator-ASSERTED merge/second-parent pair is NOT admissible
     (circular — there is no flag or argument to supply one);
     same-second-parent re-merges → newest names the merge, R3's
     check anchored on M₁ governs landed-vs-reverted, and a REVERTED
     classification refuses (writes nothing, names the state).
   - WRITE only the scan-derived SHAs, via a seam
     `reattestBindingFn = bead.MergeMetadata` (same family, pointer-
     pinned — AC-11(i)'s re-attest half), in ONE call together with
     the audit keys (`mindspec_landed_reattest_*`, plan-level choice
     above): actor/authority, before/after values (empty-string
     before when absent), RFC3339 timestamp, invoking operation,
     corroborating datum. Already-correct binding → no-op re-run
     (byte-identical, no duplicate audit churn). Contradictory
     existing binding → G3-1: overwrite ONLY with the git-corroborated
     exact identity, prior value recorded in the audit — never
     silently keep, never write uncorroborated.
   - REFUSE the truly-bare state (no owned exact merge, no datum) to
     the audited ADR-0035 q9ea attested-restore exit BY NAME — honest
     recovery scope, no fleet-wide claim.
2. `cmd/mindspec/reattest.go` + `root.go`: register the top-level
   `reattest <bead-id>` cobra command (the `completeCmd` pattern).
   `specBranch` precedence is FALLBACK-ONLY (plan-gate F2-2): the
   bead's epic linkage (bead → parent epic → `spec/<id>`) is tried
   FIRST and WINS whenever derivable; `--spec-branch` is consulted
   ONLY when the linkage is underivable — it is scoping input (WHERE
   to scan), never a corroboration substitute, and the branch actually
   scanned is recorded in the audit (`mindspec_landed_reattest_
   scanned_branch`) so a mis-scoped invocation is reconstructable.
   Refusals via `guard.NewFailure` naming the state and forward exit;
   NO bypass flag exists (AC-8's flag-surface assertion enumerates the
   flag set, incl. that `--spec-branch` cannot disable corroboration);
   NOT invoked or written-through by `doctor` (AC-7 asserts). Update
   `help_golden_test.go` for the new verb.
3. Finalize the ADR-0041 amendment (pre-drafted at plan time in this
   worktree — remove the `PRE-DRAFT` marker comment; adjust wording
   only where the implementation forces it), keeping the AC-11
   anchors: derives-from-discipline-not-write-time, the STANDALONE
   (a)–(e) enumeration, the q9ea-blessed-exit clause, the audit-field
   list, "detectable-by-inspection, NOT cryptographically
   tamper-proof", and the honest recovery scope. The re-attest code
   cites the amendment (§2(ii)) in its doc comment so the
   ADR-divergence gate sees the declared touchpoint. Add the anchor
   test asserting the amendment strings are present, the marker is
   gone, and `rg 're-attest|reattest'` hits the ADR (the spec's
   Validation Proofs grep).
4. Tests: **AC-7** — fleet-state fixture (closed bead, real
   default-subject merge, branch deleted, no binding): explicit
   invocation derives + writes SHAs matching the real merge
   (rev-parse-verified); audit record inspectable in the hermetic
   store with all fields; `FindLandedMerge` subsequently identifies;
   re-run no-op; not-writable-via-doctor asserted. **AC-8** — (i)
   subject-only (no admissible corroboration → wait, subject IS the
   nominator here: the leg is a nominated candidate whose topology
   does NOT hold — e.g. named merge is not a real two-parent
   first-parent merge, or competing owned candidates with different
   second parents): refuses, metadata byte-identical, names the
   forward exit; (ii) crafted decoy merge contradicting the bead's
   surviving tip AND a descendant bead's merge: neither attested;
   (iii) no operator-asserted-pair surface exists + flag-set
   enumeration proves no bypass. **AC-9** — stale/contradictory
   binding: overwritten only with the exact-match identity, prior
   value in the audit, reconstructable by inspection. **AC-11** —
   seam pointer pins (re-attest half + citing Bead 1's executor-half
   pins) + the ADR anchor test.

**Verification**
- [ ] `go test -short ./internal/lifecycle/... ./cmd/mindspec/...` passes; final names per AC in review evidence
- [ ] AC-7 RED today (no surface exists); AC-8 decoy/descendant/circular legs red-on-lie; AC-9 audit reconstruction demonstrated; refusal messages carry named forward exits (ADR-0035)
- [ ] `rg -n 're-attest|reattest' .mindspec/adr/ADR-0041-gate-before-mutate.md` non-empty; `rg -n 'PRE-DRAFT' .mindspec/adr/ADR-0041-gate-before-mutate.md` empty; re-attest code cites §2(ii); anchor test green
- [ ] Help/golden updated for the new verb; flag-set enumeration shows no bypass flag; `doctor` writes nothing (asserted)
- [ ] `go build ./... && go test -short ./... && golangci-lint run ./...` clean; `mindspec validate spec 125-…` passes; bead completes with zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-7 — re-attest happy path, git-DERIVED, audited, no-op convergent, explicit-invocation-only (RED today)
- [ ] AC-8 — fail-closed + non-circular: no-corroboration/decoy/descendant/asserted-pair legs all refuse; no bypass flag
- [ ] AC-9 — contradiction handling audited per G3-1, reconstructable by inspection
- [ ] AC-11 — seam anti-drift pins (both halves) + ADR amendment finalized with anchors, marker removed, code citation present

**Depends on**
Bead 3 (the derivation IS R5's rule — reuses its parser and
exact-match/oldest-anchor helpers; the path-asymmetry pin needs
Bead 3's automatic-path refusals landed). (bd edges wired from
`work_chunks[].depends_on`.)

## Provenance

Every spec AC maps to exactly ONE owning bead. Two narrated
split-property notes, disjoint by construction (the 122 precedent):
AC-1b's recovery-message `-m` EDIT lands in Bead 1 (Step 4) while
Bead 3 OWNS the AC and asserts the full end-to-end including that
line; AC-11's executor-half seam pins land in Bead 1 (Step 5) while
Bead 4 OWNS the AC, adds the re-attest-half pin + ADR anchors, and
cites Bead 1's pins in evidence.

| Acceptance Criterion | Satisfied By | Verified By |
|---------------------|--------------|-------------|
| AC-1 (binding persisted through the conflict shape; RED today) | Bead 1 Steps 1–3, 5 | Bead 1 verification: conflict-shape `CompleteBead` e2e, hermetic store |
| AC-1b (pinned conflict-recovery miss, end-to-end + identifiable recovery line; RED today) | Bead 3 Step 4 (consuming Bead 1 Steps 3–4) | Bead 3 verification: `landed_e2e_test.go` full chain + `-m` string assertion |
| AC-1c (FinalizeEpic legs, default-subject already-ancestor; RED today) | Bead 1 Step 5 | Bead 1 verification: both producer-leg fixtures |
| AC-2 (branch-deleted identification, the MF-3 contract; RED today) | Bead 3 Step 4 | Bead 3 verification: AC-1 end-state continuation |
| AC-2b (exact-match-exists misattribution; red vs naive scan) | Bead 3 Steps 2–3 | Bead 3 verification: X/descendant-Y fixture + revert leg |
| AC-2c (fail-closed on no exact match; forged binding discarded) | Bead 3 Steps 2–3 | Bead 3 verification: ancestor-panel-sha + decoy fixtures |
| AC-2d (ownership discards another bead's real merge) | Bead 3 Steps 2–3 | Bead 3 verification: X-binding→Z-tip fixture |
| AC-2e (oldest-anchored content check on re-merges) | Bead 3 Steps 2–3 (consuming Bead 2 Step 1) | Bead 3 verification: revert-then-empty-re-merge fixture, both-parameter red |
| AC-2f (full-name-equality ownership, prefix-safe) | Bead 3 Steps 1, 3 | Bead 3 verification: 8nhe.1/8nhe.12 fixture |
| AC-3 (genuine miss loud, cleanup-suppressing, convergent; RED today) | Bead 1 Steps 2–3, 5 | Bead 1 verification: seam-forced miss fixture + convergence re-run |
| AC-4 (nothing-to-bind positively classified quiet) | Bead 1 Steps 3, 5 | Bead 1 verification: orphan quiet-path guard |
| AC-4b (forced miss on merged bead LOUD; red vs count classifier) | Bead 1 Steps 3, 5 | Bead 1 verification: merged-then-ancestor fixture, wrong-impl demonstration |
| AC-5 (evolved CleanDivergence identifies, precondition asserted; RED today) | Bead 2 Steps 1, 3–4 | Bead 2 verification: 8nhe.2 partial-supersession + different-region subtests |
| AC-6 (true revert refuses; residual floor; contracts preserved) | Bead 2 Steps 2, 5 | Bead 2 verification: guard suite + byte-identical diff evidence |
| AC-7 (re-attest happy path, git-derived + audited; RED today) | Bead 4 Steps 1–2, 4 | Bead 4 verification: fleet-state fixture + audit inspection |
| AC-8 (fail-closed, non-circular, no bypass) | Bead 4 Steps 1–2, 4 | Bead 4 verification: three refusal legs + flag-set enumeration |
| AC-9 (contradiction overwrite audited, G3-1) | Bead 4 Steps 1, 4 | Bead 4 verification: stale-binding fixture, before/after reconstruction |
| AC-10 (no-datum refusal preserved) | Bead 3 Steps 2, 3 | Bead 3 verification: `landed_test.go:213` contract guard |
| AC-11 (seam anti-drift + ADR anchor; amendment RED until landed) | Bead 4 Steps 1, 3–4 (executor-half pins landed by Bead 1 Step 5) | Bead 4 verification: pointer pins + anchor test + PRE-DRAFT-gone rg |

R6's six classes are distributed, never a test-only bead: (a) →
Bead 1 AC-1 / Bead 3 AC-1b+AC-2; (b) → Bead 3 AC-10; (c) → Bead 2
AC-5/AC-6; (d) → Bead 1 AC-3/AC-4b; (e) → Bead 3 AC-2b–2f; (f) →
Bead 1 AC-1c. The spec's Validation Proofs commands are distributed
per-bead (each runs its package subset; the ADR rg from Bead 4; the
bd-on-PATH `bd show … --json | jq` proof at Bead 3 and live on this
spec's own completing beads). Follow-up filing (the Conflict-arm
content-presence discriminator) and the operator-run historical
backfill are orchestrator close-out work, not beads.
