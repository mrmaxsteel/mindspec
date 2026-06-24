---
adr_citations:
    - id: ADR-0006
      sections:
        - ADR Fitness
    - id: ADR-0019
      sections:
        - ADR Fitness
    - id: ADR-0015
      sections:
        - ADR Fitness
    - id: ADR-0002
      sections:
        - ADR Fitness
approved_at: "2026-02-26T08:49:51Z"
approved_by: user
bead_ids: []
last_updated: "2026-02-26T09:00:00Z"
spec_id: 046-worktree-enforcement
status: Approved
version: 1
---

# Plan: 046-worktree-enforcement — Deterministic Worktree and Branch Enforcement

## Overview

Enforce zero-on-main invariant through three independent, deterministic layers plus a soft redirect. All mindspec-managed changes — spec, plan, and implementation — happen on branches in worktrees. Nothing is committed directly to main.

### Worktree location convention

All mindspec-managed worktrees live under `.worktrees/` at the repo root:

```
my-project/
├── .git/
├── .beads/
├── .mindspec/
├── .worktrees/                      # gitignored — canonical worktree root
│   ├── worktree-spec-046-slug/      # spec/plan worktree
│   └── worktree-bead-xxx/           # impl bead worktree
├── src/
└── .gitignore                       # contains: .worktrees/
```

This is configurable via `worktree_root` in `.mindspec/config.yaml` (default: `.worktrees`). Worktree paths passed to `bd worktree create` are prefixed with this root (e.g., `.worktrees/worktree-spec-046`). Beads handles all git plumbing — mindspec only controls the path prefix.

The implementation has eight beads organized into three phases:

1. **Foundation** (Beads 1-2): Config package, state schema extension, git helpers
2. **Core enforcement** (Beads 3-6): Worktree lifecycle in CLI commands, CWD guards, instruct redirect, agent hooks
3. **Finish** (Beads 7-8): Pre-commit hook, documentation

## ADR Fitness

- **ADR-0006** (Protected main with PR-based merging): Sound. This spec is the enforcement implementation for ADR-0006's branching model. The plan creates `spec/NNN-slug` branches at spec-init, branches impl beads from the spec branch, merges beads back to the spec branch, and creates a final PR/merge from spec→main. No divergence.
- **ADR-0019** (Deterministic worktree enforcement): Sound. This spec implements all three layers defined in ADR-0019: git pre-commit hook (Layer 1), CLI guards (Layer 2), agent PreToolUse hooks (Layer 3), plus the soft instruct redirect. No divergence.
- **ADR-0015** (Molecule-derived state): Sound. Enforcement reads `state.json` as a fast hint for `activeWorktree` — it does not derive mode from the molecule in hot paths (hooks, guards). `mindspec instruct` continues to derive mode from molecules per ADR-0015. No divergence.
- **ADR-0002** (MindSpec/Beads integration): Sound. Enforcement policy is mindspec's. Worktree plumbing (`bd worktree create/info/list/remove`) is beads'. Mindspec calls beads for worktree operations but adds its own guards on top. No divergence.

## Testing Strategy

- **Unit tests**: Each bead includes tests for its package. Config parsing, state read/write with new fields, CWD guard logic, worktree detection, hook generation.
- **Integration**: `make test` passes after each bead. `make build` succeeds.
- **Manual verification**: Full lifecycle test (spec-init → approve spec → plan → approve plan → next → implement → complete → impl-approve) runs entirely in worktrees with zero commits on main.

## Bead 1: Config package and state schema extension

**Provenance**: R6 (Configuration), R7 (State schema extension)

**Steps**
1. Create `internal/config/config.go`:
   - `Config` struct with fields: `ProtectedBranches []string`, `MergeStrategy string`, `WorktreeRoot string`, `Enforcement struct { PreCommitHook, CLIGuards, AgentHooks bool }`
   - `Load(root string) (*Config, error)` — reads `.mindspec/config.yaml`, returns defaults if file missing
   - `DefaultConfig()` — returns `Config{ProtectedBranches: ["main", "master"], MergeStrategy: "auto", WorktreeRoot: ".worktrees", Enforcement: {true, true, true}}`
   - `IsProtectedBranch(branch string) bool` helper
   - `WorktreePath(root, name string) string` — returns `filepath.Join(root, cfg.WorktreeRoot, name)`
