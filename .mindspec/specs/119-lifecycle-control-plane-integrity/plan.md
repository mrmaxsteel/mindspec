---
adr_citations:
    - ADR-0037
    - ADR-0030
    - ADR-0040
    - ADR-0032
approved_at: "2026-07-18T10:17:04Z"
approved_by: user
bead_ids:
    - mindspec-lc12.1
    - mindspec-lc12.2
    - mindspec-lc12.3
    - mindspec-lc12.4
    - mindspec-lc12.5
    - mindspec-lc12.6
spec_id: 119-lifecycle-control-plane-integrity
status: Approved
version: "5"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/complete/complete.go
        - internal/complete/complete_test.go
        - internal/complete/reconcile_test.go
        - internal/lifecycle/landed.go
        - internal/lifecycle/landed_test.go
        - internal/gitutil/gitops.go
        - internal/gitutil/gitops_test.go
        - internal/resolve/target.go
        - internal/resolve/target_test.go
        - cmd/mindspec/complete.go
    - depends_on:
        - 1
        - 3
      id: 2
      key_file_paths:
        - internal/lifecycle/stale_open.go
        - internal/lifecycle/stale_open_test.go
        - internal/doctor/stale_open.go
        - internal/doctor/stale_open_test.go
        - internal/doctor/finalize_orphans.go
        - internal/doctor/finalize_orphans_test.go
        - internal/doctor/doctor.go
        - internal/instruct/instruct.go
        - internal/instruct/templates/idle.md
        - .github/workflows/ci.yml
        - .mindspec/domains/workflow/OWNERSHIP.yaml
    - depends_on: []
      id: 3
      key_file_paths:
        - internal/executor/executor.go
        - internal/executor/mock.go
        - internal/executor/mindspec_executor.go
        - internal/executor/finalize_scoping_test.go
        - internal/executor/executor_test.go
        - internal/executor/finalize_orphan_test.go
        - internal/executor/finalize_worktree_only_test.go
        - internal/executor/layout_guard_test.go
        - internal/executor/merge_conflict_test.go
        - internal/lifecycle/finalize_orphans.go
        - internal/lifecycle/finalize_orphans_test.go
        - internal/gitutil/gitops.go
        - internal/gitutil/gitops_test.go
        - internal/phase/derive.go
        - internal/phase/derive_test.go
        - internal/approve/impl.go
        - internal/approve/impl_test.go
    - depends_on: []
      id: 4
      key_file_paths:
        - internal/approve/spec.go
        - internal/approve/spec_test.go
        - internal/approve/plan.go
        - internal/approve/plan_test.go
        - internal/approve/scaffold_roundtrip_test.go
    - depends_on:
        - 1
      id: 5
      key_file_paths:
        - internal/complete/bead_scope.go
        - internal/complete/bead_scope_test.go
        - internal/approve/plan.go
        - internal/approve/plan_lint_test.go
        - internal/instruct/run_test.go
    - depends_on:
        - 1
        - 3
        - 4
        - 5
      id: 6
      key_file_paths:
        - .mindspec/adr/ADR-0041-gate-before-mutate.md
        - internal/complete/fault_injection_test.go
        - internal/approve/fault_injection_test.go
        - internal/executor/mindspec_executor.go
        - internal/executor/finalize_fault_test.go
        - internal/complete/complete.go
        - internal/approve/impl.go
        - internal/approve/plan.go
---
# Plan: 119-lifecycle-control-plane-integrity

Six beads implement the gate-before-mutate contract on the residual lifecycle
paths. The decomposition follows the spec's natural requirement clusters and
keeps the dependency DAG shallow (longest chain: 2). Only genuinely consumed
outputs are declared as edges:

- Bead 2 (doctor) consumes the exported landed-merge predicate Bead 1
  introduces in `internal/lifecycle` (AC-12 anti-drift demands the SAME
  function) and the exported finalize-orphan predicates Bead 3 introduces
  in `internal/lifecycle` (P8: `internal/doctor` AND `internal/instruct`
  must both be able to import them, which a doctor-package-private or
  executor-private home forbids) — so `depends_on: [1, 3]`.
- Bead 5 hooks the advisory scope WARN into the gate-evaluation phase of the
  preflight pipeline Bead 1 restructures — it consumes Bead 1's produced
  preflight seam (and sequencing avoids two conflicting restructures of
  `internal/complete/complete.go`) — so `depends_on: [1]`.
- Bead 6 fault-injects the FIXED mutation sequences of all three verbs and
  inserts the ADR-0041 citation into their new preflight code, so it must
  follow every bead that rewrites those sequences: Bead 1 (`complete`'s
  pipeline), Bead 3 (`impl approve` preflight + `FinalizeEpic`), Bead 4
  (`plan approve` preflight + the dep-wiring loop at
  `internal/approve/plan.go:361-373` whose partially-wired death state
  Bead 6's p2 convergence test constructs), and Bead 5 (the last edit of `complete.go`'s gate
  phase) — so `depends_on: [1, 3, 4, 5]`. The DAG depth stays 2 (roots
  1/3/4 → {2, 5, 6}).
- Beads 1, 3, 4 are mutually independent roots. Two shared-file adjacencies
  exist WITHOUT edges (shared source files are not dependencies) — flagged
  here for the implementers:
  - Beads 4 ∥ 5 both touch `internal/approve/plan.go` in DISJOINT functions
    (Bead 4: the preflight hoist in `ApprovePlan`/`createImplementationBeads`
    and the two new warnings; Bead 5: a NEW plan-lint pass function). No
    shared hunks; whichever merges second rebases trivially.
  - Beads 1 ∥ 3 both touch `internal/gitutil/gitops.go` in DISJOINT
    functions (Bead 1: a NEW appended `FirstParentMerges` helper; Bead 3:
    the existing `RemoteHeadSHA` body at `gitops.go:327-346` and the
    `PushBranchForceWithLease` doc comment). Same discipline.

Unresolved plan-level choices from the spec, resolved here:

- **AC-10 / R4 landed-work mechanism — landed-merge-commit identity**
  (REPLACES the round-1 first-parent-history-exclusion choice, which
  false-flags every fresh bead branch once any prior `--no-ff` merge puts
  commits on second-parent lineage). Grounding: every in-binary bead→spec
  merge is `gitutil.MergeInto` — `git merge --no-ff -m "Merge <beadBranch>"`
  run in the spec worktree with the spec branch checked out
  (`internal/gitutil/gitops.go:152-162`), invoked from `CompleteBead`
  (`internal/executor/mindspec_executor.go:302`) and from `FinalizeEpic`'s
  auto-merge (`mindspec_executor.go:410`). Therefore a landed bead merge is
  a commit that is (a) on the SPEC branch's first-parent chain (the spec
  branch is always the merge's first parent), (b) a ≥2-parent merge commit
  whose subject is exactly `Merge bead/<id>` (`workspace.BeadBranchPrefix`,
  `internal/workspace/worktree.go:21,43` — the deterministic `-m` format is
  the bead-ID tag embedded in every landed merge), and (c) whose SECOND
  parent is the landed bead tip. Two consequences do all the work:
  - **"bead did work" needs NO fork-point computation.** `git merge` of a
    branch that is already an ancestor of the current tip performs no merge
    and creates no commit ("Already up to date" — `--no-ff` only forces a
    merge commit where a fast-forward would otherwise happen). A freshly
    claimed bead branch has zero own commits (`DispatchBead` forks it AT the
    spec tip), is trivially an ancestor, and therefore can NEVER produce a
    landed merge commit — so keying on the existence of the identified merge
    commit provably cannot false-flag a fresh bead, regardless of how many
    `--no-ff` merges the spec branch carries.
  - **"work landed" is the identified commit itself** (it is on the spec
    branch), plus — when the `bead/<id>` ref still exists — the existing
    `IsAncestor(beadTip, specBranch)` confirmation so a branch carrying
    NEW unlanded commits on top of a landed merge is not flagged.
  Identification algorithm (Bead 1's exported predicate): scan the spec
  branch's first-parent merge commits newest-first (new
  `gitutil.FirstParentMerges` ≈ `git rev-list --first-parent --merges
  --format=%P%n%s <specBranch>`); accept the first candidate whose subject
  matches `Merge bead/<id>` AND whose every AVAILABLE corroboration agrees:
  (i) when a registered panel records `reviewed_head_sha` (captured at
  fan-out into `panel.json` — `internal/panel/panel.go:57`,
  `cmd/mindspec/panel.go:187`, read via `panel.Scan`/`panel.ForBead`), it
  must equal or be an ANCESTOR of the candidate's second parent (ancestor,
  not equality: `complete --commit-msg`'s own auto-commit advances the bead
  tip past `reviewed_head_sha` before the merge); (ii) when `bead/<id>`
  still exists, its tip must equal or be an ancestor of the second parent.
  A subject match contradicted by an available corroboration is NOT a
  positive identification (AC-8 refuses). `reviewed_head_sha` ancestry was
  deliberately NOT chosen as the primary key: panel-free beads (ADR-0037 §6)
  record no `reviewed_head_sha` at all, and a panel registered before the
  bead's first commit records the fork point itself — trivially an ancestor
  of the spec branch — which would false-flag; as a corroborating leg it
  only ever tightens. No reflog, no recorded creation base, no first-parent
  EXCLUSION arithmetic; derivable in fresh clones/CI from committed state
  alone. (Squash merges never occur on the bead→spec path — both call sites
  are `MergeInto`; squash-tolerance for spec→main is the spec's named
  follow-up.)
- **R4 landed-merge evidence anchor**: the reconcile path positively
  identifies the landed merge commit M via the SAME exported predicate;
  doc-sync/ADR gates evaluate the diff `M^1..M` (the landed content).
  No positive identification → refusal, never close.
- **AC-24 hermetic mechanism**: real `git init` in the temp sandbox plus
  `GIT_CEILING_DIRECTORIES` on the test environment. This confines the
  HERMETIC fix to `internal/instruct` test setup (workflow domain); the
  spec's conditional core declaration for the override variant is not
  exercised. Core is nonetheless touched — see the next bullet.
