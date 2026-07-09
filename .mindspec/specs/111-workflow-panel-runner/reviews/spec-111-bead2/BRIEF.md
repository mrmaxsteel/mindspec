# spec-111-bead2 — Round 1 Review Panel (8 reviewers, Claude-only)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-111-workflow-panel-runner/.worktrees/worktree-mindspec-9cyu.2`
**Branch**: `bead/mindspec-9cyu.2`
**Commit under review**: `1ad0d796280b81a1c55e218cb981cca6b8e1f00b` — `feat(111): /ms-panel workflow adapter — ALLOWED_CLI chokepoint, embed + Claude install, structural tests [mindspec-9cyu.2]`
**Panel**: 8 slots — R1–R3 Opus, R4–R6 Sonnet, R7 Fable, **R8 Sonnet-sub** (codex reserved for the final review; the empirical/injection slot runs as a Sonnet substitute — write reviewer_id "R8 sonnet-sub"). **Pass = ≥7 APPROVE, no REJECT.**

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; pin reads to `1ad0d796`; leave `git status` clean. Any scratch under ABSOLUTE /tmp only (never a relative `.mindspec/` write); **remove scratch when done (disk is limited)**.

## What the work does (bead 9cyu.2 — spec 111 R1/R2/R3/R3b/R4/R5/R8)
The `/ms-panel` **workflow adapter** — a Claude Code dynamic JS workflow (`ms-panel.js`) that is the runner behind spec 110's `mindspec panel create|verify|tally` verbs + verdict schema and spec 109's `panel:` config. The script coordinates `agent()` steps and does NO shell/file I/O itself (every CLI touch + file write is an agent step). Delivered: the workflow (both byte-identical copies), its embed + Claude-target install, and structural/exact-set tests.

7 files (+1487): `plugins/mindspec/workflows/ms-panel.js` (526) + its byte-identical mirror `.claude/workflows/ms-panel.js` (526), `plugins/mindspec/embed.go` (WorkflowFiles), `plugins/mindspec/workflow_test.go` (the tests), `internal/setup/claude.go` (installWorkflows), `internal/setup/claude_test.go` (created-count), `.mindspec/domains/workflow/interfaces.md` (doc-sync).

## CRITICAL branch-base context (do NOT flag as defects)
spec/111 branched BEFORE specs 110 + 112 merged to main, so this branch has NO `mindspec panel` verbs (110) and NO per-gate schema (112):
- The `ms-panel.js` script INVOKES `mindspec panel create/verify/tally` as runtime CLI strings — correct; they're runtime invocations, not compile-time Go deps.
- The automated Go tests (`TestWorkflowFiles_EmbedsMsPanel`, `TestMsPanelWorkflow_AllowedCLIExactSet`) are STATIC ANALYSIS of the `.js` script + embed — they do NOT run the verbs, so they pass on this base.
- The plan's **Manual e2e** (codex PATH-shim running real `mindspec panel create`) CANNOT run on this base (no verbs) — it requires the 110-merged binary. Do NOT treat its non-runnability here as a defect.
- The `internal/setup/claude_test.go` created-count was set relative to THIS branch's base (+1 for the workflow file), not the plan's literal "14→15" (which was written against a different base). Judge it as base-count+1, not the literal number.
- The spec→main integration (workflow-doc union with 110/112) happens at 111's impl-approve.

