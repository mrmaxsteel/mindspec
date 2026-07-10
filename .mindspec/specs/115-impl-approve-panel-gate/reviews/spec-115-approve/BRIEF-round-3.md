# spec-115-approve — Round 3 Spec-Approval Panel (9 reviewers, three families)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-115-impl-approve-panel-gate`
**Branch**: `spec/115-impl-approve-panel-gate` @ **77d8a1a92e04b473d1b7a7beeee2a39b08545119**. Base = `origin/main` (`f02a3a49`, includes spec 114).
**Under review**: `.mindspec/specs/115-impl-approve-panel-gate/spec.md` (round-2-revised DRAFT) + `.mindspec/domains/{execution,workflow}/OWNERSHIP.yaml`.
**Panel**: 9 slots — F1–F3 Fable, O1–O3 Opus, **G1–G3 codex (real gpt-5.6-sol)**. **Pass = ≥8 APPROVE, no REJECT.** Every finding adjudicated (fixed or evidenced-refuted), never out-voted.

**History**: R1 = 3 APPROVE / 6 RC (design core validated). R2 @ `eb6a2ed1` = **7 APPROVE / 2 RC** — G2 codex (branch-probe fail-open) + G3 codex (AC-runnability). This round (R3) assesses whether the 6 must-fix items + O3's two nits are ADDRESSED at `77d8a1a9`.

**READ-ONLY RULE (HARDENED)**: verdict JSON only; pin reads to `77d8a1a9`; ALL scratch/build under ABSOLUTE `/tmp`; NEVER edit the shared worktree; leave `git status` clean.

## What this spec is
Spec 114 (on main) closed the silent panel-gate out-vote at `mindspec complete`. 115 closes the SAME class on `mindspec impl approve`: `ApproveImpl`→`FinalizeEpic` (`internal/executor/mindspec_executor.go:381-422`) auto-merges bead branches closed via raw `bd close`, skipping the gate. **Design (validated R1, unchanged): REFUSE, not RUN** — impl approve REFUSES (pre-terminal, no close/phase-write/merge/push) when any closed bead under the finalizing epic was closed via raw `bd close` (detected by `lifecycle.FindOrphanedClosedBeads`), recovery `mindspec complete <bead>` re-gates through the one gate home. ADR-0030: executor can't import `internal/complete` (cycle), so RUN = a second settlement surface (114's discharge saga proved that dangerous).

## Round-2 findings the R3 revision claims to fix — verify each is ADDRESSED (grade ADDRESSED / PARTIAL / MISSED / NEW_ISSUE)
Full round-2 brief: `consolidated-round-2.md` (same dir). The 6 must-fix + O3 nits:

