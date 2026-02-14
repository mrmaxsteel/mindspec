# Benchmarking MindSpec vs Freestyle Claude Code

This guide walks through running an A/B comparison between a MindSpec-assisted session and a freestyle Claude Code session, then producing a quantitative report. Both sessions run sequentially in the same window from the same starting commit.

## What You'll Measure

| Metric | Source |
|:-------|:-------|
| Input/output/cache tokens | Claude Code OTel telemetry |
| Estimated cost (USD) | Claude Code OTel telemetry |
| Wall-clock duration | Event timestamps |
| API call count | Claude Code OTel telemetry |
| Cache hit rate | Derived from token counts |
| Context pack token overhead | MindSpec `--trace` (Session A only) |

## Prerequisites

- MindSpec binary built: `make build`
- A feature description written down (same prompt for both sessions)
- Clean git working tree

## Step 1: Record the Starting Commit

```bash
export BENCH_START=$(git rev-parse HEAD)
echo "Starting commit: $BENCH_START"
```

## Step 2: Print the Setup Instructions

```bash
mindspec bench setup
```

This prints environment variable blocks for both sessions. You can also configure manually — see below.

## Step 3: Run Session A (MindSpec)

### Start the collector

In a separate terminal:

```bash
mindspec bench collect --port 4318 --output /tmp/bench-session-a.jsonl
```

### Configure telemetry

In your main terminal, set these environment variables **before** starting Claude Code:

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export MINDSPEC_TRACE=/tmp/mindspec-bench-a-trace.jsonl
```

### Run the session

Start Claude Code and follow the MindSpec workflow:

1. `/spec-init` → write the spec
2. `/spec-approve` → plan
3. Write the plan → `/plan-approve`
4. `mindspec next` → implement each bead
5. `mindspec complete` when done

When finished, exit Claude Code and press `Ctrl-C` in the collector terminal.

## Step 4: Preserve Session A and Reset

```bash
# Commit any uncommitted work
git add -A && git commit -m "bench: Session A (mindspec)"

# Tag the result so you can come back to it
git tag bench-a-result

# Reset back to the starting commit
git checkout $BENCH_START
```

You're now back at the exact same starting point, on a detached HEAD.

## Step 5: Neutralize MindSpec for Session B

The repo contains `CLAUDE.md`, `.mindspec/`, and `.claude/` — these would cause Claude Code to follow MindSpec workflows automatically. Strip them out:

```bash
# Remove MindSpec project instructions (Claude Code reads this automatically)
rm -f CLAUDE.md

# Remove MindSpec state (prevents hooks from emitting guidance)
rm -rf .mindspec/

# Remove project-level Claude Code hooks and rules
rm -rf .claude/
```

These deletions are uncommitted and will be discarded when you return to a branch. Without these files, Claude Code operates as a vanilla agent with no MindSpec awareness.

## Step 6: Run Session B (Baseline)

### Start the collector

In a separate terminal:

```bash
mindspec bench collect --port 4318 --output /tmp/bench-session-b.jsonl
```

### Configure telemetry

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

(No `MINDSPEC_TRACE` — there's no MindSpec to trace.)

### Run the session

Start Claude Code and give it the **exact same feature description** you used in Session A. For example:

> "Implement a `widget list` command that reads widgets from a YAML file and prints them in a table. Include tests."

No MindSpec commands — just let Claude implement it however it wants. Iterate until satisfied.

When finished, exit Claude Code and press `Ctrl-C` in the collector terminal.

**Important:** Try to achieve roughly equivalent quality in both sessions. The comparison is most meaningful when both produce working, tested code.

## Step 7: Preserve Session B and Return to Main

```bash
# Commit any uncommitted work (on detached HEAD)
git add -A && git commit -m "bench: Session B (baseline)"
git tag bench-b-result

# Return to main
git checkout main
```

## Step 8: Generate the Report

```bash
mindspec bench report \
  /tmp/bench-session-a.jsonl \
  /tmp/bench-session-b.jsonl \
  --labels "mindspec,baseline"
```

Output:

```
Metric                        mindspec        baseline           Delta
────────────────────────────────────────────────────────────────────
API Calls                           12              18              -6
Input Tokens                     45000           82000          -37000
Output Tokens                     8500           14000           -5500
Cache Read Tokens                12000            3000           +9000
Cache Create Tokens               2000            1500            +500
Total Tokens                     53500           96000          -42500
Cost (USD)                     $0.3200         $0.6800        -$0.3600
Duration                       8.2 min        14.5 min        -6.3 min
Cache Hit Rate                   20.3%            3.5%
Output/Input Ratio               0.19x           0.17x
```

For machine-readable output:

```bash
mindspec bench report /tmp/bench-session-a.jsonl /tmp/bench-session-b.jsonl \
  --labels "mindspec,baseline" --format json > /tmp/bench-report.json
