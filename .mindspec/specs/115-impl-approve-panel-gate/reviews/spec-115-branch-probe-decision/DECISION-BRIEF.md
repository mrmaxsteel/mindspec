# spec-115 branch-probe DESIGN DECISION panel (5 slots)

**Not a normal review — a DESIGN-DECISION panel.** Spec 115's `impl approve` gate detects raw-`bd close`'d orphan beads via a per-branch existence probe. Across spec-approval rounds 2→5, the codex empirical lens found a deeper fail-open/false-refusal edge in that probe EACH round, and round 5 (9/9 RC) PROVED that no single git ref-probe distinguishes "genuinely absent" from "ref-store unreadable" by exit code alone. This panel PICKS how to resolve it. Your verdict directs the round-6 revision.

## Read first
- The full evidence + the four candidate approaches + the orchestrator recommendation: `../spec-115-approve/consolidated-round-5.md` (READ THIS).
- The spec under revision: `../../spec.md` (the branch-probe leg is in Requirement 1).
- Context: `../spec-115-approve/consolidated-round-4.md` (the round-4 finding), `G1-round-5.json`/`G2-round-5.json` (the codex reproductions).

## The proven facts (do not re-litigate — verify if you wish, ABSOLUTE /tmp)
1. `show-ref --verify` (no `--quiet`): present=0, absent=**128**, unreadable-store=**128** (absent collides with infra → false-refuses every deleted branch).
2. `show-ref --verify --quiet` / `rev-parse --verify --quiet`: present=0, absent=1, unreadable-store=**1** (infra masked as absent → fail-open).
3. `for-each-ref`: exits 0 empty on an unreadable store (silently drops).
4. `git show-ref --exists` (git ≥2.43): present=0, absent=2, error=1 — a GENUINE three-way, but needs a git-version floor (currently undocumented in this project).
5. **The fail-open is not a real un-gated-merge hole**: `MergeBase("main",specBranch)` at `impl.go:249` runs BEFORE the scan and fails-closed (exit 128) on a whole-store-unreadable `refs/heads`; for a single corrupt bead ref (store otherwise fine), FinalizeEpic's own merge of that ref fails-closed (exit 1) and/or the corrupt orphan has no worktree so FinalizeEpic never merges it. Every infra sub-case the probe could misclassify is independently fail-closed downstream.

## The decision — pick ONE (A / B / C / D), with reasoning
- **A — `git show-ref --exists`** (genuine 0/2/1 three-way; requires git ≥2.43 floor + CI enforcement + older-git handling).
- **B — `--quiet` probe + explicit store-health precheck** (version-agnostic; fail-closed at the gate via a store-health signal, e.g. reuse `MergeBase@:249` or an explicit health probe; a second probe = more surface).
- **C — revert probe to simple `--quiet`/bool + AUDITED REFUTATION** (spec-114 "delete the classifier" move; version-agnostic, minimal; relies on the fact-5 refutation holding — the merge is structurally fail-closed on exactly the refs the probe could mask).
- **D — worktree-enumeration reframe** (scan uses the same `git worktree list` FinalizeEpic merges from; robust by construction; diverges from the shared `FindOrphanedClosedBeads` predicate that complete/next/doctor use = bigger blast radius).

Orchestrator recommendation: **C (or a C/B synthesis)** — stop classifying per-ref exit codes; use `--quiet` (never false-refuses); rest the fail-closed posture on the store-health signal that already exists (`MergeBase@:249`) + the audited refutation for the single-corrupt-ref residual. A is the clean alternative IF a git≥2.43 floor is acceptable.

## Your job
Pick the approach you judge best and justify it against: (1) does it CLOSE the real risk (un-gated merge of an unsettled orphan) with NO false-refusal on the normal deleted-branch happy path? (2) is it robust against the spiral (no deeper exit-code edge)? (3) blast radius / simplicity / version-portability? (4) for C specifically — does the fact-5 refutation actually hold, or can you construct an un-gated-merge path it misses? (5) does it fit the spec's structure (the shared predicate, the fail-open wrapper for complete/next/doctor, ADR-0030)?

## Per-slot lens
- **O1 (opus)** — anti-gaming: which approach most robustly closes the un-gated-merge risk with no false-refusal; if you pick C, stress-test the refutation.
- **O2 (opus)** — architecture/ADR fit + blast radius: shared-predicate impact, ADR-0030, project-portability of a git floor (A).
- **F1 (fable)** — spec-coherence: which fits the spec's existing structure + ACs with least disruption; falsifiability of the resulting ACs.
- **G1 (codex)** — empirical: verify the exit-code facts; for A, confirm `--exists` codes + the git-version floor reality; pick the approach whose tests are genuinely implementable against real git.
- **G2 (codex)** — adversarial: you found the probe edges every round. Try HARDEST to break approach C's fact-5 refutation (find an un-gated-merge path via a masked probe that MergeBase@:249 and FinalizeEpic-merge-failure both miss). If you cannot, say so and endorse; if you can, name the exact reproduction and pick a different approach.

## Output
Write JSON to `<this-dir>/<slot>-decision.json`: `reviewer_id`, `chosen_approach` (one of "A"/"B"/"C"/"D" — or a named synthesis like "C+B"), `confidence` (0–1), `rationale` (≤250 words), `rejected` (one line each on why the others lose), and for G2 a `refutation_holds` boolean + any `break_repro`. (Codex: WRITE the JSON to the exact path with a Write tool call as your final step; if you cannot, write an error there.)