1. **[G2] Branch-existence probe fails OPEN.** `gitutil.BranchExists(name) bool` (`gitops.go:94-100`, `return cmd.Run()==nil`) maps every git error to `false` → a branch-probe infra error looks like a deleted branch → gate passes → FinalizeEpic's worktree loop still merges the branch (o4fd re-opened via infra noise). → Verify the revision adds `gitutil.BranchExistsE(name)(bool,error)` as a NEW error-preserving seam beside the unchanged `BranchExists`; R1's error-preserving core (`ScanOrphanedClosedBeads`) lists the branch-existence probe as a 4th fail-CLOSED leg (with epic-lookup/bd-list/ancestry); the gate REFUSES on it; the fail-open wrapper keeps swallowing it (complete/next/doctor byte-identical). AC1(b) must gain `TestApproveImpl_BranchProbeErrorFailsClosed`. `internal/gitutil/**` should be added to Impacted Domains/Scope as execution-owned.
2. **[G3] Six AC proofs non-discriminating.** `go test -run '<nomatch>'` exits 0 with `[no tests to run]` (TRUE — verify yourself). → Verify AC1-AC7 each now chain a `grep -q 'func Test<Name>' <file>` existence discriminator with an exact-named `go test -run`, so each proof FAILS at `77d8a1a9` (the named tests don't exist yet). AC4's ordering discriminator must anchor on `ScanOrphanedClosedBeads` (0 hits repo-wide today = RED).
3. **[G3] AC7 import-cycle.** Grounding: `internal/approve` does NOT import `internal/complete` and vice versa (comment only, `complete.go:486`); R3 adds `approve→complete`, so an in-`package complete` test calling `ApproveImpl` would cycle. → Verify AC7 is SPLIT: (a) `TestCompleteRun_RegatesAlreadyClosedOrphan` package-local in `internal/complete`; (b) `TestApproveImpl_PassesAfterOrphanSettled` in `internal/approve`. No cross-package cycle; existence checks on both.
4. **[G3] Branchless-R3 recovery line untruthful.** `mindspec complete <bead>` errors at step-3.5 merge-base (`complete.go:492-495`) for a branch-less bead, before reconciliation. → Verify R3 gives the branch-LESS obligation refusal a DISTINCT recourse naming the restoration prerequisite (restore `bead/<id>` ref then complete, or settle out-of-band), separate from the with-branch message; AC6 asserts the prerequisite; ADR-0035 touchpoint updated. Branchless reconciliation stays deferred to `mindspec-h4n5`.
5. **[G3] Persist `mindspec-h4n5`.** h4n5 (OPEN P3) was absent from the tracked branch `.beads/issues.jsonl` at `eb6a2ed1`. → Verify `grep -c mindspec-h4n5 .beads/issues.jsonl` == 1 at `77d8a1a9` (a separate `chore(beads)` commit).
6. **[G3] AC10 "exactly one claimant".** → Verify AC10's proof enumerates `internal/lifecycle/**` across ALL `.mindspec/domains/*/OWNERSHIP.yaml` and asserts exactly one claimant == workflow (`grep -l ... == the single workflow path`), not just a two-file positive/negative.
7. **[O3 LOW] `cmd/**` over-declared + execution self-ownership gap.** → Verify `cmd/**` is tightened/dropped in Impacted Domains (no cmd/ file in scope), and the pre-existing execution self-ownership gap is evidence-refuted as out-of-scope pre-existing (not fixed by 115).

## What to verify (SPEC panel — at `77d8a1a9`)
1. **Each R2 finding ADDRESSED** — grade 1-7. A finding marked fixed but not actually fixed (or a discriminator that still passes trivially at `77d8a1a9`) = REQUEST_CHANGES.
2. **No new hole from the branch-probe fix** — does `BranchExistsE` + the 4th fail-closed leg genuinely close G2's residue without regressing the three fail-open consumers? Is there ANY remaining fail-open detection leg (G2 lens)? Is FinalizeEpic's worktree loop still the merge actor the gate must front-run?
3. **AC discriminators genuinely RED at this SHA** — spot-check that the named tests do NOT exist yet (`grep -q 'func Test<Name>'` exits 1) and the `ScanOrphanedClosedBeads` AC4 anchor has 0 code hits. Any AC still non-discriminating = finding.
4. **Grounding at 77d8a1a9** — every cited file:line exists + means what the spec says (BranchExists `gitops.go:94-100`; orphans.go:40/:95; the AC7 cycle grounding; complete.go:492-495/:576-579).
5. **OQ coherence + no contradiction** — all 4 OQs `[x]`, OQ2 asymmetry (advisory fail-open / gate fail-closed, now incl. the branch-probe leg) internally consistent; the truthful-recovery split doesn't contradict R4 convergence.
6. **Design core intact + scope** — REFUSE closes o4fd; recovery converges; ~2-3 beads; no over/under-scope introduced by the branch-probe seam.

## Per-slot lens defaults
- **F1** falsifiability (every AC discriminator genuinely RED at 77d8a1a9); **F2** grounding (claims ↔ real code); **F3** contradiction/scope/OQ coherence.
- **O1** REFUSE soundness + anti-gaming (branch-probe fix closes the residue, no new hole); **O2** ADR fitness (ADR-0035 truthful-recovery amendment; the gitutil seam ownership) + ownership; **O3** impacted-domains parity (gitutil→execution, cmd/** disposition) + decomposition.
- **G1** grounding empirical (verify cited lines + that named tests are absent today); **G2** anti-gaming (YOUR branch-probe finding — confirm `BranchExistsE` genuinely closes it and hunt for any OTHER fail-open leg or new residue); **G3** AC-runnability (YOUR finding — confirm every AC1-AC7 proof now FAILS at 77d8a1a9 via its discriminator; AC10 exactly-one; h4n5 persisted; branchless recovery truthful).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-3.json`: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence` (0–1), `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings` (per R2 item 1-7: ADDRESSED / PARTIAL / MISSED / NEW_ISSUE).
