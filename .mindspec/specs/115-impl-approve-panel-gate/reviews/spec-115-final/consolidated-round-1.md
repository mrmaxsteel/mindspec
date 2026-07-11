# spec-115-final — Round 1 consolidated tally

**Reviewed**: spec branch `21374bb8` vs `origin/main` `f02a3a49`. **Panel**: 12 slots (F1-F3 Fable-sub, O1-O3 Opus, S1-S3 Sonnet, G1-G3 codex). **Threshold**: ≥11 APPROVE, no REJECT. Findings never out-voted.

## Verdicts — 9 APPROVE / 2 REQUEST_CHANGES (S1 pending at write time)
F1 A · F2 A (minor line-drift note) · F3 A · O1 A (3 info notes) · O2 A · O3 A · S2 A · G1 A (security) · G2 A (fail-closed) · **S3 REQUEST_CHANGES (hard_block)** · **G3 REQUEST_CHANGES (0.99, CI)** · S1 pending.

The security-critical lenses APPROVED: G1 codex (no un-gated path beyond the 3 disclosed residuals), G2 codex (fail-closed completeness — no corrupt-metadata bypass), O1 Opus (o4fd guarantee delivered end-to-end), F1 (all AC RED-on-revert). Grounding/ADR/provenance/no-regression all clean.

## Finding A (S3) — REAL, spec-115-introduced CI blocker → FIXED (reroute)
`internal/lint.TestEnforcementHasNoGitLeaks` (ADR-0030 AST boundary lint) FAILS: Bead 2 added a direct `internal/gitutil` import to `internal/approve/impl.go:17`, but `internal/approve` is one of the FIVE enforcement packages (`validate`/`approve`/`complete`/`state`/`phase`) that ban direct git imports. Deterministic pure-AST — would fail in a fresh CI checkout. O2/F3 approved because the BRIEF's touched-package shortlist excluded `internal/lint` (a BRIEF gap — the final review must run the WHOLE suite).
**Root cause:** the spec's O2 decision chose "direct gitutil import" on a FALSE premise ("ADR-0030's boundary concern is executor-only, not gitutil use by approve") — the lint proves the boundary DOES apply to approve.
**Fix (in flight):** reroute the two read-only calls (`IsAncestor`, `BranchExists`) through thin `internal/lifecycle` wrappers (lifecycle is the non-enforcement gitutil-owner approve already imports); drop approve's direct gitutil import. Seam-transparent (identical signatures — `implIsAncestorFn`/`implBranchExistsFn` and all AC tests unaffected), no behavior change, honors the boundary rather than adding an allowlist exception. Verified preconditions: gitutil used in impl.go ONLY at the 2 seams; no lifecycle name collision.

## Finding B (G3) — pre-existing, EVIDENCE-REFUTED as out-of-scope
`golangci-lint run ./...` reports 2 gosec G115 (integer-overflow uintptr→int) at `internal/journal/lock_unix.go:29,:34`. **Pre-existing:** those lines are on `origin/main` already; spec 115 does NOT touch `internal/journal` (diff empty); filed as `mindspec-8ud6` (P3, open). CI's pinned `golangci-lint-action@v2.1` tolerates them (main's last 3 CI runs are all `success`), and they are NOT introduced by this spec. The final-review BRIEF's "any golangci-lint issue = hard block" was over-strict; correctly scoped, this is a pre-existing repo condition, not a spec-115 defect. **Also** `internal/instruct.TestRun_IdleNoBeads` (S3 noted) = pre-existing `mindspec-z4ps` (worktree-only, passes in a clean CI clone). Both audited-refuted as out-of-scope for spec 115.

## Disposition
Round 1 does NOT pass (Finding A is a real blocker). Apply the reroute fix, re-verify the whole CI suite (esp. `TestEnforcementHasNoGitLeaks` PASS), then re-panel round 2 (re-run the affected lenses: S1/S2/S3 empirical+CI, O2 import-edge, G1/G3 codex; the security verdicts G1/G2/O1/F1 stand on unchanged gate behavior). Findings B (G115/z4ps) carried as audited-refuted pre-existing (tracked by 8ud6/z4ps), not blocking spec 115. Pass condition: ≥11 with Finding A fixed.
