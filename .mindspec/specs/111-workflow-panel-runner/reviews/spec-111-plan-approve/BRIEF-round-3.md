# spec-111-plan-approve — Round 3 (second fix re-verification, 9 reviewers)

**Under review**: plan.md @ **631d7c14** (round-2 fix on bf01a21a; +143/−34, plan.md only) in worktree `/Users/Max/replit/mindspec/.worktrees/worktree-spec-111-workflow-panel-runner`. Delta: `git diff bf01a21a..631d7c14 -- .mindspec/specs/111-workflow-panel-runner/plan.md`
**Round-2 tally**: 5 APPROVE (F2, O1, O2, O3, G3) / 4 RC (F1, F3, G1, G2). Consolidated: `consolidated-round-2.md` (5 items).
**Pass = >=8 APPROVE, no REJECT.** READ-ONLY rule unchanged.

## What the round-2 fix did

1. (G1/G2/F3) malformed-once shim: exact verbatim payload pair — first = ONE schema-valid verdict object WRAPPED in narrative (rejected by exactly-one-object rule, canonically decodable for the fidelity diff on verdict/hard_block/concrete_changes_required); second = clean re-serialize. malformed-always stays pure-narrative (MISSING-path-only check).
2. (F1) `const [CMD_PANEL_CREATE, CMD_CODEX_EXEC, CMD_PANEL_VERIFY, CMD_PANEL_TALLY] = ALLOWED_CLI;` — ALL call sites use the identifiers, never literals; Step 6's test pins "exactly four identifiers destructured in one binding"; buildCommand gains a leading-dash argument-injection guard; the exact-match bypass reading is explicitly closed (identifier-count pin named as the mechanism).
3. (G2) `target` validated against a branch-name-safe grammar (git check-ref-format constructs, control bytes, leading dash), passed as its own argv token; e2e scenario 7 expanded to 7a slug-traversal / 7b target argument-injection+metachar / 7c target control-byte (escape-notation in the doc).
4. (F3) e2e Setup seeds the scratch config's `panel:` block matched to each run's mix (checked against the real internal/config schema; no gates: block — 111 reads none of 112's surface).
5. Folded into 2.

`validate plan` passes (advisory WARN unchanged). File verified clean UTF-8, zero control bytes.

## Round-3 jobs

- **F1, F3, G1, G2 (round-2 RC voters)**: disposition YOUR round-2 asks ADDRESSED/PARTIAL/MISSED/NEW_ISSUE against 631d7c14. F1: do the destructured identifiers + count pin + dash guard now make the prescribed code pass the prescribed test, with no remaining literal/concat bypass? F3: is the fidelity comparison now runnable on a real operand, and does the seeded panel: block let runs 1/3 discriminate? G1: exact payload shapes now specified and consistent? G2: target grammar + scenarios 7a-c close your injection surface?
- **F2, O1, O2, O3, G3 (approvers)**: confirm the delta introduces no regression in your lens (one-line check each; O2: steps still ≤7 — Bead 2 is at 6).

Verdict → `<slot>-round-3.json` in this dir. Keys as before; RC voters include per-item dispositions.
