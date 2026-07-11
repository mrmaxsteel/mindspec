# spec-115-bead3 — Round 1 consolidated tally

**Reviewed**: `bead/mindspec-fgmg.3` @ `1733886e`. **Panel**: 8 slots (R7 = Opus-sub for quota-walled Fable). **Threshold**: 8/8 UNANIMOUS.

## Verdicts — 7 APPROVE / 1 REQUEST_CHANGES
R1 Opus APPROVE · R2 Opus APPROVE (ADR-accuracy — read as honest/residual-disclosed) · R3 Opus APPROVE (AC10 RED-on-revert — tested narrower/sibling globs) · R4 Sonnet APPROVE (empirical, all green) · R5 Sonnet APPROVE (skill consistency) · R6 Sonnet APPROVE (no-regression) · R7 Opus-sub APPROVE (grounding — read as grounded/residual-disclosed) · **R8 codex REQUEST_CHANGES (0.99)**.

The 7 Claude slots read the docs holistically (residual IS disclosed → honest) and tested the standard AC10 revert cases. R8's adversarial depth caught two real gaps the others missed. **Both confirmed legitimate by orchestrator against the actual artifacts** — findings never out-voted.

## Finding 1 (R8) — doc OVERCLAIM / internal contradiction (confirmed)
The ADR-0037 amendment opening states impl approve "REFUSES … while any closed bead … lacks proof of panel settlement — it was closed without `mindspec complete`, or … a durable refutation obligation not covered." A raw-`git merge`d-then-`bd close`d bead WAS closed without `mindspec complete`, so the opening's set includes it — but the gate CANNOT detect it (it's a merged ancestor → no orphan; no durable obligation → R3 blind). The amendment's own residual paragraph then carves exactly that out → the unqualified opening contradicts the disclosed residual. The 3 skill surfaces carry the same unqualified "REFUSES" sentence. Also: "after which `impl approve` finalizes" overclaims — impl approve's OTHER gates (ADR-divergence, doc-sync, phase) may still refuse.
**For a P1 fail-closed spec's CONTRACT docs, the opening must be scoped to the DETECTABLE guarantee, not walked back later.** R2/R7's "residual disclosed = honest" is defensible but R8's precision reading wins for a contract.

## Finding 2 (R8) — AC10 test GAMEABLE (confirmed; R3 missed it)
`claimsLifecyclePackage` matches only `internal/lifecycle`, `internal/lifecycle/**`, or `internal/lifecycle/`-prefixed globs. A BROADER glob such as `internal/**` from another domain would ALSO claim `internal/lifecycle` files but does NOT prefix-match → the test would not count it → the "exactly one claimant" invariant is not truly pinned. R3 tested narrower/sibling globs (`internal/lifecycleX`) but not broader ones.
**Orchestrator verified safe to fix:** every domain OWNERSHIP.yaml today uses specific `internal/<pkg>/**` globs — NO broad `internal/**` exists — so switching to overlap-based matching still passes at HEAD and now catches a future broader-glob claimant.

## Fix (round 2, then re-panel)
**Finding 1 — doc precision (ADR + 3 skill surfaces):**
- Qualify the "REFUSES … closed without `mindspec complete`" claim to the DETECTABLE sense: refuses a closed bead whose `bead/<id>` branch is still an unmerged non-ancestor (closed without `mindspec complete`), OR that carries a durable uncovered `refutation_pending` obligation — so the opening matches what the gate detects and no longer includes the disclosed raw-merged-no-obligation residual. Keep the residual disclosure.
- Soften "after which `impl approve` finalizes" → the orphan/obligation gate no longer blocks; `impl approve` proceeds subject to its remaining gates.
- Apply the same qualification to the "REFUSES" sentence in all three skill surfaces (content-consistent); the skills may briefly note the raw-merged residual or simply not overclaim.

**Finding 2 — AC10 overlap matching (`internal/lifecycle/ownership_test.go`):**
- Replace `claimsLifecyclePackage`'s string-prefix heuristic with OVERLAP matching: a domain claims the package if any of its globs would match a representative `internal/lifecycle` file (e.g. `internal/lifecycle/orphans.go`). Prefer the repo's own ownership glob-matcher (ADR-0036 ownership discovery; see `internal/ownership`) or `doublestar`-style matching so the test's notion of "claims" matches production glob semantics — NOT a hand-rolled prefix.
- Add RED subtests (in a throwaway assertion or table): (a) a BROADER second-domain claim `internal/**` → detected as a 2nd claimant → count 2 → FAIL; (b) an exact-duplicate `internal/lifecycle/**` in a 2nd domain → FAIL; (c) workflow claim removed → FAIL. Confirm the test still PASSES at HEAD (only workflow overlaps).

Keep the fix confined to the ADR + 3 skill surfaces + `ownership_test.go`. One commit; full gates green; `validate spec` passes.
