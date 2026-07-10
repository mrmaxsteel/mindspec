# spec-113-bead2 — Round 1 Bead Panel (8 reviewers) — R2

**Bead**: `mindspec-r6hk.2` (spec 113, Bead 2 = R2). **Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-113-panel-verb-workflow-followups/.worktrees/worktree-mindspec-r6hk.2`
**Branch**: `bead/mindspec-r6hk.2` @ **52d052d0267cc6c0a48eb6a98be6f37400651a42** — `fix(workflow): SHELL_METACHAR_RE rejects bare $ (R2); mirror byte-identical`
**Panel**: 8 slots — O1–O3 Opus, S1–S3 Sonnet, F1 Fable, **R8 sonnet-sub** (no codex on bead panels this session). **Pass = every finding adjudicated (fixed or evidenced-refuted) — a raised finding is NOT out-voted by the APPROVE count.**

**READ-ONLY RULE (MANDATORY)**: edit nothing but your verdict JSON; pin reads to `52d052d0`; scratch under ABSOLUTE /tmp only; leave `git status` clean. Write your verdict ONLY to the exact absolute path at the bottom — do NOT create a `reviews/` dir inside the bead worktree.

## What the bead does
`SHELL_METACHAR_RE` in `plugins/mindspec/workflows/ms-panel.js` matched `$(` but NOT a bare `$`, so `$HOME`/`${x}`/`$x` variable-expansion survived `validateShellSafe` on slug/spec/target/bead_id (the workflow assembles a shell-executed command string via `buildCommand`). The fix folds a bare `$` into the character class: `const SHELL_METACHAR_RE = /[\x60;|&\n$]/;` (line 141). Monotone tightening — `$(` still matches (contains `$`); the prior classes (`` ` ``=\x60, `;`, `|`, `&`, `\n`) are unchanged.

## Files in scope (final state at 52d052d0)
- `plugins/mindspec/workflows/ms-panel.js` (the regex)
- `.claude/workflows/ms-panel.js` (byte-identical mirror)
- `plugins/mindspec/workflow_test.go` (new pin test `TestMsPanelWorkflow_ShellMetacharRejectsBareDollar`)

## What to verify (each concern → a disposition)
1. **Regex correctness (O2/F1)** — `/[\x60;|&\n$]/` rejects a bare `$` (so `$HOME`, `${x}`, `$x` are rejected by `validateShellSafe`). MONOTONE: everything the old regex `/[\x60;|&\n]|\$\(/` rejected is still rejected (the char class keeps `` ` `` `;` `|` `&` `\n`; `$(` still matches via the bare `$`). NO regression. Also confirm the char class is well-formed (adding `$` inside `[...]` is a literal `$`, not an anchor — correct).
2. **No over-rejection / false positives (O2/R8)** — no legitimate slug, spec ID, target ref, or bead ID in this repo's conventions contains `$` (e.g. `p113`, `113-panel-verb-workflow-followups`, `bead/mindspec-x.1`, `spec/113-...`). Confirm the tightening doesn't reject any real input the workflow must accept.
3. **Mirror byte-identical (O3/S1/R8)** — `cmp plugins/mindspec/workflows/ms-panel.js .claude/workflows/ms-panel.js` exits 0. If there's a Go embed of ms-panel.js, it still matches (that's what `TestWorkflowFiles_EmbedsMsPanel` checks).
4. **Floor tests unmodified + green (O3/S1)** — `TestWorkflowFiles_EmbedsMsPanel` and `TestMsPanelWorkflow_AllowedCLIExactSet` (spec 111's exact-set/chokepoint pins) unchanged and green. `go test ./plugins/mindspec`.
5. **New test genuinely pins (S2/F1)** — `TestMsPanelWorkflow_ShellMetacharRejectsBareDollar` asserts the exact new declaration appears in the embedded `WorkflowFiles()["ms-panel.js"]`. Is it a real pin (would it red if the regex reverted)? Note Go can't execute the JS regex, so behavioral verification is the node matrix (AC2) run separately; the Go test is a string-pin.
6. **Behavioral (R8 empirical)** — run the node metachar matrix: extract `SHELL_METACHAR_RE`, assert it MATCHES (rejects) `$HOME`, `${x}`, `$x`, `` a`b ``, `a;b`, `a|b`, `a&b`, `a\nb`, `$(x)` and does NOT match legit `p113` / `113-panel-verb-workflow-followups` / `bead/mindspec-x.1`. Confirm `go test ./plugins/mindspec` + `go build ./...` green.
7. **Scope fence (S3)** — exactly the 3 files, one commit, nothing out of scope; git status clean.
8. **Adversarial (F1)** — mutation-probe in /tmp: revert the `$` from the char class → does `TestMsPanelWorkflow_ShellMetacharRejectsBareDollar` (or the node matrix) red? If the test stays green with the `$` removed, the pin is hollow = finding. Also: is there ANY way a `$`-bearing value still reaches `buildCommand`'s shell string (does every user value actually route through `validateShellSafe`/the regex)? Any bypass = finding.

## Per-slot lens defaults
- **O1 Opus** — author-of-record (diff ↔ plan Bead 2). **O2 Opus** — regex correctness + monotonicity + no-false-positive. **O3 Opus** — mirror byte-identity + embed + floor tests unmodified.
- **S1 Sonnet** — codebase-pin (symbols/tests exist + green). **S2 Sonnet** — new pin test really pins. **S3 Sonnet** — scope fence.
- **F1 Fable** — adversarial (mutation-probe the pin; hunt a `$` bypass to buildCommand). **R8 sonnet-sub** — empirical (node matrix + cmp + go test; try to slip a `$` through).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to the EXACT absolute path `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id`, `verdict`, `confidence` (0–1), `rationale` (≤180 words), `concrete_changes_required` (empty if APPROVE), `findings`.
