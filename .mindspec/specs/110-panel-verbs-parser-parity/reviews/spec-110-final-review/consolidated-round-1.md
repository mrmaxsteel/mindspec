# spec-110-final-review — disposition

**Tally: round-1 11 APPROVE / 1 REQUEST_CHANGES (S2) / 0 REJECT → S2 fixed → S2 round-2 APPROVE ⇒ effectively 12/12. PASS (≥11/12).**

Family split: Fable 3/3 (F1 0.93, F2 0.92, F3 0.90), Opus 3/3 (O1 0.90, O2 0.90, O3 0.90), Sonnet 3/3 (S1 0.93, S2 0.82→r2 0.93, S3 0.95), codex 3/3 **SUBBED** (codex walled on ChatGPT-plan usage limit, resets ~4:12 PM): G1 sonnet-sub 0.93, G2 opus-sub 0.93, G3 fable-sub 0.88 — slot ids kept, tiers spread to preserve family diversity per the codex-wall fallback protocol.

The whole spec was verified end-to-end: both outcomes (panel create/verify/tally as single decision home; spec-approve parser parity) delivered against every R1–R8 falsification clause. Single decision home REAL across all four consumers (internal/complete, internal/instruct --panel-state, panel verify, panel tally — F1/F2/F3 grepped; the old `verdict()` matrix deleted). `internal/panel/gate.go` BYTE-IDENTICAL to merge-base (R7a). Config-free leaf empirically confirmed (R7b). Parser-parity byte-identical to plan.go, all 4 R5/R6 outcomes reproduced live (mangled domain errors; bare-name tolerated; touchpoint anchored-only; bare-prose ADR tolerated). Security validators (traversal + full C0/C1/DEL via unicode.IsControl, %q-escaped messages) hold on slug/--bead/--target across all three verbs. Schema doc matches internal/panel constants. No regressions (2 known failures — harness timeout, instruct z4ps — verified pre-existing at merge-base).

## The one RC — S2 (test-quality), FIXED
S2 empirically proved `TestPanelCreate_RejectsUnsafeSlugAndControlBytes` leaked the cobra `--bead` flag across subtests: deleting the `rejectControlBytes("--target", …)` call left all 10 subtests passing (the `--target` rows were rejected by a leaked stale `--bead` value). Shipped code correct (F3/G1/S1 defeated hostile inputs empirically); the gap was test coverage — a future regression to the `--target`/`--bead` validation would ship undetected. **Fix `59031008`**: a `resetPanelCreateFlags(t)` helper (mirrors the existing `resetOtel*` idiom) resets spec/target/bead/round + clears `Changed` before each subtest. S2 round-2 re-ran its falsification for ALL THREE validations (slug/--bead/--target) — each now fails exactly its own rows, zero cross-masking. ADDRESSED.

## Non-blocking notes for impl-approve
1. **Workflow-doc merge (G3 fable-sub, verified)**: 112 (per-gate config) is on main; both specs append to `.mindspec/domains/workflow/{architecture,interfaces}.md`. G3 extracted the auto-merged tree to /tmp — it BUILDS and internal/panel + cmd/mindspec + internal/complete + internal/config all test GREEN. The two conflicts are same-insertion-point APPENDs with disjoint topics (no contradictory claims; 112 even forward-cites 110's writer). impl-approve needs only a **doc-only union resolve** (keep both sections in each hunk).
2. **redact.go cross-domain (O3, recorded)**: fbel.4 necessarily edited core-owned `internal/redact/redact.go` (verb token registration for the drift guard) — the spec's "workflow is the sole impacted domain" prose is thus slightly inaccurate, but the edit is correct/necessary (matches spec 101's `release`-token precedent) and was accepted via `--override-adr` at fbel.4's complete. Not a code defect; forward-only.

## Follow-up candidates (info, filed post-approve)
- G3: a `mindspec panel create --gate <name>` flag would complete 112's per-gate advisory loop for CLI/workflow-created panels (currently gate-less by 112's designed degradation).

## Decision: PASS → `mindspec impl approve 110-panel-verbs-parser-parity` (resolve the doc union, then PR to main).
