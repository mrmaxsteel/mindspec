# spec-128-bead7 — consolidated concrete_changes_required, round 2

Tally: 3 APPROVE (R2/R4/R5) / 3 REQUEST_CHANGES (R1 ×2, R3 ×3, R6 ×4 with hard_block). 9 asks in 4 groups — R3 and R6 independently converged on the same two mechanism defects. Artifact-gate screen: clear.

## G1 — Replace lexical samePath gating with canonical-toplevel CONTAINMENT (R6 asks 1-2 + R3 G-r2-2, convergent; drives the hard block) — MUST FIX
samePath(anchor, cwd) is lexical and cwd-equality-based: (a) invoking from a SUBDIRECTORY of the anchor tree disarms every class (samePath false) while porcelain is toplevel-relative regardless — the guard observes the dirt but the classes are off; (b) a symlink alias of the tree evades it. Fix: derive the invoking git working tree's CANONICAL top-level (rev-parse --show-toplevel + filesystem identity/EvalSymlinks), treat invocation from any subdir or symlink alias as that tree, and include a write class iff its RESOLVED target is CONTAINED by that observed top-level. Also fix filepath.Rel bases to the toplevel, not cwd. Regressions: real-claim from a subdirectory and through a symlink alias, dirty entry-absent .gitignore — both refuse before claim, bytes/branches/worktrees/symlink targets unchanged.

## G2 — Custom worktree_root recording divergence (R3 G-r2-1 + R6 ask 3, convergent) — MUST FIX
workspace.SpecDir's worktree probes join DefaultWorktreesDir (hardcoded .worktrees, nil cfg) while claimAnchorRoot honors cfg.WorktreeRoot — with a custom worktree_root + recording enabled + spec worktree present, the two resolve DIFFERENT trees and the recording class mis-anchors. Fix: derive recording entanglement from the ACTUAL resolved EventsPath/ManifestPath targets (the same resolution recording.EmitBeadMarker uses), not a parallel reimplementation. Pin: recording-enabled custom-root fixture where dirty recording files cannot be waved through then RMW'd.

## G3 (R1 ask 1) — ADR-0025 amendment must match the shipped rule — MUST FIX
The amendment still narrates the round-1 "derived enumeration exactly (a)(b)(c), EMPTY today" + "explicit list (empty beyond the exempt artifact)" story. Rewrite to the shipped mechanism: conditional per-spec classes, resolved-target containment in the observed tree, the anti-drift pin's real shape.

## G4 (R1 ask 2 + R3 G-r2-3 + R6 ask 4, three-slot convergent) — Fix ComputeEntangleablePaths' contradictory header comments — MUST FIX
Lines ~94-116 still assert the first-draft story (recording "ALWAYS under repoRoot", "does NOT route through resolveAnchorRoot", gates on "repoRoot == cwd") — all false post-fix. One honest description of the single target-resolution rule.
