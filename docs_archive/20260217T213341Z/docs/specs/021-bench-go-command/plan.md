---
adr_citations:
    - id: ADR-0003
      sections:
        - Decision
        - Decision Details
    - id: ADR-0004
      sections:
        - Decision
        - Rationale
approved_at: "2026-02-14T15:00:28Z"
approved_by: user
bead_ids: []
last_updated: "2026-02-14"
spec_id: 021-bench-go-command
status: Approved
version: 1
---

# Plan: Spec 021 — Go Bench Run Command

## Summary

Port `scripts/bench-e2e.sh` (~770 lines) to `mindspec bench run`, add N-way side-by-side reporting, and persist benchmark artifacts in `docs/specs/<id>/benchmark/`.

## ADR Fitness

### ADR-0003: Centralized Agent Instruction Emission via MindSpec CLI

**Verdict: Sound — conform.**

`bench run` becomes a first-class CLI command, consistent with the "CLI-first tooling" principle. The benchmarking pipeline (which currently lives as a shell script outside the Go codebase) moves into the `mindspec` binary, giving it the same distribution, versioning, and discoverability properties as other commands. No divergence needed.

### ADR-0004: Go as Implementation Language

**Verdict: Sound — conform.**

This spec is literally about replacing a bash script with Go. The port eliminates external dependencies (python3, curl, timeout/gtimeout) by using Go stdlib (`encoding/json`, `net.DialTimeout`, `context.WithTimeout`). The in-process OTLP collector (running as a goroutine) is possible precisely because the collector is already Go code in the same module. No divergence needed.

## Bead 1: N-way report formatting

**Scope**: Add N-way side-by-side comparison to `internal/bench/report.go` alongside existing 2-way code. Update CLI to accept 2+ JSONL files.

**Steps**
1. Add `MultiReport` struct holding `[]*Session` and `mergedModelNamesN()` helper
2. Add `CompareN(sessions []*Session) *MultiReport` that aggregates N sessions without computing deltas
3. Add `FormatTableN(r *MultiReport) string` — N columns, no delta column, dynamic width based on label length
4. Update `cmd/mindspec/bench.go`: change `benchReportCmd` from `ExactArgs(2)` to `MinimumNArgs(2)`; route 3+ args through N-way formatter
5. Add tests: `TestCompareN` (3 sessions), `TestFormatTableN` (verify columns and no delta), verify 2-arg path still uses pairwise

**Verification**
- [ ] `make test` passes
- [ ] `mindspec bench report a.jsonl b.jsonl` still produces pairwise table with delta
- [ ] `mindspec bench report a.jsonl b.jsonl c.jsonl --labels "x,y,z"` produces N-way table

**Depends on**: none

## Bead 2: Worktree management and neutralization

**Scope**: Create `internal/bench/worktree.go` — git worktree creation/removal, neutralization logic for baseline and no-docs sessions, settings.json hook stripping via `encoding/json`.

