# spec-115-final — Round 2 consolidated tally — PASS 12/12

**Reviewed**: spec branch `389fcc41` (round-1 `21374bb8` + the boundary-fix reroute) vs `origin/main` `f02a3a49`. **Panel**: 12 slots (F1-F3 Fable-sub, O1-O3 Opus, S1-S3 Sonnet, G1-G3 codex). **Threshold**: ≥11.

## Verdicts — 12 APPROVE / 0 REQUEST_CHANGES / 0 REJECT
F1 A · F2 A · F3 A · O1 A · O2 A · O3 A · S1 A · S2 A · S3 A · G1 A (0.99) · G2 A (0.99) · G3 A (0.99). **UNANIMOUS PASS.**

## The two-round arc
- **Round 1 (9/3):** the gate's BEHAVIOR cleared every lens (G1/G2 codex security + fail-closed; O1 o4fd guarantee; F1 all-AC RED-on-revert; grounding/ADR/provenance/no-regression). Three RC: S1+S3 found a REAL, deterministic CI blocker — `internal/lint.TestEnforcementHasNoGitLeaks` failed because Bead 2's direct `internal/gitutil` import in the ADR-0030 enforcement package `internal/approve` violated the boundary lint (the spec's O2 "direct edge" decision rested on a premise the lint falsified); G3 flagged pre-existing gosec G115 (`8ud6`). S1 independently reproduced the boundary failure in an isolated /tmp clone (genuinely CI-breaking, not worktree noise). O2/F3 had approved round 1 because the BRIEF's touched-package shortlist excluded `internal/lint` — LESSON: the final review must run the WHOLE suite.
- **The fix (`389fcc41`):** rerouted approve's two read-only calls (`IsAncestor`, `BranchExists`) through thin `internal/lifecycle` wrappers (`gitquery.go`); dropped approve's direct gitutil import. Seam-transparent (identical signatures — no test change, gate behavior byte-identical), honors the boundary rather than adding a `boundary-allowlisted:` exception. Whole-tree re-verified: `TestEnforcementHasNoGitLeaks` PASSES, approve→gitutil gone, no import cycle, no NEW CI failure.
- **Round 2 (12/12):** every slot confirmed the reroute is seam-transparent + the boundary honored; the two CI raisers (S1/S3) and G3 flipped. The pre-existing conditions were correctly scoped out.

## Pre-existing conditions (out of scope for spec 115 — NOT blocking)
- `internal/journal/lock_unix.go` gosec G115 ×2 — bead `mindspec-8ud6` (P3). On origin/main, untouched by 115, CI-tolerated (main's CI green).
- `internal/instruct.TestRun_IdleNoBeads` — bead `z4ps`. Worktree-context-only; passes in a clean CI clone.
- `internal/harness` LLM-timeout — only without `-short`; CI uses `-short`.

## Non-blocking notes carried
- F2 round-1: some `panel_advisory.go` line ranges in spec.md drifted 50-150 lines (R8 hardening inserted validation earlier) — line-staleness, all symbols resolve. (Cosmetic; spec.md is historical record.)
- O1/O3/F3 round-1 INFO notes (all non-blocking).

## Decision
**PASS 12/12 → `mindspec impl approve 115-impl-approve-panel-gate`** (protected-main flow: finalize+push spec branch → PR → CI → merge to main). Whole-spec doc-sync via `--allow-doc-skew` (repo has no `.mindspec/docs/domains/`; contract docs = ADR-0037 + skill). HELD: v0.11.0 release (not part of impl-approve).
