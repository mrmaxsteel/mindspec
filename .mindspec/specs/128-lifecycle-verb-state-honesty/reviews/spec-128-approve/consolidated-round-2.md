# spec-128-approve — consolidated concrete_changes_required, round 2

Tally: 2 APPROVE (R2, R3) / 4 REQUEST_CHANGES (R1, R4, R5, R6). 9 RC asks + 4 R3-advisory items (R3 approved, but per findings-never-outvoted its asks are folded in, not dropped): 12 distinct items in 11 groups. R3-1 merges into G7 (convergent with R6). Artifact-gate screen: clear.

## G1 (R4) — The written bd-failure reproduction command is self-contradictory — MUST FIX (mechanical, but it is the spec's own falsifiability surface)
`env PATH=/usr/bin:/bin mindspec state show` cannot invoke `/Users/Max/.local/bin/mindspec` (not on that PATH). Correct every occurrence in settled-fact 6, the #220 live-evidence paragraph, and Validation Proofs to invoke MindSpec absolutely while failing only bd discovery: `env PATH=/usr/bin:/bin /Users/Max/.local/bin/mindspec state show` (or a failing-bd PATH shim + absolute mindspec). Remove/qualify the same-binary claim as written.

## G2 (R1) — `release` breaks the R1.3 table's release→plan row under BOTH OQ-3 options — MUST FIX
Derived at reviewed commit: release's step-6.5 sync calls stored-trusting `phase.DerivePhase` (release.go:315-318; derive.go:174-193), so releasing the sole claimed bead back to all-open leaves stored `implement` vs derived `plan` under both OQ-3 options — the row's three-surface agreement on `plan` is unsatisfiable without (a) extending the plan-approve OQ-3 carve-out to this row, or (b) an explicit requirement redesigning release's phase sync. Rider: note in R3.1/R3.2 that this release-produced state is a SECOND deterministic reproduction route into the #248 warning window (release's own DerivePhase call emits the repair-phase hint on the disagreement it preserves) — the R3 fixture coverage statement must include it, not be plan-approve-shaped only.

## G3 (R5) — ADR-0025 amendment must read as MANDATORY everywhere — MUST FIX (mechanical)
R5.1/R5.3 already establish every admissible direction amends ADR-0025 (no no-amendment leg). Replace the surviving "conditional R5 clarification"/"priced conditional amendment" wording at spec.md lines 80, 87, 154 so ADR Touchpoints and Scope say the same thing.

## G4 (R5) — R6.4/AC-11: pin ADR-0042 rendering, not just content passthrough — MUST FIX
Extend to inject multi-line bd stderr including at least one control-bearing/termsafe-sensitive line: legitimate schema-error text survives; each surfaced line routes through `termsafe.Escape`; no raw control byte reaches the rendered error. (Current benign assertion stays green under an unsafe raw-passthrough implementation.)

## G5 (R6) — Positional `--emit-only` vs the every-path filter-loudness claims — MUST FIX (coherence)
R6.1 pins positional emit-only as a readiness-bypassing preview; R6.3(a) claims every path reports exclusions. Make them coherent: either positional emit-only participates in raw-ready/filter construction and reports exclusions, or R6.3's every-path claim explicitly excludes that combination and pins its honest bypass message. Add a test: positional `--emit-only` with `--spec` and a title-unreferenced ready bead.

## G6 (R6) — Close the ACTUAL repair race (last-read → MergeMetadata) — MUST FIX
Round-1 G9's re-derive-before-write only narrows the window. Required: a concurrent `complete` advancing stored phase to `review` after repair's final derivation but before its metadata write must not be overwritten with stale `implement`. Price a real mechanism (conditional write / serialization primitive / other) that makes the invariant hold; add the concurrent-complete test leg.

## G7 (R6 + R3 convergent) — AC-7(iii) and AC-6 must be conditional on OQ-1(c) — MUST FIX (coherence with round-1 G3.1)
Under OQ-1(a) reorder or (b) rollback there is no exemption, so an independent named-artifact-only commit must NOT be asserted exempt "under all mechanisms". Tag AC-7(iii) AND AC-6's diff-pathspec sentence explicitly "under OQ-1(c) only" (R3's phrasing), reconciling with OQ-1's "ACs hold under any of (a)/(b)/(c)" claim; and if (c) is selected, explicitly adjudicate the review-integrity cost of exempting any actor's post-review tracker commit (accepted cost with rationale, not forced consequence).

## G8 (R6) — Explicit `--spec` scope-resolution honesty — MUST FIX
An explicit `--spec` whose epic lookup fails or is absent must refuse with the real scope-resolution error — never silently fall back to global `bd ready`. Pin on claim AND emit-only paths, including a full spec ID (ResolveSpecPrefix currently passes it through without proving existence).

## G9 (R6) — Forced backward repair's success OUTPUT names the reason — MUST FIX (small)
Add an acceptance assertion that the forced-repair success line itself names the recorded reason (matching R2.1/R3.3), not only that the reason is stored.

## G10 (R3) — Release row needs a tail-less carve-out; AC-14 anti-drift second arm is hollow — MUST FIX
(a) R2.1 records `release` emits no instruct tail, but R1.3's table contract has every row's fixture assert three surfaces including the tail's selected template — unrunnable for a tail-less verb. State the release row's tail-less carve-out the way the plan-approve carve-out is stated (interlocks G2). (b) AC-14's second permitted anti-drift arm ("count sentinel plus in-diff extension obligation") cannot go red when a new durable-state-mutating verb is added elsewhere — a hardcoded list and its count stay untouched, and the obligation is process hope, not a failing test. Drop the second arm or mechanize it; keep only enumeration arms that actually fail on omission.

## G11 (R3) — AC-4 clause scoping — MUST FIX (small; round-1 MINOR never consolidated, still live)
One scoping sentence: the agreement-with-`Result.NextMode` assertion applies exactly when context resolution succeeds; the indeterminate-marker branch exactly when it does not. As written, a tail that ALWAYS renders the indeterminate marker arguably satisfies both clauses.
