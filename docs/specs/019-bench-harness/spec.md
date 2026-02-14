# Spec 019-bench-harness: End-to-End Benchmark Harness

## Goal

Automate the full MindSpec A/B/C benchmarking workflow into a single script. Given a feature prompt and spec ID, the script runs three Claude Code sessions under different conditions (full MindSpec, baseline with docs, no docs), collects telemetry, generates quantitative cost/token reports, produces a qualitative code quality analysis, and commits structured results to the spec folder. This enables validating MindSpec's effectiveness on every new feature.

## Background

The current benchmarking workflow (documented in `docs/core/BENCHMARKING.md`) is entirely manual — creating worktrees, neutralizing files, starting collectors, launching Claude sessions interactively, tagging results, and running `mindspec bench report`. The Spec 015 benchmark took ~45 minutes of operator overhead on top of actual session time.

The manual process also only compared two sessions (MindSpec vs baseline). A third session — baseline without any project documentation — is needed to measure the value of MindSpec's documentation layer independently from its workflow tooling.

Additionally, the manual process produced only a quantitative report (tokens/cost). The qualitative analysis (architecture, code quality, test quality, completeness) was done ad-hoc in conversation. Automating this produces repeatable, comparable results across specs.

## Impacted Domains

- **core**: New script in `scripts/`, updates to BENCHMARKING.md
- **workflow**: Leverages existing `mindspec bench collect` and `mindspec bench report` commands

## ADR Touchpoints

- [ADR-0004](../../adr/ADR-0004.md): Observability — the bench harness builds on the OTEL telemetry and trace infrastructure from Spec 018

## Requirements

1. Script takes a feature prompt (string or file) and spec ID as arguments
2. Runs 3 sessions from the same git commit, each in an isolated git worktree:
   - **A (mindspec)**: Full tooling — CLAUDE.md, .claude/ hooks, .mindspec/, docs/
   - **B (baseline)**: No CLAUDE.md, .claude/, .mindspec/ — but docs/ still present
   - **C (no-docs)**: No CLAUDE.md, .claude/, .mindspec/, AND no docs/
3. All sessions receive the exact same prompt; differentiation comes from environment only
4. Each session runs `claude -p` (non-interactive print mode) with `--dangerously-skip-permissions`
5. Each session has its own OTEL collector on a unique port (4318, 4319, 4320)
6. Sessions run sequentially (avoids API rate limits)
7. After sessions complete, generates 3 pairwise quantitative reports via `mindspec bench report`
8. Runs qualitative code analysis via `claude -p`, rating each session on: architecture, code quality, test quality, documentation, functional completeness, consistency with project conventions
9. Generates `improvements.md` via `claude -p` — specifically what B and C did better than A
10. Writes `benchmark.md` and `improvements.md` to `docs/specs/<ID>/`
11. Optionally commits results on the current branch
12. Cleans up worktrees on exit (with `--skip-cleanup` override)
13. Diffs for qualitative analysis exclude spec/beads/mindspec artifacts to focus on code
14. Must unset `CLAUDECODE` env var before spawning nested `claude -p` sessions

## Scope

### In Scope
- `scripts/bench-e2e.sh` — the orchestration script
- `docs/core/BENCHMARKING.md` — add "Automated E2E" section

### Out of Scope
- Go code changes (existing `bench collect`, `bench report`, `trace summary` are sufficient)
- Parallel session execution (potential future enhancement)
- CI/CD integration

## Non-Goals

- Replacing the manual workflow — the script is complementary, not a replacement
- Modifying the bench report command to support 3-way comparison (pairwise is sufficient)
- Making the qualitative analysis deterministic (LLM output varies; the structure is fixed)

## Acceptance Criteria

- [ ] `scripts/bench-e2e.sh --help` prints usage with all flags documented
- [ ] `shellcheck scripts/bench-e2e.sh` passes with no errors
- [ ] Script creates 3 worktrees from the current commit and cleans them up on exit (including on error/interrupt)
- [ ] Session B has CLAUDE.md, .claude/, .mindspec/ removed but docs/ intact
- [ ] Session C has CLAUDE.md, .claude/, .mindspec/, AND docs/ removed
- [ ] Each session's telemetry is collected to a separate JSONL file
- [ ] Script produces `benchmark.md` containing: metadata, session table, 3 pairwise quantitative reports, qualitative analysis
- [ ] Script produces `improvements.md` with actionable findings from B/C that A should adopt
- [ ] `--skip-qualitative` flag skips the claude -p analysis steps (produces quantitative-only report)
- [ ] `--skip-commit` flag prevents git commit of results
- [ ] `--timeout` flag limits per-session duration (default 30 min)
- [ ] `--prompt-file` reads prompt from a file for multi-line feature descriptions

## Validation Proofs

- `shellcheck scripts/bench-e2e.sh`: Zero errors
- `scripts/bench-e2e.sh --help`: Prints usage
- Short dry run: `scripts/bench-e2e.sh --spec-id test --prompt "Add a hello command that prints hello" --timeout 120 --skip-commit` — creates worktrees, runs 3 short sessions, produces benchmark.md

## Open Questions

*None — all resolved during planning discussion.*

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-14
- **Notes**: Approved via mindspec approve spec