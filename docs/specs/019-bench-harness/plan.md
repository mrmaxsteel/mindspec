---
approved_at: "2026-02-14T09:05:36Z"
approved_by: user
last_updated: "2026-02-14"
spec_id: 019-bench-harness
status: Approved
version: "0.1"
work_chunks:
    - depends_on: []
      id: 1
      scope: scripts/bench-e2e.sh
      title: Create bench-e2e.sh orchestration script
      verify:
        - shellcheck scripts/bench-e2e.sh passes with no errors
        - scripts/bench-e2e.sh --help prints usage
        - Script creates 3 worktrees, neutralizes B and C correctly
        - Collectors start/stop per session without port conflicts
        - claude -p runs with CLAUDECODE unset and correct OTEL env vars
        - Diffs exclude .beads/.mindspec/docs/specs artifacts
        - benchmark.md contains metadata, 3 pairwise reports, qualitative analysis
        - improvements.md contains actionable findings
        - Cleanup trap removes worktrees on exit/error/interrupt
    - depends_on:
        - 1
      id: 2
      scope: docs/core/BENCHMARKING.md
      title: Update BENCHMARKING.md with automated workflow
      verify:
        - New 'Automated E2E' section references scripts/bench-e2e.sh
        - Documents all flags and the 3-session model
---

# Plan: Spec 019 — End-to-End Benchmark Harness

**Spec**: [spec.md](spec.md)

---

## Context

The existing `mindspec bench` subcommands (setup, collect, report) handle individual pieces of the benchmark workflow. This plan creates an orchestration script that composes them into a single automated run with 3 sessions and qualitative analysis.

No Go changes needed — the script composes existing CLI commands: `mindspec bench collect`, `mindspec bench report`, `mindspec trace summary`, `claude -p`, and `git worktree`.

## ADR Fitness

- **ADR-0004 (Observability)**: The bench harness builds directly on the OTEL telemetry infrastructure from Spec 018. The ADR's design of separate collector ports per session and NDJSON output is well-suited for the 3-session model. No divergence needed.

---

## Bead 019-A: Create `scripts/bench-e2e.sh`

**Scope**: The main orchestration script (~300 lines of bash)

**Steps**:

1. Create `scripts/bench-e2e.sh` with the following function structure:
   - `parse_args()` — getopts-style parsing for `--spec-id`, `--prompt`, `--prompt-file`, `--timeout`, `--max-turns`, `--model`, `--work-dir`, `--skip-cleanup`, `--skip-qualitative`, `--skip-commit`
   - `usage()` — prints help text
   - `validate_prerequisites()` — checks: `claude` on PATH, `mindspec` binary built, clean git tree, ports 4318-4320 free
   - `create_worktree()` — `git worktree add --detach` + `checkout -b` with timestamped branch names
   - `neutralize_baseline()` — removes CLAUDE.md, .claude/, .mindspec/
   - `neutralize_nodocs()` — above + removes docs/
   - `wait_for_port()` — polls HTTP endpoint until collector is ready
   - `run_session()` — starts collector, runs `claude -p` with correct env vars, stops collector, auto-commits worktree changes
   - `collect_plans()` — extracts plan artifacts: Session A's plan from `docs/specs/<ID>/plan.md`, Sessions B/C plans from `.claude/plans/*.md` in each worktree (Claude's built-in `/plan` mode writes plans there)
   - `generate_diffs()` — `git diff` excluding .beads/.mindspec/docs/specs
   - `run_qualitative()` — structured comparison prompt via `claude -p`
   - `run_improvements()` — focused "what did B/C do better" prompt via `claude -p`
   - `assemble_benchmark_md()` — heredoc assembly of the full report
   - `cleanup()` — trap handler: kill collectors, remove worktrees, prune
   - `main()` — orchestration flow

2. Key implementation details:
   - **Claude invocation**: `(cd "${wt_path}" && CLAUDECODE= CLAUDE_CODE_ENABLE_TELEMETRY=1 ... timeout "${TIMEOUT}" claude -p "${PROMPT}" --dangerously-skip-permissions --no-session-persistence)`
   - Session A additionally sets `MINDSPEC_TRACE`
   - **Collector lifecycle**: one per session (start before, sleep 2s after, SIGTERM)
   - **Plan capture for B and C**: After each session completes, copy Claude's plan artifact (the file written during `/plan` mode, typically at `.claude/plans/*.md` or the plan file referenced in output) into the worktree as `plan.md` at the repo root. If no plan file is found, extract the plan from the session output (`output-{b,c}.txt`). This enables comparing all three planning approaches in the qualitative review.
   - **Diff size guard**: truncate each diff at 100K chars for the qualitative prompt
   - **Qualitative prompt**: asks for 1-5 ratings on 6 dimensions (architecture, code quality, test quality, documentation, functional completeness, project consistency) plus overall verdict, key differentiators, surprising findings. Also compares the planning artifacts: Session A's `docs/specs/<ID>/plan.md` vs B and C's Claude-generated plans.
   - **Improvements prompt**: separate call focused on actionable items from B/C

3. Make executable: `chmod +x scripts/bench-e2e.sh`

**Verification**:
- [ ] `shellcheck scripts/bench-e2e.sh` — zero errors
- [ ] `scripts/bench-e2e.sh --help` — prints usage
- [ ] Trap cleanup works on SIGINT (Ctrl-C kills worktrees)

**Depends on**: nothing

---

## Bead 019-B: Update BENCHMARKING.md

**Scope**: Add automated E2E section to existing benchmarking docs

**Steps**:
1. Add "## Automated E2E Benchmarking" section after the existing manual workflow
2. Document the 3-session model (mindspec / baseline / no-docs)
3. Show example invocation and flag reference
4. Note that the manual workflow remains available for interactive sessions

**Verification**:
- [ ] New section exists with working example command
- [ ] Existing manual workflow content unchanged

**Depends on**: 019-A

---

## Dependency Graph

```
019-A (bench-e2e.sh script)
  └── 019-B (BENCHMARKING.md update)
```
