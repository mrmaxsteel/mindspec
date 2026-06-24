# 106-final-panel — Final Review Panel (whole spec branch vs main, before the PR)

**Spec**: `106-layout-flatten` — flatten the `.mindspec/` layout. **Spec branch**: `spec/106-layout-flatten` @ `32d22f38`. **Base**: `main` @ `1d043af2`.
**Cumulative diff**: `git diff main...spec/106-layout-flatten` — **554 files (441 renames + 35 A + 4 D + 74 M), +9537/-1159, 72 .go source files changed**.
**Spec/plan (flat)**: in the spec worktree `/Users/Max/replit/mindspec/.worktrees/worktree-spec-106-layout-flatten/.mindspec/specs/106-layout-flatten/{spec.md, plan.md}`.

## What the spec delivered (all 6 beads merged)
- **Bead 1** — per-artifact three-tier resolvers (flat→canonical→legacy) + `DetectLayout` (5-state, in-progress-scoped mixed guard) + `ClassifyLayout` signature helper + flat worktree tier (ew79-preserving) + born-flat bootstrap.
- **Bead 2** — permanently multi-prefix gate matchers (docsync/divergence/ownership incl. the ref-anchored `LoadOwnershipAtRef`) + additive reviews classifier + `project-docs/**` non-source + tier-aware enumerators.
- **Bead 3** — net-new `internal/layout` transactional two-commit mover (+ crash-resume, scoped rollback, finite-pattern link-rewriter, doctor 404 link-check, `migrate layout` CLI, branch/PR precondition) + gitutil/executor git primitives.
- **Bead 4** — layout-aware transitional panel gate + directional merge-fingerprint guard at the real executor seams + doctor layout detection/dual-ID error.
- **Bead 5** — executed the IRREVERSIBLE flatten: `.mindspec/docs/{specs,adr,domains,core}` + context-map → top-level `.mindspec/`; dogfood `{user,installation,research}` → `project-docs/`; root `review/` → co-located `<spec-dir>/reviews/`; dropped `glossary.md` + `policies.yml`; self-globs repointed; 0 dangling links.
- **Bead 6** — flat-path skills/setup-text/snapshots/templates; governance (created `ADR-0039`, amended `DOCS-LAYOUT.md` + `ADR-0037`); migrate rubric; harness/testdata fixture migration; doctor/ownership tier-awareness.

## Known state (verified)
- Non-LLM suite GREEN modulo exactly **2 pre-existing, layout-independent failures** (R4 of the Bead-6 panel confirmed both fail identically on the un-flattened `main` baseline): `internal/instruct/TestRun_IdleNoBeads` (z4ps test-isolation leak) and `internal/harness/TestInstructPhaseDetection/{plan,implement}` (this env's `init.defaultBranch=main` makes the sandbox branch protected → `RenderIdleIfProtected` idle short-circuit; dated pre-spec-106). **NEITHER is a spec-106 regression.**
- `mindspec validate spec` (AC25) hangs in this OFFLINE sandbox (Dolt git-network) — could not be run; spec is Approved, ADR-0039 Proposed/uncited.
- `./bin/mindspec` was rebuilt from the spec branch code (recognizes flat paths).

## Your job (holistic FINAL review — each bead was already 6-reviewed; do NOT re-litigate line-by-line)
Assess MERGE-READINESS of the whole spec per your lens (below). The diff is large — focus on the 72 `.go` source files (`git diff main...spec/106-layout-flatten -- '*.go' ':(exclude)docs_archive'`), the spec.md/plan.md, and the cumulative coherence — not every rename. Use the spec worktree to run things if needed. **NEVER run `TestLLM_*` / unfiltered `./internal/harness/...`.**

**Verdict**: APPROVE / REQUEST_CHANGES / REJECT (REQUEST_CHANGES only for a genuine merge-blocker — a regression, an incomplete deliverable, or a correctness hole that survived the per-bead panels).

Output JSON to `/Users/Max/replit/mindspec/review/106-final-panel/<your-slot>-round-1.json`: `reviewer_id`, `verdict`, `confidence` (0-1), `rationale` (≤200 words), `concrete_changes_required` (array), `findings` (array of {severity, area, issue, hard_block?}).