**Steps**
1. Implement `CreateWorktree(repoRoot, branch, wtPath, commit string) error` wrapping `git worktree add`
2. Implement `RemoveWorktree(repoRoot, wtPath string) error` wrapping `git worktree remove --force`
3. Implement `NeutralizeBaseline(wtPath string) error` — rm CLAUDE.md, .mindspec/, specific .claude/commands/*.md, strip hooks from settings.json
4. Implement `NeutralizeNoDocs(wtPath string) error` — calls NeutralizeBaseline then rm docs/
5. Implement `stripHooks(settingsPath string) error` — read/parse/delete hooks key/write with `encoding/json` (no python3)
6. Add tests with temp dir fixtures: verify correct files removed/retained, verify settings.json hook stripping preserves other keys

**Verification**
- [ ] `make test` passes
- [ ] No python3 dependency in any code path
- [ ] stripHooks preserves non-hook keys in settings.json

**Depends on**: none

## Bead 3: Session execution with in-process collector

**Scope**: Create `internal/bench/session.go` — runs a single claude session with an in-process OTLP collector goroutine, tees output, handles timeout via context.

**Steps**
1. Define `SessionResult` struct (Label, JSONLPath, OutputPath, EventCount, Duration)
2. Define `SessionDef` struct (Label, Description, Port, Neutralize func, EnableTrace bool)
3. Implement `RunSession(ctx, cfg, def, workDir)` — starts collector goroutine, runs `claude -p` with OTEL env vars, tees output via `io.MultiWriter`
4. Implement timeout via `context.WithTimeout` and graceful shutdown via `cmd.Cancel` with SIGTERM
5. Implement port availability check via `net.DialTimeout` (no curl)
6. Add unit tests for config wiring, env var construction, and port checking (test HTTP server for waitForPort)

**Verification**
- [ ] `make test` passes
- [ ] Port checks use `net.DialTimeout` (no curl)
- [ ] Timeouts use `context.WithTimeout` (no timeout/gtimeout/perl)

**Depends on**: Bead 2

## Bead 4: Qualitative analysis and report assembly

**Scope**: Create `internal/bench/qualitative.go` (analysis prompts + claude invocation) and `internal/bench/markdown.go` (report assembly + artifact persistence to `docs/specs/<id>/benchmark/`).

**Steps**
1. Implement `buildQualPrompt(input)` and `buildImprovPrompt(input)` — 7-dimension rating prompt matching shell script structure
2. Implement `runClaudeAnalysis(prompt) (string, error)` — pipes to `claude -p --no-session-persistence`
3. Implement `collectPlans(cfg, sessions)` and `generateDiffs(cfg, sessions)` for qualitative input gathering
4. Implement `BenchmarkDir(repoRoot, specID)` returning `docs/specs/<id>/benchmark/`
5. Implement `WriteResults(...)` — creates dir, writes report.md + improvements.md, copies JSONL/output/trace files
6. Implement `assembleReportMD(...)` — markdown template using N-way table format
7. Add tests for prompt construction, report assembly, artifact directory structure

**Verification**
- [ ] `make test` passes
- [ ] Report uses single N-way table format (not 3 pairwise blocks)
- [ ] Artifacts written to correct directory structure

**Depends on**: Bead 1

## Bead 5: Runner orchestration, CLI wiring, and cleanup

**Scope**: Create `internal/bench/runner.go` (orchestration entry point), wire `bench run` in CLI, delete shell script, update docs.

**Steps**
1. Define `RunConfig` struct (SpecID, Prompt, PromptFile, Timeout, MaxTurns, Model, WorkDir, SkipCleanup, SkipQualitative, SkipCommit)
2. Implement `Run(cfg *RunConfig) error` — full pipeline: prereqs → worktrees → neutralize → sessions A→B→C → report → qualitative → write results → commit → cleanup
3. Implement prereq checks: `claude`/`git` on PATH, `bin/mindspec` exists, clean git tree, ports 4318-4320 free
4. Wire `benchRunCmd` in `cmd/mindspec/bench.go` with all flags (--spec-id, --prompt, --prompt-file, --timeout, --max-turns, --model, --work-dir, --skip-cleanup, --skip-qualitative, --skip-commit)
5. Delete `scripts/bench-e2e.sh` and `scripts/` dir if empty; update `docs/core/BENCHMARKING.md` automated section
6. Add tests for prereq validation, prompt file loading, RunConfig defaults

**Verification**
- [ ] `make build && make test` passes
- [ ] `mindspec bench run --help` shows all flags
- [ ] `scripts/bench-e2e.sh` deleted
- [ ] No python3, curl, timeout, or gtimeout dependencies

**Depends on**: Bead 1, Bead 2, Bead 3, Bead 4

## Key Design Decisions

1. **In-process collector**: Run `bench.Collector` as goroutine with context cancellation instead of spawning subprocess. Simpler, no PID management, no zombie process risk.
2. **`encoding/json` replaces python3**: For settings.json hook stripping during neutralization.
3. **`net.DialTimeout` replaces curl**: For port availability checks before starting collectors.
4. **`context.WithTimeout` replaces timeout/gtimeout/perl**: For session execution timeout. Uses `cmd.Cancel` with SIGTERM for graceful shutdown.
5. **Keep existing 2-way report**: `FormatTable`/`Compare` unchanged for backward compat. N-way is additive.
6. **Sequential sessions preserved**: A→B→C order, MindSpec session last to avoid cache warmup advantage.

## Output Directory Structure

```
docs/specs/<id>/benchmark/
  report.md           # full benchmark report (quantitative + qualitative)
  improvements.md     # extracted improvements from non-MindSpec sessions
  session-a.jsonl     # telemetry NDJSON
  session-b.jsonl
  session-c.jsonl
  output-a.txt        # claude session output
  output-b.txt
  output-c.txt
  trace-c.jsonl       # mindspec trace (session C only)
```

## Validation Proofs

- `make build && make test`: All tests pass
- `./bin/mindspec bench run --help`: Shows usage with --spec-id, --prompt, --timeout, --max-turns, --model, --work-dir, --skip-cleanup, --skip-qualitative, --skip-commit flags
- `./bin/mindspec bench report --help`: Shows usage accepting 2+ JSONL files
