# spec-111-approve — Round 2 Consolidation (tally record)

**Reviewed SHA**: bd898dfadfd1559939317d500c95927100af23d3
**Tally**: 5 APPROVE (R1 0.90, R2 0.93, R3 0.85, R4 0.86, R6 0.86) / 1 REQUEST_CHANGES (R5 0.84) / 0 REJECT. No hard_blocks.
**Decision**: PASS (≥ N−1 APPROVE, no REJECT) → `mindspec spec approve 111-workflow-panel-runner`.

## Round-1 asks — final disposition

All eight consolidated round-1 asks graded ADDRESSED by their raising reviewers. R3 (the round-1 REJECTer) re-attacked the anti-laundering ladder on four surfaces (wrapper chain-of-custody, re-prompt deviation binding, partial-render boundary, ALLOWED_CLI indirection) and found each either inside ADR-0037 §8's explicitly accepted no-signing trust boundary or covered by behavior-anchored falsified clauses checkable at bead review with the persisted `.codex.log` as evidence.

## Carry-forward to plan phase (non-blocking)

1. **[R5, the sole RC] AC4 exact-set gap** — newly introduced by the remediation: R5's requirement text claims ALLOWED_CLI admits *exactly* four commands, but AC4 only proves the four are present and `mindspec complete` is absent; a fifth admitted command would pass the AC while falsifying the requirement. Plan-time resolution: the bead implementing the workflow adds a static exact-set check (extract the ALLOWED_CLI declaration, compare to the four-element list, fail on any extra) as a test — ACs are floors, so this strengthens without a spec edit. If 111's pre-plan-approve rebase-forward touches spec.md anyway, tightening AC4's one-liner then is also acceptable.
2. **[R3 nit] Manual proof** could also assert verdict-VALUE fidelity across the re-serialize step (re-emitted verdict equals the rendered one, not just same reviewer_id).
3. **[R3 nit] R5's falsified clause** could name dynamic command construction (`mindspec ${VERB}`) explicitly as an indirection defeat.
4. **[R1/R2 info] `../../adr/` relative-link convention** in the mixed layout — pre-existing since round 1, not a regression; already known repo-wide.
