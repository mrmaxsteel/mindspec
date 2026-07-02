# spec-108-approve — Round 1 Review Panel (spec-approve gate)

**Worktree**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-108-cleanup-frontmatter-perf-wave2
**Branch**: spec/108-cleanup-frontmatter-perf-wave2
**Commit under review**: 655052a23ecf1024399c5750f75418c0757c043c
**Target**: the SPEC DOCUMENT (pre-approval). No implementation exists. Evidence report: review-source-report.md at the worktree root.

## What the work does

Spec 108 = cleanup wave 2, four work areas: (1) claim internal/trace/** + .golangci.yml in workflow OWNERSHIP, delete dead trace.Event.MarshalJSON, remove the TWO verifiably-inert unparam carve-outs (findRoot carve-out explicitly deferred — on this branch base findRoot still exists and unparam flags it; grill-verified); (2) frontmatter consolidation onto internal/frontmatter.Parse (two approve mutate-rewrite scanners → one helper; four reader fence-scans migrated; validate/state.go prose-scan → validate.SpecStatus; fence-strictness deliberately tightened to canonical semantics); (3) validate perf: OWNERSHIP loads hoisted per-domain-per-run (was per file×domain incl. git-show subprocesses), ADR parsing memoized per run; (4) doctor dead-manifest check walks tree once. Impacted domains: workflow + context-system; ADRs: 0036/0032/0033 (all Accepted, intersecting). 10 requirements, 14 runnable ACs (PASS-line pattern for new tests, seam-counting perf proofs). Grill closed: trace→workflow ownership grounded in the real import graph (the review report's "recording/otel" claim was verified false), R3 rescoped after empirical unparam evidence.

## Notable cross-spec context

Spec 107 (wave 1) is mid-implementation on its own branch; 108's base predates it. The spec bakes in sequencing constraints (ownership claims land same-diff-or-earlier than trace/.golangci edits; findRoot carve-out deferred post-107). Evaluate whether these constraints are complete and whether any other 107↔108 interaction is unhandled (e.g. both touch internal/validate/plan.go).

## Your job

Is this SPEC ready for approval? Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to review/spec-108-approve/<your-slot>-round-1.json (relative to worktree root) with keys: reviewer_id, verdict, confidence (0-1), rationale (<=200 words), concrete_changes_required (empty if APPROVE), findings; optional "hard_block": true.

---

# ROUND 2 ADDENDUM (commit under review: 8b9453596bfc7bce9faaed70e804798919d51908)

**Prior round**: 3 APPROVE (R1/R2/R3 claude), 3 REQUEST_CHANGES (R4/R5/R6 codex). All 8 asks consolidated and claimed applied in 8b9453596bfc7bce9faaed70e804798919d51908:

A. (R6) Spec 107 is now an explicit HARD PREREQUISITE requirement: 108 enters Plan Mode only after 107 merges to main and spec/108 rebases onto post-107 main; merge-resolution order for internal/validate/plan.go documented; bead-decomposition constraints (3-5 beads, same-diff-or-earlier claims, bead-unique doc targets across 108's own beads) baked into the spec.
B. Consequence: the findRoot carve-out removal is REINSTATED into R3 (inert post-rebase), Out-of-Scope deferral dropped, AC updated to all-three-absent.
C. (R4) unparam evidence reworded precisely (probe was --no-config, bypassing all exclusions).
D. (R5.1) Doctor walk seam named in R9 (package-level walkWorkspaceFn routed through the refactor; TestOwnershipCheckWalksTreeOnce counts it).
E. (R5.2) Byte-identical promises now carry proofs: golden-diagnostics AC (full error+warning set identical pre/post caching), doctor full-Report-comparison, contextpack narrowed to existing coverage + cited test (or a new golden AC if absent — check what the spec now says).
F. (R5.3/.4) Manifest-load seam-count AC covers all three call sites; carve-out AC also asserts validateReviewMode remains.

## Your job (round 2)

R4/R5/R6: mark each of YOUR round-1 asks ADDRESSED / PARTIAL / MISSED in findings, verifying against the spec text and tree. R1/R2/R3: re-verify approval holds (requirements renumbered/extended; prerequisite requirement added). Verdict: APPROVE / REQUEST_CHANGES / REJECT → `review/spec-108-approve/<slot>-round-2.json`.
