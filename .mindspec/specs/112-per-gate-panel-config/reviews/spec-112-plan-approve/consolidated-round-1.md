# spec-112-plan-approve — round 1 consolidated plan fixes

**Tally: 7 APPROVE / 2 REQUEST_CHANGES (O1, F3) / 0 REJECT — BELOW the 8/9 threshold → FIX ROUND.** Both dissents are surgical (cross-bead coordination + test-coverage falsification gaps), not design/decomposition problems (F2/O3/O2 confirmed those sound). All fixes are plan-text edits (no spec change, no bead-structure change). Reviewed SHA e06c0b76.

## Must-fix (O1, implementability — empirically verified)

1. **Bead 1 must own every `Reviewer.Count` consumer update.** Pointerizing `Reviewer.Count int → *int` silently breaks `cmd/mindspec/config.go:149` (`fmt.Fprintf(..., "count: %d\n", r.Count)` — `%d` on a `*int` passes `go build` AND `go vet` but prints the pointer ADDRESS, breaking the existing 109 test `TestConfigShow_EmitsPanelModelsLoop`). Fix in the PLAN: (a) add `cmd/mindspec/config.go` to Bead 1's `key_file_paths`; (b) add a Bead-1 step to update the sole external `Reviewer.Count` reader (config.go:149) to deref via the new value accessor (`count()`) so Bead 1 leaves cmd/mindspec GREEN; (c) add `go test ./cmd/mindspec/... ./internal/config/...` (or at minimum the affected test) to Bead 1's Verification so the gate is not false-green for pointerization. Net: the branch stays green after every bead.

2. **Nil-guard in Bead 3 step 1.** The `panelReg.Panel.Gate` read must be guarded by `panelReg != nil` — `panelGate` returns nil on the fail-open no-panel path (the common `mindspec complete` case), so an unguarded deref panics. Add the guard to Bead 3's step + a test case for the panel-less path.

3. **Export the enum for cross-package use.** `panelGateKeys` is unexported in `internal/config` but named as Bead 3's cross-package enum-order source; `cmd/mindspec` (package main) and `internal/complete` cannot reference it. Bead 1 must export it (`PanelGateKeys`) or provide an exported accessor; Bead 3 consumes the exported form. Update both beads' steps.

## Should-fix

4. **jq the slot-count proof (F1-1).** Bead 3's protocol-YAML Verification uses `grep -c '"slot"'` == 12, which assumes pretty-printed JSON; the plan only pins `encoding/json` and compact output would make `grep -c` print 1 and FALSE-FAIL a correct impl. Change to `jq '.slots | length'` (jq is already used one line later for the adhoc≡bead equality).

5. **Home the ADR-0037 amendment in §1, not §3 (O2).** Bead 2 step 3 places the new `gate`-field amendment note under §3 (the threshold-rule section) "alongside the 2026-07-07 note"; but the repo convention puts schema/registration additions under §1 (the abandon_reason / 099/102/106 precedents). The 109 note is in §3 only because it extended the threshold rule, which the gate field does not. Move the note to §1's schema block.

## Must-fix (F3, adversarial — upgraded from F1's nit)

6. **cmd/mindspec gate-aware advisory has a falsification GAP (F3-1, = F1-2 sharpened).** R7 names TWO caller surfaces; only `internal/complete` gets a test that fails if the wiring is skipped. The `cmd/mindspec` caller (`reviewerCountNotesFor`, config.go:224-231) has no verification: the sole existing test (`TestConfigShow_ReviewerCountNoteWhenPanelDiffers`) runs GATES-ABSENT, where R7 mandates 109-identical behavior, so it passes whether or not Bead 3 step 2 is done — a skipped step 2 leaves the exact spurious-note regression R7 exists to kill LIVE in `config show` with every Verification green. Add a **gates-configured panel-scan case** to the `cmd/mindspec` config-show test in Bead 3's Verification.

## Nits (fix if cheap)

7a. **Cursor-start fixture ordering (F3 note):** `TestPanelGateSlots_DeterministicExpansion`'s fixture must place explicitly-lensed entries BEFORE lens-less ones where slot-index-mod-6 diverges from the cursor position, else a `lens[slot-index mod 6]` (wrong) impl passes everything listed. Note in Bead 1's Verification.
7. **F1-3 / F2 / O3 info:** the R2 doc-example proof's `grep -q 'claude-fable-5'` is a weak anchor (id also in known-model seed docs) — tighten to a more specific anchor if cheap; the "R1–R5 → Bead 1" summary line should note R5's reporting half lands in Bead 3; the architecture.md overlap (B2+B3) is fine (rides the real edge).

## Re-verification plan
After the fix: O1 (the RC voter) re-checks all three must-fixes ADDRESSED; O3 (bead-scope changed by fix #1) + F1 (its jq/coverage findings) confirm no regression. The six other approvers' lenses are unaffected by these coordination edits.
