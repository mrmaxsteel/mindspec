---
adr_citations:
    - id: ADR-0005
      sections:
        - State Tracking
    - id: ADR-0015
      sections:
        - Molecule-Derived State
approved_at: "2026-02-26T20:24:52Z"
approved_by: user
bead_ids:
    - mindspec-mol-41m89.1
    - mindspec-mol-41m89.2
    - mindspec-mol-41m89.3
    - mindspec-mol-41m89.4
last_updated: "2026-02-26"
spec_id: 049-hook-command
status: Approved
version: 1
---

# Plan: 049-hook-command — Consolidate hooks into mindspec hook CLI command

## ADR Fitness

- **ADR-0005 (State Tracking)**: Superseded by ADR-0015. The hook command reads `state.json` for the convenience cursor (mode, activeWorktree, needs_clear), which is consistent with ADR-0015's demotion of state.json to a non-canonical hint. The hooks don't derive mode authoritatively — they read the cursor for fast gating decisions. This is appropriate: hooks run on every tool call and must be fast; querying molecules via `bd mol show` on every keystroke would be too slow. The cursor is sufficient for guard logic because the worst case of a stale cursor is a false pass (agent gets warned by `mindspec instruct` at session start anyway).
- **ADR-0015 (Molecule-Derived State)**: No divergence. The hook command is a consumer of state, not a writer. It reads whatever mode is in state.json and acts on it. This aligns with ADR-0015's intent that state.json is a fast hint for UX purposes.

No ADR divergence detected. No new ADRs needed.

## Testing Strategy

- **Unit tests** for each hook function in `internal/hook/` — test the decision logic in isolation by constructing `State` + stdin JSON, asserting exit code and output
- **Unit tests** for protocol detection and response formatting — verify Claude vs Copilot output for the same logical decision
- **Integration tests** in `internal/setup/` — verify `setup claude` and `setup copilot` emit `mindspec hook <name>` commands and that the generated config is valid
- **Shared test helpers**: a `hookTest` helper that sets up state.json, pipes stdin JSON, and captures stdout/stderr/exit code

## Bead 1: Core hook infrastructure and protocol layer

**Steps**
1. Create `internal/hook/hook.go` with the core types: `Result` (pass/block/warn + message), `Input` (normalized tool input from stdin), and `Protocol` (claude/copilot) enum
2. Implement `ParseInput(r io.Reader) (*Input, Protocol, error)` — reads stdin JSON, auto-detects protocol by checking for `tool_input` vs `toolName`/`toolArgs` keys, normalizes field access (`FilePath`, `Command` fields)
3. Implement `FormatResult(r Result, p Protocol) (stdout string, stderr string, exitCode int)` — formats the result per protocol:
   - Claude block: stderr message, exit 2
   - Claude warn: `{"additionalContext":"..."}` on stdout, exit 0
   - Claude pass: exit 0
   - Copilot block: `{"permissionDecision":"deny","permissionDecisionReason":"..."}` on stdout, exit 0
   - Copilot warn: `{"permissionDecision":"allow","permissionDecisionReason":"..."}` on stdout, exit 0
   - Copilot pass: exit 0
4. Create `cmd/mindspec/hook.go` — cobra subcommand `mindspec hook <name>` with `--format` flag and `--list` flag
5. Wire `--list` to print all registered hook names
6. Write unit tests for `ParseInput` (both protocols, edge cases like empty stdin) and `FormatResult` (all result×protocol combinations)

**Verification**
- [ ] `go test ./internal/hook/...` passes
- [ ] `go build ./cmd/mindspec/...` compiles
- [ ] `mindspec hook --list` prints hook names

**Depends on**
None

## Bead 2: Migrate existing hook logic to Go

**Steps**
1. Implement `PlanGateExit(input *Input, st *state.State) Result` — block if mode is plan, pass otherwise
2. Implement `PlanGateEnter(input *Input, st *state.State) Result` — warn with additionalContext if mode is plan, pass otherwise
3. Implement `WorktreeFile(input *Input, st *state.State, configPath string) Result` — block if activeWorktree is set, file path is outside worktree and main repo, and enforcement is not disabled. Pass otherwise.
4. Implement `WorktreeBash(input *Input, st *state.State, configPath string) Result` — block if activeWorktree is set, CWD is outside worktree, and command is not in the allowlist (cd, mindspec, bd, make, git, go). Handle env var prefix stripping.
5. Implement `NeedsClear(input *Input, st *state.State) Result` — block if needs_clear is true and command matches `mindspec next` (without `--force`). Pass otherwise.
6. Register all five hooks in the cobra command dispatch
7. Write unit tests for each hook function covering: mode match, mode mismatch, missing state file (graceful pass), enforcement disabled

**Verification**
- [ ] `go test ./internal/hook/...` passes with tests for all 5 migrated hooks
- [ ] Each hook produces identical behavior to the shell script it replaces (same messages, same block/pass decisions)
- [ ] `echo '{}' | mindspec hook plan-gate-exit` exits 0
- [ ] `echo '{"tool_input":{"file_path":"/outside"}}' | mindspec hook worktree-file` exits 2 when worktree is active

**Depends on**
Bead 1

## Bead 3: Workflow guard implementation

