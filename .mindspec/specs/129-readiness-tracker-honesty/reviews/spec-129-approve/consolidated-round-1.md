# Spec 129 — spec gate, round 1: consolidation

- Reviewed commit: `7a490064`. Panel: 9 slots (F1–F3, O1–O3, G1–G3), all REQUEST_CHANGES, 95 asks.
- I read every verdict's `findings` and `concrete_changes_required`, including the INFO/"verified" parts. No finding was out-voted.
- Each ask is FIXED in `spec.md`/ADR text, REFUTED with re-derived evidence, or (none this round) DEFERRED.
- Probes ran only in scratch:
  - a throwaway bd 1.1.0 database;
  - a `--no-hardlinks` clone with a probe test `internal/lifecycle/zz_reviser_probe_test.go`.

  Paths are under `/private/tmp/claude-503/-Users-Max-replit-mindspec/d05dec1f-1671-4cbd-8202-e55ee47cdf39/scratchpad/p129s/reviser/`.

## Groups and what was done

### A — Membership (F2, F3, O1, O2, O3, G1, G2, G3; convergent)

**Finding, verified.** R1.1 (descendant OR `metadata.spec_id`) admitted beads that nothing downstream can handle:
- `EvaluateReadiness` resolves lineage via `phase.FindEpicForBead`. That function reads a direct dependency of issue_type epic, then falls back to a `[NNN]` number from the title. It has no metadata leg.
- `beadIndex` then needs a trailing `.<N>`.
- R4.1 delegated membership to that same title-fallback function.
- R1.4's tally used the epic's child list.

**Decision: Option N (narrow).** Selection is the **plan bead**: in `bd ready --parent <E> -n 0`, not an epic, ID `<E>.<N>`, and row `parent == E`. That is exactly the set that readiness, MF-1 and `complete` can already evaluate. Every other bd-ready bead linked to the spec (descendant, title, or `metadata.spec_id`) is a named near-miss, with one of three reason/recovery classes (R1.2).

Evidence, from the scratch bd 1.1.0 database:
- A child made with `--parent` and then detached with `bd dep remove` is **still** returned by `bd ready --parent E`, because bd treats `<E>.<N>` as a descendant by ID even with no edge. Its row has no `parent`.
- After `bd update <id> --parent E`, the row has `parent: E` and `bd show` carries `(E, parent-child, epic)`. That is the edge `FindEpicForBead` resolves, and the ID is still `<E>.3`. So class N1's recovery provably yields a claimable bead (AC-2).
- A non-dotted bead reparented the same way stays non-dotted (`pr-3qh`). No bd edit makes a non-plan bead claimable, so N3's recovery is inspection-first and stated as non-mechanical. This is the narrowing of the brief's "a recovery that provably yields a claimable bead": it holds for N1 only, and the spec says so.

**Rejected: Option W.** It needs `FindEpicForBead` (core) to learn a metadata leg. It needs `beadIndex` to map non-dotted beads onto plan sections, a mapping that no rule defines. It needs a conflict-precedence rule. And it would make claimable a class of bead that no shipped flow creates. Recorded in OQ-9.

**Applied consequences:**
- R3 drops the epic `spec_id` write: the epic row would enter every metadata-field query (verified).
- R4.1 uses a status-independent, title-free, fail-closed membership for dependencies: plan-form OR dependency-entry `metadata.spec_id` (carried by `bd show` dependency entries, verified) OR parent read. A read error is FAIL.
- R1.4 tallies over the epic's child list, which contains every member.
- R1.8 adds a lineage-equality refusal, so the bead is never validated against spec T and dispatched into spec S.
- R1.7 lists the spec-128 tests that are rewritten. The mechanism of settled fact 1 is kept.

### B — R4.2 full-history scan (O3 BLOCKING; F2, F3, O1, O2)

**Reproduced.** In the scratch clone, an emulated seam over `firstParentMergesFn` gave:

| Shape | Today | Draft (full history) | Redesign |
|---|---|---|---|
| refresh-then-first-parent landing | identified | `*LandedMergeNoEvidence` "owned merges disagree" | identified |
| #239 off-first-parent with a pre-landing refresh | `ErrLandedMergeNoCandidate` | `*LandedMergeNoEvidence` ambiguity | identified |
| sibling-bead merge, then off-first-parent | NoCandidate | — | identified (X and Y) |

