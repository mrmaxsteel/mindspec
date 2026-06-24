# Spec 023: Bench Resume — Implementation Phase Benchmarking

## Goal

Allow `mindspec bench resume` to pick up from a completed phase-1 benchmark run (where sessions produced plans/specs but no implementation code) and execute a second round of sessions that implement from those artifacts. This gives operators a complete planning-through-implementation benchmark comparison across all three session conditions (no-docs, baseline, mindspec).

## Background

`bench run` (Spec 021) executes three Claude Code sessions under different conditions and produces a comparative report. For complex features, all sessions may only produce plans or specs without implementing code — as observed in the 022-agentmind-viz-mvp benchmark where:

- **Session A** (no-docs) entered Claude Code's built-in plan mode, produced a Node.js plan (209 lines), but wrote zero files to disk (plan was internal to Claude Code session state)
- **Session B** (baseline) also entered plan mode, produced a Go plan (139 lines), but wrote zero files
- **Session C** (mindspec) correctly wrote a 126-line spec per the MindSpec workflow, but stopped at the spec phase awaiting approval

The phase-1 artifacts were recovered (plans from `~/.claude/plans/`, spec from the git branch) and saved to the benchmark directory as `plan-a.md`, `plan-b.md`, `spec-c.md`. The three bench branches (`bench-{a,b,c}-<spec-id>-<timestamp>`) still exist with their neutralized environments intact.

`bench resume` creates worktrees from those existing branches, injects each session's artifact as an implementation prompt (wrapped with "implement now, do not ask for approval"), and runs a second round of sessions with OTLP collection and report generation.

## Impacted Domains

- **core**: New `bench resume` subcommand; new `internal/bench/resume.go`
- **workflow**: No changes — benchmark infrastructure only

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): CLI-first — `bench resume` is a CLI command, follows existing `bench run` patterns
- [ADR-0004](../../adr/ADR-0004.md): Go implementation — all new code in Go

## Requirements

### Branch Discovery

1. `bench resume` finds existing phase-1 branches by pattern `bench-{a,b,c}-<spec-id>-*` using `git branch --list`
2. If multiple branch sets exist for the same spec-id (from multiple runs), the latest (lexicographically last, since timestamps sort correctly) is used
3. If any of the three branches (a, b, c) is missing, the command errors with a clear message

### Artifact Loading

4. Artifacts are read from `docs/specs/<spec-id>/benchmark/`: `plan-a.md`, `plan-b.md`, `spec-c.md` (or `plan-c.md` as fallback)
5. If an artifact file is missing, the command errors with a message naming the expected file path

### Prompt Construction

6. Each session's prompt is: `{instruction}\n\n---\n\n{artifact_content}`
7. The default instruction is: "Implement the feature described in the plan below. Do not enter plan mode or prompt for human approval. Proceed directly to writing all necessary code, tests, and documentation. Commit your changes when complete."
8. The instruction text is overridable via `--instruction` flag

### Session Execution

9. Worktrees are created from the existing phase-1 branches (checkout, not new branch creation)
10. For session C: MindSpec state (`.mindspec/state.json`) is set to `plan` mode with `activeSpec` = spec-id before running Claude (plan mode doesn't require a bead ID; this allows MindSpec hooks to provide plan/implementation guidance)
11. Sessions run sequentially (A → B → C) with per-session prompts, OTLP collection on ports 4318/4319/4320, and the same environment variables as `bench run`
12. Session C has `EnableTrace = true` (same as `bench run`)

### Report Generation

13. Quantitative report uses the same `CompareN` / `FormatTableN` as `bench run`
14. Implementation diffs are computed against the phase-1 branch tip (the last commit before resume), excluding `.beads`, `.mindspec`, `docs/specs`
15. Qualitative analysis runs the same Claude analysis pipeline as `bench run`
16. Results are written with `-impl` suffix to avoid overwriting phase-1 results: `report-impl.md`, `improvements-impl.md`, `session-impl-{a,b,c}.jsonl`, `output-impl-{a,b,c}.txt`, `trace-impl-c.jsonl`

### CLI Interface

17. `mindspec bench resume --spec-id <id> [--instruction "..."] [--timeout 1800] [--max-turns 0] [--model MODEL] [--work-dir DIR] [--skip-cleanup] [--skip-qualitative] [--skip-commit]`
18. `mindspec bench resume --help` prints usage

## Scope

### In Scope

- `cmd/mindspec/bench.go`: Add `benchResumeCmd`
- `internal/bench/resume.go`: New file — `ResumeConfig`, `Resume()`, branch discovery, artifact loading, prompt wrapping, session C state prep
- `internal/bench/session.go`: Add `Prompt` field to `SessionDef`, use in `RunSession`
- `internal/bench/worktree.go`: Add `CheckoutWorktree()` for creating worktrees from existing branches
- `internal/bench/markdown.go`: Add `WriteResumeResults()` and `assembleResumeReportMD()`

### Out of Scope

- Modifying `bench run` behavior
- Automatic plan recovery from `~/.claude/plans/` (manual step — user places artifacts in benchmark dir)
- Running only a subset of sessions (always runs all three)
- Parallel session execution

## Non-Goals

- Not a general "resume any benchmark" tool — specifically designed for the plan→implement phase transition
- Not responsible for recovering artifacts from Claude Code internals — artifacts must already exist in the benchmark directory
- Not changing the neutralization of sessions A and B — their environments remain as-is from phase 1

## Acceptance Criteria

- [ ] `mindspec bench resume --help` prints usage showing all flags
- [ ] `mindspec bench resume --spec-id 022-agentmind-viz-mvp --skip-qualitative --skip-commit --skip-cleanup` creates worktrees from existing `bench-{a,b,c}-022-*` branches
- [ ] Each session receives a prompt containing the instruction wrapper and its artifact content
- [ ] Session C's worktree has `.mindspec/state.json` set to plan mode before Claude runs
- [ ] OTLP telemetry is collected for all three sessions on ports 4318/4319/4320
- [ ] Results are written with `-impl` suffix: `report-impl.md`, `session-impl-{a,b,c}.jsonl`, `output-impl-{a,b,c}.txt`
- [ ] Phase-1 artifacts (`report.md`, `plan-a.md`, `plan-b.md`, `spec-c.md`) are not overwritten
- [ ] `make build` and `make test` pass

## Validation Proofs

- `mindspec bench resume --help`: prints usage with `--spec-id`, `--instruction`, `--timeout`, `--max-turns`, `--model`, `--work-dir`, `--skip-cleanup`, `--skip-qualitative`, `--skip-commit` flags
- `make test`: all existing tests pass, no regressions
- `git branch --list 'bench-*-022-*'`: confirms branches exist (prerequisite for resume)

## Open Questions

*None — design is fully specified above.*

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-14
- **Notes**: Approved via mindspec approve spec