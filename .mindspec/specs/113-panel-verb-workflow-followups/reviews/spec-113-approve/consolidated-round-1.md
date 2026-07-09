# spec-113-approve — Round 1 tally: PASS (9/9 APPROVE, 0 REJECT)

All three families unanimous. No blocking changes. Plan-time carry-forwards (NOT spec edits):

1. **[O1, R1 — load-bearing plan constraint]** R1's message hygiene (no `bead/<empty>`) MUST live in the CLI rendering layer (`cmd/mindspec` panel verify/tally), NOT in `PanelGateDecision`'s shared `Decision.Message` — `internal/instruct`'s `verdict()` already consumes `d.Message` for non-bead panels, so mutating the shared message would break the AC-global "instruct tests pass unmodified" fence AND R1's own falsification. The paired AC-global + falsification already pin this; the R1 bead brief must state it explicitly.
2. **[O2, cosmetic]** ADR-0034 touchpoint misattributes 112 R4's gate-vocabulary disambiguation to ADR-0034 (which is ceremony-collapse, not gate vocab). Link resolves + content true → cosmetic. Re-attribute to spec 112 during plan authoring.
3. **[F2, background nuance]** Spec Background's "the CLI verbs can never Block on a non-bead panel" is slightly overbroad: legs (2) unreadable panel.json and (4) round-mismatch precede leg (5) and CAN still Block. Tighten wording at plan time; behavior unaffected.
4. **[F3, wording]** "read-only/advisory" phrasing vs `tally`'s Block exit — reword to "read-only (no repo mutation)" at plan time.

Grounding (F2/G1/O1): R1 bug confirmed live — `resolvePanelGateFacts` cmd/mindspec/panel.go:302-306 leaves beadID="" for non-bead; gate.go:372 rev-parses literal `bead/`→MissingRef→leg(5) Warn `references branch bead/` at gate.go:186-189. R2 regex at ms-panel.js:138 (`.claude` mirror byte-identical). R3 no `--gate` today; Panel.Gate + resolvers exist. R4 code already resolves-to-family (Reviewer.model() config.go:139-144).