```

## Step 9: Inspect MindSpec Trace (Optional)

Session A also produces a MindSpec-side trace showing where tokens were spent:

```bash
mindspec trace summary /tmp/mindspec-bench-a-trace.jsonl
```

```
Trace Summary: /tmp/mindspec-bench-a-trace.jsonl
  Events:     34
  Duration:   4823.1 ms
  Tokens:     3142

  Event                      Count   Duration   Tokens
  -----------------------------------------------------
  bead.cli                      12   3200.0 ms        -
  command.end                    8    4823.1 ms        -
  command.start                  8          -        -
  contextpack.build              2    120.3 ms     2400
  glossary.match                 1      1.2 ms       42
  instruct.render                2      5.1 ms      700
  state.transition               3          -        -
```

You can also inspect individual events:

```bash
# See context pack token breakdown
jq 'select(.event=="contextpack.build") | .data' /tmp/mindspec-bench-a-trace.jsonl

# See all bead CLI calls sorted by duration
jq -r 'select(.event=="bead.cli") | "\(.data.dur_ms | tostring | .[:6])ms \(.data.op) \(.data.args | join(" "))"' \
  /tmp/mindspec-bench-a-trace.jsonl | sort -rn
```

## Comparing the Code

To diff the output of both sessions:

```bash
git diff bench-a-result bench-b-result
```

## Cleanup

```bash
# Remove tags
git tag -d bench-a-result bench-b-result

# Remove trace files
rm -f /tmp/bench-session-a.jsonl /tmp/bench-session-b.jsonl /tmp/mindspec-bench-a-trace.jsonl
```

## Automated E2E Benchmarking

For repeatable benchmarks, use `scripts/bench-e2e.sh`. It automates the full workflow — worktree creation, session execution, telemetry collection, quantitative reports, qualitative analysis, and result persistence — for 3 sessions:

| Session | Description |
|:--------|:------------|
| A (mindspec) | Full MindSpec tooling |
| B (baseline) | No CLAUDE.md/.mindspec; hooks stripped from settings; docs/ present |
| C (no-docs)  | Same as B plus docs/ removed — no project documentation at all |

### Example

```bash
scripts/bench-e2e.sh \
  --spec-id 015-project-bootstrap \
  --prompt "Plan and implement: mindspec init — a bootstrap command that scaffolds a new project" \
  --max-turns 30 \
  --timeout 1800
```

Or read the prompt from a file:

```bash
scripts/bench-e2e.sh \
  --spec-id 015-project-bootstrap \
  --prompt-file prompts/015.txt
```

### Flags

| Flag | Description | Default |
|:-----|:------------|:--------|
| `--spec-id <NNN-slug>` | Spec folder ID (required) | — |
| `--prompt <string>` | Feature prompt for all 3 sessions (required unless `--prompt-file`) | — |
| `--prompt-file <path>` | Read prompt from file | — |
| `--timeout <seconds>` | Per-session timeout | 1800 (30 min) |
| `--max-turns <int>` | Max agentic turns per session | unlimited |
| `--model <model>` | Claude model for all sessions | system default |
| `--work-dir <path>` | Base dir for worktrees | `/tmp/mindspec-bench-<spec-id>` |
| `--skip-cleanup` | Preserve worktrees after completion | false |
| `--skip-qualitative` | Skip qualitative analysis (quantitative only) | false |
| `--skip-commit` | Don't commit results to docs/specs/ | false |

### Output

Results are written to `docs/specs/<spec-id>/`:

- **`benchmark.md`** — metadata, 3 pairwise quantitative reports, qualitative analysis with per-dimension 1-5 ratings
- **`improvements.md`** — actionable findings: what the non-MindSpec sessions did better

The script auto-commits these to the current branch unless `--skip-commit` is passed.

### How It Works

1. Creates 3 git worktrees from the current HEAD
2. Neutralizes B (removes CLAUDE.md, .mindspec/, MindSpec commands, hooks) and C (same + removes docs/)
3. Runs `claude -p` sequentially in each worktree with OTEL telemetry pointed at a per-session collector
4. Collects plans: Session A's `docs/specs/<ID>/plan.md`, Sessions B/C's `.claude/plans/*.md`
5. Generates pairwise `mindspec bench report` comparisons (A-vs-B, A-vs-C, B-vs-C)
6. Runs a qualitative analysis via `claude -p` comparing all 3 implementations and plans
7. Runs an improvements analysis identifying what B/C did better
8. Assembles results and cleans up worktrees

The manual workflow above remains available for interactive sessions where you want more control.

## Tips

- **Same complexity:** Pick a feature that's complex enough to show differentiation (at least 2-3 files, with tests) but not so large that session variance dominates.
- **Same model:** Ensure both sessions use the same Claude model. Check with `/model` in Claude Code, or use `--model` with the automated script.
- **Warm cache:** If running multiple experiments, the second run may benefit from prompt caching. Consider running a throwaway warm-up session first, or comparing cache rates as part of the analysis.
- **Quality:** The quantitative report measures cost and time. For the manual workflow, assess quality separately. The automated script handles this via qualitative analysis.
- **Order bias:** Running MindSpec first means the baseline session happens after you've already seen one implementation. To control for this, consider alternating order across experiments.
