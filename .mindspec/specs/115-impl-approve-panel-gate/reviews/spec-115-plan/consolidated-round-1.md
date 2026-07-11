# spec-115-plan — Round 1 consolidated tally

**Reviewed HEAD**: `57d043fb` (plan draft). **Panel**: 9 slots (F1–F3 Fable, O1–O3 Opus, G1–G3 codex). **Threshold**: ≥8 APPROVE, no REJECT. Findings never out-voted.

## Verdicts

| Slot | Family | Lens | Verdict | Conf |
|------|--------|------|---------|------|
| F1 | Fable | falsifiability / RED-on-revert | APPROVE | 0.92 |
| F2 | Fable | grounding empirical | APPROVE | high |
| F3 | Fable | decomposition / dep-edges / contradiction | APPROVE | 0.90 |
| O1 | Opus | anti-gaming / spec-coverage completeness | APPROVE | high |
| O2 | Opus | ADR / import-edge / blast-radius | APPROVE | 0.93 |
| O3 | Opus | provenance parity (G3-vs-O1 tiebreaker) | APPROVE | — |
| G1 | codex | grounding empirical | APPROVE | 0.99 |
| G2 | codex | seam / type / import correctness | APPROVE | 0.98 |
| G3 | codex | plan-approve readiness / provenance | REQUEST_CHANGES | 0.97 |

**Aggregate: 8 APPROVE / 1 REQUEST_CHANGES / 0 REJECT.** Family split (APPROVEs): 3/3 Fable, 3/3 Opus, 2/3 codex. **PASSES** the ≥8 threshold.

## The sole REQUEST_CHANGES (G3) — AUDITED REFUTATION

**G3's finding:** AC1, AC6, AC7 are each nominally assigned to Bead 2 while their spec-named proof chains land one test in Bead 1 (substrate) and one in Bead 2 (gate) — G3 calls this "double-ownership" violating the exactly-one-bead-owner rule, and asks to "make Bead 2 the sole verification owner running every complete spec-named proof."

**Disposition: EVIDENCE-REFUTED (not out-voted).** Adjudicated by O3 (the dedicated provenance-parity tiebreaker lens), corroborated by O1:

1. **The spec itself authored these as compound, cross-package ACs.** Spec AC1 explicitly names the lifecycle test `TestScanOrphanedClosedBeads_ErrorPreserving`; AC6 names the complete-package `TestPendingObligationPredicate`; AC7's own spec text *mandates* the two-test split and states the import-cycle reason. The plan faithfully homes each half; it did not invent a split.
2. **No proof test is authored twice.** Each named test has exactly one bead home (the bead owning its package). Only the AC→test *mapping* spans the dependency edge — inherent to a spec-written compound AC, not "double-ownership in the load-bearing sense" (nothing is built in two places).
3. **G3's proposed fix is provably infeasible.** Verified at HEAD: `internal/approve` does not import `internal/complete` (comment only); Bead 2 adds the `approve→complete` edge. A white-box `package complete` test calling `ApproveImpl` therefore forms a real Go build cycle (`complete→approve→complete`). `TestCompleteRun_RegatesAlreadyClosedOrphan` drives `complete.Run` and physically cannot relocate into Bead 2's `internal/approve`-only scope. "Make Bead 2 run every proof" cannot be implemented.

The finding is a taxonomy artifact of a sound substrate/consumer split, not a decomposition defect. Refutation is audited here per the findings-never-out-voted discipline.

## Non-blocking clarifications applied (responsive to panel notes; doc-only, zero change to any verified claim)

Folded into the plan before plan-approve to sharpen the bead briefs (two are genuine implementation traps):

- **F3-1 (impl trap):** Bead 1 step 3 — "`reconcilePendingRefutations` re-expressed over the same core" clarified to name an *unexported* compute-uncovered-set helper beneath BOTH the exported check-only predicate (errors on uncovered) AND reconcile (settles); the shared core is NOT the exported predicate.
- **F2-2 (discriminator trap):** `PanelGateRoots` must not be discriminated by a bare symbol-grep — pre-existing `TestPanelGateRoots_LayoutAware` contains the substring; use a func-def / file-scoped grep.
- **F2-1 (grounding):** Bead 3 step 2 — `refreshManagedSkill` is a doc-comment-only name; corrected to the real shipped-vs-user-modified mechanism (`installSkills`/`matchesShipped`, skills.go).
- **F1-7 (wording):** the "ZERO hits repo-wide at `eb6a2ed1`" phrasing for `ScanOrphanedClosedBeads` tightened to "ZERO code hits (sole match is the spec's own prose)" — matches the round-7 spec AC13 fix style.
- **O3 legend (optional, applied):** a one-line Provenance legend noting compound ACs spanning the substrate/consumer edge name the completion-owner bead in the Bead column with the prerequisite half annotated inline.

None touches decomposition, proof chains, ADR fitness, or grounding substance — the 8 approvers' verified claims stand byte-for-byte. Per the round-7 spec precedent (single responsive wording fix → approve, no re-panel), these clarifications do not warrant a re-panel; the orchestrator reviews the diff as a doc-only gate before plan-approve.

## Decision

**PASS → plan approve.** Apply the doc-only clarifications, orchestrator-review the diff (confirm plan.md-only, `mindspec validate plan` green), then `mindspec plan approve 115-impl-approve-panel-gate` (creates 3 beads under epic `mindspec-fgmg` in dep order 1→2, Bead 3 independent).
