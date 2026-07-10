# spec-114-plan-approve — Round 1: 4 APPROVE / 5 REQUEST_CHANGES

APPROVE: F1 0.88, O1 0.88, O2 0.9, O3 0.86. RC: F2 0.85, F3 0.85, G1 0.97, G2 0.97, G3 0.99 (all G on gpt-5.6-sol). Core decomposition/ADR/provenance/domain confirmed sound; the RC findings are durable-audit airtightness + AC5-enumeration accuracy + prose.

## LOAD-BEARING: durable-audit rework (G2 — holes O1's "structurally unfalsifiable" missed)
1. **TOCTOU / write-after-close gap**: the plan writes panel_refuted at step 4.75 (AFTER step-4 close, before terminal merge). If the audit write fails, the bead is ALREADY CLOSED; on retry, panelGate FAIL-OPENS when the panel is absent (fail-open-without-a-panel, leg — ADR-0037 §6), so a removed/lost panel.json lets the already-closed bead merge with NO panel_refuted. The convergence contract (complete.go:695-708) guarantees merge-retry, NOT audit-obligation preservation. FIX: make the audit obligation durable BEFORE the bead can close — write+verify panel_refuted immediately BEFORE step-4 close (AppliedRefutations are already known from panelGate at step 2.25), failing the completion pre-close if it can't write; OR persist a non-bypassable pending obligation an already-closed retry must satisfy even if the panel later disappears. Add a retry test: audit-write-fails → bead NOT closed (or obligation persisted) → panel removed → re-run must NOT merge un-audited.
2. **Post-merge metadata erasure**: `bead.MergeMetadata` (internal/bead/bdcli.go:263-266) IGNORES metadata-READ errors and does a replace-style update from an empty map; post-merge best-effort doc/ADR writes use the same helper. So a transient show-failure + successful update ERASES a previously-written panel_refuted → successful completion with the audit LOST. FIX: make MergeMetadata FAIL-CLOSED on an existing-metadata read failure (or use an atomic merge/patch primitive); ADD internal/bead to scope (execution domain — update Impacted Domains + the plan's domain note; O3 had it read-only, now it's edited); test a successful refutation completion where a post-merge best-effort write hits a read error → panel_refuted must survive.
3. **Duplicate refutation entries** (same slot+round, or across matched panels): specify DETERMINISTIC selection + DEDUPLICATE AppliedRefutations before auditing. (Extends AC12.)

## AC5 enumeration accuracy (F2/F3/G1/G3 — the plan's "exhaustive" claim is false)
4. The AC5 reconciliation misses fixtures that flip under leg-9.5 but are in NEITHER the flip list NOR the outcome-preserving list — breaking Bead 1's own AC5 diff-review proof ("flips ONLY in the step-5 set"). Classify EACH explicitly (prefer an outcome-preserving input update — add a genuine APPROVE — keeping the AC5 3-flip carve-out intact):
   - `internal/panel/panel_decision_test.go:211-219` (2 APPROVE + 1 RC "expected_reviewers:3 → Allow, no hardcoded 6") — all 4 reviewers found it.
   - `cmd/mindspec/panel_test.go:557-558` (G3).
   - `internal/instruct/panelstate_test.go:903-905` (G3).
   Update Bead 1's diff-review checklist + completion-note file list to include them; retain the message-substring assertions for the genuinely sub-threshold Block rows.

## Prose/accuracy fixes (G1/G2/O1)
5. "abandon/skip are terminal non-merge exits" is WRONG — abandoned (gate.go leg 3) + env-skip both Warn→Allow→MERGE. Ground the durable asymmetry SOLELY in: a refutation CHANGES the gate outcome and produces AppliedRefutations; abandon/skip produce NO AppliedRefutations (so they stay best-effort). Fix the rationale in plan + the ADR-0037 §7 amendment.
6. "leg-10 Block is unreachable with zero refutations" is WRONG — reachable via the threshold>0 guard (panel_test.go:260-277). Correct Bead 1 step 4.
7. Only ENV-skip writes panel_gate_skipped; CONFIG-disabled writes no audit. Correct Bead 2 step 4 / Bead 3 prose.
8. The already-closed RECOVERY path (complete.go:547-554) does NOT force dolt-commit + committed-state verify (unlike the normal close path 546-650). Qualify Bead 2 step 5's durability rationale + test the recovery path preserves/re-writes the audit.

## Minor (F1/O3/O1/F3)
9. Add `internal/complete/panel_gate_layout_test.go` (panelGate callers :152,:162) to Bead 2 key_file_paths.
10. F1: AC5 carve-out is 4 fixtures incl. the VoteDecision twin (votedecision_test.go:64-70) — Bead-1 panel gates on the four-flip set. F1/F3 nits: Dup-named AC12 subtest; step-6(c) pointer → shared buildFacts (panel_test.go:417-428); a skip-env-with-refutations e2e row (no panel_refuted on skip).

Keep the core R1/R2 model + resolved OQs. The durable-audit rework (items 1-3) is a real design strengthening — treat it carefully. Re-validate 0 ERRORs.
