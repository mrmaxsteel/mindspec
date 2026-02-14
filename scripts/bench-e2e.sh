#!/usr/bin/env bash
set -euo pipefail

# bench-e2e.sh — End-to-end A/B/C benchmark harness for MindSpec
#
# Runs 3 Claude Code sessions under different conditions against the same
# prompt, collects telemetry, generates quantitative + qualitative reports,
# and writes results to docs/specs/<ID>/.

# ── Constants ──────────────────────────────────────────────────────────────
PORT_A=4318
PORT_B=4319
PORT_C=4320
DEFAULT_TIMEOUT=1800
MAX_DIFF_CHARS=100000

# ── Globals (set by parse_args) ────────────────────────────────────────────
SPEC_ID=""
PROMPT=""
PROMPT_FILE=""
SESSION_TIMEOUT="${DEFAULT_TIMEOUT}"
MAX_TURNS=""
MODEL=""
WORK_DIR=""
SKIP_CLEANUP=false
SKIP_QUALITATIVE=false
SKIP_COMMIT=false
REPO_ROOT=""
BENCH_COMMIT=""
TIMESTAMP=""
COLLECTOR_PIDS=()

# ── Usage ──────────────────────────────────────────────────────────────────
usage() {
    cat <<'USAGE'
Usage: scripts/bench-e2e.sh --spec-id <ID> --prompt "..." [options]
       scripts/bench-e2e.sh --spec-id <ID> --prompt-file <path> [options]

Run 3 Claude Code sessions under different conditions and produce a
comparative benchmark report.

  Session A (mindspec):  Full MindSpec tooling
  Session B (baseline):  No CLAUDE.md/.mindspec; hooks stripped; docs/ present
  Session C (no-docs):   No CLAUDE.md/.mindspec; hooks stripped; no docs/

Required:
  --spec-id <NNN-slug>    Spec folder ID (e.g., 015-project-bootstrap)
  --prompt <string>       Feature prompt (same for all 3 sessions)
  --prompt-file <path>    Read prompt from file (overrides --prompt)

Options:
  --timeout <seconds>     Per-session timeout (default: 1800 = 30 min)
  --max-turns <int>       Max agentic turns per session (default: unlimited)
  --model <model>         Claude model for all sessions (default: system default)
  --work-dir <path>       Base dir for worktrees (default: /tmp/mindspec-bench-<spec-id>)
  --skip-cleanup          Preserve worktrees and temp files after completion
  --skip-qualitative      Skip qualitative analysis (quantitative report only)
  --skip-commit           Don't commit results to docs/specs/
  --help                  Show this help
USAGE
}

# ── Argument Parsing ───────────────────────────────────────────────────────
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --spec-id)      SPEC_ID="$2"; shift 2 ;;
            --prompt)       PROMPT="$2"; shift 2 ;;
            --prompt-file)  PROMPT_FILE="$2"; shift 2 ;;
            --timeout)      SESSION_TIMEOUT="$2"; shift 2 ;;
            --max-turns)    MAX_TURNS="$2"; shift 2 ;;
            --model)        MODEL="$2"; shift 2 ;;
            --work-dir)     WORK_DIR="$2"; shift 2 ;;
            --skip-cleanup)     SKIP_CLEANUP=true; shift ;;
            --skip-qualitative) SKIP_QUALITATIVE=true; shift ;;
            --skip-commit)      SKIP_COMMIT=true; shift ;;
            --help|-h)      usage; exit 0 ;;
            *) echo "Unknown option: $1" >&2; usage >&2; exit 1 ;;
        esac
    done

    # Read prompt from file if specified
    if [[ -n "${PROMPT_FILE}" ]]; then
        if [[ ! -f "${PROMPT_FILE}" ]]; then
            echo "ERROR: prompt file not found: ${PROMPT_FILE}" >&2
            exit 1
        fi
        PROMPT="$(cat "${PROMPT_FILE}")"
    fi

    if [[ -z "${SPEC_ID}" ]]; then
        echo "ERROR: --spec-id is required" >&2
        usage >&2
        exit 1
    fi
    if [[ -z "${PROMPT}" ]]; then
        echo "ERROR: --prompt or --prompt-file is required" >&2
        usage >&2
        exit 1
    fi
}

