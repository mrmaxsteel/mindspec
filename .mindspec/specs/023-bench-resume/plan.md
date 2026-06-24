---
adr_citations:
    - id: ADR-0003
      sections:
        - CLI-first tooling
    - id: ADR-0004
      sections:
        - Go implementation
approved_at: "2026-02-14T16:47:27Z"
approved_by: user
bead_ids: []
last_updated: "2026-02-14"
spec_id: 023-bench-resume
status: Approved
version: 3
---

# Plan: 023 — Bench Resume

## Overview

Single implementation bead. Adds `bench resume` subcommand with a retry-based session runner that detects plan-mode stalls and auto-approves to drive sessions through to implementation.

## ADR Fitness

- **ADR-0003 (CLI-first)**: Sound. No divergence needed.
- **ADR-0004 (Go)**: Sound. All new code in Go.

## Spec Deviation

Spec requirement #7 specified an instruction wrapper that says "do not enter plan mode." Per user direction, this is changed: the prompt does NOT discourage plan mode. Instead, the retry loop detects stalls and auto-approves. This is fairer — it lets each session naturally choose whether to plan.

Additionally, session C now follows the full MindSpec workflow (`/spec-approve` → plan → `/plan-approve` → implement) rather than receiving the same "implement this" prompt as A and B.

## Key Design: Divergent Prompts per Session Type

### Sessions A & B (no-docs / baseline)

Initial prompt is neutral — just the feature request with the plan artifact:
```
Implement and commit the feature described below:

---

{plan-a.md or plan-b.md content}
```

If Claude enters plan mode and stalls, the retry auto-approves with:
- Retry 1: "Your plan is approved. Proceed to implementation. Write all code and tests, then commit."
- Retry 2: "Implementation is required. Write the code now and commit all changes."

### Session C (MindSpec)

Session C gets a workflow-native prompt. The approved spec is already on disk from phase 1. State is pre-set to `spec` mode so the MindSpec hooks fire with spec-mode guidance.

Initial prompt:
```
The specification at docs/specs/{specID}/spec.md is ready for review.
Follow the MindSpec workflow:
1. Review the spec, then use /spec-approve to approve it
2. Create a plan at docs/specs/{specID}/plan.md, then use /plan-approve
3. Implement all code and tests described in the plan
4. Commit your changes when complete
```

On retry, the auto-approve logic checks MindSpec state:
- If `mode=spec` (stuck at spec approval): update spec frontmatter to Approved, set state to `plan` mode. Retry prompt: "The spec is approved. Create a plan and use /plan-approve, then implement."
- If `mode=plan` (stuck at plan approval): update plan frontmatter to Approved, set state to `implement` mode. Retry prompt: "The plan is approved. Proceed to implementation. Write all code and tests, then commit."
- If `mode=implement` (stuck during implementation): Retry prompt: "Continue implementing. Write all remaining code and commit."

## Key Design: Retry Loop

Each session runs inside `runWithRetries()` (max 3 retries). After each `claude -p` invocation:

1. **Commit uncommitted changes** — `git add -A && git commit` if worktree is dirty
2. **Check for implementation code** — `git diff --name-only <baseCommit>..HEAD` looking for source files outside `docs/`, `.claude/`, `.mindspec/`, `.beads/`
3. **If implementation found** → session complete, break
4. **If no implementation** → auto-approve the current gate (plan mode for A/B, MindSpec state for C), build retry prompt, continue loop

### OTLP Collection Across Retries

Collector runs for the **entire retry loop**, not per-attempt. All telemetry events accumulate in one JSONL file for accurate total cost/token metrics.

This requires extracting `runClaude()` from `RunSession`. `RunSession` is unchanged for `bench run`; `runWithRetries()` manages collector lifecycle and calls `runClaude()` in the loop.

## Bead 1: Implement `bench resume`

**Steps**:

1. `internal/bench/session.go` — Add `Prompt string` to `SessionDef`. Extract core `claude -p` execution into `runClaude(ctx, prompt, wtPath, env, maxTurns, model, timeout, stdout, outFile) (exitCode, timedOut, err)`. `RunSession` calls `runClaude` internally (no behavior change for `bench run`).
2. `internal/bench/worktree.go` — Add `CheckoutWorktree(repoRoot, branch, wtPath)` that runs `git worktree add <wtPath> <branch>` for existing branch checkout.
3. `internal/bench/resume.go` (new file) — `ResumeConfig` struct, `Resume()` orchestrator (find branches, read artifacts, create worktrees, prepare session C state, run sessions with `runWithRetries()`, generate report), plus helpers: `findBenchBranches()`, `readArtifact()`, `hasCodeChanges()`, `prepareSessionC()`, `autoApprove()`, `buildRetryPrompt()`, `updateFrontmatterApproval()`, `getCurrentCommit()`. See design details in sections above.
4. `internal/bench/markdown.go` — Add `WriteResumeResults()` and `assembleResumeReportMD()` (same structure as existing functions but `-impl` suffixed filenames and "Implementation Phase" header).
5. `cmd/mindspec/bench.go` — Add `benchResumeCmd` with flags: `--spec-id`, `--timeout` (1800), `--max-turns` (0), `--max-retries` (3), `--model`, `--work-dir`, `--skip-cleanup`, `--skip-qualitative`, `--skip-commit`. Wire into `benchCmd.AddCommand()` in `init()`.
6. Verify: `make build`, `make test`, `mindspec bench resume --help`.

**Verification**:
- [ ] `make build` succeeds
- [ ] `make test` passes (no regressions)
- [ ] `mindspec bench resume --help` shows all flags

**Depends on**: None (standalone feature).
