# spec-112-bead1 — Round 2 (fix re-verification, 8 reviewers)

**Worktree**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-112-per-gate-panel-config/.worktrees/worktree-mindspec-lma4.1
**Commit under review**: ad56c295 (fix on top of r1's 68fe576e) — delta: `git diff 68fe576e..ad56c295` (internal/config only, +103/−2).
**Round-1 tally**: 7 APPROVE / 1 RC (G1). Consolidated: `consolidated-round-1.md` in this dir.
**Pass = >=7 APPROVE, no REJECT.** READ-ONLY rule unchanged.

## What the fix did (verified present; judge sufficiency)

1. (G1.1) Audited all 8 refusal messages added by 68fe576e; the TWO embedding config-controlled strings raw in recovery clauses (unknown panel.gates key ~:574; substitutes self-map key ~:635) now route through `strconv.Quote`; the other 6 verified already-safe (enum-only or already-%q).
2. (G1.2) `TestLoad_UnknownGateKeyEscapesControlBytes`: YAML fixture key carrying real BEL/ESC/newline bytes; asserts no raw control byte in the error text, exactly one newline (the message/recovery separator — no forged physical recovery line), final line still `recovery: `-prefixed. Verified failing pre-fix.
3. (F1-1 + S2 fold-in) `cursorReset` fixture added to `TestPanelGateSlots_DeterministicExpansion`: `[lens-less, explicit(adversarial, Model+Family both set), lens-less]` — kills the cursor-reset-on-explicit wrong variant (verified failing against that patched variant) and pins Model-wins-over-Family.

## Round-2 jobs

- **G1 (RC voter)**: re-run your hostile probes against ad56c295 — the unknown-gates-key ESC/BEL/newline injection specifically (build the branch to /tmp, feed your round-1 fixtures) — and disposition your two asks ADDRESSED/PARTIAL/MISSED/NEW_ISSUE on what you OBSERVE. Also spot-check the fix's claim that the remaining 6 refusal messages are safe (enum-only or %q).
- **O1, O2, O3, S1, S2, S3, F1 (approvers)**: confirm the delta (2 files, escaping + tests only) introduces no regression in your lens. O2: full checklist still green at ad56c295. F1: your cursor-reset variant now fails the suite as intended.

Verdict → `<slot>-round-2.json` in this dir. Keys as round 1; RC voter includes per-item dispositions.
