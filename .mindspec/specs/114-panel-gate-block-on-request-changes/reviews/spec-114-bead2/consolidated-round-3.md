# spec-114-bead2 round-3 → REDESIGN (operator decision: delete the discharge path)

Round-3 tally: 6 APPROVE, 1 REQUEST_CHANGES (G1 codex 0.99, third distinct false-discharge variant — basename-cross-root aliasing), 1 F1 APPROVE that filed the SAME issue as an "accepted fs-trust-boundary" info note. Three review rounds each surfaced a deeper hole in the **discharge-by-re-tally** mechanism (reconciliation re-reading mutable panel files to "verify" a finding resolved naturally). 

**OPERATOR DECISION (Max, 2026-07-10): DELETE the discharge path entirely.** Rationale: any re-tally-based discharge trusts mutable on-disk panel files, so it has an irreducible spoofing surface (whack-a-mole). Removing it eliminates the attack surface, and the honest `panel_refuted` (reason+evidence) record is strictly better provenance than a "resolved naturally" claim. This DEVIATES from the plan's approved discharge mechanism (plan step 5c / AC11 verified-discharge) — the plan + ADR-0037 are amended to match (Bead 3 does the ADR; plan Bead-2/AC11 updated on the spec branch).

## The redesign — self-contained marker; satisfy-always; no discharge

**Keep the two-phase durable obligation** (it provides the hatch/crash safety): the `refutation_pending` marker is still written as part of the allow-decision, and reconciliation still runs before close on every path. What CHANGES:

1. **Marker becomes SELF-CONTAINED.** Each `refutation_pending` entry now carries the FULL refutation content: `{slot, round, reason, evidence}` (persisted at apply time from the panel.json refutations entry). DROP the `panels`/origin field — it existed only to verify discharge against a panel, and there is no discharge anymore.
2. **Reconciliation = flush pending → `panel_refuted`, from the marker.** For every recorded `refutation_pending` entry NOT already covered by a durable `panel_refuted` (the already-covered no-op stays), write `panel_refuted` (slot, round, reason, evidence, timestamp) FROM THE MARKER — no re-tally, no panel read, no origin matching. This works uniformly on panel-present, no-panel, and hatch paths, and even after `panel.json` is removed/changed, because the marker is self-contained. Fail-closed: a `panel_refuted` write failure fails completion pre-close (bead stays `in_progress`); a `GetMetadata` read/parse error → Refuse; a malformed marker → Refuse.
3. **DELETE entirely**: `dischargeVerified`, `dischargeEvidence`, the `refutation_discharged` audit key + writer, the re-tally of `panelReg.Dir`/`ForBead` inside reconcile, and all origin-presence/`Panels` logic. Reconciliation no longer reads any panel directory. (panelGate still scans panels for the GATE decision — that's unchanged; only the reconciliation's discharge re-tally is removed.)
4. **Natural resolution (AC4b) still holds, trivially and by construction.** A re-panel where a dissenter flips to APPROVE and NO refutation was ever applied creates NO marker → NO obligation → completes with zero ceremony. It never needed discharge; discharge only ever existed to handle "an obligation exists but the finding later resolved," which self-contained-satisfy now records honestly as `panel_refuted` (a refutation WAS applied — recording it is truthful).

## Why this closes everything the panel found
- Round-1 (first-panel-only re-tally), round-2 (origin-unbound), round-3 (basename-cross-root alias) were ALL discharge-by-re-tally identity bugs. With no re-tally and no discharge, none can exist. An operator deleting/corrupting/aliasing panels can no longer manufacture a "resolved naturally" outcome, because that outcome no longer exists — the obligation is always settled as the honest `panel_refuted` from the marker.
- The only remaining fail-closed guards are metadata-integrity ones (write-fail → Block, read-fail/malformed → Refuse), which don't trust panel files.

## What to KEEP (do not regress — the confirmed guards)
- **applied ≡ durably-persisted**: the marker write is part of panelGate's allow-with-refutations decision; read-then-UNION-then-write (never bare replace); read OR write failure ⟹ the refutation is NOT applied ⟹ Block (RC unresolved), not abort-with-applied.
- **reconcile-before-close on EVERY path** incl. hatches (`MINDSPEC_SKIP_PANEL`, `enforcement.panel_gate:false`): a pre-existing pending obligation is satisfied (from marker) before close even on a hatch — hatches except the GATE, not the obligation.
- **already-covered no-op** (a pending already covered by a durable `panel_refuted` → skip; (slot,round)-exact).
- **fail-closed `bead.MergeMetadata` + `GetMetadata`** (read/parse error returns, no erase-from-empty).
- **malformed marker/entries ⟹ Refuse** (fail-closed decode).
- **anti-gaming in `internal/panel`** (tally.go/gate.go): refute clears only canonical latest-round RC; never REJECT/hard_block/unrecognized/other-slot/newer-round; `Approves` never incremented (AC7/AC9); AC12 byte-exact slot + dedup. These are byte-unchanged.

## Tests
- REMOVE the discharge-specific tests: `TestPanelRefuted_CrossRun_NaturalResolution_Discharges`, `TestPanelRefuted_WarnPathDoesNotDischarge`, `TestPanelRefuted_CrossPanel_NoFalseDischarge`, `TestPanelRefuted_OriginRemoved_Refuses`, `TestPanelRefuted_OriginCorrupted_Refuses` (they test a mechanism that no longer exists).
- ADD/REWRITE for the new invariant:
  - `TestPanelRefuted_SatisfyFromMarker_PanelGone` — run 1 applies refutation X@1 (marker persisted with reason/evidence), `panel_refuted` write fails → `in_progress`; run 2 REMOVE `panel.json` entirely, retry → reconciliation writes `panel_refuted(X@1, reason, evidence)` from the marker and completes+merges (the honest audit lands with NO panel present). This is the case that used to falsely-discharge; it now correctly SATISFIES.
  - `TestPanelRefuted_NaturalResolution_NoObligation` — a completion where NO refutation was applied (dissenter flipped on re-panel) writes NO marker and completes with zero ceremony.
  - `TestPanelRefuted_MarkerCarriesReasonEvidence` — the persisted marker round-trips slot/round/reason/evidence; `panel_refuted` written from it carries them.
  - Keep + confirm still green: marker-write-fail ⟹ Block; union-not-replace; already-covered no-op; malformed ⟹ Refuse; GetMetadata-error ⟹ Refuse; hatch-still-reconciles (now: satisfies pending from marker); MergeMetadata fail-closed; the whole `internal/panel` `TestPanelGateDecision_Refutations`/`AppliedRefutations`/AC7/8/9/12 suite.
- Mutation-proof: making the reconciliation swallow the `panel_refuted` write error (so an obligation could merge un-audited) must turn a named test RED.

## Constraints
- Scope: `internal/complete/*` (+ tests) primarily; the marker schema change is complete-side. `internal/panel` tally/gate stay byte-unchanged (anti-gaming). `internal/bead` fail-closed metadata stays. NO ADR edits in THIS bead (Bead 3 amends ADR-0037's discharge language). Plan.md is updated separately by the orchestrator on the spec branch.
- Full plan Verification checklist + the rewritten tests green; `gofmt -l ./cmd ./internal` empty (go 1.23.0 CI; no backtick doc-comment shell-escape spans); `go test -race` on panel/complete/bead green.
- ABSOLUTE /tmp scratch; never touch a sibling worktree. EXACTLY ONE commit on `bead/mindspec-mvp8.2`. No push/merge/complete.
