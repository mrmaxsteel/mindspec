# spec-128-approve — consolidated concrete_changes_required, round 1

Tally: 1 APPROVE (R2) / 5 REQUEST_CHANGES (R1, R3, R4, R5, R6). 23 raw asks deduped into 10 groups below, ranked by criticality. Convergent slots noted per group. R2 (APPROVE) raised 2 advisory items — folded in where they duplicate groups.

## G1 — Main-checkout `state show` shape: CONFLICTING empirical evidence must be resolved to a discriminator (R4 ×2, R3) — MUST FIX, highest priority
The evidence is three-way inconsistent and the spec may not paper over it:
- The aytt charter recorded unqualified main-checkout `state show` → `{"mode": ""}`.
- The author, the grill, AND the orchestrator (re-run 2026-09-01 16:4x, installed CLI, cwd `/Users/Max/replit/mindspec`, no flags) each observed `{"mode": "idle"}` exit 0.
- R4 (same day, reviewed commit) observed `{"mode": ""}` exit 0 from BOTH the installed CLI and a binary built from the reviewed source, and notes the source permits it: `state.go` serializes `ctx.Phase` even when discovery failed and left it empty.
Fix:
1. Establish the discriminating variable (cwd? env? flag? concurrent worktree set?) by re-deriving BOTH shapes with exact reproduction lines (cwd, argv, relevant env) and rewrite Background settled-fact 6 / live-evidence to state both shapes and their trigger conditions. Neither "the `""` shape was not reproduced" nor an unconditional `""` claim survives the evidence.
2. Regardless of which shape wins where, R4's structural point is REAL and unpinned: the code can exit 0 with an empty mode on discovery failure, and no requirement or AC covers the main-checkout invocation at all (R1.1 only forbids idle-with-activeSpec; the main-checkout output has no activeSpec field). Extend R1 + the AC surface with an explicit main-checkout fixture: `state show` never exits 0 with an empty mode; it renders the chosen honest draft state or an explicit indeterminate/error result. Keep the spec-worktree idle+activeSpec assertion as a separate case.

