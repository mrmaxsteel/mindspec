# Bead 3 implementation prompt — 128-lifecycle-verb-state-honesty

## Identity
- Bead id: mindspec-ta30.3
- Branch: bead/mindspec-ta30.3
- Worktree: /Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.3
- Spec branch: spec/128-lifecycle-verb-state-honesty

## Read first
- `.mindspec/specs/128-lifecycle-verb-state-honesty/spec.md` — §R1 (all
  four items), AC-1, settled fact 5 (ADR-0023's C1-1 ambiguity), settled
  fact 6 (the bd-availability discriminator — quoted below), and the
  live-evidence "#220 … three shapes" paragraph.
- `.mindspec/specs/128-lifecycle-verb-state-honesty/plan.md` — §"OQ-2 —
  draft-mode derivation" (the resolution this bead implements) and §Bead 3
  (steps quoted verbatim below).
- The MERGED bead-1/2 code in THIS worktree — the landed API is the
  authority over any plan shorthand.

## The OQ-2 resolution (plan, binding — do not relitigate)
Pre-epic draft discovery is FILESYSTEM-SCOPED: a spec is in draft (`spec`)
mode when its spec directory + `spec/<id>` branch exist and no lifecycle
epic exists; from `spec approve` (epic creation, the durable approval
record) onward, beads is the sole authority, unchanged. The ADR-0023
amendment is NARROW: it scopes the authority model's "all lifecycle
discovery goes through beads" to post-approval lifecycle — matching the
table row the ADR always promised ("No epic with matching spec_num →
spec") and the mechanism one code path already ships
(`ResolveContextFromDirWithCache`'s `WorktreeSpec` branch,
`derive.go`'s "spec worktree without epic → spec mode").
R1.4's either/or is RESOLVED to the NON-ZERO REFUSAL form for
`state show` (a read-only query fails loudly; no second success shape);
the explicit indeterminate OBJECT is reserved for the instruct tail
(bead 5's surface), which must not fail its verb — do not build the
indeterminate object here.

## Files in scope (from plan §Bead 3, quoted verbatim)
1. "`internal/resolve/resolve.go`: `ResolveModeWithCache`'s no-epic path
   gains the on-disk draft probe (spec dir + `spec/<id>` branch via
   `internal/workspace`) → `state.ModeSpec` for a draft; bd/discovery
   ERRORS propagate instead of collapsing to idle (`resolve.go:60-63`
   today returns `ModeIdle, nil` on lookup error — split error from
   empty)."
2. "`cmd/mindspec/state.go`: recompose so the context derivation is
   authoritative for mode (R1.2) — drop the `ResolveMode`-for-mode /
   context-for-bead split (`state.go:80-97`); the fallback path no longer
   serializes an empty `ctx.Phase`: discovery failure → non-zero ADR-0035
   refusal naming the failed query (R1.4; the plan resolves R1.4's
   either/or to the refusal form for `state show`)."
3. "Main-checkout draft rendering: exactly one draft →
   `{"mode":"spec","activeSpec":…,"specBranch":…}`; multiple drafts → the
   existing ambiguity listing; draft + active epic states unchanged."
4. "`.mindspec/adr/ADR-0023.md`: the draft-discovery exception amendment
   (OQ-2's scoping paragraph)." (ONLY that amendment — the `:107`/`:108`
   row strike is bead 4's; do not touch the derivation table rows.)
5. "Fixtures for AC-1's three legs, red-on-revert against the three
   captured 2026-09-01 outputs."
6. R3.5's TAKEN disposition rides this bead (plan "Delegated
   dispositions"): "as a side effect of bead 3's `state.go` recomposition:
   one derivation authoritative means one `phase.Cache` constructed per
   invocation and passed through the existing `WithCache` variants —
   within R3.5's stated bar (no new exported API)." One Cache per
   `state show` invocation across the previously-three fresh-Cache call
   sites (`resolve.ResolveMode`, `resolve/target.go`, the context path).

## Live evidence the fixtures pin (spec settled fact 6 — the
bd-availability discriminator, all three shapes captured 2026-09-01)
- Main checkout, bd healthy (Dolt query succeeds, zero open epics):
  `mindspec state show` → `{"mode": "idle"}`, exit 0 — while a draft was
  active: DISHONEST, AC-1(iii)'s counterexample.
- Main checkout, bd unreachable
  (`env PATH=/usr/bin:/bin /Users/Max/.local/bin/mindspec state show` —
  MindSpec by absolute path, bd unresolvable): `{"mode": ""}`, exit 0 —
  a discovery failure serialized as success, AC-1(ii)'s counterexample.
- From a spec worktree, BOTH bd conditions:
  `{"mode":"idle","activeSpec":"128-…","specBranch":"spec/128-…"}` —
  idle alongside a non-empty activeSpec, AC-1(i)'s counterexample.
Mechanism (spec, verified): `state.go`'s fallback serializes `ctx.Phase`
from the `WorktreeMain` branch, which sets `idle` only on err==nil with
zero specs and sets NOTHING when the beads query errors
(`derive.go` WorktreeMain arm); `ResolveModeWithCache` returns
`ModeIdle, nil` whenever no epic exists OR the lookup errs.

## Spec ACs this bead claims (quoted verbatim from spec.md)
"**AC-1 (R1)**: Three fixture cases, separately asserted: (i) from a fresh
no-epic draft spec's worktree, `mindspec state show` output contains no
`"mode": "idle"` alongside a non-empty `activeSpec`; the draft's mode
renders per the OQ-2 resolution (default expectation: `"mode": "spec"`) —
red-on-revert counterexample: the 2026-09-01 capture
`{"mode":"idle","activeSpec":"128-…"}`; (ii) from the main checkout with
discovery *failing* (bd shell-out errors, injected via seam), `state show`
does not exit 0 with `"mode": ""` — it exits non-zero with a refusal
naming the discovery failure, or emits the explicit indeterminate result
R1.4 defines — red-on-revert counterexample: the 2026-09-01 capture
`{"mode": ""}` exit 0 under `PATH=/usr/bin:/bin`; (iii) from the main
checkout with discovery succeeding on a draft-only state, the output
renders the honest draft representation, not bare `idle`."
(For (ii), the plan's resolution selects the REFUSAL arm — assert the
non-zero exit + ADR-0035 refusal naming the failed query.)

## Bead-2 landed surfaces you must NOT disturb (verified in this worktree)
- `phase.LegacyPhaseSignature(stored string, children []ChildInfo) bool`
  (`internal/phase/semantics_v2.go:76`) — exactly-open strictness.
- `phase.EnsurePhaseSemanticsV2(root, epicID string) (bool, error)`
  (`semantics_v2.go:126`).
- The legacy NOTICE emission in `derive.go` (~:192: "legacy phase
  semantics detected (pre-v2 stored implement)" + repair-phase recovery)
  and the direction-conditional AC-5(iv) recovery in the retained
  warning. Your `state.go`/`resolve.go` recomposition must keep the
  read paths that reach these emissions intact — the bead-2 tests for
  them must stay green UNMODIFIED (any exception goes in Deviations
  with the reason).

## Named hazard (hard-blocked beads 1 AND 2 at round 1 — do not repeat)
The error-discarding-writer/reader class: every new read (workspace scan,
branch existence, epic lookup, children fetch) PROPAGATES its error —
never `_`-discarded, never collapsed into a fabricated mode. Fresh
per-call `phase.NewCache()` where the landed convention uses one. A
failure to read is a refusal (R1.4's form), never a value.

## Shared helpers to REUSE (do not reimplement)
- `internal/workspace` — spec dir/branch existence probes
  (`workspace.SpecDir`, `workspace.SpecBranch`, the worktree-context
  detection); do not shell raw git for what it already answers.
- `phase.ResolveContextFromDir`/`WithCache`, `phase.NewCache`, the
  `WithCache` derivation variants — the recomposition threads ONE Cache
  through existing APIs (R3.5's bar: no new exported API).
- `guard.NewFailure` (ADR-0035 refusal shape) and the existing
  `resolve.ErrAmbiguousTarget` multi-draft listing path.
- bd seam vars in `internal/resolve`/`internal/phase` for AC-1(ii)'s
  injected discovery failure.

## Guardrails
AGENTS.md § Bead-loop guardrails applies in full: no `mindspec complete`,
no `git push`, no raw `git merge`, no scope creep beyond the files above,
no reimplementing landed helpers, exactly ONE commit whose message ends
with `Deviations: <list or "none">`, and tests must PASS —
`go build ./... && go test ./... -short` green FROM THE BEAD WORKTREE
(`-short` is mandatory — spec settled fact 3). Hermeticity: fixture repo
roots in `t.TempDir()`; no test may depend on this repo's own lifecycle
state (the live-probe verification is the ORCHESTRATOR's step, not a
test). Absolute paths everywhere; scratch ONLY under
/private/tmp/claude-503/-Users-Max-replit-mindspec/da570e49-a6d2-4faa-a36d-9cb5b5386c7a/scratchpad/.
Work exclusively inside the bead worktree listed under Identity.

## Expected commit shape
```
impl(mindspec-ta30.3): draft-mode derivation + state show honesty

- resolve.go: on-disk draft probe; discovery errors propagate
- state.go: context-authoritative mode; R1.4 non-zero refusal; one Cache
- ADR-0023 draft-discovery exception amendment
- AC-1 three-leg fixtures (red-on-revert vs the 2026-09-01 captures)

Deviations: <list any prompt items you couldn't address literally, or "none">
```

## Report back with
- Commit SHA on `bead/mindspec-ta30.3`.
- Test counts: passed / failed / skipped for the bead's test scope
  (`internal/resolve`, `internal/phase`, `cmd/mindspec` state tests, plus
  the full `go test ./... -short` result line).
- Flagged deviations from this prompt (one line each + reason) —
  explicitly including any bead-1/2 test that needed modification.
