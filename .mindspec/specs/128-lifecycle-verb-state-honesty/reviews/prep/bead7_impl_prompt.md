# Bead 7 implementation prompt — 128-lifecycle-verb-state-honesty

## Identity
- Bead id: mindspec-ta30.7
- Branch: bead/mindspec-ta30.7
- Worktree: /Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.7
- Spec branch: spec/128-lifecycle-verb-state-honesty

## Read first
- `.mindspec/specs/128-lifecycle-verb-state-honesty/spec.md` — §R5 (all
  three items — the rebuilt guard with its DERIVED-EMPTY enumeration),
  §R3.6 (safe replay), AC-8, AC-16, and the ADR-0025 fitness entry (the
  amendment is MANDATORY, both directions — the no-amendment leg was
  removed at spec-gate round 2, with the round-1 G6 scope fence: the
  amendment is expressly limited to the worktree-creating `mindspec next`
  claim path; `approve`'s and every other dirty-tree guard's semantics
  are unchanged).
- `.mindspec/specs/128-lifecycle-verb-state-honesty/plan.md` — §Bead 7
  (steps quoted verbatim below) and §"R5.1" in the spec for the derived
  enumeration's rationale.
- The MERGED bead-1..6 code in THIS worktree — landed API is the
  authority.

## MAX'S STANDING DIRECTIVE (operator directive with the force of a
requirement — 2026-09-02, bd key `spec-128-bead7-warning-note`, verbatim)
"bead 7 (mindspec-ta30.7, #179 claim guard) must additionally print an
informational notice naming the ignored unrelated dirt while PROCEEDING
with the claim (visibility without the block). Fold into bead7 Phase A
prompt; one-line diagnostic, pinned by test per the spec's message
discipline; compatible with AC-8's success leg."
Implementation notes: ONE line, on stderr (informational diagnostic, not
part of the result contract), naming the ignored dirty paths — porcelain
filenames are agent-writable, so each path routes through
`termsafe.Escape` (ADR-0042, the exact discipline `DirtyTreeFailure`
already applies); the claim PROCEEDS (exit unchanged); the AC-8(i)
success fixture asserts BOTH the success and the notice; the notice never
appears when the invoking tree is clean.

## Files in scope (from plan §Bead 7, quoted verbatim)
1. "`internal/next/guard.go` + `cmd/mindspec/next.go`: remove the
   invoking-worktree user-dirt block for the worktree-creating claim path
   (R5.1's derived-empty enumeration); keep the pinned in-code
   claim-written-paths list (empty beyond exempt artifacts) + the
   enumeration-pinning test + the seam-injected refusal leg preserving
   the `DirtyTreeFailure` contract."
   Context you must not lose: the enumeration was DERIVED at plan time —
   the claim writes only `.mindspec/session.json` (gitignored,
   `.gitignore:3` — can never appear as blockable dirt), the ADR-0025
   named artifact `.beads/issues.jsonl` (exempt + auto-handled), and the
   bead worktree directory (created AFTER the guard runs) — so the
   entangleable user-dirt set is EMPTY today and the block is REMOVED,
   not narrowed to a vacuous branch; the in-code list + pinning test
   re-arms the block if a future claim step writes a new tracked path.
2. "`internal/approve/spec.go:76`: R3.6 absent-only backfill under the
   bead-1 lock (write `plan` only when stored phase is empty); replay
   output distinguishes backfill / no-op / later-phase-untouched (the
   fail-closed alternative is NOT taken — a replay is a legitimate
   idempotent re-run; refusing it would break `spec approve`'s existing
   idempotence contract at `spec.go:73-74`)."
   (Line numbers were pinned at the spec base — bead 1 lock-wrapped this
   site; re-locate by content: the existing-epic `mindspec_phase: "plan"`
   merge-write inside `WithPhaseWriteLock`. The `spec.go:84` creation
   seed stays untouched and outside the lock.)
3. "`.mindspec/adr/ADR-0025-jsonl-as-build-artifact.md`: the mandatory
   clarifying amendment (decision item 1 scoped to entangleable dirt)."
   Scope fence (spec, binding): expressly limited to the
   worktree-creating `mindspec next` claim path — name that limit in the
   amendment text.
4. "AC-8 and AC-16 fixtures (all AC-16 legs: implement/review/done
   variants, same-value plan replay, empty-phase backfill)."

## Spec ACs this bead claims (quoted verbatim)
- "**AC-8 (R5)**: Three legs (re-scoped for the empty enumeration,
  round-1 G8): (i) with an unrelated dirty user file in the invoking
  checkout and no active bead worktree, a worktree-creating claim
  succeeds; (ii) an enumeration-pinning test derives the paths a real
  claim run writes in the invoking checkout (fixture repo) and asserts
  the in-code claim-written-paths list matches — red when a future claim
  step writes a new tracked path the list omits; (iii) seam leg, so the
  refusal machinery stays live while the real enumeration is empty: with
  a path injected into the enumeration and dirt on that path, the claim
  refuses with the preserved `DirtyTreeFailure` contract."
  (Per Max's directive, leg (i) additionally asserts the one-line
  ignored-dirt notice.)
