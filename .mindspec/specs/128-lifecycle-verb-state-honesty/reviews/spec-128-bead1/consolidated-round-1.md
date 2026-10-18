# spec-128-bead1 — consolidated concrete_changes_required, round 1

Tally: 0 APPROVE / 6 REQUEST_CHANGES, **hard_block from R6** (evidence-bearing: error-discarding sanctioned sync + red -short suite in the linked worktree via non-hermetic lock-root resolution). 14 asks in 8 groups. This is a fix-round halt (implementation defects), not an artifact-gate halt. Orchestrator action taken: the stray unheld `/Users/Max/replit/mindspec/.mindspec/phase.lock` is removed.

## G1 — Error/presence-aware reads under the lock (R6-hb + R5 + R3, convergent core) — MUST FIX, drives the hard block
1. `SanctionedDerivedSync` currently discards the GetChildren/derivation-read error and derives `plan` from the nil slice — bead 2's release wiring would turn FAILED DISCOVERY into a sanctioned backward write (the spec's own #220 bug class reproduced in its own fix). Propagate the error; NO metadata write on read failure; behavioral tests (no-write-on-failure + a success test over the real lock/merge seam).
2. The under-lock epic read: propagate lookup/decode errors BEFORE invoking fn or writing; preserve invalid-but-present phase values as unknown (never convert to absence); EnsureMigrated backfills only a genuinely missing/empty field. Tests: fresh-read error and invalid-but-present metadata can trigger neither migration nor any merge write, incl. the fresher-writer contention shape.
3. repair.go: move the phase derivation INSIDE the critical section (fresh children-read + derive with a per-call Cache, as sanctioned_sync.go:38-40 does), so guard-compare is fresh-vs-fresh per spec.md:113's pinned read→derive→compare→write mechanism. Keep AC-5(v)'s stale-target leg via SetListJSONForTest.

## G2 — Hermetic fixtures; the required suite must be green FROM the bead worktree (R4 + R6, convergent; drives the hard block) — MUST FIX
Every test reaching WithPhaseWriteLock injects/fixture-scopes the lock root (t.TempDir): targeted and -race runs of the repair tests must never resolve or write the enclosing primary checkout; the full `go test ./... -short` must pass from the bead worktree; no blockable dirt is created in ANY checkout; fixture lockfiles confined and cleaned.

## G3 (R4) — Lock-error propagation at every user-facing writer — MUST FIX
release.go and complete.go currently discard WithPhaseWriteLock errors. Every surfaced timeout names `.mindspec/phase.lock` and ends with an ADR-0035 re-run recovery line that never advises deleting the file. Per-path tests, including current best-effort writers.

## G4 (R1 + R2, convergent) — The plan-named crashed-holder recovery test was silently dropped — MUST FIX
Add the unix leg (real subprocess acquires the lock at a fixture path, SIGKILLed while holding; parent's next acquire succeeds well under the wait bound) + the plan's Windows-leg disposition (seam-driven on non-Windows CI) — or an explicitly argued deviation for the panel to accept. Silent absence is the defect.

## G5 (R2 + R6, convergent) — Writer-inventory walker hardening or honest narrowing — MUST FIX
Catch (a) IndexExpr string-key WRITES outside the lock extent (map-index assignment; rvalue reads stay unflagged), (b) const-bound key Idents (file-local at minimum), (c) helper-built update maps reaching MergeMetadata-family writes — and prove red-on-new-unlocked-writer. OR, if the composite-literal anchor is argued sufficient for the accidental-drift threat model, narrow the test's doc comment to match the mechanism (the claim may not exceed it).

## G6 (R6) — Deterministic barriers in the concurrency tests — MUST FIX
Replace sleep-based scheduling in the phaselock and migration-contention tests: assert attempted-before-release, cannot-enter-before-release, enters-after, no error.

## G7 (R5) — Windows error classification — MUST FIX
Retry only ERROR_LOCK_VIOLATION on the synchronous FAIL_IMMEDIATELY path; other LockFileEx errors close the retained handle and return immediately (never wrapped as ErrPhaseLockTimeout). Cover via a Windows-call seam.

## G8 (R3) — WithPhaseWriteLock doc-comment caller obligations — MUST FIX (small)
State: (a) freshness covers ONLY the stored mindspec_phase value passed to fn — all other decision inputs must be read inside fn; (b) the lock is non-reentrant — nested acquisition self-blocks to ErrPhaseLockTimeout.
