# spec-110-bead4 — consolidated round-1 changes

Tally: 6 APPROVE (R1, R2, R3, R4 0.93, R7 fable 0.9, R8 codex) / 2 REQUEST_CHANGES (R5 sonnet 0.9, R6 sonnet 0.8) / 0 REJECT. Threshold 7/8 not met → fix round. The verb tree, R7a single-home (both `TestPanelVerbs_DecisionIsPanelGateDecision` + `TestPanelTally_ExitCodeTracksDecision`), resolver stamping/co-bump, read-only verify, Decision.Action-derived tally exit, layout-aware dir parity with `internal/complete.panelGateRoots`, and the C0/DEL/traversal validators all verified independently. Two distinct real gaps:

## Fixes (both landed in `989e2684`)
1. **(R5, security) C1 control-byte gap.** `rejectControlBytes` rejected only C0+DEL (`r < 0x20 || r == 0x7f`), missing the C1 range (U+0080–U+009F incl. CSI U+009B) — the exact terminal-injection class `cmd/mindspec/report.go`'s `stripControl` already fixes ("codex-render-leak #2"). A slug/`--bead`/`--target` with U+009B passed and printed the raw C1 byte. Fix: predicate → `unicode.IsControl(r)` + 3 C1-CSI test cases.
2. **(R6, downstream contract) missing panel-directory in `create` output.** `panel create` never printed the resolved panel dir, but spec 111's plan requires capturing it from `create`'s output (and forbids re-deriving layout). Fix: added `panel directory: <dir>` stdout line + interfaces.md doc.

## Non-blocking / carried
- R7-fable confirmed the security validators otherwise hold (traversal, C0/ESC/TAB/DEL, `--target` RejectOptionLike/RRejectOptionLike, `--bead`/`--spec` never become path components). R4 confirmed the full empirical contract (stamping, co-bump, read-only verify exit 0, tally exit tracks Decision.Action).

## Constraints for the fix author (met)
ONE commit on `bead/mindspec-fbel.4`; only `cmd/mindspec/panel.go`, `cmd/mindspec/panel_test.go`, `.mindspec/domains/workflow/interfaces.md`; all tests green; no push/bd/lifecycle.