- **`internal/phase` (core) touch — Impacted-Domains reconciliation**: the
  spec declared core conditionally (only for the hermetic root-override
  variant). This plan adds a DIFFERENT, unconditional core touch: Bead 3
  exports a lifecycle-classification helper from `internal/phase` (P3 —
  `classifyChild` is unexported, `internal/phase/derive.go:272`, and
  `FinalizeEpic` cannot otherwise consume the classification). The core
  domain therefore IS impacted, by the classifier export rather than the
  hermetic fix; `internal/phase/**` is already in core's `OWNERSHIP.yaml`
  globs (per the spec's Impacted Domains note), so the divergence gate
  attributes it cleanly. The helper is bd-only (no git I/O), so
  `TestEnforcementHasNoGitLeaks` (ADR-0030's pin over `internal/phase`)
  is unaffected. Its ONLY caller is `ApproveImpl` in `internal/approve`,
  which ALREADY imports `internal/phase` (`impl.go:20`) — no new import
  edge anywhere, no cycle (`internal/phase` imports only
  `bead`/`state`/`workspace` — `derive.go:11-13`, `cache.go:10-11`). The
  EXECUTOR never calls it: `internal/executor` is forbidden from
  importing enforcement packages, and `internal/phase` is one
  (`internal/executor/executor.go:6-11`;
  `internal/lint/boundary_test.go:116`) — so `FinalizeEpic` receives the
  computed lifecycle allow-set as a PARAMETER instead (P6, Bead 3
  Step 2).
- **`.github/workflows/ci.yml` ownership**: claimed into workflow's
  `OWNERSHIP.yaml` (Bead 2), following the `.golangci.yml` precedent from
  spec 108 wave 2, so the divergence gate attributes the CI edit cleanly.
- **ADR-0041 status**: authored as Accepted in Bead 6 (precedent: ADR-0029,
  finalized within its shipping spec) — the contract it records ships in the
  same spec branch. AC-25 permits Accepted-or-Proposed; Accepted avoids a
  post-ship flip and satisfies the coverage gates immediately. It is not
  cited in this plan's `adr_citations` because it does not exist at
  plan-approve time (`adr-cite-missing` would fail the validator); the
  divergence-gate touchpoint is carried by the spec's ADR Touchpoints
  declaration and verified in Bead 6.
- **ADR-divergence verification command (named)**: the gate has no
  standalone CLI subcommand (`mindspec validate` exposes only
  spec/plan/docs); its two in-binary homes are the per-bead gate inside
  `mindspec complete` (`internal/complete/complete.go:523`,
  `validate.CheckADRDivergence`) and the whole-branch backstop inside
  `mindspec impl approve` (`internal/approve/impl.go:335`). AC-25's
  "divergence gate passes" is therefore verified by (a) Bead 6's OWN
  lifecycle completion — `mindspec complete <bead-6-id>` with ZERO
  `--override-adr`/`--supersede-adr`, whose diff contains ADR-0041 plus the
  citing preflight code, recorded in review evidence — and (b) a unit
  fixture in `internal/approve/fault_injection_test.go` (or a sibling)
  driving `validate.CheckADRDivergence` over a fixture diff touching the
  preflight files with ADR-0041 present vs absent.
- **Orphan-stats scope**: the finalize-orphan `CommitCount`/`DiffStat`
  (Bead 3) are computed over `origin/main..<ref>` — the remote-tracking ref
  as last updated by `FinalizeEpic`'s existing best-effort
  `git fetch origin main` (`mindspec_executor.go` — the fetch's
  degrade-to-stderr-warning behavior on offline/auth failure is the spec's
  NAMED DROP, 3xqm item 4, and is not escalated here); never bare local
  `main` (`mindspec_executor.go:433-437`'s current base), which can be
  arbitrarily stale in worktree-heavy checkouts.

## ADR Fitness

- **ADR-0037 (Panel Gate as Enforced Contract)** — remains the right
  contract, consumed UNCHANGED. Bead 1's merged-unclosed reconcile applies
  the decision ladder exactly as written: decisions (2) malformed-registration
  and (4) round-mismatch still Block on the reconcile path (they precede the
  missing-ref check); decision (5) MissingRef → Allow + Warn
  (`internal/panel/gate.go:139`, `:208-218`) is the ladder's own verdict in
  the deleted-ref state and is surfaced, not overridden; §6's
  registered-panels-only fail-open scope (`panel_advisory.go:176-177`) is
  preserved for panel-free beads. No amendment is made; the anti-bypass burden
  rides on positive landed-merge evidence plus the doc-sync/ADR gates against
  landed content (Requirement 4). The stronger fail-closed variant is the
  spec's named follow-up, out of scope.
- **ADR-0030 (Executor Boundary)** — remains fit, applied with its actual
  scope: enforcement packages (`internal/complete`, `internal/approve`,
  `internal/validate`, `internal/state`, `internal/phase` — pinned by
  `TestEnforcementHasNoGitLeaks`) consume git facts only through
  `executor.Executor`; Bead 1's preflight and reconcile git facts therefore
  flow through executor seams (mockable for AC tests). `internal/lifecycle`,
  `internal/doctor`, and executor-side code may use `internal/gitutil`
  directly (the shipped `orphans.go` precedent), which Beads 1 (the
  `landed.go` predicate), 2, and 3 follow. Bead 3's new exported
  `internal/phase` classifier helper performs bd queries only — no git —
  so the boundary pin over `internal/phase` is untouched; its consumer is
  `internal/approve` (an existing `phase` importer), NEVER the executor —
  the executor may not import enforcement packages
  (`executor.go:6-11`, `boundary_test.go:116`) and instead receives the
  computed allow-set as a `FinalizeEpic` parameter (P6).
  `internal/lifecycle` is NOT in the enforcement-package pin list, so
  Bead 3's finalize-orphan predicates live there with direct `gitutil`
  use, importable by doctor AND instruct (P8).
- **ADR-0040 (Orchestration Layering Ratchet)** — this spec is a direct
  application: operator-memory rules ("run complete from the right worktree",
  "check for stray finalize branches") ratchet into L1 in-binary gates
  (Beads 1, 3, 4) and doctor checks generated from the gates' own exported
  predicates (Bead 2). No change to the ADR.
- **ADR-0032 (ADR Semantic Gates)** — unchanged. The per-bead scope check
  (Bead 5) is explicitly advisory (WARN + plan-lint) and does not alter
  ADR-0032's hard ownership/divergence semantics. The doc-sync/ADR-divergence
  gates are re-anchored (evaluated against the landed merge commit on the
  reconcile path) but their decision rules are untouched.
- **ADR-0034 (Ceremony Collapse)** — its idempotent legacy-migration step is
  the one sanctioned pre-preflight mutation, in all THREE verbs
  (`complete.go:286-293`, `impl.go:186-192` — `phase.EnsureMigrated` at
  `:190`, `plan.go:70-76` — `:74`); Requirement 1's exemption is recorded in
  ADR-0041 rather than amending ADR-0034.
- **ADR-0041 (NEW, Bead 6)** — *Gate-Before-Mutate Lifecycle Verb Contract
  (preflight → commit → reconcile)*: all immutable gate facts resolved and
  every derivable refusal evaluated before the first state mutation (the
  idempotent ADR-0034 migration exempted); tracker-only commits never target
  protected `main`; recovery is always forward re-invocation converging to
  completion or a clean named refusal — never rollback. Domains: workflow,
  execution, core. Cited from the new preflight code in all three verbs so
  the ADR-divergence gate sees the declared touchpoint (AC-25).

## Testing Strategy

- **Unit tests via executor seams.** All `internal/complete` /
  `internal/approve` behavior is pinned with `MockExecutor` (per ADR-0030):
  the mock records the full call sequence, so tests assert ordering facts
  directly — "no commit created on `main`" (AC-3), "no merge-base call on the
  reconcile path" (AC-5), "gates evaluated against the landed merge commit"
  (AC-5/AC-6), "CommitCount retains its consensus-revision-9 position"
  (AC-17, `TestApproveImplCallOrder`), and the new facts-before-mutation
  call-order pins for `impl approve` (Bead 3) and `plan approve` (Bead 4).
- **Integration tests with real git fixtures.** `internal/gitutil`,
  `internal/lifecycle`, `internal/executor`, and `internal/doctor` tests
  build real throwaway repos (existing fixture helpers): planted decoy refs
  for `RemoteHeadSHA` (AC-16), `--no-ff` merged-then-deleted bead branches
  for the landed-merge predicate and the stale-OPEN check (AC-5, AC-10),
  planted `chore/finalize-*` branches and stale tracker commits on `main`
  (AC-15).
- **Fault injection (AC-26) — three SOUND kill mechanisms, matched to what
  each seam can actually enact.** `MockExecutor` performs NO real git or
  worktree mutation — its methods only record the call and return a
  configured error (`internal/executor/mock.go:132` and siblings) — so it
  serves ONLY the ordering/zero-mutation pins above and is NEVER used for
  "kill-after-the-real-mutation" convergence (the mock never mutated;
  there would be nothing to converge FROM). The kill matrix instead uses:
  - **(A) Real-git decorator executor** for git-mutating points: a
    test-only wrapper around a real `MindspecExecutor` in a real
    throwaway-repo fixture that DELEGATES the targeted call (the real
    commit/merge lands), then returns a terminal injected error — the
    in-process equivalent of dying immediately after the mutation
    persisted. Legal inside enforcement-package tests:
    `TestEnforcementHasNoGitLeaks` scans non-test files only
    (`boundary_test.go` skips `_test.go`).
  - **(B) Tracker `*Fn` seams whose errors genuinely TERMINATE the verb**:
    the wrapper mutates an in-memory tracker fake (so idempotence is
    observable on re-invocation), then returns a terminal error. Only
    seams VERIFIED to abort the run qualify — `persistRefutationPending`'s
    fail-closed return (`panel_advisory.go:246`), `doltCommitFn`'s guard
    failure (`complete.go:651-658`), the phase-reconcile
    `guard.NewFailure` (`impl.go:391-397`),
    `createImplementationBeads`' error propagation (`plan.go:124-130`),
    the obligation-settlement write's pre-close failure
    (`panel_advisory.go:740-745`), the two supersede-ADR placeholder
    creates' terminating returns (`complete.go:546-548`,
    `impl.go:354-356` — seams `adrCreateWithIDFn` `complete.go:61-66` /
    `implCreateWithIDFn` `impl.go:41-44`), and `handleExistingBeads`'
    supersede-close propagation (`plan.go:504-506`, via
    `planRunBDCombinedFn`).
  - **(C) A stage-labeled test-only hook inside `internal/executor`**
    (`finalizeStepHookFn func(stage string) error`, nil default = no-op —
    the same `*Fn` convention as `complete.go`'s seams) invoked at the
    intra-`FinalizeEpic` boundaries no existing seam separates, exercised
    in real-git executor fixtures (`finalize_fault_test.go`).
  Mutation points whose errors are SWALLOWED (warn-and-continue) CANNOT be
  kill points — injecting an error there does not terminate the run — and
  are instead DOCUMENTED as forward-safe in Bead 6's matrix, each with the
  code cite proving the swallow. Where no terminating seam exists at all
  (partial dep wiring), the test simulates process death by CONSTRUCTING
  the mid-mutation state and re-invoking. Every kill/death test then
  re-invokes the verb for real and asserts convergence to the completed
  end state OR a clean named refusal (the auto-commit-advanced-tip case
  asserting the panel-staleness refusal as the accepted outcome). The full
  per-verb kill-vs-forward-safe matrix is enumerated in Bead 6's Steps.
- **Anti-drift pins (AC-12).** Doctor's new checks call the exported
  predicates directly; a compile-level pin test asserts the doctor check
  functions are wired to `internal/lifecycle`/executor-exported predicate
  symbols (function-value identity through a seam variable), so a private
  reimplementation fails the test.
- **RED-on-revert discipline.** Every AC test reproduces the original trigger
  and fails on today's `main` with zero product changes; the bead panels
  verify this by reverting the fix hunk locally where cheap.
- **Hermetic full-suite proof (AC-24).** After Bead 5, `go test ./...`
  (harness excluded per convention) is run from INSIDE this active spec
  worktree as a validation proof, per the spec's Validation Proofs block.

## Bead 1: Complete preflight + gated forward reconcile

Restructures `internal/complete` into the explicit preflight → commit →
reconcile shape (R1–R4): lineage-authoritative spec resolution, hint-mismatch
refusal, spec-branch pathspec-scoped tracker commits, and the merged-unclosed /
branch-less forward reconcile that skips only git plumbing — never gate
evaluation — with ADR-0037's ladder applied unchanged. Exports the
landed-merge predicate from `internal/lifecycle` (gitutil allowed there) for
Bead 2's doctor reuse.

**Steps**

1. Restructure `internal/complete/complete.go` around an explicit preflight
   phase: resolve the bead FIRST, derive the owning spec from the bead's
   parent epic as the authoritative source (any in-repo cwd — `main`
   checkout, own worktree, or a different spec's worktree — yields the same
   resolution; outside-repo cwd still fails at `findRoot()` in
   `cmd/mindspec/complete.go:41-44`). Demote cwd-derived resolution
   (`resolveTargetFn`, `complete.go:251-270`; `internal/resolve/target.go:90-94`)
   to non-authoritative: an explicit `--spec` hint that resolves to the
   lineage spec proceeds; a mismatching hint refuses in preflight naming BOTH
   spec IDs. The idempotent ADR-0034 migration (`complete.go:286-293`) stays
   ahead of preflight, exempt.
2. Move every refusal condition derivable from immutable gate facts (epic
   membership, branch existence/ancestry, plan parse, obligation state) ahead
   of the first mutation, so a preflight refusal leaves tracker and git state
   byte-identical modulo the migration (R1). Pin with a `MockExecutor` test
   asserting zero mutating calls before the refusal.
3. Rework the artifact-dirt self-heal (`complete.go:440-475`): stage with an
   explicit lifecycle-artifact pathspec (`.beads/issues.jsonl` and siblings),
   never an `add -A` equivalent; surface unexpected dirty paths as a warning.
   Target the commit at the spec branch; when the checkout is on `main`,
   route the tracker commit to the spec branch or refuse with a named
   re-invocation command — never commit on `main`. The shipped
   `chore/finalize-*` recovery carrier (`finalizeOrphanedSpecBranch`) is
   explicitly excluded (R3).
4. Add `internal/lifecycle/landed.go`: the exported landed-merge predicate
   implementing the plan-choice algorithm above. Concretely:
   `FindLandedMerge(root, specBranch, beadID)` returns the identified merge
   commit (SHA + second parent) or a typed not-found. Mechanism: a new
   `gitutil.FirstParentMerges(workdir, ref)` helper (appended to
   `internal/gitutil/gitops.go` — DISJOINT from Bead 3's `RemoteHeadSHA`
   edit, see the preamble adjacency flag) lists the spec branch's
   first-parent merge commits newest-first with parents + subject
   (`git rev-list --first-parent --merges` + `%P`/`%s` format);
   `FindLandedMerge` accepts the first commit whose subject is exactly
   `Merge bead/<id>` (the deterministic `gitutil.MergeInto` message,
   `gitops.go:156-157` — the only bead→spec merge producer:
   `mindspec_executor.go:302`, `:410`) AND whose available corroborations
   agree: a registered panel's `reviewed_head_sha` (via `panel.Scan` /
   `panel.ForBead` over the layout-aware roots, `internal/panel/panel.go:57`)
   equal to or an ancestor of the second parent; a surviving `bead/<id>` tip
   equal to or an ancestor of the second parent. Contradiction or no match →
   not positively identified. Also export the derived merged-unclosed
   state check (`FindLandedMerge` succeeded AND — when the branch survives —
   `IsAncestor(beadTip, specBranch)` holds). Both exported for doctor reuse
   (AC-12); consumed in `complete`'s preflight. A fresh zero-own-commit
   branch yields not-found BY CONSTRUCTION (git creates no merge commit for
   an already-ancestor branch — "Already up to date", even under `--no-ff`),
   which is the AC-10 negative fixture's load-bearing property.
5. Implement the reconcile path (R4): when preflight detects merged-unclosed
   (branch possibly deleted), skip ONLY the `MergeBase` plumbing
   (`complete.go:492-495`) and the merge/branch-cleanup legs; evaluate
   doc-sync and ADR-divergence against the landed merge commit's content
   (the `M^1..M` diff of the Step-4-identified commit); evaluate the panel
   gate through the UNCHANGED ADR-0037 ladder (decisions 2 and 4 still
   Block; decision 5 MissingRef → Allow + Warn surfaced; §6 panel-free → no
   gate); write durable evidence naming the landed merge commit SHA; close;
   reach `reconcilePendingRefutations` (`complete.go:570`). Second
   invocation is a no-op success. No positively identified landed merge →
   refusal naming the missing evidence (AC-8).
6. Route the same skip-legs path for the already-closed, branch-less bead
   with unsettled `refutation_pending_entries` (h4n5): settle the obligation
   without branch restoration (AC-9). Make AC-9's "subsequent impl approve
   gate passes" clause explicit: after settlement the test asserts
   `complete.CheckPendingObligations` — the EXACT predicate `impl approve`'s
   Leg-3 gate consumes (`internal/approve/impl.go:88`, `:627`) — returns nil
   for the bead, and a second `complete <id>` run is a no-op success.
7. Add the AC-1..AC-9 tests: `internal/complete` MockExecutor tests for
   AC-1/2/3/4 and the reconcile matrix AC-5/6/7/8/9 (registered decision-5
   panel leg, planted doc-sync/ADR failure leg, panel-free leg, no-evidence
   negative), plus `internal/lifecycle` real-git tests for the landed-merge
   predicate — including the fresh-branch not-found case and the
   corroboration-contradiction case.

**Verification**

- [ ] `go test ./internal/complete/... ./internal/lifecycle/... ./internal/gitutil/... ./internal/resolve/... ./cmd/mindspec/...` passes
- [ ] AC-1/AC-2 subtests: wrong-worktree + `main`-checkout invocations resolve via lineage; mismatching `--spec` refuses naming both IDs, state byte-identical (MockExecutor records no mutation)
- [ ] AC-3/AC-4 subtests: no commit on `main` recorded; planted unrelated dirty file excluded from the auto-commit and named in a warning
- [ ] AC-5 subtests: reconcile closes with the landed merge commit SHA in durable evidence; NO `MergeBase` call recorded; doc-sync/ADR gates asserted to have received the `M^1..M` range; second run no-op success
- [ ] AC-6 explicit outcomes: leg (a) close PERFORMED, the decision-5 rerun-after-merge Warn text asserted present in output, NO Block; leg (b) run exits non-zero naming the planted failing gate, bead status re-read as NOT closed, no evidence metadata written
- [ ] AC-7 subtest: panel-free bead reconciles and closes with no panel output at all
- [ ] AC-8 subtest: no-evidence refusal names the missing landed-merge evidence; bead NOT closed
- [ ] AC-9 explicit outcomes: obligation settled to `panel_refuted_entries` without branch restoration; `complete.CheckPendingObligations` returns nil afterwards (the impl-approve Leg-3 gate predicate); idempotent re-run
- [ ] Landed-merge predicate: fresh zero-commit branch → not-found; `--no-ff` merged-then-deleted branch → identified; corroboration contradiction → not-found
- [ ] Each new test fails when its fix hunk is reverted (RED-on-revert spot check recorded in review evidence)

**Acceptance Criteria**

- [ ] AC-1 — lineage-authoritative spec resolution from a different spec's worktree and from a `main` checkout
- [ ] AC-2 — `--spec` hint mismatch refuses in preflight naming both spec IDs; matching hint proceeds; state byte-identical modulo migration
- [ ] AC-3 — tracker auto-commit lands on the spec branch or refuses with a named command; never a commit on `main`
- [ ] AC-4 — pathspec-scoped staging; planted unrelated dirty file excluded and named in a warning
- [ ] AC-5 — merged-unclosed reconcile: no exit-128 merge-base path, gates evaluated against the landed merge commit, durable evidence written, close + `reconcilePendingRefutations` reached, idempotent second run
- [ ] AC-6 — decision-5 fidelity (Allow + Warn surfaced, no fabricated Block; decisions 2/4 still Block) + doc-sync/ADR refusal leg naming the failing gate
- [ ] AC-7 — §6 fail-open parity: panel-free bead reconciles with no panel warning
- [ ] AC-8 — no positively identified landed merge → refusal naming the missing evidence; bead not closed
- [ ] AC-9 — closed, branch-less bead with unsettled `refutation_pending_entries` settles without branch restoration; subsequent impl-approve obligation gate (same predicate) passes

**Depends on**: None (foundational; independent of all other beads).

## Bead 2: Doctor divergence checks, instruct guidance, CI wiring

The doctor half of R5 and R7: the stale-OPEN cross-check (inverse of the
shipped `checkOrphanedBeads`), finalize-orphan surfacing, generated instruct
guidance, and the CI doctor gate. Consumes Bead 1's exported landed-merge
predicate and Bead 3's exported finalize-orphan predicates so check and gate
cannot drift (AC-12). SHARED-PREDICATE VISIBILITY (P8): everything both
doctor AND instruct must invoke lives EXPORTED in `internal/lifecycle` —
a package-private `internal/doctor` predicate is invisible to
`internal/instruct` (Go visibility), so doctor holds only thin
render/registration wrappers, mirroring the shipped
`lifecycle.FindOrphanedClosedBeads` pattern
(`internal/doctor/orphaned_beads.go:16`).

**Steps**

1. Add the EXPORTED stale-OPEN predicate `lifecycle.FindStaleOpenBeads` in
   `internal/lifecycle/stale_open.go` (P8: the SAME package as Bead 1's
   landed-merge predicate, so `internal/doctor` and `internal/instruct`
   can BOTH import it — a doctor-package-private predicate cannot be
   called from instruct). It returns finding DATA (bead ID, spec branch,
   landed merge SHA, message text, recovery command
   `mindspec complete <id>`) for any bead OPEN/in_progress in the
   tracker for which Bead 1's exported landed-merge predicate POSITIVELY
   IDENTIFIES a landed merge commit on the spec branch's first-parent
   chain AND — when the `bead/<id>` ref still exists —
   `IsAncestor(beadTip, specBranch)` holds (no unlanded commits on top).
   NEVER a private reimplementation, and NO fork-point computation of any
   kind: the fresh-claim negative (R5) holds by construction because a
   zero-own-commit branch can never have produced a `--no-ff` merge commit
   (see the AC-10 plan-choice bullet). Then add
   `internal/doctor/stale_open.go`: `checkStaleOpenBeads`, a THIN wrapper
   registered in `RunWithOptions` alongside `checkOrphanedBeads`
   (`internal/doctor/doctor.go:75,95`) that calls the exported predicate
   through a seam variable (mirroring `orphaned_beads.go:16`'s
   `findOrphanedClosedBeadsFn = lifecycle.FindOrphanedClosedBeads`) and
   renders each finding as `Status=Error`.
2. Add `internal/doctor/finalize_orphans.go`: a thin wrapper (same
   seam-variable pattern) reporting (a) an outstanding unmerged
   `chore/finalize-*` branch and (b) stale committed tracker state
   on `main`, by calling the finalize-orphan predicates Bead 3 exports
   from `internal/lifecycle` (`finalize_orphans.go`); the reported stats
   come from the same `origin/main`-anchored computation Bead 3 fixes
   (see the orphan-stats plan-choice bullet).
3. Wire the finalize-orphan and stale-OPEN findings into the generated
   `instruct` surface (R7): `instruct`'s context build
   (`internal/instruct/instruct.go:32-59` — `Context` feeding `Render` and
   the `Guidance` payload) imports `internal/lifecycle` and invokes the
   SAME EXPORTED predicates the doctor wrappers call (P8 — it cannot call
   doctor's package-private wrappers), then passes each finding's
   message + recovery text into a new Context field that
   `templates/idle.md` renders verbatim (template pass-through: the
   predicate is the single source of both the decision and the text; no
   second copy of the logic and no re-derivation). AC-15's instruct half
   asserts the identical guidance string appears in both `mindspec doctor`
   and `mindspec instruct` output from one planted fixture.
4. Add the AC-12 anti-drift pin: a test that asserts (via seam-variable
   function identity) BOTH the doctor wrappers AND instruct's context
   build invoke the exported `internal/lifecycle` predicate symbols;
   rewiring either consumer to a private copy fails it.
5. Wire `mindspec doctor` into `.github/workflows/ci.yml` so a non-zero exit
   fails the build, and pin it with a test asserting the workflow file
   contains the doctor invocation (AC-11's permitted form). Claim
   `.github/workflows/ci.yml` into `.mindspec/domains/workflow/OWNERSHIP.yaml`
   (the `.golangci.yml` precedent from spec 108) so the divergence gate
   attributes the edit.
6. Fixture tests: positive stale-OPEN (merged `--no-ff`, still OPEN — with
   the branch surviving AND deleted variants), negative fresh-claim (zero
   own commits, spec branch advanced past the fork with other `--no-ff`
   merges present — the round-1 false-flag scenario, now provably healthy),
   healthy-agreement, planted finalize branch, planted stale tracker commit
   on `main`.

**Verification**

- [ ] `go test ./internal/doctor/... ./internal/instruct/...` passes
- [ ] AC-10 explicit outcomes: positive fixture (both branch-surviving and branch-deleted variants) flags `Status=Error` naming `mindspec complete <id>`; fresh-bead negative WITH prior `--no-ff` merges on the spec branch reports healthy; agreement fixture reports healthy
- [ ] AC-15 explicit outcomes: both finalize-orphan findings reported by doctor; the SAME guidance string (message + recovery command) asserted present in `mindspec instruct` output from the same fixture; orphan stats asserted against `origin/main`
- [ ] AC-11 pin: workflow-file test finds the `mindspec doctor` step; AC-12 pin fails when a check is rewired to a private predicate copy
- [ ] `rg -n 'mindspec doctor' .github/workflows/ci.yml` non-empty

**Acceptance Criteria**

- [ ] AC-10 — stale-OPEN cross-check flags landed work, never fresh zero-commit branches (even with prior `--no-ff` merges present); healthy on agreement
- [ ] AC-11 — CI runs `mindspec doctor` with non-zero exit failing the build (pinned by workflow-content test)
- [ ] AC-12 — doctor divergence checks invoke the same exported `internal/lifecycle` predicates the verbs gate on (anti-drift pin test covering doctor AND instruct wiring)
- [ ] AC-15 — finalize-orphan branch + stale-tracker-on-`main` findings surfaced by doctor AND generated instruct (same exported `internal/lifecycle` predicate, same text); orphan `CommitCount`/`DiffStat` asserted vs `origin/main`

**Depends on**: Bead 1 (consumes the exported landed-merge predicate) and Bead 3 (consumes the exported finalize-orphan predicates and `origin/main` stats — both exported from `internal/lifecycle`, P8).

## Bead 3: Epic-scoped finalization, impl-approve preflight, gitutil hardening

The executor half of R6–R8 plus `impl approve`'s R1 leg: both `FinalizeEpic`
enumerations scoped to plan-declared lifecycle beads and fail-closed — via an
allow-set computed in `ApproveImpl` (the ENFORCEMENT layer, using a NEW
exported `internal/phase` classifier — a core-domain touch, see the
plan-choice bullet) and PASSED into `FinalizeEpic` as a parameter, because
the executor may not import `internal/phase` (P6: `executor.go:6-11`,
`boundary_test.go:116`) — plus the impl-approve facts-before-mutation
restructure, orphan stats vs `origin/main`, exact-refname `RemoteHeadSHA`,
the `PushBranchForceWithLease` doc-comment correction, and the narrowed
`impl approve` cleanup preserving the `CommitCount` retention.

**Steps**

1. Export a lifecycle-classification helper from `internal/phase` (P3):
   `classifyChild` is UNEXPORTED (`internal/phase/derive.go:272`), so
   `FinalizeEpic` owns no classification surface today. Add
   `phase.LifecycleChildIDsForEpic(epicID string) ([]string, error)` — a
   FAIL-CLOSED sibling of the exported advisory
   `OpenNonLifecycleChildren`/`OpenNonLifecycleChildrenForEpic`
   (`derive.go:346`, `:364`; those swallow query errors because they feed a
   never-blocking hint — this one must ERROR on a bd query failure because
   it feeds a destructive enumeration). It resolves the epic's children via
   the shared `phase.Cache` (one `bd list --parent` call, as `:365`) and
   returns the IDs classified `childLifecycle` by the SAME unexported
   `classifyChild` (`task`/empty → lifecycle; `bug`/other explicit types →
   non-lifecycle). bd-only, no git I/O (`TestEnforcementHasNoGitLeaks`
   unaffected). Its ONLY caller is `ApproveImpl` — `internal/approve`
   ALREADY imports `internal/phase` (`impl.go:20`), so no new import edge
   and no cycle; the EXECUTOR never calls it (P6: `internal/executor` must
   not import enforcement packages, and `internal/phase` is one —
   `executor.go:6-11`, `boundary_test.go:116`). This is the plan's
   unconditional CORE-domain touch — declared in this bead's footprint and
   reconciled in the plan-choice bullet.
2. Compute the finalize allow-set in `ApproveImpl`'s PREFLIGHT (the
   enforcement layer — P6: the executor cannot compute it without an
   illegal `internal/phase` import), defined precisely as the
   intersection: `X ∈ planDeclared(finalizing spec) ∩
   lifecycleChildren(finalizing epic)`, where `planDeclared` is
   `readPlanBeadIDs` over the finalizing spec's plan.md (already
   approve-side, `internal/approve/impl.go:528`) and `lifecycleChildren`
   is Step 1's `phase.LifecycleChildIDsForEpic(finalizing epic)` — so
   `parent == finalizing epic` is a NECESSARY cross-check (the child query
   is parent-scoped) but never the sole selector (the lifecycle-type ∩
   plan-declared legs exclude same-epic non-lifecycle children).
   Resolution happens with the other preflight FACTS, before any
   mutation: a plan `bead_ids` read failure or a
   `LifecycleChildIDsForEpic` error refuses PRE-mutation with a named
   error (fail-closed, and strictly stronger than executor-side
   detection — nothing has mutated yet). PASS the set into the executor:
   the `FinalizeEpic` signature becomes
   `FinalizeEpic(epicID, specID, specBranch string, lifecycleAllowSet
   []string) (FinalizeResult, error)`, updated in lockstep at the
   interface (`internal/executor/executor.go:42`), `MindspecExecutor`
   (`mindspec_executor.go:349`), `MockExecutor` (`mock.go:132`), and the
   single production call site (`impl.go:448`) — PLUS all existing test
   call sites (P11 — footprint honesty): the signature change is
   compilation-forced through the 16 direct 3-arg `FinalizeEpic(...)`
   test calls across 5 `internal/executor` test files, all declared in
   this bead's footprint — `executor_test.go` (:495, :513, :714),
   `finalize_orphan_test.go` (:125, :212, :226, :273, :317, :360),
   `finalize_worktree_only_test.go` (:50, :114), `layout_guard_test.go`
   (:479), `merge_conflict_test.go` (:188, :270, :360, :404) — ~20
   call-site edits in total (4 production/interface/mock + 16 test),
   each passing an explicit allow-set (or the fixture-appropriate set)
   so no test silently changes semantics. Inside `FinalizeEpic` a
   candidate branch `bead/<X>` is admitted iff `X ∈ lifecycleAllowSet`,
   for BOTH legs: the auto-merge enumeration (the `WorktreeOps.List()`
   loop, `mindspec_executor.go:382-427`) and the worktree/branch cleanup
   leg (`:526-534`). The executor performs NO plan-file read, NO bd
   classification, and NO `internal/phase` import — it only filters
   candidates against the passed set. Exclusion boundary is lifecycle
   identity — same-epic non-lifecycle children survive whether open or
   closed (R6). The executor-side legs stay FAIL-CLOSED: a
   `WorktreeOps.List()` error (today swallowed by `if listErr == nil`)
   aborts the finalize with a named error, and `bead/*` candidates
   present alongside a nil allow-set abort rather than being silently
   skipped or admitted — no silent leg-skip, no silent admission (AC-14).
3. Restructure `impl approve` to facts-before-mutation (P2 / R1): hoist the
   Spec 115 orphan/obligation gate (`internal/approve/impl.go:374`,
   `runOrphanObligationGate`) AHEAD of the `--supersede-adr` placeholder-ADR
   file write (`impl.go:354`, `implCreateWithIDFn`) — today that file write
   precedes a derivable refusal, violating R1. The reordered sequence:
   gates (1/3)–(3/3) fact evaluation (`impl.go:270-339`) → the Step-2
   allow-set resolution (fail-closed preflight fact, P6) → orphan/obligation
   gate → supersede placeholder pre-create → ADR-divergence refusal decision
   (`impl.go:360` — Spec 087's pre-create-before-the-gate-skip-decision rule
   is PRESERVED: the placeholder still exists before that decision, and when
   `--supersede-adr` is set the decision is skipped, so no refusal follows
   the write) → phase-reconcile write (`:391`, first tracker mutation) →
   epic close (`:407`) → phase=done (`:415`) → `CommitCount` (`:440`, its
   consensus-revision-9 position UNTOUCHED) → `FinalizeEpic` (`:448`). The
   idempotent ADR-0034 migration (`impl.go:186-192`, `phase.EnsureMigrated`
   at `:190`) stays ahead of preflight, exempt per R1. Pin the whole order
   with a seam/MockExecutor call-order test (extending
   `TestApproveImplCallOrder`), and pin that a preflight refusal (e.g. a
   planted orphan) performs ZERO mutating calls — no `implCreateWithIDFn`,
   no `implPhaseMetadataFn`, no `implRunBDCombinedFn`, no executor mutation.
4. Export the finalize-orphan predicates (unmerged `chore/finalize-*` branch
   detection; stale committed tracker state on `main`) from
   `internal/lifecycle` (`finalize_orphans.go` — P8: NOT executor-private
   and NOT doctor-private, so `internal/doctor` AND `internal/instruct`
   can both import them; `internal/lifecycle` is outside the
   enforcement-package pin list and uses `gitutil` directly per the
   shipped `orphans.go` precedent), returning finding data for Bead 2's
   consumers; and compute orphan-case
   `CommitCount`/`DiffStat` against `origin/main` instead of possibly-stale
   local `main` (`mindspec_executor.go:433-437`; scope per the orphan-stats
   plan-choice bullet).
5. Harden `gitutil.RemoteHeadSHA` (`internal/gitutil/gitops.go:327-346`):
   match the exact refname on the `ls-remote` refname column so
   `refs/heads/aaa/chore/finalize-105` never satisfies a lookup for
   `chore/finalize-105`; correct the `PushBranchForceWithLease` doc comment
   (the lease protects only the read-to-push window; drop "never silently
   clobbered"). (Disjoint-function adjacency with Bead 1's appended
   `FirstParentMerges` — see the preamble flag.)
6. Remove the unreachable refusal disjunction in `impl approve`
   (`internal/approve/impl.go:440-445`) while PRESERVING the
   `exec.CommitCount` call at its consensus-revision-9 position between the
   phase-metadata write and `FinalizeEpic`; keep `TestApproveImplCallOrder`
   pinning the surviving order including the call, and keep a test pinning
   that a missing/empty plan is refused pre-mutation by the Leg 3
   orphan-obligation gate (R8).
7. Tests: the AC-13 four-plant fixture (foreign-epic in_progress worktree,
   other-spec closed orphan branch, same-epic OPEN `bug` with real worktree,
   same-epic CLOSED non-lifecycle branch) alongside
   `TestFinalizeEpic_MergesOnlyWorktreeRealBranches`, asserting all four
   survive BOTH legs while lifecycle children finalize (the fixture passes
   the allow-set the approve-side intersection produces); the AC-14 legs —
   an injected `List()` failure aborting inside `FinalizeEpic`, plus
   classifier-error and plan-read-error refusals firing in `ApproveImpl`
   preflight BEFORE any mutation (zero mutating calls recorded);
   the AC-16 decoy-ref fixture; the AC-17 pins; the Step-3 preflight
   ordering + zero-mutation-on-refusal pins; `internal/phase` unit tests for
   `LifecycleChildIDsForEpic` (lifecycle/non-lifecycle/epic children,
   query-error → error not nil).

**Verification**

- [ ] `go test ./internal/executor/... ./internal/gitutil/... ./internal/phase/... ./internal/approve/...` passes
- [ ] AC-13 explicit outcomes: only beads in `planDeclared ∩ lifecycleChildren` merged/worktree-removed/branch-deleted; all four planted non-lifecycle candidates present (branch AND worktree) after BOTH legs; finalize exits success
- [ ] AC-14 subtests: injected `LifecycleChildIDsForEpic` error and unreadable plan `bead_ids` each refuse in `ApproveImpl` preflight BEFORE any mutation; injected `WorktreeOps.List` error aborts inside `FinalizeEpic` before any merge/removal — all with named errors
- [ ] Impl-approve preflight pins: call-order test proves orphan gate precedes `implCreateWithIDFn`; planted-orphan refusal records zero mutating calls; `TestApproveImplCallOrder` still pins `CommitCount` between phase-metadata write and `FinalizeEpic`
- [ ] AC-16 subtests: decoy ref never resolves; doc comment no longer overstates the lease guarantee
- [ ] AC-17: refusal disjunction gone, `TestApproveImplCallOrder` still green, Leg-3 pre-mutation refusal still pinned

**Acceptance Criteria**

- [ ] AC-13 — both `FinalizeEpic` enumerations scoped to the plan-declared ∩ lifecycle-classified child set (computed in `ApproveImpl`, passed as the `lifecycleAllowSet` parameter — P6); foreign and same-epic non-lifecycle candidates (open AND closed) survive both legs
- [ ] AC-14 — enumeration/classification/plan-read failure aborts fail-closed with a named error (classification/plan-read refuse in preflight pre-mutation; enumeration aborts inside `FinalizeEpic` before any merge/removal)
- [ ] AC-15 (executor half) — orphan-case `CommitCount`/`DiffStat` computed against `origin/main`
- [ ] AC-16 — `RemoteHeadSHA` exact-refname match; `PushBranchForceWithLease` doc comment corrected
- [ ] AC-17 — unreachable refusal removed; `CommitCount` call-order retention pinned; missing/empty plan still refused pre-mutation
- [ ] R1 (impl approve) — facts-before-mutation order pinned; preflight refusal mutates nothing (exercised end-to-end by Bead 6's AC-26 matrix)

**Depends on**: None (independent of Beads 1, 4; the shared `gitops.go` file with Bead 1 is disjoint-function, no edge). The `FinalizeEpic` signature change is self-contained within this bead's footprint — interface (`executor.go:42`), `MockExecutor` (`mock.go:132`), `MindspecExecutor` (`mindspec_executor.go:349`), and the single production call site (`impl.go:448`) change in one commit TOGETHER WITH the 16 direct test call sites in the five `internal/executor` test files enumerated in Step 2 (P11 — the compiler forces every caller into the same commit; ~20 call-site edits total), so no other bead compiles against a stale signature. Domain footprint: workflow (`internal/approve`, `internal/lifecycle`), execution (`internal/executor`), core (`internal/phase` classifier export) — unchanged in kind by P6/P8; the phase consumption simply moved from an illegal executor import to the existing approve-side import.

## Bead 4: Validator-complete scaffold + plan-approve preflight + loud dependency wiring

R9–R10 plus `plan approve`'s R1 leg: the plan scaffold finally feeds the
shipped `work_chunks` wiring, plan-content refusals move ahead of the first
mutation, the two silent fail-opens at plan approve become visible warnings,
and both scaffolds round-trip through their own validators.

**Steps**

1. Restructure `plan approve` to facts-before-mutation (P2 / R1): today
   `ValidateWorkChunkAlignment` runs INSIDE `createImplementationBeads`
   (`internal/approve/plan.go:274`) — AFTER the frontmatter Approved write
   (`updatePlanApproval`, `plan.go:118`) and AFTER `handleExistingBeads` may
   have supersede-closed the previous bead set (`plan.go:240`, close at
   `:504`) — so a misaligned plan refuses having already mutated both the
   plan file and the tracker. Hoist the pure plan-content fact gathering —
   plan read, `validate.ParseBeadSections`, `validate.ParsePlanFrontmatter`,
   `ValidateWorkChunkAlignment`, and the epic lookup (today
   `phase.FindEpicBySpecID`, `:111` — replaced by the mode-distinguishing
   `Cache.AllEpics` resolution below) — into an explicit preflight block
   BEFORE Step 3's frontmatter write, so every refusal derivable from plan
   content fires with tracker and plan file byte-identical (modulo the
   exempt ADR-0034 migration, `plan.go:70-76`). The epic/child-set leg is
   FAIL-CLOSED (P9 — R1 completeness): today `FindEpicBySpecID`'s error is
   silently swallowed (`plan.go:110-113` — `parentID` stays empty and bead
   creation is silently skipped AFTER the Approved write) and
   `handleExistingBeads` fail-opens on its child query (`plan.go:460-462`
   — a `bd list --parent` error → `return nil`, "proceed with creation").
   The preflight instead resolves the target epic AND its child set (the
   same parent-scoped `bd list --parent` query `handleExistingBeads`
   consumes) BEFORE any mutation, and BOTH epic failure modes refuse
   FAIL-CLOSED pre-mutation (P10 — the epic is ALWAYS created at spec
   approve, so an absent epic at plan approve is an anomaly, not a
   supported path; the round-3 warn-and-continue "legacy no-bead path"
   is REMOVED). Because `phase.FindEpicBySpecID` CONFLATES the two
   modes (`Cache.FindEpicBySpecID` returns an error both when the
   `AllEpics` query fails AND when no epic matches —
   `internal/phase/cache.go:143-172`), the preflight distinguishes them
   via the exported `phase.Cache.AllEpics` (`cache.go:59-75`: an error
   ⇔ the bd query failed; a nil error ⇔ the authoritative epic list),
   matching the spec with the same `ExtractSpecMetadata` /
   `SpecIDFromMetadata` logic the cache itself uses
   (`cache.go:158-160`): (a) a bd QUERY error refuses naming the
   failed query, recovery = re-run `mindspec plan approve <specID>`
   once bd is reachable; (b) a GENUINELY ABSENT epic (query succeeded,
   no match) refuses naming the anomaly — the only code path that
   produces an epic-less approved spec is spec-approve's WARN-DEGRADED
   epic create (`internal/approve/spec.go:85-87` appends a Warning
   instead of failing), a failure state, not a feature — with recovery
   `mindspec spec approve <specID>`, whose idempotent re-run recreates
   the missing epic without disturbing an existing one
   (`spec.go:69-77`). No genuinely-supported no-epic plan-approve
   scenario exists in the code, so no documented exception is carried.
   The resolved child-set facts also let `handleExistingBeads`'
   supersede-safety refusals (in_progress child `plan.go:481`, closed
   child `:486`) fire in preflight, before the frontmatter write; the
   supersede-CLOSE of all-open children (`:504`) remains the first
   sanctioned mutation, after all content- and child-set-derived refusals.
   Pin with call-order tests: a misaligned `work_chunks` plan, an injected
   child-set query failure, an injected `AllEpics` query failure, a
   genuinely-absent-epic fixture (query succeeds, empty/no-match epic
   list), and a planted in_progress child each refuse with the plan file
   unmodified and zero bd mutations recorded — the two epic refusals
   asserted to carry their DISTINCT messages and recovery commands
   (failed-query vs `mindspec spec approve <specID>`).
2. Extend `scaffoldPlan` (`internal/approve/spec.go:230-268`) to emit a
   `work_chunks` frontmatter block with per-chunk `id`, `depends_on`, and
   `key_file_paths` matching `internal/validate/plan.go`'s `WorkChunk` shape,
   so a filled-as-given scaffold gets edges wired by the shipped
   `work_chunks[].depends_on` path (`internal/approve/plan.go:354-373`). No
   prose dependency parsing anywhere (R9 / 097 R3 preserved).
3. Keep the per-bead prose `**Depends on**` section (the validator's
   `bead-depends` check expects it) but label it in the scaffold as
   human-readable documentation that is never parsed; add the per-bead
   `**Acceptance Criteria**` section the validator hard-requires
   (`internal/validate/plan.go:838`).
4. Make `plan approve` warn loudly on the two fail-opens: (a) a plan with NO
   `work_chunks` frontmatter → warning naming the absence, approve otherwise
   unchanged for legacy plans; (b) a failed `bd dep add`
   (`internal/approve/plan.go:368-370` — today a silent `continue`) →
   warning naming BOTH bead IDs of the unwired edge, approve still
   best-effort.
5. Add `internal/approve/scaffold_roundtrip_test.go`: the emitted plan
   scaffold, minimally filled, passes every `internal/validate/plan.go` check;
   the emitted spec scaffold, minimally filled, passes the spec validator;
   and the filled plan scaffold's `work_chunks` (chunk 2 `depends_on: [1]`)
   produce a wired bd edge — ready set contains Bead 1 not Bead 2, closing
   Bead 1 readies Bead 2 (AC-18, AC-21).
6. Add the warning-path tests: prose-only plan wires no edges and emits the
   missing-`work_chunks` warning with no prose text parsed (AC-19); injected
   `bd dep add` failure (via `SetPlanRunBDForTest`, `plan.go:44`) emits the
   both-IDs warning while remaining edges wire and approve succeeds (AC-20).

**Verification**

- [ ] `go test ./internal/approve/... ./internal/validate/...` passes
- [ ] Preflight pin: misaligned-`work_chunks` plan, injected epic child-set query failure, injected `AllEpics` query failure, genuinely-absent epic, and planted in_progress child each refuse with plan.md byte-identical and zero bd mutations recorded (no supersede-close, no create); the absent-epic refusal names `mindspec spec approve <specID>` as recovery and the query-failure refusal names the failed query — distinct messages, both fail-closed (P10)
- [ ] AC-18 subtest: scaffold → fill → approve → bd edge exists; ready-set ordering asserted before and after closing Bead 1
- [ ] AC-19/AC-20 subtests: missing-`work_chunks` warning; injected dep-add failure warning naming both bead IDs with approve succeeding
- [ ] AC-21 round-trip: emitted plan scaffold shape pinned (`work_chunks` + per-bead `**Acceptance Criteria**` + labeled non-authoritative `**Depends on**`); both scaffolds pass their validators minimally filled

**Acceptance Criteria**

- [ ] AC-18 — scaffold emits `work_chunks` with `depends_on`; filled scaffold round-trips into a wired bd edge with correct ready-set ordering
- [ ] AC-19 — no-`work_chunks` plan approves with zero edges and a warning naming the missing block; no prose parsed
- [ ] AC-20 — failed `bd dep add` warns naming the unwired edge's both bead IDs; approve proceeds; other edges wire
- [ ] AC-21 — scaffold shape pinned; plan and spec scaffolds pass their own validators minimally filled
- [ ] R1 (plan approve) — plan-content refusals AND the fail-closed epic/child-set resolution (query failure refuses; genuinely absent epic refuses with the `mindspec spec approve` recovery, the legacy warn-and-continue no-bead path removed — P9/P10; supersede-safety refusals fire pre-mutation) precede the first mutation; refusal leaves plan file and tracker byte-identical modulo migration (exercised end-to-end by Bead 6's AC-26 matrix)

**Depends on**: None (independent; touches `internal/approve/plan.go` in functions disjoint from Bead 5's lint pass — see the preamble adjacency flag).

## Bead 5: Advisory bead scope + hermetic instruct tests

R11–R12: the warn-only per-bead scope check at `complete`, the
double-assignment plan-lint at `plan approve`, and the hermetic
`setupRunTestProject`. Sequenced after Bead 1 because the WARN hooks into the
gate-evaluation phase of the restructured preflight pipeline (and to avoid two
conflicting restructures of `complete.go`). Shares `internal/approve/plan.go`
with Bead 4 in disjoint functions (the NEW lint pass vs Bead 4's preflight +
warnings — see the preamble adjacency flag).

**Steps**

1. Add `internal/complete/bead_scope.go`: during gate evaluation (post-Bead-1
   pipeline), attribute the bead's changed files to owning domains and emit a
   non-fatal WARN — exit code unchanged — when a file falls outside the
   bead's declared `Domain:` while still within the spec's impacted domains,
   naming the file and both domains. Route all new path/ID-bearing output
   through `internal/termsafe` (R11 / spec 116).
2. Add the double-assignment plan-lint to `plan approve`: flag any single
   file assigned to two or more beads' step lists (reusing the validator's
   parsed `StepLines`/path extraction), naming the file and both beads;
   advisory only (AC-23).
3. Make `setupRunTestProject` (`internal/instruct/run_test.go:16-25`)
   hermetic: replace the fake `.git` marker with a real `git init` in the
   temp sandbox and set `GIT_CEILING_DIRECTORIES` so phase/git resolution
   cannot walk up into the enclosing real worktree. No `internal/phase`
   change for THIS fix (Bead 3's classifier export is the plan's only core
   touch; R12's core-conditional override variant is not taken).
4. Tests: AC-22 WARN fixture (cross-domain file within spec domains,
   termsafe-escaped output asserted, exit code compared to the no-warn run);
   AC-23 lint fixture; AC-24 — `TestRun_IdleNoBeads` executed from a sandbox
   nested inside a git worktree with live lifecycle state, asserting
   resolution cannot escape the test root.

**Verification**

- [ ] `go test ./internal/complete/... ./internal/approve/... ./internal/instruct/...` passes
- [ ] AC-22 subtest: WARN names file + both domains, rendered through termsafe, exit unchanged vs no-warn case
- [ ] AC-23 subtest: plan-lint finding names the file and both beads
- [ ] AC-24: `go test ./...` (harness excluded) green when run from inside this active-spec worktree; nested-sandbox assertion proves enclosing state is not read

**Acceptance Criteria**

- [ ] AC-22 — advisory cross-domain WARN at `complete`, termsafe-escaped, exit unchanged
- [ ] AC-23 — double-assignment plan-lint names file and both beads
- [ ] AC-24 — hermetic `setupRunTestProject`; full suite green from inside an active-spec worktree

**Depends on**: Bead 1 (consumes the restructured preflight/gate-evaluation seam in `internal/complete`).

## Bead 6: ADR-0041 + fault-injection regression suite

R13 and the ADR touchpoint: author ADR-0041 recording the gate-before-mutate
contract, cite it from the new preflight code in ALL THREE verbs, and pin the
forward-reconcile protocol with fault injection at each significant
post-preflight mutation point per verb — through mechanisms that can ACTUALLY enact each
kill (P7): real-git decorator executors for git-mutating points, tracker
seams whose errors genuinely terminate the run, and the stage-labeled
executor hook — with swallowed-error mutation points DOCUMENTED as
forward-safe (each with the code cite proving the swallow) instead of
fictitiously kill-tested through seams that cannot terminate anything.
Preserves AC-26's intent: multiple real mutation points per verb, each
interruption converging to completion or a clean recoverable refusal.

**Steps**

1. Author `.mindspec/adr/ADR-0041-gate-before-mutate.md` (Accepted; domains:
   workflow, execution, core): preflight → commit → reconcile contract — all
   immutable gate facts resolved and all derivable refusals evaluated before
   the first mutation, the idempotent ADR-0034 migration exempted;
   tracker-only commits never target protected `main`; recovery is forward
   re-invocation converging to completion or a clean named refusal, never
   rollback (AC-25).
2. Cite ADR-0041 from the new preflight code (doc comments at the preflight
   entry points in `internal/complete/complete.go`, `internal/approve/impl.go`,
   `internal/approve/plan.go`) and verify the ADR-divergence gate passes with
   the declared touchpoint via the NAMED commands (plan-choice bullet):
   Bead 6's own `mindspec complete <bead-6-id>` with zero overrides (the
   per-bead gate at `complete.go:523`), recorded in review evidence, plus a
   unit fixture driving `validate.CheckADRDivergence` over the citing files
   with ADR-0041 present vs absent.
3. `complete` fault matrix (AC-26) — each significant post-preflight
   mutation point is either KILL-TESTED (its seam genuinely TERMINATES the run; mechanism A
   or B per Testing Strategy) or DOCUMENTED-FORWARD-SAFE (its error is
   swallowed by design, with the code cite; injecting an error there
   cannot terminate the run, and an interruption at that point leaves a
   state the same run or an idempotent re-run already absorbs — no
   individual kill test is possible or needed):
   - (c1) KILL — durable-obligation marker write (panelGate step 2.25,
     `persistRefutationPending` — its failure TERMINATES via
     `guard.NewFailure`, `panel_advisory.go:246`): metadata-seam wrapper
     (mechanism B, `completeMergeMetadataFn`/`completeGetMetadataFn`,
     `complete.go:42`) mutates the tracker fake then fails; re-invocation
     re-unions idempotently and converges;
   - (c2) KILL — `--commit-msg` tracker auto-commit (step 2.5,
     `exec.CommitAll`, `complete.go:414-416` — its error RETURNS):
     real-git decorator executor (mechanism A) lands the REAL commit,
     then dies; re-invocation converges to the panel-staleness refusal
     (the accepted outcome for the advanced-tip case), no manual git
     surgery;
   - (c3) KILL — artifact-sync commit (step 3, `exec.CommitAll`,
     `complete.go:469-471` — its error RETURNS): same decorator, second
     `CommitAll`; re-invocation finds a clean tree and converges to done;
   - (c4) KILL — `bd close` + dolt durability (step 4, `closeBeadFn`
     `complete.go:575`, `doltCommitFn` `:651-658` — the dolt-commit
     failure TERMINATES via `guard.NewFailure` naming the re-run
     command): seam wrappers (mechanism B) close the fake-tracker bead
     then fail; re-invocation hits the already-closed tolerance
     (`:576-579`) and converges;
   - (c5) KILL — bead→spec merge (step 5, `exec.CompleteBead`,
     `complete.go:697` — hard failure per spec 092): the decorator
     performs the REAL `--no-ff` merge in the git fixture, then dies;
     re-invocation converges through Bead 1's merged-unclosed reconcile —
     this point doubles as AC-5's end-to-end kill proof;
   - (c6) FORWARD-SAFE (documented, no kill test) — post-terminal
     override/audit metadata and the phase-mode write: every error is
     swallowed as a Warning print (`complete.go:766-768`, `:782-784`,
     `:801-803`) or discarded outright (`_ =` at `:827`), so the run
     CONTINUES and no seam can enact a kill; an interruption there
     leaves the terminal mutation complete with the metadata absent —
     exactly the state the audit-trail design already treats as the
     failure record;
   - (c7) KILL — supersede-ADR placeholder pre-create (P12; between the
     gate evaluation and the gate-failure decision: `adrCreateWithIDFn`,
     `complete.go:546-548` — its error TERMINATES the run via
     `return nil, fmt.Errorf("--supersede-adr: %w", err)`; the seam
     already exists as a package var, `complete.go:61-66`): mechanism-B
     wrapper performs the REAL placeholder write in the fixture, then
     fails; re-invocation WITH the same `--supersede-adr` flag converges
     to the clean NAMED collision refusal — `adr.CreateWithID`'s
     exact-path + slug-glob collision check names the existing file
     (`internal/adr/create.go:190-204`) — the accepted outcome (c2
     precedent); the documented recovery is a flag-LESS re-run: on the
     per-bead lane, Proposed-only ADR coverage is an advisory WARNING,
     not a gate failure (`internal/validate/adr_divergence.go:27-29`),
     so the already-written placeholder covers the domain and the
     flag-less re-run converges to done;
   - (c8) KILL — pending-obligation settlement write (P12; step 3.75,
     `reconcilePendingRefutations` → `writePanelRefutedMetadata`,
     `complete.go:570-572` — a settlement-write failure TERMINATES
     pre-close by design: "an obligation may NEVER merge un-audited",
     `panel_advisory.go:740-745`): mechanism B via the SAME metadata
     seams as c1 (`completeGetMetadataFn` read `panel_advisory.go:464`,
     `completeMergeMetadataFn` write `:480`) — the wrapper lands the
     merged `panel_refuted_entries` union in the tracker fake, then
     fails; re-invocation recomputes `uncoveredPendingObligations`
     (`panel_advisory.go:722`), finds every pending entry already
     covered → settlement no-op (`:726-728`) → proceeds to close and
     converges.
   The spec's minimum three (after tracker auto-commit, after `bd close`,
   after the merge) are c2/c3, c4, and c5.
4. `impl approve` fault matrix (AC-26), same classification:
   - (i0) KILL — supersede-ADR placeholder pre-create (P12;
     `implCreateWithIDFn`, `impl.go:354-356` — its error TERMINATES via
     `return nil, fmt.Errorf(...)`; the seam already exists,
     `impl.go:41-44`; in Bead 3 Step 3's reordered sequence this is the
     FIRST post-preflight mutation — a file-system write after the
     orphan/obligation gate, before the ADR-divergence decision):
     mechanism-B wrapper writes the REAL placeholder, then fails;
     re-invocation with the flag converges to the clean NAMED collision
     refusal (`internal/adr/create.go:190-204`, names the existing
     path); documented recovery: the operator completes the
     already-created placeholder and flips it to Accepted — the
     impl-approve lane hard-requires Accepted (Proposed-only coverage
     is an ERROR on that lane, `internal/validate/adr_divergence.go:30-36`)
     — then re-runs flag-less and converges;
   - (i1) KILL — phase-reconcile write (`impl.go:391-397` — TERMINATES
     via `guard.NewFailure`): `implPhaseMetadataFn` wrapper (mechanism B,
     `impl.go:40`); re-derivation is deterministic, re-invocation
     converges;
   - (i2) FORWARD-SAFE — epic close (`impl.go:407-410`): the error is
     appended to `result.Warnings` and the run CONTINUES (not a kill
     point); an interruption is absorbed by the idempotent re-close on
     re-run (`isAlreadyClosedErr` tolerance);
   - (i3) FORWARD-SAFE — phase=done write (`impl.go:415-420`): same
     warn-and-continue swallow; re-run rewrites it;
   - (i4) KILL — the intra-`FinalizeEpic` mutation chain, via the NEW
     stage-labeled `finalizeStepHookFn` (mechanism C — ONE test-only
     package-var hook, `*Fn` convention, nil default; no existing seam
     separates these points), invoked at FIVE stages and exercised in the
     real-git `internal/executor` fixture (`finalize_fault_test.go`):
     (a) after the auto-merge leg (`mindspec_executor.go:382-427`);
     (b) after the unconditional spec-branch push (`:450-452` — a
     previously omitted point; a real push failure already terminates
     the finalize, so the hook's terminal error faithfully models a
     kill);
     (c) after `finalizeOrphanedSpecBranch` returns (`:500-506`, the
     wu7t path — previously omitted; its error already terminates);
     (d) between the merge/push legs and the cleanup leg (`:526-534`);
     (e) after the cleanup leg's mutations complete (`:525-575` — a
     previously omitted point covering a death AFTER the cleanup
     mutations: best-effort bead worktree/branch removals (`:527-534`,
     `_ =` discards), spec-worktree removal with its already-removed
     tolerance (`:538-542`), the no-remote direct spec→main merge
     (`:547-563`), and spec-branch deletion with the same tolerance
     (`:566-570`)). This is spec 115's shipped protected-main finalize
     flow; a death here strands a closed epic with the finalize's
     side effects already landed, and the re-run converges because
     every cleanup op is idempotent — `isAlreadyRemovedErr` absorbs
     re-removals (`:539`, `:567`) and a re-attempted `MergeBranch` of
     an already-merged spec branch is an ancestor no-op — while a
     death that instead leaves the spec branch not yet finalized is
     forward-reconciled by the shipped orphan-finalize recovery on
     re-invocation of `impl approve` (`finalizeOrphanedSpecBranch`,
     entered at `:500-506` — the wu7t path stage c already exercises).
     Each stage returns its terminal error AFTER the real mutations
     landed; a re-run of `impl approve` is asserted to converge
     (re-attempted merges see ancestors, the push is idempotent, cleanup
     completes);
   - (i5) FORWARD-SAFE — post-finalize override metadata
     (`implMergeMetadataFn`, `impl.go:455-491`): every write failure is a
     `result.Warnings` append; the run continues; absence of metadata is
     the designed audit record.
5. `plan approve` fault matrix (AC-26), same classification:
   - (p0a) KILL — supersede-close of the previous all-open bead set
     (P12; `handleExistingBeads`, close via `planRunBDCombinedFn`,
     `plan.go:493-506` — a close error propagates and TERMINATES the
     approve, `plan.go:504-506`; post-Bead-4 this is the FIRST
     sanctioned mutation, firing BEFORE the frontmatter Approved
     write): mechanism B via `SetPlanRunBDCombinedForTest`
     (`plan.go:50-55`) — the wrapper closes the fake-tracker beads,
     then fails; re-invocation converges to the clean NAMED
     supersede-safety refusal: Bead 4's preflight child-set check now
     sees CLOSED children and refuses with the closed-child message +
     `bd delete <id> --force` recovery line (`plan.go:485-490`), whose
     text explicitly walks the operator through exactly this
     partial-create leftover case (accepted outcome, c2 precedent);
   - (p0b) SIMULATED-DEATH (state-construction) — the frontmatter
     Approved write (P12; `updatePlanApproval`, `plan.go:118-120` —
     its own error TERMINATES, but the death of interest is
     immediately AFTER the write, before the first bead create, and no
     seam separates the write from its return): the test CONSTRUCTS
     the death state (plan.md frontmatter already Approved, zero
     children under the epic) and re-invokes `ApprovePlan`, asserting
     convergence to the fully wired bead set — `updatePlanApproval` is
     idempotent (the p3 cite) and `handleExistingBeads` sees no
     children → full create proceeds. This promotes the
     Approved-write→first-create gap from a side effect of p1's
     re-traversal to an EXPLICITLY classified point;
   - (p1) KILL — after the Nth bead creation (`planRunBDFn` `create`,
     `plan.go:338`; a create error propagates out of
     `createImplementationBeads` and TERMINATES the approve,
     `plan.go:124-130`): swap via `SetPlanRunBDForTest` (`plan.go:44`)
     with a call-counting wrapper (mechanism B: create in the fake
     tracker, then fail on call N); re-invocation converges through
     `handleExistingBeads`' supersede-close + full recreate
     (`plan.go:459-510`) — and in doing so re-traverses the frontmatter
     Approved write (`:118`) idempotently, which also covers the
     Approved-write→first-create death gap;
   - (p2) SIMULATED-DEATH (state-construction) — partial dep wiring:
     post-Bead-4 a failed `bd dep add` WARNS and CONTINUES by design
     (Bead 4 Step 4b; today a silent `continue`, `plan.go:368-370`), so
     NO seam error terminates the run mid-wiring. The test instead
     CONSTRUCTS the mid-wiring death state (beads created, only a subset
     of edges wired, frontmatter already Approved) and re-invokes
     `ApprovePlan`, asserting convergence to the fully wired set via
     supersede-close + recreate;
   - (p2b) FORWARD-SAFE (documented, no kill test) — the `bead_ids`
     frontmatter write (P12; `writeBeadIDsToFrontmatter`,
     `plan.go:133-135` — a persisted state write between bead creation
     and the approval auto-commit): its own error is SWALLOWED into
     `result.Warnings` and the run CONTINUES (`plan.go:134`), so no
     seam can enact a kill; and a death immediately AFTER the write is
     re-runnable — the next `plan approve` re-derives the bead set from
     the same plan sections via `handleExistingBeads`' supersede-close
     + full recreate (`plan.go:459-510`, the p1 convergence path) and
     re-executes this same write with the fresh IDs, overwriting the
     stale list (bead creation + the `bead_ids` write form the
     idempotent-create region, so re-approval reconciles); the named
     seamless alternative, `mindspec bead create-from-plan`
     (`plan.go:126-129`), equally re-derives without re-approving —
     no downstream state depends on the interrupted write;
   - (p3) KILL — the approval auto-commit (`exec.CommitAll`,
     `plan.go:160-162`, and the residual-sync commit `:167-169` — both
     errors TERMINATE): real-git decorator executor (mechanism A); the
     real commit lands, then death; re-invocation converges
     (`updatePlanApproval` is idempotent);
   - (p4) FORWARD-SAFE — phase=implement metadata write
     (`planMergeMetadataFn`, `plan.go:180-183`): warn-degraded by design
     (`result.Warnings`); the run continues; re-run rewrites it.
   Assert each KILL/SIMULATED-DEATH re-invocation converges to a
   fully-approved plan with the complete wired bead set, or a clean named
   refusal.
6. Sweep the spec's fixed failure classes for regression pins: confirm each
   AC test from Beads 1–5 reproduces its original trigger and goes RED on
   revert; record the exact `go test <pkg> -run <test>` command per AC in the
   review evidence, per the spec's Validation Proofs contract.

**Verification**

- [ ] `go test ./internal/complete/... ./internal/approve/... ./internal/executor/...` passes including the new fault-injection tests
- [ ] AC-26 matrix: every enumerated point covered — KILL/SIMULATED-DEATH tests (c1–c5, c7–c8; i0–i1; i4 stages a–e; p0a–p0b, p1–p3) each re-invoke and converge to done or a named refusal (panel-staleness asserted for the advanced-tip case c2; the named collision refusal + documented flag-less recovery for the supersede-placeholder kills c7/i0; the closed-child supersede-safety refusal for p0a), and FORWARD-SAFE points (c6, i2, i3, i5, p2b, p4) each documented in the test file with the code cite proving the error is swallowed/warn-degraded. Every SIGNIFICANT persisted state-transition point is individually classified (KILL/SIMULATED-DEATH or DOCUMENTED-FORWARD-SAFE) in the c/i/p matrix above (P12); the remaining best-effort, error-swallowed, idempotent metadata writes — e.g. the recording bead marker (`complete.go:680-683`), the panel audit metadata (`complete.go:806-813` via `panel_advisory.go:847-871`), and `recording.StopRecording` (`impl.go:498-501`) — are forward-safe BY CONSTRUCTION: each swallows its own error and continues (`_ =` at `complete.go:682`; warn-prints at `panel_advisory.go:858-859`/`:868-869`; `result.Warnings` append at `impl.go:499-500`), so a mid-write interruption leaves an idempotent, re-runnable state; they are covered by this general forward-safety policy rather than individual kill-tests
- [ ] `rg -n 'preflight|reconcile' .mindspec/adr/ADR-0041-*.md` non-empty; ADR cited from all three verbs' preflight code; ADR-divergence gate passes via the named commands (Bead 6's own zero-override `mindspec complete` + the CheckADRDivergence fixture)
- [ ] Review evidence maps every AC-1..AC-26 to an exact runnable `go test -run` command

**Acceptance Criteria**

- [ ] AC-25 — ADR-0041 exists (Accepted), states the contract including the idempotent-migration exemption, is cited from the new preflight code in all three verbs, and the ADR-divergence gate passes (named verification commands)
- [ ] AC-26 — fault injection at each significant post-preflight mutation point per verb (`complete` incl. the supersede-ADR placeholder pre-create and the obligation-settlement write, `impl approve` incl. its supersede-ADR placeholder pre-create and the five intra-`FinalizeEpic` stages (incl. the post-cleanup stage e), `plan approve` incl. the supersede-close, the Approved-frontmatter write, and the forward-safe `bead_ids` frontmatter write — P12), each enacted through a mechanism that genuinely performs the real mutation AND terminates the run (real-git decorator executor / terminating tracker seam / executor stage hook / constructed death state); swallowed-error points documented forward-safe with cites instead of fictitious kill tests, and the best-effort, error-swallowed, idempotent metadata remainder (recording marker, panel audit metadata, `StopRecording`) covered by the Verification matrix's stated general forward-safety policy rather than individual kill tests; every re-invocation converges to completion or a clean recoverable refusal with a named recovery command

**Depends on**: Beads 1, 3, 4, and 5 — it fault-injects the FIXED mutation sequences of all three verbs (Bead 1: `complete`; Bead 3: `impl approve`/`FinalizeEpic`; Bead 4: `plan approve`), its plan-approve p1 kill wraps the bead-create seam and its p2 convergence test constructs the partially-wired state of the dep-wiring loop Bead 4 rewrites, and it must land after Bead 5's final edit of `complete.go`'s gate phase; it also inserts the ADR citations into the preflight code all of them produce.

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| AC-1 (lineage-authoritative resolution) | Bead 1, Step 1 / Verification AC-1 subtest |
| AC-2 (`--spec` mismatch refusal, byte-identical state) | Bead 1, Steps 1–2 / Verification AC-2 subtest |
| AC-3 (no tracker commit on `main`) | Bead 1, Step 3 / Verification AC-3 subtest |
| AC-4 (pathspec-scoped staging + warning) | Bead 1, Step 3 / Verification AC-4 subtest |
| AC-5 (merged-unclosed reconcile, gates vs landed commit, idempotent) | Bead 1, Steps 4–5 / Verification AC-5 subtests |
| AC-6 (decision-5 fidelity + doc-sync/ADR anti-bypass legs, explicit outcomes) | Bead 1, Step 5 / Verification AC-6 explicit outcomes |
| AC-7 (§6 panel-free parity on the reconcile path) | Bead 1, Step 5 / Verification AC-7 subtest |
| AC-8 (no-evidence refusal) | Bead 1, Steps 4–5 / Verification AC-8 subtest |
| AC-9 (branch-less obligation settlement + subsequent impl-approve gate passes) | Bead 1, Step 6 / Verification AC-9 explicit outcomes (`CheckPendingObligations` nil) |
| AC-10 (stale-OPEN cross-check via landed-merge identity; fresh-bead negative) | Bead 2, Steps 1, 6 / Verification AC-10 explicit outcomes (predicate produced by Bead 1 Step 4) |
| AC-11 (doctor in CI, non-zero fails build) | Bead 2, Step 5 / Verification AC-11 pin |
| AC-12 (doctor shares exported predicates, anti-drift pin) | Bead 2, Step 4 / Verification AC-12 pin (predicates produced by Bead 1 Step 4 and Bead 3 Step 4) |
| AC-13 (lifecycle-scoped finalize, both legs, four-plant fixture) | Bead 3, Steps 1–2, 7 / Verification AC-13 explicit outcomes |
| AC-14 (fail-closed enumeration/classification/plan-read) | Bead 3, Steps 2, 7 / Verification AC-14 subtests |
| AC-15 (finalize-orphan doctor+instruct, same check/text; stats vs `origin/main`) | Bead 2, Steps 2–3, 6 + Bead 3, Step 4 / Verification AC-15 explicit outcomes |
| AC-16 (exact-refname `RemoteHeadSHA`; lease doc comment) | Bead 3, Steps 5, 7 / Verification AC-16 subtests |
| AC-17 (vestigial refusal gone; `CommitCount` retention pinned) | Bead 3, Steps 6–7 / Verification AC-17 pins |
| AC-18 (scaffold→wiring round trip, ready-set ordering) | Bead 4, Steps 2, 5 / Verification AC-18 subtest |
| AC-19 (missing-`work_chunks` warning, no prose parsing) | Bead 4, Steps 4, 6 / Verification AC-19 subtest |
| AC-20 (dep-add failure warning naming both IDs) | Bead 4, Steps 4, 6 / Verification AC-20 subtest |
| AC-21 (scaffold shape pinned; both validators round-trip) | Bead 4, Steps 3, 5 / Verification AC-21 round-trip |
| AC-22 (advisory cross-domain WARN, termsafe, exit unchanged) | Bead 5, Steps 1, 4 / Verification AC-22 subtest |
| AC-23 (double-assignment plan-lint) | Bead 5, Steps 2, 4 / Verification AC-23 subtest |
| AC-24 (hermetic instruct tests, full suite in-worktree) | Bead 5, Steps 3–4 / Verification AC-24 check |
| AC-25 (ADR-0041 authored, cited in all three verbs, divergence gate passes via named commands) | Bead 6, Steps 1–2 / Verification ADR checks |
| AC-26 (fault injection at each significant post-preflight mutation point, ALL THREE verbs, convergence) | Bead 6, Steps 3 (`complete`, c1–c8), 4 (`impl approve`, i0–i5 incl. the five-stage `finalizeStepHookFn` with the post-cleanup stage e), 5 (`plan approve`, p0a/p0b + p1–p4 incl. the forward-safe p2b `bead_ids` write) / Verification AC-26 matrix (kill-tested vs documented-forward-safe per P7; significant-point sweep incl. supersede-ADR pre-creates, obligation settlement, supersede-close, Approved-frontmatter + `bead_ids` writes, post-cleanup finalize stage per P12; idempotent best-effort remainder per the stated general forward-safety policy) |

Requirement 1's gate-before-mutate restructure is carried per verb by Bead 1
(`complete`, pinned via AC-2's byte-identical refusal), Bead 3 Step 3
(`impl approve`, pinned by the zero-mutation-on-refusal call-order test), and
Bead 4 Step 1 (`plan approve`, pinned by the byte-identical misalignment
refusal) — with AC-26's per-verb kill matrix (Bead 6) exercising every
significant post-preflight mutation point end-to-end, and the idempotent
best-effort remainder covered by the matrix's stated general
forward-safety policy.
