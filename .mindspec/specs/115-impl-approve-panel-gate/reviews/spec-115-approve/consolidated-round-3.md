# spec-115-approve round-3 consolidated findings (revision brief for round 4)

Round-3 spec-approval panel @ `77d8a1a9`: **7 APPROVE / 2 REQUEST_CHANGES**, no REJECT. Below the ≥8 threshold → revise + re-panel round 4.

- APPROVE: G1 codex (0.99), F1 fable (0.93), F2 fable (0.93), F3 fable (0.90), O1 opus (0.90), O2 opus (0.90), O3 opus (0.90).
- REQUEST_CHANGES: **G2 codex (0.96)** — `BranchExistsE` contract ambiguity (new false-refusal risk); **G3 codex (0.99)** — one AC proof-string not exact-named.

**Design CORE fully validated** — every reviewer confirms REFUSE closes o4fd, the branch-probe fix (BranchExistsE 4th fail-closed leg) genuinely closes G2's round-2 residue with no other fail-open leg, recovery converges, ownership/ADR/decomposition all sound, all 12 AC discriminators genuinely RED at the SHA. The 2 RC are BOUNDED refinements — one contract clarification (with an in-tree precedent) + one proof-string fix. No design change, no new scope. Both fixed below; neither out-voted.

## MUST-FIX (the 2 RC)

### 1. [G2 0.96 — anti-gaming, orchestrator-grounded] `BranchExistsE`'s three-way contract must distinguish "absent" from "infra failure"
The round-3 spec says the new `gitutil.BranchExistsE(name)(bool,error)` "preserves the `git rev-parse --verify` error." **Problem (orchestrator-verified):** `git rev-parse --verify refs/heads/<missing>` exits **128** (confirmed by running it), NOT exit 1 — so a naive error-preserving seam would return a non-nil error for EVERY genuinely-absent (normally merged-and-deleted) branch. Since the gate REFUSES on any core error, that would make `impl approve` refuse on the normal happy path (a cleanly-completed spec with deleted bead branches) — a false-refusal regression WORSE than the original bug.
**The fix is already grounded in the tree.** `internal/gitutil/gitops.go:13-19` defines `ErrRefNotFound` with a comment explicitly distinguishing "the named ref genuinely does [not exist]" from "a transient/structural git failure (exit 128, git ...)", and `RevParseRef` (`gitops.go:524-542`, esp. the `ExitCode() == 1` check at `:542`) already implements exactly this exit-1-vs-exit-128 distinction. FIX: specify `BranchExistsE`'s THREE-way contract explicitly and ground it on that existing precedent —
- branch present → `(true, nil)`;
- branch genuinely absent (the exit-code that means "no such ref", per the `RevParseRef`/`ErrRefNotFound` precedent) → `(false, nil)` — NOT an error, so a legitimately-deleted branch does NOT refuse;
- transient/structural git failure (the OTHER exit code / spawn failure) → non-nil error — the gate refuses.
Update R1 (the branch-probe leg description) + the OQ2 asymmetry note + Scope's `internal/gitutil` bullet to state this three-way contract and cite the `RevParseRef`/`ErrRefNotFound` exit-code precedent (so the implementer builds on it, not on a naive `BranchExists`-style bool). Add:
- (a) a NAMED `internal/gitutil` test asserting all three outcomes (present → true/nil; absent → false/nil; infra failure → error), e.g. `TestBranchExistsE_ThreeWay` with an existence discriminator;
- (b) an `internal/approve` **deleted-branch normal-path** test proving a legitimately-absent bead branch does NOT refuse (fold into AC2 or a new AC sub-case) — the anti-false-refusal pin;
- (c) tighten the `TestScanOrphanedClosedBeads_ErrorPreserving` / wrapper-parity coverage to a MIXED closed-bead list: a branch-probe (or ancestry) error for ONE bead must still let the fail-open `FindOrphanedClosedBeads` wrapper return later provable orphans (byte-identical behavior for complete/next/doctor), while `ScanOrphanedClosedBeads` reports the error and the impl-approve gate refuses.

### 2. [G3 0.99 — AC runnability] AC1's lifecycle test proof is not exact-named
AC1 chains `grep -q 'func TestScanOrphanedClosedBeads_ErrorPreserving' internal/lifecycle/*_test.go && go test ./internal/lifecycle` — the final clause is a PACKAGE-WIDE run, not the exact-named `-run 'TestScanOrphanedClosedBeads_ErrorPreserving'` the Validation-Proofs section claims every named test has. FIX: chain the existence grep to `go test ./internal/lifecycle -run 'TestScanOrphanedClosedBeads_ErrorPreserving' -v` (a full-package `go test ./internal/lifecycle` MAY be retained additionally as a no-regression pin, but the named run must be present). Make the Validation-Proofs "every named test has an exact-named run" claim true.

## Disposition
Fix 1 (three-way contract + 3 test additions, all grounded on the existing `RevParseRef`/`ErrRefNotFound` precedent — no new design) and 2 (one-clause proof edit). No other changes. Re-panel round 4 (9-slot, ≥8). This is the last expected refinement pass — the core and all AC discriminators are validated; round 4 verifies only these two bounded fixes.
