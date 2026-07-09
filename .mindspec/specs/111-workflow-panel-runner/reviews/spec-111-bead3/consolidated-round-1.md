# spec-111-bead3 — consolidated round-1 changes

Tally: 5 APPROVE (R1 0.95, R2 0.95, R3 0.9, R5 0.93, R8 0.93) / 3 REQUEST_CHANGES (R4 sonnet 0.78, R6 sonnet 0.78, R7 fable 0.9) / 0 REJECT. Threshold 7/8 not met → fix round. Runner-dispatch branches/labels/default, mirror identity, +70/−0 additive (all judgment + skills-path sections retained — R3/R8), and the runner enum/config-surface (`mindspec config show` renders `runner:`) all verified. Two convergent findings.

## Fix 1 — `claude_sub_on_quota` omitted from the workflow-path arg tuple (R4 + R6 + R7 + R8-noted; load-bearing, SPEC-MANDATED)
The Runner-dispatch `claude-code-workflow` bullet invokes `/ms-panel` with `{slug, spec, target, bead_id?, round, lenses[], mix}` — it omits `claude_sub_on_quota`. Ground truth: `ms-panel.js` CANNOT read config (reading config is intentionally outside ALLOWED_CLI) and fail-closes an absent flag to `false` (`args.claude_sub_on_quota === true`); the config default is `TRUE` (internal/config). Net effect: every skill-composed workflow invocation silently DISABLES quota-wall substitution — a walled codex slot goes MISSING and the gate Blocks a panel the config said should substitute. Spec R4 makes substitution config-driven ("falsified if it ignores the 109 `claude_sub_on_quota` flag") and spec.md:40 explicitly assigns the substitution-flag config-read to the runner-dispatch (via `mindspec config show`, which renders it). This bead's OWN doc-sync (interfaces.md:236-238) also assigns the `claude_sub_on_quota` resolution to ms-panel-run. Root cause: spec/plan R6's illustrative tuple predates the Bead-2 contract and was copied literally — but this bead owns the seam.
- **Fix**: add `claude_sub_on_quota` to the Runner-dispatch `claude-code-workflow` arg tuple in `ms-panel-run/SKILL.md` (both mirrors), with a one-clause instruction to resolve it from config `panel.substitution.claude_sub_on_quota` (mirroring how `mix` is resolved from `panel:`). Update the tuple mention in `runbook.md` too.

## Fix 2 — tally Workflow-path note obscures the artifact-gate Allow-screen (R7; moderate)
The `ms-panel-tally/SKILL.md` Workflow-path note says the workflow path "narrows [the skill's] job to Step 2 consolidation and the merge terminal" — omitting the Step-1 **artifact-gate Allow-screen** (the fbel.5-restored HARD-block: a missing measurement-artifact CCR HARD-blocks even on an Allow, regardless of vote count). The screen text survives verbatim and the note says Artifact gates are unchanged on both paths (R5 confirmed it still applies), so it is obscured-not-lost — but the narrowed-job sentence invites a "consume-result-then-merge" reading that skips the screen.
- **Fix**: add one clause to the narrowed-job sentence so it names the artifact-gate Allow-screen as still applying on the workflow path (the workflow renders the mechanical tally; the human artifact-gate + consolidation judgment is NOT mechanized). Keep consistent with spec R7's phrasing.

## Non-blocking (confirmed fine, no change)
- Step-0 double-registration is explicitly prevented ("do not separately walk Step 0 through the Anti-patterns section below") — the implementer's labelling judgment call is fine (R6/R7/R8).
- `sha` omitted from the tuple — harmless (advisory-only, self-resolved by `panel create`).

## Constraints for the fix author
- ONE commit on `bead/mindspec-9cyu.3`: `fix(111): pass claude_sub_on_quota on the workflow path + name the artifact-gate screen in the tally workflow note [mindspec-9cyu.3]`.
- Edit `ms-panel-run/SKILL.md` + `ms-panel-tally/SKILL.md` (BOTH plugins + .claude mirrors, identically) + `runbook.md`. Keep `diff -q` mirror-clean; retain ALL judgment sections; stay additive.
- `go build ./...`; `go test -count=1 ./plugins/... ./internal/setup/...` green; the Bead-3 acceptance greps still pass.
- Scratch under ABSOLUTE /tmp only. No push/bd/lifecycle.