- "**AC-16 (R3.6 — round-4 G2, legs completed round-5 G3)**: Fixture: an
  existing lifecycle epic with stored `mindspec_phase=implement`
  (variants: `review`, and `done` — a closed-out epic's replay likewise
  leaves `done` unchanged with the honest no-op/refusal output).
  Re-running `mindspec spec approve` for that spec leaves the stored
  phase at the later value — never regressed to `plan` — and the output
  names the no-op or refusal per R3.6's chosen disposition,
  distinguishing it from a real backfill mutation. A same-value leg:
  stored phase already `plan` — the replay's output identifies that
  nothing was mutated and does not claim a backfill occurred. A separate
  leg pins the legitimate backfill: an existing epic with EMPTY stored
  phase gets `plan` written (under the R3.3 lock) and the output says so.
  Red-on-revert: today's `spec.go:76` write regresses `implement` to
  `plan` on replay."
  (The chosen disposition is ABSENT-ONLY BACKFILL — no refusal path
  exists for a later-phase replay; the output for those legs is the
  honest no-op naming the untouched stored phase.)

## Live evidence — #179 bit THREE times in this very autopilot run
(quote from the orchestrator's record; mirror these REAL shapes in the
AC-8 fixtures rather than inventing synthetic ones)
1. Untracked handover files in the main checkout blocked the FIRST claim
   of the run.
2. Untracked panel artifacts (reviews/ trees) blocked the claim before
   bead 4.
3. A claim attempted from the main checkout with unrelated tracked-file
   edits was blocked.
Fixture translation: leg (i) variants cover (a) an untracked unrelated
file, (b) an untracked directory tree, (c) a modified tracked file — all
in the invoking checkout, all proceeding post-fix with the notice naming
them.

## Landed-surface interactions
- Bead 1's `phase.WithPhaseWriteLock` is the R3.6 backfill's instrument
  (the site is already wrapped — this bead changes the VALUE POLICY to
  absent-only + the output honesty).
- Bead 2's `EnsurePhaseSemanticsV2` runs at `spec approve`'s
  ahead-of-preflight sites? NO — its invocation sites are
  impl/plan/complete/next/repair/release; `spec approve` is NOT one, and
  this bead does not add it. The AC-16 fixtures construct their epics
  MARKER-PRESENT (`mindspec_phase_semantics: "v2"`) per the standing
  fixture discipline, so no reconciliation interference is possible
  either way.
- Bead 6's snapshot/marker machinery is UNTOUCHED by this bead.
- The `DirtyTreeFailure` message contract (worktree-context line, final
  recovery line, steer-to-active-worktree) is preserved VERBATIM on the
  seam-injected refusal leg — do not reword it.

## Hazards (each hard-blocked an earlier bead — do not repeat)
- Error-discarding: every read propagates; the enumeration-pinning test
  derives real claim writes, never assumes them.
- Hermeticity: fixture repo roots in `t.TempDir()`; suite green FROM the
  bead worktree; the AC-8(ii) real-claim-run fixture builds its own repo
  and bd seams — it must not claim in THIS repo.
- Both suites are part of the gate: run the full `go test ./... -short`
  AND the bead-scope packages (`internal/next`, `internal/approve`,
  `internal/phase`, `cmd/mindspec`) and report both result lines.
  `-short` mandatory (spec settled fact 3).
- Interim statuses on long runs; stop-and-diagnose rather than raising
  timeouts.
- Scratch ONLY under
  /private/tmp/claude-503/-Users-Max-replit-mindspec/da570e49-a6d2-4faa-a36d-9cb5b5386c7a/scratchpad/
  and cleaned up when done.

## Shared helpers to REUSE (do not reimplement)
- `next.CheckDirtyTreeDetail`/`classifyDirty`/`artifactPaths` — the
  classifier stays; only the BLOCK decision changes.
- `phase.WithPhaseWriteLock`, the `*MergeMetadataFn` seams,
  `SetMergeMetadataForTest`, `guard.NewFailure`,
  `guard.HasFinalRecoveryLine`, `termsafe.Escape`/`idrender`.
- Bead-2/4 fixture builders for epic states.

## Guardrails
AGENTS.md § Bead-loop guardrails applies in full: no `mindspec complete`,
no `git push`, no raw `git merge`, no scope creep beyond the files above,
no reimplementing landed helpers, exactly ONE commit whose message ends
with `Deviations: <list or "none">`, and tests must PASS. Absolute paths
everywhere. Work exclusively inside the bead worktree listed under
Identity.

## Expected commit shape
```
impl(mindspec-ta30.7): proportional claim guard + spec-approve safe replay

- next: unrelated-dirt block removed for worktree-creating claims;
  ignored-dirt notice (Max directive); enumeration pin + seam refusal leg
- approve/spec.go: absent-only backfill under lock; mutation-vs-no-op output
- ADR-0025 clarifying amendment (next-claim-path fence)
- AC-8 three legs (real-shape variants) + AC-16 five legs

Deviations: <list any prompt items you couldn't address literally, or "none">
```

## Report back with
- Commit SHA on `bead/mindspec-ta30.7`.
- BOTH suite result lines (full `-short` + bead-scope packages), with
  passed/failed/skipped counts.
- The exact one-line notice text chosen (Max's directive) and its test.
- Flagged deviations (one line each + reason).
