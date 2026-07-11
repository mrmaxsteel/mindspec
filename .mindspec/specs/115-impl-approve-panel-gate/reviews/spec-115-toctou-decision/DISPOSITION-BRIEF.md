# spec-115 TOCTOU disposition DECISION panel (5 slots)

**A DESIGN-DISPOSITION panel**, not a normal review. Round-6 spec-approval (8/9, mostly APPROVE) surfaced ONE genuine correctness gap in the chosen C+B branch-existence design: a **TOCTOU transient-race** (raised by G2 codex, 0.96). This panel decides how to dispose of it. Your verdict directs the round-7 revision.

## The finding (G2 codex, orchestrator-confirmed as real)
C+B uses a simple `gitutil.BranchExists` bool probe for orphan detection (`internal/lifecycle/orphans.go:95` via `branchExistsFn`), which **fails OPEN** on an infra failure (an unreadable/locked ref masks as "absent"). The round-6 refutation claimed two structural facts fully close the un-gated-merge risk. G2 showed they don't, because the checks are **temporally separated**:
1. `exec.MergeBase("main", specBranch)` at `internal/approve/impl.go:249` succeeds (store readable at T0).
2. The orphan scan runs later (T1): `BranchExists(bead/<id>)` **transiently** fails (e.g. git ref-lock contention from a concurrent git process) → returns false → orphan masked → gate passes.
3. `FinalizeEpic` (`internal/executor/mindspec_executor.go:381-422`) runs later still (T2): the transient has cleared, the `bead/<id>` worktree is still enumerated by `WorktreeOps.List()` (:383), `IsAncestor` returns false (:394), `MergeInto` succeeds (:402) → the un-gated orphan is MERGED.
This requires **no raw-git tamper** (no tag/symref/manual merge) — just a transient probe failure recovering within one `ApproveImpl` pass. AC11 pins only faults already present at MergeBase; AC12 pins *persistently* invalid refs. Neither covers transient-then-recover. **The root cause: detection uses `BranchExists` (`orphans.go:95`) while the merge uses `WorktreeOps.List()` (`mindspec_executor.go:383`) — two different enumeration sources that can desync.**

## Constraint that shapes the options (PROVEN, do not re-open)
A probe-LEVEL fix is impossible: round 5 (9/9 RC) proved no git ref-probe distinguishes "transient/infra failure" from "genuine absence" by exit code, so you cannot fail-close the probe without false-refusing every cleanly-deleted branch. So the disposition is NOT "make BranchExists classify the transient."

## The two options — pick one (or a synthesis)
### Option A — Audited refutation → disclosed residual
Keep C+B's simple `BranchExists` probe. Add a THIRD disclosed residual to Non-Goals (joining raw-`git merge` and blp6) documenting the transient-probe-race: it requires a non-operator-controllable, sub-second git-ref fault that recovers within one synchronous `ApproveImpl` pass (bracketed by MergeBase success and FinalizeEpic success); the probe-level fix is proven impossible; and it merges the operator's OWN raw-`bd close`'d bead (not an external attacker's). Soften the refutation's "fully closes" to "closes except the disclosed transient residual." SPEC-WORDING ONLY, no design/code change. This is the spec-114-sanctioned "can't-fix-in-the-binary → audited evidence-refutation" escape ([[findings-never-outvoted]]).

### Option B — Structural fix: gate detection keys off `WorktreeOps.List()` (partial Approach D)
Make the impl-approve GATE's orphan detection enumerate from the SAME source FinalizeEpic merges from — `WorktreeOps.List()` — instead of (or in addition to) the `BranchExists` probe. Then scan and merge use ONE enumeration and CANNOT desync: any `bead/<id>` worktree FinalizeEpic would merge is exactly what the gate sees, and the remaining ancestry check is fail-CLOSED (error → refuse). Scoped to the GATE — does NOT change the shared `FindOrphanedClosedBeads` predicate that complete/next/doctor use for their (advisory) purposes; a branchy-but-worktreeless orphan is not FinalizeEpic-mergeable anyway (Fact 2), so worktree-enumeration is sufficient for the gate's merge-prevention job. Closes the TOCTOU by construction. COST: a real design change + re-review; must specify how the gate reconciles worktree-enumeration with the epic-scoped closed-bead detection, and whether the gate still needs the fail-closed epic-lookup/bd-list legs.

## Decisive questions for you to weigh
1. Is the transient-probe-race a real enough risk (ordinary flow, no tamper, but non-operator-controllable and physically narrow) to justify a design change (B), or is an honest disclosed residual (A) proportionate?
2. For B: does worktree-enumeration genuinely close the TOCTOU (scan & merge same source), and is it clean/small enough to not spawn its own review spiral? Does it lose any detection the gate needs (a mergeable orphan that has no worktree — does that exist)?
3. For A: is the residual honestly + fully scoped, and does spec-114 precedent (audited refutation for a can't-fix-in-binary residual) genuinely apply?
4. Blast radius, spec-coherence, and whether B re-opens anything the branch-probe decision settled.

## Per-slot lens
- **O1 (opus)** anti-gaming: does B actually close the TOCTOU with no new hole; is A's residual honestly complete; which better serves "no silent un-gated merge"?
- **O2 (opus)** architecture/blast-radius: B's scope + shared-predicate impact + ADR-0030 fit vs A's minimal wording change.
- **F1 (fable)** spec-coherence: which is cleaner to express falsifiably; for B, is the reconciliation specifiable with real ACs; for A, is the residual a non-synonym-dodge?
- **G1 (codex)** empirical: verify the enumeration-source desync (`orphans.go:95` BranchExists vs `mindspec_executor.go:383` WorktreeOps.List); for B, confirm worktree-enumeration is race-free vs FinalizeEpic and sufficient.
- **G2 (codex, the finding's author)** adversarial: for B, try to still desync scan-vs-merge (is there a residual TOCTOU even with worktree-enumeration?); for A, judge whether the residual is honestly bounded or whether the race is more reachable than "physically implausible." Focus on spec-correctness reasoning + read-only grounding, NOT on building exploits.

## Output
Write JSON to `<this-dir>/<slot>-toctou.json`: `reviewer_id`, `chosen_option` ("A" / "B" / synthesis), `confidence` (0-1), `rationale` (≤250 words), `rejected` (one line on the other), and for G2 an `option_b_closes_toctou` boolean + any residual repro. (Codex: WRITE the JSON to the exact path with a Write tool call as your final step; if you cannot, write an error there.)
