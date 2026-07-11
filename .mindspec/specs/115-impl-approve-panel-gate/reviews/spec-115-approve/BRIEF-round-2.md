# spec-115-approve — Round 2 Spec-Approval Panel (9 reviewers, three families)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-115-impl-approve-panel-gate`
**Branch**: `spec/115-impl-approve-panel-gate` @ **eb6a2ed1e8596652bd569846c60cb2b6e1d80a72** (the revised draft). Base = `origin/main` (`f02a3a49`, which INCLUDES spec 114).
**Under review**: `.mindspec/specs/115-impl-approve-panel-gate/spec.md` (revised DRAFT, before `mindspec spec approve`) + the two OWNERSHIP.yaml edits (`internal/lifecycle/**` moved execution→workflow).
**Panel**: 9 slots — F1–F3 Fable, O1–O3 Opus, **G1–G3 codex (real gpt-5.6-sol)**. **Pass = ≥8 APPROVE, no REJECT.** Every finding adjudicated (fixed or evidenced-refuted), never out-voted.
**Prior round**: round 1 = **3 APPROVE (O1/O2/O3) / 6 REQUEST_CHANGES (F1/F2/F3/G1/G2/G3)**. The design CORE (REFUSE, recovery re-gates via `complete`, no import cycle, epic-scoped fail-closed gate) was validated by all three Opus slots. This round assesses whether the 10 consolidated findings + the 4gsz reconciliation are ADDRESSED.

**READ-ONLY RULE (HARDENED)**: verdict JSON only; pin reads to `eb6a2ed1`; ALL scratch/build under ABSOLUTE `/tmp`; NEVER edit the shared worktree; leave `git status` clean. Verdicts ONLY to the exact absolute path at the bottom.

## What this spec is (unchanged from round 1)
Spec 114 (shipped to main) closed the silent panel-gate out-vote at `mindspec complete`. The 114 final review found the SAME class open on a different verb: `mindspec impl approve` → `ApproveImpl` → `FinalizeEpic` (`internal/executor/mindspec_executor.go:381-422`) auto-merges bead branches closed via raw `bd close`, skipping the panel gate. `ApproveImpl`'s bead gate (`internal/approve/impl.go:220-242`) checks only `status=="closed"` (which raw `bd close` satisfies). Filed `mindspec-o4fd` (P1); 115 closes it.

**Design (validated round 1): REFUSE, not RUN.** `impl approve` REFUSES (pre-terminal, mutating no close/phase-write/merge/push) when any closed bead under the finalizing epic was closed via raw `bd close` (detected by `lifecycle.FindOrphanedClosedBeads` — branch-exists + not-ancestor), naming the bead with recovery `mindspec complete <bead>`. Recovery converges through the ONE gate home: `complete.Run` re-runs the panel gate + `reconcilePendingRefutations` for an already-closed bead before its close-tolerance, then merges. ADR-0030: the executor can't import `internal/complete` (import cycle), so RUN would be a second settlement surface — exactly what 114's discharge saga proved dangerous.

## Round-1 consolidated findings — verify each is ADDRESSED (the revision's job)
The full round-1 brief is `consolidated-round-1.md` (same dir). The 10 items + the 4gsz reconciliation:

**DESIGN / CORRECTNESS**
1. **Fail-CLOSED the impl-approve gate consumer** (overrides grill OQ2). Advisory consumers (complete/next/doctor) stay fail-OPEN; the impl-approve GATE uses an error-preserving core and REFUSES on any bd/list/ancestry/GetMetadata infra error. → Verify R1 specs an error-preserving `internal/lifecycle` core + fail-open wrapper parity; OQ2 rewritten to the asymmetry; AC1(b) + falsifier cover the infra-error refusal.
2. **R3 enumeration fail-CLOSED** on unreadable `plan.md` (was `impl.go:226-227` silent skip). → Verify R3 refuses on unreadable plan bead IDs; AC6(e) covers corrupt plan.md.
3. **"mutating nothing" vs `EnsureMigrated` ordering** (EnsureMigrated at `impl.go:140` writes before the refuse point). → Verify the claim is narrowed to "no close/phase-write/merge/push" with EnsureMigrated explicitly carved out; Background's `complete.Run` ordering corrected.
4. **Epic-scope the orphan detection** (FinalizeEpic's `bead/*` loop is global — the blp6 root). DECISION: epic-scoped detection closes 115's refuse correctness; blp6 merge-loop fix stays DEFERRED P2. → Verify R1 states epic-scoping (orphans.go:75-83), AC2(c) proves a different-spec orphan does not refuse, and the blp6 deferral is explicit.

**AC-PROOF DEFECTS**
5. **Skill-doc path was wrong** (`plugins/mindspec/skills/...` nonexistent). → Verify Scope + AC9 repoint to the binary-embedded literal (`internal/setup/claude.go:681`/`:736`) + materialized `.claude/skills/` + `.agents/skills/` copies, and AC9's grep uses a refusal-specific string (`closed without`, 0 hits today) not the non-discriminating `mindspec complete`.
6. **AC8 grep couldn't match** (string spanned two lines). → Verify AC8 uses a single-line target (`rule as CompleteBead`, line :398, 1 occurrence).
7. **AC10/AC7 proof precision.** → AC10 now has an execution-file NEGATIVE discriminator (FALSE at round-1 SHA); AC7 names `TestRecoveryConvergence_AlreadyClosedOrphan` with an existence check.
8. **`internal/lifecycle` ownership → workflow** not execution. → Verify the path is REMOVED from `.mindspec/domains/execution/OWNERSHIP.yaml` and ADDED to `.mindspec/domains/workflow/OWNERSHIP.yaml` (exactly one claimant); Impacted Domains + AC10 point at workflow.
9. **Soften the Goal** vs disclosed residuals (raw-git-merge'd never-refuted-RC bead; cross-spec blp6). → Verify the Goal discloses both residuals and R3's coverage is qualified.
10. **Cosmetic** — OQ2 rationale (drop inoperative Dolt-hiccup framing), orphans.go bound, loose Background sentence.

**4gsz reconciliation** — verify Background frames 115 as the impl-approve leg of `mindspec-4gsz`'s detect-and-block floor (shared `FindOrphanedClosedBeads` predicate; complete/next/doctor legs shipped under 4gsz; CI leg stays 4gsz scope), without pulling 4gsz's other legs into 115.

## What to verify (SPEC panel — design soundness + falsifiable grounding at the REVISED SHA)
1. **Every round-1 finding ADDRESSED** — grade each 1–10 + 4gsz as ADDRESSED / PARTIAL / MISSED / NEW_ISSUE. A finding marked fixed but not actually fixed = REQUEST_CHANGES.
2. **Design core still sound** — REFUSE closes o4fd; recovery genuinely converges through `complete`; REFUSE-before-mutation correct; no import cycle (ADR-0030). Did any revision WEAKEN the core (e.g. the fail-closed refactor introducing a new bypass, or breaking the three existing consumers' parity)?
3. **Grounding at eb6a2ed1** — every cited file:line exists + means what the spec says. The revision re-grounded several refs (orphans.go:74-112; `if planErr==nil` at :227; EnsureMigrated at :140; the FinalizeEpic loop :381-422; comment :396-398; claude.go:681/:736). Spot-check the load-bearing ones. A claim the tree doesn't support = REJECT-worthy.
4. **Falsifiability** — each of the 10 ACs pairs a runnable proof (`go test`/`grep`/`git diff`) with a falsifiable assertion. The infra-error fail-closed sub-case (AC1b), corrupt-plan refuse (AC6e), epic-scope (AC2c), and the AC8/AC9/AC10 discriminators must genuinely falsify. Any AC that passes trivially at the current SHA (non-discriminating) = finding.
5. **OQ coherence** — all 4 OQs `[x]` with internally-consistent self-answers; OQ2 now = advisory-fail-open / gate-fail-closed asymmetry (does it contradict R1/R2/R3? R2 advisory stays fail-open — is that consistent with R1/R3 fail-closed on the gate path?). No synonym-dodge, no unfalsifiable prose, no contradiction.
6. **Scope/decomposition** — still ~2-3 beads (refuse gate incl. lifecycle refactor + R3 backstop; docs/ADR/skill; tests)? Anything newly over- or under-scoped by the revision?

## Per-slot lens defaults
- **F1** falsifiability (every AC a real pass/fail proof, esp. the new sub-cases); **F2** grounding (claims ↔ real code at eb6a2ed1); **F3** contradiction/scope/thinness + OQ coherence (esp. OQ2 asymmetry vs R1/R2/R3).
- **O1** REFUSE design soundness + anti-gaming completeness (does the fail-closed refactor close o4fd with no new hole, and not weaken the existing consumers?); **O2** ADR fitness (ADR-0037 amendment reach; ADR-0030 import-cycle; the ownership move to workflow — correct + minimal?); **O3** impacted-domains parity + decomposition.
- **G1** grounding empirical (verify the re-grounded lines + test surfaces exist at eb6a2ed1); **G2** anti-gaming feasibility (trace REFUSE + R3 backstop + the fail-closed infra posture + hatches for ANY un-gated-merge / false-refusal / fail-open residue — you found o4fd; hunt for its residue and for any regression the lifecycle refactor introduces); **G3** completeness/decomposition (bead-sized; every AC runnable; anything the revision missed).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-2.json`: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence` (0–1), `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings` (per round-1 item: ADDRESSED / PARTIAL / MISSED / NEW_ISSUE).
