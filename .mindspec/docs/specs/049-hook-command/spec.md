---
approved_at: "2026-02-26T20:20:59Z"
approved_by: user
molecule_id: mindspec-mol-8zh03
status: Approved
step_mapping:
    implement: mindspec-mol-41m89
    plan: mindspec-mol-i513e
    plan-approve: mindspec-mol-g41ci
    review: mindspec-mol-6n1ew
    spec: mindspec-mol-kr14h
    spec-approve: mindspec-mol-jnbpu
    spec-lifecycle: mindspec-mol-8zh03
---





# Spec 049-hook-command: Consolidate hooks into mindspec hook CLI command

## Goal

Replace all inline shell scripts in both Claude Code and Copilot hook configurations with a single `mindspec hook <name>` CLI command. This makes hook logic testable in Go, removes the jq dependency, eliminates fragile shell one-liners, and makes settings.json/.github/hooks stable across upgrades (the command string doesn't change when logic evolves). The hook command auto-detects the caller's protocol (Claude Code vs Copilot) and emits the correct response format. As part of this consolidation, add a universal workflow guard that checks every file edit against the current workflow state and emits graduated responses — hard blocks for clear violations, warnings for grey areas.

## Background

MindSpec installs hooks for two agent platforms-

**Claude Code** (`mindspec setup claude`) writes inline shell commands into `.claude/settings.json`. Today there are 6 distinct hook scripts:

1. **SessionStart** — runs `mindspec instruct` for mode guidance
2. **ExitPlanMode gate** — blocks ExitPlanMode when in plan mode (inline shell with jq)
3. **EnterPlanMode context** — injects additionalContext during plan mode (inline shell with jq)
4. **Worktree file guard** — blocks Edit/Write outside active worktree (inline shell with jq)
5. **Worktree bash guard** — blocks Bash commands outside active worktree (inline shell with jq)
6. **Needs-clear bash guard** — blocks `mindspec next` when needs_clear is set (inline shell with jq)

**Copilot** (`mindspec setup copilot`) writes `.github/hooks/mindspec.json` pointing to shell scripts in `.github/hooks/`:

1. **sessionStart** — runs `mindspec instruct`
2. **mindspec-plan-gate.sh** — blocks file-writing tools during plan mode (inline shell with jq)
3. **mindspec-worktree-guard.sh** — blocks file/bash operations outside active worktree (inline shell with jq)

Both platforms suffer the same problems- long shell one-liners that read stdin JSON via `jq`, parse `.mindspec/state.json`, and emit platform-specific deny/context responses. They are hard to test, hard to read, depend on jq being installed, and change whenever logic is refined. Worse, the same guard logic is duplicated across two different shell scripts with different response formats:

- **Claude Code** protocol: stdin has `tool_input.file_path`/`tool_input.command`; block = stderr message + exit 2; warn = `{"additionalContext": "..."}` on stdout + exit 0
- **Copilot** protocol: stdin has `toolName`/`toolArgs.file_path`/`toolArgs.command`; block = `{"permissionDecision":"deny","permissionDecisionReason":"..."}` on stdout + exit 0

Additionally, there is no enforcement that prevents an agent from editing code files outside of implementation mode. This was demonstrated when the `ms-` prefix rename was implemented directly without going through the spec workflow. The fix isn't a simple hard block — there are legitimate reasons to edit files outside the implement phase (debugging, quick config fixes, CI repairs). The right approach is a **state-aware guard** that understands the workflow and responds with graduated enforcement:

- **Hard block** (exit 2): clear violations like code edits during spec/plan mode, or writes outside the active worktree
- **Warning via additionalContext**: edits that are outside the expected workflow but may be legitimate — the LLM receives a clear message that it is in breach and must either stop and go through the lifecycle, or acknowledge exceptional circumstances

## Impacted Domains

- **workflow**: new hook subcommand becomes part of the CLI surface; workflow guard adds state-aware enforcement across all modes
- **setup**: both `setup claude` and `setup copilot` write simplified hook entries that call `mindspec hook` instead of inline shell scripts

## ADR Touchpoints

- [ADR-0005](../../adr/ADR-0005.md): CLI-first design — hook logic belongs in Go, not shell scripts

## Requirements

1. Add `mindspec hook <name>` subcommand that reads hook context from stdin, auto-detects the caller protocol, and emits the correct response format
2. **Protocol auto-detection**: the command inspects the stdin JSON to determine whether it came from Claude Code or Copilot:
   - If the JSON contains `tool_input` → Claude Code protocol (block = stderr + exit 2; warn = `{"additionalContext": "..."}` on stdout)
   - If the JSON contains `toolName` or `toolArgs` → Copilot protocol (block = `{"permissionDecision":"deny","permissionDecisionReason":"..."}` on stdout; warn = same format with a non-deny decision or additionalContext — Copilot doesn't have a native warn mechanism, so warnings are emitted as `permissionDecision: "allow"` with a `permissionDecisionReason` containing the warning text)
   - A `--format` flag (`claude` | `copilot`) can override auto-detection for testing or edge cases
3. Support the following hook names, each replacing the corresponding inline shell scripts on both platforms-
   - `plan-gate-exit` — replaces ExitPlanMode inline script (Claude Code only, no Copilot equivalent)
   - `plan-gate-enter` — replaces EnterPlanMode inline script (Claude Code only, no Copilot equivalent)
   - `worktree-file` — replaces worktreeFileGuardScript (Claude) and worktree file guard in mindspec-worktree-guard.sh (Copilot)
   - `worktree-bash` — replaces worktreeBashGuardScript (Claude) and worktree bash guard in mindspec-worktree-guard.sh (Copilot)
   - `needs-clear` — replaces needsClearBashGuardScript (Claude; Copilot can use same hook)
   - `workflow-guard` — new: universal state-aware guard for Edit/Write/Bash (see Req 6)
4. The hook command reads tool input from stdin and normalizes field access internally — `tool_input.file_path` (Claude) and `toolArgs.file_path` / `toolArgs.path` (Copilot) map to the same internal value. State is read from `.mindspec/state.json` and `.mindspec/config.yaml`.
5. Update `setup claude` to emit `mindspec hook <name>` as the command string instead of inline shell. Update `setup copilot` to replace `.github/hooks/mindspec-plan-gate.sh` and `.github/hooks/mindspec-worktree-guard.sh` with calls to `mindspec hook` in the hooks JSON config.
6. The `workflow-guard` hook is a single guard registered on Edit, Write, and Bash matchers. It checks the current mode and the target file/command, then responds with graduated enforcement:

   | Mode | Target is code file | Target is doc/spec file | Response |
   |:-----|:-------------------|:----------------------|:---------|
   | `idle` | any edit | any edit | **warn** — "You are editing files with no active spec. You must stop and go through the spec lifecycle (`/ms-spec-create` or `/ms-explore`). If these are exceptional circumstances (debugging a CI failure, fixing a broken build, correcting a typo in config, or other urgent operational fix), you may proceed but must note the reason." |
   | `explore` | any edit | any edit | **warn** — same message adapted for explore (exploration is for evaluation, not implementation) |
   | `spec` | code edit | doc edit ok | **block** code edits; allow doc/spec edits silently |
   | `plan` | code edit | doc edit ok | **block** code edits; allow doc/plan edits silently |
   | `implement` | within scope | any | **pass** silently |
   | `implement` | outside worktree | any | **block** (existing worktree guard behavior) |
   | `review` | any edit | any edit | **warn** — "Review mode: implementation is complete. Edits should only address review feedback." |

   Warnings are emitted as `additionalContext` JSON (exit 0), not hard blocks. This lets the LLM see the warning and make a judgment call. Hard blocks use stderr + exit 2.

7. SessionStart hooks remain as-is on both platforms (already call `mindspec instruct`, no inline logic)
8. `mindspec hook --list` prints available hook names (for discoverability)

## Scope

### In Scope
- `cmd/mindspec/hook.go` — new cobra subcommand
- `internal/hook/` — new package with hook implementations, protocol detection, and the workflow guard decision table
- `internal/setup/claude.go` — update `wantedHooks()` and remove shell script functions
- `internal/setup/claude_test.go` — update tests
- `internal/setup/copilot.go` — update `copilotHooksConfig()` and `copilotHookScripts()` to use `mindspec hook` commands, remove inline shell scripts
- `internal/setup/copilot_test.go` — update tests

### Out of Scope
- Changing the hook protocols themselves (stdin JSON format, response schemas) — those are defined by Claude Code and Copilot respectively
- Changing the SessionStart hooks (already clean on both platforms)

## Non-Goals

- Making the warning text configurable per-project — the messages are baked into the Go implementation
- Adding per-file or per-directory allow/deny lists — the guard uses mode + file classification (code vs doc)
- Removing jq as a system dependency entirely (other tools may use it)

## Acceptance Criteria

**Hook logic:**
- [ ] `mindspec hook plan-gate-exit` blocks ExitPlanMode when state is `plan`, passes otherwise
- [ ] `mindspec hook plan-gate-enter` emits additionalContext when state is `plan`, silent otherwise
- [ ] `mindspec hook worktree-file` blocks file writes outside active worktree, passes otherwise
- [ ] `mindspec hook worktree-bash` blocks bash commands outside active worktree, passes otherwise
- [ ] `mindspec hook needs-clear` blocks `mindspec next` when `needs_clear` is set, passes otherwise
- [ ] `mindspec hook workflow-guard` emits warning when mode is `idle` and a file edit is attempted
- [ ] `mindspec hook workflow-guard` hard-blocks code edits during `spec` and `plan` modes
- [ ] `mindspec hook workflow-guard` passes silently for doc edits during `spec` and `plan` modes
- [ ] `mindspec hook workflow-guard` passes silently during `implement` mode for in-scope edits
- [ ] `mindspec hook workflow-guard` emits warning during `review` mode
- [ ] Warning messages clearly state the agent is in breach of the workflow, list the expected action (`/ms-spec-create` or `/ms-explore`), and enumerate legitimate exceptions (CI fix, broken build, config typo, urgent operational fix)
- [ ] `mindspec hook --list` prints all available hook names

**Protocol support:**
- [ ] Auto-detects Claude Code protocol (stdin contains `tool_input`) and emits stderr + exit 2 for blocks, `{"additionalContext": "..."}` for warnings
- [ ] Auto-detects Copilot protocol (stdin contains `toolName`/`toolArgs`) and emits `{"permissionDecision":"deny","permissionDecisionReason":"..."}` for blocks, `{"permissionDecision":"allow","permissionDecisionReason":"..."}` for warnings
- [ ] `--format claude` and `--format copilot` flags override auto-detection
- [ ] Normalizes field access: `tool_input.file_path` (Claude) and `toolArgs.file_path` / `toolArgs.path` (Copilot) resolve to the same internal value

**Setup integration:**
- [ ] `mindspec setup claude` generates settings.json with `mindspec hook <name>` commands (no inline shell for hooks 2-6)
- [ ] `mindspec setup copilot` generates `.github/hooks/mindspec.json` with `mindspec hook <name>` commands, removes standalone `.sh` hook scripts
- [ ] All existing hook behavior is preserved on both platforms (same deny semantics, same pass-through behavior)
- [ ] `make test` passes with no regressions
- [ ] No jq dependency in any generated hook command

## Validation Proofs

- `echo '{}' | mindspec hook plan-gate-exit`: exits 0 (no state file, safe default)
- `echo '{"tool_input":{"file_path":"internal/foo.go"}}' | mindspec hook workflow-guard`: emits `{"additionalContext":"..."}` warning when mode is idle (Claude format auto-detected)
- `echo '{"toolName":"edit","toolArgs":{"file_path":"internal/foo.go"}}' | mindspec hook workflow-guard`: emits `{"permissionDecision":"allow","permissionDecisionReason":"..."}` warning when mode is idle (Copilot format auto-detected)
- `echo '{"tool_input":{"file_path":".mindspec/docs/specs/049/spec.md"}}' | mindspec hook workflow-guard`: exits 0 silently when mode is spec (doc edit allowed)
- `echo '{"tool_input":{"file_path":"internal/foo.go"}}' | mindspec hook workflow-guard`: exits 2 (hard block) when mode is spec (code edit blocked)
- `echo '{"tool_input":{"file_path":"internal/foo.go"}}' | mindspec hook --format copilot workflow-guard`: emits Copilot deny JSON regardless of stdin shape
- `mindspec hook --list`: prints hook names, one per line
- `mindspec setup claude --check`: reports no staleness after upgrade
- `mindspec setup copilot --check`: reports no staleness after upgrade
- `make test`: all tests pass

## Open Questions

None — all resolved.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-26
- **Notes**: Approved via mindspec approve spec