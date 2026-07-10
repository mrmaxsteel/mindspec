# spec-115-approve — Round 4 Spec-Approval Panel (9 reviewers, three families)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-115-impl-approve-panel-gate`
**Branch**: `spec/115-impl-approve-panel-gate` @ **4cbf437ba3c58643246a760eb1ac1ed73c14e4ed**. Base = `origin/main` (`f02a3a49`, includes spec 114).
**Under review**: `.mindspec/specs/115-impl-approve-panel-gate/spec.md`.
**Panel**: 9 slots — F1–F3 Fable, O1–O3 Opus, **G1–G3 codex**. **Pass = ≥8 APPROVE, no REJECT.** Findings never out-voted.

**READ-ONLY**: verdict JSON only; pin reads to `4cbf437b`; ALL scratch under ABSOLUTE `/tmp`; NEVER edit the worktree; leave `git status` clean.

**History**: R1 3/6 → R2 7/9 (fixed) → R3 @ `77d8a1a9` = **7 APPROVE / 2 RC** (G2 codex: BranchExistsE contract; G3 codex: AC1 proof-string). This round (R4) is a NARROW verification of ONLY those two bounded fixes at `4cbf437b` — the design core + all other ACs are fully validated across 3 rounds; do NOT re-litigate settled items, but DO flag any NEW defect the round-3 edits introduced.

## What this spec is (context)
Spec 115 closes the last silent panel-gate bypass: `mindspec impl approve`→`FinalizeEpic` auto-merges beads closed via raw `bd close`. Design (validated): REFUSE + route recovery to `mindspec complete`. The gate detects orphans via an error-preserving `ScanOrphanedClosedBeads` core (fail-CLOSED on infra errors) with `FindOrphanedClosedBeads` as the fail-open wrapper (complete/next/doctor unchanged).

## The two round-3 fixes to verify (grade ADDRESSED / PARTIAL / MISSED / NEW_ISSUE)
Full round-3 brief: `consolidated-round-3.md` (same dir).

### Fix 1 [G2 0.96] — `BranchExistsE` three-way contract (no deleted-branch false-refusal)
**Round-3 problem**: the spec's `gitutil.BranchExistsE(name)(bool,error)` was to "preserve the `git rev-parse --verify` error" — but plain `--verify` exits **128** on a missing ref (verified), so a naive error-preserving seam would return an error for EVERY cleanly-deleted branch → `impl approve` would refuse on the normal happy path (a completed spec whose bead branches were merged+deleted) = a false-refusal regression.
**The grounded fix** (built on an existing in-tree primitive): `internal/gitutil/gitops.go:13-19` (`ErrRefNotFound`) + `RevParseRef` (`:524-545`, using `--verify --quiet` so a genuinely-absent ref exits **1** → distinguished from the exit-128 infra-failure class). Verify the spec now specifies `BranchExistsE`'s THREE-way contract:
- present → `(true, nil)`;
- genuinely absent (exit-1 per the `--verify --quiet` / `ErrRefNotFound` precedent) → `(false, nil)` — NOT an error, so a legitimately-deleted branch does NOT refuse;
- transient/structural git failure (exit-128 class) → non-nil error → gate REFUSES.
Verify: (a) R1's branch-probe leg + OQ2 asymmetry note + Scope `internal/gitutil` bullet all state this three-way contract and cite the `RevParseRef`/`ErrRefNotFound` exit-1-vs-128 precedent; (b) R1's Falsified-if gained an anti-false-refusal falsifier ("genuinely-absent branch yields anything but (false,nil) → falsified"); (c) AC1 gained `TestBranchExistsE_ThreeWay` (internal/gitutil, existence-discriminated); (d) AC2 gained a deleted-branch normal-path pin `TestApproveImpl_DeletedBranchNoRefusal` (a legitimately-absent branch proceeds to FinalizeEpic, does NOT refuse); (e) AC1(b) wrapper-parity tightened to a MIXED closed-bead list (a probe/ancestry error on ONE bead → Scan errors + gate refuses, while the fail-open wrapper still returns later provable orphans, byte-identical for complete/next/doctor). Both new test names must be 0 hits at `4cbf437b` (genuinely RED).

### Fix 2 [G3 0.99] — AC1 lifecycle proof exact-named
Round-3: AC1's final clause was package-wide `go test ./internal/lifecycle` not the exact-named `-run`. Verify it is now `go test ./internal/lifecycle -run 'TestScanOrphanedClosedBeads_ErrorPreserving' -v` (a bare package run may additionally remain as a no-regression pin), and the Validation-Proofs "every named test has an exact-named run" statement is now true.

## What to verify at `4cbf437b`
1. **Both fixes ADDRESSED** — grade Fix 1 (with its 5 sub-parts a-e) and Fix 2. A fix marked done but not actually done, or a new test name that is NOT 0-hits (non-discriminating), = REQUEST_CHANGES.
2. **The three-way contract is correct + grounded** — does it genuinely prevent the exit-128 false-refusal while still failing closed on true infra errors? Is the `--verify --quiet` vs plain-`--verify` distinction accurately cited (read `gitops.go:13-19` + `:524-545` + `BranchExists` at `:94-100`)? Any residual ambiguity (G2 lens)?
3. **No NEW defect from the round-3 edits** — the diff `77d8a1a9..4cbf437b` should touch only spec.md (+ the separately-committed round-3 panel artifacts). Confirm nothing else changed and no AC regressed.
4. **Discriminators still RED** — `TestBranchExistsE_ThreeWay`, `TestApproveImpl_DeletedBranchNoRefusal`, and all prior named tests absent; `BranchExistsE`/`ScanOrphanedClosedBeads` 0 code hits.
5. **Core + OQ coherence intact** — no design change; all 4 OQs `[x]`; OQ2 asymmetry now correctly scopes fail-closed to INFRA errors (not legitimate absence).

## Per-slot lens
- **F1** falsifiability (new discriminators RED); **F2** grounding (the exit-1-vs-128 / RevParseRef precedent cited accurately); **F3** contradiction/OQ (three-way contract vs OQ2/R1 coherence).
- **O1** anti-gaming (three-way contract closes the residue WITHOUT a false-refusal hole); **O2** ADR/ownership (gitutil seam unchanged-ownership); **O3** decomposition (the extra gitutil test doesn't over-scope bead 1).
- **G1** grounding empirical (verify `gitops.go` exit-code lines + the two new tests absent); **G2** anti-gaming (YOUR finding — confirm the three-way contract genuinely prevents both the fail-open merge AND the deleted-branch false-refusal, no residual); **G3** AC-runnability (YOUR finding — confirm AC1's exact-named run + every AC discriminator still RED).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-4.json`: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence` (0–1), `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
