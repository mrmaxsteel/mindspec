# spec-128-bead10 — consolidated concrete_changes_required, round 1

Tally: 3 APPROVE (R1, R2 with a minor rider, R4) / 3 REQUEST_CHANGES (R3 ×2, R5 ×1, R6 ×4). 8 asks in 5 groups. R3+R6 converge on the two substantive docs-truth defects. Artifact-gate screen: clear.

## G1 — Workflow docs narrate the DEAD round-1 enumeration (R3 ask 1 + R6 ask 2, convergent) — MUST FIX
architecture/interfaces/overview.md's R5 prose still claims "the entanglement is exactly session.json/issues.jsonl/worktree-dir … empty today" — contradicting ADR-0025's R5.1 amendment and the landed ComputeEntangleablePaths. Rewrite to the actual conditional classes: entry-absent .gitignore at the resolved claim anchor; the target spec's recording paths when recording.enabled — both gated on canonical-toplevel containment. Every "today empty" claim goes.

## G2 — merge-safety.md oversells the retry guarantee (R3 ask 2 + R6 ask 3, convergent) — MUST FIX
The "same gate refuses again … never a DIFFERENT gate" sentence promises more than the machinery guarantees. Rewrite lines ~23-38 distinguishing (a) plain retry without override: re-evaluates the same content-gate class, no manufactured panel staleness; (b) retry WITH --allow-doc-skew/--override-adr: bypasses that gate class, the rest of the pipeline proceeds. Claim only what bead 6's M/R/S/G/T machinery pins.

## G3 (R6 ask 1) — Stale spec-119 artifact-materialization paragraph — MUST FIX
workflow/architecture.md:346-354 still says commits precede doc-sync/ADR-divergence and a content-gate refusal retains commits — the exact pre-bead-6 ordering ADR-0041's replacement killed. Replace or explicitly supersede, reconciled with the prospective-snapshot protocol.

## G4 (R6 ask 4) — Selection prose misses the positional exception — MUST FIX
Scope the ReadySet/deterministic-selection prose to non-positional + --pick flows; carry R6.3(a)'s positional --emit-only exception explicitly (no ReadySet, no readiness, no filter, no exclusion report, the exact bypass line).

## G5 (R5 + R2's approve rider, small precision) — MUST FIX (cheap)
core/overview.md's dangling "See § below and interfaces.md" fragment → accurate section reference. R2's two stale line-citations in core/architecture.md (spec.go:76→:91 for the backfill; the second citation in its verdict) → correct or de-line-number them.