2. Create `.mindspec/config.yaml` default template (used by `mindspec init` in future — for now just document the schema)
3. Extend `internal/state/state.go`:
   - Add `ActiveWorktree string` and `SpecBranch string` fields to `State` struct with JSON tags
   - These fields are set by spec-init and next, read by enforcement layers
4. Add unit tests: `internal/config/config_test.go` (load defaults, load from file, protected branch check), `internal/state/state_test.go` (round-trip with new fields)

**Verification**
- [ ] `go test ./internal/config/...` passes
- [ ] `go test ./internal/state/...` passes
- [ ] `make test` passes

**Depends on**
None

## Bead 2: Git helpers for branch and worktree operations

**Provenance**: R1 (Worktree lifecycle management)

**Steps**
1. Create `internal/gitops/gitops.go`:
   - `CurrentBranch() (string, error)` — runs `git rev-parse --abbrev-ref HEAD`
   - `BranchExists(name string) bool` — runs `git rev-parse --verify refs/heads/<name>`
   - `CreateBranch(name, from string) error` — runs `git branch <name> <from>`
   - `MergeBranch(source, target string) error` — runs `git checkout <target> && git merge --no-ff <source> && git checkout -` (in a temp worktree context or from the spec worktree)
   - `MainWorktreePath() (string, error)` — runs `git worktree list --porcelain` and finds the main worktree
   - `IsMainWorktree(path string) (bool, error)` — compares path to main worktree path
   - `CreatePR(branch, base, title, body string) (string, error)` — runs `gh pr create` if gh is available
   - `HasRemote() bool` — checks `git remote`
