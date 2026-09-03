<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-bead8`
- **Round**: 3
- **Branch**: `bead/mindspec-ta30.8`
- **Reviewed commit**: `e9acc6344b0fae246c5024224c54b5ec1e5ac0a6`

## Your job

Review the diff at the reviewed commit above and write your verdict as
a single JSON file named `<your-slot>-round-3.json` beside this BRIEF
(the panel artifact schema — see `.mindspec/domains/workflow/interfaces.md`
§ Panel Artifact Schema). Required and optional top-level fields:

- `verdict` (required): one of `APPROVE`, `REQUEST_CHANGES`, `REJECT`.
- `hard_block` (optional boolean, a top-level sibling of `verdict` — never
  a per-finding field): set `true` only for an evidence-bearing halt that
  no vote count may override.
- `reviewer_id`, `confidence`, `rationale`, `concrete_changes_required`,
  `findings`: reviewer-authored context, read presentation-only by
  `mindspec panel tally` — not consumed by the gate decision.
<!-- /mindspec:panel-header -->

## Summary

**Bead implementation review** (gate: bead) — the spec's LAST code bead. Under review: commit `7f4ad41b` on `bead/mindspec-ta30.8` — spec 128 Bead 8: deterministic selection (OQ-5, AC-9/AC-10/AC-10b/AC-15) + the FOLDED-IN ta30.9 render-leg follow-up (this bead owns next.go post-b7). Delivered: `ReadySet`/`BuildReadySet` — ONE shared query+filter+exclusion-accounting construction under both the claim path and --emit-only (AC-9 parity); SelectWork total pick semantics (0=default row 1 with the pinned line "Defaulting to item 1 (bd ready priority order). Use --pick=N or a bead ID to select a specific item."; negative rejected; single-item out-of-range bypass REMOVED); SelectNamedFromReadySet + honest "excluded by spec filter" reason (never the false not-found); scopeResolutionRefusal distinguishing bd-failure / draft-no-epic (steers "run mindspec spec approve <id> first") / nonexistent — never silent global fallback; loud empty-after-filter error (AC-10b); ResolveSpecPrefixProven (NEW SIBLING — see disclosure) proving full-ID passthroughs name a real spec before any ready query; positional --emit-only bypass line "emit-only: bead fetched by ID; readiness not checked."; readinessOverrideFailure + describePhaseSyncFailure routed through next.RenderBDError (closes the ta30.9 deferral). Extensive new fixtures incl. real-bd CLI-level AC-15 legs.

**Disclosures (assess both)**:
- **A (design choice)**: the existence proof landed as NEW `ResolveSpecPrefixProven` rather than changing ResolveSpecPrefix's signature — the existing function has callers in internal/complete + cmd/mindspec/complete.go outside this bead's scope. Reuses ResolveSpecPrefixWithCache + draftSpecStatusWithCache internally. Judge: is the unproven original still reachable on any path that needed the proof?
- **B**: Step 1.7 + dirty-tree guard still run BEFORE the new scope-resolution refusal (b7's structure grafted onto, not reordered, per the prompt's binding instruction). Judge the ordering's user-facing consequences.

b9 pins confirmed green unmodified. Gates: -short 48/48; no-short cmd/mindspec ok 1301.4s; -race ×3 ok.

**Schema note (GH #254)**: header citation DANGLING; header field list authoritative; verdict exactly APPROVE / REQUEST_CHANGES / REJECT.

## Files in Scope

`git show 7f4ad41b` in the bead worktree. Key: internal/next/{select.go,beads.go,readyset_test.go,ready_gate_test.go}, cmd/mindspec/{next.go,next_render_test.go,next_selection_test.go}, internal/resolve/{target.go,target_test.go}.

## Acceptance Criteria (verbatim from spec.md)

AC-9, AC-10 (+10b), AC-15 — read in place; OQ-5's displayed-row-1 resolution binding.

## Prior-Round Asks

Round 1: 2A/4RC → 8193ca97. Round 2: **5A / 1RC (R1's residual: the exclusion notice's PRINT sites unpinned at CLI level)** → fixed as ONE commit `e9acc634`: two CLI-level partial-exclusion tests (claim + --emit-only paths, mixed-eligibility fixtures), with the implementer HAND-VERIFYING red-on-revert (stubbed both Fprintln sites → both tests failed → clean revert confirmed by empty diff → real gates). All gates green (no-short 1522.2s).

Round-3 duty (minimal confirm): R1 classifies its residual re-deriving (run both new tests; optionally repeat the stub check); others verify diff confinement (one test file) + confirm standing verdicts; re-verdict at e9acc634.

## Reviewer conduct (binding)

Build/test only in the bead worktree (/Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.8); leave byte-identical; fixture roots + probes in /tmp, CLEANED; no commits/lifecycle verbs/tracked-file edits; NEVER claim in this repo; verify-never-confirm; contradiction licensed; write your verdict JSON to the exact path in your prompt BEFORE terminating.
