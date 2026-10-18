# Bead 4 implementation prompt — 128-lifecycle-verb-state-honesty

## Identity
- Bead id: mindspec-ta30.4
- Branch: bead/mindspec-ta30.4
- Worktree: /Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.4
- Spec branch: spec/128-lifecycle-verb-state-honesty

## Read first
- `.mindspec/specs/128-lifecycle-verb-state-honesty/spec.md` — §R1.3 (the
  full row + carve-out text, quoted below as the fixture contract) and
  AC-2.
- `.mindspec/specs/128-lifecycle-verb-state-honesty/plan.md` — §Bead 4
  (steps quoted below) and the OQ-3 resolution (option 1 + the release
  redesign leg are DECIDED — see "Which arms are live").
- The MERGED bead-1/2/3 code in THIS worktree — landed API is the
  authority: beads 2/3 delivered the semantics these rows assert.
- `.mindspec/adr/ADR-0023.md` — note bead 3 already added the
  draft-discovery exception amendment; DO NOT disturb it.

## Files in scope (from plan §Bead 4, quoted verbatim)
1. "The table lands in `project-docs/user/guides/lifecycle-modes.md` (new
   operator doc) AND is referenced from ADR-0023 (the ADR keeps derivation
   authority; the guide carries the per-verb view — location decision
   made)."
2. "`internal/phase/agreement_table_test.go`: one fixture test per row
   against the status-aware derivation surface
   (`DerivePhaseWithStatus`-class) + the `state show` output + tail
   template selection, with the carved rows: release (two surfaces,
   tail-less), terminal rows (epic `done` via `derive.go:200-202` + global
   idle via zero-open-epic discovery), complete→plan BOTH triggers
   (`derive.go:366` empty-lifecycle; `:387` all-open via non-lifecycle
   child — under OQ-3 option 1 both assert true agreement, the option-2
   annotated arms being dead)."
3. "`.mindspec/adr/ADR-0023.md`: strike/supersede-annotate rows `:107` and
   `:108`." (Line numbers pinned pre-bead-3; the rows are the original
   edge-case lines "Some children closed, some open, none in_progress →
   plan" and "Some children closed, some open, some blocked → plan" —
   re-locate by content, since bead 3's amendment may have shifted lines.
   Supersede-annotate so only the spec-095 amendment rows can be cited;
   the spec's OQ-4 resolution says the delivered table may not coexist
   with a live ADR row claiming mixed closed/blocked derives `plan`.)
