# Bead 8 implementation prompt — 128-lifecycle-verb-state-honesty

## Identity
- Bead id: mindspec-ta30.8
- Branch: bead/mindspec-ta30.8
- Worktree: /Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.8
- Spec branch: spec/128-lifecycle-verb-state-honesty (forked from the
  current tip — beads 1-7 AND 9 are all merged beneath you)

## Read first
- `.mindspec/specs/128-lifecycle-verb-state-honesty/spec.md` — §R6.1-R6.3,
  §R6.7, §OQ-5's narrowed record, and the four ACs quoted below.
- `.mindspec/specs/128-lifecycle-verb-state-honesty/plan.md` — §"OQ-5 —
  default-pick rule" (the resolution) and §Bead 8 (quoted verbatim below,
  with moved-surface corrections).
- `cmd/mindspec/next.go` and `internal/next/beads.go` IN THIS WORKTREE —
  bead 7 heavily rewrote the former and bead 9 the latter; the CURRENT
  code is the structural authority, and the plan's line references into
  next.go are STALE (see "Landed surfaces MOVED" below).

## The OQ-5 resolution (plan, binding — do not relitigate)
Default-pick rule = DISPLAYED ROW 1, bd ready's priority-first order,
printed self-describingly: `Defaulting to item 1 (bd ready priority
order). Use --pick=N or a bead ID to select a specific item.` Rationale
(recorded): dependency edges own plan order via readiness gating;
simultaneously-ready beads are order-free by the plan's own declaration;
a lowest-bead-number default would re-derive plan order from ID
lexicography — the dual-derivation anti-pattern R6.5 struck.

## Files in scope (from plan §Bead 8, quoted verbatim; corrections below)
1. "`internal/next/select.go`: total pick semantics (single-row
   validation — the `select.go:20-22` early return goes; 0=unset;
   negative rejected); ONE selection function consumed by both paths. The
   shared ready-set CONSTRUCTION (query + spec filter + exclusion
   accounting, R6.1's 'one set-construction') lands beside `QueryReady`
   in `internal/next/beads.go` (round-1 G4.3 — this is chunk 8's
   `beads.go` entry)."
2. "`cmd/mindspec/next.go`: `runEmitOnly` receives `--pick` and builds
   the SAME ordered set via the same construction as the claim path; the
   positional emit-only bypass keeps `FetchBeadByID` + prints the pinned
   bypass line; the OQ-5 default line ('Defaulting to item 1 (bd ready
   priority order)…'); filter loudness R6.3(a)/(b)/(c) — exclusion
   count + reason on every set-constructing path, loud error on
   empty-after-filter, the honest positional exclusion error."
3. "`internal/resolve/target.go` + `cmd/mindspec/next.go`: R6.7 scope
   honesty — explicit `--spec` resolution failure (prefix no-match /
   epic lookup error / draft-no-epic) refuses on both paths; the silent
   global-`bd ready` fallbacks (`next.go:166-171`, `:417-424`) are
   removed; `ResolveSpecPrefix` full-ID passthrough gains the existence
   proof." CORRECTION: those line numbers predate bead 7's rewrite —
   re-locate the two fallback sites by content (the
   `FindEpicBySpecID`-fails → `QueryReady()` global-fallback branches on
   the claim and emit-only paths). Note the draft-no-epic refusal
   message should steer honestly: bead 3 made drafts first-class, so
   "spec exists but has no epic; run `mindspec spec approve <id>` first"
   is the truthful shape.
4. "AC-9/AC-10/AC-10b/AC-15 fixtures (including the seam-recorded
   no-global-query assertion)."
5. FOLDED-IN FOLLOW-UP (ta30.9's deferral — bead 8 now owns next.go, so
   this bead closes it; say so in the commit): apply bdRenderedError-style
   safe rendering at next.go's two remaining raw-`%v` bd-error render
   legs, verified live in this worktree: (a) the
   `RecordReadinessOverride` failure message inside `guard.NewFailure`
   (`next.go:307-310` — `%v` on a bd-derived error); (b)
   `describePhaseSyncFailure`'s non-timeout leg (`next.go:622` — `%v` on
   a bd/lock-derived error). Route both through the bead-9 rendering
   discipline (per-line `termsafe.Escape` of surfaced content — reuse
   `bdRenderedError` or its helper, do not fork a second escaping path).

## Landed surfaces MOVED — the current structure is the authority
(verified live in this worktree)
- Bead 7 rewrote `cmd/mindspec/next.go`: Step 0d resolves the target
  spec AHEAD of the guard (`next.go:121`), the guard is
  `next.CheckDirtyTreeProportional(root, cwd, specFlag)` returning
  (blockingDirt, ignoredDirt) (`:163`) with the Max-directive notice —
  your step-2/3 changes graft onto THIS structure; quote/extend the
  current step comments, never resurrect the plan's pre-b7 shape.
- Bead 9 wrapped `QueryReady`/`QueryReadyForEpic` errors in
  `bdRenderedError` (`internal/next/beads.go:39/:55/:61`) with
  red-on-revert pins — your shared set-construction refactor MUST
  preserve that wrapping and keep bead 9's tests green unmodified.
- Bead 2's `advancePhaseOnClaim` + its `describePhaseSyncFailure`
  rendering and `nextPhaseAdvanceRecoveryCommand` live in next.go —
  untouched except follow-up leg (b) above.
