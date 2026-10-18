# Bead 5 implementation prompt — 128-lifecycle-verb-state-honesty

## Identity
- Bead id: mindspec-ta30.5
- Branch: bead/mindspec-ta30.5
- Worktree: /Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.5
- Spec branch: spec/128-lifecycle-verb-state-honesty

## Read first
- `.mindspec/specs/128-lifecycle-verb-state-honesty/spec.md` — §R2 (all
  four items; R2.1 quoted below), AC-3/AC-4/AC-4b/AC-14 (quoted below).
- `.mindspec/specs/128-lifecycle-verb-state-honesty/plan.md` — §Bead 5
  (steps quoted below) and the "(b) AC-4b seam shape — decided" paragraph
  in the resolutions section.
- The MERGED bead-1/2/3/4 code in THIS worktree — landed API is the
  authority. Especially: bead 3's draft-discovery surfaces in
  `internal/resolve`/`internal/phase`, and bead 4's
  `internal/phase/agreement_table_test.go` + the delivered
  `project-docs/user/guides/lifecycle-modes.md`.

## WRITTEN SCOPE EXTENSION (tally decision, carried from bead-3's panel —
BINDING, quoted from the orchestrator's dispatch)
"bead 5's declared step list is EXTENDED to include
`internal/instruct/run.go`, closing the two recorded ownership-gap
windows: (a) main-with-draft → standalone `mindspec instruct` renders
idle while state show says spec — run.go must consume the bead-3
draft-discovery surfaces; (b) run.go's bd-failure path currently falls to
handleNoState idle — must render the indeterminate/refusal form this bead
builds. AND the prerequisite recorded by bead-3 R3: derive.go's
in-worktree fabricating fallbacks (ModeImplement/ModeSpec back-filled on
bd error, ResolveContextFromDir never erroring) make the planned 'context
error or empty phase' indeterminate trigger UNREACHABLE — bead 5 must
de-swallow/rewire emitInstruct's input path so the indeterminate form
actually triggers (this was always implicit in R2.3; now it's explicit)."
Practical consequence: before writing the indeterminate template, trace
`emitInstruct` → `phase.ResolveContext` → `ResolveContextFromDirWithCache`
and give the path an error/indeterminate channel (error return, or a
context field distinguishing derived-empty from failed) so a bd failure
REACHES the new rendering instead of being back-filled into a fabricated
mode — the same de-swallowing discipline beads 1-3 applied elsewhere.
`mindspec instruct` (run.go) and the verb tails must agree with
`state show` on the same states (bead 3's honesty), including drafts.

## Files in scope (plan §Bead 5 steps quoted verbatim, plus the extension)
1. "`cmd/mindspec/instruct_tail.go`: `emitInstruct` distinguishes
   indeterminate from idle (R2.3) — context error or empty phase renders
   the indeterminate block (\"cannot determine mode: <reason>\" + ADR-0035
   final `recovery:` line; template
   `internal/instruct/templates/indeterminate.md`), NEVER `idle.md`."
2. "`cmd/mindspec/spec.go`: spec-create tail renders spec-mode guidance
   (rides bead 3's draft derivation through `emitInstruct`); AC-3's
   pinning test (idle \"Available Actions\" block absent from success
   output)."
3. "`internal/complete`: the AC-4b seam (`completeFetchChildrenFn`),
   `advanceState` error return, `Result.NextModeIndeterminate` + reason;
   `FormatResult` renders the indeterminate form; step-6.5 skips its
   write on indeterminate; ModePlan message reworded to fit both true
   triggers (exact wording landed here, satisfying the plan-time
   disposition)." The two surviving triggers (from the spec's complete
   row, which bead 4's table now documents): the empty-lifecycle-subset
   fallback, and the all-open branch via completing a non-lifecycle
   child — the reworded message must be true for BOTH; "Remaining beads
   are blocked" is true for neither and dies here.
4. "`completeTail` branch test (AC-4's two mutually exclusive branches)."
5. "`cmd/mindspec/verb_output_test.go`: AC-14's table-driven
   result-before-guidance test over all nine verbs + the total cobra-tree
   classification (mutating-with-row vs rationale-carrying non-mutating
   allowlist)."
6. EXTENSION: `internal/instruct/run.go` — (a) the main-with-draft window
   consumes bead 3's draft-discovery surfaces (standalone `mindspec
   instruct` renders spec-mode guidance for a draft, agreeing with
   `state show`); (b) the bd-failure path renders the indeterminate/
   refusal form instead of falling to `handleNoState` idle.
7. PREREQUISITE: `internal/phase/derive.go` — de-swallow the in-worktree
   fabricating fallbacks feeding `emitInstruct`'s input path (the
   back-filled ModeImplement/ModeSpec on bd error) so the indeterminate
   trigger is REACHABLE; `state show`'s bead-3 behavior and the bead-4
   row fixtures must stay green (their states are non-erroring — only
   the ERROR channel changes).

## Spec R2.1 — the nine-verb result-first contract (verbatim)
"Every verb that mutates durable lifecycle state — `spec create`,
`spec approve`, `plan approve`, `next`, `complete`, `impl approve`,
`impl adopt`, **`release`, and `repair phase`** (round-1 G7: the list's
membership criterion is durable-state mutation, not tail emission, and
those two mutate bd state) — prints an unambiguous success result
*before* any guidance output, naming what changed (already true for
`complete`'s summary and `spec create`'s \"Spec initialized\" lines —
those orderings become pinned by test rather than incidental). The
mode-transitioning verbs emit the instruct tail per R2.2/R2.3; `release`
and `repair phase` emit no tail today — their result lines satisfy this
requirement and become pinned (`release`'s claim-reversal summary;
`repair phase`'s existing `Phase for <spec> repaired: \"<old>\" ->
\"<new>\"` line, `cmd/mindspec/repair.go:199-205`, extended under R3.3 to
name the recorded reason on a forced backward write) — and if either
later gains a tail it follows R2.2/R2.3."

## Spec ACs this bead claims (quoted verbatim)
- AC-3: "`mindspec spec create <slug>` success output, in order: result
  lines (\"Spec initialized\", worktree/branch), then guidance whose
  rendered template is the spec-mode template — a test pins that
  `idle.md`'s \"Available Actions\" block is absent from spec-create
  success output."
- AC-4: "Two mutually exclusive branches, each asserted only in its own
  driving case (round-2 G11 — a tail that always renders the
  indeterminate marker fails the first branch): when context resolution
  *succeeds*, `completeTail`'s instruct tail agrees with
  `Result.NextMode` and contains no indeterminate marker; when context
  resolution *fails to yield a phase*, the tail contains the
  indeterminate marker text, does not contain `idle.md` guidance, and
  ends with a copy-pastable final `recovery:` line naming the resolving
  action (ADR-0035 — round-1 G6.3). The test drives both branches.
  Ordering (summary before tail) is pinned by a unit test over the real
  wiring."
- AC-4b: "A failing post-close child read, driven via a child-read seam
  through `complete`'s real result path (today's direct
  `phase.FetchChildren` call at `complete.go:1434` gains the seam as part
  of the fix): the result summary and the tail both render the
  indeterminate form — no fabricated `plan` appears in either — and no
  phase sync writes a value derived from the failed read (the step-6.5
  `completeMergeMetadataFn` write at `complete.go:1366` is absent or
  carries no `mindspec_phase` fabricated from the nil child set)."
  (Line numbers were pinned at the spec base — beads 1/2 moved this
  code; re-locate by content: advanceState's child read and the step-6.5
  locked sync write.)
- AC-14: "A table-driven test covers every verb in R2.1's list
  (`spec create`, `spec approve`, `plan approve`, `next`, `complete`,
  `impl approve`, `impl adopt`, `release`, `repair phase`), asserting
  each prints its result summary before any guidance output on success;
  the two tail-less verbs' rows assert the pinned result line itself
  (`release`'s claim-reversal summary; `repair phase`'s
  repaired/consistent line). Anti-drift is mechanized, not hoped for
  (round-2 G10b …): the test walks the registered cobra command tree and
  requires every command to be classified — present in the
  mutating-verbs row set or in an explicit rationale-carrying
  non-mutating allowlist — so registering ANY new command without
  classifying it is red, and classifying it as mutating without a row is
  red."

## The decided AC-4b seam shape (plan, binding)
"a package-level function seam `completeFetchChildrenFn =
phase.FetchChildren` in `internal/complete` (the exact house pattern of
`completeMergeMetadataFn`, `complete.go:46`), replacing the direct call;
`advanceState` gains an error return (`(mode, nextBead string, err
error)`); `Run` maps a non-nil error to `Result.NextModeIndeterminate =
true` + reason — `FormatResult` renders the indeterminate form and
step-6.5 SKIPS its phase write entirely (no value fabricated from a nil
child set)."

## Bead-4 interaction — EXPECTED red, by design
Bead 4's `agreement_table_test.go` structurally cell-pins tail-template
selection per row, and the delivered `lifecycle-modes.md` documents it.
Your tail changes WILL turn those pins red — that is the anti-drift gate
working as designed, not a regression: update the pinned records AND the
guide's affected rows IN THE SAME COMMIT as the behavior change, keeping
table/fixture/doc in lockstep. List every updated pin in Deviations only
if a row's MODE changed (template-content updates within the same mode
need no deviation note).

## Hazards and fixture discipline
- Error-discarding class (hard-blocked beads 1 AND 2 at round 1; the
  scope extension exists BECAUSE of the same class in derive.go): every
  new read propagates; the indeterminate form renders a REAL reason,
  never a fabricated mode; "cannot determine" is never presented as
  idle.
- Marker discipline: any fixture needing stored/derived states is
  constructed marker-PRESENT (`mindspec_phase_semantics: "v2"`);
  marker-absent states belong to the bead-2 legacy fixtures only.
- Hermeticity: fixture roots in `t.TempDir()`; suite green FROM the bead
  worktree; the AC-14 table drives verbs via their RunE/tail functions
  with seams, not via a live lifecycle in this repo.
- The indeterminate tail must not fail its verb: the VERB's success exit
  code is unchanged when only the tail is indeterminate (the refusal
  form belongs to `state show`, bead 3); AC-4's failure branch asserts
  tail CONTENT, not verb failure.

## Shared helpers to REUSE (do not reimplement)
- Bead 3's draft-discovery surfaces (`internal/resolve` +
  `internal/workspace` probes) — run.go's window (a) consumes these.
- `instruct.BuildContext`/`instruct.Render` + the existing template set;
  add `indeterminate.md` beside them following the house template shape
  (idle.md/review.md front-matter and structure).
- `completeMergeMetadataFn` (the seam pattern AND the locked step-6.5
  write your skip-on-indeterminate guards), `guard.HasFinalRecoveryLine`
  (the recovery-line format authority), `termsafe.Escape`/`idrender` for
  any new message text (ADR-0042).
- Bead 4's fixture builders in `agreement_table_test.go` for the states
  your AC-4 success branch drives.

## Guardrails
AGENTS.md § Bead-loop guardrails applies in full: no `mindspec complete`,
no `git push`, no raw `git merge`, no scope creep beyond the files above
(the WRITTEN extension included), no reimplementing landed helpers,
exactly ONE commit whose message ends with `Deviations: <list or
"none">`, and tests must PASS — `go build ./... && go test ./... -short`
green FROM the bead worktree (`-short` mandatory — spec settled fact 3).
Absolute paths; scratch ONLY under
/private/tmp/claude-503/-Users-Max-replit-mindspec/da570e49-a6d2-4faa-a36d-9cb5b5386c7a/scratchpad/.
Work exclusively inside the bead worktree listed under Identity.

## Expected commit shape
```
impl(mindspec-ta30.5): verb output honesty — tails, indeterminate form, AC-14

- instruct_tail.go + templates/indeterminate.md: indeterminate vs idle
- derive.go: de-swallow fabricating fallbacks (trigger reachable)
- instruct/run.go: draft window + bd-failure indeterminate (extension)
- spec.go tail: spec-mode guidance; AC-3 pin
- internal/complete: AC-4b seam, NextModeIndeterminate, ModePlan reword
- verb_output_test.go: nine-verb result-first + total cobra classification
- agreement table pins/doc rows updated in lockstep

Deviations: <list any prompt items you couldn't address literally, or "none">
```

## Report back with
- Commit SHA on `bead/mindspec-ta30.5`.
- Test counts: passed / failed / skipped for the bead's test scope
  (`internal/instruct`, `internal/complete`, `internal/phase`,
  `cmd/mindspec`, plus the full `go test ./... -short` result line).
- Flagged deviations (one line each + reason) — explicitly including:
  which agreement-table pins/doc rows were updated and why; how the
  derive.go de-swallowing kept bead-3/bead-4 tests green; the exact
  ModePlan rewording chosen.
