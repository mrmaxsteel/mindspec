# spec-128-plan-approve — consolidated concrete_changes_required, round 1

Tally: 1 APPROVE (R4, zero asks) / 5 REQUEST_CHANGES. 17 asks deduped into 6 groups. Artifact-gate screen: clear.

## G1 — The OQ-1(a) prospective-snapshot protocol is the plan's central unfinished design (R2 + R5 ×3 + R6 ×2 — strongest convergence) — MUST FIX
The "worktree-inclusive gate measurement" is asserted, not designed. Required, all faces:
1. **Exact mechanism, named in the plan** (R2/R5): must include committed-tip delta + staged + unstaged + deletions + UNTRACKED files. Today `CommitAll`→commitWithExport runs `git add -A` (commits untracked) while every working-tree diff primitive (`gitutil.DiffNameOnlyRef`, `ValidateDocsRange` head=="" path) is `git diff` — blind to untracked. Admissible designs named by reviewers: temporary-index/tree-object snapshot committed from the measured tree; or `git status --porcelain` union / `git add --intent-to-add` pre-diff — pick one, state it applies to BOTH ValidateDocsRange and CheckADRDivergence variants.
2. **Exactly-the-measured-tree guarantee** (R5/R6): no bd export, commit hook, or intervening worktree change may add content after validation without forcing revalidation/refusal — the committed source-relevant tree equals the measured snapshot or the command refuses before advancing the tip. Specify the boundary mechanism.
3. **Executor grounding** (R6): `MindspecExecutor.ChangedFiles` and ref readers are rooted at `g.Root`, not wtPath — specify the interface/implementation changes to diff base against the bead worktree AND read OWNERSHIP/domain/config inputs from the same candidate tree; add those files to bead 6's scope; test that invoking complete from main still measures the bead worktree.
4. **ADR-0041 §1 amendment as an explicit replacement of BOTH load-bearing statements** (R5): retain panel-before-CommitAll + its reviewed-SHA rationale; replace gates-after-materialization with prospective-snapshot-before-any-complete-authored-commit; state artifact-only normalization treatment and why the gates cannot vacuously miss content; remove the unsupported measured-content-equals-committed claim (or make it mechanically true).
5. **Red-on-revert fixture legs** (R5/R6): untracked source/doc/ADR file (gate refuses pre-commit); pre-commit hook adding/changing a source file; tracker export changing content post-measurement; worktree mutation in the gate-to-commit interval.

## G2 (R6) — Upgrade reconciliation for legacy epics under option-1 semantics — MUST FIX
Existing epics carry stored phase from the OLD semantics (esp. stored=implement, all lifecycle children open, none ever claimed — indistinguishable today from a released/between-beads state). Add an explicit idempotent reconciliation: durable discriminator between the legacy window and legitimate states, the invoking lifecycle/read paths, lock-protected write, and fixtures for upgrade-before-first-claim, upgrade-then-first-claim, non-lifecycle claim, release, and already-progressed epics.

## G3 (R6) — Phase-lock operational completeness — MUST FIX
Choose + gitignore the exact lock path; bounded wait/cancellation with an ADR-0035 recovery line on contention; crash/stale-file handling for the Windows O_EXCL path; tests for timeout/error propagation, crashed-holder recovery, stale-file behavior, and lock acquisition leaving no blockable git dirt. Not just Unix flock contention tests.

## G4 (R3 ×3) — work_chunks / seam attribution corrections — MUST FIX
1. work_chunks id 1: add the six lock-wrapped writer files to key_file_paths (or in-plan rationale for lock-machinery-only, acknowledging plan.go:602-623 seeds bead metadata from the list).
2. Pin the sanctioned-entry seam: which bead creates the non-exported guard-bypassing lock-helper entry release routes through, and where the backward comparator lives — or correct bead 1/2 attribution and the OQ-3 wording that overstates bead 1.
3. Correct the Decomposition Note's sharing sentence to the policy actually applied (same-region overlap → serializing edge; distinct-region overlap → serial cycle), and either add a bead-8 step naming its beads.go change or drop beads.go from chunk 8's key_file_paths.

## G5 (R1 ×5) — Delegated dispositions the plan left unrecorded — MUST FIX (mostly small)
1. R2.4: add the mandated #245 disposition (HEAD re-reproduction result of the exact 0.12.0 shape, or explicit fixed-by-092 naming AC-14's ordering test as regression guard).
2. AC-5(iv): state what the retained warning's final recovery line becomes where still reachable outside the dissolved windows (an inspection action per the AC's letter, possibly direction-conditional) — or justify the bead-1-guarded `repair phase` line against the AC's "inspection action, never the backward write" wording.
3. R3.5: record taken / not-taken for the phase.Cache noise reduction.
4. Bead 10 step 2 + OQ-1 rationale: scope to the GATE-REFUSAL class — "(a) closes the entire class" proves too much; a post-gate failure (merge conflict / bd-close failure after CommitAll) still advances the tip and stale-wedges the retry, same as HEAD. The doc text must not inherit the overclaim.
5. OQ-3 wording: "both become TRUE three-surface agreements" contradicts the release row's two-surface carve-out — fix the sentence.

## G6 (R5) — ADR-0025 amendment scope fence — MUST FIX (one sentence)
Keep the amendment expressly limited to the worktree-creating `mindspec next` claim path; `approve` and other dirty-tree guards retain existing semantics unless separately amended.
