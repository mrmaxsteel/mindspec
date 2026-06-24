# 106-bead4 — Round 1 Review Panel

**Bead**: `mindspec-3d3i.4` — Bead 4 (Phase 3 wiring: Reqs 8, 9, 10 + layout-aware panel gate). **Branch**: `bead/mindspec-3d3i.4`.
**Commit**: `d7f3fa774f8d30c0797e55c0e7d0e82f80ac213a` (single, 16 files, +986/-22, ZERO renames/copies/deletes).
**Bead worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-106-layout-flatten/.worktrees/worktree-mindspec-3d3i.4`
**Diff**: `git -C <bead-worktree> show d7f3fa77`. **Plan/spec**: `<bead-worktree>/.mindspec/docs/specs/106-layout-flatten/` (plan Bead 4 = Reqs 8, 9, 10).

## What the bead does (code wiring; NO file moves)
- **Req 8 — doctor:** new `checkLayout` (reuses `workspace.DetectLayout`): reports layout, `would-migrate-layout` Warn on canonical/legacy, `dual-layout-spec:<id>` ERROR when a spec id exists under two tiers; `checkDryRunMigration` made tier-aware (`workspace.SpecsDir`).
- **Req 9 — directional merge guard** (`internal/executor/layout_guard.go`): `layoutAtRef` fingerprints a ref via `TreeDirsAtRef(ref,".mindspec")` → `workspace.ClassifyLayout`; `mergeLayoutRegression` blocks ⟺ **source canonical/legacy AND target flat** (the regression direction), with a rebase recovery line; ALLOWS the migration direction (flat→canonical) + same-layout; EXEMPT under `workspace.MigrationRecoveryActive` (live run only); fails OPEN on read errors. Installed at the 3 real seams in `mindspec_executor.go`: `CompleteBead` `MergeInto`, `FinalizeEpic` auto-merge `MergeInto`, `FinalizeEpic` direct `MergeBranch`. Remote-PR path non-coverage documented.
- **Req 10/8 — layout-aware TRANSITIONAL panel gate:** `panel.Scan` globs BOTH `review/<slug>` and `reviews/<slug>` segments; `complete`'s `panelGateRoots`/`specScopedReviewRoots` choose roots by `DetectLayout` — canonical/legacy → repo-root `review/` UNION co-located `<spec-dir>/reviews/`; flat → co-located only.

## ‼️ TWO CRITICAL PROPERTIES TO VERIFY (if either is wrong, the rest of THIS spec breaks)
1. **Root-`review/` MUST keep driving the gate on a CANONICAL tree.** This repo is still canonical; this spec's own remaining beads (5, 6) are reviewed with panels at repo-root `review/106-bead5/`, `review/106-bead6/`. If Bead 4's gate stops honoring root `review/` on canonical, you CANNOT `mindspec complete` the rest of the spec. Verify the canonical branch unions root `review/`.
2. **The directional guard MUST NOT block Bead 5's own move-merge.** Bead 5 runs the mover in its worktree (→ flat), then `complete` merges `bead/…5` (flat) into the spec branch (canonical). That is the ALLOWED migration direction (flat source → canonical target). Verify `guardMergeLayout` allows it (and the eventual flat-spec→canonical-main), and only blocks the regression (canonical→flat).

## Fix-author deviations — ASSESS
A. **Touched `internal/workspace/workspace.go` + core/architecture.md** (beyond Bead-4's listed files): a 3-line exported `MigrationRecoveryActive` wrapper to REUSE Bead-1's in-flight-run scoping (plan said reuse, not reimplement). OK?
B. **Only `checkDryRunMigration` made tier-aware in doctor, NOT `docs.go` orphan/docs scans** — the plan (Bead 4 step 3) assigns those exact lines to Bead 2 (not yet merged; touching them would conflict). Correct deferral, or a gap?

## Your job
Review per your lens. Scope: no Bead 2 (gate matchers/classifier), Bead 5 (moves), or Bead 6 (skills/governance) work. Verify the 2 critical properties. The pre-existing `internal/instruct/TestRun_IdleNoBeads` (z4ps) flake is unrelated. Verdict: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `/Users/Max/replit/mindspec/review/106-bead4/<your-slot>-round-1.json`: `reviewer_id`, `verdict`, `confidence` (0-1), `rationale` (≤200 words), `concrete_changes_required` (array), `findings` (array of {severity, area, issue, hard_block?}).
