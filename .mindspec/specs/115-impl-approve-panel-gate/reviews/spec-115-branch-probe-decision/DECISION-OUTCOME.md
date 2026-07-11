# spec-115 branch-probe decision — OUTCOME: Approach C+B

**Panel (5 slots + 1 adversarial sub): C+B chosen.**
- G1 codex — C+B (0.96)
- O2 Opus — C, +B as hardening (0.80)
- F1 Fable — C/B (0.85)
- G2-sub Opus (adversarial, subbed for content-filtered G2 codex) — C+B HOLDS; tag/symref-shadow is NOT FinalizeEpic-reachable (worktree requires a real branch; tag→detached/unenumerated, symref→worktree-add fails) AND is already-disclaimed raw-git tamper.
- O1 Opus — dissent A (0.60); but O1 empirically CONFIRMED C's refutation holds ("could not break it"). Its objection (safety emergent/unpinned) is addressed by +B: pin the structural facts as named RED-on-revert ACs.

## The decision
DELETE the exit-code-classification branch-existence probe (proven impossible round 5, 9/9). Revert to a simple version-portable existence probe (present=exit 0 → candidate orphan; nonzero → treat as absent → a genuinely-deleted branch NEVER false-refuses). The gate's fail-closed safety for the branch-existence leg is NOT the probe — it is TWO structural facts, pinned as named ACs:
1. `exec.MergeBase("main", specBranch)` at `impl.go:249` runs BEFORE the scan and aborts ApproveImpl fail-closed on a whole-`refs/heads`-unreadable store.
2. FinalizeEpic (`mindspec_executor.go:381-422`) merges ONLY `WorktreeOps.List()` real-branch entries; a masked single-corrupt-ref orphan either has no worktree (unenumerated) or its merge fails (fail-closed).
The absent-vs-infra ambiguity is audited-REFUTED (it cannot produce an un-gated merge). The OTHER core legs (epic-lookup / bd-list / ancestry) still fail closed (they cleanly error). git-version-floor Approach A rejected (no floor exists in the project; breaks pre-2.43 LTS; fallback re-imports the spiral). Approach D rejected (forks the shared predicate).

LESSON (→ memory): git does not distinguish "ref absent" from "ref-store unreadable" by a single ref-probe's exit code; do not build a fail-closed gate on that classification. The real safety is structural (pre-scan store-health via MergeBase + worktree-gated merge), not the probe. Spec-114 "delete > patch" confirmed again.
