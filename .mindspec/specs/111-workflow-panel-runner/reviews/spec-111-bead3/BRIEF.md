# spec-111-bead3 — Round 1 Review Panel (8 reviewers, Claude-only)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-111-workflow-panel-runner/.worktrees/worktree-mindspec-9cyu.3`
**Branch**: `bead/mindspec-9cyu.3`
**Commit under review**: `84001704398fadb8d49d9f67069cb6a7edde592c` — `impl(mindspec-9cyu.3): ms-panel-run runner dispatch + ms-panel-tally workflow-result note (skills-path retained)`
**Panel**: 8 slots — R1–R3 Opus, R4–R6 Sonnet, R7 Fable, **R8 Sonnet-sub** (codex reserved for the final review — write reviewer_id "R8 sonnet-sub"). **Pass = ≥7 APPROVE, no REJECT.**

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; pin reads to `84001704`; leave `git status` clean. Scratch under ABSOLUTE /tmp only; remove when done (disk is limited).

## Base note (do NOT be confused)
This bead branch was reset onto spec/111 AFTER merging main (109/110/112), so its ms-panel-run/tally skills are the **post-110/fbel.5 slimmed** structure (step 0 = `mindspec panel create`, no decision-matrix table, no hand-typed panel.json) — the base this bead's plan assumes. The `git diff` for THIS bead is only the 5-file `84001704` commit; review that, not the merge.

## What the work does (bead 9cyu.3 — spec 111 R6/R7, the LAST bead)
Adds runner selection to the panel skills without deleting the skills-path mechanics or any judgment section (diff is **+70/−0, pure additions**):
1. `ms-panel-run/SKILL.md` (both mirrors): a **Runner dispatch** section near the top that reads `runner:` via `mindspec config show` and branches — `claude-code-workflow` (compose lenses → invoke `/ms-panel` once with `{slug, spec, target, bead_id?, round, lenses[], mix}`), `claude-code-skills` (the DEFAULT, manual path unchanged), `external` (out-of-scope stub, ADR-0040 degraded mode). Each of the mechanized launch sections gets a one-line "claude-code-skills path only — superseded, not deleted" callout.
2. `ms-panel-tally/SKILL.md` (both mirrors): a **Workflow-path note** — on the workflow path the verdict table + decision arrive pre-rendered in the workflow result; the skill's job narrows to Step 2 consolidation + merge terminal. Judgment sections unchanged.
3. Doc-sync: `runbook.md` maintenance-notes entry.

## What to verify
1. **Runner dispatch correctness**: the three branches are present, correctly labelled workflow-path vs skills-path, `claude-code-skills` is the DEFAULT, and a host lacking workflow capability degrades to skills-path. The workflow-path invokes `/ms-panel` ONCE with the resolved args — cross-check that arg tuple against the shipped `plugins/mindspec/workflows/ms-panel.js` args contract (`{slug, spec, target, bead_id?, round, lenses[], mix, claude_sub_on_quota?}`). Any mismatch (wrong arg name, missing arg, wrong workflow name) is a finding.
2. **Nothing load-bearing deleted (the fbel.5-class risk)**: the diff is +70/−0, but confirm the skills-path mechanics (Step 0 registration, Launch the panel steps, Codex failure detection, Working directory matters, Slot lens defaults) and ALL judgment sections (ms-panel-tally: Consolidate, Artifact gates incl. the Allow-branch artifact-gate screen fbel.5 restored, After a halt, Escape hatch, Abandon) are RETAINED intact — the superseded callouts must NOT remove or weaken them (they only gate them behind the workflow branch). Verify the Allow-branch artifact-gate screen survives (a workflow-path panel still needs the artifact-gate HARD-block discipline).
3. **The workflow/skills-path consistency**: on the workflow path, does the skill correctly hand off (compose lenses → `/ms-panel`)? Does the tally workflow-path note correctly describe consuming the pre-rendered result WITHOUT skipping the artifact-gate screen or consolidation judgment?
4. **Mirror byte-identity**: `diff -q` both pairs → identical.
5. **The `/ms-panel` handoff grep**: use a word-boundary grep so `/ms-panel-tally`/`/ms-panel-run` substrings don't false-match the `/ms-panel` workflow reference.
6. **Step-0 labelling judgment call** (implementer flagged): the implementer added the "superseded" callout to the 4 plan-named sections but NOT to "Step 0 — register the panel" itself (reasoning: Step 0 wasn't in the enumerated list, and the Runner-dispatch section already tells the workflow-path reader not to walk Step 0). Assess whether that's fine or Step 0 should also be labelled.
7. **Doc-sync**: runbook.md entry accurate.

## Verify green
`go build ./...`; `go test -count=1 ./plugins/... ./internal/setup/...` (mirror/embed/refresh + grep-gate tests). The 2 known pre-existing failures (`internal/harness`, `internal/instruct` z4ps) are unrelated.

## Per-slot lens defaults
- **R1 Opus** — author-of-record: diff matches plan Bead 3 (R6/R7) exactly.
- **R2 Opus** — codebase-pin: mirror identity, grep gates, build+tests green.
- **R3 Opus** — completeness/retention: nothing deleted; all judgment sections + skills-path mechanics intact (the fbel.5-class check — esp. the Artifact-gate Allow-screen).
- **R4 Sonnet** — runner-dispatch correctness: the three branches, default, degradation; the `/ms-panel` arg tuple matches the workflow's contract.
- **R5 Sonnet** — workflow/tally consistency: the tally workflow-path note doesn't let the workflow path skip the artifact-gate screen or consolidation; the pre-rendered-result handoff is coherent.
- **R6 Sonnet** — integration/usability: can an operator follow both paths correctly? does the workflow-path handoff to the shipped `/ms-panel` (9cyu.2) actually line up (args, once-invocation, return shape)?
- **R7 Fable** — adversarial: any load-bearing invariant or safety instruction lost/weakened by the gating; any skill-vs-workflow drift (the args tuple, the runner value, the degradation rule); the Step-0-labelling gap.
- **R8 Sonnet-sub** — empirical/lint: run the acceptance greps on both copies; mirror identity; build+tests; confirm the runner-dispatch + retention greps pin what they claim.

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>", R8 = "R8 sonnet-sub"), `verdict`, `confidence` (0–1), `rationale` (≤180 words), `concrete_changes_required` (empty if APPROVE), `findings`.