## The load-bearing invariants (verify HARDEST — R7 Fable + R8 sonnet-sub + R3 Opus)
1. **`ALLOWED_CLI` static literal array** at the top of `ms-panel.js` — the exact, exhaustive 4-command set (`mindspec panel create`, `codex exec --sandbox read-only --skip-git-repo-check`, `mindspec panel verify`, `mindspec panel tally`), NO template interpolation/concatenation, so it's machine-parseable and can't smuggle a 5th command. `const [CMD_PANEL_CREATE, CMD_CODEX_EXEC, CMD_PANEL_VERIFY, CMD_PANEL_TALLY] = ALLOWED_CLI;` destructured. **`mindspec complete` appears NOWHERE** (not a command, not a comment — this workflow is an adapter, never a lifecycle mutator).
2. **`buildCommand(verb, ...args)` chokepoint** — the ONE command assembler. Verify the plan's round-3 reconciliation is implemented: fixed flags (`--spec`/`--target`/`--bead`/`--round`; the codex sandbox flags) come from a **fixed per-verb template**, NOT caller args; callers pass ONLY user-derived VALUES; the leading-dash guard applies to those values (rejecting a flag-shaped value like a `--json` slug) while the template's own fixed flags run. Every agent step's command comes from `buildCommand(CMD_*, ...values)` — no retyped literal, no string surgery. Confirm `TestMsPanelWorkflow_AllowedCLIExactSet`'s positive-enumeration + identifier-count pins actually hold **by construction** (no `mindspec`/`codex`-bearing literal outside the array).
3. **Input hardening at workflow entry, before any command/path is built**: slug/spec/bead_id/`mix[].family` validated via 110's clean-single-path-element contract (reject empty/`.`/`..`/`/`/`\`/control bytes); `target` via a branch-name-safe grammar (reject empty, control bytes, leading `-`, git-check-ref-format-disallowed `..`/`~`/`^`/`:`/`?`/`*`/`[`/trailing-`/`/`.lock`/whitespace) AND shell metacharacters (backtick, `$(`, `;`, `|`, `&`, newlines); `round` a positive integer; `target` appended as its own argv-safe token (never concatenated). Any failure aborts before Step 2. Slot ids come from a fixed internal enumeration, never args. **Try to defeat this** — can any unvalidated value reach a built command or a file path?
4. **The anti-laundering ladder (R3b)**: codex slot execs the read-only sandbox, persists unmodified stdout to `.codex.log`, accepts exactly ONE verdict JSON object (zero/multiple/narrative-wrapped → fail-closed to the reserialize-then-MISSING path → incomplete → gate Blocks); substitution (R4) is reserved for a quota wall with NO verdict rendered, keeping the slot id + `claude-sub`. The workflow returns `panel verify` + `panel tally` stdout verbatim; does NO consolidation.

## What to verify (green)
- `go build ./...`; `go test -count=1 ./plugins/... ./internal/setup/...` — ALL green (the 2 new tests + the created-count tests). `diff -q` both ms-panel.js copies → identical. Run the plan's Bead-2 acceptance greps.

## Per-slot lens defaults
- **R1 Opus** — author-of-record: diff matches plan Bead 2 steps 1–7, no more/less.
- **R2 Opus** — codebase-pin: files/tests exist + green; mirror byte-identity; embed (`WorkflowFiles`) + install (`installWorkflows`, Claude-target-only, NOT RunCodex/RunCopilot); created-count consumers updated.
- **R3 Opus** — ALLOWED_CLI/buildCommand chokepoint integrity (invariants 1–2); no `mindspec complete`; scope-fence (only the 7 files).
- **R4 Sonnet** — empirical: build + run the tests; confirm the exact-set + embed + created-count tests pass and pin what they claim.
- **R5 Sonnet** — the input-hardening validators (invariant 3): completeness + correctness of slug/spec/bead_id/target/round/mix validation; argv-safe token handling; type correctness.
- **R6 Sonnet** — integration: does the workflow consume 110's CLI contract correctly (buildCommand → `panel create/verify/tally`; captures the reported panel dir; returns verify+tally stdout verbatim)? R3b ladder + R4 substitution match 110's artifact semantics? Does it match spec 111's plan for what Bead 3 (skills handoff) consumes?
- **R7 Fable** — sharpest adversarial on the INJECTION/chokepoint surface (invariants 1–4): find any path where an unvalidated value reaches a command/path, any way to smuggle a 5th command or bypass buildCommand, any ladder hole (a rendered-but-malformed verdict escaping fail-closed), or drift between the impl and the plan's round-3 buildCommand reconciliation.
- **R8 Sonnet-sub** — empirical injection/security: exercise the validators + ALLOWED_CLI + positive-enumeration test; confirm the chokepoint holds by construction; verify the exact-set test would fail if a 5th command or a stray literal were introduced.

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>", R8 = "R8 sonnet-sub"), `verdict`, `confidence` (0–1), `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