4. "Table/fixture cross-pin so editing one without the other is red
   (AC-2's closing clause)."

## Which arms are live (plan decisions — do not implement dead arms)
OQ-3 resolved to OPTION 1 and the release-sync REDESIGN LEG WAS TAKEN
(bead 2 wired release to `phase.SanctionedDerivedSync`). Therefore, in the
R1.3 text below: the plan-approve row asserts TRUE three-surface agreement
on `plan`; the release row's plan leg asserts TWO-surface agreement on
`plan` (its tail-less carve-out stands — release emits no tail); every
"under OQ-3 option 2 …" annotated-row arm is DEAD and appears in the
delivered table only if you choose to document the carve-out history —
the fixtures assert the LIVE arms only.

## The fixture contract — spec R1.3, quoted verbatim
"The **per-verb agreement table** is delivered as a documentation artifact
(location decided at plan time; ADR-0023 amendment or a doc it references)
pinning, for each verb's success on its canonical state, the post-verb
mode reported by the three surfaces the row's fixture asserts —
`state show`'s output, the emitted instruct tail's selected template, and
the derivation, whose surface for ALL rows is the **status-aware**
derivation (`DerivePhaseWithStatus`-class, `derive.go:166`/`:174` —
round-4 G1.2: bare `DerivePhaseFromChildren` cannot express the terminal
rows, returning `review` for an all-closed child set) — with per-row
carve-outs stated in the table itself where the surfaces are *designed*
to differ. Rows: `spec create → spec`; `spec approve → plan`;
`plan approve → plan` (**settled fact 1**; this row's fixture shape
depends on OQ-3 — see the carve-out below); `next → implement`;
`complete → implement` (mid-spec — including "remaining beads blocked":
blocked is neither closed nor in_progress, so the just-closed bead makes
the some-closed branch derive `implement`, `derive.go:383`; the round-2
"all blocked → plan" conditioning was WRONG, re-derived at round-4 G1.1)
/ `plan` (TWO reachable triggers, round-5 G1: the empty-lifecycle-subset
fallback `derive.go:366` — completing a non-lifecycle follow-up child of
an epic with no lifecycle tasks — and the all-open branch `derive.go:387`
— completing a non-lifecycle child of an epic whose lifecycle task
children are all still open, since the lifecycle count excludes
non-lifecycle children (`derive.go:347-352`) and `next` applies no
issue-type filter beyond skipping epics, so such a child is claimable;
AC-2's complete→plan fixture covers both) / `review` (last bead);
`release → implement` (a sibling bead already closed — the amended row
`ADR-0023.md:140`) / `plan` (no sibling closed — `ADR-0023.md:141`);
`impl approve`/`impl adopt → idle`. … **Release row carve-outs** …
(i) *tail-less* — R2.1 records that `release` emits no instruct tail, so
this row's fixture asserts two surfaces only (`state show` and the
derivation) … **Terminal-rows carve-out (round-4 G1.2):** for the
`impl approve`/`impl adopt` rows the fixture asserts two facts at two
layers — the closed epic with its done marker derives `done` through the
status-aware surface (`derive.go:200-202`), and `state show`/the tail
report `idle` via zero-open-epic discovery — because the rows' `idle` is
a *global discovery* outcome, not a per-epic derivation. Each row is
cross-checked by a fixture test against the shipped derivation, so the
table cannot silently drift from code."
(Elided in the quote above, present in spec.md: the full plan-approve and
release carve-out histories and the sanction-boundary sentence — read
them in spec.md; the elisions are all option-2/dead-arm text plus the
sanction sentence the carry-forward item 4 checks.)
NOTE on the derivation citations inside the quote: `derive.go` line
numbers were pinned at the spec base; beads 2/3 have edited that file —
re-locate the branches by content.

## Spec AC this bead claims (quoted verbatim)
"**AC-2 (R1)**: Each agreement-table row has a fixture test asserting,
per row, the surfaces R1.3 names — the `state show` output, the instruct
tail's selected template, and the status-aware derivation
(`DerivePhaseWithStatus`-class) over the fixture's epic+child state —
agree on the row's mode; for rows carrying documented carve-outs the
fixture asserts the carved contract instead (round-3 G4): the
plan-approve row under OQ-3 option 2 asserts the documented stored/derived
pair and each surface's documented rendering of it; the release row
asserts TWO surfaces only (`state show` + derivation — no instruct-tail
assertion, `release` emits none), and its `plan` leg asserts the
documented stored-`implement`/derived-`plan` pair under both OQ-3 options
unless the sanctioned release-sync redesign leg is selected, in which
case it asserts two-surface agreement on `plan`. Editing the table
without the fixture (or vice versa) is red."
(The redesign leg IS selected — the release plan leg asserts two-surface
agreement on `plan`; the option-2 arms are dead.)

## Carry-forward notes (bead4_carryforward.md, quoted IN FULL — each item
is load-bearing for the delivered docs/rows)
"1. (b2 R1-2.5) Marker-absent legacy epic with an explicitly-blocked
lifecycle child no longer self-heals (G2 strictness, by design) — lives
behind the backward warning with operator --force-backward as the only
exit. The agreement-table/legacy docs must NAME this shape.
2. (b2 R3-N1) Claiming a newly-added lifecycle bead on a stored=review
epic writes implement over review — evidence-fresh, converging, correct;
the fence/docs wording must count advancePhaseOnClaim as an evidence-fresh
forward-by-intent writer (alongside the completeSyncPhase lexical residual
routed in its doc comment).
3. (b2 R3-N2) plan.go step 4b preserves a present-but-invalid stored phase
(conservative) — a distinct agreement-table row shape.
4. (b1) The "exactly one sanctioned backward writer" sentence: verify it
stays true in delivered docs given 1-3."
Consequences for this bead: the delivered guide gains (a) a named
legacy-shapes subsection covering item 1's blocked-child shape (behind the
backward warning; `--force-backward` the only exit) and item 3's
preserve-invalid-stored shape as explicit row-adjacent notes; (b) writer
taxonomy wording that counts `advancePhaseOnClaim` as an evidence-fresh
FORWARD-by-intent writer — it can write `implement` over `review`
(item 2), which is forward-by-evidence, NOT a second sanctioned backward
writer; (c) a truth-check pass on the "exactly one sanctioned backward
writer" sentence wherever the guide states it — it remains true (the only
unforced BACKWARD writer is release's sanctioned sync; the v2
reconciliation is a semantics translation outside the taxonomy; item 2's
writer is forward), and the guide says WHY it remains true.