**Redesign (R4.2):**
- The first-parent scan stays primary.
- Only when it nominates zero owned candidates does a `git log --merges --topo-order` fallback run. It admits only `Merge bead/<id>` or `Merge branch 'bead/<id>' into <spec-branch>`, so the direction is pinned: bead/<id> is the source and the spec branch is the target. Refreshes into `bead/<id>`, merges into another branch, and the G2-2 any-token form are excluded.
- `--topo-order` defines newest and oldest.
- The fallback is scoped to the MF-3 read and to `ReattestLandedMerge`.
- Every other caller is enumerated and **unchanged**: `MergedUnclosed` → `complete`'s reconcile (closes beads) and `stale_open`, `orphan_hints`, `planFindLandedMergeFn`, `adoptFindLandedMergeFn`, the executor's `ExactSecondParentMerges` write, and `merge_resumption`.
- AC-11 (vi) pins that `complete`'s reconcile ignores the fallback.

The ADR-0041 amendment is priced as a new § "Amendment (Spec 129)" item (i). It covers the Spec-125 "first-parent merge" text and §2(ii)'s "first-parent chain since" reading. The `SubsumptionConflict` rationale is restated as "M is an ancestor of the tip".

### C — Reattest dead end and the misquote (O1, O2, F2)

**Verified** in `reattest.go`: datum (c) calls `reviewedHeadSHAForBead` and refuses with `ReattestStatePanelContradiction`, so MF-3's NoEvidence → `mindspec reattest` recovery would loop.

**Fixed:**
- R4.3 applies the SET semantics to `FindLandedMerge` AND to reattest's datum (c). A non-equal set is non-corroborating and is recorded in the audit, and reattest proceeds on (a). That is no weaker than today's no-registration case.
- Mixed-leg rule: a (b)/(d) exact contradiction still contradicts; the panel never vetoes.
- The ADR-0041 item (c) text is quoted exactly ("a registered panel's `reviewed_head_sha` equal to an exact match's second parent"), and the change is stated as making the non-equal case explicit.
- The amendment is appended as a new section, finalized by the bead that lands the code, and pinned by AC-18.
- The existing contradiction-pinning tests are named for rewrite.

### D — Override marker (G2, G3, O2, O3, F2, F3, O1)

