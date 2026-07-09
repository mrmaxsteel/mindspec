# uopd-as-of-fix — Round 1 Review Panel

**Worktree**: /Users/Max/replit/mindspec/.claude/worktrees/agent-a6e9b49449b5b6cf9 (branch checked out; you may run `go test` here)
**Repo (for diffs)**: /Users/Max/replit/mindspec
**Branch**: fix/uopd-as-of-committed-read
**Commit under review**: d7f5c67a084e52c985aadb7abec8824cbe3132f4 — fix(complete): verify close via bd show --as-of HEAD committed-state read (mindspec-uopd)
**Panel type**: ad-hoc standalone-bug panel (bead mindspec-uopd, P2) — NOT a spec-lifecycle gate. Bead-review model mix: 3 Opus + 3 Sonnet.

## What the work does

`mindspec complete` must never merge/clean up a bead whose `bd close` did not durably persist (the recurring "2u0u" silent-close-loss). The old verifier `defaultVerifyCommitted` could only re-read via the same `bd show --json` path that had just been written — detection-via-the-same-read — because bd exposed no committed-state read (documented in the file's HONESTY-CLAUSE comment, gap tracked as this bead). bd ≥1.0.4 now ships `bd show <id> --as-of <ref>`, a true committed-state read (bd's embedded Dolt auto-commits every write, so `--as-of HEAD` reads committed state). The change: `defaultVerifyCommitted` now (1) reads via a new `next.FetchBeadAsOf(id, "HEAD")` (shared JSON parsing refactored into `parseBeadShowJSON`; exec via the canonical `runBDFn` seam), (2) requires status "closed" exactly as before, (3) falls back to the old same-read path ONLY when the error is a narrowly-detected unsupported-flag error (`bead.IsUnsupportedFlagError`: requires an `*exec.ExitError` in the chain AND stderr containing both "unknown flag" and the flag name), logging `event=complete.committed_read_downgraded` to stderr, (4) any hard read failure or not-closed status remains an error in either path. The forced `bd dolt commit` (`doltCommitFn`) and the bounded retry wrapper are untouched. HONESTY-CLAUSE comment rewritten to describe the closed gap.

## Files in scope (final state at d7f5c67a)

- `internal/bead/bdcli.go` (+21) — `IsUnsupportedFlagError(err, flag)`
- `internal/complete/complete.go` (+100/-26) — `fetchBeadAsOfFn` seam, rewritten `defaultVerifyCommitted`, `verifyCommittedSameRead`, comment rewrite
- `internal/complete/complete_test.go` (+203) — `TestDefaultVerifyCommitted` (4 table cases), `TestDefaultVerifyCommitted_AsOfHardReadFailureNeverFallsBack`, `TestFetchBeadAsOfFnDefaultsToNextFetchBeadAsOf`, `fakeBDExitError` + `captureStderr` helpers
- `internal/next/beads.go` (+30) — `FetchBeadAsOf`, `parseBeadShowJSON` refactor

## Shared modules reused (unchanged)

- `internal/bead` `RunBD`/`runBDFn` exec path (ADR-0030 os/exec boundary)
- `doltCommitFn = bead.DoltCommit` and the bounded retry wrapper in `internal/complete`
- Existing `implXxxFn`-style function-var test seams

## Your job

Evaluate the work cold (round 1). Scrutinize in particular:
1. The `--as-of HEAD` semantics claim: under bd's embedded auto-commit mode, does `--as-of HEAD` truly read COMMITTED state (and is HEAD the right ref vs e.g. WORKING)? Probe empirically if your lens calls for it — bd 1.1.0 is installed.
2. `IsUnsupportedFlagError` narrowness: can a real failure (bead-not-found, Dolt lock, network) ever be misclassified as unsupported-flag (silent downgrade)? Can a genuinely-unsupported flag ever be missed (hard failure where fallback was intended)?
3. Behavior parity: in the fallback path, is the verifier exactly as strong as the pre-change verifier? Is the never-proceed-on-unverified-close invariant preserved in ALL paths?
4. The `internal/next` refactor: are existing `FetchBeadByID` callers behavior-identical after the `parseBeadShowJSON` extraction?
5. Test quality: do the tests exercise the REAL `errors.As` chain (the `fakeBDExitError` approach) rather than a fake that can't regress?

Verdict: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `/Users/Max/replit/mindspec/.mindspec/reviews/uopd-as-of-fix/<your-slot>-round-1.json` with keys:
`reviewer_id`, `verdict`, `confidence`, `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`. An artifact-gate finding may set `"hard_block": true`.
