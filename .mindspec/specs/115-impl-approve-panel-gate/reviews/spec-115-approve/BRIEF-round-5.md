# spec-115-approve — Round 5 Spec-Approval Panel (9 reviewers, three families)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-115-impl-approve-panel-gate`
**Branch**: `spec/115-impl-approve-panel-gate` @ **e6dfa4cf45b85ce4e896e3c037d9c235a9151291** (spec.md content = commit `f07cce6a`; `e6dfa4cf` adds the round-4 review artifacts on top). Base = `origin/main` (`f02a3a49`).
**Under review**: `.mindspec/specs/115-impl-approve-panel-gate/spec.md`.
**Panel**: 9 slots — F1–F3 Fable, O1–O3 Opus, **G1–G3 codex**. **Pass = ≥8 APPROVE, no REJECT.** Findings never out-voted (a confirmed finding is fixed, not out-voted, even at 8/9 — as happened round 4).

**READ-ONLY**: verdict JSON only; pin reads to `e6dfa4cf` (read spec.md at HEAD); ALL scratch under ABSOLUTE `/tmp`; NEVER edit the worktree; leave `git status` clean.

**History**: R1 3/6 → R2 7/9 → R3 7/9 → R4 **8 APPROVE / 1 RC** (G2 codex, orchestrator-CONFIRMED: the branch-probe's exit-1 discriminator masked an unreadable ref store as "absent" = residual fail-open). This round (R5) is a NARROW verification of the ONE round-4 fix. Do NOT re-litigate the 4-round-validated design core or settled ACs — but DO flag any NEW defect the fix introduced.

## The single round-4 fix to verify (grade ADDRESSED / PARTIAL / MISSED / NEW_ISSUE)
Full round-4 brief + the orchestrator's empirical evidence table: `consolidated-round-4.md` (same dir).

**Round-4 finding (G2 0.98, orchestrator-reproduced)**: the spec grounded `BranchExistsE`'s "genuinely absent" on `git rev-parse --verify --quiet` exit 1 — but an EXISTING loose branch whose `.git/refs/heads` is unreadable ALSO returns `--quiet` exit 1 → `BranchExistsE` maps infra failure to `(false,nil)` → Scan sees no orphan → gate passes on a structural failure (residual fail-open). Orchestrator-tested exit codes (existing loose branch, store made unreadable via `chmod 000`):
| probe | genuine absent | existing + store UNREADABLE |
|:------|:---------------|:----------------------------|
| `rev-parse --verify --quiet` | 1 | **1** (masks) |
| `show-ref --verify --quiet` | 1 | **1** (masks) |
| `for-each-ref refs/heads/bead/` | 0 empty | **0 empty** (silently drops — worse) |
| **`show-ref --verify` (NO `--quiet`)** | **1** | **128** (loud — distinguishes) |

**The fix**: `BranchExistsE`'s contract is re-specified on **`git show-ref --verify refs/heads/<name>` WITHOUT `--quiet`** — the only probe whose exit code alone gives a clean three-way: **exit 0 → `(true,nil)` present; exit 1 → `(false,nil)` genuinely absent (no false-refusal); any other exit (128 unreadable store) / spawn failure → `(false, non-nil error)` → gate REFUSES.** Plus: a GENERAL PRINCIPLE (any ref-probe outcome not an unambiguous present(0)/absent(1) fails closed — no `--quiet`-masked or silently-empty probe on the gate path); a defense-in-depth note (`exec.MergeBase` at `impl.go:249` already fails closed on an unreadable ref store before the scan; `exec.FinalizeEpic` at `:372` too — so the ambiguity is only a physically-implausible transient-race tail); and a new test `TestBranchExistsE_UnreadableRefStore` (existing loose branch + unreadable store → non-nil ERROR) + AC1(b) pins the gate refusal for that outcome.

## What to verify at `e6dfa4cf` (spec.md)
1. **The fix is ADDRESSED** — R1's branch-probe leg, the Scope `internal/gitutil` bullet, and OQ2 all now specify `show-ref --verify` (NO `--quiet`) as the ACTIVE existence primitive with the 0/1/else three-way (NOT `rev-parse --verify --quiet`, which is explicitly WITHDRAWN for the existence probe / kept only as evidence). The general principle is stated. A fix marked done but not actually the active primitive = REQUEST_CHANGES.
2. **The three-way is empirically correct** — confirm YOURSELF (ABSOLUTE /tmp throwaway repo): `git show-ref --verify refs/heads/<existing>` exit 0; `<absent>` exit 1; existing-branch-with-`chmod 000 .git/refs/heads` exit 128 (loud). Does the contract (0→present, 1→absent, else→error→refuse) genuinely close BOTH the fail-open (infra masked as absent) AND avoid the false-refusal (genuine absence must be exit 1 → `(false,nil)`, NOT an error)? Any residual masking edge (G2 lens) or a NEW false-refusal = finding.
3. **Grounding** — `BranchExists` at `gitops.go:94-100` (plain `--verify`, `:98`); `exec.MergeBase` at `impl.go:249`; `exec.FinalizeEpic` at `:372` (defense-in-depth note); `RevParseRef`/`ErrRefNotFound` `gitops.go:13-19`/`:524-542` (now only the ancestry precedent). The diff `4cbf437b..f07cce6a` should be small (~5 lines) + only spec.md.
4. **Discriminators RED** — `TestBranchExistsE_UnreadableRefStore` (new), `TestBranchExistsE_ThreeWay`, `TestApproveImpl_BranchProbeErrorFailsClosed`, `BranchExistsE`, `ScanOrphanedClosedBeads` all 0 code hits at `e6dfa4cf`.
5. **No regression / OQ coherence** — core intact; all 4 OQs `[x]`; the fix doesn't contradict R2 advisory-fail-open or R5 no-regression.

## Per-slot lens
- **F1** falsifiability (new discriminator RED); **F2** grounding (show-ref exit codes cited accurately, MergeBase/FinalizeEpic lines); **F3** contradiction/OQ (three-way vs OQ2/R1/R5).
- **O1** anti-gaming (three-way closes the fail-open AND avoids false-refusal, no residual masking edge); **O2** ADR/ownership (gitutil seam unchanged-ownership); **O3** decomposition (still bead-sized).
- **G1** grounding empirical (verify the show-ref exit codes + the two impl.go lines + tests absent); **G2** anti-gaming (YOUR finding — CONFIRM the show-ref three-way genuinely closes the unreadable-store masking with no deeper edge; if you find yet another masking primitive/case, name it precisely); **G3** AC-runnability (the new test's exact-named proof + all discriminators still RED).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-5.json`: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence`, `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
