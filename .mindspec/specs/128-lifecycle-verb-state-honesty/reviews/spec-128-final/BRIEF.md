<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-final`
- **Round**: 3
- **Branch**: `spec/128-lifecycle-verb-state-honesty`
- **Reviewed commit**: `a8ac1fc4f5121c12df3f12a5eb86977c9cd0a17c`

## Your job

Review the diff at the reviewed commit above and write your verdict as
a single JSON file named `<your-slot>-round-3.json` beside this BRIEF
(the panel artifact schema — see `.mindspec/domains/workflow/interfaces.md`
§ Panel Artifact Schema). Required and optional top-level fields:

- `verdict` (required): one of `APPROVE`, `REQUEST_CHANGES`, `REJECT`.
- `hard_block` (optional boolean, a top-level sibling of `verdict` — never
  a per-finding field): set `true` only for an evidence-bearing halt that
  no vote count may override.
- `reviewer_id`, `confidence`, `rationale`, `concrete_changes_required`,
  `findings`: reviewer-authored context, read presentation-only by
  `mindspec panel tally` — not consumed by the gate decision.
<!-- /mindspec:panel-header -->

## Summary

Final 12-slot four-family review of the WHOLE spec-128 branch
(`spec/128-lifecycle-verb-state-honesty`) vs `main` — merge-base
`ae45fd7a`, reviewed tip `8eaeb05e`, 503 files, +38.5k/−1.1k. All ten
beads merged through per-bead panels (34 rounds total — counted from the
round artifacts in the per-bead review dirs). Those panels
gated each bead in isolation; THIS review's unique value is the
INTEGRATED WHOLE — inter-bead seams, whole-branch spec fidelity, and the
completeness of the AC surface. Spec 120's lesson stands: its 12-slot
final review caught 5 MAJOR inter-bead-seam leaks that ten clean per-bead
panels had missed. Assume the seams, not the beads, are where this branch
is weakest.

## Scope

Everything reachable from `git diff ae45fd7a..8eaeb05e`. The spec
(`spec.md`) and plan (`plan.md`) at the tip are the contract — both went
through multi-round gates (6 spec rounds, 8 plan rounds) and every open
question is RESOLVED in the plan (OQ-1=(a) snapshot reorder; OQ-2=on-disk
draft carve-out; OQ-3=option 1; OQ-5=displayed-row-1). Judge the code
against those resolutions, not against alternatives the gates already
adjudicated.

## Per-bead record (merge SHA, panel rounds)

1. mindspec-ta30.1 — phase write lock + backward guard + repair hardening
   — aa40b80a, 4 rounds (round 4 was a #186-forced tracker-only confirm).
2. mindspec-ta30.2 — OQ-3 option-1 value semantics + EnsurePhaseSemanticsV2
   — 0cda6272, 2 rounds.
3. mindspec-ta30.3 — draft-mode derivation + state show honesty —
   ba1da445, 4 rounds (tag-shadow class closed at probe + enumeration).
4. mindspec-ta30.4 — per-verb agreement table + row fixtures + ADR-0023
   strike — febec324, 3 rounds (structurally cell-pinned, non-circular).
5. mindspec-ta30.5 — verb output honesty: tails, indeterminate form,
   AC-14 — 7e194cf1, 5 rounds (the ReadState nil-contract arc).
6. mindspec-ta30.6 — prospective-snapshot gate reorder + ADR-0041 §1 —
   decadd92, 5 rounds (the panel EXECUTED the snapshot race in r1).
7. mindspec-ta30.7 — proportional claim guard + spec-approve safe replay
   — 7120925c, 3 rounds (parallel track with 9).
8. mindspec-ta30.8 — deterministic selection + scope honesty —
   f9d6785f, 3 rounds (closed ta30.9's next.go render-leg deferral).
9. mindspec-ta30.9 — bd stderr passthrough w/ safe rendering —
   5da8d918, 3 rounds.
10. mindspec-ta30.10 — frozen-contract guidance + docs sweep + method
    gate — c6fded51, 2 rounds (paid the eight --allow-doc-skew debts;
    validate-docs = zero errors at tip).

## THE LIVE-EVIDENCE LEDGER — this spec was dogfooded against itself

Every headline bug this spec fixes BIT THIS VERY RUN while the run was
fixing it. Treat these as integration evidence the fixtures must honor:

- **#186 bit at bead 1's own merge**: the tracker auto-commit advanced
  the tip past the reviewed SHA and forced a round-4 tracker-only confirm
  panel — the exact wedge bead 6's reorder closes.
- **#179 blocked claims THREE times during its own fix's development**:
  untracked handover files at the run's first claim; panel artifacts
  before bead 4's claim; a main-checkout claim with unrelated tracked
  edits. Bead 7's fixtures mirror those real shapes.
- **Bead 7 round 1 proved the plan's derived-empty enumeration factually
  wrong in detail** — the panel corrected the enumeration against the
  landed claim path (the plan's derivation predated beads 2/3's writes).
- **Bead 6 round 1's panel EXECUTED the snapshot race** rather than
  arguing it — empirical adversarial review found the interleaving the
  design had to close.
- **Bead 5's four-round ReadState nil-contract arc** — the indeterminate
  channel's nil-vs-empty contract took the full gauntlet to pin at every
  layer.
- **The -short blind spot bit TWICE** (a no-short-only cmd/mindspec
  failure escaped two per-bead gates) → the no-short cmd/mindspec suite
  (-timeout 30m) was ADDED to the per-bead gate mid-run. Verify it holds
  at the tip.
- **#233 + the MF-3/attestation coupling** was hit live (an audited
  --allow-not-ready claim over a false-negative MF-3) and posted to GH —
  out of this spec's scope, but reviewers should not re-file it.

## AC MAP — the completeness surface (verify numbering against spec.md;
two swapped-AC incidents occurred in orchestration briefs this run)

| AC (spec.md) | Delivered by | Pinned by |
|---|---|---|
| AC-1 (i)(ii)(iii) — state show honesty | bead 3 | cmd/mindspec state tests + internal/resolve/resolve_test.go |
| AC-2 — agreement-table row fixtures | bead 4 | internal/phase/agreement_table_test.go (cell-pinned) |
| AC-3 — spec-create tail | bead 5 | cmd/mindspec/verb_output_test.go |
| AC-4 — completeTail two branches | bead 5 | internal/complete/complete_test.go + verb_output_test.go |
| AC-4b — advanceState seam, no fabricated plan | bead 5 | internal/complete (derivation_failed / run_mode_error legs) |
| AC-5 (ii)(iii)(v)(vi)(vii) — lock/guard/inventory | bead 1 | internal/phase/{order,phaselock,writer_inventory,migrate}_test.go + cmd/mindspec/repair_test.go |
| AC-5 (i)(iv)(viii) — warning dissolution + sanction | bead 2 | internal/phase/{route_248,derive_recovery,sanctioned_sync,semantics_v2}_test.go |
| AC-6 — retry meets the same gate | bead 6 | internal/complete/complete_test.go + materialize tests |
| AC-7 (i) — stale-SHA unchanged (ii-iv vacated, OQ-1(c)-only) | bead 6 | internal/complete + panel gate red-on-revert leg |
| AC-8 (i)(ii)(iii) — proportional guard | bead 7 | internal/next/guard_test.go |
| AC-9 — pick parity + boundaries | bead 8 | cmd/mindspec/next_selection_test.go + internal/next/select_test.go |
| AC-10 / AC-10b — filter loudness | bead 8 | next_selection_test.go legs |
| AC-11 — bd stderr passthrough + rendering | bead 9 | internal/bead/bdcli_test.go + internal/next/beads_test.go |
| AC-12 — frozen-contract guidance + adopt pin | bead 10 | internal/instruct templates test (cobra-pinned) |
| AC-13 — -short method gate | bead 10 (+ every bead's verification) | full-tree `go test ./... -short` |
| AC-14 — nine-verb result-first + total cobra classification | bead 5 | cmd/mindspec/verb_output_test.go |
| AC-15 — explicit --spec scope honesty | bead 8 | next_selection_test.go (seam-recorded no-global-query) |
| AC-16 — spec-approve safe replay (5 legs) | bead 7 | internal/approve spec tests |

The table above is the ORCHESTRATION record at file granularity — R3's
audit (below) verifies it at test-name granularity against spec.md's own
AC text, including every lettered leg.

## Known residuals (honest; do not re-file, do judge)

- `mindspec-mmz5` — SpecDir custom-root handling (PRE-EXISTING on main;
  filed).
- The CompleteBead dead-branch bead (superseded code path retained
  pending a follow-up removal bead; documented in its doc comment).
- ta30.9's next.go render-leg deferral — CLOSED by bead 8 (verify).
- #256 session.json lifecycle (filed to GH; out of scope).
- The `missing-source-globs` WARN from `mindspec validate docs` —
  PRE-EXISTING on main, out of scope.
- `8ud6` — pre-existing gosec G115 in the journal lock (unrelated
  package).

## The 12 lenses

- **R1 (Fable) — whole-branch spec fidelity**: every requirement
  R1-R7 + R3.6/R6.7 against the landed code; the plan's four OQ
  resolutions faithfully implemented; no silent divergence from the
  gated design (esp. the M/R/S/G/T protocol's letter).
- **R2 (Fable) — inter-bead seams**: the named suspect seams first —
  b5↔b6 (indeterminate-skip vs the S-stage staging: can an indeterminate
  advanceState interleave the marker machinery?), b7↔b8 (next.go's
  guard/selection layering after two rewrites), b2↔b7 (the lock's
  claim-advance vs the absent-only backfill on the same epic) — then
  free hunt.
- **R3 (Fable) — AC-map completeness audit**: the table above verified
  at test-name granularity against spec.md's AC text; every lettered leg
  located or named missing; the two vacated AC-7 legs confirmed vacated
  not skipped.
- **R4 (codex) — empirical whole-lifecycle drive**: BUILD the binary
  from the tip; in a scratch repo (t.TempDir-style, NOT this repo) drive
  init → spec create → approve → plan approve → claim → complete;
  transcript the honesty surfaces (state show, tails, notices) against
  the agreement table.
- **R5 (codex) — cross-cutting schema + rendering audit**: every new
  message/error surface end-to-end for ADR-0042 discipline (termsafe/
  idrender), the panel-artifact and bd schemas untouched where claimed.
- **R6 (codex) — adversary on the integrated system**: hostile inputs
  across seams (hostile bd stderr into selection errors; hostile titles
  into exclusion reports; marker/trailer forgery attempts; lock
  contention abuse). Execute, don't argue.
- **R7 (Opus) — code quality, largest diffs**: internal/complete,
  internal/gitutil materialize, cmd/mindspec/next.go — maintainability,
  idiom, comment honesty.
- **R8 (Opus) — test-suite architecture**: the 23+ fixture legs'
  maintainability; shared builders vs copy-paste; hermeticity actually
  held (no test depends on this repo's state); -race coverage where
  concurrency is claimed.
- **R9 (Opus) — docs-vs-code final truth pass**: lifecycle-modes.md,
  merge-safety.md, the domain docs bead 10 wrote, the four ADR
  amendments — every documented claim true against the tip.
- **R10 (Sonnet) — regression hunt vs main**: behaviors main users rely
  on that this branch changes (plan-approve stored value, release sync,
  the removed dirty-block, gate ordering) — each an INTENDED change per
  the spec, or a regression to flag.
- **R11 (Sonnet) — CLI UX coherence**: all new/changed messages as one
  voice (result-first, recovery-line convention, notice wording,
  default-line self-description); no contradictory or stale guidance.
- **R12 (Sonnet) — security posture**: injection surfaces end-to-end
  (bd stderr, titles, filenames, commit messages/trailers, marker refs);
  the ADR-0042 boundary held at every NEW render sink.

## Verdict contract + conduct

- Write `R<n>-round-1.json` beside this BRIEF; `reviewer_id` =
  `"R<n> <family>"` (e.g. `"R4 codex"`). Schema per the machine header.
- Threshold: expected 12, pass ≥ 11 — but findings are never out-voted:
  every REQUEST_CHANGES finding is fixed or evidence-refuted regardless
  of the tally.
- Probes run in the SPEC worktree
  (/Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty)
  or a scratch clone — NEVER in the main checkout; leave every checkout
  byte-identical (read-only probes; build artifacts to scratch).
- Scratch under /private/tmp/claude-503/-Users-Max-replit-mindspec/da570e49-a6d2-4faa-a36d-9cb5b5386c7a/scratchpad/
  only, cleaned when done. Absolute paths in every command.
- `-short` on any full-tree test run (settled fact 3); the no-short
  cmd/mindspec suite needs `-timeout 30m`.


## ROUND 2 — Prior-Round Asks (read this first)

Round 1: 4 APPROVE (R1 fidelity, R3 AC-map, R8 test-arch, R10 regression) / 8 RC — 28 asks in 8 groups (`consolidated-round-1.md`), centered on R6's TWO EXECUTED majors (the marker-spoof gate bypass; derive.go's swallowed child-fetch fabricating plan mode). Fixed as TWO commits — `0d21cb26` (code: G1-G6+G8, 31 files) + `0783a507` (docs: G7 12/12) — the tip under review.

Key resolutions: **G1** marker provenance via bd-ATTESTATION pairing (R6's own option (a) — the consolidation's ref-is-authority shorthand was itself forgeable, disputed and corrected by the fixer): publication writes `mindspec_stage_marker_oid` attestation BEFORE each ref publish; the discriminator verifies attestation==observed-OID on both success legs; mismatch/absent ⇒ FOREIGN clear+normal-run; read-failure propagates; leg xxiv = R6's exact spoof repro (forged trailer + hand update-ref → refusal, marker cleared). **G2** both no-stored-phase arms + DiscoverActiveSpecs propagate the failed bd list (4 partial-outage regressions). **G3** every named sink + an ADDITIONAL chokepoint hardening (NET-NEW, judge it): renderBDError now strconv.Quote's any stderr line that reads as a recovery line — closing forgeable machine-greppable `recovery:` lines through every family (found by R5's own mandated real-chain test). **G4** no executable-shaped selector interpolation remains (idrender-gated IDs, operand-free guidance). **G5** truthful approve tail + real-executor e2e. **G6** kebab WARN + the misleading else-branch DROPPED per R7's analysis. **G7** 12/12 function-name-anchored. **G8** R12's preferred both-operands gating.

Also disclosed: pre-fix in-flight markers classify FOREIGN once post-upgrade (one extra gated run, converges); two ratchet registrations for the new refusal constructor.

Final suite at tip: -short 48/48 ok; no-short cmd/mindspec PASS; -race ×5 ok; validate docs zero errors.

**Round-2 duty**: R2/R4/R5/R6/R7/R9/R11/R12 classify YOUR asks ADDRESSED / PARTIAL / MISSED / NEW_ISSUE re-deriving — R6: RE-RUN your spoof repro at the tip (it must now refuse) and judge the attestation design; R5: judge the net-new recovery-line hardening + run your hostile family tests; R2: verify every sink you named. R1/R3/R8/R10: verify the fix commits don't regress your round-1 basis; confirm. All 12 write R<n>-round-2.json.


## ROUND 3 — final confirm (tip a8ac1fc4)

Round 2: 8A/4RC (six residuals) → fixed as ONE commit `a8ac1fc4`: (1) R2's metadata family escaped ONCE at the seam defaults (named wrappers, Unwrap preserved, real-chain hostile test incl. errors.As); (2) R7's crash-window comment rewritten with BOTH windows + the honest one-directional guarantee (crash can COST a resume, never ADMIT one); (3) three citations function-anchored; (4) bead-2 attribution fixed; (5) the two-eights sentence decoupled per R9's analysis with guards re-verified; (6) R12's disposition (b): the "Honest trust boundary" paragraph in ADR-0041 + the doc comments (git-only forgery dead; bd-capable actor narrowed-not-closed; local-operator-environment trust stated plainly; follow-up mindspec-an8q cited). tracedCombined filed as its own bead. Suites at tip: -short 48/48; targeted no-short ok; validate docs zero errors.

Round-3 duty (minimal confirm): R2/R7/R9/R12 classify your residuals re-deriving; all other slots verify the small diff (8 files) doesn't disturb your basis; re-verdict. All 12 write R<n>-round-3.json.
