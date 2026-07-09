# spec-112-approve — round 1 consolidated changes

**Tally: 6 APPROVE (G1,O1,O2,O3,F1,F2) / 3 REQUEST_CHANGES (G2,G3,F3) / 0 REJECT — below 8/9 → FIX ROUND.** All findings are one-sentence-scale spec tightenings; F3 explicitly "none architectural." Deduped + ranked. Reviewed SHA 825f04c5.

## Must-fix (convergent across ≥2 reviewers)

1. **Pin cursor start = 0 (F3-1, F1, O3-5 — THREE lenses).** R3's falsification clauses + the slot-expansion AC are all rotation-invariant: an impl starting the interleaved cursor at index 1–5 passes every proof yet contradicts the worked example. Add to R3 a falsification predicate + an AC assertion that the cursor starts at index 0 (first lens-less slot in declaration order = author-of-record), so the worked example is pinned, not just illustrated.

2. **R7 recorded-but-unknown gate value (O2 residual, O3-3, F3-6 — THREE lenses).** R6 is parse-lenient (permits an unexpected recorded `gate`), but R7 only maps known-gate and no-recorded-gate; the recorded-but-unknown case would call R3's resolver which ERRORS on unknown gates. Specify: an unknown recorded `gate` value is treated by the advisory path as "no known gate" → the note is skipped (decision-inert, never propagates the resolver error). Add a falsification clause + AC.

3. **Require control-byte escaping for the new string fields (G2 — codex security).** The spec must REQUIRE the 109 `escapeConfigValue`-equivalent for every config-controlled string rendered by `config show` / `config show --gate` human-text paths: `note`, reviewer `model`, reviewer `lens`, `substitutes` keys/values, and any known-model warning embedding them. Pin `config show --gate --json` to a real JSON encoder (never hand-built). Add AC coverage with hostile control-bytes/newlines in note/model/lens/substitutes proving no raw control bytes + no forged lines in text output, and that they round-trip as JSON strings in `--json`.

4. **Pin map render order (O3-1, ties to G3).** `gates` and `substitutes` are Go maps; `config show` + `--gate --json` must render deterministically (gates → the R1 enum order; substitutes → sorted keys) so the `--json` contract 110/111 consume is reproducible and the AC greps don't flake. Add to R8 + its AC.

## Should-fix

5. **Pin the exposed downstream contract (G3), without importing 110/111's requirements.** 112 must firmly pin the STABILITY of the surface it exposes — the `config show --gate <name> --json` shape (slots: slot/model/lens; expected count; raw threshold expr; effective substitution) and the recorded `gate` field — as the contract 110's writer and 111's runner will consume. Add a forward-compat requirement/AC that the `--json` schema is stable + documented. Do NOT add requirements that BELONG to 110 (the writer) or 111 (the runner) — that's their scope; 112 only guarantees the contract.

6. **R7 skip carve-out needs an AC + falsification (F1).** The gates-configured + non-bead + no-recorded-gate → skip behavior (the exact spurious-note regression R7 exists to kill) has neither. Add a 4th `TestPanelAdvisory_GateAwareCompare` case.

7. **adhoc partial-fallback + cross-field inheritance hole (F3-2, ties O3).** (a) State that `adhoc`-with-reviewers-only / threshold-only inherits the missing half PER FIELD through bead→global (same as any gate), removing the two-readings ambiguity. (b) F3 constructed a config where a `bead`-configured integer threshold, inherited by a smaller `adhoc` gate, escapes R4's range refusals → loadable but never-passable. Tighten R4's cross-field inheritance check to validate an inherited integer threshold against the INHERITING gate's resolved sum across the full adhoc→bead→global chain.

8. **Size-cap gap (G2).** Explicitly acknowledge and RESOLVE: either define maxima for expanded reviewer slots / count / individual open-string lengths, OR document the deferred cap decision with a named follow-up + a non-accidental rationale. **Recommended: defer with rationale** (inert-render surface, local-repo only; fold into the `naq0`/`pev1` follow-up class) — state it explicitly rather than leave it silent.

## Nits (fix if cheap)

9. **F2-1: name the count relaxation.** R2/AC1's "byte-identical" wording should acknowledge that a count-less legacy entry (109 REFUSES: count must be ≥1) LOADS as count=1 under R1 — a monotone relaxation, not a break — so a too-literal AC1 impl can't contradict R1.
10. **gates: {} vs absent keys off len>0 (O3-2, F3-5)** — R7's "when gates: is configured" and all "configured" checks key off `len(gates) > 0`, not key-presence, so an empty map behaves as absent (R2 identity).
11. **Unknown-gate recovery line enumerates the 5 valid keys (O3-3)** — to disambiguate from `loop.gate_authority`'s `bead_merge`/`impl_approve`.
12. **F1 lower-severity: add paired AC assertions** for note-inertness (R1), the substitutes supersession-precedence flip (R5), and a negative control for the four seeded known-model ids (AC8).

## Orchestration (NOT a spec-text change — orchestrator action)

- **Rebase 112 onto post-109 main before `spec approve`/planning (O1, F2).** The 112 branch predates the 109 merge; 109's ADR-0040/config surface aren't in this worktree (reviewers read them via `git show main:…`). The spec honestly documents this as a hard prerequisite — do the rebase-forward before the approval-driven bead creation / plan work. Fix the BRIEF's stale "109's surface is in this worktree" grounding for round 2.