**Steps**
1. Implement `classifyFile(path string) FileKind` — returns `code`, `doc`, `config`, or `unknown` based on path patterns:
   - `code`: `cmd/`, `internal/`, `*.go`, `Makefile`, `go.mod`, `go.sum`, etc.
   - `doc`: `.mindspec/docs/`, `*.md`, `GLOSSARY.md`, `AGENTS.md`, `CLAUDE.md`, `docs/`
   - `config`: `.mindspec/config.yaml`, `.claude/`, `.github/`
   - `unknown`: everything else (treated as code for safety)
2. Implement `WorkflowGuard(input *Input, st *state.State, cwd string) Result` with the decision table:
   - `idle`: warn for any edit — message includes: "You are editing files with no active spec. You must stop and go through the spec lifecycle (/ms-spec-create or /ms-explore). If these are exceptional circumstances (debugging a CI failure, fixing a broken build, correcting a typo in config, or other urgent operational fix), you may proceed but must note the reason."
   - `explore`: warn for any edit — adapted message about exploration being for evaluation, not implementation
   - `spec`: block code edits, pass doc edits
   - `plan`: block code edits, pass doc edits
   - `implement`: pass if within worktree scope (delegate to worktree guard logic), block otherwise
   - `review`: warn for any edit — "Review mode: implementation is complete. Edits should only address review feedback."
   - No state file: pass silently (graceful degradation)
3. Handle Bash tool: extract command, classify whether it's a code-modifying command vs read-only/navigation
4. Register `workflow-guard` in the cobra command dispatch
5. Write unit tests covering every cell in the decision table (mode × file kind), plus the bash command classification

**Verification**
- [ ] `go test ./internal/hook/...` passes with workflow-guard tests
- [ ] Warning messages contain the exact escape-hatch text from the spec
- [ ] Code edits during spec/plan mode are hard-blocked
- [ ] Doc edits during spec/plan mode pass silently
- [ ] Idle mode emits warning (not block) for all edits

**Depends on**
Bead 1

## Bead 4: Update setup claude and setup copilot

**Steps**
1. Update `wantedHooks()` in `internal/setup/claude.go` — replace all inline shell commands with `mindspec hook <name>`:
   - ExitPlanMode matcher: `mindspec hook plan-gate-exit`
   - EnterPlanMode matcher: `mindspec hook plan-gate-enter`
   - Write matcher: `mindspec hook worktree-file` and `mindspec hook workflow-guard`
   - Edit matcher: `mindspec hook worktree-file` and `mindspec hook workflow-guard`
   - Bash matcher: `mindspec hook worktree-bash`, `mindspec hook needs-clear`, and `mindspec hook workflow-guard`
2. Remove `worktreeFileGuardScript()`, `worktreeBashGuardScript()`, and `needsClearBashGuardScript()` functions from claude.go
3. Update `copilotHooksConfig()` in `internal/setup/copilot.go` — replace bash script references with `mindspec hook <name> --format copilot`:
   - preToolUse: `mindspec hook workflow-guard --format copilot`, `mindspec hook worktree-file --format copilot`, `mindspec hook worktree-bash --format copilot`
4. Remove or simplify `copilotHookScripts()` — the standalone `.sh` files are no longer needed (replace with a thin wrapper that calls `mindspec hook` if backward compatibility is desired, or remove entirely)
5. Update `claude_test.go` and `copilot_test.go` — adjust assertions for new command strings, verify no inline shell in generated hooks, verify script files are removed/replaced
6. Run `mindspec setup claude --check` and `mindspec setup copilot --check` to verify no staleness

**Verification**
- [ ] `go test ./internal/setup/...` passes
- [ ] Generated `.claude/settings.json` contains only `mindspec hook <name>` commands (no jq, no inline shell for hooks 2-6)
- [ ] Generated `.github/hooks/mindspec.json` references `mindspec hook <name> --format copilot`
- [ ] `mindspec setup claude --check` reports clean
- [ ] `mindspec setup copilot --check` reports clean
- [ ] `make test` passes with no regressions

**Depends on**
Bead 2, Bead 3

## Provenance

| Spec Acceptance Criterion | Verified By |
|:---|:---|
| plan-gate-exit blocks ExitPlanMode in plan mode | Bead 2 tests |
| plan-gate-enter emits additionalContext in plan mode | Bead 2 tests |
| worktree-file blocks outside worktree | Bead 2 tests |
| worktree-bash blocks outside worktree | Bead 2 tests |
| needs-clear blocks mindspec next | Bead 2 tests |
| workflow-guard warns in idle mode | Bead 3 tests |
| workflow-guard blocks code edits in spec/plan | Bead 3 tests |
| workflow-guard passes doc edits in spec/plan | Bead 3 tests |
| workflow-guard passes in implement mode | Bead 3 tests |
| workflow-guard warns in review mode | Bead 3 tests |
| Warning messages list exceptions | Bead 3 tests |
| --list prints hook names | Bead 1 tests |
| Auto-detects Claude protocol | Bead 1 tests |
| Auto-detects Copilot protocol | Bead 1 tests |
| --format flag overrides detection | Bead 1 tests |
| Field normalization | Bead 1 tests |
| setup claude generates mindspec hook commands | Bead 4 tests |
| setup copilot generates mindspec hook commands | Bead 4 tests |
| Existing behavior preserved | Bead 2 + Bead 4 tests |
| make test passes | Bead 4 final verification |
| No jq dependency | Bead 4 tests (grep for jq in output) |
