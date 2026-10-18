# Bead 9 implementation prompt — 128-lifecycle-verb-state-honesty

## Identity
- Bead id: mindspec-ta30.9
- Branch: bead/mindspec-ta30.9
- Worktree: /Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.9
- Spec branch: spec/128-lifecycle-verb-state-honesty (forked from bed348c2)

## Read first
- `.mindspec/specs/128-lifecycle-verb-state-honesty/spec.md` — §R6.4 (bd
  stderr passthrough) and AC-11 (quoted below).
- `.mindspec/specs/128-lifecycle-verb-state-honesty/plan.md` — §Bead 9
  (quoted verbatim below).
- `internal/bead/bdcli.go` in THIS worktree — the bd exec seam this bead
  hardens.

## PARALLEL-TRACK BOUNDARY (binding)
Bead 7 is running concurrently and owns `cmd/mindspec/next.go` and
`internal/next/guard.go`. This bead touches NEITHER — its surfaces are
exactly `internal/bead/bdcli.go` (+test) and `internal/next/beads.go`
(+test). `beads.go` shares the `internal/next` package with bead 7's
`guard.go` but neither bead changes the other's files or any shared
export. If you find yourself needing an edit in `next.go` or `guard.go`,
STOP and report instead of editing.

## Files in scope (from plan §Bead 9, quoted verbatim)
1. "`internal/bead/bdcli.go`: on `cmd.Output()` failure, surface
   `exec.ExitError.Stderr` content through the returned error (a bounded,
   line-capped excerpt — cap pinned here)."
   Grounding: the bd exec layer lives in this package (execution domain
   per OWNERSHIP) — `execCommand` seam var (`bdcli.go:25-27`), `RunBD`
   (`:70`) → `tracedOutput` (`cmd.Output()`), which already captures
   stderr into `exec.ExitError.Stderr` and discards it via the
   `%w`-of-exit-status wrapping. Pin the excerpt cap (line count and/or
   byte bound) as a named constant with a doc comment; the error text
   carries the excerpt plus a truncation marker when capped.
2. "`internal/next/beads.go`: the `QueryReady`/`QueryReadyForEpic`
   wrapping errors carry the surfaced stderr; every surfaced line routes
   through `termsafe.Escape` (ADR-0042)."
   Rendering discipline: bd stderr is AGENT-WRITABLE hostile bytes — a
   compromised or hostile bd (or a shim) can emit ANSI escapes, carriage
   returns, and control bytes; every surfaced line is escaped
   INDIVIDUALLY (per-line, the house pattern from `DirtyTreeFailure`'s
   porcelain-filename handling) so a hostile line cannot forge extra
   lines or control the terminal. The escape happens at the RENDER
   boundary (the wrapping error message construction), not inside the
   capture.
3. "AC-11's two legs: content (schema-error line present) + rendering
   (multi-line, control-bearing stderr — escaped, no raw control bytes)."
   The #146 motivating case: a stale bd binary's
   `Error 1105 (HY000): column "depends_on_id" could not be found …`
   schema error was captured and thrown away, leaving only
   `exit status 1`. The content leg's fake bd emits exactly that class
   of line.

## Spec AC this bead claims (quoted verbatim)
"**AC-11 (R6)**: A failing bd shell-out behind `next` (fake bd exiting 1
with a schema-error line on stderr) produces a mindspec error containing
that stderr line. Rendering leg (round-2 G4, ADR-0042): the fake bd's
stderr additionally includes multi-line output with at least one
control-bearing/termsafe-sensitive line — the legitimate schema-error
text survives into the rendered error, every surfaced stderr line routes
through `termsafe.Escape`, and no raw control byte reaches the rendered
output (an unsafe raw-passthrough implementation fails this leg while
passing the content leg)."

## Landed-surface notes (verified in this worktree)
- The bd exec seam is `internal/bead`'s `execCommand` package var — the
  fake bd for AC-11 is injected there (or via a PATH shim in a hermetic
  fixture); `runBDFn` in `internal/next/beads.go` is the higher seam the
  `QueryReady` tests already use. Prefer the existing seams; add none.
- `tracedOutput`/`tracedCombined` (`bdcli.go`) are the two capture
  shapes; the passthrough lands where `cmd.Output()`'s error is wrapped
  so EVERY `RunBD` caller benefits — but the AC's assertion surface is
  `next`'s error path specifically; do not rewrite unrelated callers'
  messages beyond what the shared wrapping gives them.
- Beads 1-6's landed code is untouched by this bead.

## Hazards
- Error-discarding (the run's recurring hard-block class): the surfaced
  stderr must reach the RENDERED error — capturing it into a struct
  nobody prints fails the content leg.
- Hermeticity: fixture repos and fake-bd shims in `t.TempDir()`; suite
  green FROM the bead worktree.
- Both suites are part of the gate: run the full `go test ./... -short`
  AND the bead-scope packages (`internal/bead`, `internal/next`), and
  ALSO the no-short `cmd/mindspec` suite with `-timeout 30m` — report
  all result lines.
- Interim statuses on long runs; stop-and-diagnose rather than raising
  timeouts further.
- Scratch ONLY under
  /private/tmp/claude-503/-Users-Max-replit-mindspec/da570e49-a6d2-4faa-a36d-9cb5b5386c7a/scratchpad/
  and cleaned up when done.

## Guardrails
AGENTS.md § Bead-loop guardrails applies in full: no `mindspec complete`,
no `git push`, no raw `git merge`, no scope creep beyond the two files +
tests above, no reimplementing existing seams, exactly ONE commit whose
message ends with `Deviations: <list or "none">`, and tests must PASS.
Absolute paths everywhere. Work exclusively inside the bead worktree
listed under Identity.

## Expected commit shape
```
impl(mindspec-ta30.9): bd stderr passthrough with safe rendering

- bdcli.go: ExitError.Stderr surfaced (line-capped, pinned constant)
- beads.go: QueryReady errors carry per-line termsafe-escaped stderr
- AC-11 content + rendering legs

Deviations: <list any prompt items you couldn't address literally, or "none">
```

## Report back with
- Commit SHA on `bead/mindspec-ta30.9`.
- ALL suite result lines: full `go test ./... -short`, the bead-scope
  packages, and the no-short `cmd/mindspec` run (`-timeout 30m`), with
  passed/failed/skipped counts.
- The pinned cap value chosen and its rationale line.
- Flagged deviations (one line each + reason).
