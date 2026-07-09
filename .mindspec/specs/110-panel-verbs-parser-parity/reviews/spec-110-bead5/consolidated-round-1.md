# spec-110-bead5 — consolidated round-1 changes

Tally: 6 APPROVE (R1 0.95, R2 0.97, R3 0.87, R4 0.95, R5 0.95, R8 codex 0.92) / 2 REQUEST_CHANGES (R6 sonnet 0.72, R7 fable 0.78) / 0 REJECT. Threshold 7/8 not met → fix round. Mirror byte-identity, grep gates, all judgment sections preserved, single-verdict-instruction + hard_block invariants, verb/stub/exit-code accuracy — all verified clean. The two RCs name real gaps. Both mirrors (`plugins/mindspec/skills/**` AND `.claude/skills/**`) MUST be edited identically.

## Fix 1 — R7 (SAFETY, load-bearing): the artifact-gate trigger lost on the Allow branch
The OLD ms-panel-tally decision matrix row 1 had TWO disjuncts firing before the pass row: (a) `hard_block: true`, OR (b) *a `concrete_changes_required` item names a missing measurement artifact / drift report / cost projection / regression baseline*. `mindspec panel tally` mechanizes ONLY (a) (gate.go cases 9–10). The thinned skill's new step 1 hands off to the merge terminal on Allow **unconditionally**, so a 5/6-Allow whose one RC names a missing `cost_projection.json` (reviewer didn't set the flag — the exact lola-f4a8 $417 incident the SURVIVING § Artifact gates section memorializes) now goes straight to merge. The § Artifact gates section survives but its procedural TRIGGER was deleted with the matrix — an orphaned policy.
- **Fix**: on step 1's Allow / merge-handoff branch, add a clause: before handing off to the merge terminal on an Allow, screen the tally's aggregated `concrete_changes_required` (printed even on Allow) against § Artifact gates — a CCR item naming a missing measurement artifact / cost projection / drift report / regression baseline **HARD-blocks regardless of vote count and regardless of whether any reviewer set `hard_block`**. This restores disjunct (b) as an authority-side screen the binary does not mechanize.

## Fix 2 — R6 (anti-drift): use `panel create`'s reported `panel directory:` line
fbel.4 added a `panel directory: <dir>` output to `panel create` specifically so consumers capture it instead of re-deriving layout. But ms-panel-run's downstream steps still re-derive the path from the pre-existing `<spec-dir>/reviews/<panel-slug>/` prose convention — which only documents `panelDirFor`'s flat branch, never the legacy non-flat fallback (`<root>/review/<slug>`). Duplicating path logic the CLI resolves+prints is exactly the drift this bead closes elsewhere.
- **Fix**: in ms-panel-run § Step 0, after the `mindspec panel create ...` invocation, instruct the operator to capture the command's reported `panel directory: <dir>` line and use that path directly for the BRIEF stub, codex prompt files, and verdict-file existence checks — rather than re-deriving from the convention note. Optionally note in the convention block that `panelDirFor` also has a non-flat legacy fallback (`<root>/review/<slug>`).

## Minor nits (fold into the same commit)
- **R6**: `runbook.md`'s new Maintenance Notes entry names only 'Slot lens defaults' + 'Anti-patterns' as ms-panel-run's surviving judgment sections — enumerate all five (Launch the panel, Codex failure detection, Working directory matters, Slot lens defaults, Anti-patterns).
- **R7**: the stale `expected-reviewers` bullet in ms-panel-run § Inputs contradicts config-stamping (`panel create` stamps `expected_reviewers` from config now) — correct or remove it.
- **R7** (judgment, do if the surviving anti-patterns reference them): per-slot `confidence` is no longer surfaced anywhere though a kept anti-pattern ("Don't ignore confidence") requires it — restore a note that `panel tally` output / the operator should weigh confidence; the dropped "flag user if ≤2 APPROVE" heuristic is fail-safe (still Blocks on threshold) — restore only if cheap.

## Constraints for the fix author
- ONE commit on `bead/mindspec-fbel.5`: `docs(110): restore artifact-gate Allow-screen + point skills at panel create's reported dir [mindspec-fbel.5]`.
- BOTH mirror copies edited IDENTICALLY (keep `diff -q` clean); doc-sync (runbook.md) if the section list changes. Only markdown files — no Go.
- All grep gates + mirror/embed tests stay green (`go test ./internal/setup/... ./plugins/...`; `go build ./...`).
- Scratch under ABSOLUTE /tmp only. No push/bd/lifecycle.
