# spec-112-final-review — round-1 disposition

**Tally: 11 APPROVE / 1 REQUEST_CHANGES / 0 REJECT (pass threshold ≥11/12 MET).**
Family split (of 11 APPROVE): Fable 3/3 (F1 0.93, F2 0.93, F3 0.93), Opus 3/3 (O1 0.90, O2 0.93, O3 0.93), Sonnet 3/3 (S1 0.93, S2 0.90, S3 <see file>), codex 2/3 (G1 0.94, G3 0.91; G2 RC).

The whole spec was verified end-to-end: F1 ran all 12 ACs non-vacuously; S1 reproduced the full four-family protocol proof (sums 9/9/6/12, adhoc→bead fallback, refusals, 109 backward-compat); S2 mutation-tested the load-bearing tests (5 mutations, all caught — no decorative tests); F2 traced every producer→consumer edge across the 3 beads (no half-wiring); F3+G1 confirmed the escaping class clean (no raw control bytes, JSON byte-exact); O2 confirmed ADR-0040/0037/0035/0023 compliance and the config-free `internal/panel` leaf; O3 confirmed inert-but-declared (no dispatch/spawn/gate-logic activation); G3 confirmed 110/111 can consume the R9 surface; S3 confirmed no regression.

## The one dissent — G2 (codex, schema/type), REQUEST_CHANGES, hard_block flag → DECLASSIFIED to follow-up

**Finding**: config `Load` accepts `{model: "", family: codex}` (explicit empty-string model with family present), resolving it to the family, whereas R4's requirement text lists "…or an empty-string `model`" as a refusal. Empirically reproduced.

**Disposition: declassified to follow-up bead `mindspec-cthj` (P3); NOT a blocking round-2 fix.** Reasoning, with citations:
1. **Spec-internal contradiction, not a clean defect.** R1 (spec.md ~line 86) states verbatim: *"an entry with `family` and no `model` is valid and resolves its model to the family string."* A typed Go struct (`Reviewer.Model string`) **cannot distinguish** an absent `model:` key from `model: ""` — both yield `Model == ""`. So R4's "empty-string model" and R1's "family and no model is valid" collapse onto the same value, and the implementation resolves it per R1 (valid family-fallback).
2. **The acceptance criterion sides with the implementation.** R4's AC (spec.md ~line 136) enumerates the required refusal tests as "…entry with neither `model` nor `family`; explicit `count: 0`/negative; …" — it does **not** list the empty-model-with-family case. The implementation satisfies the falsifiable AC contract.
3. **Panel corroboration.** Two reviewers who specifically probed R4 — F1 (spec-goal-fidelity) and S2 (test-quality, which cross-checked every `validateGates`/`validateReviewerEntries` `fmt.Errorf` path against the AC (a)-(h) table) — found the refusal table correct and complete. No reviewer besides G2 read R4 as requiring this refusal.
4. **No correctness/security hole.** `{model:"", family: codex}` yields a valid `codex` reviewer, not a broken or never-passing panel.
5. **G2's `hard_block` flag is misapplied.** The gate reserves `hard_block` for the missing-measurement-artifact class (cost projection / drift report / regression baseline) — "could the missing artifact have caught a real defect?". There is no missing artifact here; this is a code-validation reading, so the flag does not force a block.

Resolving it means unilaterally picking a side of an R4-vs-R1 text contradiction in code (a spec decision under forward-only ADR-0023) and would require a custom `UnmarshalYAML`/`Model *string` to distinguish absent-vs-empty. That belongs in `mindspec-cthj` for a conscious spec-clarification, not an autopilot round-2 fix that contradicts R1.

## Decision: PASS → `mindspec impl approve 112-per-gate-panel-config`.
Merges cleanly to current main (`git merge-tree` = 0 conflicts). Follow-ups filed: `mindspec-cthj` (this) + `mindspec-naq0` carries R7-bead3's config-show display nits.