- `SelectWorkByName`'s exact-first/ambiguity-safe resolution (spec
  101/103) stays; the false "not found or not ready" message for
  filter-excluded beads is what R6.3(b) replaces.

## Spec ACs this bead claims (quoted verbatim)
- "**AC-9 (R6)**: With ≥2 ready beads: `--emit-only --pick 1` and
  `--emit-only --pick 2` emit different beads matching claim-path
  selection for the same N; `--pick <len+1>` errors 'out of range' on
  both paths; selection order equals the `bd ready --parent` returned
  order (fixture with priorities P1/P2 in #187's shape). Pick boundary
  semantics on both paths (round-1 G10): with exactly ONE ready bead, an
  explicit out-of-range `--pick` (e.g. `--pick 2`) errors rather than
  silently selecting the sole bead (red-on-revert: `SelectWork`'s
  single-item early return at `select.go:20-22` bypasses pick validation
  today); `--pick 0` means unset (the default path), and a negative
  `--pick` is rejected with an error rather than silently ignored. The
  no-pick default path prints the self-describing line naming the
  ordering and default rule R6.2 requires (round-1 G5)."
- "**AC-10 (R6)**: A positional bead ID present in `bd ready --parent`
  output but excluded by mindspec's spec filter produces an error naming
  the exclusion reason; the string 'not found or not ready' does not
  appear for that case. On every path that constructs the ready set
  (default, `--pick`, positional-claim, no-positional `--emit-only`), a
  partial filter exclusion prints the excluded count and reason
  alongside the displayed list — the displayed-list wording applies only
  to paths that actually display one. Positional `--emit-only` is
  R6.3(a)'s one documented exception (round-3 G3): it performs no ready
  query and no filtering, reports no exclusions, and must print only the
  pinned readiness-bypass disclosure."
- "**AC-10b (R6.3(c))**: Fixture: `bd ready --parent` returns ≥1 bead,
  none of whose titles reference the spec ID. `mindspec next --spec <s>`
  (and the `--emit-only` variant) exits non-zero with an error naming
  the excluded beads and the reason; the output does not contain 'No
  ready work found'. Red-on-revert: today's behavior prints exactly that
  success message."
- "**AC-15 (R6.7 — round-2 G8)**: Explicit-scope honesty on both paths:
  (i) `mindspec next --spec <full-ID-of-nonexistent-spec>` (claim and
  `--emit-only`) exits non-zero with a scope-resolution error naming the
  spec, and no global `bd ready` query is issued (asserted via the bd
  seam's recorded calls); (ii) `--spec <existing-draft-spec>` (no epic)
  refuses with the no-epic/not-approved message and never selects a bead
  from another spec; (iii) red-on-revert: today's behavior silently
  falls back to global `bd ready` and can emit/claim an out-of-scope
  bead."

## Hazards (each hard-blocked an earlier bead — do not repeat)
- Error-discarding: every read propagates; exclusion accounting counts
  REAL exclusions from the real filter, never a recomputation.
- Rendering: excluded-bead titles and reasons are agent-writable —
  per-line `termsafe.Escape`/`idrender` on every new message (the bead-9
  discipline); no raw `%v` on bd-derived errors (the folded follow-up
  removes the last two).
- Hermeticity: fixture repos + bd seams in `t.TempDir()`; suite green
  FROM the bead worktree; AC-15's no-global-query assertion uses the
  recorded-calls seam, never a live bd.
- Both suites are part of the gate: full `go test ./... -short`, the
  bead-scope packages (`internal/next`, `internal/resolve`,
  `cmd/mindspec`), AND the no-short `cmd/mindspec` suite with
  `-timeout 30m` — report all result lines.
- Interim statuses on long runs; stop-and-diagnose rather than raising
  timeouts. Scratch ONLY under
  /private/tmp/claude-503/-Users-Max-replit-mindspec/da570e49-a6d2-4faa-a36d-9cb5b5386c7a/scratchpad/
  and cleaned when done.

## Guardrails
AGENTS.md § Bead-loop guardrails applies in full: no `mindspec complete`,
no `git push`, no raw `git merge`, no scope creep beyond the files above
(the folded follow-up included), no reimplementing landed helpers
(bdRenderedError, SelectWorkByName, CheckDirtyTreeProportional,
advancePhaseOnClaim all stay), exactly ONE commit whose message ends with
`Deviations: <list or "none">`, and tests must PASS. Absolute paths.
Work exclusively inside the bead worktree listed under Identity.

## Expected commit shape
```
impl(mindspec-ta30.8): deterministic selection + scope honesty (OQ-5)

- select.go: total pick semantics; one selection fn both paths
- beads.go: shared ready-set construction w/ exclusion accounting
- next.go: emit-only pick parity, OQ-5 default line, filter loudness,
  scope-refusals (global fallbacks removed), positional bypass line
- target.go: ResolveSpecPrefix existence proof
- next.go render legs hardened (closes ta30.9 follow-up)
- AC-9/AC-10/AC-10b/AC-15 fixtures

Deviations: <list any prompt items you couldn't address literally, or "none">
```

## Report back with
- Commit SHA on `bead/mindspec-ta30.8`.
- ALL suite result lines: full `-short`, bead-scope packages, and the
  no-short `cmd/mindspec` run (`-timeout 30m`), with counts.
- The exact default-line and bypass-line texts as landed.
- Flagged deviations (one line each + reason) — explicitly including how
  the b7 structure absorbed the scope-refusal changes and confirmation
  the b9 pins stayed green unmodified.