# ── Prerequisites ──────────────────────────────────────────────────────────
validate_prerequisites() {
    local errors=0

    for cmd in claude git curl; do
        if ! command -v "${cmd}" >/dev/null 2>&1; then
            echo "ERROR: ${cmd} not found on PATH" >&2
            errors=$((errors + 1))
        fi
    done

    if [[ ! -x "${REPO_ROOT}/bin/mindspec" ]]; then
        echo "ERROR: mindspec binary not found. Run 'make build' first." >&2
        errors=$((errors + 1))
    fi

    if ! git -C "${REPO_ROOT}" diff --quiet || ! git -C "${REPO_ROOT}" diff --cached --quiet; then
        echo "ERROR: Git working tree is not clean. Commit or stash changes." >&2
        errors=$((errors + 1))
    fi

    for port in "${PORT_A}" "${PORT_B}" "${PORT_C}"; do
        if curl -sf "http://localhost:${port}/v1/metrics" -X POST -d '{}' >/dev/null 2>&1; then
            echo "ERROR: Port ${port} is already in use" >&2
            errors=$((errors + 1))
        fi
    done

    if [[ ${errors} -gt 0 ]]; then
        echo "Prerequisites check failed with ${errors} error(s)." >&2
        exit 1
    fi
}

# ── Worktree Management ───────────────────────────────────────────────────
create_worktree() {
    local branch_name="$1"
    local wt_path="$2"

    git -C "${REPO_ROOT}" worktree add --detach "${wt_path}" "${BENCH_COMMIT}" 2>/dev/null
    git -C "${wt_path}" checkout -b "${branch_name}" 2>/dev/null
}

