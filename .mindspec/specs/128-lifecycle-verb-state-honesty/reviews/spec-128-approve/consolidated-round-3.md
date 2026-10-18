# spec-128-approve — consolidated concrete_changes_required, round 3

Tally: 3 APPROVE (R3, R4, R5 — R4 and R5 with ZERO asks) / 3 REQUEST_CHANGES (R1, R2, R6). 7 RC asks + 3 R3-advisories + 1 re-raised previously-dropped R2 round-2 ask = 11 raw, deduped to 5 groups. Artifact-gate screen: clear.

**Process note for the audit trail**: R2's round-2 asks (raised on an APPROVE verdict) were dropped at the round-2 consolidation — the orchestrator read only the RC slots' and R3's asks that round. One of the two (the `:141` citation) was never fixed and is re-raised here as G2. Consolidation rule now standing: EVERY slot's concrete_changes_required is read every round, APPROVE verdicts included.

## G1 — mindspec_phase writer enumeration is INCOMPLETE; lock scope and fences must match the true write-site set (R1 ×2, R2, R6, R3-adv — five asks, strongest convergence of the whole gate) — MUST FIX
Independently derived by three slots at 9ac301ee, the true writer set beyond R3.3's current enumeration (plan.go:455, complete step-6.5, release.go:315-318, "the impl approve/impl adopt terminal write"):
- internal/approve/impl.go: **:578** reconcile-to-derived write (non-terminal), **:612** `done` write, plus a pre-080 backfill path near **:274** — "the terminal write" is at least three sites;
- internal/approve/adopt.go:**424**;
- internal/approve/spec.go:**76** existing-epic merge-write of `plan` (creation seed at :84);
- internal/phase/migrate.go:**60** `EnsureMigrated` — the ADR-0034 migration writer that runs AHEAD of every preflight (ADR-0041 exemption) and writes exactly when stored phase is absent.
Fix, all four faces:
1. Correct R3.3's enumeration to the true write-site set — or reword so completeness is NOT asserted prose-side and is instead delivered by the mechanism in (2). An unenumerated writer silently bypasses the lock and reopens the race AC-5(v) claims closed.
2. Pin the enumeration the way this spec pins every other drift-prone list (R5.1 enumeration test, AC-14 total classification): require a seam- or grep-anchored test asserting every MergeMetadata call site writing `mindspec_phase` sits inside the lock's critical section (or an explicit plan-time re-derive instruction if the test shape is a plan decision — but the anti-drift obligation itself is spec-side).
3. Propagate into the scope fences: In Scope — plan.go's entry becomes unconditional (the lock obligation holds regardless of OQ-3; the OQ-3 conditional governs only the WRITTEN VALUE), add internal/approve/impl.go, internal/approve/adopt.go, internal/approve/spec.go, internal/phase/migrate.go; Impacted Domains — update the internal/approve/** parenthetical and add the migrate surface under core.
4. Extend AC-5 with (a) the inventory/anti-drift assertion and (b) a legacy-migration contention leg: an unlocked stale derived value (via EnsureMigrated's path) cannot land after a fresher locked write.

## G2 — ADR-0023 citation off-by-one, re-raised (R2; dropped at round-2 consolidation) — MUST FIX (mechanical)
The all-open→plan amendment row is at `ADR-0023.md:141`, not `:142` (:142 is blank). Fix the release row's "no sibling closed" citation and the grill-record parenthetical ("the `:142` first cited was the amendment's all-open row" → `:141`).

## G3 — AC-10 must carry R6.3's positional `--emit-only` exception (R6) — MUST FIX
AC-10 as written still implies every-path exclusion reporting. Make it explicit: positional `--emit-only` performs no ready query/filter, reports no exclusions, and must print only the pinned readiness-bypass disclosure; displayed-list wording applies only to paths that actually display a list.

## G4 — AC-2 pins the release-row carve-outs (R6 + R3-adv convergent) — MUST FIX
AC-2's carve-out parenthetical names only the plan-approve row. Extend it and the fixture-alternative branch: the release row asserts two surfaces only (no instruct-tail assertion), and its plan leg asserts the documented stored-`implement`/derived-`plan` pair under both OQ-3 options unless the sanctioned release-sync redesign leg is selected.

## G5 — Sanction-boundary sentence for the redesign leg (R3-adv) — MUST FIX (one sentence + AC clause)
If the plan takes R1.3's release-sync redesign leg (sanctioned internal caller performing a guard-routed backward write), pin the sanction boundary: the operator `repair phase` path still refuses backward without force+reason; the sanctioned internal caller is the only unforced backward writer, and the AC distinguishes the two.