2. Extend `internal/bead/bdcli.go`:
   - Add `WorktreeCreateFromBase(name, branch, baseBranch string) error` — calls `bd worktree create <name> --branch=<branch> --start-point=<baseBranch>` (or uses git directly if bd doesn't support `--start-point`)
3. Add unit tests: `internal/gitops/gitops_test.go` with testable function vars

**Verification**
- [ ] `go test ./internal/gitops/...` passes
- [ ] `make test` passes

**Depends on**
Bead 1

## Bead 3: Worktree lifecycle in spec-init and next

**Provenance**: R1.1-R1.3 (spec-init creates worktree, next branches from spec branch)

**Steps**
1. Extend `internal/specinit/specinit.go` `Run()`:
   - After creating spec dir and pouring molecule, create `spec/<specID>` branch from current HEAD (main)
   - Ensure `.worktrees/` directory exists and is in `.gitignore`
   - Create worktree via `bead.WorktreeCreate(".worktrees/worktree-spec-<specID>", "spec/<specID>")`
   - Set `state.ActiveWorktree` to the absolute worktree path and `state.SpecBranch` to `spec/<specID>`
   - Copy the newly created spec dir into the worktree (or create it there directly)
   - Print `cd <worktree-path>` instruction prominently
2. Extend `cmd/mindspec/spec_init.go` to surface the worktree path
3. Extend `internal/next/beads.go` `EnsureWorktree()`:
   - Read `SpecBranch` from state — use it as the base for `bead/<beadID>` branches
   - Use config `WorktreeRoot` to place worktree at `.worktrees/worktree-<beadID>`
   - Change `worktreeCreate(wtName, branchName)` to branch from `specBranch` instead of current HEAD
4. Extend `cmd/mindspec/next.go`:
   - After worktree creation, update `state.ActiveWorktree` to the bead worktree path
5. Add/update tests

**Verification**
- [ ] `mindspec spec-init 999-test` creates a `spec/999-test` branch and worktree
- [ ] `state.json` has `activeWorktree` set after spec-init
- [ ] `mindspec next` creates `bead/<id>` branch from `spec/NNN-slug` (not main)
- [ ] `make test` passes

**Depends on**
Bead 2

## Bead 4: Worktree lifecycle in complete and impl-approve

**Provenance**: R1.4-R1.5 (complete merges bead→spec, impl-approve merges spec→main)

**Steps**
1. Extend `internal/complete/complete.go` `Run()`:
   - Before removing the bead worktree, merge `bead/<beadID>` back to `spec/<specID>` branch
   - Use `gitops.MergeBranch()` for the merge
   - After merge, update `state.ActiveWorktree` back to the spec worktree path (so agent returns to spec worktree)
   - Delete the bead branch after successful merge
2. Extend `internal/approve/impl.go` `ApproveImpl()`:
   - Read `state.SpecBranch` to know which branch to merge
   - Load config for `MergeStrategy`
   - If `pr` (or `auto` with remote): push spec branch, create PR via `gitops.CreatePR()`
   - If `direct` (or `auto` without remote): merge spec branch to main locally via `gitops.MergeBranch()`
   - Remove the spec worktree via `bead.WorktreeRemove()`
   - Clear `ActiveWorktree` and `SpecBranch` from state
3. Add/update tests

**Verification**
- [ ] `mindspec complete` merges bead branch back to `spec/NNN-slug`
- [ ] `mindspec complete` removes bead worktree and updates `activeWorktree` to spec worktree
- [ ] `mindspec approve impl` with `merge_strategy: direct` merges spec→main locally
- [ ] `mindspec approve impl` with `merge_strategy: pr` creates a PR
- [ ] `make test` passes

**Depends on**
Bead 3

## Bead 5: CLI guard rails (Layer 2) and instruct CWD redirect (soft layer)

**Provenance**: R3 (CLI guards), R5 (CWD redirect via instruct)

**Steps**
1. Create `internal/guard/guard.go`:
   - `CheckCWD(root string) error` — reads state, if `ActiveWorktree` is set and CWD is the main worktree (not the active worktree), returns error with message: `"mindspec: CWD is the main worktree. Switch to: cd <path>"`
   - `IsMainCWD(root string) bool` — returns true if CWD matches the main worktree and a worktree is active
   - Uses `config.Load()` to check if `cli_guards` is enabled
2. Add CWD guard calls to:
   - `cmd/mindspec/complete.go` — at start of RunE
   - `cmd/mindspec/approve.go` — `approveSpecCmd`, `approvePlanCmd` RunE (not `approveImplCmd` — that runs from main after all merges)
   - `cmd/mindspec/next.go` — guard is informational (next creates worktrees, so it's OK from main, but warn)
3. Extend `cmd/mindspec/instruct.go`:
   - After building context, if `guard.IsMainCWD(root)` and state has `ActiveWorktree`:
     - Replace normal template output with CWD redirect message only: `"You are in the main worktree. Run: cd <path> — then run mindspec instruct for guidance."`
     - Do NOT emit normal mode guidance, beads context, or warnings
   - This fires on every SessionStart, so it's the first thing the agent sees
4. Update `instruct_tail.go` `emitInstruct()` — same CWD check so commands like `spec-init` and `next` emit the redirect when appropriate
5. Rewrite `internal/instruct/worktree.go` `CheckWorktree()`:
   - Change signature to `CheckWorktree(activeWorktree string) string` (uses `activeWorktree` from state, not bead ID)
   - Compare CWD against `activeWorktree` path directly — no more `bd worktree list` call
   - This fixes the false alarm when called from instruct-tail of `mindspec next` (ADR-0019 §F)
6. Add unit tests: `internal/guard/guard_test.go`, update `internal/instruct/worktree_test.go`

**Verification**
- [ ] `mindspec complete` from main CWD fails with clear error mentioning worktree path
- [ ] `mindspec complete` from bead worktree CWD succeeds
- [ ] `mindspec instruct` from main CWD (when worktree active) emits ONLY redirect — no normal guidance
- [ ] `mindspec instruct` from worktree CWD emits normal mode guidance
- [ ] `make test` passes

**Depends on**
Bead 3

## Bead 6: Agent PreToolUse hooks (Layer 3)

**Provenance**: R4 (Agent PreToolUse hooks — deterministic)

**Steps**
1. Extend `internal/setup/claude.go` `wantedHooks()`:
   - Add `PreToolUse` entry with matcher `Write`:
     ```
     command: reads .mindspec/state.json, if activeWorktree is set and $CLAUDE_TOOL_ARG_FILE_PATH
     is not under activeWorktree, exit 2 with error message
     ```
   - Add `PreToolUse` entry with matcher `Edit`:
     ```
     command: same path check as Write
     ```
   - Add `PreToolUse` entry with matcher `Bash`:
     ```
     command: reads .mindspec/state.json, if activeWorktree is set and CWD (pwd) does not
     start with activeWorktree, exit 2 with error: "mindspec: blocked — your working
     directory is the main worktree. Run: cd <worktree-path>"
     ```
   - All hooks check `enforcement.agent_hooks` config (default true) — skip if disabled
   - All hooks are no-ops when state is idle (no activeWorktree)
2. Extend `internal/setup/copilot.go`:
   - Add equivalent PreToolUse hooks to `copilotHooksConfig()` and create helper scripts:
     - `.github/hooks/mindspec-worktree-guard.sh` — checks CWD and file paths
   - Update `mindspec-plan-gate.sh` to also check worktree enforcement
3. Create shared hook script logic:
   - Extract common enforcement shell logic into a helper function or shared script
   - Both Claude and Copilot hooks use the same logic: read state.json, check activeWorktree, compare paths
4. Add/update tests: `internal/setup/claude_test.go`, `internal/setup/copilot_test.go`

**Verification**
- [ ] `mindspec setup claude` installs PreToolUse hooks for Edit, Write, and Bash
- [ ] Claude Code Edit/Write hook blocks file writes outside active worktree
- [ ] Claude Code Bash hook blocks ALL bash execution when CWD is main and worktree is active
- [ ] `mindspec setup copilot` installs equivalent hooks
- [ ] `make test` passes

**Depends on**
Bead 1

## Bead 7: Git pre-commit hook (Layer 1)

**Provenance**: R2 (Git pre-commit hook)

**Steps**
1. Create `.mindspec/hooks/pre-commit` shell script:
   - Read `.mindspec/state.json` — if mode is not idle, check current branch
   - If branch is in `protected_branches` list (from `.mindspec/config.yaml` or default `[main, master]`): block commit with error message including the worktree path
   - Check `MINDSPEC_ALLOW_MAIN=1` escape hatch — if set, allow commit
   - Check `enforcement.pre_commit_hook` config — if disabled, allow commit
   - Exit 0 on success, exit 1 on block
2. Create `internal/hooks/install.go`:
   - `InstallPreCommit(root string) error` — installs the pre-commit hook via beads' chain mechanism (`bd hooks install --chain`) or directly into `.git/hooks/pre-commit`
   - Idempotent: if hook already installed, skip
3. Integrate hook installation:
   - `mindspec setup claude` calls `InstallPreCommit()`
   - `mindspec setup copilot` calls `InstallPreCommit()`
   - `mindspec spec-init` calls `InstallPreCommit()` (ensure hook is in place before any worktree work begins)
4. Add tests: `internal/hooks/install_test.go`

**Verification**
- [ ] Running `git commit` on main while mindspec is active fails with clear error mentioning worktree path
- [ ] `MINDSPEC_ALLOW_MAIN=1 git commit` on main succeeds (escape hatch works)
- [ ] `.mindspec/config.yaml` with `protected_branches: [main, develop]` protects both
- [ ] `make test` passes

**Depends on**
Bead 1

## Bead 8: Documentation

**Provenance**: R8 (Documentation)

**Steps**
1. Create `.mindspec/core/GIT-WORKFLOW.md`:
   - Zero-on-main invariant and rationale
   - Branch topology diagram (from ADR-0006): `main → spec/NNN-slug → bead/xxx`
   - Worktree location convention (`.worktrees/` directory, configurable via `worktree_root`)
   - Worktree lifecycle: when created (spec-init, next), what lives in each, when removed (complete, impl-approve)
   - Merge strategies (`pr` / `direct` / `auto`) with configuration
   - Three enforcement layers: what each catches, how they interact
   - Escape hatch (`MINDSPEC_ALLOW_MAIN=1`) and when to use it
   - Full lifecycle walkthrough: spec-init through impl-approve
   - Troubleshooting: stale worktrees, CWD mismatch, merge conflicts on spec branch
2. Update `.mindspec/core/USAGE.md`:
   - Add reference to GIT-WORKFLOW.md for branching/worktree details
   - Update workflow descriptions to mention worktree requirement
3. Update `.mindspec/docs/guides/claude-code.md`:
   - Mention worktree workflow and CWD requirement in quick start
   - Note that PreToolUse hooks enforce worktree isolation automatically
4. Update instruction templates (`internal/instruct/templates/*.md`):
   - `spec.md` — mention spec worktree context, `cd` requirement
   - `plan.md` — mention spec worktree context (shared with spec phase)
   - `implement.md` — update worktree isolation section for bead-specific worktrees branched from spec
   - `review.md` — mention returning to spec worktree for review

**Verification**
- [ ] `.mindspec/core/GIT-WORKFLOW.md` exists and covers all topics listed above
- [ ] `USAGE.md` references GIT-WORKFLOW.md
- [ ] Instruction templates reference worktree context
- [ ] `make build` succeeds (templates are embedded)
- [ ] `make test` passes

**Depends on**
Bead 5 (needs to know the final instruct/guard behavior)

## Dependency Graph

```
Bead 1 (config + state)
  ├── Bead 2 (git helpers)
  │     └── Bead 3 (spec-init + next worktree lifecycle)
  │           ├── Bead 4 (complete + impl-approve worktree lifecycle)
  │           └── Bead 5 (CLI guards + instruct redirect)
  │                 └── Bead 8 (documentation)
  ├── Bead 6 (agent PreToolUse hooks)
  └── Bead 7 (pre-commit hook)
```

## Provenance

| Acceptance Criterion | Bead | Verification |
|:---------------------|:-----|:-------------|
| `spec-init` creates branch + worktree, `activeWorktree` set | Bead 3 | `state.json` has `activeWorktree` after spec-init |
| `git commit` on main blocked when active | Bead 7 | Pre-commit hook returns error on protected branch |
| `MINDSPEC_ALLOW_MAIN=1` escape hatch | Bead 7 | Commit succeeds with env var set |
| `complete` from main CWD fails; from worktree succeeds | Bead 5 | CLI guard check in complete RunE |
| `instruct` from main emits ONLY redirect | Bead 5 | Redirect-only output, no normal guidance |
| `instruct` from worktree emits normal guidance | Bead 5 | Normal template rendering |
| `next` creates bead branch from `spec/NNN-slug` | Bead 3 | Branch parent is spec branch, not main |
| `complete` merges bead → spec branch | Bead 4 | `git log spec/NNN-slug` shows bead commits |
| `impl-approve` with `pr` creates PR | Bead 4 | `gh pr create` invoked |
| `impl-approve` with `direct` merges locally | Bead 4 | `git log main` shows merge commit |
| PreToolUse Edit/Write blocks outside worktree | Bead 6 | Hook script exits 2 for outside paths |
| PreToolUse Bash blocks all from main CWD | Bead 6 | Hook script exits 2 when CWD is main |
| `.mindspec/config.yaml` protects custom branches | Bead 1, 7 | Config loaded with custom `protected_branches` |
| GIT-WORKFLOW.md exists with required content | Bead 8 | File exists, covers all topics |
| Full lifecycle test (zero commits on main) | Bead 4 | `git log main --oneline` shows no direct commits |

## Risk Notes

- **`bd worktree create` base branch**: The current `bd worktree create` may not support `--start-point`. If not, Bead 2 will use `git branch` + `bd worktree create` as a two-step workaround. This does not require changes to beads.
- **Spec dir in worktree**: When `spec-init` creates the spec dir then creates a worktree, the spec dir exists on main but the worktree branches from main. The worktree will have the spec dir via the branch. No copy step needed — the worktree inherits the working tree state at branch creation.
- **State file location**: `state.json` lives at repo root (`.mindspec/state.json`). In a worktree, this is the worktree's root, not main's root. `workspace.FindRoot()` must correctly resolve in worktree contexts. This may need a minor fix in `FindRoot()` if it relies on `.git` being a directory (worktrees have `.git` as a file pointing to main).
- **Beads gitignore duplication**: Beads auto-adds individual worktree paths to `.gitignore`. With `.worktrees/` already ignored, these entries are redundant but harmless. No action needed.