neutralize_baseline() {
    local wt="$1"
    rm -f "${wt}/CLAUDE.md"
    rm -rf "${wt}/.mindspec/"

    # Remove mindspec-specific commands but keep .claude/ directory
    rm -f "${wt}/.claude/commands/spec-init.md"
    rm -f "${wt}/.claude/commands/spec-approve.md"
    rm -f "${wt}/.claude/commands/plan-approve.md"
    rm -f "${wt}/.claude/commands/impl-approve.md"
    rm -f "${wt}/.claude/commands/spec-status.md"

    # Strip mindspec hooks from settings.json, keep the rest
    local settings="${wt}/.claude/settings.json"
    if [[ -f "${settings}" ]]; then
        # Replace with empty hooks (preserves any non-hook settings)
        python3 -c "
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
data.pop('hooks', None)
with open(sys.argv[1], 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
" "${settings}" 2>/dev/null || rm -f "${settings}"
    fi
}

neutralize_nodocs() {
    local wt="$1"
    neutralize_baseline "${wt}"
    rm -rf "${wt}/docs/"
}

# ── Port Readiness ─────────────────────────────────────────────────────────
wait_for_port() {
    local port="$1"
    local max_wait="${2:-5}"
    local waited=0

    while ! curl -sf "http://localhost:${port}/v1/metrics" -X POST -d '{}' >/dev/null 2>&1; do
        sleep 0.5
        waited=$((waited + 1))
        if [[ ${waited} -ge $((max_wait * 2)) ]]; then
            echo "ERROR: Collector on port ${port} failed to start" >&2
            return 1
        fi
    done
}

# ── Session Runner ─────────────────────────────────────────────────────────
run_session() {
    local label="$1"
    local wt_path="$2"
    local port="$3"
    local output_jsonl="$4"
    local trace_path="$5"  # empty for non-MindSpec sessions

    echo ""
    echo "━━━ Session ${label} (port ${port}) ━━━"
    echo ""

    # Start collector in background
    "${REPO_ROOT}/bin/mindspec" bench collect --port "${port}" --output "${output_jsonl}" &
    local collector_pid=$!
    COLLECTOR_PIDS+=("${collector_pid}")

    wait_for_port "${port}" 5

    # Build claude command
    local claude_args=(-p "${PROMPT}" --dangerously-skip-permissions)
    if [[ -n "${MAX_TURNS}" ]]; then
        claude_args+=(--max-turns "${MAX_TURNS}")
    fi
    if [[ -n "${MODEL}" ]]; then
        claude_args+=(--model "${MODEL}")
    fi

    # Run claude in the worktree
    local claude_exit=0
    (
        cd "${wt_path}"
        export CLAUDECODE=
        export CLAUDE_CODE_ENABLE_TELEMETRY=1
        export OTEL_METRICS_EXPORTER=otlp
        export OTEL_LOGS_EXPORTER=otlp
        export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
        export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:${port}"
        if [[ -n "${trace_path}" ]]; then
            export MINDSPEC_TRACE="${trace_path}"
        fi
        timeout "${SESSION_TIMEOUT}" claude "${claude_args[@]}" \
            > "${WORK_DIR}/output-${label}.txt" 2>&1
    ) || claude_exit=$?

    if [[ ${claude_exit} -eq 124 ]]; then
        echo "WARNING: Session ${label} timed out after ${SESSION_TIMEOUT}s"
    elif [[ ${claude_exit} -ne 0 ]]; then
        echo "WARNING: Session ${label} exited with code ${claude_exit}"
    fi

    # Give collector time to flush, then stop
    sleep 2
    kill "${collector_pid}" 2>/dev/null || true
    wait "${collector_pid}" 2>/dev/null || true

    # Remove collector PID from array
    local new_pids=()
    for pid in "${COLLECTOR_PIDS[@]}"; do
        [[ "${pid}" != "${collector_pid}" ]] && new_pids+=("${pid}")
    done
    COLLECTOR_PIDS=("${new_pids[@]+"${new_pids[@]}"}")

    # Auto-commit any changes in the worktree
    if git -C "${wt_path}" diff --quiet && \
       git -C "${wt_path}" diff --cached --quiet && \
       [[ -z "$(git -C "${wt_path}" ls-files --others --exclude-standard)" ]]; then
        echo "Session ${label}: no changes produced"
    else
        git -C "${wt_path}" add -A
        git -C "${wt_path}" commit -m "bench: Session ${label} output" --no-verify 2>/dev/null || true
    fi

    local event_count=0
    if [[ -f "${output_jsonl}" ]]; then
        event_count=$(wc -l < "${output_jsonl}" | tr -d ' ')
    fi
    echo "Session ${label} complete. Events: ${event_count}"
}

# ── Plan Collection ────────────────────────────────────────────────────────
collect_plans() {
    local plan_a="" plan_b="" plan_c=""

    # Session A: mindspec plan is at docs/specs/<ID>/plan.md
    local spec_plan="${WORK_DIR}/wt-a/docs/specs/${SPEC_ID}/plan.md"
    if [[ -f "${spec_plan}" ]]; then
        plan_a="$(cat "${spec_plan}")"
    fi

    # Sessions B and C: Claude's built-in /plan mode writes to .claude/plans/
    for label in b c; do
        local wt="${WORK_DIR}/wt-${label}"
        local plan_content=""

        # Check .claude/plans/ directory for any plan files
        if [[ -d "${wt}/.claude/plans" ]]; then
            local plan_file
            plan_file=$(find "${wt}/.claude/plans" -name '*.md' -type f 2>/dev/null | head -1)
            if [[ -n "${plan_file}" ]]; then
                plan_content="$(cat "${plan_file}")"
            fi
        fi

        if [[ -z "${plan_content}" ]]; then
            plan_content="(No plan artifact found for Session ${label^^})"
        fi

        if [[ "${label}" == "b" ]]; then
            plan_b="${plan_content}"
        else
            plan_c="${plan_content}"
        fi
    done

    PLAN_A="${plan_a}"
    PLAN_B="${plan_b}"
    PLAN_C="${plan_c}"
}

# ── Diff Generation ────────────────────────────────────────────────────────
generate_diffs() {
    DIFF_A="$(git -C "${WORK_DIR}/wt-a" diff "${BENCH_COMMIT}" HEAD \
        -- ':(exclude).beads' ':(exclude).mindspec' ':(exclude)docs/specs' 2>/dev/null || true)"
    DIFF_B="$(git -C "${WORK_DIR}/wt-b" diff "${BENCH_COMMIT}" HEAD \
        -- ':(exclude).beads' ':(exclude).mindspec' ':(exclude)docs/specs' 2>/dev/null || true)"
    DIFF_C="$(git -C "${WORK_DIR}/wt-c" diff "${BENCH_COMMIT}" HEAD \
        -- ':(exclude).beads' ':(exclude).mindspec' ':(exclude)docs/specs' 2>/dev/null || true)"

    # Truncate if too large
    for var in DIFF_A DIFF_B DIFF_C; do
        local val="${!var}"
        if [[ ${#val} -gt ${MAX_DIFF_CHARS} ]]; then
            printf -v "${var}" '%s\n\n[... truncated at %d chars ...]' \
                "${val:0:${MAX_DIFF_CHARS}}" "${MAX_DIFF_CHARS}"
        fi
    done
}

# ── Quantitative Reports ──────────────────────────────────────────────────
generate_quantitative() {
    local ms="${REPO_ROOT}/bin/mindspec"

    REPORT_AB="$("${ms}" bench report "${WORK_DIR}/session-a.jsonl" "${WORK_DIR}/session-b.jsonl" \
        --labels "mindspec,baseline" 2>/dev/null || echo "(report failed)")"
    REPORT_AC="$("${ms}" bench report "${WORK_DIR}/session-a.jsonl" "${WORK_DIR}/session-c.jsonl" \
        --labels "mindspec,no-docs" 2>/dev/null || echo "(report failed)")"
    REPORT_BC="$("${ms}" bench report "${WORK_DIR}/session-b.jsonl" "${WORK_DIR}/session-c.jsonl" \
        --labels "baseline,no-docs" 2>/dev/null || echo "(report failed)")"
}

# ── Qualitative Analysis ──────────────────────────────────────────────────
run_qualitative() {
    local prompt_file="${WORK_DIR}/qual-prompt.txt"
    cat > "${prompt_file}" <<QUAL_EOF
You are a senior software engineer reviewing three implementations of the same feature,
produced by Claude Code under different conditions:

- **Session A (mindspec)**: Full MindSpec tooling — spec-driven workflow with CLAUDE.md,
  hooks, domain documentation, glossary, context map, and policies.
- **Session B (baseline)**: No MindSpec tooling (CLAUDE.md, .mindspec/ removed, hooks
  stripped from .claude/settings.json, MindSpec commands removed), but docs/ directory
  (domain docs, ADRs, glossary, context map) and .claude/ directory still present.
- **Session C (no-docs)**: Same as B but also with docs/ directory removed — pure freestyle
  with no project documentation.

All three sessions started from the same git commit and received the same feature prompt:

> ${PROMPT}

## Quantitative Reports

### A vs B (mindspec vs baseline)
\`\`\`
${REPORT_AB}
\`\`\`

### A vs C (mindspec vs no-docs)
\`\`\`
${REPORT_AC}
\`\`\`

### B vs C (baseline vs no-docs)
\`\`\`
${REPORT_BC}
\`\`\`

## Plans

### Session A Plan (mindspec plan.md)
\`\`\`markdown
${PLAN_A}
\`\`\`

### Session B Plan (Claude /plan mode)
\`\`\`markdown
${PLAN_B}
\`\`\`

### Session C Plan (Claude /plan mode)
\`\`\`markdown
${PLAN_C}
\`\`\`

## Implementation Diffs

### Session A (mindspec)
\`\`\`diff
${DIFF_A}
\`\`\`

### Session B (baseline)
\`\`\`diff
${DIFF_B}
\`\`\`

### Session C (no-docs)
\`\`\`diff
${DIFF_C}
\`\`\`

## Your Task

Analyze all three implementations and produce a structured comparison. Be completely unbiased.

### Dimensions

For each dimension, rate each session 1-5 and explain briefly:

1. **Planning Quality**: Clarity of the plan, scope decomposition, verification steps, architectural reasoning.
2. **Architecture**: Code organization, separation of concerns, package structure, interface design.
3. **Code Quality**: Readability, error handling, naming, idiomatic style, absence of code smells.
4. **Test Quality**: Coverage, edge cases, test isolation, meaningful assertions.
5. **Documentation**: Code comments, doc-sync, inline documentation quality.
6. **Functional Completeness**: Does the implementation satisfy the feature prompt fully?
7. **Consistency with Project Conventions**: Does the code follow patterns visible in the existing codebase?

### Output Format

Use this exact structure:

## Planning Quality
| Session | Rating | Assessment |
|---------|--------|------------|
| A (mindspec) | X/5 | ... |
| B (baseline) | X/5 | ... |
| C (no-docs)  | X/5 | ... |

[repeat for each dimension]

## Overall Verdict
[Which session produced the best overall result and why — 3-5 sentences]

## Key Differentiators
[What specific advantages did MindSpec provide, or fail to provide?]

## Surprising Findings
[Anything unexpected in the comparison]
QUAL_EOF

    echo "Running qualitative analysis..."
    QUAL_ANALYSIS="$(CLAUDECODE='' claude -p --no-session-persistence < "${prompt_file}" 2>/dev/null || echo "(qualitative analysis failed)")"
}

# ── Improvements Analysis ─────────────────────────────────────────────────
run_improvements() {
    local prompt_file="${WORK_DIR}/improv-prompt.txt"
    cat > "${prompt_file}" <<IMPROV_EOF
You are analyzing three implementations of the same feature to identify what the
non-MindSpec sessions (B and C) did BETTER than the MindSpec session (A).

The feature prompt was:
> ${PROMPT}

## Plans

### Session A Plan (mindspec)
\`\`\`markdown
${PLAN_A}
\`\`\`

### Session B Plan (baseline)
\`\`\`markdown
${PLAN_B}
\`\`\`

### Session C Plan (no-docs)
\`\`\`markdown
${PLAN_C}
\`\`\`

## Implementation Diffs

### Session A (mindspec)
\`\`\`diff
${DIFF_A}
\`\`\`

### Session B (baseline — no MindSpec, but has docs/)
\`\`\`diff
${DIFF_B}
\`\`\`

### Session C (no-docs — no MindSpec, no docs/)
\`\`\`diff
${DIFF_C}
\`\`\`

## Qualitative Analysis (already completed)

${QUAL_ANALYSIS}

## Your Task

Identify specific, actionable improvements from sessions B and C that session A should
adopt. Focus on:

1. **Code patterns** B/C used that are objectively better (simpler, more idiomatic, better error handling)
2. **Features or edge cases** B/C handled that A missed
3. **Architectural decisions** in B/C that are cleaner (even if session A was "correct by spec")
4. **Planning approaches** in B/C that produced better outcomes
5. **Test approaches** in B/C that are more thorough or practical
6. **Documentation or naming** that B/C got right where A did not

For each improvement, provide:
- Which session(s) it came from (B, C, or both)
- What specifically was better
- A concrete suggestion for how to incorporate it into the MindSpec implementation

If there are no meaningful improvements from B/C, say so explicitly.

Output format:

# Improvements from Non-MindSpec Sessions

## Summary
[1-2 sentences: overall, did B/C produce anything worth adopting?]

## Improvements

### 1. [Brief title]
**Source**: Session B / C / Both
**What was better**: ...
**Suggestion**: ...

### 2. [Brief title]
...

## Conclusion
[2-3 sentences: what does this tell us about MindSpec workflow gaps?]
IMPROV_EOF

    echo "Running improvements analysis..."
    IMPROVEMENTS="$(CLAUDECODE='' claude -p --no-session-persistence < "${prompt_file}" 2>/dev/null || echo "(improvements analysis failed)")"
}

# ── Assemble benchmark.md ─────────────────────────────────────────────────
assemble_benchmark_md() {
    local event_a=0 event_b=0 event_c=0
    [[ -f "${WORK_DIR}/session-a.jsonl" ]] && event_a=$(wc -l < "${WORK_DIR}/session-a.jsonl" | tr -d ' ')
    [[ -f "${WORK_DIR}/session-b.jsonl" ]] && event_b=$(wc -l < "${WORK_DIR}/session-b.jsonl" | tr -d ' ')
    [[ -f "${WORK_DIR}/session-c.jsonl" ]] && event_c=$(wc -l < "${WORK_DIR}/session-c.jsonl" | tr -d ' ')

    local trace_summary=""
    if [[ -f "${WORK_DIR}/trace-a.jsonl" ]]; then
        trace_summary="$("${REPO_ROOT}/bin/mindspec" trace summary "${WORK_DIR}/trace-a.jsonl" 2>/dev/null || echo "(trace summary unavailable)")"
    fi

    local prompt_display="${PROMPT}"
    if [[ ${#prompt_display} -gt 500 ]]; then
        prompt_display="${prompt_display:0:500}..."
    fi

    cat <<BENCH_EOF
# Benchmark: ${SPEC_ID}

**Date**: $(date +%Y-%m-%d)
**Commit**: ${BENCH_COMMIT}
**Timeout**: ${SESSION_TIMEOUT}s
**Model**: ${MODEL:-default}

## Prompt

${prompt_display}

## Sessions

| Session | Description | Port | Events |
|---------|-------------|------|--------|
| A (mindspec) | Full MindSpec tooling | ${PORT_A} | ${event_a} |
| B (baseline) | No CLAUDE.md/.mindspec; hooks stripped; docs/ present | ${PORT_B} | ${event_b} |
| C (no-docs)  | No CLAUDE.md/.mindspec; hooks stripped; no docs/ | ${PORT_C} | ${event_c} |

## Quantitative Comparison

### A vs B (mindspec vs baseline)

\`\`\`
${REPORT_AB}
\`\`\`

### A vs C (mindspec vs no-docs)

\`\`\`
${REPORT_AC}
\`\`\`

### B vs C (baseline vs no-docs)

\`\`\`
${REPORT_BC}
\`\`\`

## MindSpec Trace Summary (Session A)

\`\`\`
${trace_summary}
\`\`\`

## Qualitative Analysis

${QUAL_ANALYSIS:-_(skipped)_}

## Raw Data

Telemetry and output files are in \`${WORK_DIR}/\`:
- \`session-a.jsonl\` — Session A OTEL telemetry
- \`session-b.jsonl\` — Session B OTEL telemetry
- \`session-c.jsonl\` — Session C OTEL telemetry
- \`trace-a.jsonl\` — Session A MindSpec trace
- \`output-a.txt\` — Session A Claude output
- \`output-b.txt\` — Session B Claude output
- \`output-c.txt\` — Session C Claude output
BENCH_EOF
}

# ── Cleanup ────────────────────────────────────────────────────────────────
cleanup() {
    local exit_code=$?

    # Kill any remaining collector processes
    for pid in "${COLLECTOR_PIDS[@]+"${COLLECTOR_PIDS[@]}"}"; do
        kill "${pid}" 2>/dev/null || true
        wait "${pid}" 2>/dev/null || true
    done

    if [[ "${SKIP_CLEANUP}" != "true" && -n "${WORK_DIR}" ]]; then
        echo ""
        echo "Cleaning up worktrees..."
        for suffix in a b c; do
            local wt="${WORK_DIR}/wt-${suffix}"
            if [[ -d "${wt}" ]]; then
                git -C "${REPO_ROOT}" worktree remove --force "${wt}" 2>/dev/null || true
            fi
        done
        git -C "${REPO_ROOT}" worktree prune 2>/dev/null || true
    elif [[ "${SKIP_CLEANUP}" == "true" ]]; then
        echo ""
        echo "Skipping cleanup (--skip-cleanup). Worktrees at: ${WORK_DIR}/wt-{a,b,c}"
    fi

    exit "${exit_code}"
}

# ── Main ───────────────────────────────────────────────────────────────────
main() {
    parse_args "$@"

    REPO_ROOT="$(git rev-parse --show-toplevel)"
    BENCH_COMMIT="$(git -C "${REPO_ROOT}" rev-parse HEAD)"
    TIMESTAMP="$(date +%Y%m%d-%H%M%S)"

    if [[ -z "${WORK_DIR}" ]]; then
        WORK_DIR="/tmp/mindspec-bench-${SPEC_ID}"
    fi

    local spec_dir="${REPO_ROOT}/docs/specs/${SPEC_ID}"

    echo "MindSpec E2E Benchmark"
    echo "  Spec:    ${SPEC_ID}"
    echo "  Commit:  ${BENCH_COMMIT}"
    echo "  Timeout: ${SESSION_TIMEOUT}s per session"
    echo "  Work:    ${WORK_DIR}"
    echo ""

    validate_prerequisites

    mkdir -p "${WORK_DIR}"
    trap cleanup EXIT

    # Create worktrees
    echo "Creating worktrees..."
    create_worktree "bench-a-${SPEC_ID}-${TIMESTAMP}" "${WORK_DIR}/wt-a"
    create_worktree "bench-b-${SPEC_ID}-${TIMESTAMP}" "${WORK_DIR}/wt-b"
    create_worktree "bench-c-${SPEC_ID}-${TIMESTAMP}" "${WORK_DIR}/wt-c"

    # Neutralize B and C
    echo "Neutralizing sessions B and C..."
    neutralize_baseline "${WORK_DIR}/wt-b"
    neutralize_nodocs "${WORK_DIR}/wt-c"

    # Run sessions sequentially
    run_session "a" "${WORK_DIR}/wt-a" "${PORT_A}" "${WORK_DIR}/session-a.jsonl" "${WORK_DIR}/trace-a.jsonl"
    run_session "b" "${WORK_DIR}/wt-b" "${PORT_B}" "${WORK_DIR}/session-b.jsonl" ""
    run_session "c" "${WORK_DIR}/wt-c" "${PORT_C}" "${WORK_DIR}/session-c.jsonl" ""

    # Generate quantitative reports
    echo ""
    echo "Generating quantitative reports..."
    generate_quantitative

    # Generate diffs and collect plans
    echo "Collecting diffs and plans..."
    generate_diffs
    collect_plans

    # Qualitative analysis
    if [[ "${SKIP_QUALITATIVE}" != "true" ]]; then
        run_qualitative
        run_improvements
    else
        QUAL_ANALYSIS=""
        IMPROVEMENTS=""
    fi

    # Write results
    mkdir -p "${spec_dir}"

    echo "Writing benchmark.md..."
    assemble_benchmark_md > "${spec_dir}/benchmark.md"

    if [[ -n "${IMPROVEMENTS}" ]]; then
        echo "Writing improvements.md..."
        echo "${IMPROVEMENTS}" > "${spec_dir}/improvements.md"
    fi

    # Commit if requested
    if [[ "${SKIP_COMMIT}" != "true" ]]; then
        echo "Committing results..."
        git -C "${REPO_ROOT}" add \
            "docs/specs/${SPEC_ID}/benchmark.md" \
            "docs/specs/${SPEC_ID}/improvements.md" 2>/dev/null || true
        git -C "${REPO_ROOT}" commit -m "bench(${SPEC_ID}): add e2e benchmark results" --no-verify 2>/dev/null || true
    fi

    echo ""
    echo "Done. Results in docs/specs/${SPEC_ID}/"
    echo "  benchmark.md    — quantitative + qualitative report"
    if [[ -n "${IMPROVEMENTS}" ]]; then
        echo "  improvements.md — actionable findings from B/C"
    fi
}

main "$@"
