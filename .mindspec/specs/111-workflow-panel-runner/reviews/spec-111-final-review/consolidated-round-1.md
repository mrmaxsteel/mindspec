# spec-111-final-review — disposition

**Tally: 11 APPROVE / 1 REQUEST_CHANGES (G3, declassified→follow-up) / 0 REJECT — PASS (≥11/12).**
Family split: Fable 3/3 (F1 0.9, F2 0.93, F3 0.93), Opus 3/3 (O1 0.9, O2 0.9, O3 0.95), Sonnet 3/3 (S1 0.91, S2 0.85, S3 0.93), **codex 2/3 REAL codex** (G1 0.94 APPROVE, G2 0.86 APPROVE, G3 0.86 RC-declassified). Codex was back and used for the final review per operator instruction (beads used Sonnet-subs; final reviews get real codex).

The whole spec verified end-to-end: both outcomes delivered (the tracked `/ms-panel` workflow adapter; `ms-panel-run` shrinking to lens-composition + one workflow invocation on the workflow path, skills-path retained as default). All R1–R9 falsification clauses walked (O1/F1). Cross-bead coherence (F2 — ownership→workflow→dispatch, the round-1 `claude_sub_on_quota` drift confirmed fixed). ADR compliance (O2 — ADR-0040 portability adapter with degraded modes, ADR-0037 single home unchanged, 0035/0036). Scope/boundaries all hold (O3 — gate.go/complete zero-diff, config-free leaf, no 2nd decision authority, `mindspec complete` nowhere, runner default skills). End-to-end empirical (S1/G3 — built the branch binary with 110's verbs, ran the registration→verify→tally chain, install Claude-target-only). Test quality mutation-verified (S2). No regressions (S3 — 2 known failures pre-existing by zero-source-diff).

**The security surface is genuinely CLOSED** — the RCE-class shell-injection fixed during 9cyu.2's bead review holds: F3 ran 49 assertions (every payload throws), G1 (real codex) 0.94, S1 + S2 independently corroborated; the exact-set test mutation-proved to bite; `mindspec complete` appears nowhere; the anti-laundering ladder fails closed.

## The one RC — G3 (codex integration): DECLASSIFIED to follow-up `mindspec-<filed>` (a P2 bug), NOT a 111 defect
**Finding**: `panel verify`/`panel tally` mishandle a NON-BEAD panel (bead_id null — spec/final-review/PR panels the `/ms-panel` workflow can create): the fact resolution guards bead-ref rev-parse on `reg.Panel.IsBead()` (cmd/mindspec/panel.go:304), so for a non-bead panel `beadID` stays empty → a non-bead panel keeps reporting PASS advisory after its `--target` advances, with a bogus "references branch bead/" message. Empirically reproduced by G3 against the branch binary.
**Disposition — declassified with scope citation:**
1. **`cmd/mindspec/panel.go` is spec 110's code (fbel.4), already merged to main; 111's diff against it is EMPTY** (`git diff origin/main...4e8a1f45 -- cmd/mindspec/panel.go` = nothing). 111 does not own the panel verbs.
2. **111's workflow adapter is CORRECT** — it faithfully returns whatever `panel verify`/`tally` produce, which is precisely ADR-0040's principle (integrate at the artifact+CLI contract; don't re-implement or second-guess the verb). O3/F3/G3 all confirmed the adapter is not a decision authority. Fixing this in 111's scope would require touching 110's verb (scope creep) or re-implementing verb logic in the workflow (violating the adapter principle).
3. **Severity**: non-bead panels are NOT gated by `mindspec complete` (bead-only), so this is an advisory/UX gap feeding the human-gated impl-approve, not an auto-merge bypass. Real and worth fixing — in 110's verbs, via the filed P2 follow-up (fix non-bead staleness to check `panel.json.target` vs `reviewed_head_sha` + the message + an e2e regression).

## Non-blocking notes (documented; follow-ups where noted)
- F1/O1: the spec's "`node --check ms-panel.js` exits 0" Validation Proof is a mis-specified oracle (the file uses top-level await/return, valid only as the workflow-runtime async-function body — parses clean via `new AsyncFunction(src)`). Explicitly labelled "not a CI gate," not an R-clause. Cosmetic.
- S2: no CI-wired Go test byte-diffs the tracked `.claude/workflows/ms-panel.js` vs the plugin copy (only the AC1 shell one-liner, not in ci.yml) — a PRE-EXISTING systemic gap shared with `.claude/skills/**`, not a 111 regression. Worth a follow-up bead for the whole mirror-distribution class.
- S2: `installWorkflows`' user-modified→notice branch untested (mirrors `installSkills`' same untested branch).
- F2: `flattenMix` maps lenses via `lenses[index % lenses.length]` (silent wrap on a short lenses array) — leniency, not drift.

## Decision: PASS → `mindspec impl approve 111-workflow-panel-runner`.
spec/111 CONTAINS main (109/110/112 merged in earlier) → the impl-approve is a clean spec-ahead-of-main merge → PR (protected-main flow, like 109/110/112).
