# spec-115 TOCTOU disposition — OUTCOME: Option B + honest residual (4/5)

**Panel:** G1 codex B (0.93); F1 fable B (0.87); O1 opus B (0.83); G2 codex synthesis=B+ (0.94); O2 opus A (0.82). → **Option B**, 4/5.

## Decision
The impl-approve GATE gains a worktree-enumeration merge-prevention leg, keyed off the SAME source FinalizeEpic merges from (`WorktreeOps.List()` / `bd worktree list`), IN ADDITION TO the shared bead-centric `FindOrphanedClosedBeads` predicate (which stays for complete/next/doctor + R3's worktreeless-orphan obligation coverage). Rationale:
- The round-6 TOCTOU: detection used `BranchExists` (`git rev-parse --verify refs/heads/bead/<id>`, `gitops.go:98` — a lockable ref, the transient-fail vector) while the merge uses `WorktreeOps.List()` (`bd worktree list`, does not probe that ref). Keying the gate off the worktree list means a transient ref-probe failure can no longer hide a merge candidate.
- Option A (audited-refutation residual) REJECTED 4/5: misapplies the spec-114 escape (that's for can't-fix-in-binary; B fixes it, so round-5's probe-impossibility proof does not justify A — "the gaming move"), AND A's refutation is dishonest (Fact 2's "masked ref has no worktree / fails MergeInto" is FALSE for the transient-then-recover case: by merge time the transient cleared, the worktree is live, MergeInto succeeds → silent un-gated merge).
- B's fork cost (O2): mitigated by "in addition to" — B adds a merge-prevention leg for the gate, does not replace the shared predicate.

## Round-7 spec changes
1. R1: add the worktree-enumeration merge-prevention leg (gate refuses if any `bead/<id>` worktree enumerated by `WorktreeOps.List()` belongs to a closed epic bead that is NOT an ancestor of the spec branch), keyed off the same source FinalizeEpic merges from; the pre-existing `BranchExists`/shared-predicate detection stays (breadth: branchy-but-worktreeless orphans + R3). Ancestry leg stays fail-closed.
2. New RED-on-revert AC (F1's shape): seam-force `branchExistsFn=false` (simulating the transient) while the orphan bead's worktree exists + is non-ancestor → the gate MUST still refuse (via the worktree-enum leg); reverting to probe-only detection turns it RED. Name it e.g. `TestApproveImpl_WorktreeEnumRefusesDespiteProbeMiss`.
3. HONEST residual (O1/G2/F1): the Non-Goals refutation must NOT claim total closure — disclose the irreducible narrower residual: the gate's `List()` and FinalizeEpic's `List()` are two temporally-separated calls; a bead worktree ADDED between them (concurrent lifecycle mutation) is out of scope because mindspec serializes lifecycle operations (concurrent impl-approve / worktree creation on one repo is unsupported). Keep Facts 1&2/AC11/AC12; reword the probe-transient refutation to "closed by the worktree-enum leg" rather than "physically implausible".
4. (Optional) file a follow-up for the true single-snapshot handoff (thread the vetted worktree list from approve into FinalizeEpic) if absolute atomic closure is later wanted — bounded-by-serialization means not required now.

Also bundle: G3's round-6 AC10 fix (make the ownership exactly-one-claimant proof RED-on-revert via a new named structural test, not the pre-applied grep).
