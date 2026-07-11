# spec-115-approve round-2 consolidated findings (revision brief for round 3)

Round-2 spec-approval panel @ `eb6a2ed1`: **7 APPROVE / 2 REQUEST_CHANGES**, no REJECT. Below the ≥8 threshold → revise + re-panel round 3.

- APPROVE: G1 codex (0.99), F1 fable (0.93), F2 fable (0.93), F3 fable (0.85), O1 opus (0.89), O2 opus (0.90), O3 opus (0.90).
- REQUEST_CHANGES: **G2 codex (0.98)** — anti-gaming; **G3 codex (0.99)** — AC runnability/discrimination.

**Design CORE re-validated** by all 3 Opus + all 3 Fable + G1: REFUSE closes o4fd, recovery re-gates via `complete`, no import cycle, epic-scoped fail-closed gate, ownership move real, all 10 round-1 items + 4gsz ADDRESSED. The two RC are refinements — one real residual fail-open leg (G2) and AC-proof hardening (G3). No redesign. Every finding fixed or evidence-refuted below; none out-voted.

## MUST-FIX (the 2 RC)

### 1. [G2 0.98 — anti-gaming, orchestrator-verified in tree] Branch-existence probe still fails OPEN
`ScanOrphanedClosedBeads`'s branch-existence trigger uses `gitutil.BranchExists(name) bool` (`internal/gitutil/gitops.go:94-100`: `return cmd.Run() == nil`) via `branchExistsFn` (`orphans.go:40`, used at `:95`). This maps EVERY `git rev-parse --verify` error to `false` — so a branch-probe INFRA error is indistinguishable from a cleanly-deleted branch. For an ordinary unresolved-RC bead there is no R3 obligation to backstop it, so the gate passes, then `FinalizeEpic`'s independent worktree-listing loop (which does NOT use `BranchExists`) still sees and merges the branch — the o4fd class re-opened via an infra error. This contradicts R1's "refuses when it cannot prove otherwise / any infra error" posture, which currently covers only epic-lookup / bd-list / ancestry. FIX: the error-preserving core must ALSO preserve the branch-probe error (add a `(bool, error)` git/ref seam, e.g. a new `gitutil.BranchExistsE` or capture the `rev-parse` error in the lifecycle core); the impl-approve gate REFUSES on that error; `FindOrphanedClosedBeads` wrapper keeps swallowing it (complete/next/doctor byte-identical). Update R1's error-preserving-core enumeration to list the branch-existence probe as one of the fail-closed legs; extend AC1(b)'s falsifier + fault-injection test to prove a branch-probe error refuses before epic-close / phase-write / FinalizeEpic / merge / push, while the wrapper stays fail-open.

### 2. [G3 0.99 — AC runnability] Six AC proofs are non-discriminating at the review SHA
`go test ./internal/approve -run '<Pat>'` EXITS 0 with `[no tests to run]` when `<Pat>` matches nothing (verified). So AC1/AC3/AC5/AC6's `go test -run` proofs pass trivially today; AC2 passes on unrelated EXISTING `ApproveImpl` tests; AC4 passes on the PRE-extension `TestApproveImplCallOrder`. Only AC7 carries an existence check. FIX: give AC1-AC6 explicitly-NAMED new test functions/subtests and pair each proof with an existence discriminator (`grep -n 'func Test<Name>' <file>` chained with `&&`, exactly as AC7 does) so every proof FAILS at `eb6a2ed1` and passes only once the named test lands. AC4 additionally needs a discriminator for the EXTENDED ordering assertion (e.g. assert the new orphan/obligation-gate symbol appears in the AST call-order list at the required position — a check that is RED against today's `impl_test.go:714`).

### 3. [G3 0.99] AC7 convergence-test package/seam design (avoid the import cycle)
R3 has `internal/approve` import the check-only obligation predicate exported from `internal/complete`. Grounding (orchestrator-verified): today `internal/approve` does NOT import `internal/complete`, and `internal/complete` does NOT import `internal/approve` (only a comment at `complete.go:486`) — so `approve → complete` is acyclic. BUT AC7 places the convergence test "beside the existing `internal/complete` gate suite"; an in-`package complete` test that calls `ApproveImpl` would import `approve → complete` = cycle. FIX: split AC7 into (a) a package-local `internal/complete` seam test for "`complete.Run` re-gates the already-closed orphan (Blocks at step 2.25 before step-4 close tolerance; succeeds after 114 refutation)", and (b) a NAMED `internal/approve` test for "after settle+merge, `ApproveImpl` passes the R1 gate" — no cross-package cycle. Keep an existence check on BOTH named tests. (Alternatively specify the predicate extracted to a leaf package both import — but the two-package test split is the minimal fix.)

### 4. [G3 0.99] Branchless-R3 recovery line is not truthful
R3's refusal for a branchless bead with an unsettled obligation ends with `mindspec complete <bead>`, but the spec itself (OQ3) admits `complete.Run` errors at the step-3.5 merge-base (`complete.go:492-495`) BEFORE the step-3.75 reconciliation for a branch-less bead — so that recovery command does not actually settle the obligation. FIX: give the R3 branchless-obligation refusal a DISTINCT, truthful recourse message — restore the `bead/<id>` branch ref (so `complete` can reach reconciliation) then `mindspec complete <bead>`, or settle the obligation out-of-band — separate from the orphan-with-branch message whose `mindspec complete <bead>` IS runnable. Branchless reconciliation itself stays deferred to `mindspec-h4n5` (P3). Add/adjust the AC6 sub-case to assert the branchless message names the restoration prerequisite, not a bare command that fails.

### 5. [G3 0.99] Persist `mindspec-h4n5` in pinned project state
`mindspec-h4n5` exists in the beads DB (OPEN, P3) but is absent from the branch-pinned `.beads/issues.jsonl` at `eb6a2ed1` (tracked file, 0 hits), so a reviewer pinning to the SHA cannot verify the OQ3 deferral home. FIX: refresh the beads export so `.beads/issues.jsonl` on the branch records h4n5, and commit it (a separate `chore(beads)` commit is fine). Verify `grep -c mindspec-h4n5 .beads/issues.jsonl >= 1` afterward.

### 6. [G3 0.99] AC10 must enforce the "exactly one claimant" invariant
AC10 checks only workflow-positive + execution-negative (two domains), not the asserted exactly-one-claimant invariant across ALL domains. FIX: strengthen AC10's proof to count `internal/lifecycle/**` claims across EVERY `.mindspec/domains/*/OWNERSHIP.yaml` and assert the count is exactly 1 AND that the one claimant is `workflow` (e.g. `grep -l 'internal/lifecycle' .mindspec/domains/*/OWNERSHIP.yaml` yields exactly the workflow file).

## ADDRESS-OR-REFUTE (O3 LOW, non-blocking but never out-voted)

### 7. [O3 LOW] `cmd/**` over-declared + pre-existing execution self-ownership gap
(a) `cmd/**` is declared in Impacted Domains at a coarse path granularity — tighten to the actual `cmd/mindspec` surface the spec touches (or drop if no cmd file changes). (b) The pre-existing execution self-ownership gap is NOT introduced by 115 — evidence-refute as out-of-scope pre-existing (optionally file a P3 follow-up); do NOT expand 115's scope to fix it. State the disposition in the revision.

## Disposition
Fix 1-6; address 7(a), evidence-refute 7(b). No design change — the REFUSE core, recovery convergence, epic-scoped fail-closed gate, and ownership move all stand. Then re-panel round 3 (9-slot, ≥8).