Fixed:
- **No flagless re-claim:** an existing marker never authorizes a claim. Fresh `--allow-not-ready=<ids> --evidence <path>` is needed every time. This removes the forged/stale-marker gate-opening (O2.8, O3.10).
- **Marker schema v2:** `{schema, signals, details: {id: sha256(Detail)}, overridden, evidence: {path, sha256, bytes}}`. The evidence **text is not stored**, because bd metadata is exported into the tracked `.beads/issues.jsonl` (verified: existing markers at `7a490064`). The ADR-0025 touchpoint is corrected.
- **Shared validity contract:** VALID / LEGACY / MALFORMED, fail-closed; a read error is an error. LEGACY (today's `{signals, overridden}`) is never honoured.
- **Coverage requires the per-signal detail digest,** so an MF-3 override for dependency A does not cover dependency B.
- **Replace, never union:** an explicit narrower request with an old covering marker still refuses (G3.3).
- **Bare and space-separated forms:** both refuse; `=all` is not admitted.
- **Recovery line:** carries `--evidence <path-to-justification>` as a literal placeholder, with no dynamic operand (ADR-0042, the `select.go` R5/G4 rule).
- **`ready-check` exit 0-with-OVERRIDDEN** is the dispatch-ingress answer and never claim authority.
- **Engine isolation stated:** the marker is read in `cmd`, never in `EvaluateReadiness`.
- **Background F corrected:** `ms-bead-impl` already did the subset comparison in prose. Per ADR-0045 that prose is **removed**, the pin tests are rewritten, and the full bare-form surface list is given (R6.5). `autonomy.md` has no mention, so it is dropped.
- ADR-0041's fourth-verb clause is marked AMENDED.

### E — #232 / #250 / count (F1, F2)

**#232 verified:** same-spec dependency, one panel round. Mechanism = #233's: `complete`'s landing commit ≠ the reviewed SHA, so the contradiction is printed as the zero-candidate sentence.

**Closure after R4.3 + R4.4:**
- With a binding present → PASS via (d).
- Without one → NoEvidence → `reattest` writes the binding on (a) → PASS.
- AC-12 (iv)/(v) are red today. The local-ahead-of-remote leg is kept and labelled green-today.
- #233's refusal symptom is cleared as a consequence. #233 stays open for spec 130 (review-to-landing binding, OQ-3).

**#250** goes to 129b (the #244 family).

**Count:** 129 = R1–R6, estimated **7–10 beads**, plus R7 (doc, this commit) and the ADR-0041 amendment (with the R4/R6 beads). 129b = 4 deliverables plus phase-lineage unification. 130 = S0 and #233. Relief valve: R6 → 129c.

### F — R1.4 derived state (F2, F3, O2)

- **Verified:** `DerivePhaseWithStatusWithCache` returns the stored phase and only warns. It now uses `phase.DerivePhaseDetail`, printing the stored and child-derived phases, with the recovery keyed on child-derived and `repair phase` added when they disagree.
- The plan arm is split: zero children → plan approve; children open → `bd blocked --parent <E>` (verified to exist). The sentence "none bd-ready" replaces "all blocked".
- The in-progress arm is stated to mutate, and the tally prints first.
- One correction to F2's sub-claim "nothing writes review": `phase.SanctionedDerivedSync` (wired from `release`) and `mindspec repair phase` can write a child-derived `review`. The conclusion still holds, because the normal path stores `implement`.

### G — Row caps (O1, F2, F3)

- **Verified** in scratch: `bd list --json` returned 61 rows with no `-n`; `--limit 50` returned 50 rows with empty stderr; `bd ready -n 10` printed the notice. Background B is corrected.
- The waist has no cap escape, so the notice check is always valid for `ready`. The spec states that `list` has no notice guard.
- The lint is call-site based, and each literal site is dispositioned.
- AC-7's fake bd applies the default cap.

### H — Facts and AC fixes

- **#239's `--spec=<number>`** is live (`cmd/mindspec/next.go`). Fixed as R1.6 plus an AC.
- **AC-13 legs (now AC-14):** legs 1 and 4 are red today, and leg 3 is vacuous. Leg 6 was added.
- **bd native spec-id:** `bd list --spec` prefix-matches (`--spec 12` matched `129-probe`) and `bd show --json` exposes `spec_id`. Background C and R3 are corrected. Not adopted, for the stated reasons.
- **Background J roots** corrected.
- **`registerLandedPanel` is REFUTED as "nonexistent":** see the refutations below.

### I — ADR-0045 governance (F1, O2)

- **Verified:** ADR-0044's Related line says "conduct is L4 skill-layer doctrine", and ADR-0040 §1 has the "start it at L4" default and no back-pointer.
- **Fixed in ADR-0045:**
  - Amends: ADR-0040 §1 (three parts) and ADR-0044 (placement);
  - "Relation to ADR-0040 §3" and "Relation to ADR-0044" text;
  - step 1 → spec 130, steps 5–7 unassigned;
  - a Status note fencing `Accepted` (the operator's decided direction) under ADR-0032: a bead may cite it only for a rule it moves out of a skill.
- **Not flipped to Proposed:** Status is the operator's (Deciders: Max) decision, and O2.16 offered the fence as the alternative.
- Back-pointer blockquotes were added to ADR-0040 §1 and ADR-0044.
- The Impacted-Domains skill-mirror option was removed. The only skill edit left in scope deletes a rule (R6.5).

## Refutations (re-derived evidence)

1. **F2.13, O1.11 — "`registerLandedPanel` does not exist."**
   - Command: `grep -rn registerLandedPanel --include='*.go' .` → `internal/validate/readiness/fixtures.go:189: func registerLandedPanel(root, depID, target, headSHA string) error`. It is a non-test fixture file, which is why a `*_test.go` grep missed it.
   - It writes one registration under the fixed slug `fixture-<dep>`, so AC-12 now says "extended with a slug" (the part of the ask that was real).
2. **F2.10, F3.9 (premise) — "`bd ready --json` rows carry no parent or metadata."**
   - Command (scratch bd 1.1.0): `bd ready --json -n 0 | python3 -c '…print(x["id"], "parent" in x, x.get("parent"), "metadata" in x)'`. Output: `pr-1rs.2 True pr-1rs False`, `pr-3qh False None True`, `pr-1rs.1 True pr-1rs True`.
   - Both fields are omit-when-empty. The reviewers measured rows that had neither.
   - The cost ask is still fixed: the near-miss scan is one global `bd ready -n 0` with no per-hit `bd show`, so 2 calls in total. R2.4 pins the fields at the version floor.

## Coverage table (95 asks)

| Ask | Group | Disposition |
|---|---|---|
| F1.1 | E | FIXED — #250 → 129b Non-Goals and count |
| F1.2 | E | FIXED — D.5 and R4.5 name #233's mechanism; closure condition stated |
| F1.3 | E | FIXED — AC-12 (iv)/(v); (vi) kept as a guard |
| F1.4 | E | FIXED — 7–10 beads; R7 and the ADR-0041 amendment counted; R6 relief valve |
| F1.5 | H | FIXED — R1.6 + AC-5 |
| F1.6 | I | FIXED — skill-mirror option removed |
| F1.7 | I | FIXED — S0 → spec 130 in both places; ADR-0040 back-pointer |
| F2.1 | F | FIXED — `DerivePhaseDetail`, plan arm split, stored phases in AC-4 (sub-claim corrected) |
| F2.2 | F | FIXED — in-progress arm mutates; tally first |
| F2.3 | A | FIXED — one universe (plan beads ⊆ child list) |
| F2.4 | A | FIXED — ID-level helper in `internal/bead`; AC-10 (d) |
| F2.5 | B | FIXED — ADR-0041 amendment item (i) |
| F2.6 | B | FIXED — #239 topology; consistency AC pinned; write-site as a requirement |
| F2.7 | E | FIXED — fixture reproduces the issue's shape, red today |
| F2.8 | G | FIXED — fake bd applies the cap |
| F2.9 | A | FIXED — near-miss positional on any set, from global ready rows |
| F2.10 | A | FIXED (premise REFUTED, see above) |
| F2.11 | A | FIXED — R1.7 |
| F2.12 | G | FIXED — no escape; lint leg reworded |
| F2.13 | H | REFUTED — helper exists; slug extension added |
| F2.14 | H | FIXED — red legs corrected |
| F2.15 | C | FIXED — mixed-leg rule |
| F2.16 | D | FIXED — no dynamic operands |
| F2.17 | D | FIXED — ADR-0025 touchpoint; no text stored |
| F2.18 | D | FIXED — dispatch answer, never claim authority |
| F2.19 | F | FIXED — `bd blocked --parent` pinned |
| F2.20 | D | FIXED — `=all` not admitted |
| F3.1 | A | FIXED — option (b): non-plan beads are named near-misses |
| F3.2 | A | FIXED — N1 only for plan-form; N3 honest |
| F3.3 | A | FIXED — tree-only membership; R1.8 |
| F3.4 | A | FIXED — R4.1 predicate; fail-closed arm |
| F3.5 | B | FIXED — consumer table; complete reconcile unchanged (AC-11 vi) |
| F3.6 | D | FIXED — ms-bead-impl prose removed; pins rewritten |
| F3.7 | F | FIXED — tally universe; "none bd-ready" |
| F3.8 | A | FIXED — epic stripped; no epic `spec_id` |
| F3.9 | A | FIXED (premise REFUTED, see above) |
| F3.10 | B | FIXED — AC-11 (i) |
| F3.11 | G | FIXED — waist-only notice; call-site lint |
| F3.12 | D | FIXED — bare with nothing failing; release disposition; detail digest |
| F3.13 | A | FIXED — R1.7 |
| F3.14 | D | FIXED — marker read in cmd |
| O1.1 | A | FIXED |
| O1.2 | A | FIXED — claim leg for spec_id-only bead removed (Option N); core not impacted |
| O1.3 | B | FIXED — reattest.go in scope |
| O1.4 | C | FIXED — AC-12 (v) |
| O1.5 | B | FIXED — "unchanged" justified per leg; target admissibility |
| O1.6 | C | FIXED — exact quote |
| O1.7 | D | FIXED — Background F; skill and surfaces |
| O1.8 | G | FIXED — Background B; list has no notice guard |
| O1.9 | H | FIXED — leg 6 |
| O1.10 | G | FIXED — dispositions |
| O1.11 | H | REFUTED — helper exists |
| O1.12 | H | FIXED |
| O2.1 | C | FIXED |
| O2.2 | C | FIXED — AC-12 (v) |
| O2.3 | B | FIXED |
| O2.4 | B | FIXED — consumer table; AC-11 (vi) |
| O2.5 | B | FIXED |
| O2.6 | C | FIXED — new section; AC-18 |
| O2.7 | D | FIXED — AMENDED |
| O2.8 | D | FIXED — no flagless re-claim |
| O2.9 | D | FIXED — LEGACY class |
| O2.10 | D | FIXED — no text stored |
| O2.11 | D | FIXED — `--evidence` placeholder; AC-15 |
| O2.12 | A | FIXED — reparent only when parent is none; inspection otherwise; guard registry |
| O2.13 | F | FIXED |
| O2.14 | I | FIXED — Amends ADR-0044; reconciliation |
| O2.15 | I | FIXED — Amends widened; §3 sentence; back-pointer |
| O2.16 | I | FIXED — Accepted fenced (not flipped: operator decision) |
| O2.17 | I | FIXED |
| O2.18 | A | FIXED |
| O2.19 | D | FIXED — R6.5 list |
| O3.1 | B | FIXED — first-parent primary plus pinned fallback |
| O3.2 | B | FIXED — AC-11 (ii)–(iv) |
| O3.3 | B | FIXED — `--topo-order` |
| O3.4 | B | FIXED — scoped to MF-3 + reattest |
| O3.5 | B | FIXED — premise corrected; refresh regression avoided |
| O3.6 | A | FIXED |
| O3.7 | A | FIXED — single-source order (no union) |
| O3.8 | A | FIXED — conflict = near-miss; R1.8 |
| O3.9 | D | FIXED |
| O3.10 | D | FIXED — no reuse; detail digest |
| O3.11 | D | FIXED — includes ready_gate.go and next.go strings |
| O3.12 | D | FIXED — AC-15 |
| O3.13 | D | FIXED |
| G1.1 | A | FIXED — AC-10 (d), (g) |
| G1.2 | A | FIXED by narrowing — metadata-only beads are not selectable, so no claim path (Option N) |
| G1.3 | A | FIXED — near-misses on every scoped call |
| G1.4 | H | FIXED — Background C, R3.1, grill corrected |
| G2.1 | A | FIXED — R1.8 one context; AC-6 |
| G2.2 | A | FIXED — AC-10 (d), (e), (g) |
| G2.3 | D | FIXED — validity contract; AC-17 |
| G3.1 | A | FIXED |
| G3.2 | D | FIXED |
| G3.3 | D | FIXED — AC-16 |

**Totals:** 95 asks. FIXED 91 (two of them with a refuted premise: F2.10, F3.9). REFUTED 2 (F2.13, O1.11), each with the real residue applied. DEFERRED 0. Three new deferrals that no reviewer asked for are named in the spec: phase-lineage unification → 129b, #233's binding → 130, S0 → 130.

## Errors in the panel BRIEF and the round-1 dispatch brief

- (all nine slots) The BRIEF header says to write `<slot>-round-1.json` beside the BRIEF, which conflicts with the dispatch prompt's scratch path. TODO(skill) placeholder sections are duplicated. The grounding list omits `internal/lifecycle/reattest.go` and `internal/phase/derive.go`.
- (dispatch brief) "`registerLandedPanel` does not exist": it does (refutation 1).
- (dispatch brief) "ready rows carry no parent or metadata": both are present when set (refutation 2).
- (dispatch brief) "R1.2's reparent recovery leaves a non-dotted id, so it dead-ends": true only for non-dotted beads. A detached plan-form bead is repaired by exactly that recovery (verified).
- (dispatch brief) "nothing writes 'review'": `SanctionedDerivedSync`/`repair phase` can write it.
- (dispatch brief) The FindLandedMerge consumer list omitted `orphan_hints`. Reattest does not call FindLandedMerge; it shares `firstParentMergesFn`.
