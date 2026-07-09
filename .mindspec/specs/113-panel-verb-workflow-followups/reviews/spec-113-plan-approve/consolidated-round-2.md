# spec-113-plan-approve — PASS (9/9 effective)

Round 1: 8 APPROVE / 1 REQUEST_CHANGES (O2) / 0 REJECT — cleared the ≥8/9 threshold, but O2's finding was a REAL internal contradiction, so it was fixed rather than out-voted.
Round 2: O2 flipped to APPROVE (ADDRESSED). Net: F1 0.93, F2 0.93, F3 0.90, O1 0.90, O2 0.90(r2), O3 0.92, G1 0.90, G2 0.88, G3 0.86 — all APPROVE.

## The one fix (O2, Bead 1 / R1)
Round-1 plan threaded a non-bead flag into `tallyExitAction(d, slug)` (signature change) while also claiming its pinned test `TestPanelTally_ExitCodeTracksDecision` stayed UNMODIFIED — impossible given `panel.Decision` is `{Action,Message}` only + the zero-diff-to-internal/panel constraint.
FIX (O2 option a, commit 6d9b1f62): `tallyExitAction(d, slug)` stays 2-arg/byte-identical; the non-bead recovery line is rendered in the `panelTallyCmd` RunE handler (panel.go ~189-207) gated on `!reg.Panel.IsBead()`, building its own `guard.NewFailure` over the already-sanitized `d.Message` with NO `mindspec complete <bead>` clause. The pinned render tests use bead fixtures so they stay literally unmodified. Design (CLI-layer sanitize, zero internal/panel/instruct/complete diff, revParseForPanelFn non-bead closure) UNCHANGED — only WHERE the recovery renders moved.

## Load-bearing confirmations from round 1 (carry into impl)
- O1+G2+F2+F3+G1 all independently confirmed the R1 **zero-byte-diff-to-internal/panel** claim is TRUE: the bug's leg selection is fact-driven (caller-supplied GateIO.RevParse closure), and Decision.Message is a returned value the CLI owns → sanitizeNonBeadDecision fixes the message with no gate.go edit. Fences hold: complete's panelGate short-circuits on empty beadID; instruct's verdict() sets staleness facts only if IsBead().
- O3 verified ADR-0030 deliberate not-cite is CORRECT (adr-cite-irrelevant lint errors on empty domain-intersection: 0030=execution/validation/lifecycle/lint vs 113=workflow/core). ADR-0034 re-attribution to spec 112 honored.
- All 4 spec-panel carry-forwards folded (O1 CLI-layer mandate, O2 ADR re-attribution, F2 legs-(2)/(4)-can-Block precision, F3 verify=read-only/tally=block-capable).
