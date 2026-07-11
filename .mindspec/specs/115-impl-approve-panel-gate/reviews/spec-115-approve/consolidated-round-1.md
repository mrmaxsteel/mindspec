# spec-115-approve round-1 consolidated findings (spec revision brief)

Round-1 spec-approval panel: **3 APPROVE (O1, O2, O3), 6 REQUEST_CHANGES (F1, F2, F3, G1, G2, G3)**. Below the ≥8 threshold → revise + re-panel. **The design CORE is validated** — all three Opus slots (incl. the anti-gaming end-to-end lens O1) confirmed REFUSE genuinely closes the o4fd bypass, recovery re-gates through `complete` (ordering traced), REFUSE-before-mutation is correct, and ADR-0030/R3 have no import cycle. The findings are refinements + AC-proof fixes + one genuine design reconciliation. Every finding is adjudicated below (fix or evidence-refute) — none out-voted.

## DESIGN / CORRECTNESS

### 1. [G2 0.99 + G3 0.99 + O1-F2] Fail-CLOSED the impl-approve gate consumer (reconcile the grill's OQ2)
The grill self-answered OQ2 as "fail-open parity" for `FindOrphanedClosedBeads`. Two codex slots (+ O1-F2's partial-Dolt case) argue: a **merge gate cannot fail open on "can't prove settlement."** An `bd`/list/ancestry/`GetMetadata` infra error on the impl-approve path must **REFUSE** (fail-closed), not silently allow the finalize. FIX: the shared predicate stays fail-open for its ADVISORY consumers (doctor/next), but the impl-approve consumer uses an **error-preserving variant** (or wraps the query) that refuses on any infra error — an unreadable store cannot prove the epic is settled. Update R1/R2 + OQ2's recorded self-answer to this asymmetry (advisory=fail-open, gate=fail-closed), with a falsifier + AC sub-case. **This overrides the grill's OQ2 — the panel is right that a gate needs fail-closed.**

### 2. [F3-1 major + G2 + G3] R3 durable-obligation enumeration must be fail-closed
R3 enumerates beads via `readPlanBeadIDs` (`impl.go:452`), and the adjacent gate silently skips on `planErr` (`impl.go:226`, `if planErr == nil`), so a corrupt/missing `plan.md` makes R3's UNIQUE coverage (merged/deleted-branch beads with a recorded obligation) silently vanish — fail-open on exactly the leg that must be fail-closed. FIX: pin a fail-closed posture — if the plan bead IDs cannot be read, REFUSE — OR enumerate the epic's lifecycle children authoritatively from beads (not plan.md). Add a falsifier + an AC6 sub-case (corrupt plan.md → refuse).

### 3. [G1 0.99] "mutating nothing" vs `EnsureMigrated` ordering
`ApproveImpl` runs `EnsureMigrated` (writes migration metadata) BEFORE the proposed refuse point, so the spec's absolute "mutating nothing on refusal" is false. FIX: EITHER move the impl-approve refusal BEFORE `EnsureMigrated` (and add a test that migration metadata is uncalled on refusal), OR narrow every "mutating nothing" claim to "performs no merge/close/push/phase-write" and correct Background's `complete.Run` ordering description (distinguish pre-terminal enforcement from literally-pre-mutation). Recommend: narrow the claim to "no merge/close/push" unless moving before EnsureMigrated is clean.

### 4. [G2] Scope the orphan detection to the finalizing epic (+ the global `bead/*` loop)
`FinalizeEpic`'s auto-merge loop iterates `bead/*` GLOBALLY (the `blp6` cross-spec-merge root cause). The REFUSE detection must scope its orphan candidates to the **finalizing epic's** beads, not global — else it could refuse on an unrelated spec's orphan or miss scope. ADJUDICATE: make the refuse detection epic-scoped (correctness, in-scope for 115). Whether to also fix the merge loop's global iteration itself (`blp6`, currently P2 deferred) — decide: if epic-scoped detection fully closes the refuse correctness, `blp6`'s merge-loop fix can stay deferred; if not, pull it in. State the decision explicitly.

## AC-PROOF DEFECTS (clear fixes)

### 5. [F1 + F2 major] Skill-doc target path does NOT exist
AC9 + In Scope name `plugins/mindspec/skills/ms-impl-approve/SKILL.md` — nonexistent. The four lifecycle-gate skills are **binary-embedded** via `lifecycleSkillFiles()` (canonical literal `internal/setup/claude.go:736`) and materialized to `.claude/skills/ms-impl-approve/SKILL.md` + `.agents/skills/...`. FIX: repoint Scope + AC9 to the embedded literal + materialized copies. AND: AC9's grep string `mindspec complete` is ALREADY present (line 9) → non-discriminating; use a **refusal-specific** string with zero hits today (e.g. `closed without` / `bd close`), so it genuinely falsifies the new doc.

### 6. [F2] AC8 grep can never match
`same regression-only` (`:397`) and `rule as CompleteBead` (`:398`) are on different lines, so `grep '^-.*same regression-only rule'` yields 0 in any diff. FIX: use a single-line-matchable target string for the AC8 proof.

### 7. [F1 minor] AC10 + AC7 proof precision
AC10's first grep is already green at the review SHA (the OWNERSHIP add is the artifact under review) — reword so it falsifies the DELIVERED behavior, not the pre-applied edit. AC7's `AlreadyClosed` pattern matches only the to-be-written test — tighten.

## OWNERSHIP + PROSE

### 8. [O2 medium] `internal/lifecycle` ownership → workflow, not execution
The grill added `internal/lifecycle/**` to the EXECUTION domain, but all four consumers (complete/doctor/next/approve) are WORKFLOW-owned and the package self-describes as workflow-lifecycle detectors; "composes execution plumbing" isn't the ownership criterion (workflow packages routinely import execution). FIX: reassign the OWNERSHIP add to **workflow** (recommended) OR give a genuinely execution-specific rationale. (Panel leans workflow.)

### 9. [O1-F1 medium] Goal overclaims vs disclosed residuals
The Goal's absolute phrasing overclaims vs two honestly-disclosed residual paths (a raw-`git merge`'d bead with an unresolved-but-never-refuted RC — no `refutation_pending` obligation — escapes R1/R3/the un-read panel; and cross-spec `blp6` beads on the global loop). FIX: soften the Goal to match Non-Goals + qualify R3's raw-git-merge coverage language.

### 10. [cosmetic] F3-2 OQ2 rationale ("Dolt hiccup wedge" — inoperative since R3 fail-closed wedges on the same outage; use shared-predicate-parity); F2 orphans.go end-bound off-by-3 (`:71-73` not `:69-72`); loose "before any mutation" sentence in Background.

## Disposition
Fix items 1-9 (each a real spec improvement); 10 cosmetic. The design core (REFUSE, recovery re-gates, no import cycle, epic-scoped fail-closed gate) stands — this is refinement, not a redesign. Then re-panel round 2 (9-slot). Two items warrant explicit decisions in the revision: #1 fail-closed-gate (override the grill's OQ2 — DO IT, the panel is right) and #4 global-loop scoping (epic-scope the detection; decide blp6-merge-loop in/deferred).