## Bead-3 landed semantics the spec-create row asserts
Bead 3 delivered the REAL draft semantics: a pre-epic draft is discovered
on-disk (spec dir + `spec/<id>` branch union), mode `spec`, honest
`state show` rendering; discovery failure is a non-zero refusal. The
spec-create row's fixture asserts THAT delivered behavior — construct the
draft fixture the way bead 3's own AC-1(i)/(iii) fixtures do (reuse their
builders where exported; do not fork).

## Hazards and fixture discipline
- Error-discarding class (hard-blocked beads 1 AND 2 at round 1): every
  read in fixtures and any new helper propagates errors; no fabricated
  modes.
- Marker discipline (plan round-3 G2 pinning): every row fixture that
  needs a stored/derived state constructs it MARKER-PRESENT
  (`mindspec_phase_semantics: "v2"`); the marker-ABSENT
  stored-implement/all-open state belongs EXCLUSIVELY to the legacy
  fixtures (bead 2's) — do not create new marker-absent states here
  except where a row-adjacent doc note's fixture explicitly exercises a
  legacy shape (carry-forward item 1), and then say so in the test name.
- Hermeticity: fixture roots in `t.TempDir()`; the suite must be green
  run FROM the bead worktree; no dependence on this repo's own lifecycle
  state.
- The tail-template-selection surface: assert template SELECTION at the
  instruct build/render level driven by the fixture's state. Bead 5 (not
  yet landed) owns tail/`Result.NextMode` agreement and the indeterminate
  form — if a specific row's tail assertion is genuinely blocked pending
  bead 5, assert the two available surfaces for that row and record the
  third in Deviations naming bead 5, rather than papering over it.

## Shared helpers to REUSE (do not reimplement)
- `phase.DerivePhaseWithStatus`/`WithCache` — the row derivation surface.
- Bead 2/3 fixture builders and seams (semantics_v2 fixtures, AC-1
  fixtures, `SetMergeMetadataForTest`, per-package `*MergeMetadataFn`
  seams, bd seam vars).
- `instruct.BuildContext`/`instruct.Render` for template selection.
- The existing doc-sync conventions for `project-docs/user/guides/`
  (mirror `merge-safety.md`'s shape for the new guide).

## Guardrails
AGENTS.md § Bead-loop guardrails applies in full: no `mindspec complete`,
no `git push`, no raw `git merge`, no scope creep beyond the files above,
no reimplementing landed helpers, exactly ONE commit whose message ends
with `Deviations: <list or "none">`, and tests must PASS —
`go build ./... && go test ./... -short` green FROM THE BEAD WORKTREE
(`-short` mandatory — spec settled fact 3). Absolute paths; scratch ONLY
under
/private/tmp/claude-503/-Users-Max-replit-mindspec/da570e49-a6d2-4faa-a36d-9cb5b5386c7a/scratchpad/.
Work exclusively inside the bead worktree listed under Identity.

## Expected commit shape
```
impl(mindspec-ta30.4): per-verb agreement table + row fixtures + ADR strike

- project-docs/user/guides/lifecycle-modes.md: the table + carve-outs +
  legacy shapes + writer taxonomy (carry-forward items 1-4)
- internal/phase/agreement_table_test.go: per-row fixtures, live arms
- ADR-0023: supersede-annotate the two stale edge-case rows; table ref
- table/fixture cross-pin

Deviations: <list any prompt items you couldn't address literally, or "none">
```

## Report back with
- Commit SHA on `bead/mindspec-ta30.4`.
- Test counts: passed / failed / skipped for the bead's test scope
  (`internal/phase` + the full `go test ./... -short` result line).
- Flagged deviations (one line each + reason) — explicitly including any
  row whose tail-surface assertion was deferred to bead 5, and how each
  carry-forward item landed in the delivered guide.
