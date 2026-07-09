# spec-112-plan-approve — Round 2 (fix re-verification, 9 reviewers, three families)

**Under review**: `.mindspec/specs/112-per-gate-panel-config/plan.md` @ **2809c8a64a87c260ac26b9cabac6940bd9f1c7c7** (fix commit on top of round-1 e06c0b76). Read the approved `spec.md` beside it for the contract.
**Panel**: 9 reviewers, three families — F1–F3 Fable, O1–O3 Opus, G1–G3 GPT-5.5 (codex). Pass = **≥8 APPROVE, no REJECT**.
**Round-1 tally**: 7 APPROVE / 2 REQUEST_CHANGES (O1, F3) / 0 REJECT. Change list: `consolidated-round-1.md` in this dir.
**READ-ONLY RULE**: verdict JSON only; scratch under /tmp; pin reads to the SHA.

## The fix delta (`git diff e06c0b76..2809c8a6 -- .mindspec/specs/112-per-gate-panel-config/plan.md`)

Both round-1 dissents were surgical (cross-bead coordination + test-coverage falsification), not design problems. Fixes applied:

1. **(O1 must-fix) Bead 1 now owns its `Reviewer.Count` consumer.** `cmd/mindspec/config.go` added to Bead 1's `key_file_paths`; a step updates the sole external reader `config.go:149` to deref via the `count()` accessor (the `int→*int` change made `"count: %d"` print the pointer address — passes build+vet, breaks `TestConfigShow_EmitsPanelModelsLoop`); `go test ./cmd/mindspec/...` added to Bead 1's Verification so the gate can't false-green. Bead 1 stays 7 steps.
2. **(O1 must-fix) nil-guard** on Bead 3's `panelReg.Panel.Gate` read (panelGate returns nil on the fail-open no-panel path) + a panel-less test case.
3. **(O1 must-fix) `panelGateKeys` exported** as `PanelGateKeys` (or accessor) so Bead 3 (`cmd/mindspec` main + `internal/complete`) can reference the enum order.
4. **(F3 must-fix) gates-configured cmd-side advisory test** added to Bead 3 (the existing test runs gates-absent → couldn't falsify a skipped step-2 wiring, leaving R7's spurious-note regression live with all Verification green).
5. **(F1/F3 should-fix) jq the slot-count proof** — `jq '.slots | length'` instead of `grep -c '"slot"'` (compact encoding/json would false-fail the grep).
6. **(O2 should-fix) ADR-0037 amendment homed under §1** (schema block, abandon_reason precedent), not §3.
7. Nits: cursor-fixture ordering note; R5-reporting-half-in-Bead-3 summary note.

## Round-2 jobs

- **O1, F3 (round-1 RC voters)**: evaluate EACH of your round-1 concrete_changes_required as ADDRESSED / PARTIAL / MISSED / NEW_ISSUE against 2809c8a6. O1 → the Reviewer.Count consumer ownership (is cmd/mindspec genuinely green after Bead 1 now? is the deref + go-test gate correct?), the nil-guard, the PanelGateKeys export. F3 → the gates-configured cmd advisory test (does it now falsify a skipped wiring?), the jq proof fix, the cursor-fixture ordering.
- **G1, G2, G3, F1, F2, O2, O3 (round-1 approvers)**: confirm the fix delta introduces no regression in your lens. F2/O3: does adding cmd/mindspec/config.go to Bead 1 create a NEW merge-signal problem (cmd/mindspec/config.go is now touched by Bead 1 AND Bead 3 — is that a false-dep or acceptable given the B1→B3 edge)? Assess. O2: is the ADR-0037 §1 placement now correct? G1: are the jq proof + new test commands runnable?

Verdict: APPROVE / REQUEST_CHANGES / REJECT → `<slot>-round-2.json` (reviewer_id "<slot> <family>", verdict, confidence, rationale ≤160 words, concrete_changes_required, findings with per-item dispositions for RC voters).
