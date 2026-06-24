# Spec 021-bench-go-command: Go Bench Run Command

## Goal

Replace `scripts/bench-e2e.sh` with a native `mindspec bench run` Go command that runs 3-session A/B/C benchmarks, produces N-way side-by-side quantitative reports (instead of 3 pairwise comparisons), and stores all benchmark artifacts in `docs/specs/<id>/benchmark/`.

## Background

Spec 019 delivered `scripts/bench-e2e.sh` — a ~770-line bash script that automates 3-session benchmarking (no-docs / baseline / mindspec). While functional, it has portability issues (macOS bash 3.2, missing `timeout`, `python3` dependency for JSON manipulation) and lives outside the Go codebase. The current pairwise report format (3 separate A-vs-B, A-vs-C, B-vs-C tables with deltas) is verbose — a single side-by-side table is more scannable.

Benchmark artifacts currently live in `/tmp/` and are lost after cleanup. Persisting them alongside the spec makes results reproducible and reviewable.

## Impacted Domains

- workflow: benchmark tooling is part of the development workflow for validating MindSpec effectiveness
- core: extends the `mindspec bench` CLI command family

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): CLI-first tooling — bench run becomes a first-class CLI command
- [ADR-0004](../../adr/ADR-0004.md): Go as implementation language — replaces shell script with Go

## Requirements

1. `mindspec bench run --spec-id <id> --prompt "..." [flags]` executes the full benchmark pipeline
2. Quantitative report uses N-way side-by-side format (all sessions in columns, no delta column)
3. `mindspec bench report` accepts 2+ JSONL files (backward compatible: 2 files = existing pairwise with delta)
4. All benchmark artifacts stored in `docs/specs/<id>/benchmark/` (report.md, improvements.md, telemetry JSONL, session outputs, trace)
5. OTLP collectors run in-process as goroutines (not as subprocesses)
6. Settings.json hook stripping uses Go `encoding/json` (no python3 dependency)
7. Port availability checks use `net.DialTimeout` (no curl dependency)
8. Session timeout uses `context.WithTimeout` (no timeout/gtimeout/perl)
9. Delete `scripts/bench-e2e.sh` after porting

## Scope

### In Scope
- `internal/bench/runner.go` — orchestration
- `internal/bench/worktree.go` — git worktree + neutralization
- `internal/bench/session.go` — session execution with in-process collector
- `internal/bench/qualitative.go` — analysis prompts + claude invocation
- `internal/bench/markdown.go` — report assembly + artifact management
- `internal/bench/report.go` — extend with N-way comparison
- `cmd/mindspec/bench.go` — add `bench run`, extend `bench report`
- `docs/core/BENCHMARKING.md` — update automated section
- Tests for all new code

### Out of Scope
- Changes to the OTLP collector itself
- Changes to the manual benchmarking workflow
- Parallel session execution (sessions remain sequential A→B→C)

## Non-Goals

- Replacing the manual benchmarking workflow (it remains available)
- Adding new session types beyond the existing 3 (no-docs, baseline, mindspec)
- Changing the qualitative analysis prompt structure

## Acceptance Criteria

- [ ] `mindspec bench run --help` prints usage with all flags
- [ ] `mindspec bench run --spec-id <id> --prompt "..." --skip-qualitative --skip-commit` creates worktrees, runs sessions, produces quantitative report
- [ ] `mindspec bench report a.jsonl b.jsonl c.jsonl --labels "x,y,z"` outputs N-way side-by-side table
- [ ] `mindspec bench report a.jsonl b.jsonl` still outputs existing pairwise table with delta (backward compat)
- [ ] Benchmark artifacts written to `docs/specs/<id>/benchmark/` (report.md, improvements.md, session JSONL, output txt, trace)
- [ ] `scripts/bench-e2e.sh` deleted
- [ ] All tests pass (`make test`)
- [ ] No python3, curl, timeout, or gtimeout dependencies

## Validation Proofs

- `make build && make test`: All tests pass
- `./bin/mindspec bench run --help`: Shows usage with --spec-id, --prompt, --timeout, --max-turns, --model, --work-dir, --skip-cleanup, --skip-qualitative, --skip-commit flags
- `./bin/mindspec bench report --help`: Shows usage accepting 2+ JSONL files

## Open Questions

None — design decisions resolved during planning.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-14
- **Notes**: Approved via mindspec approve spec