## G2 — R6.3/#255 seam: partial/total filter-drop loudness is owned by neither spec (R1, R3, R6 — 3-slot convergence) — MUST FIX
As bounded, the step-2.5 title-filter's silent drops (partial: index shift; total: false "No ready work") are disclaimed by both 128 and 129. Fix in THIS spec (selection honesty):
1. Add a normative requirement + AC: whenever the filter excludes any bd-ready item from the displayed/selected set — on the default, `--pick`, positional, and `--emit-only` paths — the output names the excluded count and reason (e.g. "N ready beads excluded by spec filter: title does not reference spec <id>").
2. Add an AC for the empty-after-filter case: a non-empty raw `bd ready --parent` set that filters to empty must produce a loud error naming the exclusions — never a successful "No ready work found". (The remaining #255 readiness-truth/autopilot-conclusion class stays deferred; update the Non-Goals #255 entry to record this narrowing so exactly one spec owns each face.)

## G3 — OQ-1 option pricing errors in R4/ADR touchpoints (R1, R5 — convergent; interlocks R3's AC-7 ask and R6's provenance ask) — MUST FIX
1. (R1) R4.2's "Whichever mechanism: the fact is … fed into `panel.GateFacts`" mandates option-(c) machinery unconditionally. Reword conditional: "if the chosen mechanism involves a staleness-exemption fact, that fact is computed caller-side and fed into GateFacts…"; mark the `internal/panel` entries in Impacted Domains and In Scope as conditional on OQ-1(c).
2. (R5) OQ-1(b) rollback: price it explicitly as a reversal/amendment of ADR-0041 §1's "Never a rollback" (Reconcile item) and §2's forward-reconcile-only contract — or remove (b) as admissible. The draft's "keeps §1" framing is wrong.
3. (R5+R3+R6) OQ-1(c) exemption pin: replace bare "`.beads/**` tracker-only" with an explicit reviewed set of machine-generated tracker artifacts (at minimum distinguish `.beads/issues.jsonl` from arbitrary `.beads/` paths); either specify how tool authorship is PROVEN (diff pathspec cannot see authorship) or drop "tool-authored" from the pin so the spec claims the exemption's true paths-only shape; add negative ACs: (i) a mixed commit touching `.beads/**` AND a non-tracker path still trips stale-SHA (AC-7 ambiguity), (ii) an independent actor's `.beads/**`-only commit landed after review remains stale/blocking if the pin claims provenance — or is exempt if the pin is paths-only; whichever, the AC must match the pin.

## G4 — R1.3/AC-2 agreement-table surface ambiguity + OQ-3 coherence (R3; R1's option-3 ask) — MUST FIX
1. (R3) State which surfaces each agreement-table fixture asserts. Either AC-2 asserts `state show` + instruct-tail + derivation per row and the plan-approve row is carved for OQ-3 option 2 (an annotated stored-vs-derived row rather than a three-surface agreement — as written the row is unsatisfiable under option 2 while OQ-3 claims all options live), or strike option 2's live-claim.
2. (R1) OQ-3 option 3 (derive-implement-when-children-exist): name the evidence bar for taking it — audited refutation of settled fact 1 with evidence, per the established gate discipline — or strike the option. The charter's "do not relitigate" is flat.

## G5 — OQ-5 is half-foreclosed by R6.2/AC-9 (R1, R3 — convergent) — MUST FIX
R6.2 sentence 1 + AC-9 already pin bd-ready order end-to-end, foreclosing OQ-5's "plan order" arm; OQ-5's claim that "AC-9 pins whichever choice is made" is false. Narrow OQ-5 to what is genuinely open (the default-pick rule over a settled bd-ordered display; note a non-first default clashes with the quoted "Defaulting to first item" line) — and add an assertion (AC-9 or sibling) that the no-pick default path prints the self-describing ordering line R6.2 requires but no AC tests.

## G6 — ADR pricing/citation corrections (R5 ×3) — MUST FIX (cheap, mechanical)
1. ADR-0025 pricing for R5: use the ADR's exact named-artifact classification rather than claiming "`.beads/**` is artifact-exempt"; state that R5.3's `--allow-dirty` fallback ALSO requires an ADR-0025 amendment (it permits a claim despite user-authored dirt).
2. OQ-4 citations: the amended table row is at `ADR-0023.md:140`, not `:142` — correct all four occurrences (Background "additional ADR staleness" paragraph, ADR Touchpoints ADR-0023 entry, OQ-4, first grill-record line). [R5 + R2 convergent]
2b. (R2) Background residue-#2: the false "not found or not ready" error is emitted at `select.go:92`, not `:97` (line 97 is inside FormatWorkList's doc comment) — correct the citation.
3. ADR-0035 consistency: require any retained phase-mismatch warning or indeterminate post-mutation guidance to end in a safe final `recovery:` line, pinned in AC-4/AC-5 — or narrow the touchpoint's "every new warning keeps such a line" claim.

## G7 — R2.1 verb-coverage breadth (R3, R6 — convergent) — MUST FIX
1. (R3) The result-before-tail ordering pin: add a table-driven AC across all seven listed verbs, or scope R2.1 down to the verbs the ACs pin.
2. (R6) `mindspec release` and `mindspec repair phase` are state-changing verbs outside the "every state-changing verb" list: either extend the agreement/output contract to cover them (release-to-plan vs release-to-implement post-state; forced backward repair's result line) or narrow the phrase with a justified taxonomy.

## G8 — R5.1 claim-written path enumeration may be empty (R3) — MUST FIX
Name at least one concrete claim-written path in the invoking checkout, or state the disposition if the enumeration is empty (guard never fires → AC-8's refusal branch is vacuous; the requirement then reduces to removing the block entirely, which must be said out loud).

## G9 — `repair phase` concurrency semantics (R6) — MUST FIX (small, falsifiability)
A claim or completion landing between `DerivePhaseDetail` and `MergeMetadata` must not let a stale derived value move stored phase backward. Specify the mechanism class (compare-and-swap / re-derive-before-write / bounded retry) or explicitly scope TOCTOU out with rationale; make R3.3's guard falsifiable under the race.

## G10 — Pick-boundary ACs (R6) — MUST FIX (small)
With exactly one ready bead, explicit `--pick 2` errors rather than silently selecting the sole bead; zero/negative indices rejected (or semantics explicitly defined); parity pinned on claim and emit-only paths.

---
Deferred as pure-style: none — all 25 asks (23 RC + 2 R2-advisory) map to the groups above. R2's advisories are both citation fixes, folded into G6.2/G6.2b.
