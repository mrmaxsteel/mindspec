# spec-128-bead1 — consolidated concrete_changes_required, round 2

Tally: 3 APPROVE (R1, R2, R5 — zero asks) / 3 REQUEST_CHANGES (R3 ×1, R4 ×1, R6 ×4 with hard_block RENEWED). Both round-1 hard-block drivers confirmed GONE by R6's own re-derivation; the renewal is a NEW evidence-bearing find of the same class one writer over. 6 asks in 4 groups. Artifact-gate screen: clear.

## G1 (R6, hard-block driver; scratch-reproduced) — EnsureMigratedWithCache repeats the discovery-failure class — MUST FIX
1. It discards GetChildren errors and derives `plan` from the nil slice, AND reuses the caller-supplied Cache inside the critical section (pre-lock cached children can be stale at write time). Fix: fresh per-call Cache created INSIDE the WithPhaseWriteLock callback; propagate GetChildren failures; no metadata write on failure. Tests: child-read-failure/no-write; caller cache pre-warmed with child state that changes before lock acquisition.
2. The SanctionedDerivedSync behavioral tests promised in round 1 are still missing from the repo: child-read failure returns the underlying error with ZERO merge writes; success exercises the real phase lock + merge seam.

## G2 (R3) — repair's in-lock derivation must be status-aware — MUST FIX
repairPhaseRunE's fn derives via bare DerivePhaseFromChildren(children) — wrong for terminal rows (returns `review` for all-closed). Use the status-aware surface with the fresh per-call cache already inside fn (DerivePhaseDetailWithCache; use detail.Derived, prefer the lock's fresh read over detail.Stored), keeping in-critical-section freshness.

## G3 (R4) — ADR-0035 copy-pastable recovery COMMANDS for lock timeouts, every user-facing writer — MUST FIX
`recovery: re-run once …` prose is not a command. Every surfaced ErrPhaseLockTimeout ends with a final safe copy-pastable `recovery: <command>` line naming .mindspec/phase.lock — release, complete, plan approve, spec approve, impl's done-write, impl adopt, EnsureMigrated callers — preserving each path's fatal-vs-best-effort exit semantics. Per-path tests.

## G4 (R6 ×2) — Test-rigor items — MUST FIX
1. Real acquisition-attempt barrier INSIDE the lock-acquisition path (a channel closed before calling WithPhaseWriteLock does not prove the goroutine reached acquisition; the 150ms negative window can pass with an unscheduled contender). Apply to phaselock, migration-contention, repair-contention tests.
2. AC-5(vi): make helper-built update maps reaching MergeMetadata-family calls outside the lock RED — a call/dataflow-anchored companion assertion is acceptable; the acceptance criterion requires a new unlocked helper-built writer to fail the gate.
