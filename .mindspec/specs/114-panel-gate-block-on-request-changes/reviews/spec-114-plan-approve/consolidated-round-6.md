# spec-114-plan-approve — PASS (effective 9/9 after 6 rounds)

Round 1: 5A/4RC (AC coverage, AC1/AC5 contradiction, best-effort-audit). Round 2 (durable audit v1): 7A/2RC (TOCTOU, MergeMetadata erasure, dedup). Round 3 (durable obligation): findings on GetMetadata-fail-closed, applied-before-persist, reconcile-every-path. Round 4 (applied=persisted): union-loss, discharge-evidence, hatch. Round 5 (total reconcile): 8A/1RC. Round 6 (precision): F2 → APPROVE. **Effective 9/9.**

The mechanism (confirmed TOTAL by O1 + all 3 gpt-5.6-sol codex): unresolved-RC/unrecognized-verdict BLOCKS (leg 9.5, layered on the threshold floor); a refutation clears an RC only once its refutation_pending marker is durably UNIONED into bead metadata (applied≡persisted, fail-closed → BLOCK on failure); reconcilePendingRefutations satisfies/verified-discharges/refuses the FULL unioned set on EVERY completion path (panel-present, no-panel, hatch) before close, re-tallying from panelReg.Dir for discharge evidence (fail-closed refuse if unavailable); panel_refuted + refutation_discharged durable; MergeMetadata + GetMetadata fail-closed. ADR-0037 §7/§8 amended (operator-authorized narrow audit-durability carve-out). Spec Impacted Domains += execution (internal/bead). 3 beads serial A→B→C.

Net invariant: a refutation that ever durably clears an RC ⟹ audited (satisfied) or verified-discharged, across all retries AND all paths (incl. hatches). No un-audited-merge or lost-obligation path survives.

## For the bead briefs (non-blocking nits folded)
- F2r6-4: synthetic refutation_discharged reason wording under disjunct(i) hatch re-RC = supersession not "resolved naturally" — wording.
- Bead 2 is heavy (internal/panel + internal/complete + internal/bead; complete with --override-adr NOT needed since execution now declared). Reviewer/fixer discipline: ABSOLUTE /tmp scratch, absolute verdict paths, CI-matched gofmt (go 1.23.0; avoid backtick doc-comment code-spans with shell-escapes).
